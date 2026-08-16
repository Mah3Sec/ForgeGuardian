// Package triage uses the Anthropic API to generate AI-driven security advisories
// from a set of scan findings. It produces a structured core.Advisory with
// severity assessment, exploitability rationale, and recommended action.
package triage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/mah3sec/forgeguardian/internal/core"
)

const model anthropic.Model = "claude-sonnet-4-20250514"

const (
	maxTokens    = 2048
	systemPrompt = `You are ForgeGuardian's AI security triage engine — a senior supply chain security analyst.

Your job: given a package and its scan findings, produce a structured security advisory.

Rules:
- Be precise. Cite specific finding IDs. Don't hallucinate CVEs.
- Assess real-world exploitability, not just CVSS scores.
- For AI model / MCP server packages, assess agentic attack surface separately.
- Recommend concrete actions (upgrade version, avoid package, pin to safe hash).
- Respond ONLY with the JSON schema requested — no preamble, no markdown fences.`
)

// Engine generates AI-powered security advisories using Claude.
type Engine struct {
	client *anthropic.Client
}

// New creates a new triage Engine. If apiKey is empty, it reads ANTHROPIC_API_KEY.
func New(apiKey string) *Engine {
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	c := anthropic.NewClient(opts...)
	return &Engine{client: &c}
}

// Triage generates a security advisory for an artifact based on scan findings.
func (e *Engine) Triage(ctx context.Context, artifact core.BuiltArtifact, findings []core.Finding) (core.Advisory, error) {
	pkg := artifact.Source.Package

	// Build the user prompt with structured finding data
	prompt := buildPrompt(artifact, findings)

	msg, err := e.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: maxTokens,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return core.Advisory{}, fmt.Errorf("triage: anthropic api: %w", err)
	}

	if len(msg.Content) == 0 {
		return core.Advisory{}, fmt.Errorf("triage: empty response from model")
	}

	var raw string
	for _, block := range msg.Content {
		if tb := block.AsText(); tb.Text != "" {
			raw = tb.Text
			break
		}
	}

	advisory, err := parseAdvisoryJSON(raw, pkg, findings)
	if err != nil {
		// Fall back to a minimal advisory if JSON parsing fails
		advisory = fallbackAdvisory(pkg, findings, raw)
	}
	advisory.GeneratedAt = time.Now().UTC()
	return advisory, nil
}

// buildPrompt constructs the user-facing prompt with package + finding data.
func buildPrompt(artifact core.BuiltArtifact, findings []core.Finding) string {
	pkg := artifact.Source.Package

	// Serialize findings to JSON for precise representation
	findingsJSON, _ := json.MarshalIndent(findings, "", "  ")

	return fmt.Sprintf(`Analyze this package and produce a security advisory in the exact JSON format below.

## Package
- Name: %s
- Version: %s
- Ecosystem: %s
- SHA256: %s

## Scan Findings (%d total)
%s

## Required JSON Response Format
{
  "severity": "CRITICAL|HIGH|MEDIUM|LOW|INFORMATIONAL",
  "confidence": 0.0-1.0,
  "advisory": "One-paragraph human-readable advisory summary (max 300 words)",
  "exploitability_rationale": "Why is this exploitable or not in practice? Consider attack vector, complexity, required privileges.",
  "agentic_risk": "For AI models/MCP servers only: describe the agentic attack surface. Null for other ecosystems.",
  "recommended_action": "Specific action: upgrade to X.Y.Z / avoid entirely / pin to SHA256 hash / no action needed",
  "patch_suggestion": "For code injection / prototype pollution: brief code fix suggestion. Null otherwise."
}

Respond with ONLY the JSON object — no markdown, no explanation outside the JSON.`,
		pkg.Name, pkg.Version, pkg.Ecosystem, artifact.SHA256,
		len(findings), string(findingsJSON))
}

// triageResponse is the expected JSON shape from Claude.
type triageResponse struct {
	Severity                string  `json:"severity"`
	Confidence              float64 `json:"confidence"`
	Advisory                string  `json:"advisory"`
	ExploitabilityRationale string  `json:"exploitability_rationale"`
	AgenticRisk             *string `json:"agentic_risk"`
	RecommendedAction       string  `json:"recommended_action"`
	PatchSuggestion         *string `json:"patch_suggestion"`
}

func parseAdvisoryJSON(raw string, pkg core.PackageVersion, findings []core.Finding) (core.Advisory, error) {
	// Strip any markdown fences if Claude wrapped the output anyway
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		var inner []string
		inBlock := false
		for _, l := range lines {
			if strings.HasPrefix(l, "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				inner = append(inner, l)
			}
		}
		raw = strings.Join(inner, "\n")
	}

	var resp triageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return core.Advisory{}, fmt.Errorf("triage: parse json: %w (raw: %s)", err, truncate(raw, 200))
	}

	return core.Advisory{
		Package:                 pkg,
		Severity:                core.Severity(resp.Severity),
		Confidence:              resp.Confidence,
		Advisory:                resp.Advisory,
		ExploitabilityRationale: resp.ExploitabilityRationale,
		AgenticRisk:             resp.AgenticRisk,
		RecommendedAction:       resp.RecommendedAction,
		PatchSuggestion:         resp.PatchSuggestion,
		Findings:                findings,
	}, nil
}

func fallbackAdvisory(pkg core.PackageVersion, findings []core.Finding, rawResponse string) core.Advisory {
	sev := highestSeverity(findings)
	return core.Advisory{
		Package:                 pkg,
		Severity:                sev,
		Confidence:              0.5,
		Advisory:                fmt.Sprintf("AI triage produced a non-parseable response for %s@%s. Manual review required. Raw: %s", pkg.Name, pkg.Version, truncate(rawResponse, 300)),
		ExploitabilityRationale: "Could not be automatically assessed.",
		RecommendedAction:       "Review the scan findings manually.",
		Findings:                findings,
	}
}

func highestSeverity(findings []core.Finding) core.Severity {
	best := core.SeverityInformational
	for _, f := range findings {
		if severityOrd(f.Severity) > severityOrd(best) {
			best = f.Severity
		}
	}
	return best
}

func severityOrd(s core.Severity) int {
	switch s {
	case core.SeverityCritical:
		return 4
	case core.SeverityHigh:
		return 3
	case core.SeverityMedium:
		return 2
	case core.SeverityLow:
		return 1
	default:
		return 0
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
