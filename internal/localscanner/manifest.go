package localscanner

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ManifestEntry is a single dependency discovered in a manifest file.
type ManifestEntry struct {
	Ecosystem       string // "npm", "pypi", "go", "crates", "maven", "rubygems", "nuget", "packagist"
	Name            string
	Version         string // pinned version; "" means range-only → scanner skips
	FilePath        string // absolute path to manifest file
	Line            int    // 1-indexed line number (for editor integration)
	Raw             string // original version string e.g. "^4.17.21"
	IsDevDependency bool   // true for devDependencies (npm), dev-dependencies (cargo), etc.
}

// stripBOM removes a UTF-8 byte order mark from the start of data.
func stripBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return data[3:]
	}
	return data
}

// ParseManifest parses a manifest file and returns all dependency entries.
// Unknown file types return an empty slice without error.
func ParseManifest(path string) ([]ManifestEntry, error) {
	name := filepath.Base(path)
	switch name {
	case "package.json":
		return parsePackageJSON(path)
	case "package-lock.json":
		return parsePackageLockJSON(path)
	case "yarn.lock":
		return parseYarnLock(path)
	case "pnpm-lock.yaml":
		return parsePnpmLock(path)
	case "requirements.txt":
		return parseRequirements(path)
	case "pyproject.toml":
		return parsePyprojectTOML(path)
	case "Pipfile":
		return parsePipfile(path)
	case "Pipfile.lock":
		return parsePipfileLock(path)
	case "poetry.lock":
		return parsePoetryLock(path)
	case "go.mod":
		return parseGoMod(path)
	case "go.sum":
		return parseGoSum(path)
	case "Cargo.toml":
		return parseCargoToml(path)
	case "Cargo.lock":
		return parseCargoLock(path)
	case "pom.xml":
		return parsePomXML(path)
	case "build.gradle", "build.gradle.kts":
		return parseGradle(path)
	case "Gemfile":
		return parseGemfile(path)
	case "Gemfile.lock":
		return parseGemfileLock(path)
	case "packages.config":
		return parseNugetPackagesConfig(path)
	case "composer.json":
		return parseComposerJSON(path)
	case "composer.lock":
		return parseComposerLock(path)
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
	data = stripBOM(data)
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
	data = stripBOM(data)
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

// ── package-lock.json ──────────────────────────────────────────────────────

func parsePackageLockJSON(path string) ([]ManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = stripBOM(data)

	var lock struct {
		Packages     map[string]json.RawMessage `json:"packages"`
		Dependencies map[string]json.RawMessage `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	var entries []ManifestEntry

	// lockfileVersion 2/3 uses "packages"
	for key, raw := range lock.Packages {
		if key == "" {
			continue // root project entry
		}
		name := key
		if idx := strings.LastIndex(key, "node_modules/"); idx >= 0 {
			name = key[idx+len("node_modules/"):]
		}
		var info struct {
			Version string `json:"version"`
			Dev     bool   `json:"dev"`
		}
		if json.Unmarshal(raw, &info) != nil || info.Version == "" {
			continue
		}
		entries = append(entries, ManifestEntry{
			Ecosystem: "npm", Name: name, Version: info.Version,
			FilePath: path, Raw: info.Version, IsDevDependency: info.Dev,
		})
	}

	// lockfileVersion 1 uses "dependencies"
	if len(entries) == 0 {
		for name, raw := range lock.Dependencies {
			var info struct {
				Version string `json:"version"`
				Dev     bool   `json:"dev"`
			}
			if json.Unmarshal(raw, &info) != nil || info.Version == "" {
				continue
			}
			entries = append(entries, ManifestEntry{
				Ecosystem: "npm", Name: name, Version: info.Version,
				FilePath: path, Raw: info.Version, IsDevDependency: info.Dev,
			})
		}
	}
	return entries, nil
}

// ── yarn.lock ──────────────────────────────────────────────────────────────

var yarnPkgRe = regexp.MustCompile(`^"?(@?[^@\s"]+)@`)
var yarnVerRe = regexp.MustCompile(`^\s+version\s+"([^"]+)"`)

