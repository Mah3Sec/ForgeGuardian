package core

import (
	"testing"
)

func TestScoreFindings_Empty(t *testing.T) {
	s := ScoreFindings(nil)
	if s.Overall != 0 {
		t.Errorf("empty findings: want overall=0, got %d", s.Overall)
	}
	if s.Grade != "A" {
		t.Errorf("empty findings: want grade=A, got %s", s.Grade)
	}
}

func TestScoreFindings_CriticalCVE(t *testing.T) {
	findings := []Finding{
		{Type: "cve", Severity: SeverityCritical},
	}
	s := ScoreFindings(findings)
	if s.Factors.Vulnerability != 40 {
		t.Errorf("critical cve: want vulnerability=40, got %d", s.Factors.Vulnerability)
	}
	if s.Overall != 40 {
		t.Errorf("critical cve: want overall=40, got %d", s.Overall)
	}
	if s.Grade != "B" {
		t.Errorf("critical cve: want grade=B, got %s", s.Grade)
	}
}

func TestScoreFindings_VulnCapAt40(t *testing.T) {
	// Two CRITICAL CVEs would be 80 but vuln is capped at 40.
	findings := []Finding{
		{Type: "cve", Severity: SeverityCritical},
		{Type: "cve", Severity: SeverityCritical},
	}
	s := ScoreFindings(findings)
	if s.Factors.Vulnerability != 40 {
		t.Errorf("vuln cap: want 40, got %d", s.Factors.Vulnerability)
	}
}

func TestScoreFindings_Malware(t *testing.T) {
	findings := []Finding{
		{Type: "malware", Severity: SeverityCritical},
	}
	s := ScoreFindings(findings)
	if s.Factors.Behavioral != 30 {
		t.Errorf("malware: want behavioral=30, got %d", s.Factors.Behavioral)
	}
	if s.Grade != "B" {
		t.Errorf("malware: want grade=B, got %s", s.Grade)
	}
}

func TestScoreFindings_MaxScore(t *testing.T) {
	// One of every type at max values.
	findings := []Finding{
		{Type: "cve", Severity: SeverityCritical},
		{Type: "malware", Severity: SeverityCritical},
		{Type: "supply-chain", Severity: SeverityCritical},
		{Type: "supply-chain", Severity: SeverityInformational},
	}
	s := ScoreFindings(findings)
	if s.Overall > 100 {
		t.Errorf("score must be <=100, got %d", s.Overall)
	}
	if s.Grade != "F" {
		t.Errorf("max score: want grade=F, got %s", s.Grade)
	}
}

func TestScoreFindings_GradeBoundaries(t *testing.T) {
	cases := []struct {
		score int
		want  string
	}{
		{0, "A"}, {20, "A"},
		{21, "B"}, {40, "B"},
		{41, "C"}, {60, "C"},
		{61, "D"}, {80, "D"},
		{81, "F"}, {100, "F"},
	}
	for _, tc := range cases {
		got := scoreGrade(tc.score)
		if got != tc.want {
			t.Errorf("scoreGrade(%d): want %s, got %s", tc.score, tc.want, got)
		}
	}
}

func TestScoreFindings_HighCVE(t *testing.T) {
	findings := []Finding{
		{Type: "cve", Severity: SeverityHigh},
	}
	s := ScoreFindings(findings)
	if s.Factors.Vulnerability != 20 {
		t.Errorf("high cve: want vulnerability=20, got %d", s.Factors.Vulnerability)
	}
}

func TestScoreFindings_BehavioralSeverities(t *testing.T) {
	cases := []struct {
		sev  Severity
		want int
	}{
		{SeverityCritical, 25},
		{SeverityHigh, 15},
		{SeverityMedium, 5},
	}
	for _, tc := range cases {
		findings := []Finding{{Type: "behavioral", Severity: tc.sev}}
		s := ScoreFindings(findings)
		if s.Factors.Behavioral != tc.want {
			t.Errorf("behavioral %s: want %d, got %d", tc.sev, tc.want, s.Factors.Behavioral)
		}
	}
}

func TestScoreFindings_SupplyChainCap(t *testing.T) {
	// Three critical supply-chain = 60 but capped at 20.
	findings := []Finding{
		{Type: "supply-chain", Severity: SeverityCritical},
		{Type: "supply-chain", Severity: SeverityCritical},
		{Type: "supply-chain", Severity: SeverityCritical},
	}
	s := ScoreFindings(findings)
	if s.Factors.SupplyChain != 20 {
		t.Errorf("supply-chain cap: want 20, got %d", s.Factors.SupplyChain)
	}
}
