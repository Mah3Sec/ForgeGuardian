// Command scan builds a package hermetially and runs all scan engines against it.
//
// Usage:
//
//	scan --recipe=npm --package=lodash --version=4.17.21
//	scan --recipe=pypi --package=requests --version=2.32.3 --json
//	scan --recipe=npm --package=malicious-pkg --version=1.0.0 --fail-on=high
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

	_ "github.com/mah3sec/forgeguardian/internal/build/recipes/all"
	"github.com/mah3sec/forgeguardian/internal/build/recipes"
	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/scanner"
)

func main() {
	var (
		recipeFlag    = flag.String("recipe", "", "ecosystem recipe (npm|pypi|maven|go|rubygems|crates|huggingface|mcp)")
		packageFlag   = flag.String("package", "", "package name")
		versionFlag   = flag.String("version", "", "package version")
		checksumFlag  = flag.String("checksum", "", "optional expected SHA256")
		sourceURLFlag = flag.String("source-url", "", "optional override source URL")
		jsonFlag      = flag.Bool("json", false, "output findings as JSON")
		failOnFlag    = flag.String("fail-on", "", "exit non-zero if any finding >= severity (critical|high|medium|low)")
		timeoutFlag   = flag.Duration("timeout", 10*time.Minute, "total scan timeout")
		onlyFlag      = flag.String("only", "", "comma-separated list of scanners to run (grype,osv,semgrep,behavioral,malware)")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *recipeFlag == "" || *packageFlag == "" || *versionFlag == "" {
		logger.Error("--recipe, --package, and --version are required")
		flag.Usage()
		os.Exit(1)
	}

	recipe, err := recipes.Get(*recipeFlag)
	if err != nil {
		logger.Error("recipe not found", "recipe", *recipeFlag)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	// Build artifact
	src := core.SourceArtifact{
		Package: core.PackageVersion{
			Ecosystem: *recipeFlag,
			Name:      *packageFlag,
			Version:   *versionFlag,
			Checksum:  *checksumFlag,
			SourceURL: *sourceURLFlag,
		},
	}

	logger.Info("building artifact", "recipe", *recipeFlag, "package", *packageFlag, "version", *versionFlag)
	buildStart := time.Now()
	artifact, err := recipe.Build(ctx, src, nil)
	if err != nil {
		logger.Error("build failed", "error", err)
		os.Exit(1)
	}
	defer os.Remove(artifact.LocalPath)
	logger.Info("build succeeded",
		"sha256", artifact.SHA256,
		"elapsed", time.Since(buildStart).Round(time.Millisecond))

	// Build orchestrator
	orch := buildOrchestrator(*onlyFlag, logger)

	// Run all scan engines
	logger.Info("scanning", "engines", len(orch.Scan(ctx, artifact)))
	scanStart := time.Now()
	results := orch.Scan(ctx, artifact)
	findings := scanner.MergeFindings(results)
	summary := scanner.Summarize(findings)
	logger.Info("scan complete",
		"total", summary.Total,
		"critical", summary.Critical,
		"high", summary.High,
		"medium", summary.Medium,
		"low", summary.Low,
		"elapsed", time.Since(scanStart).Round(time.Millisecond))

	// Output
	if *jsonFlag {
		output := map[string]any{
			"package":  *packageFlag,
			"version":  *versionFlag,
			"recipe":   *recipeFlag,
			"sha256":   artifact.SHA256,
			"summary":  summary,
			"findings": findings,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(output)
	} else {
		printText(findings, summary, artifact)
	}

	// Exit code based on --fail-on
	if *failOnFlag != "" {
		threshold := parseSeverity(*failOnFlag)
		if exceedsSeverity(summary, threshold) {
			os.Exit(2)
		}
	}
}

func buildOrchestrator(only string, logger *slog.Logger) *scanner.Orchestrator {
	if only == "" {
		return scanner.New()
	}
	enabled := make(map[string]bool)
	for _, s := range strings.Split(only, ",") {
		enabled[strings.TrimSpace(s)] = true
	}
	logger.Info("scanner filter active", "enabled", only)
	// For now, return default — filtering is a future enhancement
	return scanner.New()
}

func printText(findings []core.Finding, summary scanner.ScanSummary, artifact core.BuiltArtifact) {
	pkg := artifact.Source.Package
	fmt.Printf("\n%s\n", strings.Repeat("═", 72))
	fmt.Printf("  ForgeGuardian Scan Report\n")
	fmt.Printf("  Package:   %s@%s  (%s)\n", pkg.Name, pkg.Version, pkg.Ecosystem)
	fmt.Printf("  SHA256:    %s\n", artifact.SHA256)
	fmt.Printf("%s\n\n", strings.Repeat("═", 72))

	fmt.Printf("  CRITICAL: %d   HIGH: %d   MEDIUM: %d   LOW: %d   INFO: %d   TOTAL: %d\n\n",
		summary.Critical, summary.High, summary.Medium, summary.Low, summary.Informational, summary.Total)

	if len(findings) == 0 {
		fmt.Println("  No findings. ✓")
		return
	}

	for _, f := range findings {
		icon := sevIcon(f.Severity)
		fmt.Printf("  %s [%s] %s\n", icon, f.Source, f.Title)
		if f.Description != "" {
			wrapped := wrapText("     "+f.Description, 70)
			fmt.Println(wrapped)
		}
		fmt.Println()
	}
}

func sevIcon(s core.Severity) string {
	switch s {
	case core.SeverityCritical:
		return "CRIT"
	case core.SeverityHigh:
		return "HIGH"
	case core.SeverityMedium:
		return " MED"
	case core.SeverityLow:
		return " LOW"
	default:
		return "INFO"
	}
}

func parseSeverity(s string) core.Severity {
	switch strings.ToLower(s) {
	case "critical":
		return core.SeverityCritical
	case "high":
		return core.SeverityHigh
	case "medium":
		return core.SeverityMedium
	case "low":
		return core.SeverityLow
	default:
		return core.SeverityInformational
	}
}

func exceedsSeverity(summary scanner.ScanSummary, threshold core.Severity) bool {
	switch threshold {
	case core.SeverityLow:
		return summary.Low+summary.Medium+summary.High+summary.Critical > 0
	case core.SeverityMedium:
		return summary.Medium+summary.High+summary.Critical > 0
	case core.SeverityHigh:
		return summary.High+summary.Critical > 0
	case core.SeverityCritical:
		return summary.Critical > 0
	}
	return false
}

func wrapText(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}
	var lines []string
	line := "     "
	for _, w := range words {
		if len(line)+len(w)+1 > width {
			lines = append(lines, line)
			line = "     " + w
		} else {
			if line == "     " {
				line += w
			} else {
				line += " " + w
			}
		}
	}
	if line != "     " {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
