#!/usr/bin/env bash
# ForgeGuardian — Complete installer
#
# One command installs everything:
#   curl -sSfL https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.sh | bash
#
# What it does:
#   1. Installs fgctl + companion binaries
#   2. Installs the pre-built web dashboard
#   3. Installs scanner engines (grype, trivy, semgrep)
#   4. Downloads community threat signatures
#   5. Creates default configuration
#   6. Ready to run: fgctl serve
#
# Environment variables (all optional):
#   FORGEGUARDIAN_VERSION  — version to install (default: latest)
#   INSTALL_DIR            — binary install directory (default: ~/.local/bin)
#   SKIP_ENGINES           — set to 1 to skip scanner engine installation

set -euo pipefail

# ─── colors ───────────────────────────────────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; BOLD='\033[1m'
DIM='\033[2m'; CYAN='\033[0;36m'; NC='\033[0m'
step()  { echo -e "\n  ${CYAN}${BOLD}[$1/6]${NC} ${BOLD}$2${NC}"; }
info()  { echo -e "  ${GREEN}✓${NC}  $*"; }
warn()  { echo -e "  ${YELLOW}!${NC}  $*"; }
err()   { echo -e "  ${RED}✗${NC}  $*" >&2; }
die()   { err "$1"; exit 1; }

# ─── config ───────────────────────────────────────────────────────────────────
VERSION="${FORGEGUARDIAN_VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"
DATA_DIR="${HOME}/.forgeguardian"
REPO="Mah3Sec/ForgeGuardian"
CURL="curl --retry 3 --retry-delay 2 --max-time 120 --connect-timeout 10 -sSfL"
SKIP_ENGINES="${SKIP_ENGINES:-0}"

# ─── platform ────────────────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  linux|darwin) ;;
  msys*|cygwin*|mingw*) OS="windows" ;;
  *) die "Unsupported OS: $OS" ;;
esac

case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) die "Unsupported architecture: $ARCH" ;;
esac

EXT="tar.gz"
[ "$OS" = "windows" ] && EXT="zip"

# ─── banner ──────────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}  ╔══════════════════════════════════════════╗${NC}"
echo -e "${BOLD}  ║    ForgeGuardian — Full Installer        ║${NC}"
echo -e "${BOLD}  ╚══════════════════════════════════════════╝${NC}"
echo -e "  ${DIM}Platform: ${OS}/${ARCH}${NC}"

# ─── helpers ─────────────────────────────────────────────────────────────────
ensure_dir() {
  mkdir -p "$1"
}

place_binary() {
  local src="$1" dst="$2"
  if [ -w "$(dirname "$dst")" ]; then
    install -m 755 "$src" "$dst"
  else
    sudo install -m 755 "$src" "$dst"
  fi
}

has_cmd() {
  command -v "$1" >/dev/null 2>&1
}

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 1: Resolve version + download ForgeGuardian binaries
# ═══════════════════════════════════════════════════════════════════════════════
step 1 "Installing ForgeGuardian"

if [ "$VERSION" = "latest" ]; then
  VERSION=$($CURL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d'"' -f4) \
    || die "Could not resolve latest version (check network)"
  [ -n "$VERSION" ] || die "No releases found"
fi
VERSION_BARE="${VERSION#v}"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
info "Version: ${VERSION}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

ARCHIVE="forgeguardian_${VERSION_BARE}_${OS}_${ARCH}.${EXT}"
info "Downloading ${ARCHIVE}..."
$CURL "${BASE_URL}/${ARCHIVE}" -o "${TMPDIR}/${ARCHIVE}" \
  || die "Download failed — check version and network"

# checksum
if $CURL "${BASE_URL}/checksums.txt" -o "${TMPDIR}/checksums.txt" 2>/dev/null; then
  (
    cd "$TMPDIR"
    if has_cmd sha256sum; then
      grep "$ARCHIVE" checksums.txt | sha256sum --check --status 2>/dev/null && info "Checksum verified"
    elif has_cmd shasum; then
      grep "$ARCHIVE" checksums.txt | shasum -a 256 --check --status 2>/dev/null && info "Checksum verified"
    fi
  ) || true
fi

# extract
( cd "$TMPDIR"; if [ "$EXT" = "zip" ]; then unzip -qo "$ARCHIVE"; else tar -xzf "$ARCHIVE"; fi )

