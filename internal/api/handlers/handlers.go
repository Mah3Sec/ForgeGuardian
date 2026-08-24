// Package handlers implements all ForgeGuardian REST API handlers.
package handlers

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"regexp"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"

	"github.com/mah3sec/forgeguardian/internal/agent/triage"
	dbsqlc "github.com/mah3sec/forgeguardian/internal/api/db/sqlc"
	"github.com/mah3sec/forgeguardian/internal/auth"
	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/intelligence"
	"github.com/mah3sec/forgeguardian/internal/localscanner"
	"github.com/mah3sec/forgeguardian/internal/notify"
	"github.com/mah3sec/forgeguardian/internal/policy"
	"github.com/mah3sec/forgeguardian/internal/sbom"
	"github.com/mah3sec/forgeguardian/internal/scanner"
	"github.com/mah3sec/forgeguardian/internal/signer"
)

// Config is the subset of server config needed by handlers.
type Config interface {
	GetAnthropicKey() string
	GetRekorURL() string
	GetIntelStorePath() string
	GetAdminIdentity() *auth.AdminIdentity
	GetSessionSecret() []byte
	GetCookieSecure() bool
}

// ServerConfig adapts the main package config to this interface.
type ServerConfig struct {
	AnthropicAPIKey string
	RekorURL        string
	IntelStorePath  string
	AdminIdentity   *auth.AdminIdentity // nil when dashboard login is disabled
	SessionSecret   []byte
	CookieSecure    bool
}

func (s *ServerConfig) GetAnthropicKey() string               { return s.AnthropicAPIKey }
func (s *ServerConfig) GetRekorURL() string                   { return s.RekorURL }
func (s *ServerConfig) GetIntelStorePath() string             { return s.IntelStorePath }
func (s *ServerConfig) GetAdminIdentity() *auth.AdminIdentity { return s.AdminIdentity }
func (s *ServerConfig) GetSessionSecret() []byte              { return s.SessionSecret }
func (s *ServerConfig) GetCookieSecure() bool                 { return s.CookieSecure }

// cachedScanResult holds an in-memory record of a completed scan so that the
// dashboard can display stats, graphs, and activity without PostgreSQL.
type cachedScanResult struct {
	Ecosystem       string             `json:"ecosystem"`
	Name            string             `json:"name"`
	Version         string             `json:"version"`
	SHA256          string             `json:"sha256"`
	Summary         scanner.ScanSummary `json:"summary"`
	HighestSeverity string             `json:"highest_severity"`
	ScannedAt       time.Time          `json:"scanned_at"`
	TotalFindings   int                `json:"total_findings"`
	Workspace       string             `json:"workspace,omitempty"`
}

// scanCache is a bounded in-memory store of recent scan results. It serves as
// the data source for all dashboard endpoints when no database is configured.
// When a persist path is set, the cache auto-saves to disk on every add so
// data survives container restarts without PostgreSQL.
type scanCache struct {
	mu          sync.RWMutex
	results     []cachedScanResult
	persistPath string
}

func newScanCache(persistPath string) *scanCache {
	sc := &scanCache{persistPath: persistPath}
	sc.load()
	return sc
}

func (sc *scanCache) add(r cachedScanResult) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.results = append(sc.results, r)
	if len(sc.results) > 500 {
		sc.results = sc.results[len(sc.results)-500:]
	}
	sc.saveLocked()
}

func (sc *scanCache) list() []cachedScanResult {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	cp := make([]cachedScanResult, len(sc.results))
	copy(cp, sc.results)
	return cp
}

// latest returns deduplicated results keeping only the most recent scan per
// ecosystem/name@version key, which prevents re-scanning from inflating stats.
func (sc *scanCache) latest() []cachedScanResult {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	seen := map[string]int{}
	var out []cachedScanResult
	for i := len(sc.results) - 1; i >= 0; i-- {
		key := sc.results[i].Ecosystem + "/" + sc.results[i].Name + "@" + sc.results[i].Version
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = i
		out = append(out, sc.results[i])
	}
	return out
}

// latestForWorkspace returns deduplicated results filtered by workspace name.
// An empty workspace string returns all results (backward-compatible).
func (sc *scanCache) latestForWorkspace(ws string) []cachedScanResult {
	all := sc.latest()
	if ws == "" {
		return all
	}
	var out []cachedScanResult
	for _, r := range all {
		if strings.EqualFold(r.Workspace, ws) {
			out = append(out, r)
		}
	}
	return out
}

// workspaces returns a deduplicated list of workspace names seen in the cache.
func (sc *scanCache) workspaces() []string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	seen := map[string]bool{}
	var out []string
	for _, r := range sc.results {
		ws := r.Workspace
		if ws == "" {
			ws = "Default"
		}
		if !seen[ws] {
			seen[ws] = true
			out = append(out, ws)
		}
	}
	return out
}

func (sc *scanCache) load() {
	if sc.persistPath == "" {
		return
	}
	data, err := os.ReadFile(sc.persistPath)
	if err != nil {
		return
	}
	var results []cachedScanResult
	if err := json.Unmarshal(data, &results); err != nil {
		return
	}
	sc.mu.Lock()
	sc.results = results
	sc.mu.Unlock()
}

