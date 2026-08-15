// Package agent runs a Claude tool-use loop that converts raw threat intelligence
// findings into typed DetectionSignature entries for ForgeGuardian's signature store.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/mah3sec/forgeguardian/internal/intelligence"
)

const (
	model     anthropic.Model = "claude-sonnet-4-20250514"
	maxTokens = 4096
	maxIter   = 20
)

const systemPrompt = `You are ForgeGuardian's intelligence agent — a supply chain security specialist.

You receive raw threat intelligence findings (new CVEs, malicious packages, behavioral patterns) and
use the provided tools to convert them into typed detection signatures.

For each threat, decide which signature type fits best and call the appropriate tool:
- add_typosquat_target: when a legitimate popular package name should be monitored for typosquats
- add_malware_pattern: for hex byte patterns that identify malicious payloads
- add_blocklisted_package: for packages confirmed malicious or actively exploited
- add_behavioral_rule: for regex patterns matching malicious install scripts or code patterns
- add_mcp_injection_pattern: for prompt injection / tool shadowing patterns in MCP servers
- add_pickle_rule: for dangerous Python pickle opcode sequences in AI model weights

Rules:
- Only generate signatures from the threat data provided. Do not invent threats.
- Set severity based on real-world exploitability: critical = active exploitation/RCE, high = likely exploited/backdoor.
- For blocklisted_package, include the CVE ID when available.
- Hex patterns for add_malware_pattern must be valid hex strings (e.g., "7f454c46" for ELF).
- Call multiple tools if the data warrants multiple signatures.
- When done, stop calling tools.`

// Agent generates DetectionSignatures from raw threat intelligence using Claude.
type Agent struct {
	client *anthropic.Client
}

// New creates a new intelligence Agent.
func New(apiKey string) *Agent {
	var opts []option.RequestOption
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	c := anthropic.NewClient(opts...)
	return &Agent{client: &c}
}

// GenerateSignatures runs a Claude tool-use loop against the provided threat context
// and returns all DetectionSignature entries produced by tool calls.
func (a *Agent) GenerateSignatures(ctx context.Context, threatContext string) ([]intelligence.DetectionSignature, error) {
	tools := buildTools()
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(threatContext)),
	}

	var collected []intelligence.DetectionSignature

	for i := 0; i < maxIter; i++ {
		msg, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     model,
			MaxTokens: maxTokens,
			System: []anthropic.TextBlockParam{
				{Text: systemPrompt},
			},
			Tools:    tools,
			Messages: messages,
			ToolChoice: anthropic.ToolChoiceUnionParam{
				OfAuto: &anthropic.ToolChoiceAutoParam{},
			},
		})
		if err != nil {
			return collected, fmt.Errorf("agent: api call %d: %w", i, err)
		}

		messages = append(messages, anthropic.NewAssistantMessage(contentToParams(msg.Content)...))

		if msg.StopReason == anthropic.StopReasonEndTurn {
			break
		}

		if msg.StopReason != anthropic.StopReasonToolUse {
			break
		}

		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range msg.Content {
			tu := block.AsToolUse()
			if tu.ID == "" {
				continue
			}
			sig, result := dispatchToolUse(tu)
			if sig != nil {
				collected = append(collected, *sig)
			}
			toolResults = append(toolResults, anthropic.NewToolResultBlock(tu.ID, result, false))
		}
		if len(toolResults) > 0 {
			messages = append(messages, anthropic.NewUserMessage(toolResults...))
		}
	}

	return collected, nil
}

// --- Tool definitions ---

