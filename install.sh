#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════════
# ForgeGuardian — Universal Single-Script Installer
# ═══════════════════════════════════════════════════════════════════════════════
#
# ONE script. Detects your OS. Installs everything.
#
#   Linux / macOS:
#     curl -sSfL https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.sh | bash
#
#   Windows (PowerShell — no Git Bash needed):
#     irm https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.sh | iex
#
#   Windows (Git Bash):
#     curl -sSfL https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.sh | bash
#
# Supported: Ubuntu, Debian, Kali, Fedora, RHEL, CentOS, Arch, Alpine,
#            SUSE, macOS (Intel + Apple Silicon), Windows 10/11
#
# This file is a polyglot — it runs as both bash AND PowerShell:
#   - Bash: executes the bash section below, exits before the embedded PS block
#   - PowerShell: `irm | iex` skips the bash section (it errors silently) and
#     the trailing PS block self-extracts via the bootstrapper at the very end.

set -euo pipefail

# ─── colors ──────────────────────────────────────────────────────────────────
if [ -t 1 ] && [ "${TERM:-dumb}" != "dumb" ]; then
  GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; BOLD='\033[1m'
  DIM='\033[2m'; CYAN='\033[0;36m'; MAGENTA='\033[0;35m'; NC='\033[0m'
else
  GREEN=''; YELLOW=''; RED=''; BOLD=''; DIM=''; CYAN=''; MAGENTA=''; NC=''
fi

step()  { printf "\n  ${CYAN}${BOLD}[%s/8]${NC} ${BOLD}%s${NC}\n" "$1" "$2"; }
info()  { printf "  ${GREEN}+${NC}  %s\n" "$*"; }
warn()  { printf "  ${YELLOW}!${NC}  %s\n" "$*"; }
err()   { printf "  ${RED}x${NC}  %s\n" "$*" >&2; }
die()   { err "$1"; exit 1; }

# ─── config ──────────────────────────────────────────────────────────────────
VERSION="${FORGEGUARDIAN_VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"
DATA_DIR="${HOME}/.forgeguardian"
REPO="Mah3Sec/ForgeGuardian"
SKIP_ENGINES="${SKIP_ENGINES:-0}"
SKIP_SERVER="${SKIP_SERVER:-0}"

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 1: Detect platform
# ═══════════════════════════════════════════════════════════════════════════════
step 1 "Detecting platform"

RAW_OS=$(uname -s)
OS=$(printf '%s' "$RAW_OS" | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
IS_WINDOWS=false
IS_WSL=false

case "$OS" in
  linux)
    grep -qiE 'microsoft|wsl' /proc/version 2>/dev/null && IS_WSL=true
    ;;
  darwin)  ;;
  msys*|cygwin*|mingw*)
    OS="windows"
    IS_WINDOWS=true
    ;;
  *)
    die "Unsupported OS: $RAW_OS"
    ;;
esac

case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  armv7*|armhf)  ARCH="armv7" ;;
  i386|i686)     ARCH="386" ;;
  *) die "Unsupported architecture: $ARCH" ;;
esac

EXT="tar.gz"; BIN_SUFFIX=""
if $IS_WINDOWS; then EXT="zip"; BIN_SUFFIX=".exe"; fi

# distro name
DISTRO="unknown"; DISTRO_PRETTY="unknown"
if [ "$OS" = "darwin" ]; then
  DISTRO="macos"
  DISTRO_PRETTY="macOS $(sw_vers -productVersion 2>/dev/null || printf 'unknown')"
elif $IS_WINDOWS; then
  DISTRO="windows"
  DISTRO_PRETTY="Windows ($(uname -o 2>/dev/null || printf 'Git Bash'))"
elif [ -f /etc/os-release ]; then
  _FG_VER="$VERSION"
  . /etc/os-release
  DISTRO="${ID:-unknown}"
  DISTRO_PRETTY="${PRETTY_NAME:-${ID} ${VERSION_ID:-}}"
  VERSION="$_FG_VER"
  unset _FG_VER
fi

# package manager
PKG=""; SUDO_CMD=""
if [ "$(id -u)" -eq 0 ]; then SUDO_CMD=""
elif command -v sudo >/dev/null 2>&1; then SUDO_CMD="sudo"
elif command -v doas >/dev/null 2>&1; then SUDO_CMD="doas"
fi

if $IS_WINDOWS; then
  for pm in winget scoop choco; do command -v $pm >/dev/null 2>&1 && PKG=$pm && break; done
else
  for pm in apt-get dnf yum pacman apk zypper brew; do command -v $pm >/dev/null 2>&1 && PKG=${pm%%-*} && break; done
  [ "$PKG" = "apt" ] && PKG="apt"
fi

printf "\n"
printf "  ${BOLD}+==========================================+${NC}\n"
printf "  ${BOLD}|    ForgeGuardian - Universal Installer    |${NC}\n"
printf "  ${BOLD}+==========================================+${NC}\n"
printf "\n"
printf "  ${MAGENTA}${BOLD}Detected system:${NC}\n"
printf "    OS ............. %s\n" "$DISTRO_PRETTY"
printf "    Architecture ... %s\n" "$ARCH"
printf "    Pkg manager .... %s\n" "${PKG:-none}"
$IS_WSL && printf "    Environment .... WSL (Windows Subsystem for Linux)\n"
printf "\n"