func (sc *scanCache) saveLocked() {
	if sc.persistPath == "" {
		return
	}
	data, err := json.Marshal(sc.results)
	if err != nil {
		return
	}
	dir := filepath.Dir(sc.persistPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(sc.persistPath, data, 0o644)
}

// Handler holds all dependencies for API handlers.
// logEntry is a single log line captured from the structured logger.
type logEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Msg     string `json:"msg"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// logRing is a fixed-size circular buffer that captures recent log entries
// so the dashboard can display server-side logs.
type logRing struct {
	mu      sync.RWMutex
	entries []logEntry
	max     int
}

func newLogRing(max int) *logRing {
	return &logRing{entries: make([]logEntry, 0, max), max: max}
}

func (lr *logRing) add(e logEntry) {
	lr.mu.Lock()
	defer lr.mu.Unlock()
	lr.entries = append(lr.entries, e)
	if len(lr.entries) > lr.max {
		lr.entries = lr.entries[len(lr.entries)-lr.max:]
	}
}

func (lr *logRing) list(limit int) []logEntry {
	lr.mu.RLock()
	defer lr.mu.RUnlock()
	if limit <= 0 || limit > len(lr.entries) {
		limit = len(lr.entries)
	}
	start := len(lr.entries) - limit
	cp := make([]logEntry, limit)
	copy(cp, lr.entries[start:])
	return cp
}

// logCaptureHandler is an slog.Handler that tees log records into the ring buffer.
type logCaptureHandler struct {
	inner slog.Handler
	ring  *logRing
}

func (h *logCaptureHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *logCaptureHandler) Handle(ctx context.Context, r slog.Record) error {
	fields := map[string]any{}
	r.Attrs(func(a slog.Attr) bool {
		fields[a.Key] = a.Value.Any()
		return true
	})
	var f map[string]any
	if len(fields) > 0 {
		f = fields
	}
	h.ring.add(logEntry{
		Time:   r.Time.Format(time.RFC3339),
		Level:  r.Level.String(),
		Msg:    r.Message,
		Fields: f,
	})
	return h.inner.Handle(ctx, r)
}

func (h *logCaptureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &logCaptureHandler{inner: h.inner.WithAttrs(attrs), ring: h.ring}
}

func (h *logCaptureHandler) WithGroup(name string) slog.Handler {
	return &logCaptureHandler{inner: h.inner.WithGroup(name), ring: h.ring}
}

type Handler struct {
	cfg           Config
	log           *slog.Logger
	db            *dbsqlc.Queries // nil when DATABASE_URL is not set
	jobs          *jobRegistry
	cache         *scanCache
	logs          *logRing
	monitorMu     sync.Mutex
	monitorEvents []monitorEvent
}

// New creates a handler suite. pool may be nil; DB-backed endpoints will return
// 503 gracefully when the database is unavailable.
func New(cfg Config, log *slog.Logger, pool *pgxpool.Pool) *Handler {
	cachePath := os.Getenv("FG_CACHE_PATH")
	if cachePath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		cachePath = filepath.Join(home, ".forgeguardian", "scan-cache.json")
	}
	h := &Handler{cfg: cfg, log: log, jobs: newJobRegistry(), cache: newScanCache(cachePath), logs: newLogRing(500)}
	if pool != nil {
		h.db = dbsqlc.New(pool)
	}
	return h
}

// LogRing returns the log ring buffer so main.go can wire it into the slog handler.
func (h *Handler) LogRing() *logRing { return h.logs }

// NewLogCaptureHandler wraps an slog.Handler to tee records into the ring buffer.
func NewLogCaptureHandler(inner slog.Handler, ring *logRing) slog.Handler {
	return &logCaptureHandler{inner: inner, ring: ring}
}

// dbAvailable returns false and writes a 503 when no DB connection is present.
func (h *Handler) dbAvailable(c *gin.Context) bool {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not configured"})
		return false
	}
	return true
}

// ─── Package endpoints ────────────────────────────────────────────────────────

// ListPackages returns a paginated list of packages.
// GET /api/v1/packages?page=1&page_size=50&ecosystem=npm
func (h *Handler) ListPackages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	eco := c.Query("ecosystem")
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 50
	}

	if h.db != nil {
		offset := int32((page - 1) * size)
		packages, err := h.db.ListPackages(c.Request.Context(), eco, int32(size), offset)
		if err != nil {
			h.log.Error("ListPackages query failed", "error", err)
			internalError(c, h.log, "operation failed", err)
			return
		}
		total, err := h.db.CountPackages(c.Request.Context(), eco)
		if err != nil {
			h.log.Error("CountPackages query failed", "error", err)
			internalError(c, h.log, "operation failed", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"page":      page,
			"page_size": size,
			"total":     total,
			"packages":  packages,
		})
		return
	}

	type cachedPkg struct {
		Ecosystem string `json:"ecosystem"`
		Name      string `json:"name"`
		Versions  int    `json:"version_count"`
		Findings  int    `json:"total_findings"`
	}
	cached := h.cache.latest()
	pkgMap := map[string]*cachedPkg{}
	versionMap := map[string]map[string]bool{}
	for _, r := range cached {
		if eco != "" && r.Ecosystem != eco {
			continue
		}
		key := r.Ecosystem + "/" + r.Name
		if _, ok := pkgMap[key]; !ok {
			pkgMap[key] = &cachedPkg{Ecosystem: r.Ecosystem, Name: r.Name}
			versionMap[key] = map[string]bool{}
		}
		versionMap[key][r.Version] = true
		pkgMap[key].Findings += r.Summary.Total
	}
	for key, p := range pkgMap {
		p.Versions = len(versionMap[key])
	}
	pkgs := make([]cachedPkg, 0, len(pkgMap))
	for _, p := range pkgMap {
		pkgs = append(pkgs, *p)
	}
	total := len(pkgs)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	c.JSON(http.StatusOK, gin.H{
		"page":      page,
		"page_size": size,
		"total":     total,
		"packages":  pkgs[start:end],
	})
}

// GetPackage returns detail for a single package.
// GET /api/v1/packages/:ecosystem/:name
func (h *Handler) GetPackage(c *gin.Context) {
	eco := c.Param("ecosystem")
	name := c.Param("name")
	if h.db != nil {
		pkg, err := h.db.GetPackage(c.Request.Context(), eco, name)
		if err != nil {
			if errNoRows(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
				return
			}
			internalError(c, h.log, "operation failed", err)
			return
		}
		c.JSON(http.StatusOK, pkg)
		return
	}
	for _, r := range h.cache.latest() {
		if strings.EqualFold(r.Ecosystem, eco) && strings.EqualFold(r.Name, name) {
			c.JSON(http.StatusOK, gin.H{
				"ecosystem":  r.Ecosystem,
				"name":       r.Name,
				"latest":     r.Version,
				"risk_score": riskScore(r.Summary.Critical, r.Summary.High, r.Summary.Medium, r.Summary.Low),
				"scanned_at": r.ScannedAt,
			})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
}

// ListVersions returns all known versions of a package.
// GET /api/v1/packages/:ecosystem/:name/versions
func (h *Handler) ListVersions(c *gin.Context) {
	eco := c.Param("ecosystem")
	name := c.Param("name")
	if h.db != nil {
		versions, err := h.db.ListVersions(c.Request.Context(), eco, name)
		if err != nil {
			internalError(c, h.log, "operation failed", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"ecosystem": eco, "name": name, "versions": versions})
		return
	}
	var vers []gin.H
	for _, r := range h.cache.latest() {
		if strings.EqualFold(r.Ecosystem, eco) && strings.EqualFold(r.Name, name) {
			vers = append(vers, gin.H{"version": r.Version, "scanned_at": r.ScannedAt})
		}
	}
	c.JSON(http.StatusOK, gin.H{"ecosystem": eco, "name": name, "versions": vers})
}

// ─── Scan endpoints ───────────────────────────────────────────────────────────

// TriggerScan downloads the package from its registry, runs ALL scan engines
// against the real artifact files, and returns a job ID immediately — the
// scan itself runs asynchronously on the job worker pool. Large scans
// (grype/trivy/semgrep) can run well past typical HTTP client timeouts, so
// the caller polls GET /api/v1/jobs/:id for status and the final result.
// POST /api/v1/scan   body: {"ecosystem":"npm","package":"lodash","version":"4.17.21"}
func (h *Handler) TriggerScan(c *gin.Context) {
	var req struct {
		Ecosystem string `json:"ecosystem" binding:"required"`
		Package   string `json:"package"   binding:"required"`
		Version   string `json:"version"   binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !isValidPackageInput(req.Ecosystem, req.Package, req.Version) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ecosystem, package name, or version"})
		return
	}

	job := h.jobs.submit(func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		pkgLabel := fmt.Sprintf("%s@%s", req.Package, req.Version)
		notifyCfg, _ := loadNotifyConfig()
		scanStart := time.Now()

		notify.NotifyScanStart(notifyCfg, notify.ScanEvent{
			Package:  pkgLabel,
			ScanType: "registry",
			Target:   fmt.Sprintf("%s/%s", req.Ecosystem, pkgLabel),
		})

		// Download artifact from registry and extract to temp dir.
		artifact, cleanup, dlErr := downloadArtifact(ctx, req.Ecosystem, req.Package, req.Version)
		if dlErr != nil {
			// Fallback: run OSV + behavioral only (no local file needed).
			h.log.Warn("artifact download failed — running API-only engines", "error", dlErr,
				"ecosystem", req.Ecosystem, "package", req.Package, "version", req.Version)
			artifact = core.BuiltArtifact{
				Source: core.SourceArtifact{
					Package: core.PackageVersion{
						Ecosystem: req.Ecosystem,
						Name:      req.Package,
						Version:   req.Version,
					},
				},
				SHA256:    "",
				LocalPath: "",
			}
			cleanup = func() {}
		}
		defer cleanup()

		agentBus.publish(AgentEvent{
			Type:    "start",
			Message: fmt.Sprintf("Scanning %s@%s (%s)", req.Package, req.Version, req.Ecosystem),
			Package: req.Package,
			Version: req.Version,
		})

		results := scanner.NewWithIntelligence(h.cfg.GetIntelStorePath()).Scan(ctx, artifact)

		agentBus.publish(AgentEvent{
			Type:    "step",
			Message: fmt.Sprintf("Merging findings from %d engines", len(results)),
			Package: req.Package,
			Version: req.Version,
		})

		var checker scanner.AllowlistChecker
		if h.db != nil {
			checker = h.db
		}
		findings := scanner.MergeFindingsFiltered(results, checker, ctx, req.Package, req.Ecosystem)

		pol, _ := policy.Load()
		if pol != nil {
			findings = pol.Enforce(req.Package, req.Version, findings)
		}

		summary := scanner.Summarize(findings)

		var engineNames []string
		for _, r := range results {
			engineNames = append(engineNames, r.Scanner)
		}
		notify.NotifyScanComplete(notifyCfg, notify.ScanEvent{
			Package:  pkgLabel,
			ScanType: "registry",
			Target:   fmt.Sprintf("%s/%s", req.Ecosystem, pkgLabel),
			Engines:  engineNames,
			Duration: time.Since(scanStart),
			Findings: findings,
			Summary: &notify.ScanSummaryInfo{
				Total:      summary.Total,
				Critical:   summary.Critical,
				High:       summary.High,
				Medium:     summary.Medium,
				Low:        summary.Low,
				HighestSev: string(summary.HighestSev),
			},
		})

		h.cache.add(cachedScanResult{
			Ecosystem:       req.Ecosystem,
			Name:            req.Package,
			Version:         req.Version,
			SHA256:          artifact.SHA256,
			Summary:         summary,
			HighestSeverity: string(summary.HighestSev),
			ScannedAt:       time.Now(),
		})

		if h.db != nil {
			if err := h.persistScanResult(ctx, req.Ecosystem, req.Package, req.Version, artifact.SHA256, findings, summary, ""); err != nil {
				h.log.Warn("failed to persist scan result", "error", err)
			}
		}

		// Surface which engines ran vs skipped so dashboard can show it
		engineStatus := make([]map[string]any, 0, len(results))
		for _, r := range results {
			status := "ok"
			msg := ""
			if r.Err != nil {
				status = "skipped"
				msg = r.Err.Error()
			}
			engineStatus = append(engineStatus, map[string]any{
				"engine":   r.Scanner,
				"status":   status,
				"findings": len(r.Findings),
				"error":    msg,
			})
		}

		policyPass := pol == nil || !pol.ShouldFail(findings)

		downloaded := dlErr == nil

		agentBus.publish(AgentEvent{
			Type:    "done",
			Message: fmt.Sprintf("Scan complete: %d findings for %s@%s", summary.Total, req.Package, req.Version),
			Package: req.Package,
			Version: req.Version,
		})

		return gin.H{
			"package":     fmt.Sprintf("%s@%s", req.Package, req.Version),
			"sha256":      artifact.SHA256,
			"downloaded":  downloaded,
			"engines":     engineStatus,
			"summary":     summary,
			"findings":    findings,
			"policy_pass": policyPass,
		}, nil
	})

	c.JSON(http.StatusAccepted, job)
}

