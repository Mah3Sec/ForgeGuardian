// Package store provides typed database access for scraped package data.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/mah3sec/forgeguardian/internal/core"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps a pgx connection pool and provides typed query methods.
type Store struct {
	pool *pgxpool.Pool
}

// New creates a Store from a PostgreSQL connection string.
func New(ctx context.Context, connStr string) (*Store, error) {
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the connection pool.
func (s *Store) Close() { s.pool.Close() }

// UpsertPackageVersion inserts or updates a package version row.
// Returns the row id and whether it was newly inserted.
func (s *Store) UpsertPackageVersion(ctx context.Context, pv core.PackageVersion) (id int64, isNew bool, err error) {
	const q = `
		INSERT INTO package_versions (ecosystem, name, version, source_url, checksum, published_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (ecosystem, name, version) DO UPDATE
		  SET source_url   = EXCLUDED.source_url,
		      checksum     = EXCLUDED.checksum,
		      updated_at   = now()
		RETURNING id, (xmax = 0) AS inserted`

	var inserted bool
	err = s.pool.QueryRow(ctx, q,
		pv.Ecosystem, pv.Name, pv.Version,
		pv.SourceURL, pv.Checksum, pv.PublishedAt,
		pv.Metadata,
	).Scan(&id, &inserted)
	if err != nil {
		return 0, false, fmt.Errorf("store: upsert package version: %w", err)
	}
	return id, inserted, nil
}

// GetScraperLastRun returns the timestamp of the last successful run for an ecosystem.
func (s *Store) GetScraperLastRun(ctx context.Context, ecosystem string) (time.Time, error) {
	const q = `SELECT last_run FROM scraper_state WHERE ecosystem = $1`
	var t time.Time
	err := s.pool.QueryRow(ctx, q, ecosystem).Scan(&t)
	if err != nil {
		if err == pgx.ErrNoRows {
			return time.Unix(0, 0), nil
		}
		return time.Time{}, fmt.Errorf("store: get scraper last run: %w", err)
	}
	return t, nil
}

// SetScraperLastRun updates the last-run timestamp for an ecosystem.
func (s *Store) SetScraperLastRun(ctx context.Context, ecosystem string, t time.Time, count int) error {
	const q = `
		INSERT INTO scraper_state (ecosystem, last_run, last_count)
		VALUES ($1, $2, $3)
		ON CONFLICT (ecosystem) DO UPDATE
		  SET last_run   = EXCLUDED.last_run,
		      last_count = EXCLUDED.last_count,
		      updated_at = now()`
	_, err := s.pool.Exec(ctx, q, ecosystem, t, count)
	if err != nil {
		return fmt.Errorf("store: set scraper last run: %w", err)
	}
	return nil
}

// UpsertPackage creates or updates the top-level package record.
func (s *Store) UpsertPackage(ctx context.Context, ecosystem, name, latestVersion string) error {
	const q = `
		INSERT INTO packages (ecosystem, name, latest_version)
		VALUES ($1, $2, $3)
		ON CONFLICT (ecosystem, name) DO UPDATE
		  SET latest_version = EXCLUDED.latest_version,
		      updated_at     = now()`
	_, err := s.pool.Exec(ctx, q, ecosystem, name, latestVersion)
	if err != nil {
		return fmt.Errorf("store: upsert package: %w", err)
	}
	return nil
}
