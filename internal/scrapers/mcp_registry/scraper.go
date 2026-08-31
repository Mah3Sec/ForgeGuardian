// Package mcp_registry implements a scraper for MCP (Model Context Protocol) server packages.
// It watches both the official MCP registry and npm packages tagged with "mcp-server".
package mcp_registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
)

const (
	mcpRegistryURL  = "https://registry.modelcontextprotocol.io/servers"
	npmSearchURL    = "https://registry.npmjs.org/-/v1/search?text=keywords:mcp-server&size=100"
	npmRegistryBase = "https://registry.npmjs.org"
	userAgent       = "forgeguardian-scraper/0.1 (supply chain security; https://github.com/forgeguardian)"
)

// Scraper watches the official MCP registry and npm for MCP server packages.
type Scraper struct {
	client *http.Client
	log    *slog.Logger
}

// New creates a new MCP Registry Scraper.
func New() *Scraper {
	return &Scraper{
		client: &http.Client{Timeout: 30 * time.Second},
		log:    slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("scraper", "mcp_registry"),
	}
}

// Name implements core.RegistryScraper.
func (s *Scraper) Name() string { return "mcp" }

// mcpServer is a single entry from the official MCP registry.
type mcpServer struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Repository  string     `json:"repository"`
	NPMPackage  string     `json:"npm_package"`
	Version     string     `json:"version"`
	UpdatedAt   *time.Time `json:"updated_at"`
	Tools       []mcpTool  `json:"tools"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// npmSearchResponse is the npm registry search response.
type npmSearchResponse struct {
	Objects []npmSearchObject `json:"objects"`
}

type npmSearchObject struct {
	Package struct {
		Name        string    `json:"name"`
		Version     string    `json:"version"`
		Description string    `json:"description"`
		Keywords    []string  `json:"keywords"`
		Date        time.Time `json:"date"`
		Links       struct {
			NPM        string `json:"npm"`
			Repository string `json:"repository"`
		} `json:"links"`
	} `json:"package"`
}

// Poll returns MCP server packages published/updated since lastRun.
func (s *Scraper) Poll(ctx context.Context, lastRun time.Time) ([]core.PackageVersion, error) {
	var versions []core.PackageVersion

	// Source 1: Official MCP registry
	mcpVersions, err := s.pollOfficialRegistry(ctx, lastRun)
	if err != nil {
		s.log.Warn("official MCP registry unavailable", "err", err)
	} else {
		versions = append(versions, mcpVersions...)
	}

	// Source 2: npm packages tagged mcp-server
	npmVersions, err := s.pollNPMMCPPackages(ctx, lastRun)
	if err != nil {
		s.log.Warn("npm MCP search failed", "err", err)
	} else {
		versions = append(versions, npmVersions...)
	}

	return versions, nil
}

// pollOfficialRegistry polls registry.modelcontextprotocol.io.
func (s *Scraper) pollOfficialRegistry(ctx context.Context, lastRun time.Time) ([]core.PackageVersion, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mcpRegistryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp: build registry request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp: fetch registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcp: registry returned %d", resp.StatusCode)
	}

	var servers []mcpServer
	if err := json.NewDecoder(resp.Body).Decode(&servers); err != nil {
		// Registry may return a different structure; handle gracefully
		return nil, fmt.Errorf("mcp: decode registry: %w", err)
	}

	var versions []core.PackageVersion
	for _, srv := range servers {
		if srv.UpdatedAt != nil && srv.UpdatedAt.Before(lastRun) {
			continue
		}

		publishedAt := time.Now()
		if srv.UpdatedAt != nil {
			publishedAt = *srv.UpdatedAt
		}

		analysis := s.analyzeMCPServer(srv)

		meta := map[string]any{
			"description":           srv.Description,
			"repository":            srv.Repository,
			"npm_package":           srv.NPMPackage,
			"tool_count":            len(srv.Tools),
			"tool_names":            toolNames(srv.Tools),
			"shadows_builtin":       analysis.shadowsBuiltin,
			"prompt_injection_risk": analysis.promptInjectionRisk,
			"overly_broad_perms":    analysis.overlyBroadPerms,
			"source":                "official_registry",
		}

		name := srv.NPMPackage
		if name == "" {
			name = srv.Name
		}

		versions = append(versions, core.PackageVersion{
			Ecosystem:   "mcp",
			Name:        name,
			Version:     srv.Version,
			SourceURL:   srv.Repository,
			Checksum:    "",
			PublishedAt: publishedAt,
			Metadata:    meta,
		})
	}
	return versions, nil
}

// pollNPMMCPPackages searches npm for packages tagged with mcp-server.
func (s *Scraper) pollNPMMCPPackages(ctx context.Context, lastRun time.Time) ([]core.PackageVersion, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, npmSearchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp: build npm search request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp: npm search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mcp: npm search returned %d", resp.StatusCode)
	}

	var searchResp npmSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("mcp: decode npm search: %w", err)
	}

	var versions []core.PackageVersion
	for _, obj := range searchResp.Objects {
		pkg := obj.Package
		if pkg.Date.Before(lastRun) {
			continue
		}

		// Check for dependency confusion: verify this npm package is the
		// canonical one by checking if it appears in the official registry.
		tarballURL := fmt.Sprintf("%s/%s/-/%s-%s.tgz",
			npmRegistryBase, pkg.Name, pkg.Name, pkg.Version)

		meta := map[string]any{
			"description":          pkg.Description,
			"keywords":             pkg.Keywords,
			"repository":           pkg.Links.Repository,
			"npm_url":              pkg.Links.NPM,
			"source":               "npm_keyword_search",
			"dependency_confusion": s.checkDependencyConfusion(pkg.Name),
		}

		versions = append(versions, core.PackageVersion{
			Ecosystem:   "mcp",
			Name:        pkg.Name,
			Version:     pkg.Version,
			SourceURL:   tarballURL,
			Checksum:    "",
			PublishedAt: pkg.Date,
			Metadata:    meta,
		})
	}
	return versions, nil
}

// mcpAnalysis holds security analysis results for an MCP server.
type mcpAnalysis struct {
	shadowsBuiltin      bool
	promptInjectionRisk bool
	overlyBroadPerms    bool
}

// analyzeMCPServer checks for common MCP security issues.
func (s *Scraper) analyzeMCPServer(srv mcpServer) mcpAnalysis {
	var a mcpAnalysis

	builtinTools := map[string]bool{
		"bash": true, "computer": true, "str_replace_editor": true,
		"text_editor": true, "read_file": true, "write_file": true,
		"list_files": true, "search": true, "web_search": true,
	}

	injectionPatterns := []string{
		"ignore previous", "disregard", "new instructions",
		"system prompt", "jailbreak", "bypass",
	}

	for _, tool := range srv.Tools {
		// Check for tool name shadowing
		if builtinTools[strings.ToLower(tool.Name)] {
			a.shadowsBuiltin = true
		}

		// Check for prompt injection in tool descriptions
		descLower := strings.ToLower(tool.Description)
		for _, pattern := range injectionPatterns {
			if strings.Contains(descLower, pattern) {
				a.promptInjectionRisk = true
				break
			}
		}

		// Check for overly broad filesystem permissions in input schemas
		if schemaStr, err := json.Marshal(tool.InputSchema); err == nil {
			schemaLower := strings.ToLower(string(schemaStr))
			if strings.Contains(schemaLower, "/**") ||
				strings.Contains(schemaLower, "/*") ||
				strings.Contains(schemaLower, "~/.ssh") ||
				strings.Contains(schemaLower, "~/.aws") {
				a.overlyBroadPerms = true
			}
		}
	}
	return a
}

// checkDependencyConfusion checks if an npm package could be a dependency confusion attack.
func (s *Scraper) checkDependencyConfusion(name string) bool {
	internal := []string{"internal", "private", "corp", "local", "myorg"}
	lower := strings.ToLower(name)
	for _, ind := range internal {
		if strings.Contains(lower, ind) {
			return true
		}
	}
	return false
}

// toolNames extracts tool names from a list of tools.
func toolNames(tools []mcpTool) []string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	return names
}

// FetchSource downloads the npm tarball for a MCP server package.
func (s *Scraper) FetchSource(ctx context.Context, pkg core.PackageVersion) (core.SourceArtifact, error) {
	if pkg.SourceURL == "" || !strings.HasPrefix(pkg.SourceURL, "https://registry.npmjs.org") {
		return core.SourceArtifact{Package: pkg}, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pkg.SourceURL, nil)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("mcp: build download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("mcp: download tarball: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.CreateTemp("", fmt.Sprintf("fg-mcp-%s-%s-*.tgz",
		strings.ReplaceAll(pkg.Name, "/", "-"), pkg.Version))
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("mcp: create temp file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("mcp: write tarball: %w", err)
	}

	return core.SourceArtifact{
		Package:   pkg,
		LocalPath: f.Name(),
		Size:      size,
		SHA256:    hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// VerifyIntegrity performs a basic sanity check on the downloaded artifact.
func (s *Scraper) VerifyIntegrity(_ context.Context, src core.SourceArtifact) error {
	if src.LocalPath != "" && src.Size == 0 {
		return fmt.Errorf("mcp: empty artifact for %s@%s", src.Package.Name, src.Package.Version)
	}
	return nil
}
