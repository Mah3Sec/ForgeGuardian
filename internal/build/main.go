// Package main is the ForgeGuardian hermetic build worker.
// It accepts a single build job via env vars and writes the result to stdout as
// JSON. Designed to run as a Kubernetes Job or docker run --rm.
//
// Required env vars:
//
//	FG_RECIPE   — ecosystem (npm, pypi, maven, go, rubygems, crates, huggingface, mcp)
//	FG_PACKAGE  — package name
//	FG_VERSION  — package version
//
// Optional:
//
//	FG_CHECKSUM            — expected SHA256 for pre-check
//	FG_VERIFY_REPRODUCIBLE — "true" to perform a second build and compare hashes
//	FG_TIMEOUT             — build timeout (default 10m)
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mah3sec/forgeguardian/internal/build/recipes"
	_ "github.com/mah3sec/forgeguardian/internal/build/recipes/all"
	"github.com/mah3sec/forgeguardian/internal/build/runner"
	"github.com/mah3sec/forgeguardian/internal/core"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("forgeguardian build-worker starting", "version", "0.2.0")

	recipe := mustEnv("FG_RECIPE")
	pkg := mustEnv("FG_PACKAGE")
	version := mustEnv("FG_VERSION")
	checksum := os.Getenv("FG_CHECKSUM")
	verifyReproducible := os.Getenv("FG_VERIFY_REPRODUCIBLE") == "true"

	timeout := 10 * time.Minute
	if t := os.Getenv("FG_TIMEOUT"); t != "" {
		if d, err := time.ParseDuration(t); err == nil {
			timeout = d
		}
	}

	r, err := recipes.Get(recipe)
	if err != nil {
		logger.Error("recipe not found", "recipe", recipe)
		os.Exit(1)
	}

	src := core.SourceArtifact{
		Package: core.PackageVersion{
			Ecosystem: recipe,
			Name:      pkg,
			Version:   version,
			Checksum:  checksum,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	run := runner.New(runner.Options{
		VerifyReproducible: verifyReproducible,
		Logger:             logger,
	})

	artifact, err := run.Run(ctx, r, src)
	if err != nil {
		logger.Error("build failed", "error", err)
		os.Exit(1)
	}

	result := map[string]any{
		"ecosystem":  artifact.Source.Package.Ecosystem,
		"package":    artifact.Source.Package.Name,
		"version":    artifact.Source.Package.Version,
		"local_path": artifact.LocalPath,
		"sha256":     artifact.SHA256,
		"built_at":   artifact.BuildTime,
		"build_log":  artifact.BuildLog,
	}
	if artifact.Reproducible != nil {
		result["reproducible"] = *artifact.Reproducible
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "encode result: %v\n", err)
		os.Exit(1)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "required env var %s is not set\n", key)
		os.Exit(1)
	}
	return v
}