// ScanUpload accepts a multipart file upload (tarball or zip of a project),
// extracts it, and runs all scan engines asynchronously — returns a job ID
// immediately, same async model as TriggerScan. Poll GET /api/v1/jobs/:id.
// POST /api/v1/scan/upload   multipart form: file=<archive>, name=<label>
func (h *Handler) ScanUpload(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file field required in multipart form"})
		return
	}

	label := c.PostForm("name")
	if label == "" {
		label = fileHeader.Filename
	}

	if fileHeader.Size > maxDownloadSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file exceeds 512 MB limit"})
		return
	}

	// Save upload to temp file synchronously — the multipart body is only
	// available on this request, so it can't be deferred into the job.
	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "open upload: " + err.Error()})
		return
	}
	defer src.Close()

	tmp, err := os.CreateTemp("", "fg-upload-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create temp: " + err.Error()})
		return
	}
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "write upload: " + err.Error()})
		return
	}
	tmp.Close()
	tmpPath := tmp.Name()

	// Extract to temp dir
	dir, err := os.MkdirTemp("", "fg-upload-ext-*")
	if err != nil {
		os.Remove(tmpPath)
		internalError(c, h.log, "operation failed", err)
		return
	}

	if extErr := extractArchive(tmpPath, dir); extErr != nil {
		os.Remove(tmpPath)
		os.RemoveAll(dir)
		h.log.Warn("failed to extract uploaded archive", "error", extErr)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to extract archive — ensure the file is a valid tar.gz or zip"})
		return
	}
	os.Remove(tmpPath)

	job := h.jobs.submit(func() (any, error) {
		defer os.RemoveAll(dir)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		notifyCfg, _ := loadNotifyConfig()
		scanStart := time.Now()

		notify.NotifyScanStart(notifyCfg, notify.ScanEvent{
			Package:  label,
			ScanType: "upload",
			Target:   label,
		})

		agentBus.publish(AgentEvent{
			Type:    "start",
			Message: fmt.Sprintf("Scanning uploaded archive: %s", label),
		})

		ls := localscanner.New(h.cfg.GetIntelStorePath())
		result, err := ls.Scan(ctx, dir, nil)
		if err != nil {
			notify.NotifyScanError(notifyCfg, notify.ScanEvent{
				Package:  label,
				ScanType: "upload",
				Target:   label,
				Error:    err.Error(),
			})
			agentBus.publish(AgentEvent{
				Type:    "error",
				Message: fmt.Sprintf("Upload scan failed: %s", err.Error()),
			})
			return nil, err
		}
		result.RootDir = label

		var allFindings []core.Finding
		for _, r := range result.Results {
			allFindings = append(allFindings, r.Findings...)
		}
		summary := result.Summary
		notify.NotifyScanComplete(notifyCfg, notify.ScanEvent{
			Package:  label,
			ScanType: "upload",
			Target:   label,
			Duration: time.Since(scanStart),
			Findings: allFindings,
			Summary: &notify.ScanSummaryInfo{
				Total:      summary.Total,
				Critical:   summary.Critical,
				High:       summary.High,
				Medium:     summary.Medium,
				Low:        summary.Low,
				HighestSev: string(summary.HighestSev),
			},
		})

		agentBus.publish(AgentEvent{
			Type:    "done",
			Message: fmt.Sprintf("Upload scan complete: %s", label),
		})

		return result, nil
	})

	c.JSON(http.StatusAccepted, job)
}

// GetScanResults returns persisted scan results for a package version.
// GET /api/v1/scan/:ecosystem/:name/:version
func (h *Handler) GetScanResults(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "scan result persistence requires a database — results are available in the scan response itself"})
		return
	}
	eco := c.Param("ecosystem")
	name := c.Param("name")
	version := c.Param("version")

	sr, err := h.db.GetScanResult(c.Request.Context(), eco, name, version)
	if err != nil {
		if errNoRows(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "no scan results found — trigger a scan first"})
			return
		}
		internalError(c, h.log, "operation failed", err)
		return
	}

	var findings []core.Finding
	if err := json.Unmarshal(sr.FindingsJson, &findings); err != nil {
		h.log.Warn("corrupt findings JSON in stored scan result", "error", err, "package", name)
	}
	if findings == nil {
		findings = []core.Finding{}
	}

	c.JSON(http.StatusOK, gin.H{
		"package":          fmt.Sprintf("%s/%s@%s", eco, name, version),
		"status":           sr.Status,
		"sha256":           sr.Sha256,
		"highest_severity": sr.HighestSeverity,
		"summary": scanner.ScanSummary{
			Total:      int(sr.TotalFindings),
			Critical:   int(sr.CriticalFindings),
			High:       int(sr.HighFindings),
			Medium:     int(sr.MediumFindings),
			Low:        int(sr.LowFindings),
			HighestSev: core.Severity(sr.HighestSeverity),
		},
		"findings":   findings,
		"scanned_at": sr.ScannedAt,
	})
}

// ─── Advisory endpoint ────────────────────────────────────────────────────────

