#!/usr/bin/env bash
# ForgeGuardian — Automated Test Runner
# Run this inside the fg-testenv container:
#   docker compose -f docker-compose.testenv.yml exec forgeguardian bash /run-tests.sh
#
# Or manually on your host (with Go + tools installed):
#   cd <repo-root> && bash testenv/run-tests.sh

set -euo pipefail

FGCTL="${FGCTL:-./fgctl}"
AGENT="${AGENT:-./fg-agent}"
RESULTS_DIR="${RESULTS_DIR:-/tmp/fg-test-results}"
PASS=0
FAIL=0
SKIP=0

GREEN='\033[0;32m'
RED='\033[0;31m'
AMBER='\033[0;33m'
CYAN='\033[0;36m'
RESET='\033[0m'

mkdir -p "$RESULTS_DIR"

log()  { echo -e "${CYAN}[fg-test]${RESET} $*"; }
pass() { echo -e "${GREEN}[PASS]${RESET} $1"; PASS=$((PASS+1)); }
fail() { echo -e "${RED}[FAIL]${RESET} $1"; FAIL=$((FAIL+1)); }
skip() { echo -e "${AMBER}[SKIP]${RESET} $1 (reason: $2)"; SKIP=$((SKIP+1)); }

# ─── Helper: assert exit code ────────────────────────────────────────
assert_exit() {
  local desc="$1" expected="$2"; shift 2
  local actual=0
  "$@" > /dev/null 2>&1 || actual=$?
  if [ "$actual" -eq "$expected" ]; then
    pass "$desc (exit $actual)"
  else
    fail "$desc — expected exit $expected, got $actual"
  fi
}

# ─── Helper: assert JSON field ───────────────────────────────────────
assert_json_field() {
  local desc="$1" file="$2" field="$3" expected="$4"
  local actual
  actual=$(jq -r "$field" "$file" 2>/dev/null || echo "PARSE_ERROR")
  if [ "$actual" = "$expected" ]; then
    pass "$desc ($field = $actual)"
  else
    fail "$desc — expected $field=$expected, got $actual"
  fi
}

# ═══════════════════════════════════════════════════════════════════════
echo ""
echo "════════════════════════════════════════════════════════════"
echo "  ForgeGuardian Test Suite"
echo "════════════════════════════════════════════════════════════"
echo ""

# ─── GROUP 1: Binary check ───────────────────────────────────────────
log "Group 1: Binary availability"
for bin in "$FGCTL" "$AGENT"; do
  if [ -x "$bin" ]; then
    pass "binary exists: $bin"
  else
    fail "binary missing: $bin — run: go build -o $bin"
  fi
done

# ─── GROUP 2: Clean package baseline (expect 0 findings) ─────────────
log "Group 2: Clean package — chalk@5.3.0"
CHALK_SCAN="$RESULTS_DIR/chalk-scan.json"
if $FGCTL scan --recipe=npm --package=chalk --version=5.3.0 --json > "$CHALK_SCAN" 2>/dev/null; then
  CRIT=$(jq -r '(.summary.Critical // .summary.critical // 0)' "$CHALK_SCAN")
  if [ "$CRIT" -eq 0 ]; then
    pass "chalk@5.3.0: 0 critical findings (expected)"
  else
    fail "chalk@5.3.0: unexpected $CRIT critical findings"
  fi
else
  fail "chalk scan exited non-zero"
fi

# ─── GROUP 3: Known CVE packages (npm) ───────────────────────────────
log "Group 3: Known CVE packages"

run_vuln_scan() {
  local recipe="$1" pkg="$2" ver="$3" min_findings="$4"
  local outfile="$RESULTS_DIR/${pkg//\//-}-${ver}-scan.json"
  $FGCTL scan --recipe="$recipe" --package="$pkg" --version="$ver" --json > "$outfile" 2>/dev/null || true
  local total
  total=$(jq -r '(.summary.Total // .summary.total // 0)' "$outfile")
  if [ "$total" -ge "$min_findings" ]; then
    pass "$pkg@$ver: $total findings (expected >= $min_findings)"
  else
    fail "$pkg@$ver: only $total findings, expected >= $min_findings"
  fi
}

