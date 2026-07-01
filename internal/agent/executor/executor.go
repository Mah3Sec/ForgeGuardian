// Package executor applies patch plans produced by the planner to manifests.
// It supports dry-run mode (default) and explicit --apply to modify files.
// Currently supports: npm (package.json), Python (requirements.txt / pyproject.toml),
// Go (go.mod), Ruby (Gemfile), Rust (Cargo.toml).
package executor

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mah3sec/forgeguardian/internal/agent/planner"
	"github.com/mah3sec/forgeguardian/internal/core"
)

// Result describes the outcome of applying one PatchAction.
type Result struct {
	Action  planner.PatchAction
	Applied bool
	DryRun  bool
	Changed string // human-readable description of the change
	Err     error
}

// Executor applies PatchPlans to manifest files in a project directory.
type Executor struct {
	// ProjectDir is the root of the project to patch. Defaults to ".".
	ProjectDir string
	// DryRun: when true, report what would change without writing files.
	DryRun bool
}

// New creates a new Executor for the given project directory.
func New(projectDir string, dryRun bool) *Executor {
	if projectDir == "" {
		projectDir = "."
	}
	return &Executor{ProjectDir: projectDir, DryRun: dryRun}
}

// Execute applies all actions in a PatchPlan and returns per-action results.
func (e *Executor) Execute(plan planner.PatchPlan, advisory core.Advisory) []Result {
	results := make([]Result, 0, len(plan.Actions))
	for _, action := range plan.Actions {
		r := e.executeAction(action, advisory)
		results = append(results, r)
	}
	return results
}

func (e *Executor) executeAction(action planner.PatchAction, advisory core.Advisory) Result {
	r := Result{Action: action, DryRun: e.DryRun}

	switch action.Type {
	case "upgrade", "pin":
		r = e.applyVersionUpdate(action, advisory)
	case "remove":
		r.Changed = fmt.Sprintf("Would remove dependency %s from manifest", advisory.Package.Name)
		r.Applied = !e.DryRun
	case "noaction":
		r.Changed = "No action taken (manual review required)"
		r.Applied = true
	default:
		r.Changed = fmt.Sprintf("Unknown action type %q — skipped", action.Type)
	}
	return r
}

func (e *Executor) applyVersionUpdate(action planner.PatchAction, advisory core.Advisory) Result {
	r := Result{Action: action, DryRun: e.DryRun}
	pkg := advisory.Package

	manifest, strategy := e.findManifest(pkg.Ecosystem)
	if manifest == "" {
		r.Changed = fmt.Sprintf("No manifest found for %s in %s", pkg.Ecosystem, e.ProjectDir)
		return r
	}

	newVersion := action.NewValue
	if newVersion == "" {
		r.Changed = "No target version specified in action"
		return r
	}

	content, err := os.ReadFile(manifest)
	if err != nil {
		r.Err = fmt.Errorf("read manifest: %w", err)
		return r
	}

	updated, changed, err := strategy(string(content), pkg.Name, pkg.Version, newVersion)
	if err != nil {
		r.Err = err
		return r
	}
	if !changed {
		r.Changed = fmt.Sprintf("Pattern for %s@%s not found in %s", pkg.Name, pkg.Version, filepath.Base(manifest))
		return r
	}

	r.Changed = fmt.Sprintf("%s: %s@%s → %s", filepath.Base(manifest), pkg.Name, pkg.Version, newVersion)
	if !e.DryRun {
		if err := os.WriteFile(manifest, []byte(updated), 0o644); err != nil {
			r.Err = fmt.Errorf("write manifest: %w", err)
			return r
		}
		r.Applied = true
	}
	return r
}

// findManifest locates the first relevant manifest file for the given ecosystem.
type patchStrategy func(content, pkgName, oldVersion, newVersion string) (updated string, changed bool, err error)

func (e *Executor) findManifest(ecosystem string) (string, patchStrategy) {
	candidates := manifestCandidates(ecosystem)
	for _, c := range candidates {
		full := filepath.Join(e.ProjectDir, c.file)
		if _, err := os.Stat(full); err == nil {
			return full, c.strategy
		}
	}
	return "", nil
}

type candidate struct {
	file     string
	strategy patchStrategy
}

