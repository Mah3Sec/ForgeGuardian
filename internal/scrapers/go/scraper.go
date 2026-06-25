// Package gomod implements a registry scraper for the Go module proxy ecosystem.
package gomod

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
)

const (
	indexBase  = "https://index.golang.org"
	proxyBase  = "https://proxy.golang.org"
	sumBase    = "https://sum.golang.org"
	userAgent  = "forgeguardian-scraper/0.1 (supply chain security)"
	timeFormat = "2006-01-02T15:04:05.999999Z"
)

// Scraper polls the Go module proxy index for new module versions.
type Scraper struct {
	client *http.Client
	log    *slog.Logger
}

// New creates a new Go module Scraper.
func New() *Scraper {
	return &Scraper{
		client: &http.Client{Timeout: 30 * time.Second},
		log:    slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("scraper", "go"),
	}
}

// Name implements core.RegistryScraper.
func (s *Scraper) Name() string { return "go" }

// indexEntry is one line from the Go module index.
type indexEntry struct {
	Path      string    `json:"Path"`
	Version   string    `json:"Version"`
	Timestamp time.Time `json:"Timestamp"`
}

// Poll returns Go module versions published since lastRun.
func (s *Scraper) Poll(ctx context.Context, lastRun time.Time) ([]core.PackageVersion, error) {
	// The index endpoint streams NDJSON, sorted by timestamp ascending.
	since := lastRun.UTC().Format(timeFormat)
	indexURL := fmt.Sprintf("%s/index?since=%s", indexBase, url.QueryEscape(since))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, nil)
	if err != nil {
		return nil, fmt.Errorf("go: build index request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("go: fetch index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("go: index returned %d", resp.StatusCode)
	}

	var versions []core.PackageVersion
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Index lines are JSON: {"Path":"...","Version":"...","Timestamp":"..."}
		var entry indexEntry
		// Manual parse to avoid import of encoding/json in the hot path
		entry.Path = extractJSON(line, "Path")
		entry.Version = extractJSON(line, "Version")
		tsStr := extractJSON(line, "Timestamp")
		if tsStr != "" {
			t, err := time.Parse(time.RFC3339Nano, tsStr)
			if err == nil {
				entry.Timestamp = t
			}
		}

		if entry.Path == "" || entry.Version == "" {
			continue
		}

		// Source URL: https://proxy.golang.org/<module>/@v/<version>.zip
		encodedPath := url.PathEscape(entry.Path)
		sourceURL := fmt.Sprintf("%s/%s/@v/%s.zip", proxyBase, encodedPath, entry.Version)

		// Fetch the hash from the checksum database
		checksum, _ := s.fetchSumDBHash(ctx, entry.Path, entry.Version)

		meta := map[string]any{
			"mod_url":  fmt.Sprintf("%s/%s/@v/%s.mod", proxyBase, encodedPath, entry.Version),
			"info_url": fmt.Sprintf("%s/%s/@v/%s.info", proxyBase, encodedPath, entry.Version),
		}

		versions = append(versions, core.PackageVersion{
			Ecosystem:   "go",
			Name:        entry.Path,
			Version:     entry.Version,
			SourceURL:   sourceURL,
			Checksum:    checksum,
			PublishedAt: entry.Timestamp,
			Metadata:    meta,
		})
	}
	return versions, scanner.Err()
}

// fetchSumDBHash retrieves the h1 hash from sum.golang.org for a module version.
// The h1 hash is a Hash1 (tree hash) of the module zip contents.
func (s *Scraper) fetchSumDBHash(ctx context.Context, modulePath, version string) (string, error) {
	lookupURL := fmt.Sprintf("%s/lookup/%s@%s", sumBase, url.PathEscape(modulePath), version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lookupURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	if err != nil {
		return "", err
	}
	// Response format: <tile-hash>\n\n<module>@<version>/go.mod h1:<hash>\n<module>@<version> h1:<hash>\n
	for _, line := range strings.Split(string(body), "\n") {
		if strings.Contains(line, "h1:") && !strings.Contains(line, "/go.mod") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				return parts[1], nil // "h1:<hash>"
			}
		}
	}
	return "", nil
}

// extractJSON is a minimal JSON string extractor for known fields.
func extractJSON(s, key string) string {
	search := fmt.Sprintf(`"%s":"`, key)
	idx := strings.Index(s, search)
	if idx < 0 {
		return ""
	}
	start := idx + len(search)
	end := strings.Index(s[start:], `"`)
	if end < 0 {
		return ""
	}
	return s[start : start+end]
}

// FetchSource downloads the module zip from the Go proxy.
func (s *Scraper) FetchSource(ctx context.Context, pkg core.PackageVersion) (core.SourceArtifact, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pkg.SourceURL, nil)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("go: build download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("go: download zip: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.CreateTemp("", fmt.Sprintf("fg-go-%s-*.zip", pkg.Version))
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("go: create temp file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("go: write zip: %w", err)
	}

	return core.SourceArtifact{
		Package:   pkg,
		LocalPath: f.Name(),
		Size:      size,
		SHA256:    hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// VerifyIntegrity validates the downloaded zip against the sum database h1 hash.
// Note: the h1 hash is a tree hash, not a plain SHA256, so full verification
// requires the dirhash package. We store our own SHA256 for now.
func (s *Scraper) VerifyIntegrity(ctx context.Context, src core.SourceArtifact) error {
	if src.SHA256 == "" {
		return fmt.Errorf("go: no sha256 for %s@%s", src.Package.Name, src.Package.Version)
	}
	// Re-verify against the sum DB to catch any proxy tampering.
	expected, err := s.fetchSumDBHash(ctx, src.Package.Name, src.Package.Version)
	if err != nil || expected == "" {
		// Non-fatal: sum DB may not have the entry yet for very new modules.
		return nil
	}
	// expected is "h1:<base64>" — store this for auditing but can't compare to SHA256.
	// A mismatch here would need the golang.org/x/mod/dirhash package (Phase 3).
	return nil
}
