// Command intel-agent is the ForgeGuardian real-time intelligence daemon.
//
// It polls threat feeds (OSV, OpenSSF malicious-packages, popularity APIs),
// runs a Claude AI agent loop to generate typed detection signatures, and
// writes them to the ForgeGuardian signature store.
//
// Usage:
//
//	intel-agent [flags]
//
// Flags:
//
//	--store=~/.forgeguardian/signatures.json  path to signature store (default: ~/.forgeguardian/signatures.json)
//	--loop                                    run continuously (default: one-shot)
//	--interval=6h                             poll interval in loop mode
//	--api-key=...                             Anthropic API key (or ANTHROPIC_API_KEY env)
//	--ecosystems=npm,pypi,go                  comma-separated ecosystems to poll (default: npm,pypi,go,rubygems,crates)
//	--max-ossf=100                            max malicious-packages entries to fetch per ecosystem
//	--dry-run                                 print signatures without saving to disk
//	--skip-ai                                 skip the AI generation step (feeds only)
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/mah3sec/forgeguardian/internal/intelligence"
	"github.com/mah3sec/forgeguardian/internal/intelligence/agent"
	"github.com/mah3sec/forgeguardian/internal/intelligence/feeds"
)

func main() {
	storePath  := flag.String("store", "", "path to signature store (default: ~/.forgeguardian/signatures.json)")
	loop       := flag.Bool("loop", false, "run continuously")
	interval   := flag.Duration("interval", 6*time.Hour, "poll interval in loop mode")
	apiKey     := flag.String("api-key", "", "Anthropic API key (or ANTHROPIC_API_KEY env)")
	ecosystems := flag.String("ecosystems", "npm,pypi,go,rubygems,crates", "comma-separated ecosystems to poll")
	maxOSSF    := flag.Int("max-ossf", 100, "max OpenSSF malicious-packages entries per ecosystem")
	dryRun     := flag.Bool("dry-run", false, "print signatures without saving")
	skipAI     := flag.Bool("skip-ai", false, "skip AI generation step")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *apiKey == "" {
		*apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if *apiKey == "" && !*skipAI {
		logger.Error("ANTHROPIC_API_KEY not set — pass --api-key or set the env var, or use --skip-ai")
		os.Exit(1)
	}

	if *storePath == "" {
		var err error
		*storePath, err = intelligence.DefaultStorePath()
		if err != nil {
			logger.Error("cannot determine store path", "error", err)
			os.Exit(1)
		}
	}

	ecos := strings.Split(*ecosystems, ",")
	for i, e := range ecos {
		ecos[i] = strings.TrimSpace(e)
	}

	if *loop {
		logger.Info("intel-agent starting in loop mode", "interval", interval.String(), "store", *storePath)
		for {
			if err := runOnce(logger, *storePath, *apiKey, ecos, *maxOSSF, *dryRun, *skipAI); err != nil {
				logger.Error("run failed", "error", err)
			}
			logger.Info("sleeping until next run", "interval", interval.String())
			time.Sleep(*interval)
		}
	} else {
		if err := runOnce(logger, *storePath, *apiKey, ecos, *maxOSSF, *dryRun, *skipAI); err != nil {
			logger.Error("run failed", "error", err)
			os.Exit(1)
		}
	}
}

func runOnce(logger *slog.Logger, storePath, apiKey string, ecosystems []string, maxOSSF int, dryRun, skipAI bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	logger.Info("loading signature store", "path", storePath)
	store, err := intelligence.LoadStore(storePath)
	if err != nil {
		return fmt.Errorf("load store: %w", err)
	}
	logger.Info("store loaded", "existing_signatures", len(store.Signatures))

	// Build seed package map for popularity and OSV pollers
	seedPackages := buildSeedPackages(ecosystems)

	// --- Feed polling ---

	var osvSigs []intelligence.DetectionSignature
	logger.Info("polling OSV vulnerability feed")
	osvPoller := feeds.NewOSVPoller()
	osvSigs, err = osvPoller.Poll(ctx, seedPackages)
	if err != nil {
		logger.Warn("OSV poll error", "error", err)
	}
	logger.Info("OSV feed complete", "findings", len(osvSigs))

	logger.Info("polling OpenSSF malicious-packages feed")
	ossf := feeds.NewMaliciousPackagesPoller(maxOSSF)
	ossfSigs, err := ossf.Poll(ctx, ecosystems)
	if err != nil {
		logger.Warn("OpenSSF poll error", "error", err)
	}
	logger.Info("OpenSSF feed complete", "findings", len(ossfSigs))

	logger.Info("polling popularity feeds")
	popPoller := feeds.NewPopularityPoller(100)
	popSigs, err := popPoller.Poll(ctx, seedPackages)
	if err != nil {
		logger.Warn("popularity poll error", "error", err)
	}
	logger.Info("popularity feed complete", "new_targets", len(popSigs))

	// Direct feed signatures (no AI needed for these)
	feedSigs := append(osvSigs, ossfSigs...)
	feedSigs = append(feedSigs, popSigs...)

	// --- AI signature generation ---
	var aiSigs []intelligence.DetectionSignature
	if !skipAI && (len(osvSigs) > 0 || len(ossfSigs) > 0) {
		logger.Info("running AI signature generation agent")
		ag := agent.New(apiKey)
		prompt := agent.BuildThreatContext(osvSigs, ossfSigs)
		aiSigs, err = ag.GenerateSignatures(ctx, prompt)
		if err != nil {
			logger.Warn("AI agent error", "error", err)
		}
		logger.Info("AI agent complete", "generated_signatures", len(aiSigs))
	}

	allNew := append(feedSigs, aiSigs...)
	if dryRun {
		fmt.Printf("=== Dry Run: %d candidate signatures ===\n", len(allNew))
		for _, s := range allNew {
			fmt.Printf("[%s] %s/%s — %s (source: %s)\n",
				s.Severity, s.Ecosystem, coalesce(s.Package, s.Target, s.Rule, s.Pattern),
				s.Title, s.Source)
		}
		return nil
	}

	added := intelligence.MergeSignatures(store, allNew)
	logger.Info("merged signatures", "added", added, "total", len(store.Signatures))

	if added > 0 {
		if err := intelligence.SaveStore(storePath, store); err != nil {
			return fmt.Errorf("save store: %w", err)
		}
		logger.Info("store saved", "path", storePath, "total_signatures", len(store.Signatures))
	} else {
		logger.Info("no new signatures — store unchanged")
	}
	return nil
}

// buildSeedPackages returns a baseline package set per ecosystem for OSV and popularity polling.
func buildSeedPackages(ecosystems []string) map[string][]string {
	all := map[string][]string{
		"npm": {
			"lodash", "express", "react", "react-dom", "axios", "moment", "chalk",
			"commander", "webpack", "babel-core", "typescript", "eslint", "jest",
			"mocha", "request", "underscore", "async", "bluebird", "debug",
			"dotenv", "nodemon", "cors", "body-parser", "mongoose", "sequelize",
			"socket.io", "passport", "bcryptjs", "jsonwebtoken", "uuid",
		},
		"pypi": {
			"requests", "numpy", "pandas", "flask", "django", "boto3", "pillow",
			"cryptography", "setuptools", "pip", "wheel", "urllib3", "certifi",
			"six", "click", "pydantic", "fastapi", "httpx", "aiohttp",
			"sqlalchemy", "celery", "redis", "pytest", "black", "mypy",
			"transformers", "torch", "tensorflow", "scikit-learn", "langchain",
		},
		"go": {
			"github.com/gin-gonic/gin", "github.com/gorilla/mux",
			"github.com/stretchr/testify", "github.com/spf13/viper",
			"github.com/spf13/cobra", "go.uber.org/zap",
			"github.com/jackc/pgx/v5", "github.com/redis/go-redis/v9",
			"github.com/golang-jwt/jwt/v5", "github.com/prometheus/client_golang",
		},
		"rubygems": {
			"rails", "rake", "bundler", "devise", "rspec", "nokogiri",
			"activerecord", "sinatra", "sidekiq", "capistrano",
		},
		"crates": {
			"serde", "tokio", "rand", "log", "clap", "anyhow", "thiserror",
			"reqwest", "hyper", "axum", "actix-web", "diesel", "sqlx",
			"rayon", "crossbeam", "parking_lot",
		},
	}

	result := make(map[string][]string)
	for _, eco := range ecosystems {
		if pkgs, ok := all[eco]; ok {
			result[eco] = pkgs
		}
	}
	return result
}

func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return "?"
}
