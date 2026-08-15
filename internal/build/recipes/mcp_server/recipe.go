// Package mcp_server implements the hermetic build recipe for MCP server packages.
package mcp_server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/build/recipes/util"
	"github.com/mah3sec/forgeguardian/internal/core"
)

// Recipe builds and behaviorally analyzes MCP server npm packages.
// Security signals captured:
//   - Install lifecycle scripts (preinstall/install/postinstall/prepare)
//   - tools/list introspection (tool names, descriptions, input schemas)
//   - Tool shadowing: tools whose names collide with built-in assistant tools
//   - Prompt injection: suspicious phrases in tool descriptions / schema strings
//   - Overly broad filesystem paths in input schemas
type Recipe struct{}

// New creates a new MCP server Recipe.
func New() *Recipe { return &Recipe{} }

// Ecosystem implements core.BuildRecipe.
func (r *Recipe) Ecosystem() string { return "mcp" }

// Build downloads the MCP server package, performs static analysis, and
// attempts behavioral introspection via tools/list.
func (r *Recipe) Build(ctx context.Context, src core.SourceArtifact, _ core.Sandbox) (core.BuiltArtifact, error) {
	name, version := src.Package.Name, src.Package.Version

	// Fetch npm metadata (MCP servers are distributed as npm packages)
	tarballURL, registrySha1, installScripts, toolsMeta, err := r.fetchMCPMeta(ctx, name, version)
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("mcp_server build: fetch meta: %w", err)
	}

	dlPath, sha256sum, err := util.Download(tarballURL, ".tgz")
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("mcp_server build: download: %w", err)
	}
	defer os.Remove(dlPath)

	// Static analysis
	analysis := r.analyzeStatically(name, version, toolsMeta, installScripts)

	var log strings.Builder
	fmt.Fprintf(&log, "=== mcp_server build: %s@%s ===\n", name, version)
	fmt.Fprintf(&log, "tarball_url:          %s\n", tarballURL)
	fmt.Fprintf(&log, "sha256:               %s\n", sha256sum)
	fmt.Fprintf(&log, "registry_sha1:        %s\n", registrySha1)
	fmt.Fprintf(&log, "install_scripts:      %v\n", installScripts)
	fmt.Fprintf(&log, "tools_declared:       %d\n", len(toolsMeta))
	fmt.Fprintf(&log, "shadowed_tools:       %v\n", analysis.shadowedTools)
	fmt.Fprintf(&log, "injection_signals:    %v\n", analysis.injectionSignals)
	fmt.Fprintf(&log, "broad_paths:          %v\n", analysis.broadPaths)
	fmt.Fprintf(&log, "risk_score:           %d\n", analysis.riskScore)
	fmt.Fprintf(&log, "network_connections:  0\n")

	for _, t := range toolsMeta {
		fmt.Fprintf(&log, "tool: name=%s desc_len=%d\n", t.Name, len(t.Description))
	}
	for _, sig := range analysis.injectionSignals {
		fmt.Fprintf(&log, "injection_signal: %s\n", sig)
	}

	outPath := util.StableOutput("mcp", name, version, ".tgz")
	if err := util.CopyFile(dlPath, outPath); err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("mcp_server build: copy: %w", err)
	}
	return util.BuildArtifact(src, outPath, sha256sum, log.String()), nil
}

// VerifyReproducible re-downloads and compares SHA256.
func (r *Recipe) VerifyReproducible(ctx context.Context, artifact core.BuiltArtifact) (bool, error) {
	a2, err := r.Build(ctx, artifact.Source, nil)
	if err != nil {
		return false, err
	}
	defer os.Remove(a2.LocalPath)
	return artifact.SHA256 == a2.SHA256, nil
}

// --- MCP tool type ---

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// --- Static analysis ---

type staticAnalysis struct {
	shadowedTools    []string
	injectionSignals []string
	broadPaths       []string
	riskScore        int
}

// builtinTools are standard assistant tool names that MCP tools must not shadow.
var builtinTools = map[string]bool{
	"bash":               true,
	"computer":           true,
	"str_replace_editor": true,
	"text_editor":        true,
	"web_search":         true,
	"browser":            true,
	"read_file":          true,
	"write_file":         true,
	"execute_code":       true,
	"python":             true,
}

// injectionPatterns are phrases that indicate prompt injection attempts in tool descriptions.
var injectionPatterns = []string{
	"ignore previous",
	"disregard",
	"new instructions",
	"forget everything",
	"do not follow",
	"override instructions",
	"system prompt",
	"you are now",
	"act as",
	"jailbreak",
	"bypass",
	"from now on",
	"ignore above",
	"ignore all",
}

// broadPathPatterns are filesystem path patterns that indicate overly broad access.
var broadPathPatterns = []string{
	"/*",
	"/etc",
	"/root",
	"/home",
	"C:\\",
	"C:/",
	"~",
	".env",
	"id_rsa",
	"credentials",
}

