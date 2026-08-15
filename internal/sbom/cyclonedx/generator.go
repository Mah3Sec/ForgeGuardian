// Package cyclonedx generates CycloneDX 1.5 SBOMs from ForgeGuardian build artifacts.
// AI model and MCP server components are represented using CycloneDX 1.5's
// machine-learning-model component type and custom properties under the
// "forgeguardian" namespace.
package cyclonedx

import (
	"fmt"
	"io"
	"strings"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/google/uuid"
	"github.com/mah3sec/forgeguardian/internal/core"
)

const (
	toolVendor  = "ForgeGuardian"
	toolName    = "forgeguardian-sbom"
	toolVersion = "0.1.0"
	propNS      = "forgeguardian"
)

// Generator produces CycloneDX 1.5 SBOMs.
type Generator struct{}

// New returns a new CycloneDX Generator.
func New() *Generator { return &Generator{} }

// GenerateJSON writes a CycloneDX 1.5 JSON SBOM for the artifact to w.
func (g *Generator) GenerateJSON(artifact core.BuiltArtifact, w io.Writer) error {
	bom := g.buildBOM(artifact)
	enc := cdx.NewBOMEncoder(w, cdx.BOMFileFormatJSON)
	enc.SetPretty(true)
	return enc.EncodeVersion(bom, cdx.SpecVersion1_5)
}

// GenerateXML writes a CycloneDX 1.5 XML SBOM for the artifact to w.
func (g *Generator) GenerateXML(artifact core.BuiltArtifact, w io.Writer) error {
	bom := g.buildBOM(artifact)
	enc := cdx.NewBOMEncoder(w, cdx.BOMFileFormatXML)
	enc.SetPretty(true)
	return enc.EncodeVersion(bom, cdx.SpecVersion1_5)
}

// buildBOM constructs the full CycloneDX BOM structure from a BuiltArtifact.
func (g *Generator) buildBOM(artifact core.BuiltArtifact) *cdx.BOM {
	bom := cdx.NewBOM()
	bom.SerialNumber = "urn:uuid:" + uuid.New().String()

	// Metadata
	now := time.Now().UTC()
	bom.Metadata = &cdx.Metadata{
		Timestamp: now.Format(time.RFC3339),
		Tools: &cdx.ToolsChoice{
			Components: &[]cdx.Component{
				{
					Type:    cdx.ComponentTypeApplication,
					Name:    toolName,
					Version: toolVersion,
					Supplier: &cdx.OrganizationalEntity{
						Name: toolVendor,
					},
				},
			},
		},
		Component: g.buildComponent(artifact),
	}

	// Root component + all sub-components (for future multi-dep graphs)
	rootComp := g.buildComponent(artifact)
	bom.Components = &[]cdx.Component{*rootComp}

	// Build provenance via externalReferences on the metadata component
	bom.Metadata.Component.ExternalReferences = buildExternalRefs(artifact)

	// Dependencies: root depends on nothing by default (filled in by scanner phase)
	bom.Dependencies = &[]cdx.Dependency{
		{
			Ref:          rootComp.BOMRef,
			Dependencies: &[]string{},
		},
	}

	return bom
}

// buildComponent maps a BuiltArtifact to the appropriate CycloneDX Component type.
func (g *Generator) buildComponent(artifact core.BuiltArtifact) *cdx.Component {
	pkg := artifact.Source.Package
	comp := &cdx.Component{
		BOMRef:     bomRef(pkg),
		Name:       pkg.Name,
		Version:    pkg.Version,
		PackageURL: buildPURL(pkg),
		Hashes: &[]cdx.Hash{
			{
				Algorithm: cdx.HashAlgoSHA256,
				Value:     artifact.SHA256,
			},
		},
		Properties: buildProperties(artifact),
	}

	// Component type by ecosystem
	switch pkg.Ecosystem {
	case "huggingface":
		comp.Type = cdx.ComponentTypeMachineLearningModel
		comp.Description = "AI model artifact from HuggingFace Hub"
		injectAIModelProperties(comp, artifact)
	case "mcp":
		comp.Type = cdx.ComponentTypeApplication
		comp.Description = "MCP server package"
		injectMCPProperties(comp, artifact)
	case "npm", "pypi", "rubygems", "crates":
		comp.Type = cdx.ComponentTypeLibrary
	case "maven":
		comp.Type = cdx.ComponentTypeLibrary
	case "go":
		comp.Type = cdx.ComponentTypeLibrary
	default:
		comp.Type = cdx.ComponentTypeApplication
	}

	return comp
}

