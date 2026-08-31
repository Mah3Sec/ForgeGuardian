// Package planner uses AI tool-use to autonomously reason about
// and plan patch actions for a given security advisory.
// It runs a multi-turn agentic loop: AI calls tools → we execute → AI reasons → repeat.
package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mah3sec/forgeguardian/internal/ai"
	"github.com/mah3sec/forgeguardian/internal/core"
)

const (
	maxTokens = 4096
	maxIter   = 10
)

// PatchPlan describes the actions the planner decided to take.
type PatchPlan struct {
	PackageName    string        `json:"package_name"`
	CurrentVersion string        `json:"current_version"`
	Actions        []PatchAction `json:"actions"`
	Rationale      string        `json:"rationale"`
	RiskLevel      string        `json:"risk_level"` // low|medium|high
	AutoApply      bool          `json:"auto_apply"` // true if safe to apply without human review
}

// PatchAction is a single concrete remediation step.
type PatchAction struct {
	Type        string `json:"type"` // upgrade|pin|replace|remove|noaction
	Description string `json:"description"`
	OldValue    string `json:"old_value,omitempty"`
	NewValue    string `json:"new_value,omitempty"`
	FilePath    string `json:"file_path,omitempty"`
}

// Planner runs an agentic planning loop using AI tool use.
type Planner struct {
	provider ai.Provider
}

// New creates a new patch Planner with the given AI provider.
func New(provider ai.Provider) *Planner {
	return &Planner{provider: provider}
}

// NewFromAPIKey creates a Planner using legacy Anthropic API key.
// Deprecated: use New(provider) with ai.NewProviderFromEnv() instead.
func NewFromAPIKey(apiKey string) *Planner {
	cfg := ai.LoadConfig()
	if apiKey != "" {
		cfg.APIKey = apiKey
	}
	p, err := ai.NewProvider(cfg)
	if err != nil {
		p, _ = ai.NewAnthropicProvider(ai.Config{APIKey: apiKey})
	}
	return &Planner{provider: p}
}

// Plan generates a PatchPlan for the given advisory using a multi-turn tool-use loop.
func (p *Planner) Plan(ctx context.Context, advisory core.Advisory) (PatchPlan, error) {
	if !p.provider.SupportsToolUse() {
		return p.planWithoutTools(ctx, advisory)
	}

	tools := buildTools()
	messages := []ai.Message{{Role: "user", Content: buildPlannerPrompt(advisory)}}

	var toolState plannerState
	toolState.advisory = advisory

	for i := 0; i < maxIter; i++ {
		resp, err := p.provider.Complete(ctx, ai.CompletionRequest{
			SystemPrompt: plannerSystemPrompt,
			Messages:     messages,
			MaxTokens:    maxTokens,
			Tools:        tools,
		})
		if err != nil {
			return PatchPlan{}, fmt.Errorf("planner: api call %d: %w", i, err)
		}

		messages = append(messages, ai.Message{Role: "assistant", Content: resp.Text})

		if resp.StopReason == "end_turn" {
			return extractPlanFromText(resp.Text, advisory)
		}

		if resp.StopReason != "tool_use" || len(resp.ToolCalls) == 0 {
			break
		}

		var toolResults []ai.ToolResult
		for _, tc := range resp.ToolCalls {
			result := dispatchTool(tc, &toolState)
			toolResults = append(toolResults, ai.ToolResult{
				ToolCallID: tc.ID,
				Content:    result,
			})
		}

		messages = append(messages, ai.Message{
			Role:    "user",
			Content: formatToolResults(toolResults),
		})
	}

	if toolState.patchPlan != nil {
		return *toolState.patchPlan, nil
	}

	return PatchPlan{
		PackageName:    advisory.Package.Name,
		CurrentVersion: advisory.Package.Version,
		Actions:        []PatchAction{{Type: "noaction", Description: "Planner iteration limit reached — manual review required"}},
		Rationale:      "Max planning iterations exceeded",
		RiskLevel:      "high",
		AutoApply:      false,
	}, nil
}

