# ForgeGuardian — Product Modes

> How to configure ForgeGuardian for your use case. Five modes, from a single developer laptop to a fully airgapped enterprise deployment.

---

## Table of Contents

1. [Developer Mode](#1-developer-mode)
2. [CI/CD Mode](#2-cicd-mode)
3. [Monitor Mode](#3-monitor-mode)
4. [Enterprise Mode](#4-enterprise-mode)
5. [Offline / Airgapped Mode](#5-offline--airgapped-mode)

---

## 1. Developer Mode

**One-line description:** Local manifest scanning with instant feedback, fix hints, and VS Code integration.

**Who it's for:** Individual developers who want to catch supply chain issues before they commit — without slowing down their workflow.

### Required Setup

Just the `fgctl` binary. No database, no docker, no API keys.

```bash
# Install
go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest
# or: brew install mah3sec/tap/forgeguardian
# or: curl -sSfL https://raw.githubusercontent.com/mah3sec/forgeguardian/main/install.sh | bash

# Install mode options (installer env vars):
# FORGEGUARDIAN_INSTALL_MODE=local bash install.sh    # build from source
# FORGEGUARDIAN_LOCAL_BUNDLE=/path/bundle.tar.gz bash install.sh  # air-gapped

# First-time setup (validates env, auto-installs missing tools, downloads signatures)
fgctl doctor --fix   # prints each repair command before running it
fgctl update
```

That's it. No config file required. `fgctl scan .` works from any directory with a supported manifest file.

### Key Commands

```bash
fgctl scan .                          # scan all manifests in current directory (recursive)
fgctl scan . --compact                # one line per PACKAGE with severity bracket counts
fgctl scan . --only-fixable           # show only findings with a known fix
fgctl scan . --severity=high          # show only HIGH and CRITICAL findings
fgctl scan . --verbose                # expand all grouped findings (default shows top 3 + grade badge)
fgctl scan . --prod-only              # exclude dev dependencies
fgctl scan . --debug                  # show engine metadata alongside findings
fgctl scan npm/lodash@4.17.20         # scan a specific package
fgctl advisory npm/lodash@4.17.20     # AI advisory (requires ANTHROPIC_API_KEY)
fgctl patch . --project-dir=.         # apply autonomous patches
fgctl doctor                          # check tool health
```

### Example Output

```
  ForgeGuardian v1.3.0

  Scanning /home/alice/myapp...

  package.json  (npm)
  ─────────────────────────────────────────────────────────────────
  lodash@4.17.20       HIGH     CVE-2021-23337
    Prototype Pollution via the merge, mergeWith, defaultsDeep functions.
    Fix: upgrade to 4.17.21

  minimist@1.2.5       MEDIUM   CVE-2021-44906
    Prototype Pollution via the key "__proto__".
    Fix: upgrade to 1.2.6

  ─────────────────────────────────────────────────────────────────
  2 findings  ·  0 CRITICAL  ·  1 HIGH  ·  1 MEDIUM  ·  0 LOW
  Risk Score: D

  Run 'fgctl advisory npm/lodash@4.17.20' for AI remediation.
```

### Configuration Tips

**Set a default severity filter** so noise stays out of your terminal:

```bash
fgctl config set scan.min_severity=medium
```

**Set default workers** if you have a large monorepo:

```bash
fgctl config set scan.workers=8
```

**View your current config:**

```bash
fgctl config show
```

**Reset to defaults:**

```bash
fgctl config init
```

### Git Pre-Commit Hook

Block commits with critical findings by adding this to `.git/hooks/pre-commit`:

```bash
#!/bin/sh
# ForgeGuardian pre-commit hook
if command -v fgctl &>/dev/null; then
  fgctl scan . --quiet --fail-on=critical
  EXIT=$?
  if [ $EXIT -eq 2 ]; then
    echo ""
    echo "ForgeGuardian: CRITICAL vulnerability detected. Commit blocked."
    echo "Run 'fgctl scan .' to see details, or 'fgctl advisory <pkg>' for remediation."
    echo ""
    exit 1
  fi
fi
```

Make it executable:
```bash
chmod +x .git/hooks/pre-commit
```

To skip the hook in an emergency: `git commit --no-verify` (use sparingly).

---

## 2. CI/CD Mode

**One-line description:** Policy enforcement gate with SARIF output and exit codes for GitHub Actions, GitLab CI, Jenkins, and any other CI system.

**Who it's for:** DevOps and platform engineers who own the CI pipeline and want to block merges that introduce high-severity vulnerabilities.

### Required Setup

Install `fgctl` in your CI runner. No persistent services required.

```bash
# In your CI job:
go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest
fgctl update     # fetch latest signatures (cached between runs)
```

Optionally set `ANTHROPIC_API_KEY` as a CI secret for AI advisory features in the scan summary.

### Key Commands

```bash
# Single-flag CI shortcut (equivalent to: --quiet --format=sarif --fail-on=high)
# v1.3.0: produces pure stdout SARIF — zero human text, no banner contamination
fgctl scan . --ci

# Standard policy gate (exits 2 on HIGH or above)
fgctl scan . --fail-on=high

# SARIF output for GitHub Code Scanning
# v1.3.0: pure SARIF 2.1.0 on stdout — no banner or ANSI codes leak into the file
fgctl scan . --format=sarif --fail-on=high > forgeguardian.sarif

# JSON output for downstream tooling
# v1.3.0: pure JSON on stdout — pipe directly to jq with no stderr redirect needed
fgctl scan . --format=json 2>/dev/null | jq .

# SBOM generation (CycloneDX JSON) — banner automatically suppressed (machine-readable output)
fgctl sbom . > sbom.cyclonedx.json

# Compact output for readable CI logs (one line per PACKAGE with severity bracket counts)
fgctl scan . --compact --fail-on=high

# Summary only (fastest CI output)
fgctl scan . --summary --fail-on=high

# Specific ecosystem only (e.g., only scan npm in a monorepo job)
fgctl scan . --ecosystem=npm --fail-on=high

# Silent mode: exit code only, no output (for simple pass/fail gates)
fgctl scan . --quiet --fail-on=critical
```

### Exit Codes

| Code | Meaning | CI Behavior |
|---|---|---|
| 0 | Clean — no findings at `--fail-on` threshold | Allow merge |
| 1 | Scan error (tool failure, network error) | Investigate CI failure |
| 2 | Policy violation — findings at or above threshold | Block merge |

### GitHub Actions — Full Example

```yaml
name: Supply Chain Security

on:
  pull_request:
  push:
    branches: [main]

permissions:
  security-events: write
  contents: read

jobs:
  supply-chain:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"

      - name: Cache ForgeGuardian signature database
        uses: actions/cache@v4
        with:
          path: ~/.forgeguardian
          key: forgeguardian-sigs-${{ github.run_id }}
          restore-keys: forgeguardian-sigs-

      - name: Install ForgeGuardian
        run: go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest

      - name: Update signatures
        run: fgctl update

      - name: Scan for vulnerabilities (SARIF)
        # v1.3.0: --format=sarif produces pure stdout SARIF — no banner contamination.
        # Alternatively: fgctl scan . --ci (sets --quiet --format=sarif --fail-on=high automatically)
        run: |
          fgctl scan . \
            --format=sarif \
            --fail-on=high \
            --timeout=15m \
            > forgeguardian.sarif
        continue-on-error: true

      - name: Upload results to GitHub Code Scanning
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: forgeguardian.sarif

      - name: Enforce policy gate
        run: fgctl scan . --quiet --fail-on=high
```

### GitLab CI — Example

```yaml
forgeguardian-scan:
  stage: security
  image: golang:1.23-alpine
  before_script:
    - apk add --no-cache git
    - go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest
    - fgctl update
  script:
    # v1.3.0: --format=sarif produces pure stdout SARIF with no banner contamination
    - fgctl scan . --format=sarif --fail-on=high > gl-sast-report.sarif
  artifacts:
    when: always
    reports:
      sast: gl-sast-report.sarif
  allow_failure: false
```

### Jenkins — Pipeline Snippet

```groovy
stage('Supply Chain Scan') {
    steps {
        sh 'go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest'
        sh 'fgctl update'
        sh 'fgctl scan . --format=sarif --fail-on=high > forgeguardian.sarif || true'
        recordIssues(tools: [sarif(pattern: 'forgeguardian.sarif')])
        sh 'fgctl scan . --quiet --fail-on=high'
    }
}
```

### Configuration Tips

**Set policy via environment variables** (useful for CI where you can't write config files):

```bash
export FORGEGUARDIAN_FAIL_ON=high
export FORGEGUARDIAN_MIN_SEVERITY=medium
fgctl scan .
```

**Use `--timeout` for large monorepos** to prevent CI hangs:

```bash
fgctl scan . --fail-on=high --timeout=20m
```

**Cache the signature database** between CI runs to avoid re-downloading on every build:
```yaml
# GitHub Actions
- uses: actions/cache@v4
  with:
    path: ~/.forgeguardian
    key: forgeguardian-${{ github.run_id }}
    restore-keys: forgeguardian-
```

---

## 3. Monitor Mode

**One-line description:** Continuous manifest watching that diffs findings on every change and alerts on new vulnerabilities.

**Who it's for:** Developers who want persistent background monitoring, DevOps engineers watching production dependency files, and SOC analysts tracking dependency changes in real time.

### Required Setup

Same as Developer Mode — just `fgctl`. No persistent services required for basic monitoring.

For webhook notifications, add `notifications:` to `~/.forgeguardian/config.yaml`.

### Key Commands

```bash
# Watch the current directory (polls every 3s by default)
fgctl monitor --watch .

# Watch a specific directory with a custom interval
fgctl monitor --watch /path/to/project --interval=10s

# Watch with more workers for large projects
fgctl monitor --watch . --workers=8

# One-shot scan without watching (alias for fgctl scan .)
fgctl monitor .
```

### Example Output

```
  ForgeGuardian v1.3.0

  Watching /home/alice/myapp for manifest changes (poll interval: 3s)...
  Press Ctrl+C to stop.

  [14:02:01] Initial scan: 0 finding(s)
  [14:03:45] package.json changed — rescanning...
  [14:03:47] NEW [CRITICAL]  event-stream@3.3.6 — Known supply chain attack (blocklisted)
  [14:03:47] 1 new, 0 resolved
  [14:07:12] package.json changed — rescanning...
  [14:07:14] RESOLVED  event-stream@3.3.6 — Known supply chain attack (blocklisted)
  [14:07:14] 0 new, 1 resolved
```

### Webhook Notification Configuration

Add to `~/.forgeguardian/config.yaml`:

```yaml
notifications:
  slack_webhook: https://hooks.slack.com/services/T.../B.../xxx
  discord_webhook: https://discord.com/api/webhooks/.../xxx
  min_severity: high     # only notify for HIGH and CRITICAL
  include_resolved: false  # don't notify when findings are resolved
```

Slack notification format:
```
[CRITICAL] ForgeGuardian Alert — /home/alice/myapp
Package: event-stream@3.3.6
Finding: Known supply chain attack (blocklisted)
Ecosystem: npm
Time: 2026-05-24 14:03:47
```

### Running as a Background Service

**systemd (Linux):**

```ini
# /etc/systemd/system/forgeguardian-monitor.service
[Unit]
Description=ForgeGuardian Supply Chain Monitor
After=network.target

[Service]
Type=simple
User=alice
ExecStart=/usr/local/bin/fgctl monitor --watch /home/alice/projects --interval=30s
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable forgeguardian-monitor
sudo systemctl start forgeguardian-monitor
```

**launchd (macOS):**

```xml
<!-- ~/Library/LaunchAgents/dev.forgeguardian.monitor.plist -->
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC ...>
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>dev.forgeguardian.monitor</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/fgctl</string>
    <string>monitor</string>
    <string>--watch</string>
    <string>/Users/alice/projects</string>
    <string>--interval=30s</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
</dict>
</plist>
```

### Configuration Tips

- Set `--interval` to at least 10s in production to avoid excessive CPU usage on large directories
- Use `--workers` to control concurrency during rescans
- The monitor compares findings by `ecosystem/package@version:findingID` — it only reports genuinely new findings, not the same finding re-detected

---

## 4. Enterprise Mode

**One-line description:** Full platform with web dashboard, persistent storage, metrics, and Dependency-Track integration.

**Who it's for:** Security teams managing supply chain risk across multiple projects, organizations that need centralized visibility, and teams with compliance requirements.

### Required Setup

Docker and Docker Compose (v2).

```bash
git clone https://github.com/mah3sec/forgeguardian
cd forgeguardian

# Minimal stack: API + postgres + redis + minio
make up

# Full enterprise stack: adds Rekor, Dep-Track, prometheus, grafana
make docker-enterprise
```

### Stack Components

| Service | Port | Purpose |
|---|---|---|
| ForgeGuardian API | 8080 | REST + GraphQL API |
| ForgeGuardian Worker | — | Background build/scan jobs |
| React Dashboard | 5173 | Web UI |
| PostgreSQL | 5432 | Scan results + metadata |
| Redis | 6379 | Job queue |
| MinIO | 9000 / 9001 | Artifact store (SBOMs, attestations) |
| Rekor Server | 3001 | Local Sigstore transparency log |
| Dependency-Track | 8081 | Continuous SBOM monitoring |
| Prometheus | 9090 | Metrics collection |
| Grafana | 3002 | Observability dashboards |

### Key Commands

```bash
make up                     # start minimal stack (postgres, redis, minio, API)
make docker-enterprise      # start full enterprise stack
make down                   # stop all services
make logs                   # tail all service logs
make health                 # check all service health endpoints

# Access the web dashboard
open http://localhost:5173

# Point fgctl at the local API
fgctl config set api_url=http://localhost:8080

# Run scans that persist results to the API
fgctl scan npm/lodash@4.17.20   # results stored in postgres
```

### Dashboard Pages

| Page | URL | Purpose |
|---|---|---|
| Dashboard | `/` | Overview: timeline, risk scores, ecosystem breakdown |
| Scan | `/scan` | Run scans, view results |
| Advisory | `/advisory` | AI advisory generation |
| SBOM | `/sbom` | SBOM generation and download |
| Sign / Verify | `/sign` | Artifact signing and verification |
| Monitor | `/monitor` | Live scan results, polling |
| Intelligence | `/intelligence` | Community signature viewer |
| Risks | `/risks` | Risk heatmap and portfolio-level risk review |
| Inventory | `/inventory` | Full package inventory across all scanned projects |
| Policy | `/policy` | Policy-as-code viewer and status |

### Environment Variables

Create a `.env` file (the template is at `.env.example`):

```bash
# Required
DATABASE_URL=postgres://forgeguardian:devpassword@localhost:5432/forgeguardian
REDIS_URL=redis://localhost:6379
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin

# Optional — enables AI features
ANTHROPIC_API_KEY=sk-ant-...

# Optional — use local Rekor instead of sigstore.dev
REKOR_URL=http://localhost:3001
```

### Grafana Dashboards

Access Grafana at `http://localhost:3002` (default credentials: `admin` / `admin`, configurable via `GRAFANA_PASSWORD`).

Pre-built dashboards show:
- Scan throughput over time
- Finding severity distribution
- API response times
- Worker job queue depth
- Signature database freshness

### Configuration Tips

**For production deployment behind a reverse proxy:**

```bash
# Set the external base URL so the dashboard API calls route correctly
FORGEGUARDIAN_EXTERNAL_URL=https://security.yourcompany.com
```

**For Kubernetes deployment:**

```bash
kubectl apply -k infra/k8s/base          # base resources
kubectl apply -k infra/k8s/overlays/prod  # production overlay
```

See `infra/` for Terraform (EKS + RDS + S3) and Kubernetes (Kustomize base + prod overlay) configurations.

---

## 5. Offline / Airgapped Mode

**One-line description:** Full local scan capability with no internet access — after a one-time database pre-cache.

**Who it's for:** Security-sensitive environments, classified networks, development machines with restricted internet access, and teams with strict data residency requirements.

### What Works Offline

Once databases are pre-cached, the following features work without any internet connection:

| Feature | Works Offline | Notes |
|---|---|---|
| `fgctl scan .` | Yes | Uses cached vulnerability DBs |
| Behavioral / malware / MCP scan | Yes | All pattern matching is local |
| Community signatures | Yes | Cached at `~/.forgeguardian/signatures.json` |
| SBOM generation | Yes | No network calls |
| Provenance generation | Yes | No network calls |
| Monitor mode | Yes | No network calls |
| System audit | Yes | No network calls |
| Web dashboard (Enterprise Mode) | Yes | Reads from local postgres |
| AI advisory / patch | No | Requires Anthropic API |
| `fgctl sign` (public Rekor) | No | Requires rekor.sigstore.dev |
| `fgctl sign` (self-hosted Rekor) | Yes | Configure `signing.rekor_url` |
| `fgctl update` | No | Requires GitHub |
| Package scan by name | Partial | OSV query fails; Grype/Trivy use local DBs |

### Pre-Caching for Airgapped Deployment

Run this on a machine with internet access before entering the airgapped environment:

```bash
# 1. Install ForgeGuardian
go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest

# 2. Download community signatures
fgctl update
# Signatures saved to: ~/.forgeguardian/signatures.json

# 3. Update Grype's vulnerability database
grype db update
# Cached at: ~/.cache/grype/db/

# 4. Update Trivy's vulnerability database
trivy image --download-db-only --no-progress
# Cached at: ~/.cache/trivy/

# 5. Package everything for transport
tar czf forgeguardian-offline-bundle.tar.gz \
  ~/.forgeguardian/ \
  ~/.cache/grype/ \
  ~/.cache/trivy/ \
  $(which fgctl)
```

On the airgapped machine, extract the bundle and set the cache paths:

```bash
tar xzf forgeguardian-offline-bundle.tar.gz -C ~/

# Verify offline scan works
fgctl scan .   # should work with no internet
```

### Self-Hosted Rekor for Offline Signing

If you need artifact signing in an airgapped environment, run a local Rekor instance:

```bash
# Start local Rekor (included in the enterprise docker-compose profile)
docker compose -f docker-compose.enterprise.yml up rekor-server -d

# Configure fgctl to use it
fgctl config set signing.rekor_url=http://localhost:3001

# Sign and verify using local Rekor
fgctl sign --recipe=npm --package=lodash --version=4.17.21 --out=att.json
fgctl verify --attestation=att.json --sha256=<hash>
```

Local Rekor uses Trillian + MySQL as the backend, structurally identical to the public sigstore.dev infrastructure.

### Offline Signature Bundle Preparation

For environments where `fgctl update` cannot reach GitHub, prepare a signature bundle offline:

```bash
# On internet-connected machine: export current signatures
cp ~/.forgeguardian/signatures.json ./signatures-bundle.json

# Transfer signatures-bundle.json to airgapped machine via approved media

# On airgapped machine: import
cp ./signatures-bundle.json ~/.forgeguardian/signatures.json

# Verify signature count after bundle install
fgctl stats            # human-readable signature runtime statistics
fgctl stats --json     # machine-readable for scripting / health checks
fgctl debug | grep Signatures   # alternative: grep from full debug dump
```

### Configuration for Airgapped Operation

```yaml
# ~/.forgeguardian/config.yaml
api_url: http://internal-forgeguardian-server:8080   # local API server if running enterprise
scan:
  workers: 8
  timeout: 30m
signing:
  rekor_url: http://internal-rekor:3001   # local Rekor instance
```

### Airgapped Enterprise Stack

The full enterprise stack can run completely offline after the initial image pull:

```bash
# On internet-connected machine: pull and save all images
docker compose -f docker-compose.enterprise.yml pull
docker save $(docker compose -f docker-compose.enterprise.yml config --images | tr '\n' ' ') \
  | gzip > forgeguardian-images.tar.gz

# Transfer to airgapped machine via approved media

# On airgapped machine: load images and start
docker load < forgeguardian-images.tar.gz
docker compose -f docker-compose.enterprise.yml up -d
```

After this, the full enterprise stack (API, dashboard, postgres, redis, minio, Rekor, Dep-Track, Prometheus, Grafana) runs without any external network access.

### Configuration Tips

- Re-run the pre-cache procedure regularly (weekly or monthly) to keep vulnerability databases current
- For very restricted environments, consider running a ForgeGuardian enterprise instance on a DMZ server that has limited internet access for database updates, while developer workstations connect to it via the internal API
- The `fgctl debug` command shows the age of your local signature and vulnerability databases — use it to check if an update is overdue
