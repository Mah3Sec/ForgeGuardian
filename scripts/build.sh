#!/usr/bin/env bash
# ForgeGuardian — Cross-platform release build script
# Produces signed, checksummed binaries for all supported platforms.
#
# Usage:
#   bash scripts/build.sh                   # build all platforms
#   PLATFORMS="linux/amd64" bash scripts/build.sh   # single platform
#   VERSION=v1.2.3 bash scripts/build.sh    # set version

set -euo pipefail

# ─── Config ────────────────────────────────────────────────────────────────
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}"
BUILD_TIME="${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
DIST_DIR="${DIST_DIR:-dist}"

LDFLAGS="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}"

# Binaries to build: "output_name:source_path"
BINARIES=(
    "fgctl:./cmd/fgctl/main.go"
    "fg-agent:./agent/main.go"
    "intel-agent:./cmd/intel-agent/main.go"
)

# Platforms to build for
PLATFORMS="${PLATFORMS:-linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64}"

# ─── Colors ────────────────────────────────────────────────────────────────
GREEN='\033[0;32m'
CYAN='\033[0;36m'
AMBER='\033[0;33m'
RESET='\033[0m'

log()  { echo -e "${CYAN}[build]${RESET} $*"; }
ok()   { echo -e "${GREEN}[✓]${RESET} $*"; }
warn() { echo -e "${AMBER}[!]${RESET} $*"; }

# ─── Preflight ─────────────────────────────────────────────────────────────
log "ForgeGuardian Release Build"
log "Version:    ${VERSION}"
log "Commit:     ${COMMIT}"
log "Built at:   ${BUILD_TIME}"
echo ""

mkdir -p "${DIST_DIR}"

# ─── Build loop ────────────────────────────────────────────────────────────
for platform in ${PLATFORMS}; do
    GOOS="${platform%%/*}"
    GOARCH="${platform##*/}"

    for binary_def in "${BINARIES[@]}"; do
        BINARY_NAME="${binary_def%%:*}"
        SOURCE="${binary_def##*:}"

        # Output filename
        OUTPUT="${BINARY_NAME}-${GOOS}-${GOARCH}"
        [[ "${GOOS}" == "windows" ]] && OUTPUT="${OUTPUT}.exe"
        OUTPUT_PATH="${DIST_DIR}/${OUTPUT}"

        log "Building ${OUTPUT} ..."

        CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
            go build \
            -trimpath \
            -ldflags="${LDFLAGS}" \
            -o "${OUTPUT_PATH}" \
            "${SOURCE}"

        # SHA256 checksum
        CHECKSUM_FILE="${OUTPUT_PATH}.sha256"
        if command -v sha256sum &>/dev/null; then
            sha256sum "${OUTPUT_PATH}" > "${CHECKSUM_FILE}"
        elif command -v shasum &>/dev/null; then
            shasum -a 256 "${OUTPUT_PATH}" > "${CHECKSUM_FILE}"
        fi

        SIZE=$(du -sh "${OUTPUT_PATH}" | cut -f1)
        ok "${OUTPUT}  (${SIZE})"
    done
done

# ─── Checksums manifest ────────────────────────────────────────────────────
echo ""
log "Generating checksums manifest ..."
MANIFEST="${DIST_DIR}/checksums.txt"
cat "${DIST_DIR}"/*.sha256 > "${MANIFEST}" 2>/dev/null || true
ok "Checksums: ${MANIFEST}"

# ─── Archive (tar.gz for unix, zip for windows) ───────────────────────────
echo ""
log "Creating archives ..."

for platform in ${PLATFORMS}; do
    GOOS="${platform%%/*}"
    GOARCH="${platform##*/}"

    ARCHIVE_NAME="forgeguardian-${VERSION}-${GOOS}-${GOARCH}"
    FILES=()

    for binary_def in "${BINARIES[@]}"; do
        BINARY_NAME="${binary_def%%:*}"
        OUTPUT="${BINARY_NAME}-${GOOS}-${GOARCH}"
        [[ "${GOOS}" == "windows" ]] && OUTPUT="${OUTPUT}.exe"
        FILES+=("${OUTPUT}")
    done

    # Include docs
    DOCS=(README.md LICENSE GUIDE.md SIGNATURES.md)

    pushd "${DIST_DIR}" > /dev/null

    if [[ "${GOOS}" == "windows" ]]; then
        if command -v zip &>/dev/null; then
            zip -q "${ARCHIVE_NAME}.zip" "${FILES[@]}" 2>/dev/null || true
            for doc in "${DOCS[@]}"; do
                [[ -f "../${doc}" ]] && zip -q "${ARCHIVE_NAME}.zip" "../${doc}" 2>/dev/null || true
            done
            ok "${ARCHIVE_NAME}.zip"
        else
            warn "zip not found — skipping Windows archive"
        fi
    else
        tar -czf "${ARCHIVE_NAME}.tar.gz" "${FILES[@]}" 2>/dev/null || true
        ok "${ARCHIVE_NAME}.tar.gz"
    fi

    popd > /dev/null
done

# ─── Summary ───────────────────────────────────────────────────────────────
echo ""
echo "────────────────────────────────────────────────────"
echo "  Release: ${VERSION}"
echo "  Output:  ${DIST_DIR}/"
echo "────────────────────────────────────────────────────"
ls -lh "${DIST_DIR}/" | grep -v "^total"
echo ""
ok "Build complete."
