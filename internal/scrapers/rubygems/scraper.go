// Package rubygems implements a registry scraper for the RubyGems ecosystem.
package rubygems

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
	gemsBase  = "https://rubygems.org"
	userAgent = "forgeguardian-scraper/0.1 (supply chain security)"
)

// Scraper polls RubyGems for recently updated gems.
type Scraper struct {
	client *http.Client
	log    *slog.Logger
}

// New creates a new RubyGems Scraper.
func New() *Scraper {
	return &Scraper{
		client: &http.Client{Timeout: 30 * time.Second},
		log:    slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("scraper", "rubygems"),
	}
}

// Name implements core.RegistryScraper.
func (s *Scraper) Name() string { return "rubygems" }

type gemVersion struct {
	Name             string `json:"name"`
	Version          string `json:"version"`
	Platform         string `json:"platform"`
	VersionCreatedAt string `json:"version_created_at"`
	Info             string `json:"info"`
	GemURI           string `json:"gem_uri"`
	SHA              string `json:"sha"`
	Dependencies     struct {
		Runtime []struct {
			Name         string `json:"name"`
			Requirements string `json:"requirements"`
		} `json:"runtime"`
	} `json:"dependencies"`
}

// Poll returns gems updated since lastRun.
func (s *Scraper) Poll(ctx context.Context, lastRun time.Time) ([]core.PackageVersion, error) {
	// /api/v1/activity/just_updated.json returns the 50 most recently updated gems.
	url := gemsBase + "/api/v1/activity/just_updated.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("rubygems: build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rubygems: fetch activity: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rubygems: activity returned %d", resp.StatusCode)
	}

	var gems []gemVersion
	if err := json.NewDecoder(resp.Body).Decode(&gems); err != nil {
		return nil, fmt.Errorf("rubygems: decode activity: %w", err)
	}

	var versions []core.PackageVersion
	for _, g := range gems {
		if g.VersionCreatedAt == "" {
			continue
		}
		// RubyGems returns ISO8601: "2026-05-13T11:25:56.393Z"
		published, err := time.Parse("2006-01-02T15:04:05.000Z", g.VersionCreatedAt)
		if err != nil {
			published, err = time.Parse(time.RFC3339, g.VersionCreatedAt)
			if err != nil {
				continue
			}
		}
		if published.Before(lastRun) {
			continue
		}

		versions = append(versions, core.PackageVersion{
			Ecosystem:   "rubygems",
			Name:        g.Name,
			Version:     g.Version,
			SourceURL:   g.GemURI,
			Checksum:    g.SHA,
			PublishedAt: published,
			Metadata: map[string]any{
				"platform": g.Platform,
				"info":     g.Info,
			},
		})
	}
	return versions, nil
}

// FetchSource downloads a .gem file.
func (s *Scraper) FetchSource(ctx context.Context, pkg core.PackageVersion) (core.SourceArtifact, error) {
	if pkg.SourceURL == "" {
		pkg.SourceURL = fmt.Sprintf("%s/gems/%s-%s.gem", gemsBase, pkg.Name, pkg.Version)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pkg.SourceURL, nil)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("rubygems: build download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("rubygems: download gem: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.CreateTemp("", fmt.Sprintf("fg-rubygems-%s-%s-*.gem", pkg.Name, pkg.Version))
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("rubygems: create temp file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("rubygems: write gem: %w", err)
	}

	return core.SourceArtifact{
		Package:   pkg,
		LocalPath: f.Name(),
		Size:      size,
		SHA256:    hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// VerifyIntegrity checks the SHA256 checksum against the registry value.
func (s *Scraper) VerifyIntegrity(_ context.Context, src core.SourceArtifact) error {
	if src.Package.Checksum == "" {
		return nil
	}
	if src.SHA256 != src.Package.Checksum {
		return fmt.Errorf("rubygems: integrity mismatch for %s@%s",
			src.Package.Name, src.Package.Version)
	}
	return nil
}
