package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/sbom"
)

// MonitorResult is the outcome of monitoring a single package version.
type MonitorResult struct {
	Package  core.PackageVersion
	Project  *DTProject
	Findings []core.Finding
	NewVulns int
	Err      error
}

// Worker continuously monitors packages by uploading SBOMs to Dependency-Track
// and polling for new vulnerabilities.
type Worker struct {
	client *Client
	log    *slog.Logger
}

// NewWorker creates a monitoring worker.
func NewWorker(client *Client, log *slog.Logger) *Worker {
	return &Worker{client: client, log: log}
}

// Monitor uploads an SBOM for the given artifact to Dependency-Track and
// returns the current vulnerability findings.
func (w *Worker) Monitor(ctx context.Context, artifact core.BuiltArtifact) MonitorResult {
	pkg := artifact.Source.Package
	result := MonitorResult{Package: pkg}

	// Generate CycloneDX SBOM
	var buf bytes.Buffer
	if err := sbom.Generate(artifact, sbom.FormatCycloneDXJSON, &buf); err != nil {
		result.Err = fmt.Errorf("monitor: generate sbom: %w", err)
		return result
	}

	// Upsert project in Dependency-Track
	project, err := w.client.UpsertProject(ctx, pkg)
	if err != nil {
		result.Err = fmt.Errorf("monitor: upsert project: %w", err)
		return result
	}
	result.Project = project
	w.log.Info("monitor: project ready", "project", project.UUID, "package", pkg.Name)

	// Upload SBOM
	if err := w.client.UploadBOM(ctx, project.UUID, buf.Bytes()); err != nil {
		result.Err = fmt.Errorf("monitor: upload bom: %w", err)
		return result
	}
	w.log.Info("monitor: bom uploaded", "project", project.UUID)

	// Wait for DT analysis to complete (up to 60s)
	if err := w.waitForAnalysis(ctx, project.UUID); err != nil {
		w.log.Warn("monitor: analysis wait timeout, fetching partial results", "error", err)
	}

	// Fetch findings
	dtFindings, err := w.client.GetFindings(ctx, project.UUID)
	if err != nil {
		result.Err = fmt.Errorf("monitor: get findings: %w", err)
		return result
	}

	result.Findings = FindingsToCore(dtFindings)
	result.NewVulns = len(result.Findings)
	w.log.Info("monitor: findings retrieved", "count", result.NewVulns, "package", pkg.Name)
	return result
}

func (w *Worker) waitForAnalysis(ctx context.Context, projectUUID string) error {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
		req, err := w.client.newReq(ctx, http.MethodGet,
			fmt.Sprintf("/api/v1/project/%s", projectUUID), nil)
		if err != nil {
			continue
		}
		resp, err := w.client.http.Do(req)
		if err != nil {
			continue
		}
		var project DTProject
		dec := json.NewDecoder(resp.Body)
		decErr := dec.Decode(&project)
		resp.Body.Close()
		if decErr == nil && !project.LastBOMImport.IsZero() {
			return nil
		}
	}
	return fmt.Errorf("analysis did not complete within 60s")
}