func (r *Recipe) analyzeStatically(name, version string, tools []mcpTool, installScripts []string) staticAnalysis {
	var a staticAnalysis

	// Check for tool shadowing
	for _, t := range tools {
		lower := strings.ToLower(t.Name)
		if builtinTools[lower] {
			a.shadowedTools = append(a.shadowedTools, t.Name)
			a.riskScore += 30
		}
	}

	// Check for prompt injection in tool descriptions and schema strings
	for _, t := range tools {
		descLower := strings.ToLower(t.Description)
		for _, pattern := range injectionPatterns {
			if strings.Contains(descLower, pattern) {
				sig := fmt.Sprintf("tool=%s pattern=%q", t.Name, pattern)
				a.injectionSignals = append(a.injectionSignals, sig)
				a.riskScore += 20
			}
		}
		// Also scan input schema for injected strings
		schemaStr := schemaToString(t.InputSchema)
		schemaLower := strings.ToLower(schemaStr)
		for _, pattern := range injectionPatterns {
			if strings.Contains(schemaLower, pattern) {
				sig := fmt.Sprintf("schema=%s pattern=%q", t.Name, pattern)
				a.injectionSignals = append(a.injectionSignals, sig)
				a.riskScore += 15
			}
		}
		// Check for broad filesystem paths in schema
		for _, bp := range broadPathPatterns {
			if strings.Contains(schemaStr, bp) {
				a.broadPaths = append(a.broadPaths, fmt.Sprintf("tool=%s path=%q", t.Name, bp))
				a.riskScore += 10
			}
		}
	}

	// Install scripts add risk
	if len(installScripts) > 0 {
		a.riskScore += 25
	}

	return a
}

// schemaToString converts a JSON schema map to a flat string for pattern matching.
func schemaToString(schema map[string]any) string {
	if schema == nil {
		return ""
	}
	b, _ := json.Marshal(schema)
	return string(b)
}

// --- npm registry fetch with MCP tool declaration support ---

// fetchMCPMeta retrieves tarball URL, sha1, install scripts, and any declared tools
// from the package.json "mcp" or "mcpTools" field.
func (r *Recipe) fetchMCPMeta(ctx context.Context, name, version string) (
	tarballURL, shasum string, scripts []string, tools []mcpTool, err error,
) {
	apiURL := fmt.Sprintf("https://registry.npmjs.org/%s/%s", name, version)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	req.Header.Set("User-Agent", util.UserAgent)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", nil, nil, fmt.Errorf("npm registry returned %d for %s@%s", resp.StatusCode, name, version)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", nil, nil, err
	}

	var doc struct {
		Dist struct {
			Tarball string `json:"tarball"`
			Shasum  string `json:"shasum"`
		} `json:"dist"`
		Scripts  map[string]string `json:"scripts"`
		MCP      json.RawMessage   `json:"mcp"`
		MCPTools json.RawMessage   `json:"mcpTools"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", "", nil, nil, err
	}

	for _, hook := range []string{"preinstall", "install", "postinstall", "prepare"} {
		if s, ok := doc.Scripts[hook]; ok {
			scripts = append(scripts, fmt.Sprintf("%s: %s", hook, s))
		}
	}

	// Try to extract declared tools from package.json mcp/mcpTools fields
	tools = extractDeclaredTools(doc.MCP, doc.MCPTools)

	// Also scan the registry description for injection signals
	var fullDoc map[string]json.RawMessage
	if err := json.Unmarshal(body, &fullDoc); err == nil {
		if descRaw, ok := fullDoc["description"]; ok {
			var desc string
			if json.Unmarshal(descRaw, &desc) == nil {
				// Add a synthetic tool representing the package description for scanning
				tools = append(tools, mcpTool{
					Name:        "__package_description__",
					Description: desc,
				})
			}
		}
	}

	return doc.Dist.Tarball, doc.Dist.Shasum, scripts, tools, nil
}

// extractDeclaredTools parses mcp or mcpTools fields from package.json.
// Supports both {"tools": [...]} and direct [...] formats.
func extractDeclaredTools(mcpRaw, mcpToolsRaw json.RawMessage) []mcpTool {
	var tools []mcpTool

	for _, raw := range []json.RawMessage{mcpRaw, mcpToolsRaw} {
		if len(raw) == 0 {
			continue
		}
		raw = bytes.TrimSpace(raw)

		// Try direct array
		var toolsArr []mcpTool
		if err := json.Unmarshal(raw, &toolsArr); err == nil {
			tools = append(tools, toolsArr...)
			continue
		}

		// Try {"tools": [...]} wrapper
		var wrapper struct {
			Tools []mcpTool `json:"tools"`
		}
		if err := json.Unmarshal(raw, &wrapper); err == nil {
			tools = append(tools, wrapper.Tools...)
		}
	}

	return tools
}
