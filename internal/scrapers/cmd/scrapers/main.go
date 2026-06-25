// Package main is the ForgeGuardian registry scraper service.
// Run with --dry-run --limit=10 to test without a database.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/mah3sec/forgeguardian/internal/scrapers"
	"github.com/mah3sec/forgeguardian/internal/scrapers/crates"
	gomod "github.com/mah3sec/forgeguardian/internal/scrapers/go"
	"github.com/mah3sec/forgeguardian/internal/scrapers/huggingface"
	mcp "github.com/mah3sec/forgeguardian/internal/scrapers/mcp_registry"
	"github.com/mah3sec/forgeguardian/internal/scrapers/maven"
	"github.com/mah3sec/forgeguardian/internal/scrapers/npm"
	"github.com/mah3sec/forgeguardian/internal/scrapers/oci"
	"github.com/mah3sec/forgeguardian/internal/scrapers/pypi"
	"github.com/mah3sec/forgeguardian/internal/scrapers/rubygems"
)

func main() {
	var (
		dryRun      = flag.Bool("dry-run", false, "Run scrapers without writing to the database or queue")
		limit       = flag.Int("limit", 0, "Maximum number of versions to return per ecosystem (0 = unlimited)")
		ecosystem   = flag.String("ecosystem", "", "Scrape only this ecosystem (default: all)")
		since       = flag.Duration("since", 24*time.Hour, "Look back this far in time for new packages")
		jsonOutput  = flag.Bool("json", false, "Output results as JSON")
		databaseURL = flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
		redisURL    = flag.String("redis-url", os.Getenv("REDIS_URL"), "Redis connection string")
		watch       = flag.Bool("watch", false, "Run continuously on a ticker (uses FG_SCRAPER_INTERVAL env or default 30m)")
	)
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *dryRun {
		log.Info("dry-run mode: database and queue writes are disabled")
	}

	// Build scraper list
	allScrapers := []core.RegistryScraper{
		npm.New(),
		pypi.New(),
		maven.New(),
		gomod.New(),
		rubygems.New(),
		crates.New(),
		huggingface.New(os.Getenv("HF_TOKEN")),
		mcp.New(),
		oci.New(os.Getenv("DOCKER_HUB_TOKEN"), os.Getenv("GHCR_TOKEN")),
	}

	// Filter by ecosystem if requested
	if *ecosystem != "" {
		filtered := allScrapers[:0]
		for _, sc := range allScrapers {
			if sc.Name() == *ecosystem {
				filtered = append(filtered, sc)
			}
		}
		if len(filtered) == 0 {
			fmt.Fprintf(os.Stderr, "error: unknown ecosystem %q\n", *ecosystem)
			os.Exit(1)
		}
		allScrapers = filtered
	}

	// Wire up storage if not dry-run
	var store scrapers.VersionStore
	if !*dryRun {
		if *databaseURL == "" {
			log.Error("DATABASE_URL is required (or use --dry-run)")
			os.Exit(1)
		}
		if *redisURL == "" {
			log.Error("REDIS_URL is required (or use --dry-run)")
			os.Exit(1)
		}
		s, err := scrapers.NewVersionStore(context.Background(), *databaseURL, *redisURL)
		if err != nil {
			log.Error("failed to connect to storage", "err", err)
			os.Exit(1)
		}
		defer s.Close()
		store = s
	}

	// Compute per-ecosystem lastRun times
	ctx := context.Background()
	lastRunFor := make(map[string]time.Time)
	lastRunDefault := time.Now().Add(-*since)

	for _, sc := range allScrapers {
		if !*dryRun && store != nil {
			t, err := store.GetLastRun(ctx, sc.Name())
			if err != nil {
				log.Warn("could not get last run time", "ecosystem", sc.Name(), "err", err)
				lastRunFor[sc.Name()] = lastRunDefault
			} else {
				lastRunFor[sc.Name()] = t
			}
		} else {
			lastRunFor[sc.Name()] = lastRunDefault
		}
	}

	// Run orchestrator
	orch := scrapers.NewOrchestrator(allScrapers, log)
	versions, errs := orch.RunOnce(ctx, lastRunFor, *limit)

	if len(errs) > 0 {
		for _, e := range errs {
			log.Error("scraper error", "err", e)
		}
	}

	// Persist or print results
	if *dryRun || *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		summary := struct {
			DryRun   bool                  `json:"dry_run"`
			Count    int                   `json:"count"`
			Versions []core.PackageVersion `json:"versions"`
		}{
			DryRun:   *dryRun,
			Count:    len(versions),
			Versions: versions,
		}
		if err := enc.Encode(summary); err != nil {
			log.Error("failed to encode output", "err", err)
			os.Exit(1)
		}
		return
	}

	// Write to DB + enqueue build jobs
	var written, queued int
	for _, v := range versions {
		id, isNew, err := store.UpsertVersion(ctx, v)
		if err != nil {
			log.Error("failed to upsert version", "package", v.Name, "version", v.Version, "err", err)
			continue
		}
		written++
		if isNew {
			if err := store.EnqueueBuild(ctx, id, v); err != nil {
				log.Error("failed to enqueue build job", "id", id, "err", err)
			} else {
				queued++
			}
		}
	}

	log.Info("scrape complete",
		"total_versions", len(versions),
		"written", written,
		"build_jobs_queued", queued,
		"errors", len(errs),
	)

	if !*watch || *dryRun || store == nil {
		return
	}

	// Continuous scheduler: re-run every FG_SCRAPER_INTERVAL (default 30m).
	interval := 30 * time.Minute
	if v := os.Getenv("FG_SCRAPER_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		} else {
			log.Warn("invalid FG_SCRAPER_INTERVAL, using default", "default", interval)
		}
	}
	log.Info("scraper scheduler started", "interval", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		// Refresh lastRun from DB so each tick only fetches what's new.
		for _, sc := range allScrapers {
			if store != nil {
				t, err := store.GetLastRun(ctx, sc.Name())
				if err != nil {
					lastRunFor[sc.Name()] = time.Now().Add(-interval)
				} else {
					lastRunFor[sc.Name()] = t
				}
			}
		}

		vs, errs := orch.RunOnce(ctx, lastRunFor, *limit)
		if len(errs) > 0 {
			for _, e := range errs {
				log.Error("scraper error", "err", e)
			}
		}

		var w, q int
		for _, v := range vs {
			id, isNew, err := store.UpsertVersion(ctx, v)
			if err != nil {
				log.Error("upsert failed", "package", v.Name, "err", err)
				continue
			}
			w++
			if isNew {
				if err := store.EnqueueBuild(ctx, id, v); err != nil {
					log.Error("enqueue failed", "id", id, "err", err)
				} else {
					q++
				}
			}
		}
		log.Info("tick complete", "written", w, "queued", q, "errors", len(errs))
	}
}
