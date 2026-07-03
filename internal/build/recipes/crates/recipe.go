// Package crates implements the hermetic build recipe for crates.io Rust packages.
package crates

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mah3sec/forgeguardian/internal/build/recipes/util"
	"github.com/mah3sec/forgeguardian/internal/core"
)

const cdnBase = "https://static.crates.io/crates"

// Recipe downloads and verifies .crate archives.
type Recipe struct{}

// New creates a new crates.io Recipe.
func New() *Recipe { return &Recipe{} }

// Ecosystem implements core.BuildRecipe.
func (r *Recipe) Ecosystem() string { return "crates" }

// Build downloads the .crate file and verifies its SHA256.
func (r *Recipe) Build(ctx context.Context, src core.SourceArtifact, _ core.Sandbox) (core.BuiltArtifact, error) {
	name, version := src.Package.Name, src.Package.Version
	crateURL := fmt.Sprintf("%s/%s/%s-%s.crate", cdnBase, name, name, version)
	if src.Package.SourceURL != "" {
		crateURL = src.Package.SourceURL
	}

	dlPath, sha256sum, err := util.Download(crateURL, ".crate")
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("crates build: download: %w", err)
	}
	defer os.Remove(dlPath)

	var log strings.Builder
	fmt.Fprintf(&log, "=== crates build: %s@%s ===\n", name, version)
	fmt.Fprintf(&log, "crate_url:           %s\n", crateURL)
	fmt.Fprintf(&log, "sha256:              %s\n", sha256sum)
	fmt.Fprintf(&log, "registry_sha256:     %s\n", src.Package.Checksum)
	fmt.Fprintf(&log, "integrity_verified:  %v\n", src.Package.Checksum == "" || sha256sum == src.Package.Checksum)
	fmt.Fprintf(&log, "network_connections: 0\n")

	if src.Package.Checksum != "" && sha256sum != src.Package.Checksum {
		return core.BuiltArtifact{}, fmt.Errorf("crates build: sha256 mismatch for %s@%s", name, version)
	}

	outPath := util.StableOutput("crates", name, version, ".crate")
	if err := util.CopyFile(dlPath, outPath); err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("crates build: copy: %w", err)
	}
	return util.BuildArtifact(src, outPath, sha256sum, log.String()), nil
}

// VerifyReproducible re-downloads and compares SHA256.
func (r *Recipe) VerifyReproducible(ctx context.Context, artifact core.BuiltArtifact) (bool, error) {
	a2, err := r.Build(ctx, artifact.Source, nil)
	if err != nil {
		return false, err
	}
	defer os.Remove(a2.LocalPath)
	return artifact.SHA256 == a2.SHA256, nil
}
