#!/usr/bin/env bash
# ForgeGuardian — One-line installer
#
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.sh | bash
#
# What it does:
#   1. Downloads fgctl + companion binaries
#   2. Installs the pre-built dashboard
#   3. Downloads community threat signatures
#   4. Ready to run: fgctl serve
#
# Environment variables (all optional):
#   FORGEGUARDIAN_VERSION  — version to install (default: latest)
#   INSTALL_DIR            — binary install directory (default: ~/.local/bin)

set -euo pipefail

# ─── colors ───────────────────────────────────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; BOLD='\033[1m'
DIM='\033[2m'; NC='\033[0m'
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
echo -e "${BOLD}  ForgeGuardian Installer${NC}"
echo -e "  ${DIM}──────────────────────────────────────${NC}"
echo ""

# ─── step 1: resolve version ────────────────────────────────────────────────
if [ "$VERSION" = "latest" ]; then
  VERSION=$($CURL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d'"' -f4) \
    || die "Could not resolve latest version (check network)"
  [ -n "$VERSION" ] || die "No releases found"
fi
VERSION_BARE="${VERSION#v}"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
info "Version: ${VERSION}"

# ─── step 2: download + install binaries ─────────────────────────────────────
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

ARCHIVE="forgeguardian_${VERSION_BARE}_${OS}_${ARCH}.${EXT}"
info "Downloading ${ARCHIVE}..."
$CURL "${BASE_URL}/${ARCHIVE}" -o "${TMPDIR}/${ARCHIVE}" \
  || die "Download failed — check version and network"

# checksum verification
if $CURL "${BASE_URL}/checksums.txt" -o "${TMPDIR}/checksums.txt" 2>/dev/null; then
  (
    cd "$TMPDIR"
    if command -v sha256sum >/dev/null 2>&1; then
      grep "$ARCHIVE" checksums.txt | sha256sum --check --status && info "Checksum verified"
    elif command -v shasum >/dev/null 2>&1; then
      grep "$ARCHIVE" checksums.txt | shasum -a 256 --check --status && info "Checksum verified"
    fi
  ) || true
fi

# extract
(
  cd "$TMPDIR"
  if [ "$EXT" = "zip" ]; then
    unzip -q "$ARCHIVE"
  else
    tar -xzf "$ARCHIVE"
  fi
)

# install binaries
mkdir -p "$INSTALL_DIR"
for bin in fgctl fg-agent intel-agent; do
  src="${TMPDIR}/${bin}"
  [ "$OS" = "windows" ] && src="${src}.exe"
  if [ -f "$src" ]; then
    if [ -w "$INSTALL_DIR" ]; then
      install -m 755 "$src" "${INSTALL_DIR}/${bin}"
    else
      sudo install -m 755 "$src" "${INSTALL_DIR}/${bin}"
    fi
  fi
done
info "Binaries installed → ${INSTALL_DIR}/"

# ─── step 3: download + install dashboard ────────────────────────────────────
DASH_DIR="${DATA_DIR}/dashboard"
info "Downloading dashboard..."
if $CURL "${BASE_URL}/dashboard-dist.tar.gz" -o "${TMPDIR}/dashboard-dist.tar.gz" 2>/dev/null; then
  rm -rf "$DASH_DIR"
  mkdir -p "$DASH_DIR"
  tar -xzf "${TMPDIR}/dashboard-dist.tar.gz" -C "$DASH_DIR"
  info "Dashboard installed → ${DASH_DIR}/"
else
  warn "Dashboard not available in this release — fgctl serve will run API-only"
fi

# ─── step 4: PATH check ─────────────────────────────────────────────────────
FGCTL="${INSTALL_DIR}/fgctl"
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    warn "${INSTALL_DIR} is not in your PATH"
    warn "Add to your shell rc:  export PATH=\"\$PATH:${INSTALL_DIR}\""
    export PATH="${PATH}:${INSTALL_DIR}"
    ;;
esac

# ─── step 5: download threat signatures ──────────────────────────────────────
if [ -x "$FGCTL" ]; then
  info "Downloading community threat signatures..."
  "$FGCTL" update 2>/dev/null && info "Signatures updated" \
    || warn "Signature download failed — run 'fgctl update' manually"
fi

# ─── done ────────────────────────────────────────────────────────────────────
echo ""
echo -e "  ${GREEN}${BOLD}Installation complete!${NC}"
echo ""
echo -e "  ${BOLD}Start ForgeGuardian:${NC}"
echo -e "    ${GREEN}fgctl serve${NC}"
echo -e "    ${DIM}→ Opens API + dashboard on http://localhost:8080${NC}"
echo ""
echo -e "  ${BOLD}Or scan right away:${NC}"
echo -e "    ${GREEN}fgctl scan .${NC}                        ${DIM}# scan current project${NC}"
echo -e "    ${GREEN}fgctl scan npm/lodash@4.17.21${NC}       ${DIM}# scan a package${NC}"
echo -e "    ${GREEN}fgctl audit system${NC}                  ${DIM}# audit all installed packages${NC}"
echo ""
