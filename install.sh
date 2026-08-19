#!/usr/bin/env bash
# ForgeGuardian installer
# Usage: curl -sSfL https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.sh | bash
#
# Environment variables:
#   FORGEGUARDIAN_VERSION       — version to install (default: latest)
#   FORGEGUARDIAN_INSTALL_MODE  — install mode: auto|local|offline|minimal|full|dev (default: auto)
#   FORGEGUARDIAN_LOCAL_BUNDLE  — path to a local tarball to extract binaries from
#   INSTALL_DIR                 — install directory (default: ~/.local/bin)
#   BINARIES                    — space-separated list (default: "fgctl fg-agent intel-agent")

set -euo pipefail

# ─── colors ───────────────────────────────────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; BOLD='\033[1m'; NC='\033[0m'
info()  { echo -e "${GREEN}✓${NC}  $*"; }
warn()  { echo -e "${YELLOW}⚠${NC}  $*"; }
error() { echo -e "${RED}✗${NC}  $*" >&2; }
die() {
  error "$1"
  echo    "  → Try: FORGEGUARDIAN_INSTALL_MODE=local bash install.sh"
  echo    "  → Or:  go install github.com/${GO_MODULE}/cmd/fgctl@latest"
  exit 1
}

# ─── config ───────────────────────────────────────────────────────────────────
INSTALL_MODE="${FORGEGUARDIAN_INSTALL_MODE:-auto}"
LOCAL_BUNDLE="${FORGEGUARDIAN_LOCAL_BUNDLE:-}"
VERSION="${FORGEGUARDIAN_VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"
BINARIES="${BINARIES:-fgctl fg-agent intel-agent}"
REPO="Mah3Sec/ForgeGuardian"
GO_MODULE="mah3sec/forgeguardian"
CURL_OPTS="--retry 3 --retry-delay 2 --max-time 60 --connect-timeout 10"

# ─── platform detection ───────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
  linux|darwin) ;;
  msys*|cygwin*|mingw*) OS="windows" ;;
  *) die "Unsupported OS: $OS (unsupported platform)" ;;
esac

case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) die "Unsupported architecture: $ARCH (unsupported platform)" ;;
esac

EXT="tar.gz"
[ "$OS" = "windows" ] && EXT="zip"

# ─── helpers ──────────────────────────────────────────────────────────────────

# ensure the install directory exists and is on PATH
ensure_install_dir() {
  mkdir -p "$INSTALL_DIR"
  case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
      warn "${INSTALL_DIR} is not in PATH."
      warn "Add the following line to your shell rc file:"
      warn "  export PATH=\"\$PATH:${INSTALL_DIR}\""
      ;;
  esac
}

# copy (or sudo-copy) a binary into INSTALL_DIR
place_binary() {
  local src="$1"
  local name="$2"
  local dst="${INSTALL_DIR}/${name}"
  if [ -w "$INSTALL_DIR" ]; then
    install -m 755 "$src" "$dst"
  else
    sudo install -m 755 "$src" "$dst"
  fi
  info "Installed ${name} → ${dst}"
}

# ─── github reachability probe ────────────────────────────────────────────────
probe_github() {
  curl -sf --max-time 5 https://api.github.com > /dev/null 2>&1
}

# ─── install strategies ───────────────────────────────────────────────────────

# Strategy 1: extract binaries from a local tarball (FORGEGUARDIAN_LOCAL_BUNDLE)
install_from_bundle() {
  [ -f "$LOCAL_BUNDLE" ] || die \
    "LOCAL_BUNDLE file not found: ${LOCAL_BUNDLE} (file does not exist)"

  local tmpdir
  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  info "Extracting binaries from local bundle: ${LOCAL_BUNDLE}"
  tar xzf "$LOCAL_BUNDLE" -C "$tmpdir" \
    || die "Failed to extract bundle: ${LOCAL_BUNDLE} (tar returned non-zero)"

  ensure_install_dir

  local installed=0
  for binary in $BINARIES; do
    local src="${tmpdir}/${binary}"
    [ "$OS" = "windows" ] && src="${src}.exe"
    if [ -f "$src" ]; then
      place_binary "$src" "$binary"
      installed=$((installed + 1))
    fi
  done

  [ "$installed" -gt 0 ] \
    || die "No binaries found in bundle (check that the tarball contains: ${BINARIES})"
}

