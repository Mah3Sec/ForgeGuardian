package localscanner

import (
	"context"
	"path/filepath"
	"sort"
	"sync"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/scanner"
)

// LocalScanResult is the result for a single dependency found in a manifest.
type LocalScanResult struct {
	Entry    ManifestEntry
	Findings []core.Finding
	Skipped  bool // true when Version is empty (range-only constraint)
	Err      error
}

// ProjectScanResult is the aggregate result of scanning a local directory.
type ProjectScanResult struct {
	RootDir      string
	Manifests    []ManifestFile
	Results      []LocalScanResult
	MissingTools []string // tool names that were not installed (unique, sorted)
	Summary      scanner.ScanSummary
}

// Scanner performs local project scanning against the intelligence signature store.
type Scanner struct {
	IntelStorePath string
	Workers        int
	ProdOnly       bool     // when true, dev dependencies are excluded from scanning
	cache          sync.Map // keyed by "ecosystem/name@version" → []core.Finding
}

// New creates a Scanner with sensible defaults.
func New(intelStorePath string) *Scanner {
	return &Scanner{
		IntelStorePath: intelStorePath,
		Workers:        4,
	}
}

// Scan walks rootDir, parses all manifests, and scans each dependency.
// progress is called after each package is processed (done, total).
func (s *Scanner) Scan(ctx context.Context, rootDir string, progress func(done, total int)) (*ProjectScanResult, error) {
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}

	manifests, err := Walk(abs)
	if err != nil {
		return nil, err
	}

	// Collect all entries across all manifests.
	var entries []ManifestEntry
	for _, mf := range manifests {
		e, err := ParseManifest(mf.Path)
		if err != nil {
			continue
		}
		entries = append(entries, e...)
	}

	// P0-2: filter dev dependencies when ProdOnly is set.
	if s.ProdOnly {
		filtered := entries[:0]
		for _, e := range entries {
			if !e.IsDevDependency {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	total := len(entries)
	results := make([]LocalScanResult, total)

	// Semaphore to limit concurrent scanner goroutines.
	sem := make(chan struct{}, s.Workers)
	var mu sync.Mutex
	var wg sync.WaitGroup
	done := 0

	for i, entry := range entries {
		if entry.Version == "" {
			results[i] = LocalScanResult{Entry: entry, Skipped: true}
			mu.Lock()
			done++
			if progress != nil {
				progress(done, total)
			}
			mu.Unlock()
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, e ManifestEntry) {
			defer wg.Done()
			defer func() { <-sem }()

			// P5-1: check scan result cache before invoking scanner engines.
			cacheKey := e.Ecosystem + "/" + e.Name + "@" + e.Version
			var findings []core.Finding
			if cached, ok := s.cache.Load(cacheKey); ok {
				findings = cached.([]core.Finding)
			} else {
				artifact := synthesizeArtifact(e)
				orch := scanner.NewWithIntelligence(s.IntelStorePath)
				scanResults := orch.Scan(ctx, artifact)
				findings = scanner.MergeFindings(scanResults)
				s.cache.Store(cacheKey, findings)
			}

			mu.Lock()
			results[idx] = LocalScanResult{Entry: e, Findings: findings}
			done++
			if progress != nil {
				progress(done, total)
			}
			mu.Unlock()
		}(i, entry)
	}

	wg.Wait()

	// Dedup TOOL-NOT-INSTALLED findings: collect unique tool names once globally,
	// then strip them from per-package results to avoid repeating the warning N times.
	missingToolSet := map[string]struct{}{}
	for i := range results {
		var kept []core.Finding
		for _, f := range results[i].Findings {
			if f.ID == "TOOL-NOT-INSTALLED" {
				missingToolSet[f.Source] = struct{}{}
			} else {
				kept = append(kept, f)
			}
		}
		results[i].Findings = kept
	}
	var missingTools []string
	for name := range missingToolSet {
		missingTools = append(missingTools, name)
	}
	sort.Strings(missingTools)

	// Build aggregate summary (after dedup so informational tool warnings don't inflate counts).
	var allFindings []core.Finding
	for _, r := range results {
		allFindings = append(allFindings, r.Findings...)
	}
	summary := scanner.Summarize(allFindings)

	return &ProjectScanResult{
		RootDir:      abs,
		Manifests:    manifests,
		Results:      results,
		MissingTools: missingTools,
		Summary:      summary,
	}, nil
}

// synthesizeArtifact creates a BuiltArtifact from a ManifestEntry without
// downloading the actual package. LocalPath is intentionally empty so that
// scanners requiring a local archive (grype, semgrep, trivy) skip gracefully.
// OSV (CVE lookup by name+version), behavioral (typosquat heuristics), and
// malware (blocklist lookup) all produce findings with name+version alone.
func synthesizeArtifact(e ManifestEntry) core.BuiltArtifact {
	return core.BuiltArtifact{
		Source: core.SourceArtifact{
			Package: core.PackageVersion{
				Ecosystem: e.Ecosystem,
				Name:      e.Name,
				Version:   e.Version,
			},
		},
		LocalPath: "", // intentionally empty — triggers graceful skip in archive scanners
		BuildLog:  "install_scripts: []\nnetwork_connections: 0\nfiles: []",
	}
}
