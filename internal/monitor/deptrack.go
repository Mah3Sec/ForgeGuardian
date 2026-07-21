// Package monitor integrates with Dependency-Track for continuous SBOM monitoring
// and vulnerability tracking across all ingested packages.
package monitor

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
)

// Client is a Dependency-Track API client for continuous monitoring.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New creates a Dependency-Track client.
// baseURL is e.g. "http://localhost:8081" and apiKey is a DT API key with
// PORTFOLIO_MANAGEMENT and VULNERABILITY_ANALYSIS permissions.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// DTProject represents a Dependency-Track project.
type DTProject struct {
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	LastBOMImport time.Time `json:"lastBomImport,omitempty"`
}

// DTVulnerability represents a vulnerability from Dependency-Track.
type DTVuln struct {
	VulnID      string  `json:"vulnId"`
	Source      string  `json:"source"`
	Severity    string  `json:"severity"`
	CVSS        float64 `json:"cvssV3BaseScore,omitempty"`
	Description string  `json:"description"`
	Published   string  `json:"published,omitempty"`
}

// DTFinding is a Dependency-Track finding (component + vulnerability).
type DTFinding struct {
	Component     DTComponent `json:"component"`
	Vulnerability DTVuln      `json:"vulnerability"`
	Analysis      struct {
		State string `json:"state"`
	} `json:"analysis"`
}

// DTComponent is a component within a DT project.
type DTComponent struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	PackageURL  string `json:"purl,omitempty"`
}

// UpsertProject creates or retrieves a DT project for the given package version.
func (c *Client) UpsertProject(ctx context.Context, pkg core.PackageVersion) (*DTProject, error) {
	// Try to find existing project first
	existing, err := c.findProject(ctx, pkg.Name, pkg.Version)
	if err == nil && existing != nil {
		return existing, nil
	}

	body, _ := json.Marshal(map[string]string{
		"name":    pkg.Name,
		"version": pkg.Version,
	})
	req, err := c.newReq(ctx, http.MethodPut, "/api/v1/project", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deptrack: create project: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deptrack: create project status %d", resp.StatusCode)
	}
	var project DTProject
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("deptrack: decode project: %w", err)
	}
	return &project, nil
}

// UploadBOM uploads a CycloneDX BOM to the given project.
// bomJSON should be valid CycloneDX JSON bytes.
func (c *Client) UploadBOM(ctx context.Context, projectUUID string, bomJSON []byte) error {
	encoded := base64.StdEncoding.EncodeToString(bomJSON)
	payload, _ := json.Marshal(map[string]string{
		"project": projectUUID,
		"bom":     encoded,
	})
	req, err := c.newReq(ctx, http.MethodPut, "/api/v1/bom", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("deptrack: upload bom: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("deptrack: upload bom status %d", resp.StatusCode)
	}
	return nil
}

// GetFindings returns all active vulnerability findings for a project.
func (c *Client) GetFindings(ctx context.Context, projectUUID string) ([]DTFinding, error) {
	req, err := c.newReq(ctx, http.MethodGet,
		fmt.Sprintf("/api/v1/finding/project/%s", projectUUID), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deptrack: get findings: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deptrack: get findings status %d", resp.StatusCode)
	}
	var findings []DTFinding
	if err := json.NewDecoder(resp.Body).Decode(&findings); err != nil {
		return nil, fmt.Errorf("deptrack: decode findings: %w", err)
	}
	return findings, nil
}

// FindingsToCore converts Dependency-Track findings to core.Finding slice.
func FindingsToCore(findings []DTFinding) []core.Finding {
	result := make([]core.Finding, 0, len(findings))
	for _, f := range findings {
		if f.Analysis.State == "SUPPRESSED" {
			continue
		}
		result = append(result, core.Finding{
			ID:       f.Vulnerability.VulnID,
			Severity: mapDTSeverity(f.Vulnerability.Severity),
			Type:     "vulnerability",
			Title:    fmt.Sprintf("%s in %s@%s", f.Vulnerability.VulnID, f.Component.Name, f.Component.Version),
			Description: f.Vulnerability.Description,
			Source:   "dependency-track",
			Metadata: map[string]any{
				"component":  f.Component.Name,
				"version":    f.Component.Version,
				"purl":       f.Component.PackageURL,
				"cvss_v3":    f.Vulnerability.CVSS,
				"source":     f.Vulnerability.Source,
			},
		})
	}
	return result
}

// GetMetrics returns portfolio-level vulnerability metrics.
func (c *Client) GetMetrics(ctx context.Context) (map[string]any, error) {
	req, err := c.newReq(ctx, http.MethodGet, "/api/v1/metrics/portfolio/current", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deptrack: metrics: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deptrack: metrics status %d", resp.StatusCode)
	}
	var metrics map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		return nil, fmt.Errorf("deptrack: decode metrics: %w", err)
	}
	return metrics, nil
}

// IsHealthy checks if Dependency-Track is reachable and healthy.
func (c *Client) IsHealthy(ctx context.Context) bool {
	req, err := c.newReq(ctx, http.MethodGet, "/api/version", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c *Client) findProject(ctx context.Context, name, version string) (*DTProject, error) {
	req, err := c.newReq(ctx, http.MethodGet,
		fmt.Sprintf("/api/v1/project?name=%s&version=%s&pageSize=1", name, version), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("not found")
	}
	var projects []DTProject
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil || len(projects) == 0 {
		return nil, fmt.Errorf("not found")
	}
	return &projects[0], nil
}

func (c *Client) newReq(ctx context.Context, method, path string, body *bytes.Reader) (*http.Request, error) {
	url := c.baseURL + path
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, body)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func mapDTSeverity(s string) core.Severity {
	switch s {
	case "CRITICAL":
		return core.SeverityCritical
	case "HIGH":
		return core.SeverityHigh
	case "MEDIUM":
		return core.SeverityMedium
	case "LOW":
		return core.SeverityLow
	default:
		return core.SeverityInformational
	}
}
