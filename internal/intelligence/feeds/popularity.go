package feeds

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/intelligence"
)

const (
	npmDownloadsBase = "https://api.npmjs.org/downloads/point/last-month"
	depsDev          = "https://api.deps.dev/v3/systems"
)

// depsDev system names for each FG ecosystem
var depsDevSystem = map[string]string{
	"pypi":     "PYPI",
	"go":       "GO",
	"rubygems": "RUBYGEMS",
	"crates":   "CARGO",
	"maven":    "MAVEN",
}

// PopularityPoller discovers the most-downloaded packages per ecosystem
// and returns typosquat_target signatures for each.
type PopularityPoller struct {
	client *http.Client
	// TopN is the number of packages to include per ecosystem.
	TopN int
}

// NewPopularityPoller creates a popularity poller.
// topN sets how many top packages per ecosystem to track.
func NewPopularityPoller(topN int) *PopularityPoller {
	return &PopularityPoller{
		client: &http.Client{Timeout: 20 * time.Second},
		TopN:   topN,
	}
}

// Poll fetches popularity data and generates typosquat_target signatures.
// seedPackages provides a starting set to measure and expand from.
func (p *PopularityPoller) Poll(ctx context.Context, seedPackages map[string][]string) ([]intelligence.DetectionSignature, error) {
	var all []intelligence.DetectionSignature

	// npm: use bulk download count API
	if seeds, ok := seedPackages["npm"]; ok {
		sigs, err := p.pollNPM(ctx, seeds)
		if err == nil {
			all = append(all, sigs...)
		}
	}

	// Other ecosystems: use deps.dev dependent count as popularity proxy
	for eco, seeds := range seedPackages {
		if eco == "npm" {
			continue
		}
		system, ok := depsDevSystem[eco]
		if !ok {
			continue
		}
		sigs, err := p.pollDepsDev(ctx, eco, system, seeds)
		if err == nil {
			all = append(all, sigs...)
		}
	}

	return all, nil
}

type npmBulkDownload struct {
	Downloads map[string]struct {
		Downloads int    `json:"downloads"`
		Package   string `json:"package"`
	} `json:"downloads"`
}

type npmSingleDownload struct {
	Downloads int    `json:"downloads"`
	Package   string `json:"package"`
}

func (p *PopularityPoller) pollNPM(ctx context.Context, seeds []string) ([]intelligence.DetectionSignature, error) {
	type pkgDownloads struct {
		name      string
		downloads int
	}
	var ranked []pkgDownloads

	// npm bulk API: up to 128 packages per request
	for i := 0; i < len(seeds); i += 128 {
		end := i + 128
		if end > len(seeds) {
			end = len(seeds)
		}
		chunk := seeds[i:end]

		apiURL := fmt.Sprintf("%s/%s", npmDownloadsBase, url.PathEscape(strings.Join(chunk, ",")))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "forgeguardian-intelligence/1.0")

		resp, err := p.client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusOK {
			if len(chunk) == 1 {
				var single npmSingleDownload
				if err := json.NewDecoder(resp.Body).Decode(&single); err == nil {
					ranked = append(ranked, pkgDownloads{single.Package, single.Downloads})
				}
			} else {
				var bulk npmBulkDownload
				if err := json.NewDecoder(resp.Body).Decode(&bulk); err == nil {
					for _, d := range bulk.Downloads {
						ranked = append(ranked, pkgDownloads{d.Package, d.Downloads})
					}
				}
			}
		}
		resp.Body.Close()
	}

	// Sort by download count descending
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].downloads > ranked[j].downloads
	})

	topN := p.TopN
	if topN > len(ranked) {
		topN = len(ranked)
	}

	var sigs []intelligence.DetectionSignature
	for _, pkg := range ranked[:topN] {
		sigs = append(sigs, intelligence.DetectionSignature{
			ID:          fmt.Sprintf("FG-SIG-%d", time.Now().UnixNano()),
			Type:        intelligence.SigTypoTarget,
			Ecosystem:   "npm",
			Target:      pkg.name,
			Severity:    "high",
			Title:       fmt.Sprintf("Popular npm package: %s (%d downloads/month)", pkg.name, pkg.downloads),
			Description: fmt.Sprintf("npm package %s receives ~%d downloads/month and is a high-value typosquatting target.", pkg.name, pkg.downloads),
			Source:      "popularity",
			CreatedAt:   time.Now().UTC(),
		})
	}
	return sigs, nil
}

type depsDevPackage struct {
	Package struct {
		Name           string `json:"name"`
		VersionCount   int    `json:"versionCount"`
		DependentCount int    `json:"dependentCount"`
	} `json:"package"`
}

func (p *PopularityPoller) pollDepsDev(ctx context.Context, eco, system string, seeds []string) ([]intelligence.DetectionSignature, error) {
	type pkgScore struct {
		name  string
		score int
	}
	var scored []pkgScore

	for _, name := range seeds {
		apiURL := fmt.Sprintf("%s/%s/packages/%s", depsDev, system, url.PathEscape(name))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "forgeguardian-intelligence/1.0")

		resp, err := p.client.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode == http.StatusOK {
			var d depsDevPackage
			if err := json.NewDecoder(resp.Body).Decode(&d); err == nil {
				scored = append(scored, pkgScore{name, d.Package.DependentCount})
			}
		}
		resp.Body.Close()
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	topN := p.TopN
	if topN > len(scored) {
		topN = len(scored)
	}

	var sigs []intelligence.DetectionSignature
	for _, pkg := range scored[:topN] {
		sigs = append(sigs, intelligence.DetectionSignature{
			ID:          fmt.Sprintf("FG-SIG-%d", time.Now().UnixNano()),
			Type:        intelligence.SigTypoTarget,
			Ecosystem:   eco,
			Target:      pkg.name,
			Severity:    "high",
			Title:       fmt.Sprintf("Popular %s package: %s (%d dependents)", eco, pkg.name, pkg.score),
			Description: fmt.Sprintf("%s package %s has %d dependent packages and is a high-value typosquatting target.", eco, pkg.name, pkg.score),
			Source:      "popularity",
			CreatedAt:   time.Now().UTC(),
		})
	}
	return sigs, nil
}
