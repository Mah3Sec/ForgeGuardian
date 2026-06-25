// Package pypi implements a registry scraper for the PyPI package ecosystem.
package pypi

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
	pypiBase  = "https://pypi.org"
	userAgent = "forgeguardian-scraper/0.1 (supply chain security)"
)

// Scraper polls PyPI for new package releases.
type Scraper struct {
	client *http.Client
	log    *slog.Logger
}

// New creates a new PyPI Scraper.
func New() *Scraper {
	return &Scraper{
		client: &http.Client{Timeout: 30 * time.Second},
		log:    slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("scraper", "pypi"),
	}
}

// Name implements core.RegistryScraper.
func (s *Scraper) Name() string { return "pypi" }

// PyPI JSON API response for a single project.
type pypiProject struct {
	Info     pypiInfo              `json:"info"`
	Releases map[string][]pypiFile `json:"releases"`
	URLs     []pypiFile            `json:"urls"`
}

type pypiInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Summary     string `json:"summary"`
	License     string `json:"license"`
	ProjectURL  string `json:"project_url"`
	RequiresDist []string `json:"requires_dist"`
}

type pypiFile struct {
	Filename        string   `json:"filename"`
	URL             string   `json:"url"`
	Digests         digests  `json:"digests"`
	PackageType     string   `json:"packagetype"`
	UploadTime      string   `json:"upload_time_iso_8601"`
	RequiresPython  string   `json:"requires_python"`
}

type digests struct {
	MD5    string `json:"md5"`
	SHA256 string `json:"sha256"`
}

// PyPI "recent updates" feed.
type pypiUpdates []pypiUpdateEntry

type pypiUpdateEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Date    string `json:"date"` // RFC3339
}

// Poll returns package versions published since lastRun.
func (s *Scraper) Poll(ctx context.Context, lastRun time.Time) ([]core.PackageVersion, error) {
	// PyPI provides a "latest updates" RSS/JSON feed.
	// /pypi/rss/updates.xml is HTML; use the JSON API feed instead.
	// We use the PyPI Warehouse /packages/latest API.
	url := pypiBase + "/pypi/last_serial/"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pypi: get last serial: %w", err)
	}
	defer resp.Body.Close()

	// Fall back to the simple API changelog feed approach:
	// https://pypi.org/rss/updates.xml is the official feed but not JSON.
	// We use the JSON-based /pypi/<project>/json endpoint polled from a
	// list of recently-updated packages discovered via the XML-RPC changelog.
	return s.pollViaChangelog(ctx, lastRun)
}

// pollViaChangelog uses PyPI's XML-RPC changelog to find recently updated packages.
func (s *Scraper) pollViaChangelog(ctx context.Context, lastRun time.Time) ([]core.PackageVersion, error) {
	// PyPI's XML-RPC endpoint returns newline-delimited log entries.
	// We call the changelog_last_serial and changelog_since_serial methods
	// via the simple JSON wrapper PyPI provides.
	since := lastRun.Unix()
	url := fmt.Sprintf("%s/pypi?%%3Aaction=changelog&since=%d", pypiBase, since)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("pypi: build changelog request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pypi: fetch changelog: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("pypi: read changelog body: %w", err)
	}

	// PyPI changelog response is a JSON array of [name, version, timestamp, action] tuples.
	var changelog [][]any
	if err := json.Unmarshal(body, &changelog); err != nil {
		// Fallback: use the simple updates feed
		return s.pollViaUpdates(ctx, lastRun)
	}

	seen := map[string]bool{}
	var packages []string
	for _, entry := range changelog {
		if len(entry) < 2 {
			continue
		}
		name, _ := entry[0].(string)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		packages = append(packages, name)
	}

	var versions []core.PackageVersion
	for _, name := range packages {
		pvs, err := s.fetchProjectVersions(ctx, name, lastRun)
		if err != nil {
			s.log.Warn("failed to fetch project versions", "package", name, "err", err)
			continue
		}
		versions = append(versions, pvs...)
	}
	return versions, nil
}

