// Command agent is the ForgeGuardian autonomous security patch agent.
// It builds a package hermetically, scans it, generates an AI triage advisory,
// plans a patch, reviews it, and optionally applies it.
//
// Usage:
//
//	agent --recipe=npm --package=lodash --version=4.17.21
//	agent --recipe=pypi --package=requests --version=2.28.0 --apply
//	agent --recipe=npm --package=lodash --version=4.17.21 --skip-scan --json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/build/recipes"
	_ "github.com/mah3sec/forgeguardian/internal/build/recipes/all"
	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/scanner"

	"github.com/mah3sec/forgeguardian/internal/agent/executor"
	"github.com/mah3sec/forgeguardian/internal/agent/planner"
	"github.com/mah3sec/forgeguardian/internal/agent/reviewer"
	"github.com/mah3sec/forgeguardian/internal/agent/triage"
)

func main() {
	var (
		recipeFlag     = flag.String("recipe", "", "ecosystem recipe (npm|pypi|maven|go|rubygems|crates|huggingface|mcp)")
		packageFlag    = flag.String("package", "", "package name")
		versionFlag    = flag.String("version", "", "package version")
		checksumFlag   = flag.String("checksum", "", "optional expected SHA256")
		applyFlag      = flag.Bool("apply", false, "apply the patch (default: dry-run only)")
		skipScanFlag   = flag.Bool("skip-scan", false, "skip scan, use empty findings (for testing triage)")
		jsonFlag       = flag.Bool("json", false, "output full result as JSON")
		projectDirFlag = flag.String("project-dir", ".", "project directory containing manifests to patch")
		apiKeyFlag     = flag.String("api-key", "", "Anthropic API key (default: ANTHROPIC_API_KEY env var)")
		timeoutFlag    = flag.Duration("timeout", 15*time.Minute, "total agent timeout")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *recipeFlag == "" || *packageFlag == "" || *versionFlag == "" {
		logger.Error("--recipe, --package, and --version are required")
		flag.Usage()
		os.Exit(1)
	}

	apiKey := *apiKeyFlag
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		logger.Error("ANTHROPIC_API_KEY not set — set it or pass --api-key")
		os.Exit(1)
	}

	recipe, err := recipes.Get(*recipeFlag)
	if err != nil {
		logger.Error("recipe not found", "recipe", *recipeFlag)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	// ── Step 1: Build ────────────────────────────────────────────────────────
	logger.Info("step 1/5: building artifact",
		"recipe", *recipeFlag, "package", *packageFlag, "version", *versionFlag)

	src := core.SourceArtifact{
		Package: core.PackageVersion{
			Ecosystem: *recipeFlag,
			Name:      *packageFlag,
			Version:   *versionFlag,
			Checksum:  *checksumFlag,
		},
	}
	artifact, err := recipe.Build(ctx, src, nil)
	if err != nil {
		logger.Error("build failed", "error", err)
		os.Exit(1)
	}
	defer os.Remove(artifact.LocalPath)
	logger.Info("build succeeded", "sha256", artifact.SHA256)

	// ── Step 2: Scan ─────────────────────────────────────────────────────────
	var findings []core.Finding
	if !*skipScanFlag {
		logger.Info("step 2/5: scanning artifact")
		orch := scanner.New()
		results := orch.Scan(ctx, artifact)
		findings = scanner.MergeFindings(results)
		sum := scanner.Summarize(findings)
		logger.Info("scan complete",
			"total", sum.Total, "critical", sum.Critical,
			"high", sum.High, "medium", sum.Medium)
	} else {
		logger.Info("step 2/5: scan skipped (--skip-scan)")
	}

	// ── Step 3: Triage ───────────────────────────────────────────────────────
	logger.Info("step 3/5: generating AI triage advisory")
	triageEngine := triage.New(apiKey)
	advisory, err := triageEngine.Triage(ctx, artifact, findings)
	if err != nil {
		logger.Error("triage failed", "error", err)
		os.Exit(1)
	}
	logger.Info("triage complete",
		"severity", advisory.Severity,
		"confidence", fmt.Sprintf("%.0f%%", advisory.Confidence*100))

	// ── Step 4: Plan ─────────────────────────────────────────────────────────
	logger.Info("step 4/5: planning patch")
	patchPlanner := planner.New(apiKey)
	plan, err := patchPlanner.Plan(ctx, advisory)
	if err != nil {
		logger.Error("planning failed", "error", err)
		os.Exit(1)
	}
	logger.Info("plan produced",
		"actions", len(plan.Actions),
		"risk_level", plan.RiskLevel,
		"auto_apply", plan.AutoApply)

	// ── Step 5: Review + Execute ─────────────────────────────────────────────
	logger.Info("step 5/5: reviewing and executing patch",
		"dry_run", !*applyFlag)
	exec := executor.New(*projectDirFlag, !*applyFlag)
	execResults := exec.Execute(plan, advisory)

	patchReviewer := reviewer.New(apiKey)
	review, err := patchReviewer.Review(ctx, advisory, plan, execResults)
	if err != nil {
		logger.Warn("review failed (non-fatal)", "error", err)
	}

	logger.Info("review complete",
		"approved", review.Approved,
		"recommendation", review.Recommendation,
		"confidence", fmt.Sprintf("%.0f%%", review.ConfidenceScore*100))

	// ── Output ───────────────────────────────────────────────────────────────
	if *jsonFlag {
		output := map[string]any{
			"package":      *packageFlag,
			"version":      *versionFlag,
			"recipe":       *recipeFlag,
			"sha256":       artifact.SHA256,
			"advisory":     advisory,
			"patch_plan":   plan,
			"exec_results": execResults,
			"review":       review,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(output)
		return
	}

	printReport(artifact, advisory, plan, execResults, review, *applyFlag)
}

func printReport(
	artifact core.BuiltArtifact,
	advisory core.Advisory,
	plan planner.PatchPlan,
	execResults []executor.Result,
	review reviewer.Review,
	applied bool,
) {
	pkg := artifact.Source.Package
	sep := strings.Repeat("═", 72)
	fmt.Printf("\n%s\n  ForgeGuardian Autonomous Agent Report\n  %s@%s (%s)\n%s\n\n",
		sep, pkg.Name, pkg.Version, pkg.Ecosystem, sep)

	fmt.Printf("▸ SEVERITY: %s  (confidence %.0f%%)\n", advisory.Severity, advisory.Confidence*100)
	fmt.Printf("▸ ADVISORY:\n  %s\n\n", wrapText(advisory.Advisory, 68))
	fmt.Printf("▸ EXPLOITABILITY:\n  %s\n\n", wrapText(advisory.ExploitabilityRationale, 68))
	if advisory.AgenticRisk != nil {
		fmt.Printf("▸ AGENTIC RISK:\n  %s\n\n", wrapText(*advisory.AgenticRisk, 68))
	}
	fmt.Printf("▸ RECOMMENDED ACTION: %s\n\n", advisory.RecommendedAction)
	if advisory.PatchSuggestion != nil {
		fmt.Printf("▸ PATCH SUGGESTION:\n  %s\n\n", *advisory.PatchSuggestion)
	}

	fmt.Printf("─── PATCH PLAN (risk: %s, auto-apply: %v) ─────────────\n",
		plan.RiskLevel, plan.AutoApply)
	for _, action := range plan.Actions {
		fmt.Printf("  [%s] %s\n", strings.ToUpper(action.Type), action.Description)
	}
	fmt.Printf("  Rationale: %s\n\n", plan.Rationale)

	mode := "DRY-RUN"
	if applied {
		mode = "APPLIED"
	}
	fmt.Printf("─── EXECUTION (%s) ─────────────────────────────────────\n", mode)
	for _, r := range execResults {
		status := "○"
		if r.Applied {
			status = "✓"
		}
		if r.Err != nil {
			status = "✗"
		}
		fmt.Printf("  %s %s\n", status, r.Changed)
		if r.Err != nil {
			fmt.Printf("    ERROR: %v\n", r.Err)
		}
	}
	fmt.Println()

	approved := "✗ REJECTED"
	if review.Approved {
		approved = "✓ APPROVED"
	}
	fmt.Printf("─── AI REVIEW: %s (%s) ─────────────────────────\n",
		approved, review.Recommendation)
	fmt.Printf("  %s\n", review.RiskAssessment)
	for _, c := range review.Concerns {
		fmt.Printf("  ⚠  %s\n", c)
	}
	fmt.Printf("\n%s\n", sep)
}

func wrapText(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	var lines []string
	line := ""
	for _, w := range words {
		if len(line)+len(w)+1 > width {
			lines = append(lines, line)
			line = w
		} else {
			if line == "" {
				line = w
			} else {
				line += " " + w
			}
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n  ")
}
