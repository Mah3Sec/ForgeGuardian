// Command sbomgen generates SBOMs for packages built by the ForgeGuardian build engine.
// It wraps the build engine runner and immediately serializes the result as a SBOM.
//
// Usage:
//
//	sbomgen --recipe=npm --package=lodash --version=4.17.21
//	sbomgen --recipe=npm --package=lodash --version=4.17.21 --format=cyclonedx-xml
//	sbomgen --recipe=pypi --package=requests --version=2.32.3 --out=requests.spdx.json
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/build/recipes"
	_ "github.com/mah3sec/forgeguardian/internal/build/recipes/all"
	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/sbom"
)

func main() {
	var (
		recipeFlag    = flag.String("recipe", "", "ecosystem recipe (npm|pypi|maven|go|rubygems|crates|huggingface|mcp)")
		packageFlag   = flag.String("package", "", "package name")
		versionFlag   = flag.String("version", "", "package version")
		checksumFlag  = flag.String("checksum", "", "optional expected SHA256")
		sourceURLFlag = flag.String("source-url", "", "optional override source URL")
		formatFlag    = flag.String("format", "cyclonedx-json", "SBOM format: cyclonedx-json|cyclonedx-xml|spdx-json|spdx-tv")
		outFlag       = flag.String("out", "", "output file path (default: stdout)")
		timeoutFlag   = flag.Duration("timeout", 5*time.Minute, "build timeout")
		allFormats    = flag.Bool("all-formats", false, "generate all four SBOM formats and write to files")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *recipeFlag == "" || *packageFlag == "" || *versionFlag == "" {
		logger.Error("--recipe, --package, and --version are required")
		flag.Usage()
		os.Exit(1)
	}

	recipe, err := recipes.Get(*recipeFlag)
	if err != nil {
		logger.Error("recipe not found", "recipe", *recipeFlag)
		os.Exit(1)
	}

	src := core.SourceArtifact{
		Package: core.PackageVersion{
			Ecosystem: *recipeFlag,
			Name:      *packageFlag,
			Version:   *versionFlag,
			Checksum:  *checksumFlag,
			SourceURL: *sourceURLFlag,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
	defer cancel()

	logger.Info("building artifact", "recipe", *recipeFlag, "package", *packageFlag, "version", *versionFlag)
	artifact, err := recipe.Build(ctx, src, nil)
	if err != nil {
		logger.Error("build failed", "error", err)
		os.Exit(1)
	}
	defer os.Remove(artifact.LocalPath)

	logger.Info("build succeeded", "sha256", artifact.SHA256)

	if *allFormats {
		generateAllFormats(logger, artifact, *packageFlag, *versionFlag)
		return
	}

	format := sbom.Format(*formatFlag)
	if err := generateSBOM(artifact, format, *outFlag, logger); err != nil {
		logger.Error("SBOM generation failed", "error", err)
		os.Exit(1)
	}
}

func generateSBOM(artifact core.BuiltArtifact, format sbom.Format, outPath string, logger *slog.Logger) error {
	var w *os.File
	if outPath == "" {
		w = os.Stdout
	} else {
		var err error
		w, err = os.Create(outPath)
		if err != nil {
			return fmt.Errorf("create output file: %w", err)
		}
		defer w.Close()
		logger.Info("writing SBOM", "format", format, "path", outPath)
	}

	if err := sbom.Generate(artifact, format, w); err != nil {
		return fmt.Errorf("generate %s SBOM: %w", format, err)
	}

	if outPath != "" {
		logger.Info("SBOM written", "format", format, "path", outPath)
	}
	return nil
}

func generateAllFormats(logger *slog.Logger, artifact core.BuiltArtifact, name, version string) {
	base := fmt.Sprintf("%s-%s", sanitize(name), version)
	for _, format := range sbom.Formats() {
		outPath := base + sbom.FileExtension(format)
		if err := generateSBOM(artifact, format, outPath, logger); err != nil {
			logger.Error("SBOM generation failed", "format", format, "error", err)
		}
	}
	logger.Info("all formats written",
		"files", strings.Join(listFiles(filepath.Dir(base), base), ", "))
}

func sanitize(s string) string {
	return strings.NewReplacer("/", "-", "@", "", ":", "-").Replace(s)
}

func listFiles(dir, prefix string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(filepath.Base(e.Name()), filepath.Base(prefix)) {
			out = append(out, e.Name())
		}
	}
	return out
}