func (p *Planner) planWithoutTools(ctx context.Context, advisory core.Advisory) (PatchPlan, error) {
	prompt := buildPlannerPrompt(advisory) + `

Since tool calls are not available, produce the patch plan JSON directly:
{
  "actions": [{"type":"upgrade|pin|replace|remove|noaction","description":"...","old_value":"...","new_value":"..."}],
  "rationale": "...",
  "risk_level": "low|medium|high",
  "auto_apply": true|false
}
Respond with ONLY the JSON object.`

	resp, err := p.provider.Complete(ctx, ai.CompletionRequest{
		SystemPrompt: plannerSystemPrompt,
		Messages:     []ai.Message{{Role: "user", Content: prompt}},
		MaxTokens:    maxTokens,
	})
	if err != nil {
		return PatchPlan{}, fmt.Errorf("planner: %w", err)
	}

	return extractPlanFromText(resp.Text, advisory)
}

func formatToolResults(results []ai.ToolResult) string {
	var parts []string
	for _, r := range results {
		parts = append(parts, fmt.Sprintf("[tool_result id=%s] %s", r.ToolCallID, r.Content))
	}
	return strings.Join(parts, "\n")
}

var plannerSystemPrompt = `You are ForgeGuardian's autonomous patch planner. Given a security advisory,
use the available tools to research the vulnerability and produce a concrete patch plan.

Available tools allow you to:
- Look up the latest safe version of a package
- Check if a version is affected by a CVE
- Generate a patch plan JSON

Always call generate_patch_plan as your final action when you have enough information.
Be conservative: if unsure, recommend human review rather than auto-apply.`

func buildTools() []ai.ToolDef {
	return []ai.ToolDef{
		{
			Name:        "lookup_safe_version",
			Description: "Look up the latest version of a package not affected by known vulnerabilities.",
			Parameters: map[string]any{
				"ecosystem":       map[string]string{"type": "string"},
				"package":         map[string]string{"type": "string"},
				"current_version": map[string]string{"type": "string"},
			},
			Required: []string{"ecosystem", "package", "current_version"},
		},
		{
			Name:        "check_version_affected",
			Description: "Check whether a specific version is affected by a given CVE or GHSA ID.",
			Parameters: map[string]any{
				"vuln_id":   map[string]string{"type": "string"},
				"package":   map[string]string{"type": "string"},
				"version":   map[string]string{"type": "string"},
				"ecosystem": map[string]string{"type": "string"},
			},
			Required: []string{"vuln_id", "package", "version", "ecosystem"},
		},
		{
			Name:        "generate_patch_plan",
			Description: "Generate the final structured patch plan. Call when you have enough information.",
			Parameters: map[string]any{
				"actions":    map[string]any{"type": "array"},
				"rationale":  map[string]string{"type": "string"},
				"risk_level": map[string]string{"type": "string"},
				"auto_apply": map[string]string{"type": "boolean"},
			},
			Required: []string{"actions", "rationale", "risk_level"},
		},
	}
}

type plannerState struct {
	advisory  core.Advisory
	patchPlan *PatchPlan
}

