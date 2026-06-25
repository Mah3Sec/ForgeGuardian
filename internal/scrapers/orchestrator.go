package scrapers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
)

// PollResult is the outcome of a single scraper's poll run.
type PollResult struct {
	Ecosystem string
	Versions  []core.PackageVersion
	Err       error
	Duration  time.Duration
}

// Orchestrator runs all registered scrapers concurrently on a schedule.
type Orchestrator struct {
	scrapers []core.RegistryScraper
	log      *slog.Logger
}

// NewOrchestrator creates an Orchestrator with the given set of scrapers.
func NewOrchestrator(scrapers []core.RegistryScraper, log *slog.Logger) *Orchestrator {
	return &Orchestrator{scrapers: scrapers, log: log}
}

// PollAll runs all scrapers concurrently and collects results.
// lastRunFor maps ecosystem name → last run time; missing entries default to epoch.
func (o *Orchestrator) PollAll(ctx context.Context, lastRunFor map[string]time.Time) []PollResult {
	results := make([]PollResult, len(o.scrapers))
	var wg sync.WaitGroup

	for i, sc := range o.scrapers {
		i, sc := i, sc
		wg.Add(1)
		go func() {
			defer wg.Done()
			lastRun := lastRunFor[sc.Name()]
			start := time.Now()
			versions, err := sc.Poll(ctx, lastRun)
			results[i] = PollResult{
				Ecosystem: sc.Name(),
				Versions:  versions,
				Err:       err,
				Duration:  time.Since(start),
			}
		}()
	}
	wg.Wait()
	return results
}

// RunOnce executes a single poll across all scrapers and returns all discovered versions.
// It is the core loop body used by both the scheduler and the dry-run CLI.
func (o *Orchestrator) RunOnce(ctx context.Context, lastRunFor map[string]time.Time, limit int) ([]core.PackageVersion, []error) {
	results := o.PollAll(ctx, lastRunFor)

	var allVersions []core.PackageVersion
	var errs []error

	for _, r := range results {
		if r.Err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.Ecosystem, r.Err))
			o.log.Error("scraper error", "ecosystem", r.Ecosystem, "err", r.Err, "duration", r.Duration)
			continue
		}

		o.log.Info("scraper completed",
			"ecosystem", r.Ecosystem,
			"count", len(r.Versions),
			"duration", r.Duration)

		vs := r.Versions
		if limit > 0 && len(vs) > limit {
			vs = vs[:limit]
		}
		allVersions = append(allVersions, vs...)
	}
	return allVersions, errs
}
