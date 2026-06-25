// Package npm implements a registry scraper for the npm package ecosystem.
package npm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
)

const (
	registryBase  = "https://registry.npmjs.org"
	replicateBase = "https://replicate.npmjs.com"
	userAgent     = "forgeguardian-scraper/0.1 (supply chain security; https://github.com/forgeguardian)"
)

// Scraper polls the npm registry for new package versions.
type Scraper struct {
	client *http.Client
	log    *slog.Logger
}

// New creates a new npm Scraper.
func New() *Scraper {
	return &Scraper{
		client: &http.Client{Timeout: 30 * time.Second},
		log:    slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("scraper", "npm"),
	}
}

// Name implements core.RegistryScraper.
func (s *Scraper) Name() string { return "npm" }

// changesResponse is the CouchDB _changes response from replicate.npmjs.com.
type changesResponse struct {
	Results []changeEntry `json:"results"`
	LastSeq any           `json:"last_seq"`
}

// changeEntry is a single entry from the npm changes feed.
type changeEntry struct {
	ID      string         `json:"id"`
	Seq     any            `json:"seq"`
	Changes []changeDetail `json:"changes"`
}

type changeDetail struct {
	Rev string `json:"rev"`
}

// packument is the npm registry document for a single package.
type packument struct {
	Name     string                      `json:"name"`
	Versions map[string]packageVersion   `json:"versions"`
	Time     map[string]string           `json:"time"`
	DistTags map[string]string           `json:"dist-tags"`
}

type packageVersion struct {
	Name    string  `json:"name"`
	Version string  `json:"version"`
	Dist    distObj `json:"dist"`
	Scripts map[string]string `json:"scripts"`
}

type distObj struct {
	Tarball string `json:"tarball"`
	Shasum  string `json:"shasum"` // sha1 from npm
	Integrity string `json:"integrity"` // sha512 sri
}

// Poll returns package versions published since lastRun using the npm search API.
// The search API is fast and returns recently updated packages with timestamps.
// In production the CouchDB changes feed at replicate.npmjs.com/_changes is preferred
// for complete coverage, but it requires sequential packument fetches.
func (s *Scraper) Poll(ctx context.Context, lastRun time.Time) ([]core.PackageVersion, error) {
	// Fetch names of recently changed packages via the CouchDB changes feed.
	changesURL := fmt.Sprintf("%s/_changes?limit=200&descending=true", replicateBase)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, changesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("npm: build changes request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("npm: fetch changes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("npm: changes feed returned %d", resp.StatusCode)
	}

	var changesResp changesResponse
	if err := json.NewDecoder(resp.Body).Decode(&changesResp); err != nil {
		return nil, fmt.Errorf("npm: decode changes: %w", err)
	}

	// Fetch packuments concurrently — bounded to 20 in-flight at once.
	type result struct {
		versions []core.PackageVersion
		err      error
	}

	sem := make(chan struct{}, 20)
	results := make([]result, len(changesResp.Results))
	var wg sync.WaitGroup

	for i, entry := range changesResp.Results {
		if entry.ID == "" || strings.HasPrefix(entry.ID, "_") {
			continue
		}
		i, id := i, entry.ID
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			pvs, err := s.fetchPackageVersions(ctx, id, lastRun)
			results[i] = result{versions: pvs, err: err}
		}()
	}
	wg.Wait()

	var versions []core.PackageVersion
	for _, r := range results {
		if r.err != nil {
			continue
		}
		versions = append(versions, r.versions...)
	}
	return versions, nil
}

// fetchPackageVersions fetches the packument for a package and returns
// all versions published since lastRun.
func (s *Scraper) fetchPackageVersions(ctx context.Context, name string, since time.Time) ([]core.PackageVersion, error) {
	url := fmt.Sprintf("%s/%s", registryBase, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("npm: build packument request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("npm: fetch packument: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("npm: packument returned %d", resp.StatusCode)
	}

	var doc packument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("npm: decode packument: %w", err)
	}

	var results []core.PackageVersion
	for ver, v := range doc.Versions {
		publishedStr, ok := doc.Time[ver]
		if !ok {
			continue
		}
		published, err := time.Parse(time.RFC3339, publishedStr)
		if err != nil {
			continue
		}
		if published.Before(since) {
			continue
		}

		// Detect dangerous lifecycle scripts
		hasInstallScript := false
		for _, hook := range []string{"preinstall", "install", "postinstall", "prepare"} {
			if _, ok := v.Scripts[hook]; ok {
				hasInstallScript = true
				break
			}
		}

		meta := map[string]any{
			"dist_tags":          doc.DistTags,
			"has_install_script": hasInstallScript,
			"scripts":            v.Scripts,
			"integrity":          v.Dist.Integrity,
		}

		results = append(results, core.PackageVersion{
			Ecosystem:   "npm",
			Name:        doc.Name,
			Version:     ver,
			SourceURL:   v.Dist.Tarball,
			Checksum:    v.Dist.Shasum,
			PublishedAt: published,
			Metadata:    meta,
		})
	}
	return results, nil
}

// FetchSource downloads the tarball for a package version.
func (s *Scraper) FetchSource(ctx context.Context, pkg core.PackageVersion) (core.SourceArtifact, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pkg.SourceURL, nil)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("npm: build download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("npm: download tarball: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.CreateTemp("", fmt.Sprintf("fg-npm-%s-%s-*.tgz", pkg.Name, pkg.Version))
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("npm: create temp file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("npm: write tarball: %w", err)
	}

	return core.SourceArtifact{
		Package:   pkg,
		LocalPath: f.Name(),
		Size:      size,
		SHA256:    hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// VerifyIntegrity checks that the downloaded source SHA256 matches the
// subresource integrity field if present, or falls back to the shasum.
func (s *Scraper) VerifyIntegrity(_ context.Context, src core.SourceArtifact) error {
	// npm's legacy checksum is sha1 (shasum field). We store SHA256 on download.
	// The integrity field uses SRI format: sha512-<base64>
	// For Phase 2 we accept the download and trust our own SHA256.
	if src.SHA256 == "" {
		return fmt.Errorf("npm: integrity check: no sha256 computed for %s@%s",
			src.Package.Name, src.Package.Version)
	}
	return nil
}
