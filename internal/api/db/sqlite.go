package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS packages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    ecosystem       TEXT NOT NULL,
    name            TEXT NOT NULL,
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (ecosystem, name)
);

CREATE TABLE IF NOT EXISTS package_versions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    package_id      INTEGER NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    version         TEXT NOT NULL,
    source_url      TEXT NOT NULL DEFAULT '',
    checksum        TEXT NOT NULL DEFAULT '',
    published_at    TEXT NOT NULL DEFAULT (datetime('now')),
    metadata        TEXT,
    scan_status     TEXT NOT NULL DEFAULT 'pending',
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (package_id, version)
);

CREATE TABLE IF NOT EXISTS scan_results (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    package_version_id  INTEGER NOT NULL REFERENCES package_versions(id) ON DELETE CASCADE,
    sha256              TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL DEFAULT 'complete',
    total_findings      INTEGER NOT NULL DEFAULT 0,
    critical_findings   INTEGER NOT NULL DEFAULT 0,
    high_findings       INTEGER NOT NULL DEFAULT 0,
    medium_findings     INTEGER NOT NULL DEFAULT 0,
    low_findings        INTEGER NOT NULL DEFAULT 0,
    highest_severity    TEXT NOT NULL DEFAULT 'none',
    findings_json       TEXT NOT NULL DEFAULT '[]',
    error_message       TEXT NOT NULL DEFAULT '',
    scanned_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS advisories (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    package_version_id      INTEGER NOT NULL REFERENCES package_versions(id) ON DELETE CASCADE,
    severity                TEXT NOT NULL,
    confidence              REAL,
    advisory_text           TEXT NOT NULL,
    exploitability_rationale TEXT NOT NULL DEFAULT '',
    agentic_risk            TEXT,
    recommended_action      TEXT NOT NULL,
    patch_suggestion        TEXT,
    generated_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS attestations (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    package_version_id  INTEGER NOT NULL REFERENCES package_versions(id) ON DELETE CASCADE,
    sha256              TEXT NOT NULL DEFAULT '',
    signature           TEXT NOT NULL DEFAULT '',
    public_key          TEXT NOT NULL DEFAULT '',
    rekor_log_id        TEXT NOT NULL DEFAULT '',
    rekor_index         INTEGER NOT NULL DEFAULT 0,
    rekor_url           TEXT NOT NULL DEFAULT '',
    attestation_json    TEXT NOT NULL DEFAULT '{}',
    signed_at           TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS allowlist (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    ecosystem TEXT NOT NULL,
    package   TEXT NOT NULL,
    reason    TEXT NOT NULL DEFAULT '',
    added_by  TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (ecosystem, package)
);

CREATE TABLE IF NOT EXISTS alerts (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    type         TEXT NOT NULL,
    message      TEXT NOT NULL,
    severity     TEXT NOT NULL DEFAULT 'info',
    package_name TEXT,
    ecosystem    TEXT,
    version      TEXT,
    metadata     TEXT,
    dismissed    INTEGER NOT NULL DEFAULT 0,
    occurred_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS scraper_state (
    ecosystem   TEXT PRIMARY KEY,
    last_run    TEXT NOT NULL DEFAULT '1970-01-01T00:00:00Z',
    last_count  INTEGER NOT NULL DEFAULT 0,
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
`

// ConnectSQLite opens (or creates) an embedded SQLite database at the given
// path and runs the schema migration. This is the zero-config default when
// DATABASE_URL is not set.
func ConnectSQLite(ctx context.Context, dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("sqlite: create dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: ping: %w", err)
	}

	if _, err := db.ExecContext(ctx, sqliteSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: migrate: %w", err)
	}

	return db, nil
}

// DefaultSQLitePath returns ~/.forgeguardian/forgeguardian.db.
func DefaultSQLitePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".forgeguardian", "forgeguardian.db"), nil
}