// GenerateAdvisory generates an AI advisory for a package.
// POST /api/v1/advisory  body: {"ecosystem":"npm","package":"lodash","version":"4.17.21","findings":[...]}
func (h *Handler) GenerateAdvisory(c *gin.Context) {
	var req struct {
		Ecosystem string         `json:"ecosystem" binding:"required"`
		Package   string         `json:"package"   binding:"required"`
		Version   string         `json:"version"   binding:"required"`
		Findings  []core.Finding `json:"findings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.cfg.GetAnthropicKey() == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ANTHROPIC_API_KEY not configured"})
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
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	eng := triage.New(h.cfg.GetAnthropicKey())
	advisory, err := eng.Triage(ctx, artifact, req.Findings)
	if err != nil {
		h.log.Error("advisory generation failed", "error", err)
		internalError(c, h.log, "operation failed", err)
		return
	}
	c.JSON(http.StatusOK, advisory)
}

// ─── SBOM endpoint ────────────────────────────────────────────────────────────

// GetSBOM generates and returns a SBOM for a package.
// GET /api/v1/sbom/:ecosystem/:name/:version?format=cyclonedx-json
func (h *Handler) GetSBOM(c *gin.Context) {
	eco := c.Param("ecosystem")
	name := c.Param("name")
	version := c.Param("version")
	format := sbom.Format(c.DefaultQuery("format", "cyclonedx-json"))

	artifact := core.BuiltArtifact{
		Source: core.SourceArtifact{
			Package: core.PackageVersion{
				Ecosystem: eco,
				Name:      name,
				Version:   version,
			},
		},
	}

	var buf bytes.Buffer
	if err := sbom.Generate(artifact, format, &buf); err != nil {
		internalError(c, h.log, "operation failed", err)
		return
	}

	contentType := "application/json"
	if format == sbom.FormatCycloneDXXML || format == sbom.FormatSPDXTV {
		contentType = "application/xml"
	}
	c.Data(http.StatusOK, contentType, buf.Bytes())
}

// ─── Signing endpoints ────────────────────────────────────────────────────────

// SignArtifact signs an artifact and persists + returns the attestation.
// POST /api/v1/sign  body: {"sha256":"hex","ecosystem":"npm","package":"lodash","version":"4.17.21"}
func (h *Handler) SignArtifact(c *gin.Context) {
	var req struct {
		SHA256    string `json:"sha256"    binding:"required"`
		Ecosystem string `json:"ecosystem" binding:"required"`
		Package   string `json:"package"   binding:"required"`
		Version   string `json:"version"   binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	artifact := core.BuiltArtifact{
		SHA256: req.SHA256,
		Source: core.SourceArtifact{
			Package: core.PackageVersion{
				Ecosystem: req.Ecosystem,
				Name:      req.Package,
				Version:   req.Version,
			},
		},
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()

	s := signer.New(h.cfg.GetRekorURL())
	prov := signer.NewProvenance(artifact)
	att, err := s.Sign(ctx, artifact, prov)
	if err != nil {
		internalError(c, h.log, "operation failed", err)
		return
	}

	// Persist attestation to DB if available
	if h.db != nil {
		if err := h.persistAttestation(ctx, req.Ecosystem, req.Package, req.Version, att); err != nil {
			h.log.Warn("failed to persist attestation", "error", err)
		}
	}

	c.JSON(http.StatusOK, att)
}

// GenerateProvenance builds and returns a SLSA v1.0 provenance statement for
// an artifact without signing it or persisting anything.
// POST /api/v1/provenance  body: {"sha256":"hex","ecosystem":"npm","package":"lodash","version":"4.17.21"}
func (h *Handler) GenerateProvenance(c *gin.Context) {
	var req struct {
		SHA256    string `json:"sha256"    binding:"required"`
		Ecosystem string `json:"ecosystem" binding:"required"`
		Package   string `json:"package"   binding:"required"`
		Version   string `json:"version"   binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	artifact := core.BuiltArtifact{
		SHA256: req.SHA256,
		Source: core.SourceArtifact{
			Package: core.PackageVersion{
				Ecosystem: req.Ecosystem,
				Name:      req.Package,
				Version:   req.Version,
			},
		},
	}

	prov := signer.NewProvenance(artifact)
	c.JSON(http.StatusOK, prov)
}

// VerifyAttestation verifies a previously created attestation.
// POST /api/v1/verify  body: {"attestation":{...},"sha256":"hex"}
func (h *Handler) VerifyAttestation(c *gin.Context) {
	var req struct {
		Attestation *signer.Attestation `json:"attestation" binding:"required"`
		SHA256      string              `json:"sha256"      binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	s := signer.New(h.cfg.GetRekorURL())
	result := s.Verify(ctx, req.Attestation, req.SHA256)
	status := http.StatusOK
	if !result.Valid {
		status = http.StatusUnprocessableEntity
	}
	c.JSON(status, result)
}

// ─── Dashboard endpoints ───────────────────────────────────────────────────────

// DashboardStats returns aggregate statistics for the dashboard.
// GET /api/v1/dashboard/stats
func (h *Handler) DashboardStats(c *gin.Context) {
	wsFilter := c.Query("workspace")

	if h.db != nil {
		stats, err := h.db.DashboardStats(c.Request.Context())
		if err != nil {
			h.log.Error("DashboardStats query failed", "error", err)
			internalError(c, h.log, "operation failed", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"total_packages":     stats.TotalPackages,
			"total_versions":     stats.TotalVersions,
			"total_findings":     stats.TotalFindings,
			"critical_findings":  stats.CriticalFindings,
			"high_findings":      stats.HighFindings,
			"medium_findings":    stats.MediumFindings,
			"low_findings":       stats.LowFindings,
			"scanned_today":      stats.ScannedToday,
			"ecosystems_covered": []string{"npm", "pypi", "go", "rubygems", "crates", "maven", "huggingface", "mcp"},
			"last_updated":       time.Now().UTC(),
		})
		return
	}

	cached := h.cache.latestForWorkspace(wsFilter)
	pkgs := map[string]bool{}
	versions := map[string]bool{}
	var total, crit, high, medium, low int64
	today := time.Now().Format("2006-01-02")
	var scannedToday int64
	for _, r := range cached {
		pkgs[r.Ecosystem+"/"+r.Name] = true
		versions[r.Ecosystem+"/"+r.Name+"@"+r.Version] = true
		total += int64(r.Summary.Total)
		crit += int64(r.Summary.Critical)
		high += int64(r.Summary.High)
		medium += int64(r.Summary.Medium)
		low += int64(r.Summary.Low)
		if r.ScannedAt.Format("2006-01-02") == today {
			scannedToday++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"total_packages":     len(pkgs),
		"total_versions":     len(versions),
		"total_findings":     total,
		"critical_findings":  crit,
		"high_findings":      high,
		"medium_findings":    medium,
		"low_findings":       low,
		"scanned_today":      scannedToday,
		"ecosystems_covered": []string{"npm", "pypi", "go", "rubygems", "crates", "maven", "huggingface", "mcp"},
		"last_updated":       time.Now().UTC(),
	})
}

// DashboardRecent returns recent scan activity.
// GET /api/v1/dashboard/recent?limit=20
func (h *Handler) DashboardRecent(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	if h.db != nil {
		results, err := h.db.ListRecentScans(c.Request.Context(), int32(limit))
		if err != nil {
			h.log.Error("ListRecentScans query failed", "error", err)
			internalError(c, h.log, "operation failed", err)
			return
		}
		if results == nil {
			results = []dbsqlc.RecentScanRow{}
		}
		c.JSON(http.StatusOK, gin.H{"limit": limit, "results": results})
		return
	}

	wsFilter := c.Query("workspace")
	allCached := h.cache.list()
	var cached []cachedScanResult
	if wsFilter != "" {
		for _, r := range allCached {
			if strings.EqualFold(r.Workspace, wsFilter) || (r.Workspace == "" && strings.EqualFold(wsFilter, "Default")) {
				cached = append(cached, r)
			}
		}
	} else {
		cached = allCached
	}
	type recentRow struct {
		Ecosystem       string `json:"ecosystem"`
		Name            string `json:"name"`
		Version         string `json:"version"`
		TotalFindings   int    `json:"total_findings"`
		CriticalFindings int   `json:"critical_findings"`
		HighFindings    int    `json:"high_findings"`
		HighestSeverity string `json:"highest_severity"`
		ScannedAt       string `json:"scanned_at"`
	}
	rows := make([]recentRow, 0, limit)
	start := len(cached) - limit
	if start < 0 {
		start = 0
	}
	for i := len(cached) - 1; i >= start; i-- {
		r := cached[i]
		rows = append(rows, recentRow{
			Ecosystem:        r.Ecosystem,
			Name:             r.Name,
			Version:          r.Version,
			TotalFindings:    r.Summary.Total,
			CriticalFindings: r.Summary.Critical,
			HighFindings:     r.Summary.High,
			HighestSeverity:  r.HighestSeverity,
			ScannedAt:        r.ScannedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"limit": limit, "results": rows})
}

// DashboardGraph returns a dependency-graph view of recently scanned
// packages — a root node plus one node per package, sized/colored by its
// highest finding severity. Without a real dependency tree (recent scans
// aren't linked to each other), this is a flat star topology rather than a
// true transitive graph — good enough for an at-a-glance risk map.
// GET /api/v1/dashboard/graph?limit=20
func (h *Handler) DashboardGraph(c *gin.Context) {
	type graphNode struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Version  string `json:"version"`
		Severity string `json:"severity"`
	}
	type graphLink struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	nodes := []graphNode{{ID: "root", Name: "your-app", Version: "", Severity: "none"}}
	links := []graphLink{}

	if h.db != nil {
		rows, err := h.db.ListRecentScans(c.Request.Context(), int32(limit))
		if err != nil {
			h.log.Error("ListRecentScans query failed", "error", err)
			internalError(c, h.log, "operation failed", err)
			return
		}
		for _, r := range rows {
			id := fmt.Sprintf("%s/%s@%s", r.Ecosystem, r.Name, r.Version)
			nodes = append(nodes, graphNode{
				ID:       id,
				Name:     r.Name,
				Version:  r.Version,
				Severity: strings.ToLower(r.HighestSeverity),
			})
			links = append(links, graphLink{Source: "root", Target: id})
		}
	} else {
		wsFilter := c.Query("workspace")
		cached := h.cache.latestForWorkspace(wsFilter)
		seen := map[string]bool{}
		start := len(cached) - limit
		if start < 0 {
			start = 0
		}
		for i := len(cached) - 1; i >= start; i-- {
			r := cached[i]
			id := fmt.Sprintf("%s/%s@%s", r.Ecosystem, r.Name, r.Version)
			if seen[id] {
				continue
			}
			seen[id] = true
			nodes = append(nodes, graphNode{
				ID:       id,
				Name:     r.Name,
				Version:  r.Version,
				Severity: strings.ToLower(r.HighestSeverity),
			})
			links = append(links, graphLink{Source: "root", Target: id})
		}
	}

	c.JSON(http.StatusOK, gin.H{"nodes": nodes, "links": links})
}

// DashboardTimeline returns daily finding counts for the past N days.
// GET /api/v1/dashboard/timeline?days=30
func (h *Handler) DashboardTimeline(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 || days > 90 {
		days = 30
	}

	type timelinePoint struct {
		Date     string `json:"date"`
		Critical int64  `json:"critical"`
		High     int64  `json:"high"`
		Medium   int64  `json:"medium"`
		Low      int64  `json:"low"`
		Total    int64  `json:"total"`
	}

	points := make([]timelinePoint, 0, days)

	if h.db != nil {
		rows, err := h.db.DashboardTimeline(c.Request.Context(), int32(days))
		if err != nil {
			h.log.Error("DashboardTimeline query failed", "error", err)
		} else {
			for _, row := range rows {
				points = append(points, timelinePoint{
					Date:     row.Day.Time.Format("2006-01-02"),
					Critical: row.Critical,
					High:     row.High,
					Medium:   row.Medium,
					Low:      row.Low,
					Total:    row.Critical + row.High + row.Medium + row.Low,
				})
			}
		}
	} else {
		cached := h.cache.latest()
		byDay := map[string]*timelinePoint{}
		cutoff := time.Now().AddDate(0, 0, -days)
		for _, r := range cached {
			if r.ScannedAt.Before(cutoff) {
				continue
			}
			day := r.ScannedAt.Format("2006-01-02")
			p, ok := byDay[day]
			if !ok {
				p = &timelinePoint{Date: day}
				byDay[day] = p
			}
			p.Critical += int64(r.Summary.Critical)
			p.High += int64(r.Summary.High)
			p.Medium += int64(r.Summary.Medium)
			p.Low += int64(r.Summary.Low)
			p.Total += int64(r.Summary.Total)
		}
		for _, p := range byDay {
			points = append(points, *p)
		}
	}

	c.JSON(http.StatusOK, gin.H{"days": days, "points": points})
}

// ─── Intelligence endpoints ───────────────────────────────────────────────────

// ListSignatures returns all detection signatures from the intelligence store.
// GET /api/v1/intelligence/signatures?type=blocklisted&ecosystem=npm
func (h *Handler) ListSignatures(c *gin.Context) {
	sigType := c.Query("type")
	eco := c.Query("ecosystem")

	store, err := intelligence.LoadStoreWithLocalFallback(h.cfg.GetIntelStorePath())
	if err != nil {
		internalError(c, h.log, "operation failed", err)
		return
	}

	var filtered []intelligence.DetectionSignature
	for _, s := range store.Signatures {
		if sigType != "" && string(s.Type) != sigType {
			continue
		}
		if eco != "" && s.Ecosystem != eco && s.Ecosystem != "*" {
			continue
		}
		filtered = append(filtered, s)
	}
	if filtered == nil {
		filtered = []intelligence.DetectionSignature{}
	}

	// A store that's never been saved has a zero-value UpdatedAt — sending
	// that raw marshals to "0001-01-01T00:00:00Z", which the dashboard
	// would otherwise render as a bogus "1/1/1" instead of hiding the field.
	var updatedAt *time.Time
	if !store.UpdatedAt.IsZero() {
		updatedAt = &store.UpdatedAt
	}

	c.JSON(http.StatusOK, gin.H{
		"total":      len(filtered),
		"updated_at": updatedAt,
		"signatures": filtered,
	})
}

// RefreshIntelligence downloads the latest community signature bundle and
// merges it into the local store — the same real fetch-and-merge path as
// `fgctl intel update`, not a fabricated "queued" response. Synchronous: a
// GitHub raw-content fetch is fast enough not to need a job/poll dance.
// POST /api/v1/intelligence/refresh
func (h *Handler) RefreshIntelligence(c *gin.Context) {
	result, err := intelligence.UpdateFromCommunity(h.cfg.GetIntelStorePath(), intelligence.DefaultCommunitySignaturesURL)
	if err != nil {
		h.log.Warn("intelligence refresh failed", "error", err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "could not reach community signature feed: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "signatures updated",
		"before":  result.Before,
		"added":   result.Added,
		"total":   result.Total,
	})
}

// ─── Activity, Risks, Policy endpoints ───────────────────────────────────────

// DashboardActivity returns recent scan events for the live activity feed.
// GET /api/v1/dashboard/activity?limit=20
func (h *Handler) DashboardActivity(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	type activityEvent struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		Message     string `json:"message"`
		Severity    string `json:"severity"`
		PackageName string `json:"package_name,omitempty"`
		Ecosystem   string `json:"ecosystem,omitempty"`
		OccurredAt  string `json:"occurred_at"`
	}

	var events []activityEvent

	if h.db != nil {
		results, err := h.db.ListRecentScans(c.Request.Context(), int32(limit))
		if err == nil {
			for i, r := range results {
				evType := "scan_complete"
				msg := fmt.Sprintf("%s@%s scanned — %d finding(s)", r.Name, r.Version, r.TotalFindings)
				sev := "INFORMATIONAL"
				if r.HighestSeverity != "" {
					sev = r.HighestSeverity
					if r.CriticalFindings > 0 {
						evType = "finding_critical"
						msg = fmt.Sprintf("[%s] %s@%s — %d critical finding(s)", r.HighestSeverity, r.Name, r.Version, r.CriticalFindings)
					} else if r.HighFindings > 0 {
						evType = "finding_critical"
						msg = fmt.Sprintf("[%s] %s@%s — %d high finding(s)", r.HighestSeverity, r.Name, r.Version, r.HighFindings)
					}
				}
				events = append(events, activityEvent{
					ID:          fmt.Sprintf("evt-%d", i),
					Type:        evType,
					Message:     msg,
					Severity:    sev,
					PackageName: r.Name,
					Ecosystem:   r.Ecosystem,
					OccurredAt:  r.ScannedAt.Time.Format(time.RFC3339),
				})
			}
		}
	} else {
		cached := h.cache.list()
		start := len(cached) - limit
		if start < 0 {
			start = 0
		}
		for i := len(cached) - 1; i >= start; i-- {
			r := cached[i]
			evType := "scan_complete"
			msg := fmt.Sprintf("%s@%s scanned — %d finding(s)", r.Name, r.Version, r.Summary.Total)
			sev := "INFORMATIONAL"
			if r.HighestSeverity != "" {
				sev = r.HighestSeverity
				if r.Summary.Critical > 0 {
					evType = "finding_critical"
					msg = fmt.Sprintf("[%s] %s@%s — %d critical finding(s)", r.HighestSeverity, r.Name, r.Version, r.Summary.Critical)
				} else if r.Summary.High > 0 {
					evType = "finding_critical"
					msg = fmt.Sprintf("[%s] %s@%s — %d high finding(s)", r.HighestSeverity, r.Name, r.Version, r.Summary.High)
				}
			}
			events = append(events, activityEvent{
				ID:          fmt.Sprintf("evt-%d", len(cached)-1-i),
				Type:        evType,
				Message:     msg,
				Severity:    sev,
				PackageName: r.Name,
				Ecosystem:   r.Ecosystem,
				OccurredAt:  r.ScannedAt.Format(time.RFC3339),
			})
		}
	}

	if events == nil {
		events = []activityEvent{}
	}
	c.JSON(http.StatusOK, gin.H{"events": events})
}

// ServerLogs returns recent structured log entries from the ring buffer.
// GET /api/v1/logs?limit=100&level=ERROR
func (h *Handler) ServerLogs(c *gin.Context) {
	limit := 200
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 500 {
		limit = l
	}
	levelFilter := strings.ToUpper(c.Query("level"))

	entries := h.logs.list(limit)

	if levelFilter != "" {
		filtered := make([]logEntry, 0, len(entries))
		for _, e := range entries {
			if e.Level == levelFilter {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	c.JSON(http.StatusOK, gin.H{"logs": entries, "total": len(entries)})
}

// CLISync accepts scan results pushed from the fgctl CLI so they appear
// on the dashboard. Supports both single-package results (registry scan)
// and project results (local/remote scan).
// POST /api/v1/cli/sync
func (h *Handler) CLISync(c *gin.Context) {
	var req struct {
		ScanType  string              `json:"scan_type"`
		Label     string              `json:"label"`
		Ecosystem string              `json:"ecosystem,omitempty"`
		Package   string              `json:"package,omitempty"`
		Version   string              `json:"version,omitempty"`
		RootDir   string              `json:"root_dir,omitempty"`
		Workspace string              `json:"workspace,omitempty"`
		Summary   scanner.ScanSummary `json:"summary"`
		Findings  []core.Finding      `json:"findings"`
		Results   []struct {
			Entry struct {
				Ecosystem string `json:"ecosystem"`
				Name      string `json:"name"`
				Version   string `json:"version"`
				FilePath  string `json:"file_path"`
			} `json:"entry"`
			Findings []core.Finding `json:"findings"`
			Skipped  bool           `json:"skipped"`
		} `json:"results,omitempty"`
		Manifests []struct {
			Path      string `json:"path"`
			Ecosystem string `json:"ecosystem"`
		} `json:"manifests,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ws := req.Workspace
	if ws == "" {
		ws = "Default"
	}

	h.log.Info("CLI sync received",
		"scan_type", req.ScanType,
		"label", req.Label,
		"workspace", ws,
		"findings", len(req.Findings),
		"total", req.Summary.Total)

	// For single-package scans, add to the scan cache so dashboard stats pick it up.
	if req.ScanType == "registry" || req.ScanType == "single" {
		eco := req.Ecosystem
		name := req.Package
		ver := req.Version
		if eco != "" && name != "" {
			h.cache.add(cachedScanResult{
				Ecosystem:       eco,
				Name:            name,
				Version:         ver,
				Summary:         req.Summary,
				HighestSeverity: string(req.Summary.HighestSev),
				ScannedAt:       time.Now(),
				TotalFindings:   req.Summary.Total,
				Workspace:       ws,
			})
			if h.db != nil {
				if err := h.persistScanResult(c.Request.Context(), eco, name, ver, "", req.Findings, req.Summary, ""); err != nil {
					h.log.Warn("CLI sync DB persist failed", "error", err)
				}
			}
		}
	}

	// For project scans (local, upload, remote), persist each dependency
	// separately so they show up in dashboard stats and risks.
	if req.ScanType == "project" || req.ScanType == "local" || req.ScanType == "remote" {
		for _, r := range req.Results {
			if r.Skipped || len(r.Findings) == 0 {
				continue
			}
			eco := r.Entry.Ecosystem
			name := r.Entry.Name
			ver := r.Entry.Version
			sum := scanner.Summarize(r.Findings)
			h.cache.add(cachedScanResult{
				Ecosystem:       eco,
				Name:            name,
				Version:         ver,
				Summary:         sum,
				HighestSeverity: string(sum.HighestSev),
				ScannedAt:       time.Now(),
				TotalFindings:   sum.Total,
				Workspace:       ws,
			})
			if h.db != nil {
				if err := h.persistScanResult(c.Request.Context(), eco, name, ver, "", r.Findings, sum, ""); err != nil {
					h.log.Warn("CLI sync DB persist failed", "error", err, "package", name)
				}
			}
		}
	}

	// Auto-create alerts for Critical and High findings so they surface on the Alerts page.
	if h.db != nil {
		alertCount := 0
		allFindings := req.Findings
		for _, r := range req.Results {
			allFindings = append(allFindings, r.Findings...)
		}
		for _, f := range allFindings {
			if f.Severity != core.SeverityCritical && f.Severity != core.SeverityHigh {
				continue
			}
			pkg := req.Package
			eco := req.Ecosystem
			ver := req.Version
			if pkg == "" {
				pkg = f.Source
			}
			alertType := "finding_" + strings.ToLower(string(f.Severity))
			msg := fmt.Sprintf("[CLI] %s: %s", f.ID, f.Title)
			_, err := h.db.InsertAlert(c.Request.Context(), alertType, msg, string(f.Severity), strPtr(pkg), strPtr(eco), strPtr(ver), nil)
			if err != nil {
				h.log.Warn("CLI sync alert creation failed", "error", err, "finding", f.ID)
				continue
			}
			agentBus.publish(AgentEvent{
				Type:    "step",
				Message: msg,
				Package: pkg,
				Version: ver,
			})
			alertCount++
		}
		if alertCount > 0 {
			h.log.Info("CLI sync created alerts", "count", alertCount)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "synced",
		"label":     req.Label,
		"workspace": ws,
		"total":     req.Summary.Total,
		"message":   fmt.Sprintf("Synced %d findings from CLI scan to workspace %q", req.Summary.Total, ws),
	})
}

// ListWorkspaces returns the list of workspace names seen in scan results.
// GET /api/v1/workspaces
func (h *Handler) ListWorkspaces(c *gin.Context) {
	ws := h.cache.workspaces()
	if len(ws) == 0 {
		ws = []string{"Default"}
	}
	c.JSON(http.StatusOK, gin.H{"workspaces": ws})
}

// ActiveRisks returns a prioritized list of packages with findings.
// GET /api/v1/risks
func (h *Handler) ActiveRisks(c *gin.Context) {
	type riskItem struct {
		PackageName  string `json:"package_name"`
		Version      string `json:"version"`
		Ecosystem    string `json:"ecosystem"`
		RiskGrade    string `json:"risk_grade"`
		RiskScore    int    `json:"risk_score"`
		FindingCount int    `json:"finding_count"`
		TopSeverity  string `json:"top_severity"`
		FirstSeen    string `json:"first_seen"`
	}

	var risks []riskItem

	if h.db != nil {
		results, err := h.db.ListRecentScans(c.Request.Context(), int32(50))
		if err == nil {
			for _, r := range results {
				if r.TotalFindings == 0 {
					continue
				}
				score := riskScore(int(r.CriticalFindings), int(r.HighFindings), int(r.MediumFindings), int(r.LowFindings))
				grade := riskGrade(score)
				risks = append(risks, riskItem{
					PackageName:  r.Name,
					Version:      r.Version,
					Ecosystem:    r.Ecosystem,
					RiskGrade:    grade,
					RiskScore:    score,
					FindingCount: int(r.TotalFindings),
					TopSeverity:  r.HighestSeverity,
					FirstSeen:    r.ScannedAt.Time.Format(time.RFC3339),
				})
			}
		}
	} else {
		cached := h.cache.latest()
		seen := map[string]bool{}
		for i := len(cached) - 1; i >= 0; i-- {
			r := cached[i]
			if r.Summary.Total == 0 {
				continue
			}
			key := r.Ecosystem + "/" + r.Name + "@" + r.Version
			if seen[key] {
				continue
			}
			seen[key] = true
			score := riskScore(r.Summary.Critical, r.Summary.High, r.Summary.Medium, r.Summary.Low)
			grade := riskGrade(score)
			risks = append(risks, riskItem{
				PackageName:  r.Name,
				Version:      r.Version,
				Ecosystem:    r.Ecosystem,
				RiskGrade:    grade,
				RiskScore:    score,
				FindingCount: r.Summary.Total,
				TopSeverity:  r.HighestSeverity,
				FirstSeen:    r.ScannedAt.Format(time.RFC3339),
			})
		}
	}

	if risks == nil {
		risks = []riskItem{}
	}
	c.JSON(http.StatusOK, gin.H{"risks": risks})
}

// PolicyStatus returns the current policy enforcement status.
// GET /api/v1/policy/status
func (h *Handler) PolicyStatus(c *gin.Context) {
	pol, err := policy.Load()
	if err != nil {
		internalError(c, h.log, "operation failed", err)
		return
	}

	configured := pol != nil
	if pol == nil {
		pol = policy.DefaultPolicy()
	}

	// Evaluate policy against recent scan findings from cache.
	var violations []gin.H
	cached := h.cache.latest()
	for _, entry := range cached {
		if entry.Summary.Total == 0 {
			continue
		}
		if pol.ShouldFail(nil) {
			continue
		}
		// Check deny_packages
		for _, denied := range pol.DenyPackages {
			if strings.EqualFold(entry.Name, denied) {
				violations = append(violations, gin.H{
					"type":    "denied_package",
					"package": entry.Name,
					"version": entry.Version,
					"message": fmt.Sprintf("Package %q is denied by policy", entry.Name),
				})
			}
		}
		// Check severity threshold
		if pol.FailOn != "" {
			threshold := strings.ToUpper(pol.FailOn)
			breached := false
			switch threshold {
			case "CRITICAL":
				breached = entry.Summary.Critical > 0
			case "HIGH":
				breached = entry.Summary.Critical > 0 || entry.Summary.High > 0
			case "MEDIUM":
				breached = entry.Summary.Critical > 0 || entry.Summary.High > 0 || entry.Summary.Medium > 0
			case "LOW":
				breached = entry.Summary.Total > 0
			}
			if breached {
				violations = append(violations, gin.H{
					"type":    "severity_threshold",
					"package": entry.Name,
					"version": entry.Version,
					"message": fmt.Sprintf("%s@%s has findings at or above %s threshold", entry.Name, entry.Version, threshold),
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"configured":     configured,
		"violations":     violations,
		"last_evaluated": time.Now().UTC().Format(time.RFC3339),
		"policy": gin.H{
			"version":              pol.Version,
			"fail_on":              pol.FailOn,
			"deny_packages":        pol.DenyPackages,
			"allow_licenses":       pol.AllowLicenses,
			"max_package_age_days": pol.MaxPackageAgeDays,
			"block_typosquatting":  pol.BlockTyposquatting,
			"block_abandoned":      pol.BlockAbandoned,
			"require_signing":      pol.RequireSigning,
		},
	})
}

// SavePolicy persists a new policy configuration.
// PUT /api/v1/policy  body: full policy.Policy JSON (all fields optional)
func (h *Handler) SavePolicy(c *gin.Context) {
	var p policy.Policy
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch strings.ToLower(p.FailOn) {
	case "", "critical", "high", "medium", "low":
		// valid
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid fail_on %q — must be one of: critical, high, medium, low", p.FailOn)})
		return
	}

	if err := policy.Save(&p); err != nil {
		internalError(c, h.log, "operation failed", err)
		return
	}

	c.JSON(http.StatusOK, p)
}

// QuarantinePackage adds a package to the policy deny list.
// POST /api/v1/policy/quarantine  body: {"package": "minimist", "reason": "CVE-2020-7598"}
func (h *Handler) QuarantinePackage(c *gin.Context) {
	var req struct {
		Package string `json:"package" binding:"required"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "package name is required"})
		return
	}

	pol, err := policy.Load()
	if err != nil || pol == nil {
		if pol == nil {
			pol = policy.DefaultPolicy()
		}
	}

	for _, d := range pol.DenyPackages {
		if strings.EqualFold(d, req.Package) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "already_denied",
				"package": req.Package,
				"message": fmt.Sprintf("Package %q is already in the deny list", req.Package),
			})
			return
		}
	}

	pol.DenyPackages = append(pol.DenyPackages, req.Package)
	if err := policy.Save(pol); err != nil {
		internalError(c, h.log, "operation failed", err)
		return
	}

	h.addMonitorEvent("quarantine", req.Package, req.Reason)

	c.JSON(http.StatusOK, gin.H{
		"status":  "quarantined",
		"package": req.Package,
		"action":  "added to deny list",
		"deny_packages": pol.DenyPackages,
	})
}

