# ForgeGuardian — Makefile
# Usage: make <target>
# Run `make help` for a list of all targets.

# ─── Config ────────────────────────────────────────────────────────────────
BINARY_DIR   := bin
FGCTL        := $(BINARY_DIR)/fgctl
FG_AGENT     := $(BINARY_DIR)/fg-agent
INTEL_AGENT  := $(BINARY_DIR)/intel-agent

VERSION      ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT       := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS      := -s -w \
                -X main.version=$(VERSION) \
                -X main.commit=$(COMMIT) \
                -X main.buildTime=$(BUILD_TIME)

GO           := go
GOFLAGS      := -trimpath
CGO_ENABLED  := 0

.PHONY: all build build-fgctl build-agent build-intel clean install uninstall \
        test test-race lint fmt vet \
        extension \
        docker docker-minimal docker-down docker-logs \
        api up down logs health \
        release \
        help

# ─── Default ───────────────────────────────────────────────────────────────
all: build

# ─── Build ─────────────────────────────────────────────────────────────────

## build: Build all Go binaries into bin/
build: build-fgctl build-agent build-intel
	@echo ""
	@echo "  Built:"
	@echo "    $(FGCTL)"
	@echo "    $(FG_AGENT)"
	@echo "    $(INTEL_AGENT)"
	@echo ""
	@echo "  Run:  ./$(FGCTL) help"

## build-fgctl: Build the fgctl CLI
build-fgctl:
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) \
		-ldflags="$(LDFLAGS)" \
		-o $(FGCTL) \
		./cmd/fgctl/
	@echo "  [✓] fgctl  →  $(FGCTL)"

## build-agent: Build the autonomous patch agent
build-agent:
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) \
		-ldflags="$(LDFLAGS)" \
		-o $(FG_AGENT) \
		./cmd/fg-agent/
	@echo "  [✓] fg-agent  →  $(FG_AGENT)"

## build-intel: Build the intelligence daemon
build-intel:
	@mkdir -p $(BINARY_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) \
		-ldflags="$(LDFLAGS)" \
		-o $(INTEL_AGENT) \
		./cmd/intel-agent/
	@echo "  [✓] intel-agent  →  $(INTEL_AGENT)"

# ─── Install / Uninstall ───────────────────────────────────────────────────

## install: Install all binaries to /usr/local/bin
install: build
	@echo "Installing to /usr/local/bin ..."
	install -m 755 $(FGCTL)       /usr/local/bin/fgctl
	install -m 755 $(FG_AGENT)    /usr/local/bin/fg-agent
	install -m 755 $(INTEL_AGENT) /usr/local/bin/intel-agent
	@echo "  [✓] Installed. Run: fgctl help"

## uninstall: Remove installed binaries from /usr/local/bin
uninstall:
	rm -f /usr/local/bin/fgctl /usr/local/bin/fg-agent /usr/local/bin/intel-agent
	@echo "  [✓] Uninstalled."

# ─── Test ──────────────────────────────────────────────────────────────────

## test: Run all Go tests
test:
	$(GO) test ./internal/... ./cmd/...
	@echo "  [✓] Tests passed"

## test-race: Run tests with race detector
test-race:
	$(GO) test -race ./internal/... ./cmd/...

## test-cover: Run tests with coverage report
test-cover:
	$(GO) test -coverprofile=coverage.out ./internal/... ./cmd/...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "  [✓] Coverage report: coverage.html"

# ─── Code Quality ──────────────────────────────────────────────────────────

## lint: Run golangci-lint
lint:
	@which golangci-lint > /dev/null || (echo "Install: brew install golangci-lint" && exit 1)
	golangci-lint run --timeout=5m ./...

## fmt: Run gofmt on all packages
fmt:
	$(GO) fmt ./...

## vet: Run go vet
vet:
	$(GO) vet ./internal/... ./cmd/...

# ─── Developer Workflow ────────────────────────────────────────────────────

## api: Build and run the ForgeGuardian API server locally (requires postgres+redis)
api: build-fgctl
	@echo "  Starting API server on :8080 ..."
	@echo "  Tip: run 'make up' first to start postgres and redis"
	PORT=8080 go run ./internal/api/

## up: Start minimal docker stack (postgres + redis + API)
up: docker-minimal
	@echo "  Stack running. API → http://localhost:8080/healthz"

## down: Stop all docker containers
down: docker-down

## logs: Tail API container logs
logs:
	docker compose logs -f forgeguardian-api

## health: Check if the API is healthy
health:
	@curl -sf http://localhost:8080/healthz > /dev/null \
		&& echo "  [PASS] API healthy at http://localhost:8080" \
		|| echo "  [FAIL] API not reachable — run: make up"

# ─── Docker ────────────────────────────────────────────────────────────────

## docker: Start the full local stack (postgres + redis + API)
docker:
	@test -f .env || (echo "  Copy .env.example to .env and set ANTHROPIC_API_KEY" && cp .env.example .env)
	docker compose up -d
	@echo ""
	@echo "  Services:"
	@echo "    API        →  http://localhost:8080/healthz"
	@echo "    Dashboard  →  http://localhost:3000"

## docker-minimal: Start minimal stack (postgres + redis + API only — fastest)
docker-minimal:
	@test -f .env || cp .env.example .env
	docker compose -f docker-compose.minimal.yml up -d
	@echo "  API  →  http://localhost:8080/healthz"

## docker-down: Stop and remove all dev stack containers
docker-down:
	docker compose down -v

## docker-logs: Tail logs from all services
docker-logs:
	docker compose logs -f

# ─── Release ───────────────────────────────────────────────────────────────

## release: Build release binaries for all platforms using goreleaser
release:
	@which goreleaser > /dev/null || (echo "Install: brew install goreleaser" && exit 1)
	goreleaser release --clean

## release-snapshot: Build snapshot release (no git tag required)
release-snapshot:
	@which goreleaser > /dev/null || (echo "Install: brew install goreleaser" && exit 1)
	goreleaser release --snapshot --clean

# ─── Clean ─────────────────────────────────────────────────────────────────

## clean: Remove all build artifacts
clean:
	rm -rf $(BINARY_DIR) dist/ coverage.out coverage.html
	rm -f fgctl fg-agent intel-agent fgctl.exe fg-agent.exe intel-agent.exe
	@echo "  [✓] Cleaned"

# ─── Help ──────────────────────────────────────────────────────────────────

## help: Show this help message
help:
	@echo ""
	@echo "ForgeGuardian — Make Targets"
	@echo "────────────────────────────────────────"
	@grep -E '^## ' Makefile | sed 's/## //' | column -t -s ':'
	@echo ""