# install binaries
ensure_dir "$INSTALL_DIR"
for bin in fgctl fg-agent intel-agent; do
  src="${TMPDIR}/${bin}"
  [ "$OS" = "windows" ] && src="${src}.exe"
  [ -f "$src" ] && place_binary "$src" "${INSTALL_DIR}/${bin}"
done
info "Installed fgctl → ${INSTALL_DIR}/"

# ensure PATH
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    export PATH="${PATH}:${INSTALL_DIR}"
    warn "${INSTALL_DIR} is not in your PATH"
    echo -e "  ${DIM}  Add to your shell rc:  export PATH=\"\$PATH:${INSTALL_DIR}\"${NC}"
    ;;
esac

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 2: Install dashboard
# ═══════════════════════════════════════════════════════════════════════════════
step 2 "Installing dashboard"

DASH_DIR="${DATA_DIR}/dashboard"
if $CURL "${BASE_URL}/dashboard-dist.tar.gz" -o "${TMPDIR}/dashboard-dist.tar.gz" 2>/dev/null; then
  rm -rf "$DASH_DIR"
  ensure_dir "$DASH_DIR"
  tar -xzf "${TMPDIR}/dashboard-dist.tar.gz" -C "$DASH_DIR"
  info "Dashboard → ${DASH_DIR}/"
else
  warn "Dashboard not found in release — will be available in next tagged release"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 3: Install scanner engines (grype, trivy, semgrep)
# ═══════════════════════════════════════════════════════════════════════════════
step 3 "Installing scanner engines"

if [ "$SKIP_ENGINES" = "1" ]; then
  warn "Skipping scanner engines (SKIP_ENGINES=1)"
else
  ENGINE_DIR="$INSTALL_DIR"
  INSTALLED_ENGINES=0

  # ── grype ────────────────────────────────────────────────────────────────
  if has_cmd grype; then
    info "grype: already installed ($(grype --version 2>/dev/null | head -1))"
    INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1))
  else
    echo -e "  ${DIM}  Installing grype (vulnerability scanner)...${NC}"
    if $CURL https://raw.githubusercontent.com/anchore/grype/main/install.sh \
        | sh -s -- -b "$ENGINE_DIR" 2>/dev/null; then
      info "grype: installed → ${ENGINE_DIR}/grype"
      INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1))
    elif has_cmd brew && brew install anchore/grype/grype >/dev/null 2>&1; then
      info "grype: installed via Homebrew"
      INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1))
    else
      warn "grype: install failed — install manually: https://github.com/anchore/grype"
    fi
  fi

  # ── trivy ────────────────────────────────────────────────────────────────
  if has_cmd trivy; then
    info "trivy: already installed ($(trivy --version 2>/dev/null | head -1))"
    INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1))
  else
    echo -e "  ${DIM}  Installing trivy (vulnerability scanner)...${NC}"
    if $CURL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh \
        | sh -s -- -b "$ENGINE_DIR" 2>/dev/null; then
      info "trivy: installed → ${ENGINE_DIR}/trivy"
      INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1))
    elif has_cmd brew && brew install trivy >/dev/null 2>&1; then
      info "trivy: installed via Homebrew"
      INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1))
    else
      warn "trivy: install failed — install manually: https://github.com/aquasecurity/trivy"
    fi
  fi

  # ── semgrep ──────────────────────────────────────────────────────────────
  if has_cmd semgrep; then
    info "semgrep: already installed ($(semgrep --version 2>/dev/null))"
    INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1))
  else
    echo -e "  ${DIM}  Installing semgrep (static analysis)...${NC}"
    if has_cmd pip3 && pip3 install semgrep --quiet --break-system-packages 2>/dev/null; then
      info "semgrep: installed via pip3"
      INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1))
    elif has_cmd pip && pip install semgrep --quiet --break-system-packages 2>/dev/null; then
      info "semgrep: installed via pip"
      INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1))
    elif has_cmd brew && brew install semgrep >/dev/null 2>&1; then
      info "semgrep: installed via Homebrew"
      INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1))
    elif has_cmd pipx && pipx install semgrep >/dev/null 2>&1; then
      info "semgrep: installed via pipx"
      INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1))
    else
      warn "semgrep: install failed — install manually: pip3 install semgrep"
    fi
  fi

  info "${INSTALLED_ENGINES}/3 scanner engines ready"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 4: Download threat signatures
