# ForgeGuardian — All-in-one Docker image
#
# Usage:
#   docker build -t forgeguardian .
#   docker run -d --name forgeguardian -p 3000:3000 forgeguardian
#
# Then open http://localhost:3000
#   Default login: admin@forgeguardian.local / changeme123
#
# Custom credentials:
#   docker run -d -p 3000:3000 \
#     -e FG_ADMIN_EMAIL=you@example.com \
#     -e FG_ADMIN_PASSWORD=YourSecurePass \
#     forgeguardian

# ── Stage 1: Build Go API ────────────────────────────────────────────────────
FROM golang:1.25-bookworm AS api-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /forgeguardian-api \
    ./internal/api/

# ── Stage 2: Build Dashboard ─────────────────────────────────────────────────
FROM node:24-slim AS dashboard-builder
WORKDIR /app
COPY dashboard/package.json dashboard/package-lock.json ./
RUN npm ci --ignore-scripts
COPY dashboard/ .
RUN npm run build

# ── Stage 3: Runtime ─────────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd -r fg && useradd -r -g fg -d /data -s /bin/false fg && \
    mkdir -p /data && chown fg:fg /data

COPY --from=api-builder /forgeguardian-api /app/forgeguardian-api
COPY --from=dashboard-builder /app/dist /app/dashboard
COPY docker/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

USER fg
ENV HOME=/data
WORKDIR /app
EXPOSE 3000
VOLUME /data

ENTRYPOINT ["/app/entrypoint.sh"]