# ─── helpers ─────────────────────────────────────────────────────────────────
has_cmd() { command -v "$1" >/dev/null 2>&1; }
ensure_dir() { mkdir -p "$1"; }

place_binary() {
  local src="$1" dst="$2"
  if [ -w "$(dirname "$dst")" ]; then
    if $IS_WINDOWS; then cp "$src" "$dst"; else install -m 755 "$src" "$dst"; fi
  elif [ -n "$SUDO_CMD" ]; then
    $SUDO_CMD install -m 755 "$src" "$dst"
  else
    cp "$src" "$dst" && chmod 755 "$dst" 2>/dev/null || true
  fi
}

pkg_install() {
  local p="$1"
  case "$PKG" in
    apt)     $SUDO_CMD apt-get install -y -qq "$p" >/dev/null 2>&1 ;;
    dnf)     $SUDO_CMD dnf install -y -q "$p" >/dev/null 2>&1 ;;
    yum)     $SUDO_CMD yum install -y -q "$p" >/dev/null 2>&1 ;;
    pacman)  $SUDO_CMD pacman -S --noconfirm --needed "$p" >/dev/null 2>&1 ;;
    apk)     $SUDO_CMD apk add --quiet "$p" >/dev/null 2>&1 ;;
    zypper)  $SUDO_CMD zypper install -y "$p" >/dev/null 2>&1 ;;
    brew)    brew install -q "$p" >/dev/null 2>&1 ;;
    scoop)   scoop install "$p" >/dev/null 2>&1 ;;
    choco)   choco install "$p" -y >/dev/null 2>&1 ;;
    winget)  winget install "$p" --accept-source-agreements --accept-package-agreements --silent >/dev/null 2>&1 ;;
    *)       return 1 ;;
  esac
}

fetch() {
  local url="$1" out="$2"
  if has_cmd curl; then
    curl --retry 3 --retry-delay 2 --max-time 120 --connect-timeout 10 -sSfL "$url" -o "$out"
  elif has_cmd wget; then
    wget -q --timeout=120 --tries=3 -O "$out" "$url"
  else
    die "Neither curl nor wget found"
  fi
}

fetch_stdout() {
  if has_cmd curl; then
    curl --retry 3 --retry-delay 2 --max-time 120 --connect-timeout 10 -sSfL "$1"
  elif has_cmd wget; then
    wget -q --timeout=120 --tries=3 -O- "$1"
  else
    die "Neither curl nor wget found"
  fi
}

pip_install() {
  local p="$1" pip_cmd=""
  if has_cmd pip3; then pip_cmd="pip3"; elif has_cmd pip; then pip_cmd="pip"; else return 1; fi
  $pip_cmd install "$p" --quiet --break-system-packages 2>/dev/null && return 0
  $pip_cmd install "$p" --quiet 2>/dev/null && return 0
  $pip_cmd install "$p" --quiet --user 2>/dev/null && return 0
  return 1
}

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 2: Prerequisites
# ═══════════════════════════════════════════════════════════════════════════════
step 2 "Installing prerequisites"

if $IS_WINDOWS; then
  for dep in curl tar; do
    has_cmd "$dep" && info "${dep}: found" || warn "${dep}: missing"
  done
else
  APT_UPDATED=false
  for dep in curl tar; do
    if ! has_cmd "$dep"; then
      warn "$dep not found — installing..."
      if [ "$PKG" = "apt" ] && ! $APT_UPDATED; then
        $SUDO_CMD apt-get update -qq >/dev/null 2>&1 || true; APT_UPDATED=true
      fi
      pkg_install "$dep" || die "Cannot install $dep — install manually"
      info "$dep installed"
    else
      info "${dep}: found"
    fi
  done
fi

if [ "$SKIP_ENGINES" != "1" ]; then
  if ! has_cmd python3 && ! has_cmd python; then
    warn "python3 not found — installing..."
    if $IS_WINDOWS; then
      case "$PKG" in
        winget)  winget install Python.Python.3.12 --accept-source-agreements --accept-package-agreements --silent >/dev/null 2>&1 && info "python3 installed" ;;
        scoop)   scoop install python >/dev/null 2>&1 && info "python3 installed" ;;
        choco)   choco install python3 -y >/dev/null 2>&1 && info "python3 installed" ;;
        *)       warn "Install python3 manually for semgrep" ;;
      esac
    else
      case "$PKG" in
        apt)     pkg_install python3-pip || pkg_install python3 ;;
        dnf|yum) pkg_install python3-pip || pkg_install python3 ;;
        pacman)  pkg_install python-pip ;;
        apk)     pkg_install python3 && pkg_install py3-pip ;;
        zypper)  pkg_install python3-pip || pkg_install python3 ;;
        brew)    pkg_install python3 ;;
        *)       warn "Install python3 manually for semgrep" ;;
      esac
    fi
  else
    info "python: found"
  fi

  if ! $IS_WINDOWS && has_cmd python3 && ! has_cmd pip3 && ! has_cmd pip; then
    case "$PKG" in
      apt)     pkg_install python3-pip ;;
      dnf|yum) pkg_install python3-pip ;;
      pacman)  pkg_install python-pip ;;
      apk)     pkg_install py3-pip ;;
      zypper)  pkg_install python3-pip ;;
      *)       true ;;
    esac
  fi
