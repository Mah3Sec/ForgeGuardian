// Package osv wraps the Google OSV-Scanner binary and falls back to the OSV REST
// API when the binary is absent, ensuring vulnerability data is always available.
package osv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
)

const osvAPIBase = "https://api.osv.dev/v1"

// Scanner queries OSV for known vulnerabilities via binary or REST API fallback.
type Scanner struct {
	// BinaryPath overrides the osv-scanner binary location. Empty = use PATH.
	BinaryPath string
}

// New returns a new OSV Scanner.
func New() *Scanner { return &Scanner{} }

// Name implements core.Scanner.
func (s *Scanner) Name() string { return "osv" }

// Scan queries OSV for known vulnerabilities in the artifact's package.
// Prefers the osv-scanner binary; falls back to the REST API.
func (s *Scanner) Scan(ctx context.Context, artifact core.BuiltArtifact) ([]core.Finding, error) {
	bin := s.bin()
	if _, err := exec.LookPath(bin); err == nil {
		return s.scanBinary(ctx, artifact, bin)
	}
	return s.scanAPI(ctx, artifact)
}

// --- binary path ---

func (s *Scanner) scanBinary(ctx context.Context, artifact core.BuiltArtifact, bin string) ([]core.Finding, error) {
	pkg := artifact.Source.Package

	// osv-scanner --format json --lockfile <path> is for lock files.
	// For a single package we use --package with ecosystem + name@version.
	// Supported syntax: osv-scanner --package <purl>
	purl := buildPURL(pkg)
	cmd := exec.CommandContext(ctx, bin, "--format", "json", "--package", purl)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil && stdout.Len() == 0 {
		// Fall through to API on binary failure
		return s.scanAPI(ctx, artifact)
	}

	return parseOSVBinaryOutput(stdout.Bytes(), artifact)
}

type osvBinaryReport struct {
	Results []struct {
		Source  struct{ Path string } `json:"source"`
		Packages []struct {
			Package struct {
				Name      string `json:"name"`
				Version   string `json:"version"`
				Ecosystem string `json:"ecosystem"`
			} `json:"package"`
			Vulnerabilities []osvVuln `json:"vulnerabilities"`
		} `json:"packages"`
	} `json:"results"`
}

func parseOSVBinaryOutput(data []byte, artifact core.BuiltArtifact) ([]core.Finding, error) {
	var report osvBinaryReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("osv binary: parse: %w", err)
	}
	var findings []core.Finding
	for _, r := range report.Results {
		for _, p := range r.Packages {
			for _, v := range p.Vulnerabilities {
				findings = append(findings, vulnToFinding(v, artifact, "osv-scanner"))
			}
		}
	}
	return findings, nil
}

// --- REST API path ---

func (s *Scanner) scanAPI(ctx context.Context, artifact core.BuiltArtifact) ([]core.Finding, error) {
	pkg := artifact.Source.Package
	ecosystem := ecosystemForOSV(pkg.Ecosystem)
	if ecosystem == "" {
		return []core.Finding{noEcosystem(s.Name(), pkg.Ecosystem)}, nil
	}

	reqBody := osvQueryRequest{
		Package: osvPackage{
			Name:      pkg.Name,
			Ecosystem: ecosystem,
		},
		Version: pkg.Version,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("osv api: marshal: %w", err)
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		osvAPIBase+"/query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "forgeguardian-scanner/0.1")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("osv api: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("osv api: status %d: %s", resp.StatusCode, string(b))
	}

	var result osvQueryResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("osv api: decode: %w", err)
	}

	findings := make([]core.Finding, 0, len(result.Vulns))
	for _, v := range result.Vulns {
		findings = append(findings, vulnToFinding(v, artifact, "osv-api"))
	}
	return findings, nil
}

// --- OSV API types ---

type osvQueryRequest struct {
	Package osvPackage `json:"package"`
	Version string     `json:"version"`
}

type osvPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

type osvQueryResponse struct {
	Vulns []osvVuln `json:"vulns"`
}

type osvVuln struct {
	ID       string         `json:"id"`
	Summary  string         `json:"summary"`
	Details  string         `json:"details"`
	Severity []osvSeverity  `json:"severity"`
	Aliases  []string       `json:"aliases"`
	Modified string         `json:"modified"`
	Published string        `json:"published"`
	References []osvRef     `json:"references"`
	Affected []osvAffected  `json:"affected"`
}

type osvSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

type osvRef struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type osvAffected struct {
	Ranges []struct {
		Events []struct {
			Fixed string `json:"fixed"`
		} `json:"events"`
	} `json:"ranges"`
}

func vulnToFinding(v osvVuln, artifact core.BuiltArtifact, source string) core.Finding {
	sev := osvSev(v.Severity)
	desc := v.Details
	if desc == "" {
		desc = v.Summary
	}
	if desc == "" {
		desc = v.ID
	}

	// Extract fixed versions from affected ranges
	var fixes []string
	for _, a := range v.Affected {
		for _, r := range a.Ranges {
			for _, e := range r.Events {
				if e.Fixed != "" {
					fixes = append(fixes, e.Fixed)
				}
			}
		}
	}

	meta := map[string]any{
		"aliases":      v.Aliases,
		"published":    v.Published,
		"modified":     v.Modified,
		"fix_versions": fixes,
		"ecosystem":    artifact.Source.Package.Ecosystem,
	}

	fixedVersion := ""
	if len(fixes) > 0 {
		fixedVersion = fixes[0]
	}

	return core.Finding{
		ID:           v.ID,
		Severity:     sev,
		Type:         "cve",
		Title:        fmt.Sprintf("%s: %s", v.ID, v.Summary),
		Description:  desc,
		Source:       source,
		FixedVersion: fixedVersion,
		Metadata:     meta,
	}
}

func osvSev(severities []osvSeverity) core.Severity {
	// Use the highest CVSS score present
	var maxScore float64
	for _, s := range severities {
		// Parse numeric score from CVSS vector or direct score string
		var score float64
		fmt.Sscanf(s.Score, "%f", &score)
		if score > maxScore {
			maxScore = score
		}
	}
	switch {
	case maxScore >= 9.0:
		return core.SeverityCritical
	case maxScore >= 7.0:
		return core.SeverityHigh
	case maxScore >= 4.0:
		return core.SeverityMedium
	case maxScore > 0:
		return core.SeverityLow
	default:
		return core.SeverityMedium // Unknown CVSS — default to medium
	}
}

func ecosystemForOSV(ecosystem string) string {
	switch ecosystem {
	case "npm", "mcp":
		return "npm"
	case "pypi":
		return "PyPI"
	case "maven":
		return "Maven"
	case "go":
		return "Go"
	case "rubygems":
		return "RubyGems"
	case "crates":
		return "crates.io"
	case "huggingface":
		return "" // OSV doesn't cover HuggingFace yet
	}
	return ""
}

func buildPURL(pkg core.PackageVersion) string {
	switch pkg.Ecosystem {
	case "npm", "mcp":
		return fmt.Sprintf("pkg:npm/%s@%s", pkg.Name, pkg.Version)
	case "pypi":
		return fmt.Sprintf("pkg:pypi/%s@%s", strings.ToLower(pkg.Name), pkg.Version)
	case "maven":
		parts := strings.SplitN(pkg.Name, ":", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("pkg:maven/%s/%s@%s",
				strings.ReplaceAll(parts[0], ".", "/"), parts[1], pkg.Version)
		}
	case "go":
		return fmt.Sprintf("pkg:golang/%s@%s", pkg.Name, pkg.Version)
	case "rubygems":
		return fmt.Sprintf("pkg:gem/%s@%s", pkg.Name, pkg.Version)
	case "crates":
		return fmt.Sprintf("pkg:cargo/%s@%s", pkg.Name, pkg.Version)
	}
	return fmt.Sprintf("pkg:generic/%s@%s", pkg.Name, pkg.Version)
}

func (s *Scanner) bin() string {
	if s.BinaryPath != "" {
		return s.BinaryPath
	}
	return "osv-scanner"
}

func noEcosystem(name, ecosystem string) core.Finding {
	return core.Finding{
		ID:          "ECOSYSTEM-NOT-SUPPORTED",
		Severity:    core.SeverityInformational,
		Type:        "configuration",
		Title:       fmt.Sprintf("%s: ecosystem %q not supported — scan skipped", name, ecosystem),
		Description: "OSV does not index this ecosystem.",
		Source:      name,
		Metadata:    map[string]any{"skipped": true, "ecosystem": ecosystem},
	}
}
