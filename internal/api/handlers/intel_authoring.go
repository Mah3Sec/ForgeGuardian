package handlers

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/intelligence"
	"github.com/mah3sec/forgeguardian/internal/scanner"
)

// ─── Intelligence authoring endpoints ──────────────────────────────────────────

// generateSignatureRequest mirrors intelligence.SignatureYAML minus the
// server-computed ID field.
type generateSignatureRequest struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Ecosystem    string   `json:"ecosystem"`
	Severity     string   `json:"severity"`
	Package      string   `json:"package,omitempty"`
	VersionRange string   `json:"version_range,omitempty"`
	Target       string   `json:"target,omitempty"`
	SimilarNames []string `json:"similar_names,omitempty"`
	Pattern      string   `json:"pattern,omitempty"`
	Rule         string   `json:"rule,omitempty"`
	Description  string   `json:"description"`
	CVE          string   `json:"cve,omitempty"`
	References   []string `json:"references,omitempty"`
	Author       string   `json:"author,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

// GenerateSignature builds a complete, ID-assigned signature YAML document
// from the submitted fields, without writing it to disk.
// POST /api/v1/intelligence/signatures
func (h *Handler) GenerateSignature(c *gin.Context) {
	var req generateSignatureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name == "" || req.Type == "" || req.Ecosystem == "" || req.Severity == "" || req.Description == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name, type, ecosystem, severity, and description are required"})
		return
	}

	author := req.Author
	if author == "" {
		author = "anonymous"
	}

	sigType := intelligence.SignatureType(req.Type)
	tags := req.Tags
	if len(tags) == 0 {
		tags = intelligence.DefaultTags(sigType)
	}

	sigID := intelligence.BuildSignatureID(sigType, req.Ecosystem, req.Name)

	sig := intelligence.SignatureYAML{
		ID:           sigID,
		Name:         req.Name,
		Type:         req.Type,
		Ecosystem:    req.Ecosystem,
		Severity:     req.Severity,
		Package:      req.Package,
		VersionRange: req.VersionRange,
		Target:       req.Target,
		SimilarNames: req.SimilarNames,
		Pattern:      req.Pattern,
		Rule:         req.Rule,
		Description:  req.Description,
		CVE:          req.CVE,
		References:   req.References,
		Author:       author,
		Tags:         tags,
	}

	yamlBytes, err := yaml.Marshal(sig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Persist the new signature to the intel store so scanner engines pick it up.
	storePath := h.cfg.GetIntelStorePath()
	if storePath != "" {
		store, loadErr := intelligence.LoadStore(storePath)
		if loadErr != nil {
			store = &intelligence.SignatureStore{Version: 1}
		}
		det := intelligence.ToDetectionSignature(sig)
		added := intelligence.MergeSignatures(store, []intelligence.DetectionSignature{det})
		if added > 0 {
			if saveErr := intelligence.SaveStore(storePath, store); saveErr != nil {
				h.log.Warn("failed to persist authored signature", "error", saveErr, "id", sigID)
			}
		}
	}

	filename := sigID + ".yaml"
	c.JSON(http.StatusOK, gin.H{
		"id":             sigID,
		"yaml":           string(yamlBytes),
		"filename":       filename,
		"suggested_path": "signatures/" + intelligence.SigTypeDir(sigType) + "/" + req.Ecosystem + "/" + filename,
		"persisted":      storePath != "",
	})
}

// ValidateSignatureYAML validates a raw signature YAML document against the
// community signature schema.
// POST /api/v1/intelligence/validate  body: {"yaml": "<raw yaml text>"}
func (h *Handler) ValidateSignatureYAML(c *gin.Context) {
	var req struct {
		YAML string `json:"yaml" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var sig intelligence.SignatureYAML
	if err := yaml.Unmarshal([]byte(req.YAML), &sig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid YAML: " + err.Error()})
		return
	}

	problems := intelligence.ValidateSignature(sig)
	c.JSON(http.StatusOK, gin.H{
		"valid":    len(problems) == 0,
		"problems": problems,
	})
}

// TestSignature runs a lightweight, heuristic test of a signature against a
// live scan of the named package/version. It does NOT download the actual
// artifact bytes (that is CLI-only, via `fgctl intel test`), so it can only
// approximate matches based on the metadata-driven scan engines.
// POST /api/v1/intelligence/test  body: {"yaml":"...","ecosystem":"npm","package":"lodash","version":"4.17.20"}
func (h *Handler) TestSignature(c *gin.Context) {
	var req struct {
		YAML      string `json:"yaml"      binding:"required"`
		Ecosystem string `json:"ecosystem" binding:"required"`
		Package   string `json:"package"   binding:"required"`
		Version   string `json:"version"   binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var sig intelligence.SignatureYAML
	if err := yaml.Unmarshal([]byte(req.YAML), &sig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid YAML: " + err.Error()})
		return
	}

	artifact := core.BuiltArtifact{
		Source: core.SourceArtifact{
			Package: core.PackageVersion{
				Ecosystem: req.Ecosystem,
				Name:      req.Package,
				Version:   req.Version,
			},
		},
		LocalPath: "",
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	results := scanner.NewWithIntelligence(h.cfg.GetIntelStorePath()).Scan(ctx, artifact)
	findings := scanner.MergeFindings(results)

	matched := false
	for _, f := range findings {
		switch intelligence.SignatureType(sig.Type) {
		case intelligence.SigBlocklisted:
			if sig.Package != "" && (strings.Contains(f.Title, sig.Package) || strings.Contains(f.Description, sig.Package)) {
				matched = true
			}
		case intelligence.SigTypoTarget:
			if sig.Target != "" && (strings.Contains(f.Title, sig.Target) || strings.Contains(f.Description, sig.Target)) {
				matched = true
			}
		case intelligence.SigMalwarePattern, intelligence.SigMCPInjection:
			if sig.Pattern != "" {
				if re, err := regexp.Compile(sig.Pattern); err == nil {
					if re.MatchString(f.Title) || re.MatchString(f.Description) {
						matched = true
					}
				} else if strings.Contains(f.Title, sig.Pattern) || strings.Contains(f.Description, sig.Pattern) {
					matched = true
				}
			}
		case intelligence.SigBehavioral, intelligence.SigPickleRule:
			if sig.Rule != "" && (strings.Contains(f.Title, sig.Rule) || strings.Contains(f.Description, sig.Rule)) {
				matched = true
			}
		}
		if matched {
			break
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"matched":  matched,
		"findings": findings,
		"note":     "heuristic match against live scan — for a full artifact-content test use: fgctl intel test <file> --ecosystem=... --package=... --version=...",
	})
}