run_vuln_scan npm  lodash        4.17.20  1
run_vuln_scan npm  minimist      1.2.5    1
run_vuln_scan npm  axios         0.21.1   1
run_vuln_scan npm  qs            6.5.2    1
run_vuln_scan pypi Pillow        8.3.1    1
run_vuln_scan pypi PyYAML        5.3.1    1
run_vuln_scan pypi requests      2.25.0   1
run_vuln_scan maven "org.apache.logging.log4j:log4j-core" 2.14.1 1

# ─── GROUP 4: fail-on flag ────────────────────────────────────────────
log "Group 4: --fail-on flag (exit code 2 on findings)"
assert_exit "lodash@4.17.20 --fail-on=high exits 2" 2 \
  $FGCTL scan --recipe=npm --package=lodash --version=4.17.20 --fail-on=high

# ─── GROUP 5: SBOM generation ────────────────────────────────────────
log "Group 5: SBOM generation"
SBOM_OUT="$RESULTS_DIR/chalk-sbom.json"
if $FGCTL sbom --recipe=npm --package=chalk --version=5.3.0 \
     --format=cyclonedx-json --out="$SBOM_OUT" 2>/dev/null; then
  if jq -e '.bomFormat == "CycloneDX"' "$SBOM_OUT" > /dev/null 2>&1; then
    pass "SBOM: valid CycloneDX JSON produced"
  else
    fail "SBOM: bomFormat != CycloneDX"
  fi
else
  fail "SBOM: command failed"
fi

# ─── GROUP 6: Sign + Verify ───────────────────────────────────────────
log "Group 6: Sign + Verify"
ATT_OUT="$RESULTS_DIR/chalk-att.json"
if $FGCTL sign --recipe=npm --package=chalk --version=5.3.0 \
     --out="$ATT_OUT" 2>/dev/null; then
  pass "sign: attestation produced"
  SHA=$(jq -r '.sha256' "$ATT_OUT")
  if $FGCTL verify --attestation="$ATT_OUT" --sha256="$SHA" > /dev/null 2>&1; then
    pass "verify: attestation valid"
  else
    fail "verify: verification failed"
  fi
else
  fail "sign: command failed"
fi

# ─── GROUP 7: AI Advisory (requires ANTHROPIC_API_KEY) ───────────────
log "Group 7: AI Advisory"
if [ -z "${ANTHROPIC_API_KEY:-}" ]; then
  skip "AI advisory" "ANTHROPIC_API_KEY not set"
else
  ADV_OUT="$RESULTS_DIR/lodash-advisory.json"
  if $FGCTL advisory --recipe=npm --package=lodash --version=4.17.20 \
       --json > "$ADV_OUT" 2>/dev/null; then
    SEVERITY=$(jq -r '(.severity // .Severity // "")' "$ADV_OUT")
    if [ -n "$SEVERITY" ] && [ "$SEVERITY" != "null" ]; then
      pass "advisory: generated with severity=$SEVERITY"
    else
      fail "advisory: missing severity field"
    fi
  else
    fail "advisory: command failed"
  fi
fi

# ─── GROUP 8: Patch Agent dry-run ────────────────────────────────────
log "Group 8: Patch agent dry-run"
if [ -z "${ANTHROPIC_API_KEY:-}" ]; then
  skip "patch agent" "ANTHROPIC_API_KEY not set"
else
  # Copy vuln project to temp dir
  PROJ_TMP="$RESULTS_DIR/vuln-node-app"
  cp -r testenv/projects/vuln-node-app "$PROJ_TMP" 2>/dev/null || \
    cp -r /projects/vuln-node-app "$PROJ_TMP" 2>/dev/null || true

  if $AGENT --recipe=npm --package=lodash --version=4.17.20 \
       --project-dir="$PROJ_TMP" > "$RESULTS_DIR/agent-dryrun.txt" 2>&1; then
    pass "patch agent dry-run: completed without error"
  else
    fail "patch agent dry-run: exited non-zero"
  fi
fi

# ═══════════════════════════════════════════════════════════════════════
echo ""
echo "════════════════════════════════════════════════════════════"
printf "  Results: ${GREEN}%d PASS${RESET}  ${RED}%d FAIL${RESET}  ${AMBER}%d SKIP${RESET}\n" $PASS $FAIL $SKIP
echo "  Full output: $RESULTS_DIR"
echo "════════════════════════════════════════════════════════════"
echo ""

[ "$FAIL" -eq 0 ]
