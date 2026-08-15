// Package trivy wraps the Aqua Security Trivy binary to scan built artifacts
// for CVEs, misconfigurations, and secret leaks.
// If trivy is not installed the scanner emits a single informational finding
// rather than an error, matching the graceful-degradation pattern used by
// grype and semgrep.
package trivy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mah3sec/forgeguardian/internal/core"
)

// Scanner invokes trivy as a subprocess and parses its JSON output.
type Scanner struct {
	// BinaryPath overrides the trivy binary location. Empty = use PATH.
	BinaryPath string
}

// New returns a new Trivy Scanner.
func New() *Scanner { return &Scanner{} }

// Name implements core.Scanner.
func (s *Scanner) Name() string { return "trivy" }

// Scan runs trivy against the artifact's local path and returns findings.
func (s *Scanner) Scan(ctx context.Context, artifact core.BuiltArtifact) ([]core.Finding, error) {
	bin := s.bin()
	if _, err := exec.LookPath(bin); err != nil {
		return []core.Finding{notInstalled()}, nil
	}

	// trivy fs <path> --format json --quiet --skip-update
	cmd := exec.CommandContext(ctx, bin,
		"fs",
		artifact.LocalPath,
		"--format", "json",
		"--quiet",
		"--skip-update",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stdout.Len() == 0 {
			return nil, fmt.Errorf("trivy: %w: %s", err, stderr.String())
		}
		// trivy exits non-zero when findings exist — parse anyway
	}

	return parseTrivyOutput(stdout.Bytes(), artifact)
}

// --- binary path ---

func (s *Scanner) bin() string {
	if s.BinaryPath != "" {
		return s.BinaryPath
	}
	return "trivy"
}

// --- JSON parsing ---

// trivyReport is the top-level JSON emitted by trivy --format json.
type trivyReport struct {
	Results []trivyResult `json:"Results"`
}

type trivyResult struct {
	Target          string           `json:"Target"`
	Vulnerabilities []trivyVuln      `json:"Vulnerabilities"`
	Secrets         []trivySecret    `json:"Secrets"`
	Misconfigs      []trivyMisconfig `json:"Misconfigurations"`
}

type trivyVuln struct {
	VulnerabilityID  string `json:"VulnerabilityID"`
	PkgName          string `json:"PkgName"`
	InstalledVersion string `json:"InstalledVersion"`
	FixedVersion     string `json:"FixedVersion"`
	Severity         string `json:"Severity"`
	Title            string `json:"Title"`
	Description      string `json:"Description"`
	PrimaryURL       string `json:"PrimaryURL"`
}

type trivySecret struct {
	RuleID   string `json:"RuleID"`
	Category string `json:"Category"`
	Severity string `json:"Severity"`
	Title    string `json:"Title"`
	Match    string `json:"Match"`
}

type trivyMisconfig struct {
	ID          string `json:"ID"`
	Type        string `json:"Type"`
	Severity    string `json:"Severity"`
	Title       string `json:"Title"`
	Description string `json:"Description"`
	Resolution  string `json:"Resolution"`
}

func parseTrivyOutput(data []byte, artifact core.BuiltArtifact) ([]core.Finding, error) {
	var report trivyReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("trivy: parse output: %w", err)
	}

	var findings []core.Finding
	pkg := artifact.Source.Package

	for _, result := range report.Results {
		for _, v := range result.Vulnerabilities {
			findings = append(findings, core.Finding{
				ID:          v.VulnerabilityID,
				Severity:    mapSeverity(v.Severity),
				Type:        "cve",
				Title:       buildVulnTitle(v),
				Description: buildVulnDesc(v, result.Target),
				Source:      "trivy",
				Metadata: map[string]any{
					"pkg_name":          v.PkgName,
					"installed_version": v.InstalledVersion,
					"fixed_version":     v.FixedVersion,
					"url":               v.PrimaryURL,
					"target":            result.Target,
					"ecosystem":         pkg.Ecosystem,
				},
			})
		}

		for _, sec := range result.Secrets {
			findings = append(findings, core.Finding{
				ID:       "TRIVY-SECRET-" + strings.ToUpper(sec.RuleID),
				Severity: mapSeverity(sec.Severity),
				Type:     "secret",
				Title:    fmt.Sprintf("Exposed secret: %s", sec.Title),
				Description: fmt.Sprintf(
					"Trivy detected a hardcoded secret in %s (category: %s). Match: %s",
					result.Target, sec.Category, redact(sec.Match),
				),
				Source: "trivy",
				Metadata: map[string]any{
					"rule_id":  sec.RuleID,
					"category": sec.Category,
					"target":   result.Target,
				},
			})
		}

		for _, mc := range result.Misconfigs {
			findings = append(findings, core.Finding{
				ID:          "TRIVY-MISCONFIG-" + mc.ID,
				Severity:    mapSeverity(mc.Severity),
				Type:        "misconfiguration",
				Title:       fmt.Sprintf("Misconfiguration: %s", mc.Title),
				Description: fmt.Sprintf("%s Resolution: %s", mc.Description, mc.Resolution),
				Source:      "trivy",
				Metadata: map[string]any{
					"misconfig_type": mc.Type,
					"target":         result.Target,
				},
			})
		}
	}

	return findings, nil
}

func buildVulnTitle(v trivyVuln) string {
	if v.Title != "" {
		return fmt.Sprintf("%s — %s", v.VulnerabilityID, v.Title)
	}
	return fmt.Sprintf("%s in %s@%s", v.VulnerabilityID, v.PkgName, v.InstalledVersion)
}

func buildVulnDesc(v trivyVuln, target string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Package %s@%s in %s", v.PkgName, v.InstalledVersion, target)
	if v.FixedVersion != "" {
		fmt.Fprintf(&sb, " — fixed in %s", v.FixedVersion)
	}
	if v.Description != "" {
		fmt.Fprintf(&sb, ". %s", v.Description)
	}
	if v.PrimaryURL != "" {
		fmt.Fprintf(&sb, " (%s)", v.PrimaryURL)
	}
	return sb.String()
}

func mapSeverity(s string) core.Severity {
	switch strings.ToUpper(s) {
	case "CRITICAL":
		return core.SeverityCritical
	case "HIGH":
		return core.SeverityHigh
	case "MEDIUM":
		return core.SeverityMedium
	case "LOW":
		return core.SeverityLow
	default:
		return core.SeverityInformational
	}
}

// redact masks most characters of a secret match to avoid logging credentials.
func redact(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}

func notInstalled() core.Finding {
	return core.Finding{
		ID:          "TRIVY-NOT-INSTALLED",
		Severity:    core.SeverityInformational,
		Type:        "configuration",
		Title:       "trivy not installed — scan skipped",
		Description: "Install trivy from https://aquasecurity.github.io/trivy to enable CVE, secret, and misconfiguration scanning.",
		Source:      "trivy",
	}
}
