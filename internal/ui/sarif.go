// Package ui — SARIF 2.1.0 output for CI/GitHub Code Scanning integration.
package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/scanner"
)

// SARIF types (subset of SARIF 2.1.0 needed for supply chain findings).

type sarifLog struct {
	Schema  string      `json:"$schema"`
	Version string      `json:"version"`
	Runs    []sarifRun  `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	ShortDescription sarifMessage       `json:"shortDescription"`
	FullDescription  sarifMessage       `json:"fullDescription"`
	DefaultConfig    sarifDefaultConfig `json:"defaultConfiguration"`
	HelpURI          string             `json:"helpUri,omitempty"`
}

type sarifDefaultConfig struct {
	Level string `json:"level"` // "error" | "warning" | "note"
}

type sarifResult struct {
	RuleID  string          `json:"ruleId"`
	Level   string          `json:"level"`
	Message sarifMessage    `json:"message"`
	Locations []sarifLoc    `json:"locations,omitempty"`
}

type sarifLoc struct {
	PhysicalLocation sarifPhysLoc `json:"physicalLocation"`
}

type sarifPhysLoc struct {
	ArtifactLocation sarifArtLoc `json:"artifactLocation"`
	Region           sarifRegion `json:"region"`
}

type sarifArtLoc struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

// WriteSARIF serialises findings in SARIF 2.1.0 format.
// pkg is the package identifier (e.g. "lodash@4.17.20"), manifestURI is the
// relative path to the manifest file (e.g. "package.json") or "" for remote scans.
func WriteSARIF(w io.Writer, pkg, manifestURI string, findings []core.Finding, sum scanner.ScanSummary, toolVersion string) error {
	// Build deduplicated rule list.
	rulesSeen := map[string]bool{}
	var rules []sarifRule
	for _, f := range findings {
		if rulesSeen[f.ID] {
			continue
		}
		rulesSeen[f.ID] = true
		rules = append(rules, sarifRule{
			ID:   f.ID,
			Name: sanitizeName(f.ID),
			ShortDescription: sarifMessage{Text: f.Title},
			FullDescription:  sarifMessage{Text: firstN(f.Description, 512)},
			DefaultConfig:    sarifDefaultConfig{Level: sarifLevel(f.Severity)},
		})
	}

	// Build result list.
	var results []sarifResult
	for _, f := range findings {
		msg := f.Title
		if f.FixedVersion != "" {
			msg += fmt.Sprintf(" — fix: upgrade to %s", f.FixedVersion)
		}
		r := sarifResult{
			RuleID:  f.ID,
			Level:   sarifLevel(f.Severity),
			Message: sarifMessage{Text: msg},
		}
		if manifestURI != "" {
			r.Locations = []sarifLoc{{
				PhysicalLocation: sarifPhysLoc{
					ArtifactLocation: sarifArtLoc{URI: manifestURI},
					Region:           sarifRegion{StartLine: 1},
				},
			}}
		}
		results = append(results, r)
	}

	log := sarifLog{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{
				Driver: sarifDriver{
					Name:           "ForgeGuardian",
					Version:        toolVersion,
					InformationURI: "https://github.com/mah3sec/forgeguardian",
					Rules:          rules,
				},
			},
			Results: results,
		}},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(log)
}

func sarifLevel(sev core.Severity) string {
	switch sev {
	case core.SeverityCritical, core.SeverityHigh:
		return "error"
	case core.SeverityMedium:
		return "warning"
	case core.SeverityLow:
		return "note"
	default:
		return "none"
	}
}

func sanitizeName(id string) string {
	return strings.ReplaceAll(id, "-", "")
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