fi

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 3: Download + install binaries
# ═══════════════════════════════════════════════════════════════════════════════
step 3 "Installing ForgeGuardian"

if [ "$VERSION" = "latest" ]; then
  VERSION=$(fetch_stdout "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d'"' -f4) \
    || die "Could not resolve latest version"
  [ -n "$VERSION" ] || die "No releases found"
fi
VERSION_BARE="${VERSION#v}"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
info "Version: ${VERSION}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

ARCHIVE="forgeguardian_${VERSION_BARE}_${OS}_${ARCH}.${EXT}"
info "Downloading ${ARCHIVE}..."
fetch "${BASE_URL}/${ARCHIVE}" "${TMPDIR}/${ARCHIVE}" \
  || die "Download failed — check version and network"

if fetch "${BASE_URL}/checksums.txt" "${TMPDIR}/checksums.txt" 2>/dev/null; then
  (cd "$TMPDIR"
   if has_cmd sha256sum; then
     grep "$ARCHIVE" checksums.txt | sha256sum --check --status 2>/dev/null && info "Checksum verified"
   elif has_cmd shasum; then
     grep "$ARCHIVE" checksums.txt | shasum -a 256 --check --status 2>/dev/null && info "Checksum verified"
   fi) || true
fi

(cd "$TMPDIR"
 if [ "$EXT" = "zip" ]; then
   if has_cmd unzip; then unzip -qo "$ARCHIVE"
   elif has_cmd powershell.exe; then
     powershell.exe -NoProfile -Command "Expand-Archive -Path '$ARCHIVE' -DestinationPath '.' -Force" 2>/dev/null
   else pkg_install unzip && unzip -qo "$ARCHIVE"; fi
 else
   tar -xzf "$ARCHIVE"
 fi)

ensure_dir "$INSTALL_DIR"
INSTALLED_BINS=0
for bin in fgctl fg-agent intel-agent; do
  src="${TMPDIR}/${bin}${BIN_SUFFIX}"
  if [ -f "$src" ]; then
    place_binary "$src" "${INSTALL_DIR}/${bin}${BIN_SUFFIX}"
    INSTALLED_BINS=$((INSTALLED_BINS + 1))
  fi
done
[ "$INSTALLED_BINS" -gt 0 ] || die "No binaries found in archive"
info "Installed ${INSTALLED_BINS} binaries -> ${INSTALL_DIR}/"

# PATH
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    export PATH="${PATH}:${INSTALL_DIR}"
    if $IS_WINDOWS; then
      if has_cmd powershell.exe; then
        WIN_DIR=$(cygpath -w "$INSTALL_DIR" 2>/dev/null || printf '%s' "$INSTALL_DIR")
        powershell.exe -NoProfile -Command "
          \$p=[Environment]::GetEnvironmentVariable('Path','User');
          if(\$p -notlike '*${WIN_DIR}*'){[Environment]::SetEnvironmentVariable('Path',\"\$p;${WIN_DIR}\",'User')}
        " 2>/dev/null && info "PATH updated (Windows)" || true
      fi
    else
      SHELL_RC=""
      if [ -n "${ZSH_VERSION:-}" ] || [ "$(basename "${SHELL:-}")" = "zsh" ]; then SHELL_RC="$HOME/.zshrc"
      elif [ -f "$HOME/.bashrc" ]; then SHELL_RC="$HOME/.bashrc"
      elif [ -f "$HOME/.bash_profile" ]; then SHELL_RC="$HOME/.bash_profile"
      elif [ -f "$HOME/.profile" ]; then SHELL_RC="$HOME/.profile"; fi
      if [ -n "$SHELL_RC" ] && ! grep -q "$INSTALL_DIR" "$SHELL_RC" 2>/dev/null; then
        printf '\n# ForgeGuardian\nexport PATH="$PATH:%s"\n' "$INSTALL_DIR" >> "$SHELL_RC"
        info "PATH added to ${SHELL_RC}"
      fi
    fi
    ;;
esac

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 4: Dashboard
# ═══════════════════════════════════════════════════════════════════════════════
step 4 "Installing dashboard"

DASH_DIR="${DATA_DIR}/dashboard"
if fetch "${BASE_URL}/dashboard-dist.tar.gz" "${TMPDIR}/dashboard-dist.tar.gz" 2>/dev/null; then
  rm -rf "$DASH_DIR"
  ensure_dir "$DASH_DIR"
  tar -xzf "${TMPDIR}/dashboard-dist.tar.gz" -C "$DASH_DIR"
  info "Dashboard -> ${DASH_DIR}/"
else
  warn "Dashboard not in this release"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 5: Scanner engines (OS-aware)
# ═══════════════════════════════════════════════════════════════════════════════
step 5 "Installing scanner engines"

if [ "$SKIP_ENGINES" = "1" ]; then
  warn "Skipping (SKIP_ENGINES=1)"
else
  INSTALLED_ENGINES=0

  # ── grype ────────────────────────────────────────────────────────────────
  if has_cmd grype; then
    info "grype: already installed"; INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1))
  else
    printf "  ${DIM}  Installing grype...${NC}\n"; GRYPE_OK=false
    if $IS_WINDOWS; then
      # Windows: winget → scoop → choco
      if ! $GRYPE_OK && [ "$PKG" = "winget" ]; then
        pkg_install anchore.grype && GRYPE_OK=true && info "grype: installed via winget"; fi
      if ! $GRYPE_OK && has_cmd scoop; then
        scoop install grype >/dev/null 2>&1 && GRYPE_OK=true && info "grype: installed via scoop"; fi
      if ! $GRYPE_OK && has_cmd choco; then
        choco install grype -y >/dev/null 2>&1 && GRYPE_OK=true && info "grype: installed via choco"; fi
    else
      # Linux/macOS: official script → brew → system pkg
      if ! $GRYPE_OK; then
        fetch_stdout "https://raw.githubusercontent.com/anchore/grype/main/install.sh" \
          | sh -s -- -b "$INSTALL_DIR" >/dev/null 2>&1 \
          && GRYPE_OK=true && info "grype: installed -> ${INSTALL_DIR}/grype"; fi
      if ! $GRYPE_OK && has_cmd brew; then
        brew install anchore/grype/grype >/dev/null 2>&1 && GRYPE_OK=true && info "grype: installed via brew"; fi
      if ! $GRYPE_OK; then
        pkg_install grype 2>/dev/null && GRYPE_OK=true && info "grype: installed via ${PKG}"; fi
    fi
    $GRYPE_OK && INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1)) \
      || warn "grype: could not install — https://github.com/anchore/grype#installation"
  fi

  # ── trivy ────────────────────────────────────────────────────────────────
  if has_cmd trivy; then
    info "trivy: already installed"; INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1))
  else
    printf "  ${DIM}  Installing trivy...${NC}\n"; TRIVY_OK=false
    if $IS_WINDOWS; then
      if ! $TRIVY_OK && [ "$PKG" = "winget" ]; then
        pkg_install AquaSecurity.Trivy && TRIVY_OK=true && info "trivy: installed via winget"; fi
      if ! $TRIVY_OK && has_cmd scoop; then
        scoop install trivy >/dev/null 2>&1 && TRIVY_OK=true && info "trivy: installed via scoop"; fi
      if ! $TRIVY_OK && has_cmd choco; then
        choco install trivy -y >/dev/null 2>&1 && TRIVY_OK=true && info "trivy: installed via choco"; fi
    else
      if ! $TRIVY_OK; then
        fetch_stdout "https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh" \
          | sh -s -- -b "$INSTALL_DIR" >/dev/null 2>&1 \
          && TRIVY_OK=true && info "trivy: installed -> ${INSTALL_DIR}/trivy"; fi
      if ! $TRIVY_OK && has_cmd brew; then
        brew install trivy >/dev/null 2>&1 && TRIVY_OK=true && info "trivy: installed via brew"; fi
      if ! $TRIVY_OK; then
        pkg_install trivy 2>/dev/null && TRIVY_OK=true && info "trivy: installed via ${PKG}"; fi
    fi
    $TRIVY_OK && INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1)) \
      || warn "trivy: could not install — https://github.com/aquasecurity/trivy#installation"
  fi

  # ── semgrep ──────────────────────────────────────────────────────────────
  if has_cmd semgrep; then
    info "semgrep: already installed"; INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1))
  else
    printf "  ${DIM}  Installing semgrep...${NC}\n"; SEMGREP_OK=false
    if ! $SEMGREP_OK && pip_install semgrep; then
      SEMGREP_OK=true; info "semgrep: installed via pip"; fi
    if ! $SEMGREP_OK && has_cmd pipx; then
      pipx install semgrep >/dev/null 2>&1 && SEMGREP_OK=true && info "semgrep: installed via pipx"; fi
    if $IS_WINDOWS; then
      if ! $SEMGREP_OK && [ "$PKG" = "winget" ]; then
        pkg_install Semgrep.Semgrep && SEMGREP_OK=true && info "semgrep: installed via winget"; fi
    else
      if ! $SEMGREP_OK && has_cmd brew; then
        brew install semgrep >/dev/null 2>&1 && SEMGREP_OK=true && info "semgrep: installed via brew"; fi
    fi
    $SEMGREP_OK && INSTALLED_ENGINES=$((INSTALLED_ENGINES + 1)) \
      || warn "semgrep: could not install — pip3 install semgrep"
  fi

  info "${INSTALLED_ENGINES}/3 scanner engines ready"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 6: Signatures + config
