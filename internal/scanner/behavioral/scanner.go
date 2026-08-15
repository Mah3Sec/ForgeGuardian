// Package behavioral analyzes build logs and file manifests for supply chain
// attack signals without requiring any external binary.
// It covers: install scripts, typosquatting, network activity, lifecycle hook
// abuse, dependency confusion indicators, and suspicious file patterns.
package behavioral

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/intelligence"
)

// Scanner performs behavioral analysis on build artifacts.
type Scanner struct {
	store *intelligence.SignatureStore
}

// New returns a behavioral Scanner using the built-in hardcoded rules only.
func New() *Scanner { return &Scanner{} }

// NewWithStore returns a behavioral Scanner that loads typosquat targets and
// behavioral rules from the given intelligence store in addition to built-ins.
func NewWithStore(store *intelligence.SignatureStore) *Scanner {
	return &Scanner{store: store}
}

// Name implements core.Scanner.
func (s *Scanner) Name() string { return "behavioral" }

// Scan analyzes the build log and artifact metadata for behavioral risk signals.
func (s *Scanner) Scan(_ context.Context, artifact core.BuiltArtifact) ([]core.Finding, error) {
	var findings []core.Finding
	log := artifact.BuildLog
	pkg := artifact.Source.Package

	findings = append(findings, s.checkInstallScripts(log, pkg)...)
	findings = append(findings, s.checkNetworkActivity(log, pkg)...)
	findings = append(findings, s.checkTyposquatting(pkg)...)
	findings = append(findings, s.checkFileManifest(log, pkg)...)
	findings = append(findings, s.checkDependencyConfusion(pkg)...)
	findings = append(findings, s.checkVersionAnomaly(pkg)...)

	return findings, nil
}

// --- Install script detection ---

func (s *Scanner) checkInstallScripts(log string, pkg core.PackageVersion) []core.Finding {
	var findings []core.Finding
	scripts := extractField(log, "install_scripts")
	if scripts == "[]" || scripts == "" {
		return nil
	}

	findings = append(findings, core.Finding{
		ID:       "BEHAVIORAL-INSTALL-SCRIPT",
		Severity: core.SeverityHigh,
		Type:     "behavioral",
		Title:    fmt.Sprintf("%s@%s has install lifecycle scripts", pkg.Name, pkg.Version),
		Description: fmt.Sprintf(
			"Package declares install-time scripts that execute arbitrary code during npm install: %s. "+
				"Review these scripts before installation.", scripts),
		Source: "behavioral",
		Metadata: map[string]any{
			"install_scripts": scripts,
			"ecosystem":       pkg.Ecosystem,
		},
	})

	// Escalate if scripts contain known malicious patterns (built-in + store-loaded)
	for _, pattern := range s.scriptPatterns() {
		if strings.Contains(strings.ToLower(scripts), pattern) {
			findings = append(findings, core.Finding{
				ID:       "BEHAVIORAL-MALICIOUS-INSTALL-SCRIPT",
				Severity: core.SeverityCritical,
				Type:     "malware",
				Title:    fmt.Sprintf("%s@%s install script matches malicious pattern: %q", pkg.Name, pkg.Version, pattern),
				Description: fmt.Sprintf(
					"Install script contains pattern %q commonly used in supply chain attacks. "+
						"Do NOT install this package.", pattern),
				Source: "behavioral",
				Metadata: map[string]any{
					"pattern":   pattern,
					"scripts":   scripts,
					"ecosystem": pkg.Ecosystem,
				},
			})
		}
	}

	return findings
}

var builtinScriptPatterns = []string{
	"curl ", "wget ", "fetch(",
	"base64", "atob(", "btoa(",
	"child_process", "exec(", "spawn(",
	"eval(", "function(", "require(",
	"/tmp/", "os.system", "subprocess",
	"powershell", "cmd.exe", "/bin/sh",
	"reverse", "shell", "backdoor",
	"exfil", "keylog", "credential",
}

// scriptPatterns returns the combined set of built-in and store-loaded patterns.
func (s *Scanner) scriptPatterns() []string {
	patterns := builtinScriptPatterns
	if s.store == nil {
		return patterns
	}
	for _, sig := range s.store.Signatures {
		if sig.Type == intelligence.SigBehavioral && sig.Rule != "" {
			patterns = append(patterns, sig.Rule)
		}
	}
	return patterns
}

// --- Network activity detection ---

