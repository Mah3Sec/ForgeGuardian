// authoring.go contains reusable, pure logic for authoring and validating
// community detection signature YAML files. It is shared by cmd/fgctl's
// interactive `intel` subcommands and the REST API's intelligence authoring
// endpoints — no I/O, no CLI/HTTP concerns, just struct transforms.
package intelligence

import (
	"fmt"
	"regexp"
	"strings"
)

// SignatureYAML is the community YAML format (richer than DetectionSignature JSON).
type SignatureYAML struct {
	ID           string   `yaml:"id"`
	Name         string   `yaml:"name"`
	Type         string   `yaml:"type"`
	Ecosystem    string   `yaml:"ecosystem"`
	Severity     string   `yaml:"severity"`
	Package      string   `yaml:"package,omitempty"`
	VersionRange string   `yaml:"version_range,omitempty"`
	Target       string   `yaml:"target,omitempty"`
	SimilarNames []string `yaml:"similar_names,omitempty"`
	Pattern      string   `yaml:"pattern,omitempty"`
	Rule         string   `yaml:"rule,omitempty"`
	Description  string   `yaml:"description"`
	CVE          string   `yaml:"cve,omitempty"`
	References   []string `yaml:"references,omitempty"`
	Author       string   `yaml:"author"`
	Tags         []string `yaml:"tags,omitempty"`
}

// DefaultTags returns sensible default tags for a signature type.
func DefaultTags(t SignatureType) []string {
	switch t {
	case SigBlocklisted:
		return []string{"backdoor"}
	case SigTypoTarget:
		return []string{"typosquatting"}
	case SigBehavioral:
		return []string{"behavioral", "postinstall"}
	case SigMalwarePattern:
		return []string{"malware"}
	case SigMCPInjection:
		return []string{"prompt-injection", "mcp"}
	case SigPickleRule:
		return []string{"pickle", "ai-model"}
	default:
		return []string{}
	}
}

// SigTypeDir returns the community-repo subdirectory a signature type is
// filed under (e.g. signatures/<dir>/<ecosystem>/<id>.yaml).
func SigTypeDir(t SignatureType) string {
	switch t {
	case SigBlocklisted:
		return "blocklisted"
	case SigTypoTarget:
		return "typosquatting"
	case SigBehavioral:
		return "behavioral"
	case SigMalwarePattern:
		return "malware"
	case SigMCPInjection:
		return "mcp"
	case SigPickleRule:
		return "ai-model"
	default:
		return "other"
	}
}

// BuildSignatureID derives the canonical "FG-..." signature ID from its type,
// ecosystem, and human-readable name.
func BuildSignatureID(sigType SignatureType, ecosystem, name string) string {
	ecoSlug := ecosystem
	if ecoSlug == "*" {
		ecoSlug = "any"
	}
	nameSlug := regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(name), "-")
	if len(nameSlug) > 40 {
		nameSlug = nameSlug[:40]
	}
	prefix := map[SignatureType]string{
		SigBlocklisted:    "FG-" + ecoSlug + "-",
		SigTypoTarget:     "FG-" + ecoSlug + "-typosquat-",
		SigBehavioral:     "FG-" + ecoSlug + "-behavioral-",
		SigMalwarePattern: "FG-malware-",
		SigMCPInjection:   "FG-mcp-",
		SigPickleRule:     "FG-ai-",
	}
	return prefix[sigType] + nameSlug
}

// ValidateSignature checks a SignatureYAML for schema and content problems.
// It returns an empty (nil) slice when the signature is valid.
func ValidateSignature(sig SignatureYAML) []string {
	var problems []string

	// Required fields
	if sig.ID == "" {
		problems = append(problems, "missing: id")
	} else if !strings.HasPrefix(sig.ID, "FG-") {
		problems = append(problems, fmt.Sprintf("id %q must start with 'FG-'", sig.ID))
	}
	if sig.Name == "" {
		problems = append(problems, "missing: name")
	}
	if sig.Type == "" {
		problems = append(problems, "missing: type")
	} else {
		validTypes := map[string]bool{
			"blocklisted_package": true, "typosquatting_target": true,
			"behavioral_rule": true, "malware_pattern": true,
			"mcp_injection_pattern": true, "pickle_rule": true,
		}
		if !validTypes[sig.Type] {
			problems = append(problems, fmt.Sprintf("invalid type %q — must be one of: blocklisted_package, typosquatting_target, behavioral_rule, malware_pattern, mcp_injection_pattern, pickle_rule", sig.Type))
		}
	}
	if sig.Ecosystem == "" {
		problems = append(problems, "missing: ecosystem")
	} else {
		validEco := map[string]bool{
			"npm": true, "pypi": true, "go": true, "rubygems": true,
			"crates": true, "maven": true, "huggingface": true, "mcp": true, "*": true,
		}
		if !validEco[sig.Ecosystem] {
			problems = append(problems, fmt.Sprintf("invalid ecosystem %q", sig.Ecosystem))
		}
	}
	if sig.Severity == "" {
		problems = append(problems, "missing: severity")
	} else {
		validSev := map[string]bool{
			"CRITICAL": true, "HIGH": true, "MEDIUM": true, "LOW": true, "INFORMATIONAL": true,
		}
		if !validSev[strings.ToUpper(sig.Severity)] {
			problems = append(problems, fmt.Sprintf("invalid severity %q — must be CRITICAL, HIGH, MEDIUM, LOW, or INFORMATIONAL", sig.Severity))
		}
	}
	if sig.Description == "" {
		problems = append(problems, "missing: description")
	}

	// Type-specific required fields
	switch sig.Type {
	case "blocklisted_package":
		if sig.Package == "" {
			problems = append(problems, "blocklisted_package requires: package")
		}
	case "typosquatting_target":
		if sig.Target == "" {
			problems = append(problems, "typosquatting_target requires: target")
		}
	case "malware_pattern", "mcp_injection_pattern":
		if sig.Pattern == "" {
			problems = append(problems, sig.Type+" requires: pattern")
		} else {
			if _, err := regexp.Compile(sig.Pattern); err != nil {
				problems = append(problems, fmt.Sprintf("pattern is invalid regex: %v", err))
			}
		}
	case "behavioral_rule", "pickle_rule":
		if sig.Rule == "" {
			problems = append(problems, sig.Type+" requires: rule")
		}
	}

	// Author encouraged
	if sig.Author == "" {
		problems = append(problems, "missing: author (your GitHub username — gets credited in release notes)")
	}

	return problems
}

// ToDetectionSignature converts an authored YAML signature into the runtime
// JSON shape the scanner actually consumes (SignatureStore.Signatures).
// SimilarNames/VersionRange/Name/References/Tags are richer YAML-only
// authoring metadata with no equivalent runtime field — the scanner's
// typosquat detection only ever needs Target itself (see
// internal/scanner/behavioral/scanner.go), not the example variant list.
func ToDetectionSignature(sig SignatureYAML) DetectionSignature {
	return DetectionSignature{
		ID:          sig.ID,
		Type:        SignatureType(sig.Type),
		Ecosystem:   sig.Ecosystem,
		Target:      sig.Target,
		Pattern:     sig.Pattern,
		Package:     sig.Package,
		Rule:        sig.Rule,
		Severity:    sig.Severity,
		Title:       sig.Name,
		Description: sig.Description,
		Source:      "community",
		CVE:         sig.CVE,
	}
}
