// Package db provides database connection and migration helpers for the ForgeGuardian API.
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Connect opens a pgxpool connection to the given DATABASE_URL.
// Returns an error if the URL is empty or the connection cannot be established.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("db: DATABASE_URL is not set")
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}

// Migrate runs all pending SQL migrations in migrations/ in filename order.
// Each migration file is split on "-- +migrate Down" and only the Up section is executed.
// A schema_migrations table tracks which files have already been applied.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	// Ensure tracking table exists.
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("db migrate: create tracking table: %w", err)
	}

	// Read applied migrations.
	rows, err := pool.Query(ctx, `SELECT filename FROM schema_migrations ORDER BY filename`)
	if err != nil {
		return fmt.Errorf("db migrate: query applied: %w", err)
	}
	applied := make(map[string]bool)
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			rows.Close()
			return fmt.Errorf("db migrate: scan row: %w", err)
		}
		applied[f] = true
	}
	rows.Close()

	// Collect migration files.
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("db migrate: read dir: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		if applied[name] {
			continue
		}
		data, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("db migrate: read %s: %w", name, err)
		}
		// Only run the Up section (before "-- +migrate Down").
		upSQL := strings.SplitN(string(data), "-- +migrate Down", 2)[0]

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("db migrate: begin tx for %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, upSQL); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("db migrate: exec %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("db migrate: record %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("db migrate: commit %s: %w", name, err)
		}
	}
	return nil
}
