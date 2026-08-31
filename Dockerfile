# ForgeGuardian — All-in-one Docker image
#
# One-liner (from repo clone):
#   docker compose up -d
#
# One-liner (pre-built image):
#   docker run -d --name forgeguardian -p 3000:3000 ghcr.io/mah3sec/forgeguardian
#
# Then open http://localhost:3000
#   Default login: admin@forgeguardian.local / changeme123
#
# Custom credentials:
#   docker run -d -p 3000:3000 \
#     -e FG_ADMIN_EMAIL=you@example.com \
#     -e FG_ADMIN_PASSWORD=YourSecurePass \
#     ghcr.io/mah3sec/forgeguardian
#
# Using the CLI from Docker:
#   docker exec forgeguardian fgctl version
#   docker exec forgeguardian fgctl scan npm/lodash@4.17.21
#   docker exec forgeguardian fgctl doctor

# ── Stage 1: Build Go binaries ───────────────────────────────────────────────
FROM golang:1-bookworm AS go-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=1.0.0
ARG FG_LICENSE_PUBLIC_KEY
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X github.com/mah3sec/forgeguardian/internal/license.publicKeyHex=${FG_LICENSE_PUBLIC_KEY}" \
    -o /forgeguardian-api \
    ./internal/api/
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X github.com/mah3sec/forgeguardian/internal/license.publicKeyHex=${FG_LICENSE_PUBLIC_KEY}" \
    -o /fgctl \
    ./cmd/fgctl/

# ── Stage 2: Build Dashboard ────────────────────────────────────────────────
FROM node:22-slim AS dashboard-builder
WORKDIR /app
COPY dashboard/package.json dashboard/package-lock.json ./
RUN npm ci --ignore-scripts
COPY dashboard/ .
RUN npm run build

# ── Stage 3: Scanner engines ────────────────────────────────────────────────
FROM debian:bookworm-slim AS engine-builder

RUN apt-get update && \
    apt-get install -y --no-install-recommends curl ca-certificates && \
    rm -rf /var/lib/apt/lists/*

RUN curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /engines
RUN curl -sSfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /engines

# ── Stage 4: Runtime ────────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates openssh-client python3-pip && \
    pip3 install --no-cache-dir --break-system-packages semgrep && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd -r fg && useradd -r -g fg -d /data -s /bin/false fg && \
    mkdir -p /data/.forgeguardian && chown -R fg:fg /data

COPY --from=go-builder /forgeguardian-api /app/forgeguardian-api
COPY --from=go-builder /fgctl /usr/local/bin/fgctl
COPY --from=engine-builder /engines/grype /usr/local/bin/grype
COPY --from=engine-builder /engines/trivy /usr/local/bin/trivy
COPY --from=dashboard-builder /app/dist /app/dashboard
COPY docker/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

USER fg
ENV HOME=/data
WORKDIR /app
EXPOSE 3000
VOLUME /data

ENTRYPOINT ["/app/entrypoint.sh"]