// pollViaUpdates uses the PyPI RSS updates feed as a fallback.
func (s *Scraper) pollViaUpdates(ctx context.Context, lastRun time.Time) ([]core.PackageVersion, error) {
	// Use the warehouse API: /pypi/rss/updates.xml parsed as text
	// For simplicity, poll the top-100 recently updated packages.
	url := "https://pypi.org/rss/updates.xml"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("pypi: build updates request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pypi: fetch updates: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	// Parse package names from RSS <title> elements: "package name 1.2.3"
	var names []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "<title>") && strings.Contains(line, "</title>") {
			title := strings.TrimPrefix(strings.TrimSuffix(line, "</title>"), "<title>")
			parts := strings.Fields(title)
			if len(parts) >= 1 && !strings.Contains(parts[0], "PyPI") {
				names = append(names, parts[0])
			}
		}
	}

	var versions []core.PackageVersion
	for _, name := range names {
		pvs, err := s.fetchProjectVersions(ctx, name, lastRun)
		if err != nil {
			s.log.Warn("failed to fetch project", "package", name, "err", err)
			continue
		}
		versions = append(versions, pvs...)
	}
	return versions, nil
}

// fetchProjectVersions fetches all versions of a PyPI project via the JSON API.
func (s *Scraper) fetchProjectVersions(ctx context.Context, name string, since time.Time) ([]core.PackageVersion, error) {
	url := fmt.Sprintf("%s/pypi/%s/json", pypiBase, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("pypi: build project request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pypi: fetch project: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pypi: project returned %d", resp.StatusCode)
	}

	var project pypiProject
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("pypi: decode project: %w", err)
	}

	var results []core.PackageVersion
	for version, files := range project.Releases {
		for _, f := range files {
			if f.PackageType == "bdist_egg" {
				continue // skip old egg format
			}
			if f.UploadTime == "" {
				continue
			}
			uploadTime, err := time.Parse(time.RFC3339, f.UploadTime)
			if err != nil {
				continue
			}
			if uploadTime.Before(since) {
				continue
			}

			// Detect dependency confusion risk: internal-sounding names
			isDependencyConfusionRisk := looksInternal(name)

			meta := map[string]any{
				"filename":                   f.Filename,
				"package_type":               f.PackageType,
				"requires_python":            f.RequiresPython,
				"dependency_confusion_risk":  isDependencyConfusionRisk,
				"md5":                        f.Digests.MD5,
			}

			results = append(results, core.PackageVersion{
				Ecosystem:   "pypi",
				Name:        project.Info.Name,
				Version:     version,
				SourceURL:   f.URL,
				Checksum:    f.Digests.SHA256,
				PublishedAt: uploadTime,
				Metadata:    meta,
			})
			break // one file per version is enough for the queue
		}
	}
	return results, nil
}

// looksInternal heuristically flags package names that resemble internal tooling.
func looksInternal(name string) bool {
	indicators := []string{"internal", "private", "corp", "company", "mycompany", "local"}
	lower := strings.ToLower(name)
	for _, ind := range indicators {
		if strings.Contains(lower, ind) {
			return true
		}
	}
	return false
}

// FetchSource downloads the source distribution for a package version.
func (s *Scraper) FetchSource(ctx context.Context, pkg core.PackageVersion) (core.SourceArtifact, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pkg.SourceURL, nil)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("pypi: build download request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("pypi: download artifact: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.CreateTemp("", fmt.Sprintf("fg-pypi-%s-%s-*", pkg.Name, pkg.Version))
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("pypi: create temp file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	if err != nil {
		return core.SourceArtifact{}, fmt.Errorf("pypi: write artifact: %w", err)
	}

	return core.SourceArtifact{
		Package:   pkg,
		LocalPath: f.Name(),
		Size:      size,
		SHA256:    hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// VerifyIntegrity checks the downloaded SHA256 against the registry-provided hash.
func (s *Scraper) VerifyIntegrity(_ context.Context, src core.SourceArtifact) error {
	expected := src.Package.Checksum
	if expected == "" {
		return nil // no hash provided by registry
	}
	if src.SHA256 != expected {
		return fmt.Errorf("pypi: integrity mismatch for %s@%s: got %s want %s",
			src.Package.Name, src.Package.Version, src.SHA256, expected)
	}
	return nil
}
