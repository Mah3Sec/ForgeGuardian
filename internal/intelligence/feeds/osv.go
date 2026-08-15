// Package feeds provides threat intelligence feed pollers for ForgeGuardian.
package feeds

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mah3sec/forgeguardian/internal/intelligence"
)

const osvBatchURL = "https://api.osv.dev/v1/querybatch"

type osvQuery struct {
	Package struct {
		Name      string `json:"name"`
		Ecosystem string `json:"ecosystem"`
	} `json:"package"`
}

type osvBatchRequest struct {
	Queries []osvQuery `json:"queries"`
}

type osvVuln struct {
	ID       string   `json:"id"`
	Aliases  []string `json:"aliases"`
	Summary  string   `json:"summary"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
}

type osvBatchResult struct {
	Results []struct {
		Vulns []osvVuln `json:"vulns"`
	} `json:"results"`
}

// OSVPoller queries the OSV batch API for vulnerability data on a set of packages.
type OSVPoller struct {
	client *http.Client
}

// NewOSVPoller returns a new OSV feed poller.
func NewOSVPoller() *OSVPoller {
	return &OSVPoller{client: &http.Client{Timeout: 30 * time.Second}}
}

// osvEcosystem maps ForgeGuardian ecosystem names to OSV ecosystem names.
var osvEcosystem = map[string]string{
	"npm":         "npm",
	"pypi":        "PyPI",
	"go":          "Go",
	"rubygems":    "RubyGems",
	"crates":      "crates.io",
	"maven":       "Maven",
	"huggingface": "PyPI", // HF models have PyPI-style deps
}

type queryMeta struct {
	ecosystem string
	name      string
}

// Poll queries OSV for any known vulnerabilities across the given packages.
// packages is a map of ecosystem → []packageName. Returns blocklisted_package
// signatures for any packages with known critical/high CVEs.
func (p *OSVPoller) Poll(ctx context.Context, packages map[string][]string) ([]intelligence.DetectionSignature, error) {
	var queries []osvQuery
	var meta []queryMeta

	for eco, names := range packages {
		osvEco, ok := osvEcosystem[eco]
		if !ok {
			continue
		}
		for _, name := range names {
			var q osvQuery
			q.Package.Name = name
			q.Package.Ecosystem = osvEco
			queries = append(queries, q)
			meta = append(meta, queryMeta{eco, name})
		}
	}

	if len(queries) == 0 {
		return nil, nil
	}

	// OSV batch allows up to 1000 per request
	var all []intelligence.DetectionSignature
	for i := 0; i < len(queries); i += 1000 {
		end := i + 1000
		if end > len(queries) {
			end = len(queries)
		}
		chunk := queries[i:end]
		chunkMeta := meta[i:end]

		sigs, err := p.pollChunk(ctx, chunk, chunkMeta)
		if err != nil {
			return all, fmt.Errorf("osv poll chunk %d: %w", i/1000, err)
		}
		all = append(all, sigs...)
	}
	return all, nil
}

func (p *OSVPoller) pollChunk(ctx context.Context, queries []osvQuery, meta []queryMeta) ([]intelligence.DetectionSignature, error) {
	body, err := json.Marshal(osvBatchRequest{Queries: queries})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, osvBatchURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "forgeguardian-intelligence/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("osv http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("osv response %d", resp.StatusCode)
	}

	var result osvBatchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("osv decode: %w", err)
	}

	var sigs []intelligence.DetectionSignature
	for i, r := range result.Results {
		if i >= len(meta) {
			break
		}
		m := meta[i]
		for _, v := range r.Vulns {
			if !isHighOrCritical(v) {
				continue
			}
			cve := firstCVE(v)
			sigs = append(sigs, intelligence.DetectionSignature{
				ID:          fmt.Sprintf("FG-SIG-%d", time.Now().UnixNano()),
				Type:        intelligence.SigBlocklisted,
				Ecosystem:   m.ecosystem,
				Package:     m.name,
				Severity:    osvSeverity(v),
				Title:       fmt.Sprintf("%s: %s", v.ID, v.Summary),
				Description: fmt.Sprintf("OSV reports %s has known vulnerability %s: %s", m.name, v.ID, v.Summary),
				Source:      "osv",
				CVE:         cve,
				CreatedAt:   time.Now().UTC(),
			})
		}
	}
	return sigs, nil
}

func isHighOrCritical(v osvVuln) bool {
	for _, s := range v.Severity {
		if s.Type == "CVSS_V3" {
			var score float64
			fmt.Sscanf(s.Score, "%f", &score)
			if score >= 7.0 {
				return true
			}
		}
	}
	// Fall back: MAL-prefixed IDs are always relevant
	return len(v.ID) > 4 && v.ID[:4] == "MAL-"
}

func osvSeverity(v osvVuln) string {
	for _, s := range v.Severity {
		if s.Type == "CVSS_V3" {
			var score float64
			fmt.Sscanf(s.Score, "%f", &score)
			switch {
			case score >= 9.0:
				return "critical"
			case score >= 7.0:
				return "high"
			case score >= 4.0:
				return "medium"
			default:
				return "low"
			}
		}
	}
	return "high" // default for MAL- entries
}

func firstCVE(v osvVuln) string {
	for _, a := range v.Aliases {
		if len(a) > 4 && a[:4] == "CVE-" {
			return a
		}
	}
	return ""
}
