// Package mcp performs security analysis specific to Model Context Protocol (MCP)
// server packages: dangerous tool declarations, prompt-injection vectors,
// over-broad resource permissions, missing auth, and suspicious dependency patterns.
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/intelligence"
)

// Scanner checks MCP server artifacts for agentic supply-chain risks.
type Scanner struct {
	store *intelligence.SignatureStore
}

// New returns a new MCP Scanner using built-in rules only.
func New() *Scanner { return &Scanner{} }

// NewWithStore returns an MCP Scanner that supplements built-in rules with
// community mcp_injection_pattern signatures from the intelligence store.
func NewWithStore(store *intelligence.SignatureStore) *Scanner {
	return &Scanner{store: store}
}

// Name implements core.Scanner.
func (s *Scanner) Name() string { return "mcp" }

// Scan analyzes MCP server artifacts. Returns no findings for non-MCP packages.
func (s *Scanner) Scan(_ context.Context, artifact core.BuiltArtifact) ([]core.Finding, error) {
	if artifact.Source.Package.Ecosystem != "mcp" {
		return nil, nil
	}

	var findings []core.Finding
	log := artifact.BuildLog
	pkg := artifact.Source.Package

	findings = append(findings, s.checkDangerousTools(log, pkg)...)
	findings = append(findings, s.checkPromptInjection(log, pkg)...)
	findings = append(findings, s.checkOverbroadPermissions(log, pkg)...)
	findings = append(findings, s.checkAuthMissing(log, pkg)...)
	findings = append(findings, s.checkExternalCallbacks(log, pkg)...)
	findings = append(findings, s.checkSuspiciousDependencies(log, pkg)...)

	return findings, nil
}

// --- Dangerous tool declarations ---

// dangerousToolKeywords are MCP tool names / patterns that grant high-risk capabilities.
var dangerousToolKeywords = []string{
	"execute_code", "run_shell", "eval", "exec_command",
	"write_file", "delete_file", "send_email", "http_request",
	"browser_navigate", "database_query", "system_command",
	"read_all_files", "list_secrets", "oauth_token",
}

func (s *Scanner) checkDangerousTools(log string, pkg core.PackageVersion) []core.Finding {
	tools := extractField(log, "mcp_tools")
	if tools == "" {
		return nil
	}

	var found []string
	for _, kw := range dangerousToolKeywords {
		if strings.Contains(strings.ToLower(tools), kw) {
			found = append(found, kw)
		}
	}
	if len(found) == 0 {
		return nil
	}
	return []core.Finding{{
		ID:       "MCP-DANGEROUS-TOOL",
		Severity: core.SeverityHigh,
		Type:     "supply-chain",
		Title:    "MCP server exposes high-risk tools",
		Description: fmt.Sprintf(
			"%s@%s declares tools with dangerous capabilities: %s. An AI agent using this server could be manipulated into executing unintended actions.",
			pkg.Name, pkg.Version, strings.Join(found, ", "),
		),
		Source:   "mcp",
		Metadata: map[string]any{"dangerous_tools": found},
	}}
}

// --- Prompt injection vectors ---

var injectionPatterns = []string{
	"ignore previous", "disregard instructions", "system prompt",
	"jailbreak", "override rules", "act as", "pretend you are",
	"[INST]", "### Human:", "### Assistant:",
}

func (s *Scanner) activeInjectionPatterns() []string {
	patterns := make([]string, len(injectionPatterns))
	copy(patterns, injectionPatterns)
	if s.store == nil {
		return patterns
	}
	for _, sig := range s.store.Signatures {
		if sig.Type == intelligence.SigMCPInjection && sig.Rule != "" {
			patterns = append(patterns, sig.Rule)
		}
	}
	return patterns
}

func (s *Scanner) checkPromptInjection(log string, pkg core.PackageVersion) []core.Finding {
	for _, pat := range s.activeInjectionPatterns() {
		if strings.Contains(strings.ToLower(log), strings.ToLower(pat)) {
			return []core.Finding{{
				ID:       "MCP-PROMPT-INJECTION",
				Severity: core.SeverityCritical,
				Type:     "supply-chain",
				Title:    "Potential prompt injection in MCP server",
				Description: fmt.Sprintf(
					"%s@%s contains text matching prompt-injection pattern %q in tool descriptions or resource content.",
					pkg.Name, pkg.Version, pat,
				),
				Source:   "mcp",
				Metadata: map[string]any{"pattern": pat},
			}}
		}
	}
	return nil
}