func parseYarnLock(path string) ([]ManifestEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []ManifestEntry
	var currentPkg string
	line := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if m := yarnPkgRe.FindStringSubmatch(text); m != nil {
			currentPkg = m[1]
			continue
		}
		if currentPkg != "" {
			if m := yarnVerRe.FindStringSubmatch(text); m != nil {
				entries = append(entries, ManifestEntry{
					Ecosystem: "npm", Name: currentPkg, Version: m[1],
					FilePath: path, Line: line, Raw: m[1],
				})
				currentPkg = ""
			}
		}
	}
	return entries, scanner.Err()
}

// ── pnpm-lock.yaml ─────────────────────────────────────────────────────────

func parsePnpmLock(path string) ([]ManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lock struct {
		Packages map[string]yaml.Node `yaml:"packages"`
	}
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	var entries []ManifestEntry
	for key := range lock.Packages {
		name, version := parsePnpmPackageKey(key)
		if name == "" || version == "" {
			continue
		}
		entries = append(entries, ManifestEntry{
			Ecosystem: "npm", Name: name, Version: version,
			FilePath: path, Raw: version,
		})
	}
	return entries, nil
}

// parsePnpmPackageKey extracts name and version from pnpm lockfile keys.
// Formats: "/@scope/pkg@1.0.0" or "/pkg@1.0.0" or "/@scope/pkg/1.0.0"
func parsePnpmPackageKey(key string) (string, string) {
	key = strings.TrimPrefix(key, "/")
	// scoped: @scope/pkg@version or @scope/pkg/version
	if strings.HasPrefix(key, "@") {
		rest := key[1:]
		if slashIdx := strings.Index(rest, "/"); slashIdx >= 0 {
			afterScope := rest[slashIdx+1:]
			if atIdx := strings.Index(afterScope, "@"); atIdx >= 0 {
				name := key[:1+slashIdx+1+atIdx]
				return name, afterScope[atIdx+1:]
			}
			if slashIdx2 := strings.Index(afterScope, "/"); slashIdx2 >= 0 {
				name := key[:1+slashIdx+1+slashIdx2]
				return name, afterScope[slashIdx2+1:]
			}
		}
		return "", ""
	}
	// unscoped: pkg@version
	if atIdx := strings.LastIndex(key, "@"); atIdx > 0 {
		return key[:atIdx], key[atIdx+1:]
	}
	return "", ""
}

// ── Pipfile ────────────────────────────────────────────────────────────────

var pipfileDepRe = regexp.MustCompile(`^([A-Za-z0-9_.\-]+)\s*=\s*(?:"([^"]*)"|\{[^}]*version\s*=\s*"([^"]*)")`)

func parsePipfile(path string) ([]ManifestEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []ManifestEntry
	inPkgs := false
	isDev := false
	line := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(text, "[") {
			lower := strings.ToLower(text)
			inPkgs = lower == "[packages]" || lower == "[dev-packages]"
			isDev = lower == "[dev-packages]"
			continue
		}
		if !inPkgs || text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		m := pipfileDepRe.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		raw := m[2]
		if raw == "" {
			raw = m[3]
		}
		entries = append(entries, ManifestEntry{
			Ecosystem: "pypi", Name: m[1], Version: normalizeVersion(raw),
			FilePath: path, Line: line, Raw: raw, IsDevDependency: isDev,
		})
	}
	return entries, scanner.Err()
}

// ── Pipfile.lock ───────────────────────────────────────────────────────────

