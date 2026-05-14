package localscanner

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ManifestEntry is a single dependency discovered in a manifest file.
type ManifestEntry struct {
	Ecosystem       string // "npm", "pypi", "go", "crates", "maven", "rubygems"
	Name            string
	Version         string // pinned version; "" means range-only → scanner skips
	FilePath        string // absolute path to manifest file
	Line            int    // 1-indexed line number (for editor integration)
	Raw             string // original version string e.g. "^4.17.21"
	IsDevDependency bool   // true for devDependencies (npm), dev-dependencies (cargo), etc.
}

// ParseManifest parses a manifest file and returns all dependency entries.
// Unknown file types return an empty slice without error.
func ParseManifest(path string) ([]ManifestEntry, error) {
	name := filepath.Base(path)
	switch name {
	case "package.json":
		return parsePackageJSON(path)
	case "requirements.txt":
		return parseRequirements(path)
	case "pyproject.toml":
		return parsePyprojectTOML(path)
	case "go.mod":
		return parseGoMod(path)
	case "Cargo.toml":
		return parseCargoToml(path)
	case "pom.xml":
		return parsePomXML(path)
	case "Gemfile":
		return parseGemfile(path)
	}
	return nil, nil
}

// normalizeVersion strips common constraint operators and returns a plain
// version string. Returns "" for pure range constraints with no base version.
func normalizeVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	// Remove common prefixes: ^, ~, ~=, >=, <=, ==, !=, >, <
	for _, prefix := range []string{"==", "~=", ">=", "<=", "!=", "^", "~", ">", "<"} {
		raw = strings.TrimPrefix(raw, prefix)
		raw = strings.TrimSpace(raw)
	}
	// If what remains still starts with a non-version char (letter, comma, *),
	// the constraint is a range we can't pin — return empty.
	if raw == "" || raw == "*" || strings.ContainsAny(raw[:1], ",;<>!") {
		return ""
	}
	return raw
}

// ── package.json ────────────────────────────────────────────────────────────

func parsePackageJSON(path string) ([]ManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}
	var entries []ManifestEntry
	for name, raw := range pkg.Dependencies {
		entries = append(entries, ManifestEntry{
			Ecosystem: "npm", Name: name, Version: normalizeVersion(raw),
			FilePath: path, Raw: raw,
		})
	}
	for name, raw := range pkg.DevDependencies {
		entries = append(entries, ManifestEntry{
			Ecosystem: "npm", Name: name, Version: normalizeVersion(raw),
			FilePath: path, Raw: raw, IsDevDependency: true,
		})
	}
	return entries, nil
}

// ── requirements.txt ────────────────────────────────────────────────────────

var reqLineRe = regexp.MustCompile(`^([A-Za-z0-9_.\-\[\]]+)\s*([>=<!~^][^\s#;]*)`)

func parseRequirements(path string) ([]ManifestEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []ManifestEntry
	line := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") || strings.HasPrefix(text, "-") {
			continue
		}
		m := reqLineRe.FindStringSubmatch(text)
		if m == nil {
			// bare package name, no version
			name := strings.Fields(text)[0]
			entries = append(entries, ManifestEntry{
				Ecosystem: "pypi", Name: name, FilePath: path, Line: line,
			})
			continue
		}
		entries = append(entries, ManifestEntry{
			Ecosystem: "pypi", Name: m[1], Version: normalizeVersion(m[2]),
			FilePath: path, Line: line, Raw: m[2],
		})
	}
	return entries, scanner.Err()
}

// ── pyproject.toml ──────────────────────────────────────────────────────────

// pyproject.toml dep lines: 'requests>=2.28' or 'requests==2.28.0'
var pyprojectDepRe = regexp.MustCompile(`["']?([A-Za-z0-9_.\-]+)\s*([>=<!~^][^"',\s]*)?["']?`)

func parsePyprojectTOML(path string) ([]ManifestEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []ManifestEntry
	inDeps := false
	line := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(text, "[") {
			inDeps = strings.Contains(text, "dependencies")
			continue
		}
		if !inDeps || text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		m := pyprojectDepRe.FindStringSubmatch(text)
		if m != nil && m[1] != "" {
			entries = append(entries, ManifestEntry{
				Ecosystem: "pypi", Name: m[1], Version: normalizeVersion(m[2]),
				FilePath: path, Line: line, Raw: m[2],
			})
		}
	}
	return entries, scanner.Err()
}

