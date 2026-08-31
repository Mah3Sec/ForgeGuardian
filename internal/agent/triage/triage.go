// Package triage uses an AI provider to generate security advisories
// from a set of scan findings. It produces a structured core.Advisory with
// severity assessment, exploitability rationale, and recommended action.
package triage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/ai"
	"github.com/mah3sec/forgeguardian/internal/core"
)

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

// Engine generates AI-powered security advisories.
type Engine struct {
	provider ai.Provider
}

// New creates a triage Engine using the given AI provider.
func New(provider ai.Provider) *Engine {
	return &Engine{provider: provider}
}

// NewFromAPIKey creates a triage Engine using legacy Anthropic API key.
// Deprecated: use New(provider) with ai.NewProviderFromEnv() instead.
func NewFromAPIKey(apiKey string) *Engine {
	cfg := ai.LoadConfig()
	if apiKey != "" {
		cfg.APIKey = apiKey
	}
	p, err := ai.NewProvider(cfg)
	if err != nil {
		p, _ = ai.NewAnthropicProvider(ai.Config{APIKey: apiKey})
	}
	return &Engine{provider: p}
}

// Triage generates a security advisory for an artifact based on scan findings.
func (e *Engine) Triage(ctx context.Context, artifact core.BuiltArtifact, findings []core.Finding) (core.Advisory, error) {
	pkg := artifact.Source.Package

	prompt := buildPrompt(artifact, findings)

	resp, err := e.provider.Complete(ctx, ai.CompletionRequest{
		SystemPrompt: systemPrompt,
		Messages:     []ai.Message{{Role: "user", Content: prompt}},
		MaxTokens:    maxTokens,
	})
	if err != nil {
		return core.Advisory{}, fmt.Errorf("triage: %s: %w", e.provider.Name(), err)
	}

	if resp.Text == "" {
		return core.Advisory{}, fmt.Errorf("triage: empty response from %s", e.provider.Name())
	}

	advisory, err := parseAdvisoryJSON(resp.Text, pkg, findings)
	if err != nil {
		advisory = fallbackAdvisory(pkg, findings, resp.Text)
	}
	advisory.GeneratedAt = time.Now().UTC()
	return advisory, nil
}

func buildPrompt(artifact core.BuiltArtifact, findings []core.Finding) string {
	pkg := artifact.Source.Package
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
