package core

import "context"

// Sandbox provides an isolated execution environment for hermetic builds.
type Sandbox interface {
	// Run executes the given command inside the sandbox, returning stdout/stderr.
	Run(ctx context.Context, cmd string, args []string, env map[string]string) (stdout, stderr string, err error)
	// Close tears down the sandbox and releases resources.
	Close() error
}

// BuildRecipe knows how to build a specific ecosystem's packages reproducibly.
type BuildRecipe interface {
	// Ecosystem returns the ecosystem this recipe handles (e.g. "npm").
	Ecosystem() string
	// Build compiles/packages the source artifact inside the sandbox.
	Build(ctx context.Context, src SourceArtifact, sandbox Sandbox) (BuiltArtifact, error)
	// VerifyReproducible rebuilds the artifact and checks hashes match.
	VerifyReproducible(ctx context.Context, artifact BuiltArtifact) (bool, error)
}