func dispatchTool(tc ai.ToolCall, state *plannerState) string {
	var input map[string]any
	json.Unmarshal(tc.Input, &input)

	switch tc.Name {
	case "lookup_safe_version":
		pkg, _ := input["package"].(string)
		ecosystem, _ := input["ecosystem"].(string)
		current, _ := input["current_version"].(string)
		return lookupSafeVersion(ecosystem, pkg, current, state.advisory.Findings)

	case "check_version_affected":
		vulnID, _ := input["vuln_id"].(string)
		pkg, _ := input["package"].(string)
		version, _ := input["version"].(string)
		return checkVersionAffected(vulnID, pkg, version, state.advisory.Findings)

	case "generate_patch_plan":
		actionsRaw, _ := json.Marshal(input["actions"])
		rationale, _ := input["rationale"].(string)
		riskLevel, _ := input["risk_level"].(string)
		autoApply, _ := input["auto_apply"].(bool)

		var actions []PatchAction
		json.Unmarshal(actionsRaw, &actions)

		state.patchPlan = &PatchPlan{
			PackageName:    state.advisory.Package.Name,
			CurrentVersion: state.advisory.Package.Version,
			Actions:        actions,
			Rationale:      rationale,
			RiskLevel:      riskLevel,
			AutoApply:      autoApply,
		}
		return `{"status":"ok","message":"Patch plan recorded"}`

	default:
		return `{"error":"unknown tool"}`
	}
}

func lookupSafeVersion(ecosystem, pkg, current string, findings []core.Finding) string {
	var fixes []string
	for _, f := range findings {
		if fixVersions, ok := f.Metadata["fix_versions"]; ok {
			if versions, ok := fixVersions.([]interface{}); ok {
				for _, v := range versions {
					if vs, ok := v.(string); ok {
						fixes = append(fixes, vs)
					}
				}
			}
			if versions, ok := fixVersions.([]string); ok {
				fixes = append(fixes, versions...)
			}
		}
	}
	if len(fixes) > 0 {
		result := map[string]any{
			"recommended_version": fixes[len(fixes)-1],
			"all_fix_versions":    fixes,
			"source":              "osv_findings",
		}
		b, _ := json.Marshal(result)
		return string(b)
	}
	return fmt.Sprintf(`{"recommended_version":"latest","note":"No specific fix version found in scan data — check %s registry for latest safe version","package":"%s"}`, ecosystem, pkg)
}

func checkVersionAffected(vulnID, pkg, version string, findings []core.Finding) string {
	for _, f := range findings {
		if strings.EqualFold(f.ID, vulnID) {
			return fmt.Sprintf(`{"affected":true,"finding_id":"%s","severity":"%s","fix_available":true}`, f.ID, f.Severity)
		}
	}
	return fmt.Sprintf(`{"affected":false,"note":"Finding %s not found in scan results for %s@%s"}`, vulnID, pkg, version)
}

func extractPlanFromText(text string, advisory core.Advisory) (PatchPlan, error) {
	raw := text
	if idx := strings.Index(raw, "{"); idx >= 0 {
		var plan PatchPlan
		if err := json.Unmarshal([]byte(raw[idx:]), &plan); err == nil {
			plan.PackageName = advisory.Package.Name
			plan.CurrentVersion = advisory.Package.Version
			return plan, nil
		}
	}

	return PatchPlan{
		PackageName:    advisory.Package.Name,
		CurrentVersion: advisory.Package.Version,
		Actions: []PatchAction{{
			Type:        "upgrade",
			Description: advisory.RecommendedAction,
		}},
		Rationale: advisory.Advisory,
		RiskLevel: strings.ToLower(string(advisory.Severity)),
		AutoApply: advisory.Severity == core.SeverityLow || advisory.Severity == core.SeverityInformational,
	}, nil
}

func buildPlannerPrompt(advisory core.Advisory) string {
	return fmt.Sprintf(`Security advisory to remediate:

Package: %s@%s (%s)
Severity: %s
Confidence: %.0f%%
Advisory: %s
Exploitability: %s
Recommended Action: %s

Number of findings: %d

Use the lookup_safe_version and check_version_affected tools to verify the recommended fix,
then call generate_patch_plan with the concrete remediation steps.`,
		advisory.Package.Name,
		advisory.Package.Version,
		advisory.Package.Ecosystem,
		advisory.Severity,
		advisory.Confidence*100,
		advisory.Advisory,
		advisory.ExploitabilityRationale,
		advisory.RecommendedAction,
		len(advisory.Findings),
	)
}
