package feeds

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/intelligence"
)

// ossf GitHub API endpoints for listing malicious package reports.
// Each ecosystem has its own subdirectory in the OSV-format repo.
const ossfAPIBase = "https://api.github.com/repos/ossf/malicious-packages/contents/osv/malicious"
const ossfRawBase = "https://raw.githubusercontent.com/ossf/malicious-packages/main/osv/malicious"

// ossfEcosystem maps FG ecosystem names to OpenSSF directory names.
var ossfEcosystem = map[string]string{
	"npm":      "npm",
	"pypi":     "pypi",
	"go":       "go",
	"rubygems": "rubygems",
	"crates":   "crates.io",
	"maven":    "maven",
}

type githubEntry struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
	Type        string `json:"type"`
}

type ossfOSV struct {
	ID       string `json:"id"`
	Summary  string `json:"summary"`
	Details  string `json:"details"`
	Affected []struct {
		Package struct {
			Name      string `json:"name"`
			Ecosystem string `json:"ecosystem"`
		} `json:"package"`
	} `json:"affected"`
}

// MaliciousPackagesPoller fetches the OpenSSF malicious-packages repo entries.
type MaliciousPackagesPoller struct {
	client *http.Client
	// MaxPerEcosystem limits fetched entries per ecosystem (0 = no limit).
	MaxPerEcosystem int
}

// NewMaliciousPackagesPoller returns a new OpenSSF feed poller.
// maxPerEco limits how many JSON files are fetched per ecosystem (use 100 for first run).
func NewMaliciousPackagesPoller(maxPerEco int) *MaliciousPackagesPoller {
	return &MaliciousPackagesPoller{
		client:          &http.Client{Timeout: 20 * time.Second},
		MaxPerEcosystem: maxPerEco,
	}
}

// Poll fetches malicious package reports from the OpenSSF repo for the given ecosystems.
func (p *MaliciousPackagesPoller) Poll(ctx context.Context, ecosystems []string) ([]intelligence.DetectionSignature, error) {
	var all []intelligence.DetectionSignature
	for _, eco := range ecosystems {
		dir, ok := ossfEcosystem[eco]
		if !ok {
			continue
		}
		sigs, err := p.pollEcosystem(ctx, eco, dir)
		if err != nil {
			// Non-fatal: log and continue
			continue
		}
		all = append(all, sigs...)
	}
	return all, nil
}

func (p *MaliciousPackagesPoller) pollEcosystem(ctx context.Context, eco, dir string) ([]intelligence.DetectionSignature, error) {
	url := fmt.Sprintf("%s/%s", ossfAPIBase, dir)
	entries, err := p.listDir(ctx, url)
	if err != nil {
		return nil, err
	}

	var sigs []intelligence.DetectionSignature
	fetched := 0
	for _, e := range entries {
		if e.Type != "file" || !strings.HasSuffix(e.Name, ".json") {
			continue
		}
		if p.MaxPerEcosystem > 0 && fetched >= p.MaxPerEcosystem {
			break
		}
		rawURL := fmt.Sprintf("%s/%s/%s", ossfRawBase, dir, e.Name)
		sig, err := p.fetchEntry(ctx, eco, rawURL)
		if err != nil {
			continue
		}
		if sig != nil {
			sigs = append(sigs, *sig)
		}
		fetched++
	}
	return sigs, nil
}

func (p *MaliciousPackagesPoller) listDir(ctx context.Context, url string) ([]githubEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "forgeguardian-intelligence/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ossf list dir: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // ecosystem dir doesn't exist yet
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ossf list dir %d", resp.StatusCode)
	}

	var entries []githubEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func (p *MaliciousPackagesPoller) fetchEntry(ctx context.Context, eco, rawURL string) (*intelligence.DetectionSignature, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "forgeguardian-intelligence/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ossf fetch %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, err
	}

	var entry ossfOSV
	if err := json.Unmarshal(body, &entry); err != nil {
		return nil, err
	}
	if len(entry.Affected) == 0 {
		return nil, nil
	}

	pkgName := entry.Affected[0].Package.Name
	if pkgName == "" {
		return nil, nil
	}

	desc := entry.Details
	if desc == "" {
		desc = entry.Summary
	}
	return &intelligence.DetectionSignature{
		ID:          fmt.Sprintf("FG-SIG-%d", time.Now().UnixNano()),
		Type:        intelligence.SigBlocklisted,
		Ecosystem:   eco,
		Package:     pkgName,
		Severity:    "critical",
		Title:       fmt.Sprintf("Malicious package: %s (%s)", pkgName, entry.ID),
		Description: desc,
		Source:      "ossf",
		CreatedAt:   time.Now().UTC(),
	}, nil
}
