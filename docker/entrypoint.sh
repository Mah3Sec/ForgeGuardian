#!/bin/sh
set -e

# ── Auto-generate session secret if not provided ─────────────────────────────
if [ -z "$FG_SESSION_SECRET" ]; then
    export FG_SESSION_SECRET=$(cat /dev/urandom | tr -dc 'a-f0-9' | head -c 64)
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
export PORT="${PORT:-3000}"

exec /app/forgeguardian-api "$@"