func buildTools() []anthropic.ToolUnionParam {
	mkTool := func(name, desc string, props map[string]any, required []string) anthropic.ToolUnionParam {
		t := anthropic.ToolUnionParamOfTool(
			anthropic.ToolInputSchemaParam{Properties: props, Required: required},
			name,
		)
		t.OfTool.Description = anthropic.String(desc)
		return t
	}

	return []anthropic.ToolUnionParam{
		mkTool("add_typosquat_target",
			"Register a popular package as a typosquatting target to monitor for look-alike package names.",
			map[string]any{
				"ecosystem":   map[string]string{"type": "string", "description": "Package ecosystem: npm|pypi|go|rubygems|crates|maven"},
				"name":        map[string]string{"type": "string", "description": "The canonical package name to protect"},
				"severity":    map[string]string{"type": "string", "description": "Severity if a typosquat is detected: high or critical"},
				"description": map[string]string{"type": "string", "description": "Why this package is a high-value target"},
			},
			[]string{"ecosystem", "name", "severity", "description"},
		),
		mkTool("add_malware_pattern",
			"Add a hex-encoded byte pattern to detect malicious payloads embedded in packages.",
			map[string]any{
				"ecosystem":   map[string]string{"type": "string", "description": "Ecosystem or * for all"},
				"hex_pattern": map[string]string{"type": "string", "description": "Hex-encoded bytes, no spaces (e.g. 7f454c46 for ELF magic)"},
				"title":       map[string]string{"type": "string", "description": "Short human-readable name for this pattern"},
				"severity":    map[string]string{"type": "string", "description": "Severity level: medium, high, or critical"},
				"cve":         map[string]string{"type": "string", "description": "Associated CVE ID if applicable"},
			},
			[]string{"ecosystem", "hex_pattern", "title", "severity"},
		),
		mkTool("add_blocklisted_package",
			"Blocklist a specific package that is confirmed malicious, backdoored, or actively exploited.",
			map[string]any{
				"ecosystem":   map[string]string{"type": "string", "description": "Package ecosystem"},
				"package":     map[string]string{"type": "string", "description": "Exact package name"},
				"severity":    map[string]string{"type": "string", "description": "Severity level: high or critical"},
				"description": map[string]string{"type": "string", "description": "Why this package is dangerous"},
				"cve":         map[string]string{"type": "string", "description": "Associated CVE or MAL ID"},
			},
			[]string{"ecosystem", "package", "severity", "description"},
		),
		mkTool("add_behavioral_rule",
			"Add a regex rule to detect malicious patterns in install scripts, lifecycle hooks, or package metadata.",
			map[string]any{
				"ecosystem":   map[string]string{"type": "string", "description": "Ecosystem or * for all"},
				"regex":       map[string]string{"type": "string", "description": "Go-compatible regex pattern"},
				"title":       map[string]string{"type": "string", "description": "Short name for this rule"},
				"severity":    map[string]string{"type": "string", "description": "Severity level: medium, high, or critical"},
				"description": map[string]string{"type": "string", "description": "What malicious behavior this detects"},
			},
			[]string{"ecosystem", "regex", "title", "severity", "description"},
		),
		mkTool("add_mcp_injection_pattern",
			"Add a pattern to detect prompt injection or tool shadowing in MCP server packages.",
			map[string]any{
				"pattern":     map[string]string{"type": "string", "description": "Regex or substring pattern to match in MCP tool descriptions/schemas"},
				"severity":    map[string]string{"type": "string", "description": "Severity level: high or critical"},
				"description": map[string]string{"type": "string", "description": "What injection technique this detects"},
			},
			[]string{"pattern", "severity", "description"},
		),
		mkTool("add_pickle_rule",
			"Add a rule to detect dangerous Python pickle operations in AI model weight files.",
			map[string]any{
				"rule":        map[string]string{"type": "string", "description": "Regex pattern matching dangerous pickle opcode sequences or global references"},
				"severity":    map[string]string{"type": "string", "description": "Severity level: medium, high, or critical"},
				"description": map[string]string{"type": "string", "description": "What dangerous pickle operation this detects"},
			},
			[]string{"rule", "severity", "description"},
		),
	}
}

// --- Tool dispatcher ---