# ═══════════════════════════════════════════════════════════════════════════════
step 4 "Downloading threat signatures"

FGCTL="${INSTALL_DIR}/fgctl"
if [ -x "$FGCTL" ]; then
  "$FGCTL" update 2>/dev/null && info "Community signatures downloaded" \
    || warn "Signature download failed — run 'fgctl update' later"
else
  warn "fgctl not found — run 'fgctl update' after fixing PATH"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 5: Create default configuration
# ═══════════════════════════════════════════════════════════════════════════════
step 5 "Setting up configuration"

ensure_dir "$DATA_DIR"

# config.yaml
CFG_FILE="${DATA_DIR}/config.yaml"
if [ ! -f "$CFG_FILE" ]; then
  cat > "$CFG_FILE" <<'YAML'
default_ecosystem: auto
output_format: text
verbose: false
engines:
  osv: true
  grype: true
  trivy: true
  semgrep: true
  behavioral: true
  malware_pattern: true
  typosquat: true
  ai_model: true
  mcp_injection: true
YAML
  chmod 600 "$CFG_FILE"
  info "Config created → ${CFG_FILE}"
else
  info "Config exists → ${CFG_FILE}"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 6: Verify installation
# ═══════════════════════════════════════════════════════════════════════════════
step 6 "Verifying installation"

echo ""
# Quick check of key components
CHECK_PASS=0; CHECK_TOTAL=0

for item in fgctl grype trivy semgrep; do
  CHECK_TOTAL=$((CHECK_TOTAL + 1))
  if has_cmd "$item"; then
    info "${item}: $(command -v "$item")"
    CHECK_PASS=$((CHECK_PASS + 1))
  else
    warn "${item}: not found"
  fi
done

# Check dashboard
CHECK_TOTAL=$((CHECK_TOTAL + 1))
if [ -f "${DASH_DIR}/index.html" ] 2>/dev/null; then
  info "dashboard: installed"
  CHECK_PASS=$((CHECK_PASS + 1))
else
  warn "dashboard: not installed"
fi

# Check signatures
CHECK_TOTAL=$((CHECK_TOTAL + 1))
if [ -f "${DATA_DIR}/signatures.json" ]; then
  SIG_COUNT=$(grep -c '"id"' "${DATA_DIR}/signatures.json" 2>/dev/null || echo "?")
  info "signatures: ${SIG_COUNT} loaded"
  CHECK_PASS=$((CHECK_PASS + 1))
else
  warn "signatures: not found"
fi

# ─── final summary ──────────────────────────────────────────────────────────
echo ""
echo -e "  ${BOLD}══════════════════════════════════════════${NC}"
if [ "$CHECK_PASS" -eq "$CHECK_TOTAL" ]; then
  echo -e "  ${GREEN}${BOLD}  Installation complete — all checks passed${NC}"
else
  echo -e "  ${GREEN}${BOLD}  Installation complete${NC} ${DIM}(${CHECK_PASS}/${CHECK_TOTAL} components)${NC}"
fi
echo -e "  ${BOLD}══════════════════════════════════════════${NC}"
echo ""
echo -e "  ${BOLD}Start the platform:${NC}"
echo -e "    ${GREEN}${BOLD}fgctl serve${NC}"
echo -e "    ${DIM}→ API + Dashboard at http://localhost:8080${NC}"
echo ""
echo -e "  ${BOLD}Scan a project:${NC}"
echo -e "    ${GREEN}fgctl scan .${NC}                        ${DIM}# scan current directory${NC}"
echo -e "    ${GREEN}fgctl scan npm/lodash@4.17.21${NC}       ${DIM}# scan a specific package${NC}"
echo -e "    ${GREEN}fgctl audit system${NC}                  ${DIM}# audit all installed packages${NC}"
echo ""
echo -e "  ${BOLD}Check setup:${NC}"
echo -e "    ${GREEN}fgctl doctor${NC}                        ${DIM}# verify environment${NC}"
echo ""

# PATH reminder at the very end if needed
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo -e "  ${YELLOW}${BOLD}Important:${NC} Add this to your shell profile, then restart your terminal:"
    echo -e "    ${BOLD}export PATH=\"\$PATH:${INSTALL_DIR}\"${NC}"
    echo ""
    ;;
esac
