// Package runner implements the build pipeline that drives a BuildRecipe through
// a ProcessSandbox and returns a fully-formed BuiltArtifact.
//
// Sequence:
//  1. Create a fresh ProcessSandbox (ephemeral workdir, minimal env).
//  2. Invoke recipe.Build — the recipe downloads the source and records signals.
//  3. If verify is requested, call recipe.VerifyReproducible.
//  4. Tear down the sandbox.
package runner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mah3sec/forgeguardian/internal/build/sandbox"
	"github.com/mah3sec/forgeguardian/internal/core"
)

// Options controls optional runner behaviour.
type Options struct {
	// VerifyReproducible performs a second build and compares SHA256 hashes.
	VerifyReproducible bool
	// Logger is used for structured output. Nil = discard.
	Logger *slog.Logger
}

// Runner executes a BuildRecipe inside a ProcessSandbox.
type Runner struct {
	opts Options
}

// New returns a Runner with the given options.
func New(opts Options) *Runner {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Runner{opts: opts}
}

// Run builds src using recipe inside a fresh sandbox and returns the artifact.
func (r *Runner) Run(ctx context.Context, recipe core.BuildRecipe, src core.SourceArtifact) (core.BuiltArtifact, error) {
	sb, cleanup, err := sandbox.NewCoreSandbox()
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("runner: create sandbox: %w", err)
	}
	defer func() {
		if cerr := cleanup(); cerr != nil {
			r.opts.Logger.Warn("runner: sandbox cleanup failed", "error", cerr)
		}
	}()

	r.opts.Logger.Info("runner: starting build",
		"ecosystem", recipe.Ecosystem(),
		"package", src.Package.Name,
		"version", src.Package.Version,
	)

	artifact, err := recipe.Build(ctx, src, sb)
	if err != nil {
		return core.BuiltArtifact{}, fmt.Errorf("runner: build failed: %w", err)
	}

	r.opts.Logger.Info("runner: build succeeded",
		"sha256", artifact.SHA256,
		"path", artifact.LocalPath,
	)

	if r.opts.VerifyReproducible {
		ok, err := r.verifyReproducible(ctx, recipe, artifact)
		if err != nil {
			r.opts.Logger.Warn("runner: reproducibility check failed", "error", err)
		} else {
			artifact.Reproducible = &ok
			r.opts.Logger.Info("runner: reproducibility", "match", ok)
		}
	}

	return artifact, nil
}

func (r *Runner) verifyReproducible(ctx context.Context, recipe core.BuildRecipe, artifact core.BuiltArtifact) (bool, error) {
	ok, err := recipe.VerifyReproducible(ctx, artifact)
	if err != nil {
		return false, fmt.Errorf("runner: verify reproducible: %w", err)
	}
	return ok, nil
}
