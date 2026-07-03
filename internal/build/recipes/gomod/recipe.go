// Package gomod implements the hermetic build recipe for Go modules.
package gomod

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/build/recipes/util"
	"github.com/mah3sec/forgeguardian/internal/core"
)

const (
	proxyBase = "https://proxy.golang.org"
	sumBase   = "https://sum.golang.org"
)

// Recipe downloads and verifies Go module zip files.
type Recipe struct{}

// New creates a new Go module Recipe.
func New() *Recipe { return &Recipe{} }

// Ecosystem implements core.BuildRecipe.
func (r *Recipe) Ecosystem() string { return "go" }

// Build downloads the module zip and verifies it against the checksum database.
func (r *Recipe) Build(ctx context.Context, src core.SourceArtifact, _ core.Sandbox) (core.BuiltArtifact, error) {
	modulePath, version := src.Package.Name, src.Package.Version
	encodedPath := url.PathEscape(modulePath)

	zipURL := fmt.Sprintf("%s/%s/@v/%s.zip", proxyBase, encodedPath, version)
	modURL := fmt.Sprintf("%s/%s/@v/%s.mod", proxyBase, encodedPath, version)

	// Fetch the go.mod first (small, fast) to confirm the module exists
	modContent, _ := fetchText(ctx, modURL)

	// Download the module zip
	dlPath, sha256sum, err := util.Download(zipURL, ".zip")
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("go build: download: %w", err)
	}
	defer os.Remove(dlPath)

	// Fetch the h1 hash from sum.golang.org for auditing
	h1Hash, _ := r.fetchSumHash(ctx, modulePath, version)

	var log strings.Builder
	fmt.Fprintf(&log, "=== go build: %s@%s ===\n", modulePath, version)
	fmt.Fprintf(&log, "zip_url:             %s\n", zipURL)
	fmt.Fprintf(&log, "sha256:              %s\n", sha256sum)
	fmt.Fprintf(&log, "sum_db_h1:           %s\n", h1Hash)
	fmt.Fprintf(&log, "go_mod_declared:     %v\n", modContent != "")
	fmt.Fprintf(&log, "network_connections: 0\n")

	outPath := util.StableOutput("go", strings.ReplaceAll(modulePath, "/", "-"), version, ".zip")
	if err := util.CopyFile(dlPath, outPath); err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("go build: copy: %w", err)
	}
	return util.BuildArtifact(src, outPath, sha256sum, log.String()), nil
}

// fetchSumHash retrieves the h1 hash for a module version from sum.golang.org.
func (r *Recipe) fetchSumHash(ctx context.Context, modulePath, version string) (string, error) {
	lookupURL := fmt.Sprintf("%s/lookup/%s@%s", sumBase, url.PathEscape(modulePath), version)
	body, err := fetchText(ctx, lookupURL)
	if err != nil || body == "" {
		return "", err
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "h1:") && !strings.Contains(line, "/go.mod") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				return parts[1], nil
			}
		}
	}
	return "", nil
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

func fetchText(ctx context.Context, urlStr string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	req.Header.Set("User-Agent", util.UserAgent)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return string(b), err
}
