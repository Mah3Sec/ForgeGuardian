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
        dashboard dashboard-dev \
        extension \
        docker docker-minimal docker-dev docker-enterprise docker-down docker-logs \
        api dev up down logs health \
        release smoke-test \
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

## smoke-test: Run the automated smoke test suite against real packages
smoke-test: build
	@echo "Running smoke tests (requires internet + optional API key) ..."
	FGCTL=./$(FGCTL) AGENT=./$(FG_AGENT) bash testenv/run-tests.sh

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

## dev: Start full local dev environment (docker stack + dashboard hot reload)
dev: build
	$(MAKE) up
	@echo "  Starting dashboard dev server ..."
	cd dashboard && npm install --silent && npm run dev

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

# ─── Dashboard ─────────────────────────────────────────────────────────────

## dashboard: Build the React dashboard for production
dashboard:
	@echo "Building dashboard ..."
	cd dashboard && npm install --silent && npm run build
	@echo "  [✓] Dashboard built → dashboard/dist/"

## dashboard-dev: Start the dashboard dev server (hot reload)
dashboard-dev:
	cd dashboard && npm install --silent && npm run dev

# ─── VS Code Extension ─────────────────────────────────────────────────────

## extension: Compile the VS Code extension
extension:
	@echo "Compiling VS Code extension ..."
	cd vscode-extension && npm install --silent && npm run compile 2>/dev/null || \
		npx tsc --outDir out src/*.ts 2>/dev/null || true
	@echo "  [✓] Extension compiled → vscode-extension/out/"

# ─── Docker ────────────────────────────────────────────────────────────────

## docker: Start the full local dev stack
docker:
	@test -f .env || (echo "  Copy .env.example to .env and set ANTHROPIC_API_KEY" && cp .env.example .env)
	docker compose up -d
	@echo ""
	@echo "  Services:"
	@echo "    API        →  http://localhost:8080/healthz"
	@echo "    Grafana    →  http://localhost:3002   (admin / admin)"
	@echo "    MinIO      →  http://localhost:9001   (minioadmin / minioadmin)"
	@echo "    Dep-Track  →  http://localhost:8081"
	@echo "    Dashboard is not a docker service — run 'make dashboard-dev' → http://localhost:3000"

## docker-minimal: Start minimal stack (postgres + redis + API only — fastest)
docker-minimal:
	@test -f .env || cp .env.example .env
	docker compose -f docker-compose.minimal.yml up -d
	@echo "  API  →  http://localhost:8080/healthz"

## docker-dev: Start dev stack (+ MinIO + Prometheus + Grafana)
docker-dev:
	@test -f .env || cp .env.example .env
	docker compose -f docker-compose.dev.yml up -d
	@echo "  API     →  http://localhost:8080/healthz"
	@echo "  Grafana →  http://localhost:3002   (admin / admin)"
	@echo "  MinIO   →  http://localhost:9001   (minioadmin / minioadmin)"

## docker-enterprise: Start full enterprise stack (+ Rekor + Trillian + Dep-Track + intel-agent)
docker-enterprise:
	@test -f .env || cp .env.example .env
	docker compose -f docker-compose.enterprise.yml up -d
	@echo "  API        →  http://localhost:8080/healthz"
	@echo "  Grafana    →  http://localhost:3002"
	@echo "  Rekor      →  http://localhost:3001"
	@echo "  Dep-Track  →  http://localhost:8081"

## docker-down: Stop and remove all dev stack containers
docker-down:
	docker compose down -v

## docker-logs: Tail logs from all services
docker-logs:
	docker compose logs -f

## docker-testenv: Start the vulnerable test environment
docker-testenv:
	docker compose -f testenv/docker-compose.testenv.yml up -d
	@echo "  Vuln webapp  →  http://localhost:8888"
	@echo "  Juice Shop   →  http://localhost:3333"
	@echo "  Python API   →  http://localhost:5555"

## docker-testenv-down: Stop the test environment
docker-testenv-down:
	docker compose -f testenv/docker-compose.testenv.yml down -v

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

## clean-all: Remove build artifacts + dashboard dist + extension out
clean-all: clean
	rm -rf dashboard/dist/ vscode-extension/out/
	@echo "  [✓] Full clean done"

# ─── Help ──────────────────────────────────────────────────────────────────

## help: Show this help message
help:
	@echo ""
	@echo "ForgeGuardian — Make Targets"
	@echo "────────────────────────────────────────"
	@grep -E '^## ' Makefile | sed 's/## //' | column -t -s ':'
	@echo ""
