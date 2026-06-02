// Package sbom provides SBOM generation in CycloneDX 1.5 and SPDX 2.3 formats.
// Both formats capture ForgeGuardian's AI/MCP-specific security signals as
// first-class properties or annotations.
package sbom

import (
	"fmt"
	"io"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/sbom/cyclonedx"
	"github.com/mah3sec/forgeguardian/internal/sbom/spdx"
)

// Format identifies the SBOM serialization format.
type Format string

const (
	FormatCycloneDXJSON Format = "cyclonedx-json"
	FormatCycloneDXXML  Format = "cyclonedx-xml"
	FormatSPDXJSON      Format = "spdx-json"
	FormatSPDXTV        Format = "spdx-tv"
)

// Formats returns all supported SBOM formats.
func Formats() []Format {
	return []Format{
		FormatCycloneDXJSON,
		FormatCycloneDXXML,
		FormatSPDXJSON,
		FormatSPDXTV,
	}
}

// Generate writes a SBOM in the requested format to w.
func Generate(artifact core.BuiltArtifact, format Format, w io.Writer) error {
	switch format {
	case FormatCycloneDXJSON:
		return cyclonedx.New().GenerateJSON(artifact, w)
	case FormatCycloneDXXML:
		return cyclonedx.New().GenerateXML(artifact, w)
	case FormatSPDXJSON:
		return spdx.New().GenerateJSON(artifact, w)
	case FormatSPDXTV:
		return spdx.New().GenerateTV(artifact, w)
	default:
		return fmt.Errorf("sbom: unknown format %q (supported: cyclonedx-json, cyclonedx-xml, spdx-json, spdx-tv)", format)
	}
}

// FileExtension returns the conventional file extension for a given format.
func FileExtension(format Format) string {
	switch format {
	case FormatCycloneDXJSON:
		return ".cdx.json"
	case FormatCycloneDXXML:
		return ".cdx.xml"
	case FormatSPDXJSON:
		return ".spdx.json"
	case FormatSPDXTV:
		return ".spdx"
	default:
		return ".sbom"
	}
}
