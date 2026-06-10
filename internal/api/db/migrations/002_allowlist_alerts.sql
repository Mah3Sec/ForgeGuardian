-- +migrate Up

CREATE TABLE IF NOT EXISTS allowlist (
    id          BIGSERIAL PRIMARY KEY,
    ecosystem   TEXT NOT NULL DEFAULT '',
    package     TEXT NOT NULL,
    reason      TEXT NOT NULL DEFAULT '',
    added_by    TEXT NOT NULL DEFAULT 'api',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ecosystem, package)
);

CREATE INDEX IF NOT EXISTS idx_allowlist_package ON allowlist(package);

CREATE TABLE IF NOT EXISTS alerts (
    id          BIGSERIAL PRIMARY KEY,
    type        TEXT NOT NULL,      -- scan_complete | finding_critical | policy_violation | signature_match | new_package
    message     TEXT NOT NULL,
    severity    TEXT NOT NULL DEFAULT 'INFORMATIONAL',
    package_name TEXT,
    ecosystem   TEXT,
    version     TEXT,
    metadata    JSONB,
    dismissed   BOOLEAN NOT NULL DEFAULT false,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_alerts_occurred  ON alerts(occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_alerts_severity  ON alerts(severity);
CREATE INDEX IF NOT EXISTS idx_alerts_dismissed ON alerts(dismissed);

-- +migrate Down
DROP TABLE IF EXISTS alerts;
DROP TABLE IF EXISTS allowlist;