# ═══════════════════════════════════════════════════════════════════════════════
step 6 "Setting up signatures and config"

ensure_dir "$DATA_DIR"
FGCTL="${INSTALL_DIR}/fgctl${BIN_SUFFIX}"
if [ -f "$FGCTL" ]; then
  chmod +x "$FGCTL" 2>/dev/null || true
  "$FGCTL" update 2>/dev/null && info "Community signatures downloaded" \
    || warn "Signature download failed — run 'fgctl update' later"
fi

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
  chmod 600 "$CFG_FILE" 2>/dev/null || true
  info "Config created -> ${CFG_FILE}"
else
  info "Config exists -> ${CFG_FILE}"
fi

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 7: Auto-start server
# ═══════════════════════════════════════════════════════════════════════════════
step 7 "Setting up server"

if [ "$SKIP_SERVER" = "1" ]; then
  warn "Skipping (SKIP_SERVER=1)"
elif $IS_WINDOWS; then
  info "Run 'fgctl serve' to start the platform"
elif [ "$OS" = "darwin" ]; then
  # macOS: create a launchd agent
  PLIST_DIR="${HOME}/Library/LaunchAgents"
  PLIST_FILE="${PLIST_DIR}/com.forgeguardian.server.plist"
  ensure_dir "$PLIST_DIR"
  cat > "$PLIST_FILE" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.forgeguardian.server</string>
  <key>ProgramArguments</key>
  <array>
    <string>${INSTALL_DIR}/fgctl</string>
    <string>serve</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>${DATA_DIR}/server.log</string>
  <key>StandardErrorPath</key>
  <string>${DATA_DIR}/server.log</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key>
    <string>${INSTALL_DIR}:/usr/local/bin:/usr/bin:/bin</string>
  </dict>