// buildPURL constructs a Package URL (purl) for the package.
// See https://github.com/package-url/purl-spec
func buildPURL(pkg core.PackageVersion) string {
	switch pkg.Ecosystem {
	case "npm":
		return fmt.Sprintf("pkg:npm/%s@%s", pkg.Name, pkg.Version)
	case "pypi":
		return fmt.Sprintf("pkg:pypi/%s@%s", strings.ToLower(pkg.Name), pkg.Version)
	case "maven":
		// name is "groupId:artifactId"
		parts := strings.SplitN(pkg.Name, ":", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("pkg:maven/%s/%s@%s",
				strings.ReplaceAll(parts[0], ".", "/"), parts[1], pkg.Version)
		}
		return fmt.Sprintf("pkg:maven/%s@%s", pkg.Name, pkg.Version)
	case "go":
		return fmt.Sprintf("pkg:golang/%s@%s", pkg.Name, pkg.Version)
	case "rubygems":
		return fmt.Sprintf("pkg:gem/%s@%s", pkg.Name, pkg.Version)
	case "crates":
		return fmt.Sprintf("pkg:cargo/%s@%s", pkg.Name, pkg.Version)
	case "huggingface":
		// Unofficial but descriptive
		return fmt.Sprintf("pkg:huggingface/%s@%s", pkg.Name, pkg.Version)
	case "mcp":
		return fmt.Sprintf("pkg:npm/%s@%s", pkg.Name, pkg.Version)
	default:
		return fmt.Sprintf("pkg:generic/%s@%s", pkg.Name, pkg.Version)
	}
}

// buildProperties builds the base set of ForgeGuardian custom properties.
func buildProperties(artifact core.BuiltArtifact) *[]cdx.Property {
	pkg := artifact.Source.Package
	props := []cdx.Property{
		prop("ecosystem", pkg.Ecosystem),
		prop("build_time", artifact.BuildTime.Format(time.RFC3339)),
		prop("local_path", artifact.LocalPath),
		prop("hermetic_build", "true"),
		prop("network_connections", extractLogField(artifact.BuildLog, "network_connections")),
	}
	if pkg.Checksum != "" {
		props = append(props, prop("registry_checksum", pkg.Checksum))
	}
	if pkg.SourceURL != "" {
		props = append(props, prop("source_url", pkg.SourceURL))
	}
	return &props
}

// injectAIModelProperties adds AI-specific properties parsed from the build log.
func injectAIModelProperties(comp *cdx.Component, artifact core.BuiltArtifact) {
	extra := []cdx.Property{
		prop("ai:safetensors_count", extractLogField(artifact.BuildLog, "safetensors_count")),
		prop("ai:pickle_count", extractLogField(artifact.BuildLog, "pickle_count")),
		prop("ai:onnx_count", extractLogField(artifact.BuildLog, "onnx_count")),
		prop("ai:suspicious_count", extractLogField(artifact.BuildLog, "suspicious_count")),
		prop("ai:pickle_issues", extractLogField(artifact.BuildLog, "pickle_issues")),
		prop("ai:pipeline_tag", extractLogField(artifact.BuildLog, "pipeline_tag")),
		prop("ai:license", extractLogField(artifact.BuildLog, "license")),
	}
	all := append(*comp.Properties, extra...)
	comp.Properties = &all

	// ModelCard external reference
	modelID := artifact.Source.Package.Name
	refs := []cdx.ExternalReference{
		{
			Type: cdx.ERTypeModelCard,
			URL:  fmt.Sprintf("https://huggingface.co/%s", modelID),
		},
	}
	comp.ExternalReferences = &refs
}

