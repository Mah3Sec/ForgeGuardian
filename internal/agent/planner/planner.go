// Package planner uses Claude's tool-use capability to autonomously reason about
// and plan patch actions for a given security advisory.
// It runs a multi-turn agentic loop: Claude calls tools → we execute → Claude reasons → repeat.
package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/mah3sec/forgeguardian/internal/core"
)

const (
	model     = anthropic.ModelClaudeSonnet4_20250514
	maxTokens = 4096
	maxIter   = 10 // safety limit on tool-use iterations
)

// PatchPlan describes the actions the planner decided to take.
type PatchPlan struct {
	PackageName    string       `json:"package_name"`
	CurrentVersion string       `json:"current_version"`
	Actions        []PatchAction `json:"actions"`
	Rationale      string       `json:"rationale"`
	RiskLevel      string       `json:"risk_level"` // low|medium|high
	AutoApply      bool         `json:"auto_apply"`  // true if safe to apply without human review
}

// PatchAction is a single concrete remediation step.
type PatchAction struct {
	Type        string `json:"type"`        // upgrade|pin|replace|remove|noaction
	Description string `json:"description"`
	OldValue    string `json:"old_value,omitempty"`
	NewValue    string `json:"new_value,omitempty"`
	FilePath    string `json:"file_path,omitempty"`
}

// Planner runs an agentic planning loop using Claude tool use.
type Planner struct {
	client *anthropic.Client
}

// New creates a new patch Planner.
func New(apiKey string) *Planner {
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	c := anthropic.NewClient(opts...)
	return &Planner{client: &c}
}

// Plan generates a PatchPlan for the given advisory using a multi-turn tool-use loop.
func (p *Planner) Plan(ctx context.Context, advisory core.Advisory) (PatchPlan, error) {
	tools := buildTools()
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(buildPlannerPrompt(advisory))),
	}

	var toolState plannerState
	toolState.advisory = advisory

	for i := 0; i < maxIter; i++ {
		msg, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     model,
			MaxTokens: maxTokens,
			System: []anthropic.TextBlockParam{
				{Text: plannerSystemPrompt},
			},
			Tools:    tools,
			Messages: messages,
			ToolChoice: anthropic.ToolChoiceUnionParam{
				OfAuto: &anthropic.ToolChoiceAutoParam{},
			},
		})
		if err != nil {
			return PatchPlan{}, fmt.Errorf("planner: api call %d: %w", i, err)
		}

		// Append assistant message to history
		messages = append(messages, anthropic.NewAssistantMessage(contentToParams(msg.Content)...))

		// If Claude is done (end_turn or tool_use done), extract the plan
		if msg.StopReason == anthropic.StopReasonEndTurn {
			return extractPlan(msg.Content, advisory)
		}

		if msg.StopReason != anthropic.StopReasonToolUse {
			break
		}

		// Process tool calls
		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range msg.Content {
			tu := block.AsToolUse()
			if tu.ID == "" {
				continue
			}
			result := dispatchTool(tu, &toolState)
			toolResults = append(toolResults, anthropic.NewToolResultBlock(tu.ID, result, false))
		}
		if len(toolResults) > 0 {
			messages = append(messages, anthropic.NewUserMessage(toolResults...))
		}
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

// --- Tool definitions ---

var plannerSystemPrompt = `You are ForgeGuardian's autonomous patch planner. Given a security advisory,
use the available tools to research the vulnerability and produce a concrete patch plan.

Available tools allow you to:
- Look up the latest safe version of a package
- Check if a version is affected by a CVE
- Generate a patch plan JSON

Always call generate_patch_plan as your final action when you have enough information.
Be conservative: if unsure, recommend human review rather than auto-apply.`

func buildTools() []anthropic.ToolUnionParam {
	t1 := anthropic.ToolUnionParamOfTool(
		anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"ecosystem":       map[string]string{"type": "string"},
				"package":         map[string]string{"type": "string"},
				"current_version": map[string]string{"type": "string"},
			},
			Required: []string{"ecosystem", "package", "current_version"},
		},
		"lookup_safe_version",
	)
	t1.OfTool.Description = anthropic.String("Look up the latest version of a package not affected by known vulnerabilities.")

	t2 := anthropic.ToolUnionParamOfTool(
		anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"vuln_id":   map[string]string{"type": "string"},
				"package":   map[string]string{"type": "string"},
				"version":   map[string]string{"type": "string"},
				"ecosystem": map[string]string{"type": "string"},
			},
			Required: []string{"vuln_id", "package", "version", "ecosystem"},
		},
		"check_version_affected",
	)
	t2.OfTool.Description = anthropic.String("Check whether a specific version is affected by a given CVE or GHSA ID.")

	t3 := anthropic.ToolUnionParamOfTool(
		anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"actions":    map[string]any{"type": "array"},
				"rationale":  map[string]string{"type": "string"},
				"risk_level": map[string]string{"type": "string"},
				"auto_apply": map[string]string{"type": "boolean"},
			},
			Required: []string{"actions", "rationale", "risk_level"},
		},
		"generate_patch_plan",
	)
	t3.OfTool.Description = anthropic.String("Generate the final structured patch plan. Call when you have enough information.")

	return []anthropic.ToolUnionParam{t1, t2, t3}
}

// plannerState holds the tool state across iterations.
type plannerState struct {
	advisory   core.Advisory
	patchPlan  *PatchPlan
}

// dispatchTool executes a tool call and returns the result string.
func dispatchTool(tu anthropic.ToolUseBlock, state *plannerState) string {
	var input map[string]any
	json.Unmarshal(tu.Input, &input)

	switch tu.Name {
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

// lookupSafeVersion uses OSV fix versions from the findings to suggest a safe version.
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

// checkVersionAffected looks through findings for the specified vuln ID.
func checkVersionAffected(vulnID, pkg, version string, findings []core.Finding) string {
	for _, f := range findings {
		if strings.EqualFold(f.ID, vulnID) {
			return fmt.Sprintf(`{"affected":true,"finding_id":"%s","severity":"%s","fix_available":true}`, f.ID, f.Severity)
		}
	}
	return fmt.Sprintf(`{"affected":false,"note":"Finding %s not found in scan results for %s@%s"}`, vulnID, pkg, version)
}

// extractPlan pulls the PatchPlan from the tool state or generates one from the final message.
func extractPlan(content []anthropic.ContentBlockUnion, advisory core.Advisory) (PatchPlan, error) {
	// Collect text from the final assistant message
	var text strings.Builder
	for _, block := range content {
		if tb := block.AsText(); tb.Text != "" {
			text.WriteString(tb.Text)
		}
	}

	// Try to parse a JSON plan from the final text
	raw := text.String()
	if idx := strings.Index(raw, "{"); idx >= 0 {
		var plan PatchPlan
		if err := json.Unmarshal([]byte(raw[idx:]), &plan); err == nil {
			plan.PackageName = advisory.Package.Name
			plan.CurrentVersion = advisory.Package.Version
			return plan, nil
		}
	}

	// Best-effort plan from advisory data
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

// contentToParams converts response content blocks to MessageParam content.
func contentToParams(blocks []anthropic.ContentBlockUnion) []anthropic.ContentBlockParamUnion {
	var params []anthropic.ContentBlockParamUnion
	for _, b := range blocks {
		switch {
		case b.AsText().Text != "":
			params = append(params, anthropic.NewTextBlock(b.AsText().Text))
		case b.AsToolUse().ID != "":
			tu := b.AsToolUse()
			params = append(params, anthropic.NewToolUseBlock(tu.ID, tu.Input, tu.Name))
		}
	}
	return params
}