// BlockPackage adds a package to the deny list and sets fail_on to at least medium.
// POST /api/v1/policy/block  body: {"package": "minimist", "reason": "CVE-2020-7598"}
func (h *Handler) BlockPackage(c *gin.Context) {
	var req struct {
		Package string `json:"package" binding:"required"`
		Reason  string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "package name is required"})
		return
	}

	pol, err := policy.Load()
	if err != nil || pol == nil {
		if pol == nil {
			pol = policy.DefaultPolicy()
		}
	}

	alreadyDenied := false
	for _, d := range pol.DenyPackages {
		if strings.EqualFold(d, req.Package) {
			alreadyDenied = true
			break
		}
	}
	if !alreadyDenied {
		pol.DenyPackages = append(pol.DenyPackages, req.Package)
	}

	sevRank := map[string]int{"": 0, "critical": 4, "high": 3, "medium": 2, "low": 1}
	if sevRank[strings.ToLower(pol.FailOn)] > 2 || pol.FailOn == "" {
		pol.FailOn = "medium"
	}

	if err := policy.Save(pol); err != nil {
		internalError(c, h.log, "operation failed", err)
		return
	}

	h.addMonitorEvent("block", req.Package, req.Reason)

	c.JSON(http.StatusOK, gin.H{
		"status":         "blocked",
		"package":        req.Package,
		"action":         "denied + fail_on=medium",
		"deny_packages":  pol.DenyPackages,
		"fail_on":        pol.FailOn,
	})
}