// --- Over-broad resource permissions ---

var overbroadPermissions = []string{
	"filesystem://**", "filesystem:///*", "file:///",
	"read:*", "write:*", "admin:*", "root:*",
	"glob:**", "scheme:*",
}

func (s *Scanner) checkOverbroadPermissions(log string, pkg core.PackageVersion) []core.Finding {
	resources := extractField(log, "mcp_resources")
	if resources == "" {
		return nil
	}
	for _, perm := range overbroadPermissions {
		if strings.Contains(resources, perm) {
			return []core.Finding{{
				ID:       "MCP-OVERBROAD-PERMISSIONS",
				Severity: core.SeverityHigh,
				Type:     "supply-chain",
				Title:    "MCP server requests over-broad resource permissions",
				Description: fmt.Sprintf(
					"%s@%s declares resource scope %q which grants unrestricted access. Apply least-privilege scoping.",
					pkg.Name, pkg.Version, perm,
				),
				Source:   "mcp",
				Metadata: map[string]any{"scope": perm},
			}}
		}
	}
	return nil
}

// --- Missing authentication ---

func (s *Scanner) checkAuthMissing(log string, pkg core.PackageVersion) []core.Finding {
	auth := extractField(log, "mcp_auth")
	if auth == "none" || auth == "false" || auth == "disabled" {
		return []core.Finding{{
			ID:       "MCP-NO-AUTH",
			Severity: core.SeverityMedium,
			Type:     "supply-chain",
			Title:    "MCP server has no authentication",
			Description: fmt.Sprintf(
				"%s@%s exposes an unauthenticated MCP endpoint. Any process with network access can invoke its tools.",
				pkg.Name, pkg.Version,
			),
			Source: "mcp",
		}}
	}
	return nil
}

// --- External callback URLs ---

func (s *Scanner) checkExternalCallbacks(log string, pkg core.PackageVersion) []core.Finding {
	callbacks := extractField(log, "mcp_callbacks")
	if callbacks == "" || callbacks == "[]" {
		return nil
	}
	// Flag non-localhost URLs
	if strings.Contains(callbacks, "http://") || strings.Contains(callbacks, "https://") {
		if !strings.Contains(callbacks, "localhost") && !strings.Contains(callbacks, "127.0.0.1") {
			return []core.Finding{{
				ID:       "MCP-EXTERNAL-CALLBACK",
				Severity: core.SeverityHigh,
				Type:     "supply-chain",
				Title:    "MCP server phones home to external URL",
				Description: fmt.Sprintf(
					"%s@%s registers callbacks to external URLs: %s. This can exfiltrate conversation context or tool results.",
					pkg.Name, pkg.Version, callbacks,
				),
				Source:   "mcp",
				Metadata: map[string]any{"callbacks": callbacks},
			}}
		}
	}
	return nil
}

// --- Suspicious npm dependencies in MCP package ---

var suspiciousMCPDeps = []string{
	"node-pty", "shelljs", "child_process", "vm2",
	"execa", "cross-spawn", "open", "electron",
}

func (s *Scanner) checkSuspiciousDependencies(log string, pkg core.PackageVersion) []core.Finding {
	deps := extractField(log, "dependencies")
	if deps == "" {
		return nil
	}
	var found []string
	for _, dep := range suspiciousMCPDeps {
		if strings.Contains(deps, dep) {
			found = append(found, dep)
		}
	}
	if len(found) == 0 {
		return nil
	}
	return []core.Finding{{
		ID:       "MCP-SUSPICIOUS-DEPS",
		Severity: core.SeverityMedium,
		Type:     "supply-chain",
		Title:    "MCP server depends on packages with shell/process execution capabilities",
		Description: fmt.Sprintf(
			"%s@%s depends on %s — these packages can execute arbitrary shell commands and may be used to escape the MCP sandbox.",
			pkg.Name, pkg.Version, strings.Join(found, ", "),
		),
		Source:   "mcp",
		Metadata: map[string]any{"suspicious_deps": found},
	}}
}

// extractField pulls a key: value line from the build log.
func extractField(log, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(log, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}