func parsePipfileLock(path string) ([]ManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = stripBOM(data)

	var lock struct {
		Default map[string]struct {
			Version string `json:"version"`
		} `json:"default"`
		Develop map[string]struct {
			Version string `json:"version"`
		} `json:"develop"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	var entries []ManifestEntry
	for name, info := range lock.Default {
		entries = append(entries, ManifestEntry{
			Ecosystem: "pypi", Name: name, Version: normalizeVersion(info.Version),
			FilePath: path, Raw: info.Version,
		})
	}
	for name, info := range lock.Develop {
		entries = append(entries, ManifestEntry{
			Ecosystem: "pypi", Name: name, Version: normalizeVersion(info.Version),
			FilePath: path, Raw: info.Version, IsDevDependency: true,
		})
	}
	return entries, nil
}

// ── poetry.lock ────────────────────────────────────────────────────────────

var poetryPkgRe = regexp.MustCompile(`^\[\[package\]\]`)
var poetryNameRe = regexp.MustCompile(`^name\s*=\s*"([^"]+)"`)
var poetryVerRe = regexp.MustCompile(`^version\s*=\s*"([^"]+)"`)

func parsePoetryLock(path string) ([]ManifestEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []ManifestEntry
	var name, version string
	line := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if poetryPkgRe.MatchString(text) {
			if name != "" && version != "" {
				entries = append(entries, ManifestEntry{
					Ecosystem: "pypi", Name: name, Version: version,
					FilePath: path, Raw: version,
				})
			}
			name, version = "", ""
			continue
		}
		if m := poetryNameRe.FindStringSubmatch(text); m != nil {
			name = m[1]
		}
		if m := poetryVerRe.FindStringSubmatch(text); m != nil {
			version = m[1]
		}
	}
	if name != "" && version != "" {
		entries = append(entries, ManifestEntry{
			Ecosystem: "pypi", Name: name, Version: version,
			FilePath: path, Raw: version,
		})
	}
	return entries, scanner.Err()
}

// ── go.sum ─────────────────────────────────────────────────────────────────

var goSumRe = regexp.MustCompile(`^([^\s]+)\s+v([^\s/]+)\s+`)

func parseGoSum(path string) ([]ManifestEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := make(map[string]bool)
	var entries []ManifestEntry
	line := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line++
		text := scanner.Text()
		m := goSumRe.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		key := m[1] + "@" + m[2]
		if seen[key] {
			continue // go.sum has two lines per module (zip + go.mod)
		}
		seen[key] = true
		entries = append(entries, ManifestEntry{
			Ecosystem: "go", Name: m[1], Version: m[2],
			FilePath: path, Line: line, Raw: "v" + m[2],
		})
	}
	return entries, scanner.Err()
}

// ── Cargo.lock ─────────────────────────────────────────────────────────────

var cargoLockPkgRe = regexp.MustCompile(`^\[\[package\]\]`)
var cargoLockNameRe = regexp.MustCompile(`^name\s*=\s*"([^"]+)"`)
var cargoLockVerRe = regexp.MustCompile(`^version\s*=\s*"([^"]+)"`)

func parseCargoLock(path string) ([]ManifestEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []ManifestEntry
	var name, version string
	line := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if cargoLockPkgRe.MatchString(text) {
			if name != "" && version != "" {
				entries = append(entries, ManifestEntry{
					Ecosystem: "crates", Name: name, Version: version,
					FilePath: path, Raw: version,
				})
			}
			name, version = "", ""
			continue
		}
		if m := cargoLockNameRe.FindStringSubmatch(text); m != nil {
			name = m[1]
		}
		if m := cargoLockVerRe.FindStringSubmatch(text); m != nil {
			version = m[1]
		}
	}
	if name != "" && version != "" {
		entries = append(entries, ManifestEntry{
			Ecosystem: "crates", Name: name, Version: version,
			FilePath: path, Raw: version,
		})
	}
	return entries, scanner.Err()
}

// ── build.gradle / build.gradle.kts ────────────────────────────────────────

var gradleDepRe = regexp.MustCompile(`(?:implementation|api|compileOnly|runtimeOnly|testImplementation|classpath)\s*[("']+([^:'"]+):([^:'"]+):([^'")\s]+)`)