</dict>
</plist>
PLIST
  launchctl load "$PLIST_FILE" 2>/dev/null && info "Server started (launchd agent)" \
    || warn "Could not start launchd agent — run 'fgctl serve' manually"
  info "Logs: ${DATA_DIR}/server.log"
elif has_cmd systemctl; then
  # Linux with systemd: create a user service
  SYSTEMD_DIR="${HOME}/.config/systemd/user"
  ensure_dir "$SYSTEMD_DIR"
  cat > "${SYSTEMD_DIR}/forgeguardian.service" <<SVC
[Unit]
Description=ForgeGuardian Security Platform
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/fgctl serve
Restart=on-failure
RestartSec=5
Environment=PATH=${INSTALL_DIR}:/usr/local/bin:/usr/bin:/bin

[Install]
WantedBy=default.target
SVC
  systemctl --user daemon-reload 2>/dev/null || true
  systemctl --user enable forgeguardian.service 2>/dev/null || true
  systemctl --user start forgeguardian.service 2>/dev/null \
    && info "Server started (systemd user service)" \
    || warn "Could not start systemd service — run 'fgctl serve' manually"
  # Enable lingering so the service runs even when user is not logged in
  loginctl enable-linger "$(whoami)" 2>/dev/null || true
  info "Manage: systemctl --user {start|stop|status} forgeguardian"
  info "Logs:   journalctl --user -u forgeguardian -f"
else
  # No systemd — start in background
  nohup "${INSTALL_DIR}/fgctl" serve > "${DATA_DIR}/server.log" 2>&1 &
  SERVER_PID=$!
  sleep 1
  if kill -0 "$SERVER_PID" 2>/dev/null; then
    info "Server started in background (PID ${SERVER_PID})"
    info "Logs: ${DATA_DIR}/server.log"
    printf '%s' "$SERVER_PID" > "${DATA_DIR}/server.pid"
  else
    warn "Server failed to start — check ${DATA_DIR}/server.log"
  fi
fi

# ═══════════════════════════════════════════════════════════════════════════════
# STEP 8: Verify
# ═══════════════════════════════════════════════════════════════════════════════
step 8 "Verifying installation"

printf "\n"
CHECK_PASS=0; CHECK_TOTAL=0

for item in fgctl grype trivy semgrep; do
  CHECK_TOTAL=$((CHECK_TOTAL + 1))
  if has_cmd "$item"; then
    info "${item}: $(command -v "$item")"; CHECK_PASS=$((CHECK_PASS + 1))
  else warn "${item}: not found in PATH"; fi
done

CHECK_TOTAL=$((CHECK_TOTAL + 1))
if [ -f "${DASH_DIR}/index.html" ] 2>/dev/null; then
  info "dashboard: installed"; CHECK_PASS=$((CHECK_PASS + 1))
else warn "dashboard: not installed"; fi

CHECK_TOTAL=$((CHECK_TOTAL + 1))
if [ -f "${DATA_DIR}/signatures.json" ]; then
  SIG_COUNT=$(grep -c '"id"' "${DATA_DIR}/signatures.json" 2>/dev/null || printf "?")
  info "signatures: ${SIG_COUNT} loaded"; CHECK_PASS=$((CHECK_PASS + 1))
else warn "signatures: not found"; fi

printf "\n"
printf "  ${BOLD}==========================================${NC}\n"
if [ "$CHECK_PASS" -eq "$CHECK_TOTAL" ]; then
  printf "  ${GREEN}${BOLD}  All %d components installed${NC}\n" "$CHECK_TOTAL"
else
  printf "  ${GREEN}${BOLD}  Installed %d/%d components${NC}\n" "$CHECK_PASS" "$CHECK_TOTAL"
