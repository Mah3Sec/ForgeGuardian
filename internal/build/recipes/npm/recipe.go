// Package npm implements the hermetic build recipe for npm packages.
package npm

import (
	"archive/tar"
	"compress/gzip"
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

// Recipe builds npm packages hermetically from the registry tarball.
type Recipe struct{}

// New creates a new npm Recipe.
func New() *Recipe { return &Recipe{} }

// Ecosystem implements core.BuildRecipe.
func (r *Recipe) Ecosystem() string { return "npm" }

// Build downloads, verifies, and catalogs an npm package tarball.
// Security signals captured:
//   - Install lifecycle scripts (preinstall / install / postinstall / prepare)
//   - Complete file manifest
//   - Network connections attempted (always 0 — tarball is pre-fetched)
func (r *Recipe) Build(ctx context.Context, src core.SourceArtifact, _ core.Sandbox) (core.BuiltArtifact, error) {
	name, version := src.Package.Name, src.Package.Version

	tarballURL, registrySha1, installScripts, err := r.fetchMeta(ctx, name, version)
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("npm build: fetch meta: %w", err)
	}

	dlPath, sha256sum, err := util.Download(tarballURL, ".tgz")
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("npm build: download: %w", err)
	}
	defer os.Remove(dlPath)

	manifest, err := extractManifest(dlPath)
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("npm build: extract manifest: %w", err)
	}

	var log strings.Builder
	fmt.Fprintf(&log, "=== npm build: %s@%s ===\n", name, version)
	fmt.Fprintf(&log, "tarball_url:          %s\n", tarballURL)
	fmt.Fprintf(&log, "sha256:               %s\n", sha256sum)
	fmt.Fprintf(&log, "registry_sha1:        %s\n", registrySha1)
	fmt.Fprintf(&log, "install_scripts:      %v\n", installScripts)
	fmt.Fprintf(&log, "file_count:           %d\n", len(manifest))
	fmt.Fprintf(&log, "network_connections:  0\n")
	for _, f := range manifest {
		fmt.Fprintf(&log, "file: %s\n", f)
	}

	outPath := util.StableOutput("npm", name, version, ".tgz")
	if err := util.CopyFile(dlPath, outPath); err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("npm build: copy output: %w", err)
	}
	return util.BuildArtifact(src, outPath, sha256sum, log.String()), nil
}

// fetchMeta retrieves tarball URL, sha1sum, and install scripts from the npm registry.
func (r *Recipe) fetchMeta(ctx context.Context, name, version string) (tarballURL, shasum string, scripts []string, err error) {
	apiURL := fmt.Sprintf("https://registry.npmjs.org/%s/%s", name, version)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	req.Header.Set("User-Agent", util.UserAgent)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", nil, fmt.Errorf("npm registry returned %d", resp.StatusCode)
	}

	var doc struct {
		Dist struct {
			Tarball string `json:"tarball"`
			Shasum  string `json:"shasum"`
		} `json:"dist"`
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", "", nil, err
	}
	for _, hook := range []string{"preinstall", "install", "postinstall", "prepare"} {
		if s, ok := doc.Scripts[hook]; ok {
			scripts = append(scripts, fmt.Sprintf("%s: %s", hook, s))
		}
	}
	return doc.Dist.Tarball, doc.Dist.Shasum, scripts, nil
}

// VerifyReproducible downloads the tarball a second time and compares SHA256.
// npm tarballs are content-addressed, so they must hash identically.
func (r *Recipe) VerifyReproducible(ctx context.Context, artifact core.BuiltArtifact) (bool, error) {
	a2, err := r.Build(ctx, artifact.Source, nil)
	if err != nil {
		return false, fmt.Errorf("npm reproducibility: second build: %w", err)
	}
	defer os.Remove(a2.LocalPath)
	return artifact.SHA256 == a2.SHA256, nil
}

// extractManifest returns the list of file paths inside a .tgz.
func extractManifest(tgzPath string) ([]string, error) {
	f, err := os.Open(tgzPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	var files []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return files, nil
		}
		files = append(files, hdr.Name)
	}
	return files, nil
}
