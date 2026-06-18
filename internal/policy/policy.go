// Package policy implements policy-as-code enforcement for ForgeGuardian scans.
// Policies are stored in ~/.forgeguardian/policy.yaml and applied after scanning.
package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mah3sec/forgeguardian/internal/core"
	"gopkg.in/yaml.v3"
)

// Policy defines security enforcement rules applied after every scan.
type Policy struct {
	Version            int      `yaml:"version"              json:"version"`
	FailOn             string   `yaml:"fail_on"               json:"fail_on"`              // severity threshold: "critical"|"high"|"medium"|"low"|""
	DenyPackages       []string `yaml:"deny_packages"         json:"deny_packages"`        // exact package names to block
	AllowLicenses      []string `yaml:"allow_licenses"        json:"allow_licenses"`       // permitted SPDX license identifiers
	MaxPackageAgeDays  int      `yaml:"max_package_age_days"  json:"max_package_age_days"` // 0 = no limit
	BlockTyposquatting bool     `yaml:"block_typosquatting"   json:"block_typosquatting"`
	BlockAbandoned     bool     `yaml:"block_abandoned"       json:"block_abandoned"`
	RequireSigning     bool     `yaml:"require_signing"       json:"require_signing"`
}

// DefaultPolicyPath returns the canonical path to the policy file.
func DefaultPolicyPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forgeguardian", "policy.yaml")
}

// DefaultPolicy returns a permissive policy with sensible defaults.
func DefaultPolicy() *Policy {
	return &Policy{
		Version:       1,
		AllowLicenses: []string{"MIT", "Apache-2.0", "BSD-3-Clause", "BSD-2-Clause", "ISC"},
	}
}

// Load reads the policy file. Returns (nil, nil) if the file does not exist —
// callers treat nil as "no policy enforcement".
func Load() (*Policy, error) {
	data, err := os.ReadFile(DefaultPolicyPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("policy: read: %w", err)
	}
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("policy: parse: %w", err)
	}
	return &p, nil
}

// Save writes the policy to DefaultPolicyPath, creating parent dirs as needed.
func Save(p *Policy) error {
	path := DefaultPolicyPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("policy: mkdir: %w", err)
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("policy: marshal: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// Enforce evaluates the policy against a package and its findings.
// It appends POLICY-* findings for any violations and returns the augmented slice.
func (pol *Policy) Enforce(pkgName, pkgVersion string, findings []core.Finding) []core.Finding {
	if pol == nil {
		return findings
	}

	// Check denied packages
	for _, denied := range pol.DenyPackages {
		if strings.EqualFold(pkgName, denied) {
			findings = append(findings, core.Finding{
				ID:       "POLICY-DENIED-PACKAGE",
				Severity: core.SeverityCritical,
				Type:     "policy",
				Title:    fmt.Sprintf("Package %q is denied by policy", pkgName),
				Description: fmt.Sprintf(
					"%s@%s is explicitly blocked by your policy (deny_packages). "+
						"Remove the dependency or update policy.yaml to allow it.",
					pkgName, pkgVersion,
				),
				Source: "policy",
			})
		}
	}

	// Check typosquatting enforcement
	if pol.BlockTyposquatting {
		for _, f := range findings {
			if f.ID == "BEHAVIORAL-TYPOSQUAT" && f.Source == "behavioral" {
				findings = append(findings, core.Finding{
					ID:       "POLICY-TYPOSQUAT-BLOCKED",
					Severity: core.SeverityCritical,
					Type:     "policy",
					Title:    fmt.Sprintf("Typosquatting detected — blocked by policy"),
					Description: fmt.Sprintf(
						"%s@%s appears to be a typosquat. Policy block_typosquatting=true prevents use.",
						pkgName, pkgVersion,
					),
					Source: "policy",
				})
				break
			}
		}
	}

	return findings
}

// ShouldFail returns true if any finding's severity meets or exceeds the
// policy fail_on threshold (used for exit-code enforcement).
func (pol *Policy) ShouldFail(findings []core.Finding) bool {
	if pol == nil || pol.FailOn == "" {
		return false
	}
	threshold := severityOrd(core.Severity(strings.ToUpper(pol.FailOn)))
	for _, f := range findings {
		if severityOrd(f.Severity) >= threshold {
			return true
		}
	}
	return false
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