func (s *Scanner) checkNetworkActivity(log string, pkg core.PackageVersion) []core.Finding {
	netConns := extractField(log, "network_connections")
	// Our build engine always reports 0 — a non-zero value means the sandbox was bypassed
	if netConns == "0" || netConns == "" {
		return nil
	}
	return []core.Finding{{
		ID:       "BEHAVIORAL-NETWORK-ACTIVITY",
		Severity: core.SeverityCritical,
		Type:     "behavioral",
		Title:    fmt.Sprintf("%s@%s made network connections during build: %s", pkg.Name, pkg.Version, netConns),
		Description: "Package initiated unexpected network connections during the hermetic build. " +
			"This is a strong indicator of a supply chain attack (e.g. data exfiltration, C2 beacon).",
		Source: "behavioral",
		Metadata: map[string]any{
			"network_connections": netConns,
			"ecosystem":           pkg.Ecosystem,
		},
	}}
}

// --- Typosquatting detection ---

// popularPackages maps ecosystem to a list of commonly typosquatted package names.
var popularPackages = map[string][]string{
	"npm": {
		"lodash", "express", "react", "axios", "moment", "chalk", "commander",
		"webpack", "babel-core", "typescript", "eslint", "jest", "mocha",
		"request", "underscore", "async", "bluebird", "debug",
	},
	"pypi": {
		"requests", "numpy", "pandas", "flask", "django", "boto3", "pillow",
		"cryptography", "setuptools", "pip", "wheel", "urllib3", "certifi",
		"six", "click", "pydantic", "fastapi", "httpx", "aiohttp",
	},
	"crates": {
		"serde", "tokio", "rand", "log", "clap", "anyhow", "thiserror",
		"reqwest", "hyper", "axum", "actix-web", "diesel", "sqlx",
	},
}

// typosquatTargets returns built-in + store-loaded targets for the given ecosystem.
func (s *Scanner) typosquatTargets(ecosystem string) []string {
	targets := popularPackages[ecosystem]
	if s.store == nil {
		return targets
	}
	for _, sig := range s.store.Signatures {
		if sig.Type == intelligence.SigTypoTarget &&
			(sig.Ecosystem == ecosystem || sig.Ecosystem == "*") &&
			sig.Target != "" {
			targets = append(targets, sig.Target)
		}
	}
	return targets
}

func (s *Scanner) checkTyposquatting(pkg core.PackageVersion) []core.Finding {
	targets := s.typosquatTargets(pkg.Ecosystem)
	if len(targets) == 0 {
		return nil
	}
	name := strings.ToLower(pkg.Name)
	for _, target := range targets {
		if name == target {
			return nil // exact match — not a typosquat
		}
		if editDistance(name, target) == 1 && len(name) > 3 {
			return []core.Finding{{
				ID:       "BEHAVIORAL-TYPOSQUAT",
				Severity: core.SeverityHigh,
				Type:     "behavioral",
				Title:    fmt.Sprintf("%q is one edit away from popular package %q", pkg.Name, target),
				Description: fmt.Sprintf(
					"Package name %q closely resembles the well-known package %q (edit distance 1). "+
						"This is a common typosquatting pattern used in supply chain attacks. "+
						"Verify you intended to install this package.",
					pkg.Name, target),
				Source: "behavioral",
				Metadata: map[string]any{
					"suspected_target": target,
					"edit_distance":    1,
					"ecosystem":        pkg.Ecosystem,
				},
			}}
		}
	}
	return nil
}

// editDistance computes the Levenshtein distance between two strings.
func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	m, n := len(ra), len(rb)
	dp := make([]int, n+1)
	for j := range dp {
		dp[j] = j
	}
	for i := 1; i <= m; i++ {
		prev := i
		for j := 1; j <= n; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			next := min3(dp[j]+1, prev+1, dp[j-1]+cost)
			dp[j-1] = prev
			prev = next
		}
		dp[n] = prev
	}
	return dp[n]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// --- File manifest analysis ---

var suspiciousExtensions = []string{
	".exe", ".dll", ".so", ".dylib", ".bat", ".cmd", ".ps1",
	".sh", ".bash", ".zsh", ".fish",
}

var suspiciousFilenames = []string{
	".env", ".ssh", "id_rsa", "id_ed25519", "credentials", "secrets",
	"password", "token", "api_key", "private_key",
}