fi
printf "  ${BOLD}==========================================${NC}\n"
printf "\n"
if [ "$SKIP_SERVER" != "1" ] && ! $IS_WINDOWS; then
  printf "  ${BOLD}Dashboard:${NC}\n"
  printf "    ${GREEN}${BOLD}http://localhost:8080${NC}\n"
  printf "    ${DIM}Server is running — open the URL above in your browser${NC}\n"
else
  printf "  ${BOLD}Start the platform:${NC}\n"
  printf "    ${GREEN}${BOLD}fgctl serve${NC}\n"
  printf "    ${DIM}-> API + Dashboard at http://localhost:8080${NC}\n"
fi
printf "\n"
printf "  ${BOLD}Scan a project:${NC}\n"
printf "    ${GREEN}fgctl scan .${NC}                        ${DIM}# scan current directory${NC}\n"
printf "    ${GREEN}fgctl scan npm/lodash@4.17.21${NC}       ${DIM}# scan a specific package${NC}\n"
printf "    ${GREEN}fgctl audit system${NC}                  ${DIM}# audit all installed packages${NC}\n"
printf "\n"
printf "  ${BOLD}Check your setup:${NC}\n"
printf "    ${GREEN}fgctl doctor${NC}                        ${DIM}# full environment check${NC}\n"
printf "\n"

exit 0

# ═══════════════════════════════════════════════════════════════════════════════
# EMBEDDED POWERSHELL INSTALLER — for native Windows without bash
# ═══════════════════════════════════════════════════════════════════════════════
# Bash never reaches this section (exit 0 above). The heredoc feeds it to
# the `:` null command which discards it.
#
# install.ps1 is a thin bootstrapper that downloads this file and extracts
# the PowerShell code between the PS_BLOCK_END markers below.

: <<'PS_BLOCK_END'
$ErrorActionPreference = 'Stop'

$repo = 'Mah3Sec/ForgeGuardian'
if ($env:FORGEGUARDIAN_VERSION) { $version = $env:FORGEGUARDIAN_VERSION } else { $version = 'latest' }
if ($env:INSTALL_DIR) { $installDir = $env:INSTALL_DIR } else { $installDir = Join-Path $HOME '.local\bin' }
$dataDir = Join-Path $HOME '.forgeguardian'
$skipEngines = $env:SKIP_ENGINES -eq '1'

function Write-Step { param($n, $msg) Write-Host "`n  [$n/8] $msg" -ForegroundColor Cyan }
function Write-Ok { param($msg) Write-Host "  > $msg" -ForegroundColor Green }
function Write-Warn { param($msg) Write-Host "  ! $msg" -ForegroundColor Yellow }
function Test-Cmd { param($cmd) $null -ne (Get-Command $cmd -ErrorAction SilentlyContinue) }

# ── Step 1: Detect ──────────────────────────────────────────────────────────
Write-Step 1 'Detecting platform'

if ([Environment]::Is64BitOperatingSystem) { $arch = 'amd64' } else { $arch = '386' }
$osVer = [Environment]::OSVersion.VersionString

Write-Host ''
Write-Host '  +==========================================+' -ForegroundColor White
Write-Host '  |    ForgeGuardian - Universal Installer    |' -ForegroundColor White
Write-Host '  +==========================================+' -ForegroundColor White
Write-Host ''
Write-Host '  Detected system:' -ForegroundColor Magenta
Write-Host "    OS ............. Windows ($osVer)"
Write-Host "    Architecture ... $arch"
$pkgMgr = 'none'
foreach ($p in @('winget','scoop','choco')) { if (Test-Cmd $p) { $pkgMgr = $p; break } }
Write-Host "    Pkg manager .... $pkgMgr"
Write-Host ''

# ── Step 2: Prerequisites ──────────────────────────────────────────────────
Write-Step 2 'Checking prerequisites'

Write-Ok 'curl: built-in (Windows 10+)'
Write-Ok 'tar: built-in (Windows 10+)'

if (-not $skipEngines -and -not (Test-Cmd 'python3') -and -not (Test-Cmd 'python')) {
    Write-Warn 'python3 not found — installing...'
    if ($pkgMgr -eq 'winget') {
        try { winget install Python.Python.3.12 --accept-source-agreements --accept-package-agreements --silent 2>$null; Write-Ok 'python3: installed via winget' } catch { Write-Warn 'Could not install python3' }
    } elseif ($pkgMgr -eq 'scoop') {
        try { scoop install python 2>$null; Write-Ok 'python3: installed via scoop' } catch {}
    } elseif ($pkgMgr -eq 'choco') {
        try { choco install python3 -y 2>$null; Write-Ok 'python3: installed via choco' } catch {}
    } else { Write-Warn 'Install python3 manually for semgrep support' }
} else {
    Write-Ok 'python: found'
}

# ── Step 3: Download binaries ──────────────────────────────────────────────
Write-Step 3 'Installing ForgeGuardian'

if ($version -eq 'latest') {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -UseBasicParsing
    $version = $release.tag_name
}
$versionBare = $version -replace '^v', ''
$baseUrl = "https://github.com/$repo/releases/download/$version"
Write-Ok "Version: $version"

