package signer

import (
	"fmt"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
)

// SLSA provenance predicate types per the SLSA v1.0 spec.
const (
	StatementTypeInToto       = "https://in-toto.io/Statement/v1"
	PredicateTypeSLSAv1       = "https://slsa.dev/provenance/v1"
	BuilderIDForgeGuardian    = "https://github.com/forgeguardian/forgeguardian/.github/workflows/release.yml"
	SLSABuildTypeHermetic     = "https://forgeguardian.dev/build/hermetic/v1"
)

// SLSAProvenance is an in-toto statement with a SLSA v1.0 provenance predicate.
type SLSAProvenance struct {
	Type          string         `json:"_type"`
	Subject       []DigestSubject `json:"subject"`
	PredicateType string         `json:"predicateType"`
	Predicate     SLSAPredicate  `json:"predicate"`
}

// DigestSubject names the artifact and its digest set.
type DigestSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

// SLSAPredicate is the SLSA v1.0 provenance predicate.
type SLSAPredicate struct {
	BuildDefinition BuildDefinition `json:"buildDefinition"`
	RunDetails      RunDetails      `json:"runDetails"`
}

// BuildDefinition captures what was built.
type BuildDefinition struct {
	BuildType            string         `json:"buildType"`
	ExternalParameters   map[string]any `json:"externalParameters"`
	InternalParameters   map[string]any `json:"internalParameters,omitempty"`
	ResolvedDependencies []Dependency   `json:"resolvedDependencies,omitempty"`
}

// Dependency is a resolved build input.
type Dependency struct {
	URI    string            `json:"uri"`
	Digest map[string]string `json:"digest,omitempty"`
}

// RunDetails captures who built it and when.
type RunDetails struct {
	Builder   Builder   `json:"builder"`
	Metadata  BuildMeta `json:"metadata"`
}

// Builder identifies the build system.
type Builder struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
}

// BuildMeta holds timing and invocation metadata.
type BuildMeta struct {
	InvocationID string    `json:"invocationId,omitempty"`
	StartedOn    time.Time `json:"startedOn"`
	FinishedOn   time.Time `json:"finishedOn"`
}

// NewProvenance constructs a SLSA v1.0 provenance statement for a built artifact.
func NewProvenance(artifact core.BuiltArtifact) *SLSAProvenance {
	pkg := artifact.Source.Package

	return &SLSAProvenance{
		Type: StatementTypeInToto,
		Subject: []DigestSubject{
			{
				Name:   fmt.Sprintf("pkg:%s/%s@%s", pkg.Ecosystem, pkg.Name, pkg.Version),
				Digest: map[string]string{"sha256": artifact.SHA256},
			},
		},
		PredicateType: PredicateTypeSLSAv1,
		Predicate: SLSAPredicate{
			BuildDefinition: BuildDefinition{
				BuildType: SLSABuildTypeHermetic,
				ExternalParameters: map[string]any{
					"ecosystem": pkg.Ecosystem,
					"package":   pkg.Name,
					"version":   pkg.Version,
				},
				InternalParameters: map[string]any{
					"reproducible": artifact.Reproducible,
				},
				ResolvedDependencies: []Dependency{
					{
						URI:    pkg.SourceURL,
						Digest: map[string]string{"sha256": artifact.Source.SHA256},
					},
				},
			},
			RunDetails: RunDetails{
				Builder: Builder{
					ID:      BuilderIDForgeGuardian,
					Version: "1.0.0",
				},
				Metadata: BuildMeta{
					StartedOn:  artifact.BuildTime,
					FinishedOn: artifact.BuildTime,
				},
			},
		},
	}
}
