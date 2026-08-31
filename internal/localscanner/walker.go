package localscanner

import (
	"os"
	"path/filepath"
)

// ManifestFile is a discovered manifest in a directory tree.
type ManifestFile struct {
	Path      string
	Ecosystem string
}

// ManifestNames maps known manifest filenames to their ecosystem. Exported so
// other packages (e.g. internal/remotescan) can discover the same set of
// filenames without hand-maintaining a duplicate list.
var ManifestNames = map[string]string{
	// npm
	"package.json":      "npm",
	"package-lock.json": "npm",
	"yarn.lock":         "npm",
	"pnpm-lock.yaml":    "npm",
	// pypi
	"requirements.txt": "pypi",
	"pyproject.toml":   "pypi",
	"Pipfile":          "pypi",
	"Pipfile.lock":     "pypi",
	"poetry.lock":      "pypi",
	// go
	"go.mod": "go",
	"go.sum": "go",
	// crates
	"Cargo.toml": "crates",
	"Cargo.lock": "crates",
	// maven / gradle
	"pom.xml":           "maven",
	"build.gradle":      "maven",
	"build.gradle.kts":  "maven",
	// rubygems
	"Gemfile":      "rubygems",
	"Gemfile.lock": "rubygems",
	// nuget
	"packages.config": "nuget",
	// packagist (PHP)
	"composer.json": "packagist",
	"composer.lock": "packagist",
}

// SkipDirNames are directory names never descended into during the walk.
// Exported for the same reason as ManifestNames.
var SkipDirNames = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	".git":         true,
	"__pycache__":  true,
	"target":       true,
	".tox":         true,
	"dist":         true,
	"build":        true,
	".venv":        true,
	"venv":         true,
}

// Walk recursively finds all manifest files under rootDir,
// skipping common build/dependency cache directories.
// Symlink cycles are detected by tracking the real (resolved) path of each
// visited directory — revisiting the same inode terminates that branch.
func Walk(rootDir string) ([]ManifestFile, error) {
	abs, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}

	visited := make(map[string]bool)

	var found []ManifestFile
	err = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if SkipDirNames[d.Name()] {
				return filepath.SkipDir
			}
			// Resolve the real path to detect symlink cycles.
			real, resolveErr := filepath.EvalSymlinks(path)
			if resolveErr != nil {
				return nil // unresolvable symlink — skip
			}
			if visited[real] {
				return filepath.SkipDir // already seen this directory tree
			}
			visited[real] = true
			return nil
		}
		if eco, ok := ManifestNames[d.Name()]; ok {
			found = append(found, ManifestFile{Path: path, Ecosystem: eco})
		}
		return nil
	})
	return found, err
}
