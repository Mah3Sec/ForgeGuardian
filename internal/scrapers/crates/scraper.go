// Package crates implements a registry scraper for the crates.io Rust ecosystem.
package crates

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
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
)

const (
	cratesBase = "https://crates.io/api/v1"
	cdnBase    = "https://static.crates.io/crates"
	// crates.io requires a descriptive User-Agent identifying the crawler.
	userAgent = "forgeguardian-scraper/0.1 contact:security@forgeguardian.io"
)

// Scraper polls crates.io for recently updated crates.
type Scraper struct {
	client *http.Client
	log    *slog.Logger
}

// New creates a new crates.io Scraper.
func New() *Scraper {
	return &Scraper{
		client: &http.Client{Timeout: 30 * time.Second},
		log:    slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("scraper", "crates"),
	}
}

// Name implements core.RegistryScraper.
func (s *Scraper) Name() string { return "crates" }

type cratesResponse struct {
	Crates []crateDoc `json:"crates"`
}

type crateDoc struct {
	Name          string `json:"name"`
	NewestVersion string `json:"newest_version"`
	MaxVersion    string `json:"max_version"`
	UpdatedAt     string `json:"updated_at"`
	Description   string `json:"description"`
}

type crateVersionsResponse struct {
	Versions []crateVersion `json:"versions"`
}

type crateVersion struct {
	ID        int64               `json:"id"`
	CrateName string              `json:"crate"`
	Num       string              `json:"num"`
	DLPath    string              `json:"dl_path"`
	CreatedAt string              `json:"created_at"`
	Checksum  string              `json:"checksum"` // sha256 hex
	Downloads int64               `json:"downloads"`
	Features  map[string][]string `json:"features"`
}

// Poll returns crate versions published since lastRun.
func (s *Scraper) Poll(ctx context.Context, lastRun time.Time) ([]core.PackageVersion, error) {
	// Fetch recently updated crates sorted by update time.
	url := cratesBase + "/crates?sort=recent-updates&per_page=100"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("crates: build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("crates: fetch crates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crates: crates list returned %d", resp.StatusCode)
	}

	var cratesResp cratesResponse
	if err := json.NewDecoder(resp.Body).Decode(&cratesResp); err != nil {
		return nil, fmt.Errorf("crates: decode crates: %w", err)
	}

	var versions []core.PackageVersion
	for _, c := range cratesResp.Crates {
		updated, err := time.Parse(time.RFC3339, c.UpdatedAt)
		if err != nil || updated.Before(lastRun) {
			continue
		}

		// Fetch the specific versions updated since lastRun
		pvs, err := s.fetchVersions(ctx, c.Name, lastRun)
		if err != nil {
			s.log.Warn("failed to fetch crate versions", "crate", c.Name, "err", err)
			continue
		}
		versions = append(versions, pvs...)
	}
	return versions, nil
}

// fetchVersions retrieves all recent versions for a crate.
func (s *Scraper) fetchVersions(ctx context.Context, name string, since time.Time) ([]core.PackageVersion, error) {
	url := fmt.Sprintf("%s/crates/%s/versions", cratesBase, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var vResp crateVersionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&vResp); err != nil {
		return nil, err
	}

	var results []core.PackageVersion
	for _, v := range vResp.Versions {
		created, err := time.Parse(time.RFC3339, v.CreatedAt)
		if err != nil || created.Before(since) {
			continue
		}

		// Download URL: https://static.crates.io/crates/<name>/<name>-<version>.crate
		sourceURL := fmt.Sprintf("%s/%s/%s-%s.crate", cdnBase, name, name, v.Num)
		if v.DLPath != "" {
			sourceURL = "https://crates.io" + v.DLPath
		}

		results = append(results, core.PackageVersion{
			Ecosystem:   "crates",
			Name:        name,
			Version:     v.Num,
			SourceURL:   sourceURL,
			Checksum:    v.Checksum,
			PublishedAt: created,
			Metadata: map[string]any{
				"downloads": v.Downloads,
				"features":  v.Features,
			},
		})
	}
	return results, nil
}

// FetchSource downloads a .crate archive.
func (s *Scraper) FetchSource(ctx context.Context, pkg core.PackageVersion) (core.SourceArtifact, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pkg.SourceURL, nil)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("crates: build download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("crates: download crate: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.CreateTemp("", fmt.Sprintf("fg-crates-%s-%s-*.crate", pkg.Name, pkg.Version))
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("crates: create temp file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("crates: write crate: %w", err)
	}

	return core.SourceArtifact{
		Package:   pkg,
		LocalPath: f.Name(),
		Size:      size,
		SHA256:    hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// VerifyIntegrity validates the SHA256 checksum.
func (s *Scraper) VerifyIntegrity(_ context.Context, src core.SourceArtifact) error {
	expected := src.Package.Checksum
	if expected == "" {
		return nil
	}
	if src.SHA256 != expected {
		return fmt.Errorf("crates: integrity mismatch for %s@%s: got %s want %s",
			src.Package.Name, src.Package.Version, src.SHA256, expected)
	}
	return nil
}
