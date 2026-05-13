package core

import "context"

// Scanner is the interface all scan engines implement.
type Scanner interface {
	// Name returns a human-readable scanner identifier.
	Name() string
	// Scan analyzes a built artifact and returns its findings.
	Scan(ctx context.Context, artifact BuiltArtifact) ([]Finding, error)
}