func (s *Scanner) checkFileManifest(log string, pkg core.PackageVersion) []core.Finding {
	var findings []core.Finding
	var execFiles, credFiles []string

	for _, line := range strings.Split(log, "\n") {
		if !strings.HasPrefix(line, "file: ") {
			continue
		}
		filePath := strings.TrimPrefix(line, "file: ")
		base := strings.ToLower(filepath.Base(filePath))
		ext := strings.ToLower(filepath.Ext(filePath))

		for _, se := range suspiciousExtensions {
			if ext == se {
				execFiles = append(execFiles, filePath)
				break
			}
		}
		for _, sf := range suspiciousFilenames {
			if strings.Contains(base, sf) {
				credFiles = append(credFiles, filePath)
				break
			}
		}
	}

	if len(execFiles) > 0 {
		findings = append(findings, core.Finding{
			ID:       "BEHAVIORAL-EXECUTABLE-FILES",
			Severity: core.SeverityHigh,
			Type:     "behavioral",
			Title:    fmt.Sprintf("%s@%s contains executable files: %v", pkg.Name, pkg.Version, execFiles[:minLen(execFiles, 3)]),
			Description: fmt.Sprintf(
				"Package contains %d executable or native binary files. "+
					"Legitimate libraries rarely ship pre-compiled binaries. "+
					"Verify these files are expected.", len(execFiles)),
			Source: "behavioral",
			Metadata: map[string]any{
				"executable_files": execFiles,
				"count":            len(execFiles),
				"ecosystem":        pkg.Ecosystem,
			},
		})
	}

	if len(credFiles) > 0 {
		findings = append(findings, core.Finding{
			ID:       "BEHAVIORAL-CREDENTIAL-FILES",
			Severity: core.SeverityCritical,
			Type:     "malware",
			Title:    fmt.Sprintf("%s@%s contains files with credential-related names", pkg.Name, pkg.Version),
			Description: fmt.Sprintf(
				"Package contains files with names suggesting credentials or secrets: %v. "+
					"This is a severe supply chain attack indicator.", credFiles),
			Source: "behavioral",
			Metadata: map[string]any{
				"credential_files": credFiles,
				"ecosystem":        pkg.Ecosystem,
			},
		})
	}

	return findings
}

// --- Dependency confusion detection ---

func (s *Scanner) checkDependencyConfusion(pkg core.PackageVersion) []core.Finding {
	// Heuristic: internal packages often have org-scoped names or contain
	// common internal naming patterns but appear on the public registry
	name := pkg.Name
	if pkg.Ecosystem == "npm" {
		// Unscoped package with internal-sounding names
		internal := []string{"internal", "private", "corp", "company", "org-", "lib-"}
		nameLower := strings.ToLower(name)
		for _, indicator := range internal {
			if strings.Contains(nameLower, indicator) {
				return []core.Finding{{
					ID:       "BEHAVIORAL-DEPENDENCY-CONFUSION",
					Severity: core.SeverityMedium,
					Type:     "behavioral",
					Title:    fmt.Sprintf("%s@%s may be a dependency confusion target", pkg.Name, pkg.Version),
					Description: fmt.Sprintf(
						"Package name %q contains indicators of an internal/private package name "+
							"published to the public registry. Verify this is the intended package "+
							"and not a dependency confusion attack.", pkg.Name),
					Source: "behavioral",
					Metadata: map[string]any{
						"indicator": indicator,
						"ecosystem": pkg.Ecosystem,
					},
				}}
			}
		}
	}
	return nil
}

// --- Version anomaly detection ---

func (s *Scanner) checkVersionAnomaly(pkg core.PackageVersion) []core.Finding {
	version := pkg.Version
	// Flag very high major versions for popular packages as potentially hijacked
	// e.g. lodash@99.0.0 or requests@100.0.0
	var major int
	fmt.Sscanf(strings.TrimPrefix(version, "v"), "%d.", &major)
	if major >= 50 {
		return []core.Finding{{
			ID:       "BEHAVIORAL-VERSION-ANOMALY",
			Severity: core.SeverityMedium,
			Type:     "behavioral",
			Title:    fmt.Sprintf("%s@%s has an unusually high major version", pkg.Name, pkg.Version),
			Description: fmt.Sprintf(
				"Version %q has an unusually large major version number (>= 50). "+
					"This pattern is sometimes used in package hijacking attacks to ensure "+
					"semver resolution picks the malicious version.", version),
			Source: "behavioral",
			Metadata: map[string]any{
				"major_version": major,
				"ecosystem":     pkg.Ecosystem,
			},
		}}
	}

	// Flag versions with non-printable or homoglyph characters
	for _, r := range version {
		if !unicode.IsPrint(r) {
			return []core.Finding{{
				ID:          "BEHAVIORAL-VERSION-HOMOGLYPH",
				Severity:    core.SeverityCritical,
				Type:        "malware",
				Title:       fmt.Sprintf("%s@%s contains non-printable characters in version", pkg.Name, pkg.Version),
				Description: "Version string contains non-printable characters — likely a homoglyph or injection attack.",
				Source:      "behavioral",
				Metadata:    map[string]any{"ecosystem": pkg.Ecosystem},
			}}
		}
	}

	return nil
}

// --- helpers ---

func extractField(log, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(log, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(t, prefix))
		}
	}
	return ""
}

func minLen(s []string, n int) int {
	if len(s) < n {
		return len(s)
	}
	return n
}
