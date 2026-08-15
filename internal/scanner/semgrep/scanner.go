// Package semgrep wraps the Semgrep binary to perform static analysis on
// package source archives. It extracts the archive into a temp directory,
// runs semgrep with the "auto" rule set, and maps findings to core.Finding.
package semgrep

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mah3sec/forgeguardian/internal/core"
)

// Scanner runs Semgrep static analysis on extracted package source.
type Scanner struct {
	// BinaryPath overrides the semgrep binary location. Empty = use PATH.
	BinaryPath string
	// Rules is the semgrep rule spec. Defaults to "auto".
	Rules string
}

// New returns a new Semgrep Scanner.
func New() *Scanner { return &Scanner{Rules: "auto"} }

// Name implements core.Scanner.
func (s *Scanner) Name() string { return "semgrep" }

// Scan extracts the artifact archive into a temp directory, runs semgrep,
// and returns static analysis findings.
func (s *Scanner) Scan(ctx context.Context, artifact core.BuiltArtifact) ([]core.Finding, error) {
	bin := s.binary()
	if _, err := exec.LookPath(bin); err != nil {
		return []core.Finding{notInstalled(s.Name())}, nil
	}

	// Extract archive to temp dir
	dir, err := os.MkdirTemp("", "fg-semgrep-*")
	if err != nil {
		return nil, fmt.Errorf("semgrep: create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := extract(artifact.LocalPath, dir); err != nil {
		// If we can't extract (binary format), skip gracefully
		return []core.Finding{skipFinding(s.Name(), fmt.Sprintf("cannot extract %s for analysis", filepath.Ext(artifact.LocalPath)))}, nil
	}

	rules := s.Rules
	if rules == "" {
		rules = "auto"
	}

	cmd := exec.CommandContext(ctx, bin,
		"--config", rules,
		"--json",
		"--quiet",
		"--no-git-ignore",
		"--timeout", "60",
		dir,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_ = cmd.Run() // semgrep exits non-zero when findings exist

	if stdout.Len() == 0 {
		// No output at all — semgrep may have errored
		errStr := strings.TrimSpace(stderr.String())
		if errStr != "" {
			return []core.Finding{skipFinding(s.Name(), "semgrep produced no output: "+truncate(errStr, 200))}, nil
		}
		return nil, nil
	}

	return parseSemgrepOutput(stdout.Bytes(), artifact)
}

// --- Semgrep JSON output types ---

type semgrepReport struct {
	Results []semgrepResult `json:"results"`
	Errors  []semgrepError  `json:"errors"`
}

type semgrepResult struct {
	CheckID string       `json:"check_id"`
	Path    string       `json:"path"`
	Start   semgrepPos   `json:"start"`
	Extra   semgrepExtra `json:"extra"`
}

type semgrepPos struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}

type semgrepExtra struct {
	Message  string      `json:"message"`
	Severity string      `json:"severity"`
	Metadata semgrepMeta `json:"metadata"`
}

type semgrepMeta struct {
	OWASP      []string `json:"owasp"`
	CWE        []string `json:"cwe"`
	Confidence string   `json:"confidence"`
	Impact     string   `json:"impact"`
	References []string `json:"references"`
}

type semgrepError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func parseSemgrepOutput(data []byte, artifact core.BuiltArtifact) ([]core.Finding, error) {
	var report semgrepReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("semgrep: parse output: %w", err)
	}

	findings := make([]core.Finding, 0, len(report.Results))
	for _, r := range report.Results {
		sev := mapSemgrepSeverity(r.Extra.Severity, r.Extra.Metadata.Impact)
		relPath := strings.TrimPrefix(r.Path, string(os.PathSeparator))

		meta := map[string]any{
			"check_id":   r.CheckID,
			"file":       relPath,
			"line":       r.Start.Line,
			"owasp":      r.Extra.Metadata.OWASP,
			"cwe":        r.Extra.Metadata.CWE,
			"confidence": r.Extra.Metadata.Confidence,
			"ecosystem":  artifact.Source.Package.Ecosystem,
		}

		findings = append(findings, core.Finding{
			ID:          r.CheckID,
			Severity:    sev,
			Type:        "static",
			Title:       fmt.Sprintf("%s in %s:%d", shortRuleID(r.CheckID), filepath.Base(relPath), r.Start.Line),
			Description: r.Extra.Message,
			Source:      "semgrep",
			Metadata:    meta,
		})
	}
	return findings, nil
}

func mapSemgrepSeverity(sev, impact string) core.Severity {
	// Semgrep uses ERROR/WARNING/INFO; also check impact field
	switch strings.ToUpper(sev) {
	case "ERROR":
		if strings.ToUpper(impact) == "HIGH" {
			return core.SeverityCritical
		}
		return core.SeverityHigh
	case "WARNING":
		return core.SeverityMedium
	default:
		return core.SeverityInformational
	}
}

func shortRuleID(checkID string) string {
	parts := strings.Split(checkID, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return checkID
}

// --- Archive extraction ---

func extract(archivePath, destDir string) error {
	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".tgz") || strings.HasSuffix(lower, ".tar.gz"):
		return extractTarGz(archivePath, destDir)
	case strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".jar") ||
		strings.HasSuffix(lower, ".whl"):
		return extractZip(archivePath, destDir)
	case strings.HasSuffix(lower, ".crate"):
		return extractTarGz(archivePath, destDir)
	case strings.HasSuffix(lower, ".gem"):
		return extractTarGz(archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive type: %s", filepath.Ext(archivePath))
	}
}

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		// might be plain tar (e.g. inside gem)
		f.Seek(0, io.SeekStart)
		return extractTar(f, destDir)
	}
	defer gr.Close()
	return extractTar(gr, destDir)
}

func extractTar(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Strip the first path component (e.g. "package/") for cleaner analysis
		parts := strings.SplitN(hdr.Name, "/", 2)
		name := hdr.Name
		if len(parts) == 2 {
			name = parts[1]
		}
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		outPath := filepath.Join(destDir, filepath.FromSlash(name))
		if hdr.Typeflag == tar.TypeDir {
			os.MkdirAll(outPath, 0o750)
			continue
		}
		if err := writeFile(outPath, tr, hdr.Size); err != nil {
			return err
		}
	}
	return nil
}

func extractZip(archivePath, destDir string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if strings.Contains(f.Name, "..") {
			continue
		}
		outPath := filepath.Join(destDir, filepath.FromSlash(f.Name))
		if f.FileInfo().IsDir() {
			os.MkdirAll(outPath, 0o750)
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = writeFile(outPath, rc, int64(f.UncompressedSize64))
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func writeFile(path string, r io.Reader, size int64) error {
	if size > 10*1024*1024 { // skip files > 10 MB
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, io.LimitReader(r, 10*1024*1024))
	return err
}

func (s *Scanner) binary() string {
	if s.BinaryPath != "" {
		return s.BinaryPath
	}
	return "semgrep"
}

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

func skipFinding(name, reason string) core.Finding {
	return core.Finding{
		ID:          "SCAN-SKIPPED",
		Severity:    core.SeverityInformational,
		Type:        "configuration",
		Title:       fmt.Sprintf("%s scan skipped: %s", name, reason),
		Description: reason,
		Source:      name,
		Metadata:    map[string]any{"skipped": true},
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
