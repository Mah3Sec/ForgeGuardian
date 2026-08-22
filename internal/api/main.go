// Package main is the ForgeGuardian REST API server.
//
// Endpoints:
//
//	GET  /healthz                                   liveness probe
//	GET  /metrics                                   Prometheus metrics
//	GET  /api/v1/packages                           list packages (paginated)
//	GET  /api/v1/packages/:ecosystem/:name          package detail
//	GET  /api/v1/packages/:ecosystem/:name/versions list versions
//	POST /api/v1/scan                               trigger scan (async — returns job_id)
//	POST /api/v1/scan/upload                        trigger scan on uploaded archive (async)
//	GET  /api/v1/jobs/:id                           poll async scan job status/result
//	GET  /api/v1/scan/:ecosystem/:name/:version     get scan results
//	POST /api/v1/advisory                           generate AI advisory
//	GET  /api/v1/sbom/:ecosystem/:name/:version     get SBOM
//	POST /api/v1/sign                               sign artifact
//	GET  /api/v1/verify                             verify attestation
//	GET  /api/v1/dashboard/stats                    dashboard summary stats
//	GET  /api/v1/dashboard/recent                   recent scan activity
//	GET  /api/v1/dashboard/graph                    dependency graph of recent scans
//	GET  /api/v1/intelligence/signatures            list detection signatures
//	POST /api/v1/intelligence/refresh               trigger intel agent run
//	POST /api/v1/provenance                         generate SLSA provenance (no signing/persistence)
//	GET  /api/v1/policy/status                      current policy config + status
//	PUT  /api/v1/policy                             save policy configuration
//	POST /api/v1/intelligence/signatures            author a new detection signature (returns YAML)
//	POST /api/v1/intelligence/validate              validate a signature YAML document
//	POST /api/v1/intelligence/test                  heuristic signature test against a live scan
//	GET  /api/v1/audit/stats                        signature store runtime statistics
//	POST /api/v1/cli/sync                            accept CLI scan results for dashboard display
//	POST /api/v1/auth/login                         dashboard login (sets session cookie)
//	POST /api/v1/auth/logout                        dashboard logout (clears session cookie)
//	GET  /api/v1/auth/me                            current dashboard session auth state
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mah3sec/forgeguardian/internal/api/db"
	"github.com/mah3sec/forgeguardian/internal/api/handlers"
	"github.com/mah3sec/forgeguardian/internal/api/middleware"
	"github.com/mah3sec/forgeguardian/internal/auth"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := loadConfig()

	isDefaultCreds := cfg.AdminPassword == "changeme123"
	adminIdentity, err := auth.LoadOrBootstrapAdmin(cfg.AdminEmail, cfg.AdminPassword, isDefaultCreds)
	if err != nil {
		logger.Error("admin bootstrap failed", "error", err)
		os.Exit(1)
	}
	authEnabled := adminIdentity != nil
	if authEnabled && cfg.SessionSecret == "" {
		logger.Error("FG_ADMIN_EMAIL/FG_ADMIN_PASSWORD are set but FG_SESSION_SECRET is not — refusing to start with weak/no session signing secret")
		os.Exit(1)
	}
	if authEnabled {
		logger.Info("dashboard login enabled", "email", adminIdentity.Email)
		if adminIdentity.PasswordMustChange {
			logger.Warn("running with default credentials — password change required on first login")
		}
	} else {
		logger.Warn("FG_ADMIN_EMAIL/FG_ADMIN_PASSWORD not set — dashboard login disabled (open access, dev mode)")
	}

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Database — optional: API degrades gracefully when DATABASE_URL is unset.
	var pool *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		var err error
		dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
		pool, err = db.Connect(dbCtx, cfg.DatabaseURL)
		dbCancel()
		if err != nil {
			logger.Warn("database unavailable — DB-backed endpoints will return 503", "error", err)
		} else {
			logger.Info("database connected")
			defer pool.Close()
			migrCtx, migrCancel := context.WithTimeout(context.Background(), 30*time.Second)
			if migrErr := db.Migrate(migrCtx, pool); migrErr != nil {
				migrCancel()
				logger.Error("migration failed — check schema", "error", migrErr)
				os.Exit(1)
			}
			migrCancel()
			logger.Info("database migrations applied")
		}
	} else {
		logger.Warn("DATABASE_URL not set — DB-backed endpoints will return 503")
	}

	r := gin.New()
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.CORS(cfg.DashboardOrigin))
	r.Use(middleware.DualAuth(cfg.APIKey, []byte(cfg.SessionSecret), authEnabled))
	// 60 req/s per IP, burst of 20. Advisory endpoint calls Anthropic — keep headroom low.
	r.Use(middleware.RateLimiter(60, 20))

	if cfg.APIKey == "" {
		logger.Warn("FG_API_KEY not set — API auth disabled (dev mode)")
	} else {
		logger.Info("API key auth enabled")
	}

	// Health + metrics
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1
	hcfg := &handlers.ServerConfig{
		AnthropicAPIKey: cfg.AnthropicAPIKey,
		RekorURL:        cfg.RekorURL,
		IntelStorePath:  cfg.IntelStorePath,
		AdminIdentity:   adminIdentity,
		SessionSecret:   []byte(cfg.SessionSecret),
		CookieSecure:    cfg.CookieSecure,
	}
	h := handlers.New(hcfg, logger, pool)

	// Re-wire the global logger to tee into the handler's log ring buffer
	// so the dashboard can display server-side logs.
	captureLogger := slog.New(handlers.NewLogCaptureHandler(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
		h.LogRing(),
	))
	slog.SetDefault(captureLogger)

	v1 := r.Group("/api/v1")
	{
		v1.GET("/packages", h.ListPackages)
		v1.GET("/packages/:ecosystem/:name", h.GetPackage)
		v1.GET("/packages/:ecosystem/:name/versions", h.ListVersions)
		v1.POST("/scan", h.TriggerScan)
		v1.POST("/scan/upload", h.ScanUpload)
		v1.POST("/scan/remote", h.TriggerRemoteScan)
		v1.GET("/scan/:ecosystem/:name/:version", h.GetScanResults)
		v1.GET("/jobs/:id", h.GetJobStatus)
		v1.POST("/advisory", h.GenerateAdvisory)
		v1.GET("/sbom/:ecosystem/:name/:version", h.GetSBOM)
		v1.POST("/sign", h.SignArtifact)
		v1.POST("/verify", h.VerifyAttestation)
		v1.POST("/provenance", h.GenerateProvenance)
		v1.GET("/dashboard/stats", h.DashboardStats)
		v1.GET("/dashboard/recent", h.DashboardRecent)
		v1.GET("/dashboard/timeline", h.DashboardTimeline)
		v1.GET("/dashboard/graph", h.DashboardGraph)
		v1.GET("/intelligence/signatures", h.ListSignatures)
		v1.POST("/intelligence/refresh", h.RefreshIntelligence)
		v1.GET("/dashboard/activity", h.DashboardActivity)
		v1.GET("/logs", h.ServerLogs)
		v1.GET("/risks", h.ActiveRisks)
		v1.GET("/policy/status", h.PolicyStatus)
		v1.PUT("/policy", h.SavePolicy)
		v1.POST("/intelligence/signatures", h.GenerateSignature)
		v1.POST("/intelligence/validate", h.ValidateSignatureYAML)
		v1.POST("/intelligence/test", h.TestSignature)
		v1.GET("/audit/stats", h.AuditStats)
		v1.POST("/audit/trigger", h.TriggerAudit)
		v1.POST("/webhooks/test", h.WebhookTest)
		v1.GET("/agent/stream", h.AgentStream)
		v1.POST("/agent/events", h.PublishAgentEvent)
		v1.GET("/allowlist", h.ListAllowlist)
		v1.POST("/allowlist", h.AddAllowlist)
		v1.DELETE("/allowlist/:id", h.DeleteAllowlist)
		v1.GET("/allowlist/check", h.CheckAllowlist)
		v1.GET("/alerts", h.ListAlerts)
		v1.POST("/alerts", h.CreateAlert)
		v1.POST("/alerts/:id/dismiss", h.DismissAlert)
		v1.GET("/export/report", h.ExportReport)
		v1.POST("/cli/sync", h.CLISync)
		v1.POST("/auth/login", middleware.LoginRateLimiter(), h.Login)
		v1.POST("/auth/logout", h.Logout)
		v1.POST("/auth/password", h.ChangePassword)
		v1.GET("/auth/me", h.AuthMe)
	}

	// Embedded dashboard: serve pre-built static files with SPA fallback.
	if cfg.DashboardDir != "" {
		if _, err := os.Stat(filepath.Join(cfg.DashboardDir, "index.html")); err == nil {
			fs := http.Dir(cfg.DashboardDir)
			r.NoRoute(func(c *gin.Context) {
				p := c.Request.URL.Path
				if strings.HasPrefix(p, "/api/") {
					c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
					return
				}
				if f, err := fs.Open(p); err == nil {
					stat, _ := f.Stat()
					f.Close()
					if stat != nil && !stat.IsDir() {
						http.FileServer(fs).ServeHTTP(c.Writer, c.Request)
						return
					}
				}
				c.File(filepath.Join(cfg.DashboardDir, "index.html"))
			})
			logger.Info("embedded dashboard enabled", "dir", cfg.DashboardDir)
		} else {
			logger.Warn("DASHBOARD_DIR set but index.html not found", "dir", cfg.DashboardDir)
		}
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	go func() {
		logger.Info("api server listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
