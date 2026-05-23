// Package scanner orchestrates all scan engines concurrently and merges their findings.
package scanner

import (
	"context"
	"sort"
	"sync"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/intelligence"
	"github.com/mah3sec/forgeguardian/internal/scanner/ai_model"
	"github.com/mah3sec/forgeguardian/internal/scanner/behavioral"
	"github.com/mah3sec/forgeguardian/internal/scanner/grype"
	"github.com/mah3sec/forgeguardian/internal/scanner/malware"
	"github.com/mah3sec/forgeguardian/internal/scanner/mcp"
	"github.com/mah3sec/forgeguardian/internal/scanner/osv"
	"github.com/mah3sec/forgeguardian/internal/scanner/semgrep"
	"github.com/mah3sec/forgeguardian/internal/scanner/trivy"
)

// ScanResult holds the findings from a single scanner engine.
type ScanResult struct {
	Scanner  string
	Findings []core.Finding
	Err      error
}

// Orchestrator runs all registered scanners concurrently against a built artifact.
type Orchestrator struct {
	scanners []core.Scanner
}

// New creates an Orchestrator with the default scanner suite.
func New() *Orchestrator {
	return &Orchestrator{
		scanners: []core.Scanner{
			grype.New(),
			osv.New(),
			semgrep.New(),
			trivy.New(),
			behavioral.New(),
			malware.New(),
			ai_model.New(),
			mcp.New(),
		},
	}
}

// NewWith creates an Orchestrator with a custom set of scanners.
func NewWith(scanners ...core.Scanner) *Orchestrator {
	return &Orchestrator{scanners: scanners}
}

// NewWithIntelligence creates an Orchestrator with all intelligence-store-augmented
// scanners. storePath may use "~/" prefix.
// If the store file does not exist, falls back to built-in rules only.
// Community signatures are applied to: behavioral, malware, mcp, and ai_model scanners.
func NewWithIntelligence(storePath string) *Orchestrator {
	// Falls back to a local signatures/ dir (git clone) when signatures.json
	// doesn't exist yet — see LoadStoreWithLocalFallback's doc comment.
	store, _ := intelligence.LoadStoreWithLocalFallback(storePath)
	return &Orchestrator{
		scanners: []core.Scanner{
			grype.New(),
			osv.New(),
			semgrep.New(),
			trivy.New(),
			behavioral.NewWithStore(store),
			malware.NewWithStore(store),
			ai_model.NewWithStore(store),
			mcp.NewWithStore(store),
		},
	}
}

// Scan fans out all engines concurrently and returns per-engine results.
func (o *Orchestrator) Scan(ctx context.Context, artifact core.BuiltArtifact) []ScanResult {
	results := make([]ScanResult, len(o.scanners))
	var wg sync.WaitGroup

	for i, sc := range o.scanners {
		wg.Add(1)
		go func(idx int, sc core.Scanner) {
			defer wg.Done()
			findings, err := sc.Scan(ctx, artifact)
			results[idx] = ScanResult{
				Scanner:  sc.Name(),
				Findings: findings,
				Err:      err,
			}
		}(i, sc)
	}

	wg.Wait()
	return results
}

// MergeFindings flattens all ScanResult findings into a single deduplicated,
// severity-sorted slice. Scanner errors are surfaced as informational findings.
func MergeFindings(results []ScanResult) []core.Finding {
	seen := make(map[string]bool)
	var merged []core.Finding

	for _, r := range results {
		if r.Err != nil {
			title, desc := scannerErrorMessage(r.Scanner, r.Err)
			merged = append(merged, core.Finding{
				ID:          "SCANNER-ERROR",
				Severity:    core.SeverityInformational,
				Type:        "configuration",
				Title:       title,
				Description: desc,
				Source:      r.Scanner,
				Metadata:    map[string]any{"error": r.Err.Error()},
			})
			continue
		}
		for _, f := range r.Findings {
			key := f.Source + "/" + f.ID
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, f)
		}
	}

	// Sort: critical first, then by ID for determinism
	sort.Slice(merged, func(i, j int) bool {
		si := severityOrd(merged[i].Severity)
		sj := severityOrd(merged[j].Severity)
		if si != sj {
			return si > sj
		}
		return merged[i].ID < merged[j].ID
	})

	return merged
}

// scannerErrorMessage returns a user-friendly title and description for a scanner error.
func scannerErrorMessage(scannerName string, err error) (title, desc string) {
	switch scannerName {
	case "malware":
		return "malware signature scan skipped",
			"Heuristic analysis requires a local file. Name/version blocklist checks still ran."
	case "grype":
		return "grype scanner encountered an error",
			"grype reported an error during scanning. Ensure grype is installed and up to date: brew install anchore/grype/grype"
	case "semgrep":
		return "semgrep scanner encountered an error",
			"semgrep reported an error during scanning. Ensure semgrep is installed: pip install semgrep"
	case "trivy":
		return "trivy scanner encountered an error",
			"trivy reported an error during scanning. Ensure trivy is installed: brew install trivy"
	default:
		return scannerName + " scanner skipped",
			"The " + scannerName + " scanner was unable to complete. Run 'fgctl doctor' for diagnostics."
	}
}

func severityOrd(s core.Severity) int {
	switch s {
	case core.SeverityCritical:
		return 4
	case core.SeverityHigh:
		return 3
	case core.SeverityMedium:
		return 2
	case core.SeverityLow:
		return 1
	default:
		return 0
	}
}

// ScanSummary describes the aggregate risk of a scan.
type ScanSummary struct {
	Critical      int
	High          int
	Medium        int
	Low           int
	Informational int
	Total         int
	HighestSev    core.Severity
}

// Summarize counts findings by severity.
func Summarize(findings []core.Finding) ScanSummary {
	var s ScanSummary
	for _, f := range findings {
		s.Total++
		switch f.Severity {
		case core.SeverityCritical:
			s.Critical++
		case core.SeverityHigh:
			s.High++
		case core.SeverityMedium:
			s.Medium++
		case core.SeverityLow:
			s.Low++
		default:
			s.Informational++
		}
	}
	switch {
	case s.Critical > 0:
		s.HighestSev = core.SeverityCritical
	case s.High > 0:
		s.HighestSev = core.SeverityHigh
	case s.Medium > 0:
		s.HighestSev = core.SeverityMedium
	case s.Low > 0:
		s.HighestSev = core.SeverityLow
	default:
		s.HighestSev = core.SeverityInformational
	}
	return s
}
