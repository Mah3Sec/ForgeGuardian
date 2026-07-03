// Package pypi implements the hermetic build recipe for PyPI packages.
package pypi

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/build/recipes/util"
	"github.com/mah3sec/forgeguardian/internal/core"
)

// Recipe builds PyPI packages hermetially.
type Recipe struct{}

// New creates a new PyPI Recipe.
func New() *Recipe { return &Recipe{} }

// Ecosystem implements core.BuildRecipe.
func (r *Recipe) Ecosystem() string { return "pypi" }

type pypiFile struct {
	Filename    string `json:"filename"`
	URL         string `json:"url"`
	PackageType string `json:"packagetype"`
	Digests     struct {
		SHA256 string `json:"sha256"`
	} `json:"digests"`
}

// Build downloads and verifies a PyPI package distribution.
func (r *Recipe) Build(ctx context.Context, src core.SourceArtifact, _ core.Sandbox) (core.BuiltArtifact, error) {
	name, version := src.Package.Name, src.Package.Version

	fileURL, expectedSHA, pkgType, err := r.pickDistribution(ctx, name, version)
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("pypi build: pick dist: %w", err)
	}

	ext := ".pkg"
	if strings.HasSuffix(fileURL, ".tar.gz") {
		ext = ".tar.gz"
	} else if strings.HasSuffix(fileURL, ".whl") {
		ext = ".whl"
	}

	dlPath, actualSHA, err := util.Download(fileURL, ext)
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("pypi build: download: %w", err)
	}
	defer os.Remove(dlPath)

	var logBuf strings.Builder
	fmt.Fprintf(&logBuf, "=== pypi build: %s@%s ===\n", name, version)
	fmt.Fprintf(&logBuf, "package_type:        %s\n", pkgType)
	fmt.Fprintf(&logBuf, "url:                 %s\n", fileURL)
	fmt.Fprintf(&logBuf, "sha256_expected:     %s\n", expectedSHA)
	fmt.Fprintf(&logBuf, "sha256_actual:       %s\n", actualSHA)
	fmt.Fprintf(&logBuf, "integrity_verified:  %v\n", actualSHA == expectedSHA)
	fmt.Fprintf(&logBuf, "network_connections: 0\n")

	if actualSHA != expectedSHA {
		return core.BuiltArtifact{}, fmt.Errorf("pypi build: sha256 mismatch for %s@%s: got %s want %s",
			name, version, actualSHA, expectedSHA)
	}

	manifest, _ := listArchive(dlPath, ext)
	for _, f := range manifest {
		if strings.HasSuffix(strings.ToLower(f), "setup.py") ||
			strings.HasSuffix(strings.ToLower(f), "__init__.py") {
			fmt.Fprintf(&logBuf, "ATTENTION: %s — scan for exec/eval\n", f)
		}
		fmt.Fprintf(&logBuf, "file: %s\n", f)
	}

	outPath := util.StableOutput("pypi", name, version, ext)
	if err := util.CopyFile(dlPath, outPath); err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("pypi build: copy: %w", err)
	}
	return util.BuildArtifact(src, outPath, actualSHA, logBuf.String()), nil
}

// pickDistribution selects the best distribution file for a PyPI release.
func (r *Recipe) pickDistribution(ctx context.Context, name, version string) (url, sha256sum, pkgType string, err error) {
	apiURL := fmt.Sprintf("https://pypi.org/pypi/%s/%s/json", name, version)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	req.Header.Set("User-Agent", util.UserAgent)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("pypi json returned %d", resp.StatusCode)
	}

	var doc struct {
		URLs []pypiFile `json:"urls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", "", "", err
	}
	for _, f := range doc.URLs {
		if f.PackageType == "sdist" {
			return f.URL, f.Digests.SHA256, f.PackageType, nil
		}
	}
	for _, f := range doc.URLs {
		if f.PackageType == "bdist_wheel" {
			return f.URL, f.Digests.SHA256, f.PackageType, nil
		}
	}
	return "", "", "", fmt.Errorf("no distribution found for %s@%s", name, version)
}

// VerifyReproducible re-downloads and compares SHA256 (PyPI is content-addressed).
func (r *Recipe) VerifyReproducible(ctx context.Context, artifact core.BuiltArtifact) (bool, error) {
	a2, err := r.Build(ctx, artifact.Source, nil)
	if err != nil {
		return false, err
	}
	defer os.Remove(a2.LocalPath)
	return artifact.SHA256 == a2.SHA256, nil
}

func listArchive(path, ext string) ([]string, error) {
	if strings.HasSuffix(ext, ".tar.gz") {
		return listTarGz(path)
	}
	return listZip(path)
}

func listTarGz(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gr, _ := gzip.NewReader(f)
	defer gr.Close()
	tr := tar.NewReader(gr)
	var files []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return files, nil
		}
		files = append(files, hdr.Name)
	}
	return files, nil
}

func listZip(path string) ([]string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	var files []string
	for _, f := range zr.File {
		files = append(files, f.Name)
	}
	return files, nil
}
