// Package grype wraps the Anchore Grype binary to scan built artifacts for CVEs.
// If grype is not installed, the scanner returns a skip finding rather than an error
// so the orchestrator can continue with other engines.
package grype

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mah3sec/forgeguardian/internal/core"
)

// Scanner invokes grype as a subprocess and parses its JSON output.
type Scanner struct {
	// BinaryPath overrides the grype binary location. Empty = use PATH.
	BinaryPath string
}

// New returns a new Grype Scanner.
func New() *Scanner { return &Scanner{} }

// Name implements core.Scanner.
func (s *Scanner) Name() string { return "grype" }

// Scan runs grype against the artifact's local path and returns CVE findings.
func (s *Scanner) Scan(ctx context.Context, artifact core.BuiltArtifact) ([]core.Finding, error) {
	bin := s.bin()
	if _, err := exec.LookPath(bin); err != nil {
		return []core.Finding{notInstalled(s.Name())}, nil
	}

	// grype <path> -o json --quiet
	cmd := exec.CommandContext(ctx, bin,
		artifact.LocalPath,
		"-o", "json",
		"--quiet",
		"--add-cpes-if-none",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// grype exits non-zero when vulnerabilities are found — that's expected
		if stdout.Len() == 0 {
			return nil, fmt.Errorf("grype: %w: %s", err, stderr.String())
		}
	}

	return parseGrypeOutput(stdout.Bytes(), artifact)
}

// grypeReport is the top-level JSON structure emitted by grype -o json.
type grypeReport struct {
	Matches []grypeMatch `json:"matches"`
}

type grypeMatch struct {
	Vulnerability grypeVuln    `json:"vulnerability"`
	Artifact      grypeArtifact `json:"artifact"`
}

type grypeVuln struct {
	ID          string   `json:"id"`
	DataSource  string   `json:"dataSource"`
	Severity    string   `json:"severity"`
	Description string   `json:"description"`
	CVSSs       []grypeCVSS `json:"cvss"`
	Fix         grypeFix `json:"fix"`
	URLs        []string `json:"urls"`
}

type grypeCVSS struct {
	Version string  `json:"version"`
	Score   float64 `json:"score"`
}

type grypeFix struct {
	State    string   `json:"state"`
	Versions []string `json:"versions"`
}

type grypeArtifact struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func parseGrypeOutput(data []byte, artifact core.BuiltArtifact) ([]core.Finding, error) {
	var report grypeReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("grype: parse output: %w", err)
	}

	findings := make([]core.Finding, 0, len(report.Matches))
	for _, m := range report.Matches {
		sev := mapSeverity(m.Vulnerability.Severity)
		score := maxCVSS(m.Vulnerability.CVSSs)
		desc := m.Vulnerability.Description
		if desc == "" {
			desc = fmt.Sprintf("%s in %s@%s", m.Vulnerability.ID, m.Artifact.Name, m.Artifact.Version)
		}
		fixedVersion := ""
		if m.Vulnerability.Fix.State == "fixed" && len(m.Vulnerability.Fix.Versions) > 0 {
			fixedVersion = m.Vulnerability.Fix.Versions[0]
		}

		meta := map[string]any{
			"cvss_score":   score,
			"data_source":  m.Vulnerability.DataSource,
			"fix_state":    m.Vulnerability.Fix.State,
			"artifact":     fmt.Sprintf("%s@%s", m.Artifact.Name, m.Artifact.Version),
			"package_name": artifact.Source.Package.Name,
			"ecosystem":    artifact.Source.Package.Ecosystem,
		}
		if fixedVersion != "" {
			meta["fix_versions"] = m.Vulnerability.Fix.Versions
		}

		findings = append(findings, core.Finding{
			ID:           m.Vulnerability.ID,
			Severity:     sev,
			Type:         "cve",
			Title:        fmt.Sprintf("%s in %s@%s", m.Vulnerability.ID, m.Artifact.Name, m.Artifact.Version),
			Description:  desc,
			Source:       "grype",
			FixedVersion: fixedVersion,
			Metadata:     meta,
		})
	}
	return findings, nil
}

func (s *Scanner) bin() string {
	if s.BinaryPath != "" {
		return s.BinaryPath
	}
	return "grype"
}

func mapSeverity(s string) core.Severity {
	switch strings.ToLower(s) {
	case "critical":
		return core.SeverityCritical
	case "high":
		return core.SeverityHigh
	case "medium":
		return core.SeverityMedium
	case "low":
		return core.SeverityLow
	default:
		return core.SeverityInformational
	}
}

func maxCVSS(scores []grypeCVSS) float64 {
	var max float64
	for _, c := range scores {
		if c.Score > max {
			max = c.Score
		}
	}
	return max
}

// notInstalled returns an informational finding when a tool binary is missing.
func notInstalled(name string) core.Finding {
	return core.Finding{
		ID:          "TOOL-NOT-INSTALLED",
		Severity:    core.SeverityInformational,
		Type:        "configuration",
		Title:       fmt.Sprintf("%s not installed — scan skipped", name),
		Description: fmt.Sprintf("Install %s to enable this scan engine.", name),
		Source:      name,
		Metadata:    map[string]any{"skipped": true},
	}
}
