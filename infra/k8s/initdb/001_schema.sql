-- ForgeGuardian schema
-- Run once on a fresh database.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- ── Packages ────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS packages (
    id              BIGSERIAL PRIMARY KEY,
    ecosystem       TEXT NOT NULL,
    name            TEXT NOT NULL,
    latest_version  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ecosystem, name)
);

CREATE INDEX IF NOT EXISTS idx_packages_ecosystem ON packages(ecosystem);
CREATE INDEX IF NOT EXISTS idx_packages_name_trgm ON packages USING GIN (name gin_trgm_ops);

-- ── Package versions ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS package_versions (
    id              BIGSERIAL PRIMARY KEY,
    ecosystem       TEXT NOT NULL,
    name            TEXT NOT NULL,
    version         TEXT NOT NULL,
    source_url      TEXT NOT NULL,
    checksum        TEXT NOT NULL,  -- sha256 from registry
    published_at    TIMESTAMPTZ NOT NULL,
    metadata        JSONB,
    scan_status     TEXT NOT NULL DEFAULT 'pending',
    -- scan_status: pending | building | scanning | done | failed | blocked
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ecosystem, name, version)
);

CREATE INDEX IF NOT EXISTS idx_pv_ecosystem_name ON package_versions(ecosystem, name);
CREATE INDEX IF NOT EXISTS idx_pv_scan_status    ON package_versions(scan_status);
CREATE INDEX IF NOT EXISTS idx_pv_published_at   ON package_versions(published_at DESC);

-- ── Build jobs ──────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS build_jobs (
    id                  BIGSERIAL PRIMARY KEY,
    package_version_id  BIGINT NOT NULL REFERENCES package_versions(id) ON DELETE CASCADE,
    status              TEXT NOT NULL DEFAULT 'queued',
    -- queued | running | success | failed
    started_at          TIMESTAMPTZ,
    finished_at         TIMESTAMPTZ,
    build_log           TEXT,
    artifact_path       TEXT,   -- MinIO object key
    artifact_sha256     TEXT,
    reproducible        BOOLEAN,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_bj_status ON build_jobs(status);

-- ── Scan results ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS scan_results (
    id                  BIGSERIAL PRIMARY KEY,
    package_version_id  BIGINT NOT NULL REFERENCES package_versions(id) ON DELETE CASCADE,
    scanner             TEXT NOT NULL,  -- grype | semgrep | behavioral | ai_triage
    severity            TEXT,
    findings            JSONB NOT NULL DEFAULT '[]',
    advisory            TEXT,
    scanned_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sr_pv_id    ON scan_results(package_version_id);
CREATE INDEX IF NOT EXISTS idx_sr_severity ON scan_results(severity);

-- ── Advisories ───────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS advisories (
    id                  BIGSERIAL PRIMARY KEY,
    ecosystem           TEXT NOT NULL,
    name                TEXT NOT NULL,
    version             TEXT NOT NULL,
    severity            TEXT NOT NULL,
    confidence          NUMERIC(4,3),
    advisory_text       TEXT NOT NULL,
    recommended_action  TEXT NOT NULL,
    agentic_risk        TEXT,
    patch_suggestion    TEXT,
    published_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ecosystem, name, version)
);

CREATE INDEX IF NOT EXISTS idx_adv_severity ON advisories(severity);
CREATE INDEX IF NOT EXISTS idx_adv_published ON advisories(published_at DESC);

-- ── Scraper state ─────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS scraper_state (
    ecosystem   TEXT PRIMARY KEY,
    last_run    TIMESTAMPTZ NOT NULL DEFAULT '1970-01-01',
    last_count  INT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
