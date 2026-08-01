#!/usr/bin/env bash
# ForgeGuardian bootstrap — validates environment and builds all components.
# Usage: bash scripts/bootstrap.sh
set -euo pipefail

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'; RESET='\033[0m'
PASS() { echo -e "  ${GREEN}[PASS]${RESET} $*"; }
WARN() { echo -e "  ${YELLOW}[WARN]${RESET} $*"; }
FAIL() { echo -e "  ${RED}[FAIL]${RESET} $*"; }

echo ""
echo "ForgeGuardian Bootstrap"
echo "────────────────────────────────────────"

ERRORS=0

# ── Go ────────────────────────────────────────────────────────────────────
if ! command -v go &>/dev/null; then
  FAIL "Go not found — install from https://go.dev/dl/"
  ERRORS=$((ERRORS+1))
else
  GO_VER=$(go version | awk '{print $3}' | sed 's/go//')
  MAJOR=$(echo "$GO_VER" | cut -d. -f1)
  MINOR=$(echo "$GO_VER" | cut -d. -f2)
  if [ "$MAJOR" -lt 1 ] || { [ "$MAJOR" -eq 1 ] && [ "$MINOR" -lt 23 ]; }; then
    FAIL "Go $GO_VER found but 1.23+ required — upgrade at https://go.dev/dl/"
    ERRORS=$((ERRORS+1))
  else
    PASS "Go $GO_VER"
  fi
fi

# ── Node ──────────────────────────────────────────────────────────────────
if ! command -v node &>/dev/null; then
  WARN "Node.js not found — dashboard features unavailable (install Node 20 LTS)"
else
  NODE_VER=$(node --version | sed 's/v//')
  NODE_MAJOR=$(echo "$NODE_VER" | cut -d. -f1)
  if [ "$NODE_MAJOR" -lt 20 ]; then
    WARN "Node $NODE_VER found — recommend upgrading to Node 20 LTS (nvm install 20)"
  else
    PASS "Node $NODE_VER"
  fi
fi

# ── npm ───────────────────────────────────────────────────────────────────
if ! command -v npm &>/dev/null; then
  WARN "npm not found — dashboard features unavailable"
else
  PASS "npm $(npm --version)"
fi

# ── Git ───────────────────────────────────────────────────────────────────
if ! command -v git &>/dev/null; then
  FAIL "git not found — required for builds"
  ERRORS=$((ERRORS+1))
else
  PASS "git $(git --version | awk '{print $3}')"
fi

# ── ANTHROPIC_API_KEY ────────────────────────────────────────────────────
if [ -z "${ANTHROPIC_API_KEY:-}" ]; then
  WARN "ANTHROPIC_API_KEY not set — AI advisory and auto-patch features unavailable"
  WARN "       Set it with: export ANTHROPIC_API_KEY=sk-ant-..."
else
  PASS "ANTHROPIC_API_KEY is set"
fi

# ── Optional scan tools ───────────────────────────────────────────────────
for tool in grype semgrep trivy; do
  if command -v "$tool" &>/dev/null; then
    PASS "$tool found"
  else
    WARN "$tool not found — that scan engine will be skipped (run: fgctl doctor for details)"
  fi
done

echo ""
echo "────────────────────────────────────────"

if [ "$ERRORS" -gt 0 ]; then
  echo -e "  ${RED}Bootstrap failed — fix the $ERRORS error(s) above then re-run.${RESET}"
  exit 1
fi

# ── Build Go binaries ────────────────────────────────────────────────────
echo ""
echo "  Building ForgeGuardian binaries..."
if go build ./... 2>&1 | sed 's/^/    /'; then
  PASS "go build ./... succeeded"
else
  FAIL "go build ./... failed — check errors above"
  exit 1
fi

# ── Install dashboard deps ────────────────────────────────────────────────
if command -v npm &>/dev/null && [ -f dashboard/package.json ]; then
  echo ""
  echo "  Installing dashboard dependencies..."
  (cd dashboard && npm ci --silent 2>&1 | tail -3) && PASS "dashboard npm ci succeeded"
fi

echo ""
echo "────────────────────────────────────────"
echo -e "  ${GREEN}Bootstrap complete!${RESET}"
echo ""
echo "  Next steps:"
echo "    make up            Start postgres + redis + API"
echo "    ./bin/fgctl doctor Check environment"
echo "    ./bin/fgctl scan . Scan your project"
echo "    make dev           Start full dev stack"
echo ""
