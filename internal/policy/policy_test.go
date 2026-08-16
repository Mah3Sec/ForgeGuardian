package policy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mah3sec/forgeguardian/internal/core"
)

// ── Enforce ───────────────────────────────────────────────────────────────────

func TestEnforce_NilPolicy_Passthrough(t *testing.T) {
	var pol *Policy
	findings := []core.Finding{{ID: "x", Severity: core.SeverityCritical}}
	got := pol.Enforce("pkg", "1.0.0", findings)
	if len(got) != 1 {
		t.Errorf("nil policy: want 1 finding, got %d", len(got))
	}
}

func TestEnforce_DeniedPackage_AddsViolation(t *testing.T) {
	pol := &Policy{DenyPackages: []string{"evil-pkg"}}
	findings := pol.Enforce("evil-pkg", "1.0.0", nil)
	if len(findings) != 1 {
		t.Fatalf("want 1 policy finding, got %d", len(findings))
	}
	if findings[0].ID != "POLICY-DENIED-PACKAGE" {
		t.Errorf("want POLICY-DENIED-PACKAGE, got %s", findings[0].ID)
	}
	if findings[0].Severity != core.SeverityCritical {
		t.Errorf("denied package finding must be CRITICAL")
	}
}

func TestEnforce_DeniedPackage_CaseInsensitive(t *testing.T) {
	pol := &Policy{DenyPackages: []string{"Evil-Pkg"}}
	findings := pol.Enforce("evil-pkg", "1.0.0", nil)
	if len(findings) != 1 {
		t.Errorf("case-insensitive match: want 1 finding, got %d", len(findings))
	}
}

func TestEnforce_AllowedPackage_NoViolation(t *testing.T) {
	pol := &Policy{DenyPackages: []string{"evil-pkg"}}
	findings := pol.Enforce("safe-pkg", "1.0.0", nil)
	if len(findings) != 0 {
		t.Errorf("safe package: want 0 findings, got %d", len(findings))
	}
}

func TestEnforce_TyposquatBlocked(t *testing.T) {
	pol := &Policy{BlockTyposquatting: true}
	in := []core.Finding{{ID: "BEHAVIORAL-TYPOSQUAT", Source: "behavioral", Severity: core.SeverityHigh}}
	out := pol.Enforce("lodahs", "1.0.0", in)
	found := false
	for _, f := range out {
		if f.ID == "POLICY-TYPOSQUAT-BLOCKED" {
			found = true
		}
	}
	if !found {
		t.Error("typosquat block: expected POLICY-TYPOSQUAT-BLOCKED finding")
	}
}

func TestEnforce_TyposquatNotBlocked_WhenFlagFalse(t *testing.T) {
	pol := &Policy{BlockTyposquatting: false}
	in := []core.Finding{{ID: "BEHAVIORAL-TYPOSQUAT", Source: "behavioral", Severity: core.SeverityHigh}}
	out := pol.Enforce("lodahs", "1.0.0", in)
	for _, f := range out {
		if f.ID == "POLICY-TYPOSQUAT-BLOCKED" {
			t.Error("typosquat should not be blocked when flag is false")
		}
	}
}

// ── ShouldFail ────────────────────────────────────────────────────────────────

func TestShouldFail_NilPolicy(t *testing.T) {
	var pol *Policy
	if pol.ShouldFail([]core.Finding{{Severity: core.SeverityCritical}}) {
		t.Error("nil policy: ShouldFail must return false")
	}
}

func TestShouldFail_NoThreshold(t *testing.T) {
	pol := &Policy{FailOn: ""}
	if pol.ShouldFail([]core.Finding{{Severity: core.SeverityCritical}}) {
		t.Error("empty FailOn: ShouldFail must return false")
	}
}

func TestShouldFail_CriticalThreshold_WithCritical(t *testing.T) {
	pol := &Policy{FailOn: "critical"}
	if !pol.ShouldFail([]core.Finding{{Severity: core.SeverityCritical}}) {
		t.Error("critical finding at critical threshold: ShouldFail must return true")
	}
}

func TestShouldFail_CriticalThreshold_WithHigh(t *testing.T) {
	pol := &Policy{FailOn: "critical"}
	if pol.ShouldFail([]core.Finding{{Severity: core.SeverityHigh}}) {
		t.Error("high finding at critical threshold: ShouldFail must return false")
	}
}

func TestShouldFail_HighThreshold_WithCritical(t *testing.T) {
	pol := &Policy{FailOn: "high"}
	if !pol.ShouldFail([]core.Finding{{Severity: core.SeverityCritical}}) {
		t.Error("critical finding at high threshold: ShouldFail must return true")
	}
}

func TestShouldFail_HighThreshold_WithMedium(t *testing.T) {
	pol := &Policy{FailOn: "high"}
	if pol.ShouldFail([]core.Finding{{Severity: core.SeverityMedium}}) {
		t.Error("medium finding at high threshold: ShouldFail must return false")
	}
}

// ── severityOrd ───────────────────────────────────────────────────────────────

func TestSeverityOrd_Order(t *testing.T) {
	if severityOrd(core.SeverityCritical) <= severityOrd(core.SeverityHigh) {
		t.Error("critical must rank higher than high")
	}
	if severityOrd(core.SeverityHigh) <= severityOrd(core.SeverityMedium) {
		t.Error("high must rank higher than medium")
	}
	if severityOrd(core.SeverityMedium) <= severityOrd(core.SeverityLow) {
		t.Error("medium must rank higher than low")
	}
	if severityOrd("unknown") != 0 {
		t.Error("unknown severity must map to 0")
	}
}

// ── Save / Load ───────────────────────────────────────────────────────────────

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".forgeguardian", "policy.yaml")

	// Override the default path via env.
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	pol := &Policy{
		Version:            1,
		FailOn:             "high",
		DenyPackages:       []string{"evil-pkg", "bad-lib"},
		BlockTyposquatting: true,
	}
	if err := Save(pol); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("policy file not created at %s", path)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.FailOn != pol.FailOn {
		t.Errorf("FailOn: want %q, got %q", pol.FailOn, loaded.FailOn)
	}
	if len(loaded.DenyPackages) != 2 {
		t.Errorf("DenyPackages: want 2, got %d", len(loaded.DenyPackages))
	}
	if !loaded.BlockTyposquatting {
		t.Error("BlockTyposquatting: want true")
	}
}

func TestLoad_MissingFile_ReturnsNil(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	pol, err := Load()
	if err != nil {
		t.Fatalf("Load on missing file: want nil err, got %v", err)
	}
	if pol != nil {
		t.Error("Load on missing file: want nil policy")
	}
}