func parseGradle(path string) ([]ManifestEntry, error) {
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
		text := scanner.Text()
		matches := gradleDepRe.FindAllStringSubmatch(text, -1)
		for _, m := range matches {
			name := m[1] + ":" + m[2]
			entries = append(entries, ManifestEntry{
				Ecosystem: "maven", Name: name, Version: m[3],
				FilePath: path, Line: line, Raw: m[3],
			})
		}
	}
	return entries, scanner.Err()
}

// ── Gemfile.lock ───────────────────────────────────────────────────────────

var gemLockSpecRe = regexp.MustCompile(`^\s{4}(\S+)\s+\((\S+)\)`)

func parseGemfileLock(path string) ([]ManifestEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []ManifestEntry
	inSpecs := false
	line := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line++
		text := scanner.Text()
		trimmed := strings.TrimSpace(text)
		if trimmed == "specs:" {
			inSpecs = true
			continue
		}
		if inSpecs && (trimmed == "" || (len(text) > 0 && text[0] != ' ')) {
			inSpecs = false
			continue
		}
		if !inSpecs {
			continue
		}
		if m := gemLockSpecRe.FindStringSubmatch(text); m != nil {
			entries = append(entries, ManifestEntry{
				Ecosystem: "rubygems", Name: m[1], Version: m[2],
				FilePath: path, Line: line, Raw: m[2],
			})
		}
	}
	return entries, scanner.Err()
}

// ── packages.config (NuGet) ────────────────────────────────────────────────

type nugetPackagesConfig struct {
	Packages []struct {
		ID      string `xml:"id,attr"`
		Version string `xml:"version,attr"`
	} `xml:"package"`
}

func parseNugetPackagesConfig(path string) ([]ManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = stripBOM(data)
	var cfg nugetPackagesConfig
	if err := xml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	var entries []ManifestEntry
	for _, pkg := range cfg.Packages {
		entries = append(entries, ManifestEntry{
			Ecosystem: "nuget", Name: pkg.ID, Version: pkg.Version,
			FilePath: path, Raw: pkg.Version,
		})
	}
	return entries, nil
}

// ── composer.json (PHP/Packagist) ──────────────────────────────────────────

func parseComposerJSON(path string) ([]ManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = stripBOM(data)
	var comp struct {
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
	}
	if err := json.Unmarshal(data, &comp); err != nil {
		return nil, err
	}
	var entries []ManifestEntry
	for name, raw := range comp.Require {
		if name == "php" || strings.HasPrefix(name, "ext-") {
			continue
		}
		entries = append(entries, ManifestEntry{
			Ecosystem: "packagist", Name: name, Version: normalizeVersion(raw),
			FilePath: path, Raw: raw,
		})
	}
	for name, raw := range comp.RequireDev {
		if name == "php" || strings.HasPrefix(name, "ext-") {
			continue
		}
		entries = append(entries, ManifestEntry{
			Ecosystem: "packagist", Name: name, Version: normalizeVersion(raw),
			FilePath: path, Raw: raw, IsDevDependency: true,
		})
	}
	return entries, nil
}

// ── composer.lock (PHP/Packagist) ──────────────────────────────────────────

func parseComposerLock(path string) ([]ManifestEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	data = stripBOM(data)
	var lock struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
		PackagesDev []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages-dev"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}
	var entries []ManifestEntry
	for _, pkg := range lock.Packages {
		ver := strings.TrimPrefix(pkg.Version, "v")
		entries = append(entries, ManifestEntry{
			Ecosystem: "packagist", Name: pkg.Name, Version: ver,
			FilePath: path, Raw: pkg.Version,
		})
	}
	for _, pkg := range lock.PackagesDev {
		ver := strings.TrimPrefix(pkg.Version, "v")
		entries = append(entries, ManifestEntry{
			Ecosystem: "packagist", Name: pkg.Name, Version: ver,
			FilePath: path, Raw: pkg.Version, IsDevDependency: true,
		})
	}
	return entries, nil
}