// UnquarantinePackage removes a package from the policy deny list.
// POST /api/v1/policy/unquarantine  body: {"package": "minimist"}
func (h *Handler) UnquarantinePackage(c *gin.Context) {
	var req struct {
		Package string `json:"package" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "package name is required"})
		return
	}

	pol, err := policy.Load()
	if err != nil || pol == nil {
		c.JSON(http.StatusOK, gin.H{"status": "not_found", "message": "no policy file or package not in deny list"})
		return
	}

	var kept []string
	found := false
	for _, d := range pol.DenyPackages {
		if strings.EqualFold(d, req.Package) {
			found = true
			continue
		}
		kept = append(kept, d)
	}
	if !found {
		c.JSON(http.StatusOK, gin.H{"status": "not_found", "package": req.Package, "message": "package is not in the deny list"})
		return
	}

	pol.DenyPackages = kept
	if err := policy.Save(pol); err != nil {
		internalError(c, h.log, "operation failed", err)
		return
	}

	h.addMonitorEvent("unquarantine", req.Package, "manually removed from deny list")

	c.JSON(http.StatusOK, gin.H{
		"status":         "removed",
		"package":        req.Package,
		"deny_packages":  pol.DenyPackages,
	})
}

// monitorEvent is an in-memory log entry for policy actions.
type monitorEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Package   string    `json:"package"`
	Reason    string    `json:"reason,omitempty"`
}

func (h *Handler) addMonitorEvent(action, pkg, reason string) {
	h.monitorMu.Lock()
	defer h.monitorMu.Unlock()
	h.monitorEvents = append(h.monitorEvents, monitorEvent{
		Timestamp: time.Now().UTC(),
		Action:    action,
		Package:   pkg,
		Reason:    reason,
	})
	if len(h.monitorEvents) > 500 {
		h.monitorEvents = h.monitorEvents[len(h.monitorEvents)-500:]
	}
}

// MonitorEvents returns the in-memory log of policy actions.
// GET /api/v1/monitor/events
func (h *Handler) MonitorEvents(c *gin.Context) {
	h.monitorMu.Lock()
	events := make([]monitorEvent, len(h.monitorEvents))
	copy(events, h.monitorEvents)
	h.monitorMu.Unlock()

	// Return newest first
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  len(events),
	})
}

// WebhookTest reads ~/.forgeguardian/config.yaml (the same file
// `fgctl config set notify.*` writes) and attempts a real delivery to
// whatever webhook(s) are configured there. Previously this handler
// unconditionally returned success with no configured webhook and no
// delivery attempt — that read as "your webhook works" even with nothing
// set up. Now it reports what actually happened.
// POST /api/v1/webhooks/test
func (h *Handler) WebhookTest(c *gin.Context) {
	notifyCfg, err := loadNotifyConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read ~/.forgeguardian/config.yaml: " + err.Error()})
		return
	}

	if notifyCfg.SlackWebhookURL == "" && notifyCfg.DiscordWebhookURL == "" && notifyCfg.GenericWebhookURL == "" {
		c.JSON(http.StatusOK, gin.H{
			"status":  "not_configured",
			"message": "no webhook configured — set one with: fgctl config set notify.slack_webhook_url=<url>",
		})
		return
	}

	const testMessage = "ForgeGuardian webhook test — if you see this, delivery is working."
	var attempted, failed []string
	if notifyCfg.SlackWebhookURL != "" {
		attempted = append(attempted, "slack")
		if err := notify.SendSlack(notifyCfg.SlackWebhookURL, testMessage); err != nil {
			failed = append(failed, "slack: "+err.Error())
		}
	}
	if notifyCfg.DiscordWebhookURL != "" {
		attempted = append(attempted, "discord")
		if err := notify.SendDiscord(notifyCfg.DiscordWebhookURL, testMessage); err != nil {
			failed = append(failed, "discord: "+err.Error())
		}
	}
	if notifyCfg.GenericWebhookURL != "" {
		attempted = append(attempted, "generic")
		if err := notify.SendGeneric(notifyCfg.GenericWebhookURL, map[string]any{"message": testMessage}); err != nil {
			failed = append(failed, "generic: "+err.Error())
		}
	}

	if len(failed) > 0 {
		c.JSON(http.StatusOK, gin.H{
			"status":    "failed",
			"attempted": attempted,
			"errors":    failed,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"attempted": attempted,
		"message":   "test message delivered to " + strings.Join(attempted, ", "),
	})
}

// loadNotifyConfig reads the notify section of ~/.forgeguardian/config.yaml
// directly (fgctl's FGConfig type lives in package main and can't be
// imported here) — same file, same shape, read-only.
func loadNotifyConfig() (notify.Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return notify.Config{}, err
	}
	path := filepath.Join(home, ".forgeguardian", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return notify.Config{}, nil // no config file yet — nothing configured
		}
		return notify.Config{}, err
	}
	var wrapper struct {
		Notify notify.Config `yaml:"notify"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return notify.Config{}, fmt.Errorf("parse config.yaml: %w", err)
	}
	return wrapper.Notify, nil
}

