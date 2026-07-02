// Package ai_model performs security analysis specific to HuggingFace model
// artifacts: pickle/safetensors safety, serialization format checks, unusual
// file types inside model repos, and model-card policy signals.
package ai_model

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/intelligence"
)

// Scanner checks AI model artifacts for supply-chain risks unique to model weights.
type Scanner struct {
	store *intelligence.SignatureStore
}

// New returns a new ai_model Scanner using built-in rules only.
func New() *Scanner { return &Scanner{} }

// NewWithStore returns an ai_model Scanner that supplements built-in pickle
// risk patterns with community pickle_rule signatures from the intelligence store.
func NewWithStore(store *intelligence.SignatureStore) *Scanner {
	return &Scanner{store: store}
}

// Name implements core.Scanner.
func (s *Scanner) Name() string { return "ai_model" }

// Scan analyzes HuggingFace model artifacts for security issues.
// For non-HuggingFace ecosystems it returns no findings.
func (s *Scanner) Scan(_ context.Context, artifact core.BuiltArtifact) ([]core.Finding, error) {
	if artifact.Source.Package.Ecosystem != "huggingface" {
		return nil, nil
	}

	var findings []core.Finding
	log := artifact.BuildLog
	pkg := artifact.Source.Package

	findings = append(findings, s.checkPickleSafety(log, pkg)...)
	findings = append(findings, s.checkSerializationFormat(log, pkg)...)
	findings = append(findings, s.checkSuspiciousFiles(artifact)...)
	findings = append(findings, s.checkModelCardPolicy(log, pkg)...)
	findings = append(findings, s.checkExternalWeightURLs(log, pkg)...)

	return findings, nil
}

// --- Pickle safety ---

// pickleRiskPatterns are tokens that indicate unsafe pickle deserialization.
var pickleRiskPatterns = []string{
	"REDUCE", "GLOBAL", "__reduce__", "exec(", "eval(", "subprocess",
	"os.system", "importlib", "builtins.exec", "posix.system",
}

func (s *Scanner) activePicklePatterns() []string {
	patterns := make([]string, len(pickleRiskPatterns))
	copy(patterns, pickleRiskPatterns)
	if s.store == nil {
		return patterns
	}
	for _, sig := range s.store.Signatures {
		if sig.Type == intelligence.SigPickleRule && sig.Rule != "" {
			patterns = append(patterns, sig.Rule)
		}
	}
	return patterns
}

func (s *Scanner) checkPickleSafety(log string, pkg core.PackageVersion) []core.Finding {
	var findings []core.Finding
	pickleIssues := extractField(log, "pickle_issues")

	for _, pat := range s.activePicklePatterns() {
		if strings.Contains(log, pat) || strings.Contains(pickleIssues, pat) {
			findings = append(findings, core.Finding{
				ID:       "AI-MODEL-PICKLE-UNSAFE-OP",
				Severity: core.SeverityCritical,
				Type:     "malware",
				Title:    "Unsafe pickle opcode detected in model weights",
				Description: fmt.Sprintf(
					"%s@%s contains a pickle stream with opcode %q which can execute arbitrary code during model loading.",
					pkg.Name, pkg.Version, pat,
				),
				Source: "ai_model",
				Metadata: map[string]any{
					"opcode":    pat,
					"ecosystem": "huggingface",
				},
			})
			break // one finding is enough; avoid duplicates per model
		}
	}

	// Build log may surface a curated pickle_issues field
	if pickleIssues != "" && pickleIssues != "[]" && pickleIssues != "<nil>" && len(findings) == 0 {
		findings = append(findings, core.Finding{
			ID:          "AI-MODEL-PICKLE-ISSUES",
			Severity:    core.SeverityHigh,
			Type:        "malware",
			Title:       "Pickle safety issues found in model weights",
			Description: fmt.Sprintf("%s@%s: %s", pkg.Name, pkg.Version, pickleIssues),
			Source:      "ai_model",
		})
	}
	return findings
}

// --- Serialization format ---

func (s *Scanner) checkSerializationFormat(log string, pkg core.PackageVersion) []core.Finding {
	format := extractField(log, "weight_format")
	if format == "" {
		return nil
	}

	// safetensors is the safe format; .pt/.bin/.pkl are risky
	risky := []string{".pt", ".bin", ".pkl", "pytorch_model"}
	for _, r := range risky {
		if strings.Contains(format, r) {
			return []core.Finding{{
				ID:   "AI-MODEL-UNSAFE-FORMAT",
				Severity: core.SeverityHigh,
				Type: "supply-chain",
				Title: "Model uses unsafe serialization format",
				Description: fmt.Sprintf(
					"%s@%s uses %q format. Prefer safetensors — it cannot execute arbitrary code during load.",
					pkg.Name, pkg.Version, format,
				),
				Source: "ai_model",
				Metadata: map[string]any{"format": format},
			}}
		}
	}
	return nil
}

// --- Suspicious files in the model repo ---

var suspiciousExtensions = []string{
	".exe", ".dll", ".so", ".dylib", ".sh", ".bat", ".ps1",
	".py", ".js", ".rb", ".php", ".jar",
}

func (s *Scanner) checkSuspiciousFiles(artifact core.BuiltArtifact) []core.Finding {
	var findings []core.Finding
	if artifact.LocalPath == "" {
		return nil
	}

	dir := artifact.LocalPath
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		for _, bad := range suspiciousExtensions {
			if ext == bad {
				findings = append(findings, core.Finding{
					ID:       "AI-MODEL-SUSPICIOUS-FILE",
					Severity: core.SeverityMedium,
					Type:     "supply-chain",
					Title:    "Unexpected executable/script file in model repository",
					Description: fmt.Sprintf(
						"File %q has extension %q which is unexpected in an AI model repo and may be malicious.",
						filepath.Base(path), ext,
					),
					Source: "ai_model",
					Metadata: map[string]any{
						"file": path,
						"ext":  ext,
					},
				})
			}
		}
		return nil
	})
	return findings
}

// --- Model-card policy ---

func (s *Scanner) checkModelCardPolicy(log string, pkg core.PackageVersion) []core.Finding {
	hasCard := extractField(log, "has_model_card")
	if hasCard == "false" || hasCard == "0" {
		return []core.Finding{{
			ID:       "AI-MODEL-NO-MODEL-CARD",
			Severity: core.SeverityLow,
			Type:     "supply-chain",
			Title:    "Model has no model card",
			Description: fmt.Sprintf(
				"%s@%s has no model card. Responsible AI practice requires documentation of training data, intended use, and limitations.",
				pkg.Name, pkg.Version,
			),
			Source: "ai_model",
		}}
	}
	return nil
}

// --- External weight URL check ---

func (s *Scanner) checkExternalWeightURLs(log string, pkg core.PackageVersion) []core.Finding {
	externalURLs := extractField(log, "external_weight_urls")
	if externalURLs == "" || externalURLs == "[]" {
		return nil
	}
	return []core.Finding{{
		ID:       "AI-MODEL-EXTERNAL-WEIGHTS",
		Severity: core.SeverityHigh,
		Type:     "supply-chain",
		Title:    "Model weights loaded from external URLs",
		Description: fmt.Sprintf(
			"%s@%s references external URLs to load weights at runtime: %s. This bypasses supply chain controls.",
			pkg.Name, pkg.Version, externalURLs,
		),
		Source: "ai_model",
		Metadata: map[string]any{"urls": externalURLs},
	}}
}

// extractField pulls a key=value line from the build log.
func extractField(log, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(log, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}