// ── go.mod ──────────────────────────────────────────────────────────────────

var goModRequireRe = regexp.MustCompile(`^\s+([^\s]+)\s+v([^\s]+)`)

func parseGoMod(path string) ([]ManifestEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []ManifestEntry
	inRequire := false
	line := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line++
		text := scanner.Text()
		trimmed := strings.TrimSpace(text)
		if strings.HasPrefix(trimmed, "require (") || trimmed == "require (" {
			inRequire = true
			continue
		}
		if inRequire && trimmed == ")" {
			inRequire = false
			continue
		}
		// single-line require
		if strings.HasPrefix(trimmed, "require ") && !strings.HasSuffix(trimmed, "(") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 3 {
				ver := strings.TrimPrefix(parts[2], "v")
				entries = append(entries, ManifestEntry{
					Ecosystem: "go", Name: parts[1], Version: ver,
					FilePath: path, Line: line, Raw: parts[2],
				})
			}
			continue
		}
		if inRequire {
			m := goModRequireRe.FindStringSubmatch(text)
			if m != nil {
				entries = append(entries, ManifestEntry{
					Ecosystem: "go", Name: m[1], Version: m[2],
					FilePath: path, Line: line, Raw: "v" + m[2],
				})
			}
		}
	}
	return entries, scanner.Err()
}

// ── Cargo.toml ──────────────────────────────────────────────────────────────

var cargoDepRe = regexp.MustCompile(`^([a-zA-Z0-9_\-]+)\s*=\s*"([^"]+)"`)
var cargoDepTableRe = regexp.MustCompile(`^([a-zA-Z0-9_\-]+)\s*=\s*\{[^}]*version\s*=\s*"([^"]+)"`)

func parseCargoToml(path string) ([]ManifestEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []ManifestEntry
	inDeps := false
	line := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(text, "[") {
			inDeps = strings.Contains(text, "dependencies")
			continue
		}
		if !inDeps || text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		if m := cargoDepTableRe.FindStringSubmatch(text); m != nil {
			entries = append(entries, ManifestEntry{
				Ecosystem: "crates", Name: m[1], Version: normalizeVersion(m[2]),
				FilePath: path, Line: line, Raw: m[2],
			})
			continue
		}
		if m := cargoDepRe.FindStringSubmatch(text); m != nil {
			entries = append(entries, ManifestEntry{
				Ecosystem: "crates", Name: m[1], Version: normalizeVersion(m[2]),
				FilePath: path, Line: line, Raw: m[2],
			})
		}
	}
	return entries, scanner.Err()
}

// ── pom.xml ─────────────────────────────────────────────────────────────────

type pomXML struct {
	Dependencies []struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Version    string `xml:"version"`
	} `xml:"dependencies>dependency"`
}

func parsePomXML(path string) ([]ManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pom pomXML
	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil, err
	}
	var entries []ManifestEntry
	for _, dep := range pom.Dependencies {
		name := dep.GroupID + ":" + dep.ArtifactID
		entries = append(entries, ManifestEntry{
			Ecosystem: "maven", Name: name, Version: dep.Version,
			FilePath: path, Raw: dep.Version,
		})
	}
	return entries, nil
}

// ── Gemfile ─────────────────────────────────────────────────────────────────

var gemLineRe = regexp.MustCompile(`^gem\s+['"]([^'"]+)['"](?:\s*,\s*['"]([^'"]+)['"])?`)

func parseGemfile(path string) ([]ManifestEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []ManifestEntry
	line := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(text, "#") {
			continue
		}
		m := gemLineRe.FindStringSubmatch(text)
		if m != nil {
			entries = append(entries, ManifestEntry{
				Ecosystem: "rubygems", Name: m[1], Version: normalizeVersion(m[2]),
				FilePath: path, Line: line, Raw: m[2],
			})
		}
	}
	return entries, scanner.Err()
}