# Strategy 2: download a release binary from GitHub
install_from_github() {
  # resolve version
  if [ "$VERSION" = "latest" ]; then
    VERSION=$(curl -sSf $CURL_OPTS \
      "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' | head -1 | cut -d'"' -f4) \
      || die "Could not resolve latest version from GitHub API (network error or rate limit)"
    [ -n "$VERSION" ] \
      || die "GitHub API returned an empty tag_name (no releases published yet)"
  fi

  local version_bare="${VERSION#v}"
  local filename="forgeguardian_${version_bare}_${OS}_${ARCH}.${EXT}"
  local base_url="https://github.com/${REPO}/releases/download/${VERSION}"
  local tmpdir
  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  info "Downloading ForgeGuardian ${VERSION} (${OS}/${ARCH})..."
  curl -sSfL $CURL_OPTS "${base_url}/${filename}" -o "${tmpdir}/${filename}" \
    || die "Download failed for ${filename} (check version and network)"

  # checksum verification (best-effort — skip if file absent)
  curl -sSfL $CURL_OPTS \
    "${base_url}/checksums.txt" \
    -o "${tmpdir}/checksums.txt" 2>/dev/null || true

  if [ -f "${tmpdir}/checksums.txt" ]; then
    (
      cd "$tmpdir"
      if command -v sha256sum >/dev/null 2>&1; then
        grep "$filename" checksums.txt | sha256sum --check --status \
          || die "Checksum verification FAILED for ${filename} (archive may be corrupt)"
        info "Checksum verified (sha256sum)"
      elif command -v shasum >/dev/null 2>&1; then
        grep "$filename" checksums.txt | shasum -a 256 --check --status \
          || die "Checksum verification FAILED for ${filename} (archive may be corrupt)"
        info "Checksum verified (shasum)"
      else
        warn "No sha256sum or shasum found — skipping checksum verification"
      fi
    )
  else
    warn "Checksums file not found — skipping verification"
  fi

  # extract archive
  (
    cd "$tmpdir"
    if [ "$EXT" = "zip" ]; then
      unzip -q "$filename" \
        || die "Failed to unzip ${filename} (corrupt archive?)"
    else
      tar -xzf "$filename" \
        || die "Failed to extract ${filename} (corrupt archive?)"
    fi
  )

  ensure_install_dir

  local installed=0
  for binary in $BINARIES; do
    local src="${tmpdir}/${binary}"
    [ "$OS" = "windows" ] && src="${src}.exe"
    if [ -f "$src" ]; then
      place_binary "$src" "$binary"
      installed=$((installed + 1))
    fi
  done

  [ "$installed" -gt 0 ] \
    || die "No binaries found in the release archive (check release assets for ${VERSION})"
}

# Strategy 3: build fgctl from source
# Prefers the current git checkout (go.mod present in cwd or a parent dir) —
# this is the actual "local" case: a contributor who cloned the repo and is
# running ./install.sh from inside it, before it's ever pushed anywhere, or
# testing an uncommitted change. Falls back to `go install <remote>@latest`
# (a real network fetch of the published module) only when not run from
# inside a checkout — e.g. piped via curl with no local clone at all.
install_from_source() {
  command -v go >/dev/null 2>&1 \
    || die "go not found in PATH (install Go 1.23+ from https://go.dev/dl/)"

  local go_version
  go_version=$(go version | awk '{print $3}' | sed 's/go//')

  local checkout_root=""
  local dir="$PWD"
  while [ "$dir" != "/" ]; do
    if [ -f "${dir}/go.mod" ] && grep -qi "^module github.com/${GO_MODULE}$" "${dir}/go.mod" 2>/dev/null; then
      checkout_root="$dir"
      break
    fi
    dir="$(dirname "$dir")"
  done

  ensure_install_dir

  if [ -n "$checkout_root" ]; then
    info "Building fgctl from local checkout at ${checkout_root} (Go ${go_version})..."
    ( cd "$checkout_root" && go build -o "${INSTALL_DIR}/fgctl" ./cmd/fgctl ) \
      || die "go build ./cmd/fgctl failed in ${checkout_root}"
    info "Built and installed fgctl → ${INSTALL_DIR}/fgctl"
    return
  fi

  info "No local checkout found — fetching from the published module (Go ${go_version})..."
  local install_pkg="github.com/${GO_MODULE}/cmd/fgctl"
  local target_version="${VERSION}"
  [ "$target_version" = "latest" ] && target_version="latest"

  GOBIN="$INSTALL_DIR" go install "${install_pkg}@${target_version}" \
    || die "go install ${install_pkg}@${target_version} failed (check GOPATH and network, or run this script from inside a git clone of the repo for a local build)"

  info "Built and installed fgctl → ${INSTALL_DIR}/fgctl"
}

# Strategy 4: use pre-built binaries from ./bin/ (air-gap / offline)
install_from_local_bin() {
  local bin_dir="./bin"
  [ -d "$bin_dir" ] \
    || die "Offline mode selected but ./bin/ directory not found (place pre-built binaries there)"

  ensure_install_dir

  local installed=0
  for binary in $BINARIES; do
    local src="${bin_dir}/${binary}"
    [ "$OS" = "windows" ] && src="${src}.exe"
    if [ -f "$src" ]; then
      place_binary "$src" "$binary"
      installed=$((installed + 1))
    else
      warn "Binary not found in ./bin/: ${binary} — skipping"
    fi
  done

  [ "$installed" -gt 0 ] \
    || die "No binaries found in ./bin/ (expected: ${BINARIES})"
}

# ─── top-level fgctl installer ────────────────────────────────────────────────
install_fgctl() {
  # LOCAL_BUNDLE always takes priority regardless of mode
  if [ -n "$LOCAL_BUNDLE" ]; then
    info "Using local bundle: ${LOCAL_BUNDLE}"
    install_from_bundle
    return
  fi

  case "$INSTALL_MODE" in
    local)
      install_from_source
      ;;
    offline)
      install_from_local_bin
      ;;
    *)
      # auto / minimal / full / dev — try GitHub first, fall back to source
      if probe_github; then
        install_from_github || {
          warn "GitHub download failed — falling back to source build"
          install_from_source
        }
      else
        warn "GitHub unreachable — switching to local build mode"
        INSTALL_MODE="local"
        install_from_source
      fi
      ;;
  esac
}