func dispatchToolUse(tu anthropic.ToolUseBlock) (*intelligence.DetectionSignature, string) {
	var args map[string]string
	if err := json.Unmarshal([]byte(tu.Input), &args); err != nil {
		return nil, fmt.Sprintf("error parsing args: %v", err)
	}

	id := fmt.Sprintf("FG-SIG-%d", time.Now().UnixNano())
	now := time.Now().UTC()

	switch tu.Name {
	case "add_typosquat_target":
		return &intelligence.DetectionSignature{
			ID:          id,
			Type:        intelligence.SigTypoTarget,
			Ecosystem:   args["ecosystem"],
			Target:      args["name"],
			Severity:    args["severity"],
			Title:       fmt.Sprintf("Typosquat target: %s/%s", args["ecosystem"], args["name"]),
			Description: args["description"],
			Source:      "ai-generated",
			CreatedAt:   now,
		}, "added typosquat target"

	case "add_malware_pattern":
		return &intelligence.DetectionSignature{
			ID:          id,
			Type:        intelligence.SigMalwarePattern,
			Ecosystem:   args["ecosystem"],
			Pattern:     strings.ToLower(args["hex_pattern"]),
			Severity:    args["severity"],
			Title:       args["title"],
			Description: fmt.Sprintf("Malware byte pattern: %s", args["hex_pattern"]),
			Source:      "ai-generated",
			CVE:         args["cve"],
			CreatedAt:   now,
		}, "added malware pattern"

	case "add_blocklisted_package":
		return &intelligence.DetectionSignature{
			ID:          id,
			Type:        intelligence.SigBlocklisted,
			Ecosystem:   args["ecosystem"],
			Package:     args["package"],
			Severity:    args["severity"],
			Title:       fmt.Sprintf("Blocklisted: %s/%s", args["ecosystem"], args["package"]),
			Description: args["description"],
			Source:      "ai-generated",
			CVE:         args["cve"],
			CreatedAt:   now,
		}, "added blocklisted package"

	case "add_behavioral_rule":
		return &intelligence.DetectionSignature{
			ID:          id,
			Type:        intelligence.SigBehavioral,
			Ecosystem:   args["ecosystem"],
			Rule:        args["regex"],
			Severity:    args["severity"],
			Title:       args["title"],
			Description: args["description"],
			Source:      "ai-generated",
			CreatedAt:   now,
		}, "added behavioral rule"

	case "add_mcp_injection_pattern":
		return &intelligence.DetectionSignature{
			ID:          id,
			Type:        intelligence.SigMCPInjection,
			Ecosystem:   "mcp",
			Rule:        args["pattern"],
			Severity:    args["severity"],
			Title:       "MCP prompt injection pattern",
			Description: args["description"],
			Source:      "ai-generated",
			CreatedAt:   now,
		}, "added MCP injection pattern"

	case "add_pickle_rule":
		return &intelligence.DetectionSignature{
			ID:          id,
			Type:        intelligence.SigPickleRule,
			Ecosystem:   "huggingface",
			Rule:        args["rule"],
			Severity:    args["severity"],
			Title:       "Pickle rule",
			Description: args["description"],
			Source:      "ai-generated",
			CreatedAt:   now,
		}, "added pickle rule"
	}

	return nil, fmt.Sprintf("unknown tool: %s", tu.Name)
}

// contentToParams converts API response content blocks to MessageParam format.
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

// BuildThreatContext formats raw threat feed data into a prompt for Claude.
func BuildThreatContext(osvFindings []intelligence.DetectionSignature, ossffFindings []intelligence.DetectionSignature) string {
	var sb strings.Builder
	sb.WriteString("## New Threat Intelligence Findings\n\n")
	sb.WriteString("Convert the following findings into detection signatures using the available tools.\n\n")

	if len(osvFindings) > 0 {
		sb.WriteString("### OSV Vulnerability Feed\n")
		for _, f := range osvFindings {
			fmt.Fprintf(&sb, "- Package: %s/%s | CVE: %s | Severity: %s | %s\n",
				f.Ecosystem, f.Package, f.CVE, f.Severity, f.Title)
		}
		sb.WriteString("\n")
	}

	if len(ossffFindings) > 0 {
		sb.WriteString("### OpenSSF Malicious Packages Feed\n")
		for _, f := range ossffFindings {
			fmt.Fprintf(&sb, "- Package: %s/%s | Source: %s | %s\n",
				f.Ecosystem, f.Package, f.Source, f.Title)
			if f.Description != "" {
				fmt.Fprintf(&sb, "  Details: %s\n", truncate(f.Description, 200))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Generate detection signatures for any findings that represent novel or high-confidence threats. ")
	sb.WriteString("Focus on packages not already in common blocklists. Use add_blocklisted_package for confirmed malicious packages.")
	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