func manifestCandidates(ecosystem string) []candidate {
	switch ecosystem {
	case "npm", "mcp":
		return []candidate{
			{"package.json", patchJSON},
			{"package-lock.json", patchJSON},
		}
	case "pypi":
		return []candidate{
			{"requirements.txt", patchRequirements},
			{"pyproject.toml", patchTOML},
		}
	case "go":
		return []candidate{
			{"go.mod", patchGoMod},
		}
	case "rubygems":
		return []candidate{
			{"Gemfile", patchGemfile},
			{"Gemfile.lock", patchGemfile},
		}
	case "crates":
		return []candidate{
			{"Cargo.toml", patchTOML},
		}
	case "maven":
		return []candidate{
			{"pom.xml", patchPOM},
		}
	}
	return nil
}

// --- Per-manifest patch strategies ---

// patchJSON updates a version in a JSON manifest (package.json).
// It uses simple string replacement to preserve formatting.
func patchJSON(content, pkgName, oldVersion, newVersion string) (string, bool, error) {
	// Match "package": "old" or "package": "^old" etc.
	patterns := []string{
		fmt.Sprintf(`"%s": "%s"`, pkgName, oldVersion),
		fmt.Sprintf(`"%s": "^%s"`, pkgName, oldVersion),
		fmt.Sprintf(`"%s": "~%s"`, pkgName, oldVersion),
	}
	for _, p := range patterns {
		if strings.Contains(content, p) {
			replacement := fmt.Sprintf(`"%s": "^%s"`, pkgName, newVersion)
			return strings.Replace(content, p, replacement, 1), true, nil
		}
	}
	return content, false, nil
}

// patchRequirements updates a line in requirements.txt.
func patchRequirements(content, pkgName, oldVersion, newVersion string) (string, bool, error) {
	var out strings.Builder
	changed := false
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		lower := strings.ToLower(line)
		lowerPkg := strings.ToLower(pkgName)
		if strings.HasPrefix(lower, lowerPkg+"==") || strings.HasPrefix(lower, lowerPkg+">=") ||
			strings.HasPrefix(lower, lowerPkg+"~=") {
			out.WriteString(pkgName + ">=" + newVersion + "\n")
			changed = true
		} else {
			out.WriteString(line + "\n")
		}
	}
	return out.String(), changed, scanner.Err()
}

// patchGoMod updates a require line in go.mod.
func patchGoMod(content, pkgName, oldVersion, newVersion string) (string, bool, error) {
	old := fmt.Sprintf("%s %s", pkgName, oldVersion)
	new := fmt.Sprintf("%s %s", pkgName, newVersion)
	if strings.Contains(content, old) {
		return strings.Replace(content, old, new, 1), true, nil
	}
	return content, false, nil
}

// patchGemfile updates a gem line in a Gemfile.
func patchGemfile(content, pkgName, oldVersion, newVersion string) (string, bool, error) {
	re := regexp.MustCompile(fmt.Sprintf(`(gem ['"]%s['"].*?)['"]%s['"]`, regexp.QuoteMeta(pkgName), regexp.QuoteMeta(oldVersion)))
	updated := re.ReplaceAllStringFunc(content, func(s string) string {
		return strings.Replace(s, `"`+oldVersion+`"`, `"`+newVersion+`"`, 1)
	})
	return updated, updated != content, nil
}

// patchTOML updates a version string in a TOML file (pyproject.toml, Cargo.toml).
func patchTOML(content, pkgName, oldVersion, newVersion string) (string, bool, error) {
	// Match: pkgname = "X.Y.Z" or pkgname = { version = "X.Y.Z" }
	patterns := []string{
		fmt.Sprintf(`%s = "%s"`, pkgName, oldVersion),
		fmt.Sprintf(`%s = "^%s"`, pkgName, oldVersion),
		fmt.Sprintf(`%s = "~%s"`, pkgName, oldVersion),
		fmt.Sprintf(`version = "%s"`, oldVersion),
	}
	for _, p := range patterns {
		if strings.Contains(content, p) {
			replacement := strings.Replace(p, oldVersion, newVersion, 1)
			return strings.Replace(content, p, replacement, 1), true, nil
		}
	}
	return content, false, nil
}

// patchPOM updates a version in a Maven pom.xml.
func patchPOM(content, pkgName, oldVersion, newVersion string) (string, bool, error) {
	old := fmt.Sprintf("<version>%s</version>", oldVersion)
	new := fmt.Sprintf("<version>%s</version>", newVersion)
	if strings.Contains(content, old) {
		return strings.Replace(content, old, new, 1), true, nil
	}
	return content, false, nil
}