// injectMCPProperties adds MCP server security signal properties from the build log.
func injectMCPProperties(comp *cdx.Component, artifact core.BuiltArtifact) {
	extra := []cdx.Property{
		prop("mcp:tools_declared", extractLogField(artifact.BuildLog, "tools_declared")),
		prop("mcp:shadowed_tools", extractLogField(artifact.BuildLog, "shadowed_tools")),
		prop("mcp:injection_signals", extractLogField(artifact.BuildLog, "injection_signals")),
		prop("mcp:broad_paths", extractLogField(artifact.BuildLog, "broad_paths")),
		prop("mcp:risk_score", extractLogField(artifact.BuildLog, "risk_score")),
		prop("mcp:install_scripts", extractLogField(artifact.BuildLog, "install_scripts")),
	}
	all := append(*comp.Properties, extra...)
	comp.Properties = &all
}

// buildExternalRefs constructs distribution and build-system external references.
func buildExternalRefs(artifact core.BuiltArtifact) *[]cdx.ExternalReference {
	pkg := artifact.Source.Package
	refs := []cdx.ExternalReference{}

	if pkg.SourceURL != "" {
		refs = append(refs, cdx.ExternalReference{
			Type: cdx.ERTypeDistribution,
			URL:  pkg.SourceURL,
		})
	}

	// Registry-specific homepage refs
	switch pkg.Ecosystem {
	case "npm":
		refs = append(refs, cdx.ExternalReference{
			Type: cdx.ERTypeWebsite,
			URL:  fmt.Sprintf("https://www.npmjs.com/package/%s/v/%s", pkg.Name, pkg.Version),
		})
	case "pypi":
		refs = append(refs, cdx.ExternalReference{
			Type: cdx.ERTypeWebsite,
			URL:  fmt.Sprintf("https://pypi.org/project/%s/%s/", pkg.Name, pkg.Version),
		})
	case "go":
		refs = append(refs, cdx.ExternalReference{
			Type: cdx.ERTypeWebsite,
			URL:  fmt.Sprintf("https://pkg.go.dev/%s@%s", pkg.Name, pkg.Version),
		})
	case "crates":
		refs = append(refs, cdx.ExternalReference{
			Type: cdx.ERTypeWebsite,
			URL:  fmt.Sprintf("https://crates.io/crates/%s/%s", pkg.Name, pkg.Version),
		})
	case "rubygems":
		refs = append(refs, cdx.ExternalReference{
			Type: cdx.ERTypeWebsite,
			URL:  fmt.Sprintf("https://rubygems.org/gems/%s/versions/%s", pkg.Name, pkg.Version),
		})
	case "huggingface":
		refs = append(refs, cdx.ExternalReference{
			Type: cdx.ERTypeModelCard,
			URL:  fmt.Sprintf("https://huggingface.co/%s", pkg.Name),
		})
	}

	return &refs
}

// bomRef returns a stable BOM reference string for a package.
func bomRef(pkg core.PackageVersion) string {
	return fmt.Sprintf("%s/%s@%s", pkg.Ecosystem, pkg.Name, pkg.Version)
}

// prop creates a ForgeGuardian-namespaced CycloneDX property.
func prop(key, value string) cdx.Property {
	return cdx.Property{
		Name:  propNS + ":" + key,
		Value: value,
	}
}

// extractLogField retrieves the value of a "key: value" line from a build log.
func extractLogField(log, key string) string {
	prefix := key + ":"
	for _, line := range strings.Split(log, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}
	return ""
}