$asset = "forgeguardian_${versionBare}_windows_${arch}.zip"
$tmpDir = Join-Path ([IO.Path]::GetTempPath()) "forgeguardian-install-$(Get-Random)"
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

Write-Ok "Downloading $asset..."
try {
    Invoke-WebRequest -Uri "$baseUrl/$asset" -OutFile (Join-Path $tmpDir $asset) -UseBasicParsing
} catch { Write-Host "  x Download failed: $_" -ForegroundColor Red; exit 1 }

try {
    $checksumFile = Join-Path $tmpDir 'checksums.txt'
    Invoke-WebRequest -Uri "$baseUrl/checksums.txt" -OutFile $checksumFile -UseBasicParsing
    $expectedLine = (Get-Content $checksumFile | Where-Object { $_ -match $asset }) -split '\s+' | Select-Object -First 1
    if ($expectedLine) {
        $actualHash = (Get-FileHash (Join-Path $tmpDir $asset) -Algorithm SHA256).Hash.ToLower()
        if ($actualHash -eq $expectedLine.ToLower()) { Write-Ok 'Checksum verified' }
    }
} catch {}

$extractDir = Join-Path $tmpDir 'extracted'
Expand-Archive -Path (Join-Path $tmpDir $asset) -DestinationPath $extractDir -Force

New-Item -ItemType Directory -Path $installDir -Force | Out-Null
$count = 0
foreach ($bin in @('fgctl.exe','fg-agent.exe','intel-agent.exe')) {
    $src = Join-Path $extractDir $bin
    if (Test-Path $src) { Copy-Item $src (Join-Path $installDir $bin) -Force; $count++ }
}
if ($count -eq 0) { Write-Host '  x No binaries found' -ForegroundColor Red; exit 1 }
Write-Ok "Installed $count binaries -> $installDir"

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$installDir", 'User')
    $env:Path = "$env:Path;$installDir"
    Write-Ok 'PATH updated'
}

# ── Step 4: Dashboard ──────────────────────────────────────────────────────
Write-Step 4 'Installing dashboard'
$dashDir = Join-Path $dataDir 'dashboard'
$dashTar = Join-Path $tmpDir 'dashboard-dist.tar.gz'
try {
    Invoke-WebRequest -Uri "$baseUrl/dashboard-dist.tar.gz" -OutFile $dashTar -UseBasicParsing
    if (Test-Path $dashDir) { Remove-Item -Recurse -Force $dashDir -Confirm:$false }
    New-Item -ItemType Directory -Path $dashDir -Force | Out-Null
    tar -xzf $dashTar -C $dashDir 2>$null
    Write-Ok "Dashboard -> $dashDir"
} catch { Write-Warn 'Dashboard not in this release' }

# ── Step 5: Scanner engines ────────────────────────────────────────────────
Write-Step 5 'Installing scanner engines'
$eng = 0

if (-not $skipEngines) {
    # grype
    if (Test-Cmd 'grype') { Write-Ok 'grype: already installed'; $eng++ }
    else {
        Write-Host '    Installing grype...' -ForegroundColor DarkGray
        $ok = $false
        if (-not $ok -and $pkgMgr -eq 'winget') { try { winget install anchore.grype --accept-source-agreements --accept-package-agreements --silent 2>$null; if ($LASTEXITCODE -eq 0) { $ok=$true; Write-Ok 'grype: installed via winget' } } catch {} }
        if (-not $ok -and (Test-Cmd 'scoop')) { try { scoop install grype 2>$null; if ($LASTEXITCODE -eq 0) { $ok=$true; Write-Ok 'grype: installed via scoop' } } catch {} }
        if (-not $ok -and (Test-Cmd 'choco')) { try { choco install grype -y 2>$null; if ($LASTEXITCODE -eq 0) { $ok=$true; Write-Ok 'grype: installed via choco' } } catch {} }
        if ($ok) { $eng++ } else { Write-Warn 'grype: could not install' }
    }

    # trivy
    if (Test-Cmd 'trivy') { Write-Ok 'trivy: already installed'; $eng++ }
    else {
        Write-Host '    Installing trivy...' -ForegroundColor DarkGray
        $ok = $false
        if (-not $ok -and $pkgMgr -eq 'winget') { try { winget install AquaSecurity.Trivy --accept-source-agreements --accept-package-agreements --silent 2>$null; if ($LASTEXITCODE -eq 0) { $ok=$true; Write-Ok 'trivy: installed via winget' } } catch {} }
        if (-not $ok -and (Test-Cmd 'scoop')) { try { scoop install trivy 2>$null; if ($LASTEXITCODE -eq 0) { $ok=$true; Write-Ok 'trivy: installed via scoop' } } catch {} }
        if (-not $ok -and (Test-Cmd 'choco')) { try { choco install trivy -y 2>$null; if ($LASTEXITCODE -eq 0) { $ok=$true; Write-Ok 'trivy: installed via choco' } } catch {} }
        if ($ok) { $eng++ } else { Write-Warn 'trivy: could not install' }
    }

    # semgrep
    if (Test-Cmd 'semgrep') { Write-Ok 'semgrep: already installed'; $eng++ }
    else {
        Write-Host '    Installing semgrep...' -ForegroundColor DarkGray
        $ok = $false
        if (-not $ok -and (Test-Cmd 'pip3')) { try { pip3 install semgrep --quiet 2>$null; if ($LASTEXITCODE -eq 0) { $ok=$true; Write-Ok 'semgrep: installed via pip3' } } catch {} }
        if (-not $ok -and (Test-Cmd 'pip')) { try { pip install semgrep --quiet 2>$null; if ($LASTEXITCODE -eq 0) { $ok=$true; Write-Ok 'semgrep: installed via pip' } } catch {} }
        if (-not $ok -and (Test-Cmd 'pipx')) { try { pipx install semgrep 2>$null; if ($LASTEXITCODE -eq 0) { $ok=$true; Write-Ok 'semgrep: installed via pipx' } } catch {} }
        if (-not $ok -and $pkgMgr -eq 'winget') { try { winget install Semgrep.Semgrep --accept-source-agreements --accept-package-agreements --silent 2>$null; if ($LASTEXITCODE -eq 0) { $ok=$true; Write-Ok 'semgrep: installed via winget' } } catch {} }
        if ($ok) { $eng++ } else { Write-Warn 'semgrep: could not install' }
    }
    Write-Ok "$eng/3 scanner engines ready"
} else { Write-Warn 'Skipping (SKIP_ENGINES=1)' }