func riskScore(critical, high, medium, low int) int {
	s := critical*40 + high*20 + medium*5 + low*1
	if s > 100 {
		return 100
	}
	return s
}

func riskGrade(score int) string {
	switch {
	case score <= 20:
		return "A"
	case score <= 40:
		return "B"
	case score <= 60:
		return "C"
	case score <= 80:
		return "D"
	default:
		return "F"
	}
}

// csvSafe prefixes cell values that could trigger formula injection in
// spreadsheet applications (=, +, -, @, tab, carriage return).
var validEcosystems = map[string]bool{
	"npm": true, "pypi": true, "go": true, "rubygems": true,
	"crates": true, "maven": true, "huggingface": true, "mcp": true,
}

var safeNameRe = regexp.MustCompile(`^[a-zA-Z0-9@/_:.\-]+$`)

func isValidPackageInput(ecosystem, name, version string) bool {
	if !validEcosystems[ecosystem] {
		return false
	}
	if len(name) > 256 || len(version) > 128 {
		return false
	}
	if !safeNameRe.MatchString(name) || !safeNameRe.MatchString(version) {
		return false
	}
	if strings.Contains(name, "..") || strings.Contains(version, "..") {
		return false
	}
	return true
}

func internalError(c *gin.Context, log *slog.Logger, msg string, err error) {
	log.Error(msg, "error", err, "path", c.Request.URL.Path)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func csvSafe(s string) string {
	if len(s) == 0 {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

// ─── persistence helpers ──────────────────────────────────────────────────────

func (h *Handler) persistScanResult(
	ctx context.Context,
	ecosystem, name, version, sha256 string,
	findings []core.Finding,
	summary scanner.ScanSummary,
	errMsg string,
) error {
	pkg, err := h.db.UpsertPackage(ctx, ecosystem, name)
	if err != nil {
		return fmt.Errorf("upsert package: %w", err)
	}
	pv, err := h.db.UpsertPackageVersion(ctx, dbsqlc.UpsertPackageVersionParams{
		PackageID: pkg.ID,
		Version:   version,
	})
	if err != nil {
		return fmt.Errorf("upsert version: %w", err)
	}

	findingsJSON, _ := json.Marshal(findings)
	status := "complete"
	if errMsg != "" {
		status = "error"
	}

	_, err = h.db.UpsertScanResult(ctx, dbsqlc.UpsertScanResultParams{
		PackageVersionID: pv.ID,
		Sha256:           sha256,
		Status:           status,
		TotalFindings:    int32(summary.Total),
		CriticalFindings: int32(summary.Critical),
		HighFindings:     int32(summary.High),
		MediumFindings:   int32(summary.Medium),
		LowFindings:      int32(summary.Low),
		HighestSeverity:  string(summary.HighestSev),
		FindingsJson:     findingsJSON,
		ErrorMessage:     errMsg,
	})
	return err
}

func (h *Handler) persistAttestation(
	ctx context.Context,
	ecosystem, name, version string,
	att *signer.Attestation,
) error {
	pkg, err := h.db.UpsertPackage(ctx, ecosystem, name)
	if err != nil {
		return fmt.Errorf("upsert package: %w", err)
	}
	pv, err := h.db.UpsertPackageVersion(ctx, dbsqlc.UpsertPackageVersionParams{
		PackageID: pkg.ID,
		Version:   version,
	})
	if err != nil {
		return fmt.Errorf("upsert version: %w", err)
	}

	attJSON, _ := json.Marshal(att)

	_, err = h.db.InsertAttestation(ctx, dbsqlc.InsertAttestationParams{
		PackageVersionID: pv.ID,
		Sha256:           att.SHA256,
		Signature:        att.Signature,
		PublicKey:        att.PublicKey,
		RekorLogID:       att.RekorLogID,
		RekorIndex:       att.RekorIndex,
		RekorURL:         att.RekorURL,
		AttestationJson:  attJSON,
	})
	return err
}

// ExportReport exports scan results as JSON or CSV.
// GET /api/v1/export/report?format=json|csv
func (h *Handler) ExportReport(c *gin.Context) {
	format := c.DefaultQuery("format", "json")
	now := time.Now().Format("2006-01-02")

	type exportRow struct {
		Package     string `json:"package"`
		Version     string `json:"version"`
		Ecosystem   string `json:"ecosystem"`
		RiskScore   int    `json:"risk_score"`
		RiskGrade   string `json:"risk_grade"`
		Critical    int    `json:"critical"`
		High        int    `json:"high"`
		Medium      int    `json:"medium"`
		Low         int    `json:"low"`
		Total       int    `json:"total"`
		TopSeverity string `json:"top_severity"`
		ScannedAt   string `json:"scanned_at"`
	}

	var rows []exportRow

	if h.db != nil {
		results, err := h.db.ListRecentScans(c.Request.Context(), 500)
		if err == nil {
			for _, r := range results {
				score := riskScore(int(r.CriticalFindings), int(r.HighFindings), int(r.MediumFindings), int(r.LowFindings))
				rows = append(rows, exportRow{
					Package:     r.Name,
					Version:     r.Version,
					Ecosystem:   r.Ecosystem,
					RiskScore:   score,
					RiskGrade:   riskGrade(score),
					Critical:    int(r.CriticalFindings),
					High:        int(r.HighFindings),
					Medium:      int(r.MediumFindings),
					Low:         int(r.LowFindings),
					Total:       int(r.TotalFindings),
					TopSeverity: r.HighestSeverity,
					ScannedAt:   r.ScannedAt.Time.Format(time.RFC3339),
				})
			}
		}
	} else {
		for _, r := range h.cache.latest() {
			score := riskScore(r.Summary.Critical, r.Summary.High, r.Summary.Medium, r.Summary.Low)
			rows = append(rows, exportRow{
				Package:     r.Name,
				Version:     r.Version,
				Ecosystem:   r.Ecosystem,
				RiskScore:   score,
				RiskGrade:   riskGrade(score),
				Critical:    r.Summary.Critical,
				High:        r.Summary.High,
				Medium:      r.Summary.Medium,
				Low:         r.Summary.Low,
				Total:       r.Summary.Total,
				TopSeverity: r.HighestSeverity,
				ScannedAt:   r.ScannedAt.Format(time.RFC3339),
			})
		}
	}

	if rows == nil {
		rows = []exportRow{}
	}

	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=forgeguardian-report-%s.csv", now))
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		w.Write([]string{"package", "version", "ecosystem", "risk_score", "risk_grade", "critical", "high", "medium", "low", "total", "top_severity", "scanned_at"})
		for _, r := range rows {
			w.Write([]string{
				csvSafe(r.Package), csvSafe(r.Version), csvSafe(r.Ecosystem),
				fmt.Sprint(r.RiskScore), csvSafe(r.RiskGrade),
				fmt.Sprint(r.Critical), fmt.Sprint(r.High), fmt.Sprint(r.Medium), fmt.Sprint(r.Low), fmt.Sprint(r.Total),
				csvSafe(r.TopSeverity), r.ScannedAt,
			})
		}
		w.Flush()
		c.Data(http.StatusOK, "text/csv", buf.Bytes())
	default:
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=forgeguardian-report-%s.json", now))
		c.JSON(http.StatusOK, gin.H{
			"report_date":    now,
			"total_packages": len(rows),
			"results":        rows,
		})
	}
}

// errNoRows returns true for pgx "no rows" errors.
func errNoRows(err error) bool {
	return err != nil && err.Error() == "no rows in result set"
}

// suppress unused import warnings for pgtype (used in dbsqlc types)
var _ pgtype.Timestamptz
