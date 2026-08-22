#!/bin/sh
set -e

# ── Auto-generate session secret if not provided ─────────────────────────────
SECRET_FILE="/data/.session-secret"
if [ -z "$FG_SESSION_SECRET" ]; then
    if [ -f "$SECRET_FILE" ]; then
        export FG_SESSION_SECRET=$(cat "$SECRET_FILE")
    else
        export FG_SESSION_SECRET=$(cat /dev/urandom | tr -dc 'a-f0-9' | head -c 64)
        mkdir -p /data
        printf '%s' "$FG_SESSION_SECRET" > "$SECRET_FILE"
        chmod 600 "$SECRET_FILE"
    fi
fi

# ── Default admin credentials (demo mode) ────────────────────────────────────
if [ -z "$FG_ADMIN_EMAIL" ]; then
    export FG_ADMIN_EMAIL="admin@forgeguardian.local"
    export FG_ADMIN_PASSWORD="changeme123"
    echo ""
    echo "┌─────────────────────────────────────────────────────────────┐"
    echo "│  ForgeGuardian — running with default credentials           │"
    echo "│                                                             │"
    echo "│  Email:    admin@forgeguardian.local                        │"
    echo "│  Password: changeme123                                      │"
    echo "│                                                             │"
    echo "│  Change them:                                               │"
    echo "│  docker run -e FG_ADMIN_EMAIL=you@example.com \\             │"
    echo "│             -e FG_ADMIN_PASSWORD=YourSecurePass \\            │"
    echo "│             -p 3000:3000 ghcr.io/mah3sec/forgeguardian      │"
    echo "└─────────────────────────────────────────────────────────────┘"
    echo ""
fi

# ── All-in-one defaults ──────────────────────────────────────────────────────
export DASHBOARD_DIR="${DASHBOARD_DIR:-/app/dashboard}"
export FG_COOKIE_SECURE="${FG_COOKIE_SECURE:-false}"
export FG_CACHE_PATH="${FG_CACHE_PATH:-/data/scan-cache.json}"
export PORT="${PORT:-3000}"

# ── Database status ──────────────────────────────────────────────────────────
if [ -z "$DATABASE_URL" ]; then
    echo ""
    echo "  ℹ  No DATABASE_URL set — using file-based cache (/data/scan-cache.json)"
    echo "     Scan history persists across restarts via the /data volume."
    echo "     For full PostgreSQL support, use docker compose:"
    echo ""
    echo "       docker compose up -d"
    echo ""
fi

exec /app/forgeguardian-api "$@"