# ── Step 6: Signatures + config ───────────────────────────────────────────
Write-Step 6 'Setting up signatures and config'
New-Item -ItemType Directory -Path $dataDir -Force | Out-Null
$fgctl = Join-Path $installDir 'fgctl.exe'
if (Test-Path $fgctl) {
    try { & $fgctl update 2>$null; Write-Ok 'Community signatures downloaded' }
    catch { Write-Warn 'Signature download failed' }
}
$cfgFile = Join-Path $dataDir 'config.yaml'
if (-not (Test-Path $cfgFile)) {
    @"
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
"@ | Out-File $cfgFile -Encoding utf8
    Write-Ok "Config created -> $cfgFile"
} else { Write-Ok "Config exists -> $cfgFile" }

# ── Step 7: Server ─────────────────────────────────────────────────────────
Write-Step 7 'Setting up server'

if ($env:SKIP_SERVER -ne '1') {
    $fgctlExe = Join-Path $installDir 'fgctl.exe'
    if (Test-Path $fgctlExe) {
        try {
            Start-Process -FilePath $fgctlExe -ArgumentList 'serve' -WindowStyle Hidden -PassThru | Out-Null
            Start-Sleep -Seconds 2
            Write-Ok 'Server started in background'
            Write-Ok 'Dashboard: http://localhost:8080'
        } catch { Write-Warn 'Could not auto-start — run "fgctl serve" manually' }
    }
} else { Write-Warn 'Skipping (SKIP_SERVER=1)' }

# ── Step 8: Verify ─────────────────────────────────────────────────────────
Write-Step 8 'Verifying installation'
Write-Host ''
$pass = 0; $total = 0
foreach ($item in @('fgctl','grype','trivy','semgrep')) {
    $total++
    if (Test-Cmd $item) { Write-Ok "${item}: $((Get-Command $item).Source)"; $pass++ }
    else { Write-Warn "${item}: not found" }
}
$total++
if (Test-Path (Join-Path $dashDir 'index.html')) { Write-Ok 'dashboard: installed'; $pass++ }
else { Write-Warn 'dashboard: not installed' }
$total++
$sigFile = Join-Path $dataDir 'signatures.json'
if (Test-Path $sigFile) { Write-Ok 'signatures: loaded'; $pass++ }
else { Write-Warn 'signatures: not found' }

Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue -Confirm:$false

Write-Host ''
Write-Host '  ==========================================' -ForegroundColor White
if ($pass -eq $total) { Write-Host "    All $total components installed" -ForegroundColor Green }
else { Write-Host "    Installed $pass/$total components" -ForegroundColor Green }
Write-Host '  ==========================================' -ForegroundColor White
Write-Host ''
if ($env:SKIP_SERVER -ne '1') {
    Write-Host '  Dashboard:' -ForegroundColor Cyan
    Write-Host '    http://localhost:8080' -ForegroundColor Green
    Write-Host '    Server is running — open the URL above in your browser' -ForegroundColor DarkGray
} else {
    Write-Host '  Start the platform:' -ForegroundColor Cyan
    Write-Host '    fgctl serve' -ForegroundColor Green
    Write-Host '    -> API + Dashboard at http://localhost:8080' -ForegroundColor DarkGray
}
Write-Host ''
Write-Host '  Scan a project:' -ForegroundColor Cyan
Write-Host '    fgctl scan .                        # scan current directory' -ForegroundColor DarkGray
Write-Host '    fgctl scan npm/lodash@4.17.21       # scan a specific package' -ForegroundColor DarkGray
Write-Host '    fgctl audit system                  # audit all installed packages' -ForegroundColor DarkGray
Write-Host ''
Write-Host '  Check setup:' -ForegroundColor Cyan
Write-Host '    fgctl doctor                        # verify environment' -ForegroundColor DarkGray
Write-Host ''
PS_BLOCK_END