# ─── optional steps ───────────────────────────────────────────────────────────

install_dashboard_deps() {
  local dash_dir
  # support both running from repo root or an arbitrary cwd
  if [ -f "dashboard/package.json" ]; then
    dash_dir="dashboard"
  elif [ -f "package.json" ]; then
    dash_dir="."
  else
    warn "dashboard/package.json not found — skipping npm install"
    return
  fi

  if ! command -v npm >/dev/null 2>&1; then
    warn "npm not found — skipping dashboard dependency install"
    return
  fi

  info "Installing dashboard npm dependencies (${dash_dir})..."
  if [ "$INSTALL_MODE" = "dev" ]; then
    npm ci --prefix "$dash_dir" \
      || die "npm ci failed in ${dash_dir} (check package-lock.json)"
  else
    npm install --prefix "$dash_dir" \
      || die "npm install failed in ${dash_dir} (check package.json)"
  fi
  info "Dashboard dependencies installed"
}

start_docker_compose() {
  if ! command -v docker >/dev/null 2>&1; then
    warn "docker not found — skipping docker compose"
    return
  fi

  # prefer compose v2 plugin, fall back to standalone docker-compose
  local compose_cmd=""
  if docker compose version >/dev/null 2>&1; then
    compose_cmd="docker compose"
  elif command -v docker-compose >/dev/null 2>&1; then
    compose_cmd="docker-compose"
  else
    warn "Neither 'docker compose' nor 'docker-compose' found — skipping"
    return
  fi

  local compose_file="docker-compose.yml"
  [ -f "$compose_file" ] \
    || { warn "${compose_file} not found — skipping docker compose"; return; }

  if [ "$INSTALL_MODE" = "dev" ]; then
    info "Starting services (docker compose up --build --detach)..."
    $compose_cmd up --build --detach \
      || die "docker compose up --build failed (check Docker daemon)"
  else
    info "Starting services (docker compose up --detach)..."
    $compose_cmd up --detach \
      || die "docker compose up failed (check Docker daemon)"
  fi
  info "Services started. Run 'docker compose logs -f' to tail logs."
}

install_signatures() {
  if command -v fgctl >/dev/null 2>&1 \
       || [ -x "${INSTALL_DIR}/fgctl" ]; then
    local fgctl_bin="fgctl"
    [ -x "${INSTALL_DIR}/fgctl" ] && fgctl_bin="${INSTALL_DIR}/fgctl"
    info "Downloading community signatures..."
    "$fgctl_bin" update 2>/dev/null \
      || warn "fgctl update failed — run manually after install"
  else
    warn "fgctl not in PATH yet — skipping signature update (run 'fgctl update' later)"
  fi
}

# ─── main ─────────────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}ForgeGuardian Installer${NC}"
echo    "────────────────────────────────────────"
echo    "  Mode    : ${INSTALL_MODE}"
echo    "  Target  : ${INSTALL_DIR}"
echo    "  Binaries: ${BINARIES}"
[ -n "$LOCAL_BUNDLE" ] && echo "  Bundle  : ${LOCAL_BUNDLE}"
echo    ""

# Step 1: always install fgctl (and companion binaries)
install_fgctl

# Step 2: mode-specific extras
case "$INSTALL_MODE" in
  minimal)
    # fgctl only — nothing else
    ;;

  full)
    install_dashboard_deps
    install_signatures
    start_docker_compose
    ;;

  dev)
    install_dashboard_deps   # uses npm ci
    install_signatures
    start_docker_compose     # uses --build
    ;;

  auto|local|offline)
    # npm install if dashboard present, skip docker
    install_dashboard_deps
    ;;
esac

# ─── done ─────────────────────────────────────────────────────────────────────
echo ""
info "Installation complete."
echo ""
echo    "  Next steps:"
echo    "    fgctl doctor          — check your environment"
echo    "    fgctl update          — download community signatures"
echo    "    fgctl scan .          — scan your current project"
echo ""
echo    "  Install mode reference:"
echo    "    FORGEGUARDIAN_INSTALL_MODE=minimal   — fgctl only"
echo    "    FORGEGUARDIAN_INSTALL_MODE=local     — build from source"
echo    "    FORGEGUARDIAN_INSTALL_MODE=offline   — use ./bin/ binaries"
echo    "    FORGEGUARDIAN_INSTALL_MODE=full      — fgctl + npm + docker compose up"
echo    "    FORGEGUARDIAN_INSTALL_MODE=dev       — full + npm ci + docker compose up --build"
echo    "    FORGEGUARDIAN_LOCAL_BUNDLE=path.tar.gz — extract binaries from tarball"
echo ""
