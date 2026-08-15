// Package core defines the shared interfaces and types used across
// all ForgeGuardian components.
package core

import (
	"context"
	"encoding/json"
	"time"
)

// RegistryScraper watches an upstream package registry for new releases.
type RegistryScraper interface {
	// Name returns the ecosystem identifier (e.g. "npm", "pypi", "huggingface").
	Name() string
	// Poll checks for new package versions published after lastRun.
	Poll(ctx context.Context, lastRun time.Time) ([]PackageVersion, error)
	// FetchSource downloads the canonical source artifact for a PackageVersion.
	FetchSource(ctx context.Context, pkg PackageVersion) (SourceArtifact, error)
	// VerifyIntegrity validates the downloaded source against registry-provided hashes.
	VerifyIntegrity(ctx context.Context, src SourceArtifact) error
}

// PackageVersion represents a single versioned release of a package.
type PackageVersion struct {
	Ecosystem   string         `json:"ecosystem"`
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	SourceURL   string         `json:"source_url"`
	Checksum    string         `json:"checksum"` // sha256 from registry
	PublishedAt time.Time      `json:"published_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// packageVersionJSON mirrors PackageVersion but with PublishedAt as a pointer
// so callers that never set it (e.g. the sign/provenance endpoints, which
// only know ecosystem/name/version) omit the field instead of marshaling
// Go's zero time.Time as the misleading literal "0001-01-01T00:00:00Z".
type packageVersionJSON struct {
	Ecosystem   string         `json:"ecosystem"`
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	SourceURL   string         `json:"source_url,omitempty"`
	Checksum    string         `json:"checksum,omitempty"`
	PublishedAt *time.Time     `json:"published_at,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// MarshalJSON implements json.Marshaler.
func (p PackageVersion) MarshalJSON() ([]byte, error) {
	out := packageVersionJSON{
		Ecosystem: p.Ecosystem,
		Name:      p.Name,
		Version:   p.Version,
		SourceURL: p.SourceURL,
		Checksum:  p.Checksum,
		Metadata:  p.Metadata,
	}
	if !p.PublishedAt.IsZero() {
		out.PublishedAt = &p.PublishedAt
	}
	return json.Marshal(out)
}

// SourceArtifact is a downloaded, locally-cached source archive.
type SourceArtifact struct {
	Package   PackageVersion `json:"package"`
	LocalPath string         `json:"local_path"`
	Size      int64          `json:"size"`
	SHA256    string         `json:"sha256"`
}

// BuiltArtifact is the output of the hermetic build engine.
type BuiltArtifact struct {
	Source       SourceArtifact `json:"source"`
	LocalPath    string         `json:"local_path"`
	SHA256       string         `json:"sha256"`
	BuildLog     string         `json:"build_log"`
	BuildTime    time.Time      `json:"build_time"`
	Reproducible *bool          `json:"reproducible,omitempty"`
}

// Severity represents the risk level of a security finding.
type Severity string

const (
	SeverityCritical      Severity = "CRITICAL"
	SeverityHigh          Severity = "HIGH"
	SeverityMedium        Severity = "MEDIUM"
	SeverityLow           Severity = "LOW"
	SeverityInformational Severity = "INFORMATIONAL"
)

// Finding represents a single security finding from any scanner.
type Finding struct {
	ID           string         `json:"id"`
	Severity     Severity       `json:"severity"`
	Type         string         `json:"type"` // "cve", "static", "behavioral", "malware"
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Source       string         `json:"source"`                  // "grype", "semgrep", "behavioral", etc.
	FixedVersion string         `json:"fixed_version,omitempty"` // known safe version, e.g. "4.17.21"
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// Advisory is the AI-generated, human-readable security advisory for a package version.
type Advisory struct {
	Package                 PackageVersion `json:"package"`
	Severity                Severity       `json:"severity"`
	Confidence              float64        `json:"confidence"`
	Advisory                string         `json:"advisory"`
	ExploitabilityRationale string         `json:"exploitability_rationale"`
	AgenticRisk             *string        `json:"agentic_risk,omitempty"`
	RecommendedAction       string         `json:"recommended_action"`
	PatchSuggestion         *string        `json:"patch_suggestion,omitempty"`
	Findings                []Finding      `json:"findings"`
	GeneratedAt             time.Time      `json:"generated_at"`
}

// RiskFactors are the component scores that make up an overall risk score.
type RiskFactors struct {
	Vulnerability int // weighted CVE severity contributions, capped at 40
	Behavioral    int // malware/behavioral signal contributions, capped at 30
	SupplyChain   int // typosquatting/confusion contributions, capped at 20
	Maintenance   int // abandonment/age signals, capped at 10
}

// RiskScore is the ForgeGuardian composite security risk score for a package.
type RiskScore struct {
	Overall int    // 0–100
	Grade   string // "A" (0–20), "B" (21–40), "C" (41–60), "D" (61–80), "F" (81–100)
	Factors RiskFactors
}

// ScoreFindings computes a RiskScore from a slice of findings.
func ScoreFindings(findings []Finding) RiskScore {
	var f RiskFactors

	for _, finding := range findings {
		switch finding.Type {
		case "cve":
			switch finding.Severity {
			case SeverityCritical:
				f.Vulnerability += 40
			case SeverityHigh:
				f.Vulnerability += 20
			case SeverityMedium:
				f.Vulnerability += 8
			case SeverityLow:
				f.Vulnerability += 3
			}
		case "malware":
			f.Behavioral += 30
		case "behavioral":
			switch finding.Severity {
			case SeverityCritical:
				f.Behavioral += 25
			case SeverityHigh:
				f.Behavioral += 15
			default:
				f.Behavioral += 5
			}
		case "supply-chain":
			switch finding.Severity {
			case SeverityCritical:
				f.SupplyChain += 20
			case SeverityHigh:
				f.SupplyChain += 12
			default:
				f.SupplyChain += 5
			}
		}
		// Maintenance signals surfaced as informational
		if finding.Severity == SeverityInformational && finding.Type == "supply-chain" {
			f.Maintenance += 5
		}
	}

	// Cap each factor
	if f.Vulnerability > 40 {
		f.Vulnerability = 40
	}
	if f.Behavioral > 30 {
		f.Behavioral = 30
	}
	if f.SupplyChain > 20 {
		f.SupplyChain = 20
	}
	if f.Maintenance > 10 {
		f.Maintenance = 10
	}

	overall := f.Vulnerability + f.Behavioral + f.SupplyChain + f.Maintenance
	grade := scoreGrade(overall)
	return RiskScore{Overall: overall, Grade: grade, Factors: f}
}

func scoreGrade(score int) string {
	switch {
	case score <= 20:
		return "A"
	case score <= 40:
		return "B"
	case score <= 60:
		return "C"
	case score <= 80:
		return "D"
	default:
		return "F"
	}
}
