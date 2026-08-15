# ForgeGuardian — Complete Documentation

> Local-first AI-native Software Supply Chain Security Platform  
> Version 2.0.0 · June 2026

---

## Table of Contents

1. [Overview](#1-overview)
2. [Prerequisites](#2-prerequisites)
3. [Installation](#3-installation)
4. [Configuration](#4-configuration)
5. [CLI Quick Reference — fgctl](#5-cli-quick-reference--fgctl)
6. [CLI Quick Reference — fg-agent](#6-cli-quick-reference--fg-agent)
7. [CLI Quick Reference — intel-agent](#7-cli-quick-reference--intel-agent)
8. [Local Project Scanning](#8-local-project-scanning)
9. [Scan Output Modes](#9-scan-output-modes)
10. [Inline Remediation Hints](#10-inline-remediation-hints)
11. [Monitor Mode](#11-monitor-mode)
12. [System Audit Mode](#12-system-audit-mode)
13. [SARIF Output for CI](#13-sarif-output-for-ci)
14. [Doctor / Self-diagnostic Mode](#14-doctor--self-diagnostic-mode)
15. [Debug Command](#15-debug-command)
16. [Config Command](#16-config-command)
17. [Web Dashboard](#17-web-dashboard)
18. [VS Code Extension](#18-vs-code-extension)
19. [Docker Profiles](#19-docker-profiles)
20. [Test Environments](#20-test-environments)
21. [Vulnerable Packages for Testing](#21-vulnerable-packages-for-testing)
22. [Full Walkthrough Examples](#22-full-walkthrough-examples)
23. [Troubleshooting](#23-troubleshooting)
24. [Policy-as-Code](#24-policy-as-code)
25. [Risk Score](#25-risk-score)
26. [Webhook Notifications](#26-webhook-notifications)
27. [Product Modes](#27-product-modes)
28. [Trust & Privacy](#28-trust--privacy)
29. [Competitive Landscape](#29-competitive-landscape)

---

## 1. Overview

ForgeGuardian is a CLI-first supply chain security platform that covers:

| Capability | What it does |
|-----------|-------------|
| **Local project scan** | `fgctl scan .` — finds all manifests, scans every pinned dep |
| **Remote package scan** | `fgctl scan npm/lodash@4.17.20` — dot-notation or explicit flags |
| **Inline fix hints** | Every finding shows the exact safe version to upgrade to |
| **Output modes** | `--compact`, `--summary`, `--quiet`, `--severity`, `--only-fixable` |
| **SARIF export** | `--format=sarif` for GitHub Code Scanning / CI integration |
| **Live monitor** | `fgctl monitor --watch .` — re-scans on manifest change, diffs findings |
| **System audit** | `fgctl audit system` — scans npm globals, pip globals, cargo bins, Go bins |
| **Self-diagnostic** | `fgctl doctor` — checks tools, API key, signatures, disk space |
| **Debug report** | `fgctl debug` — collects diagnostics for bug reports |
| **Config system** | `fgctl config show/set/init` — `~/.forgeguardian/config.yaml` |
| **Hermetic build** | Downloads + SHA256-verifies packages with zero network after fetch |
| **Multi-engine scan** | Grype + OSV + Semgrep + Trivy + Behavioral + Malware + AI model + MCP |
| **AI advisory** | Claude generates plain-English advisories with agentic risk scoring |
| **Autonomous patch** | Multi-turn AI tool-use loop reads manifests and proposes/applies upgrades |
| **SBOM** | CycloneDX 1.5 JSON/XML + SPDX 2.3 JSON/TV with AI/MCP extensions |
| **Sigstore signing** | Ephemeral ECDSA + Rekor transparency log + SLSA v1.0 provenance |
| **Continuous monitor** | Dependency-Track integration for ongoing CVE alerts |
| **Community signatures** | Community-format detection signatures for known malicious packages and patterns |

**Supported ecosystems:** npm · PyPI · Maven · Go · RubyGems · crates.io · HuggingFace · MCP servers · OCI

---

## 2. Prerequisites

### Required

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.23+ | https://go.dev/dl/ |
| Git | any | system package manager |
| Internet access | — | package downloads + OSV API |

### Optional (enhances scan coverage)

| Tool | Purpose | Install |
|------|---------|---------|
| `grype` | CVE scanning (Grype DB) | `brew install anchore/grype/grype` |
| `semgrep` | Static analysis | `pip install semgrep` or `brew install semgrep` |
| `trivy` | CVE + misconfiguration scanning | `brew install trivy` |

When optional tools are not installed, those scan engines are skipped and a single consolidated warning is shown at the top of the output — not once per package. Run `fgctl doctor` to see what's missing.

### For AI features

| Requirement | Details |
|------------|---------|
| Anthropic API key | https://console.anthropic.com — required for `advisory`, `fg-agent`, `intel-agent` |

---

## 3. Installation

### Go install (recommended)

```bash
go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest
go install github.com/mah3sec/forgeguardian/cmd/fg-agent@latest
go install github.com/mah3sec/forgeguardian/cmd/intel-agent@latest
```

### Homebrew

```bash
brew tap mah3sec/forgeguardian
brew install forgeguardian
```

### Curl installer

```bash
curl -sSfL https://raw.githubusercontent.com/mah3sec/forgeguardian/main/install.sh | bash
```

Optional env vars for the installer:
```bash
FORGEGUARDIAN_VERSION=v0.1.0 bash install.sh   # pin a version
INSTALL_DIR=$HOME/.local/bin bash install.sh    # custom install directory
FORGEGUARDIAN_INSTALL_MODE=full bash install.sh # set install mode (see below)
FORGEGUARDIAN_LOCAL_BUNDLE=/path/to/bundle.tar.gz bash install.sh  # air-gapped CI
```

**Install modes** (`FORGEGUARDIAN_INSTALL_MODE`):

| Mode | Behaviour |
|------|-----------|
| `auto` *(default)* | Tries GitHub release download first; falls back to local source build on network failure (5s probe timeout) |
| `local` | Builds from source using local Go toolchain (`go build`) |
| `offline` | Uses pre-built binaries from `./bin/` — no network required |
| `minimal` | Installs `fgctl` only (no agent binaries) |
| `full` | Installs all binaries + Cosign signatures + npm dashboard dependencies |
| `dev` | `full` + starts docker compose dev stack after install |

All curl calls in the installer include retry logic (3 attempts) and a 30s timeout. Network failures produce a clear error message and fall back gracefully.

### Build from source

```bash
git clone https://github.com/mah3sec/forgeguardian
cd forgeguardian
bash scripts/bootstrap.sh   # validates Go ≥1.23, Node ≥20, optional tools; runs go build + npm ci
make build                  # → bin/fgctl  bin/fg-agent  bin/intel-agent
./bin/fgctl doctor
```

### First run — download signatures

```bash
fgctl update
# Updated: +24 signatures (total: 24)
```

This downloads the latest community detection signatures. Run it before your first scan and regularly to stay current. Running `fgctl` from inside a git clone of this repo already loads the same 24 signatures automatically from `signatures/` — `fgctl update` is for picking up newer ones published after your clone.

---

## 4. Configuration

### Environment variables

| Variable | Required | Description |
|----------|----------|-------------|
| `ANTHROPIC_API_KEY` | For AI features | Anthropic API key |
| `DATABASE_URL` | For server mode | PostgreSQL connection string |
| `REDIS_URL` | For server mode | Redis connection string |
| `MINIO_ENDPOINT` | For server mode | MinIO host:port |
| `MINIO_ACCESS_KEY` | For server mode | MinIO access key (default: `minioadmin`) |
| `MINIO_SECRET_KEY` | For server mode | MinIO secret key (default: `minioadmin`) |
| `FORGEGUARDIAN_API_URL` | Optional | API URL override (default: `http://localhost:8080`) |
| `NO_COLOR` | Optional | Disable ANSI color output |

Set for current session:
```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

### Config file

ForgeGuardian stores persistent config in `~/.forgeguardian/config.yaml`. Create it with:

```bash
fgctl config init
```

Default contents:
```yaml
api_url: http://localhost:8080
scan:
  workers: 4
  timeout: 10m
  fail_on: ""
  min_severity: ""
signing:
  rekor_url: ""
```

Manage with:
```bash
fgctl config show
fgctl config set scan.workers=8
fgctl config set scan.fail_on=high
fgctl config set api_url=http://my-api:8080
```

Valid config keys: `api_url` · `scan.workers` · `scan.timeout` · `scan.fail_on` · `scan.min_severity` · `signing.rekor_url`

### Local data directory

```
~/.forgeguardian/
├── signatures.json    ← community detection signatures (fgctl update writes here)
├── config.yaml        ← user config (fgctl config init writes here)
└── .initialized       ← marker file, set after first run
```

---

## 5. CLI Quick Reference — fgctl

### Scan

```bash
# Local project scan (finds all manifests recursively)
fgctl scan .
fgctl scan ./my-project

# Dot-notation (ecosystem/package@version)
fgctl scan npm/lodash@4.17.20
fgctl scan pypi/PyYAML@5.3.1
fgctl scan maven/org.apache.logging.log4j:log4j-core@2.14.1

# Explicit flags
fgctl scan --recipe=npm --package=lodash --version=4.17.20

# Output modes
fgctl scan . --compact                # one line per finding
fgctl scan . --summary                # summary table only
fgctl scan . --quiet                  # no output (CI — check exit code)

# Filtering
fgctl scan . --severity=high          # only HIGH+ findings
fgctl scan . --ecosystem=npm          # only npm packages
fgctl scan . --only-fixable           # only findings with a known fix version

# CI flags
fgctl scan . --fail-on=high           # exit 2 if any HIGH+ finding
fgctl scan . --format=sarif           # SARIF 2.1.0 output
fgctl scan . --format=json            # JSON output

# Performance
fgctl scan . --workers=8 --timeout=5m
```

### Patch

```bash
fgctl patch .                          # delegates to fg-agent --apply
fgctl patch . npm/lodash@4.17.20       # patch one specific package
```

### Monitor

```bash
fgctl monitor .                        # one-shot scan and exit
fgctl monitor --watch .                # watch for manifest changes, re-scan on change
fgctl monitor --watch . --interval=5s  # custom poll interval
```

### Audit

```bash
fgctl audit system    # scan all globally installed packages
```

### Doctor

```bash
fgctl doctor
```

### Debug

```bash
fgctl debug           # diagnostic report (human-readable)
fgctl debug --json    # JSON output for tooling
```

### Config

```bash
fgctl config show
fgctl config set key=value
fgctl config init
```

### Advisory (requires API key)

```bash
export ANTHROPIC_API_KEY=sk-ant-...
fgctl advisory npm/lodash@4.17.20
fgctl advisory npm/lodash@4.17.20 --json
```

### SBOM

```bash
fgctl sbom npm/chalk@5.3.0
fgctl sbom npm/chalk@5.3.0 --format=spdx-json --out=chalk.spdx.json
```

SBOM formats: `cyclonedx-json` (default) · `cyclonedx-xml` · `spdx-json` · `spdx-tv`

> **Banner suppression:** `fgctl sbom .` (and any `fgctl sbom` invocation without `--out`) automatically suppresses the ASCII banner because SBOM output is machine-readable JSON. The banner is only printed when `--out <file>` is present (output goes to a file, not stdout).

### Sign + Verify

```bash
# Sign (produces attestation.json)
fgctl sign npm/chalk@5.3.0 --out=chalk.att.json

# Verify
SHA=$(jq -r '.sha256' chalk.att.json)
fgctl verify --attestation=chalk.att.json --sha256=$SHA
```

### Update signatures

```bash
fgctl update                          # alias for: fgctl intel update
fgctl intel update                    # pull latest community signatures
fgctl intel list                      # list loaded signatures
fgctl intel list --type=malware_pattern --ecosystem=npm
```

### Community signature toolkit — fgctl intel

```bash
# Create a new signature (interactive wizard — 5 questions, writes YAML)
fgctl intel new
fgctl intel new --out=./FG-npm-my-sig.yaml

# Validate a signature YAML (schema + regex + required fields)
fgctl intel validate ./FG-npm-my-sig.yaml
fgctl intel validate signatures/**/*.yaml   # glob — validate all

# Test a signature against a live package
fgctl intel test ./FG-npm-my-sig.yaml \
  --ecosystem=npm --package=evil-pkg --version=1.0.0
# Downloads real package, extracts, runs relevant engines, shows MATCH/NO MATCH

# List loaded signatures
fgctl intel list
fgctl intel list --type=blocklisted_package
fgctl intel list --ecosystem=pypi

# Pull community bundle
fgctl intel update
fgctl intel update --url=https://your-mirror.example.com/signatures.json
```

Signature types:

| Type | Required field | What it catches |
|---|---|---|
| `blocklisted_package` | `package:` | Confirmed malicious package versions |
| `typosquatting_target` | `target:` | Popular packages to monitor for name-squatting |
| `behavioral_rule` | `rule:` | Dangerous install script patterns |
| `malware_pattern` | `pattern:` | Regex matching malicious code |
| `mcp_injection_pattern` | `pattern:` | Prompt injection in MCP tool descriptions |
| `pickle_rule` | `rule:` | Unsafe AI model configurations |

### Validate a community signature (legacy)

```bash
fgctl sig validate ./my-signature.json   # legacy JSON format still supported
```

### Version

```bash
fgctl version
# ForgeGuardian v0.1.0 (commit: abc1234, built: 2026-05-24T10:00:00Z)
```

### Help

```bash
fgctl help
fgctl help scan
```

---

## 6. CLI Quick Reference — fg-agent

The autonomous patch agent uses a multi-turn Claude tool-use loop to read manifest files and propose (or apply) version upgrades.

```bash
# Dry run — shows proposed changes, writes nothing
fg-agent --recipe=npm --package=lodash --version=4.17.20 \
  --project-dir=./my-project

# Apply changes to manifest files
fg-agent --recipe=npm --package=lodash --version=4.17.20 \
  --project-dir=./my-project --apply

# JSON output
fg-agent --recipe=npm --package=lodash --version=4.17.20 --json
```

**Key flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--apply` | false | Write changes to manifest files |
| `--project-dir` | `.` | Directory containing manifests |
| `--max-turns` | 10 | Max Claude tool-use turns |
| `--api-key` | `$ANTHROPIC_API_KEY` | Anthropic API key |

**What the agent does:**
1. Downloads and SHA256-verifies the package
2. Runs all scan engines (8 in parallel)
3. Calls Claude to generate a security advisory
4. Multi-turn tool-use loop: reads your manifests, plans the safest upgrade
5. Claude reviewer approves the plan
6. If `--apply`: writes file changes

---

## 7. CLI Quick Reference — intel-agent

Polls OSV, OpenSSF malicious-packages, and npm/PyPI popularity feeds. Uses Claude to generate community detection signatures and writes them to the local signature store.

```bash
# One-shot run
intel-agent --ecosystems=npm,pypi,go

# Continuous daemon (default in enterprise docker-compose)
intel-agent --loop --interval=6h

# Dry run (print without saving)
intel-agent --dry-run --verbose

# Skip AI generation (feeds only)
intel-agent --skip-ai
```

---

## 8. Local Project Scanning

`fgctl scan .` is the recommended workflow for scanning your own codebase.

### What it scans

| Manifest file | Ecosystem |
|--------------|-----------|
| `package.json` | npm (dependencies + devDependencies + peerDependencies) |
| `requirements.txt` | pypi |
| `pyproject.toml` | pypi |
| `go.mod` | go |
| `Cargo.toml` | crates |
| `pom.xml` | maven |
| `Gemfile` | rubygems |

### Skipped directories

`node_modules` · `vendor` · `.git` · `__pycache__` · `target` · `.tox` · `dist` · `build`

### Version handling

- Pinned versions (e.g. `"lodash": "4.17.20"`) → scanned
- Range-only (e.g. `"requests>=2.26"`) → shown as `[SKIP]` with original string
- Prefix stripped (e.g. `"^4.17.21"` → scanned as `4.17.21`)

### Missing tool deduplication

When optional scan engines (grype, semgrep, trivy) are not installed, a **single** consolidated warning is printed at the top of the output:

```
[WARN] 2 scan engine(s) unavailable: grype, semgrep
       Install missing tools for better coverage — run: fgctl doctor
```

This replaces the old behavior of repeating the warning for every scanned package.

### Example output

```
ForgeGuardian — Local Project Scan
  Project: /Users/alice/myapp

  [WARN] 1 scan engine(s) unavailable: grype
         Install missing tools for better coverage — run: fgctl doctor

  package.json (npm)
    [CRITICAL] lodash@4.17.20   CVE-2021-23337 — Command injection via template()
               → Fix: upgrade to 4.17.21   (run: fgctl patch . to apply)
    [HIGH]     axios@0.21.1     GHSA-42xw-2xvc — SSRF vulnerability
               → Fix: upgrade to 0.21.2
    [SKIP]     react@^18.0.0    version range — install to pin

  go.mod (go)
    [MEDIUM]   github.com/gin-gonic/gin@1.6.3   GHSA-... — HTTP smuggling
               → Fix: upgrade to 1.9.0
    [LOW]      golang.org/x/crypto@0.0.0-...    CVE-2020-29652

  Summary: 47 packages scanned · 3 findings (1 CRITICAL · 1 HIGH · 1 MEDIUM)
  Run 'fgctl advisory npm/lodash@4.17.20' for AI remediation advice.
```

### Concurrency

Local scans run packages concurrently. Default workers: 4. Override with `--workers=8`.

---

## 9. Scan Output Modes

| Flag | Description |
|------|-------------|
| *(default)* | Full output: banner, per-manifest findings with descriptions |
| `--compact` | One line per package with grouped severity counts: `axios@1.3.4  — 19 findings [CRIT:1 HIGH:1 MED:17]  fix: >=1.12.0` |
| `--summary` | Summary severity table only — no individual findings |
| `--quiet` | No output at all; use exit code to determine pass/fail |
| `--severity=high` | Filter: only show HIGH and CRITICAL findings. Note: unknown/empty severity values are always excluded when this flag is set (see §9 note below) |
| `--ecosystem=npm` | Filter: only packages from the specified ecosystem (local scan) |
| `--only-fixable` | Filter: only findings where a fix version is known; also excludes INFORMATIONAL findings |
| `--verbose` | Expand all grouped findings (default: top 3 per package) |
| `--debug` | Show engine errors and raw scan metadata |
| `--prod-only` | Exclude devDependencies from local project scan |
| `--exclude-dev` | Alias for `--prod-only` |
| `--ci` | CI mode shortcut: `--quiet --format=sarif --fail-on=high` combined |
| `--executive` | Executive summary: severity table only |
| `--no-banner` | Suppress the ASCII logo banner |
| `--no-color` | Disable ANSI color output |
| `--fail-on=high` | Exit code 2 if any finding meets or exceeds this severity |
| `--format=sarif` | SARIF 2.1.0 JSON — for GitHub Code Scanning |
| `--format=json` | Machine-readable JSON with full finding metadata |

Compact mode example:
```
lodash@4.17.20  — 2 findings [CRIT:1 HIGH:1]  fix: >=4.17.21
axios@0.21.1    — 1 finding  [HIGH:1]          fix: >=0.21.2
Summary: 47 scanned · 3 findings (1 CRITICAL · 1 HIGH · 1 MEDIUM)
```

> **Note — `--severity` filtering:** `severityOrd()` returns `-1` for unknown or empty severity values. This means findings with no recognized severity level are always excluded when `--severity` is set, rather than accidentally passing through. If you need to see all findings including INFORMATIONAL, omit the `--severity` flag or use `--severity=informational` explicitly.

---

## 10. Inline Remediation Hints

Every finding that has a known safe version shows the upgrade target inline:

```
[CRITICAL] lodash@4.17.20   CVE-2021-23337 — Command injection via template()
           → Fix: upgrade to 4.17.21   (run: fgctl patch . to apply)
```

The fix version is sourced from:
- **OSV API** — `affected[].ranges[].events[].fixed`
- **Grype** — `vulnerability.fix.versions[]`

If no fix is available (zero-day), the hint is omitted. Use `--only-fixable` to hide unfixed findings in CI.

---

## 11. Monitor Mode

Watch a project directory for manifest changes and re-scan automatically:

```bash
fgctl monitor --watch .
fgctl monitor --watch /path/to/project --interval=5s --workers=8
```

**How it works:**
- Polls manifest file `mtime` at the configured interval (default: 3s)
- On change: re-runs a full local scan
- Diffs findings against the previous scan
- Prints `NEW` (red) or `RESOLVED` (green) for each changed finding, with a `[HH:MM:SS]` timestamp and the affected package name on every diff line

```
  Watching /Users/alice/myapp for manifest changes (poll interval: 3s)...
  Press Ctrl+C to stop.

  [14:32:01] Initial scan: 2 finding(s)
  [14:32:15] package.json changed — rescanning...
  [14:32:17] NEW     [14:32:17] express@4.19.2  [HIGH] CVE-2024-29041 — Open redirect
  [14:32:17] 1 new, 0 resolved
  [14:35:40] package.json changed — rescanning...
  [14:35:42] RESOLVED [14:35:42] express@4.19.2  CVE-2024-29041 — Open redirect
  [14:35:42] 0 new, 1 resolved
```

Without `--watch`, `fgctl monitor .` runs a one-shot scan and exits (same as `fgctl scan .`).

---

## 12. System Audit Mode

Enumerate and scan all globally installed packages:

```bash
fgctl audit system
```

**Sources scanned:**

| Source | Command used |
|--------|-------------|
| npm globals | `npm -g list --json --depth=0` |
| pip globals | `pip list --format=json` |
| cargo installed bins | `cargo install --list` |
| Go bins | `$GOPATH/bin` directory listing |

**Example output:**

```
  Auditing globally installed packages...

  npm globals (12 packages)
    [HIGH]  nodemon@2.0.20       CVE-2022-... — Prototype pollution
    [PASS]  typescript@5.4.5

  pip globals (34 packages)
    [MEDIUM] Pillow@9.5.0        CVE-2023-44271 — Uncontrolled resource consumption
    [PASS]  requests@2.31.0

  cargo bins (5 packages)
    [PASS]  ripgrep@14.1.0

  Go bins ($GOPATH/bin) (8 packages)
    [SKIP]  staticcheck  — version not parseable

  System Audit Summary: 59 packages audited · 2 finding(s) (1 HIGH · 1 MEDIUM)
```

---

## 12b. Signature Statistics — `fgctl stats`

View a breakdown of the locally loaded detection signatures:

```bash
fgctl stats                        # formatted table (human-readable)
fgctl stats --json                 # JSON object (machine-readable)
fgctl stats --store=./custom.json  # use a custom signature store path
```

**Example output:**

```
ForgeGuardian — Detection Signature Statistics
──────────────────────────────────────────────
  Total signatures:    1 247
  Last updated:        2026-05-24 09:12:43

  Breakdown by type:
    malware_pattern          423
    typosquat_target         318
    behavioral_rule          201
    blocklisted_package      187
    mcp_injection_pattern     68
    pickle_rule               50

  Ecosystems covered:  npm · pypi · go · rubygems · crates · huggingface · mcp
```

**`--json` output:**

```json
{
  "total": 1247,
  "last_updated": "2026-05-24T09:12:43Z",
  "by_type": {
    "malware_pattern": 423,
    "typosquat_target": 318,
    "behavioral_rule": 201,
    "blocklisted_package": 187,
    "mcp_injection_pattern": 68,
    "pickle_rule": 50
  },
  "ecosystems": ["npm","pypi","go","rubygems","crates","huggingface","mcp"]
}
```

The banner also shows "Loaded N detection signatures" immediately after the logo whenever signatures are present.

---

## 13. SARIF Output for CI

Output findings in SARIF 2.1.0 format for upload to GitHub Code Scanning:

```bash
fgctl scan . --format=sarif > results.sarif
fgctl scan npm/lodash@4.17.20 --format=sarif | jq '.runs[0].results | length'
```

**Severity mapping:**

| ForgeGuardian | SARIF level |
|---------------|-------------|
| CRITICAL / HIGH | `error` |
| MEDIUM | `warning` |
| LOW | `note` |
| INFORMATIONAL | `none` |

**GitHub Actions integration:**

```yaml
- name: Scan project
  run: fgctl scan . --fail-on=high --format=sarif > results.sarif

- name: Upload to GitHub Code Scanning
  uses: github/codeql-action/upload-sarif@v3
  if: always()
  with:
    sarif_file: results.sarif
```

---

## 14. Doctor / Self-diagnostic Mode

```bash
fgctl doctor
```

### What it checks

| Check | Pass condition | Details |
|-------|---------------|---------|
| `grype` | Binary in PATH | Reports version |
| `semgrep` | Binary in PATH | Reports version |
| `trivy` | Binary in PATH | Reports version |
| `ANTHROPIC_API_KEY` | Env var set | Warn if missing (AI features unavailable) |
| `signatures` | `~/.forgeguardian/signatures.json` exists + non-empty | Reports count |
| `disk space` | ≥500 MB free on `$HOME` filesystem | Reports free space |
| `Go version` | go binary in PATH | Reports version |
| `API server` | `$FORGEGUARDIAN_API_URL` responds to `/healthz` | Only relevant in server mode |
| `signatures freshness` | `~/.forgeguardian/signatures.json` mtime ≤ 7 days | Warns (non-blocking) if stale — run `fgctl update` |
| `Docker` | `docker` binary in PATH and daemon reachable | Non-blocking warn; required for hermetic builds |
| `Node.js version` | `node --version` ≥ 20 | Required for dashboard development |
| `dashboard dist` | `./dashboard/dist/` directory exists + non-empty | Warns if dashboard not built — run `make build-dashboard` |
| `config YAML` | `~/.forgeguardian/config.yaml` parses without error | Warns on malformed YAML — run `fgctl config init` to reset |

### Auto-repair (`--fix`)

```bash
fgctl doctor --fix
```

`--fix` attempts to automatically resolve failed checks. Before running each repair command it prints the exact command it will execute:

```
[FIX] Running: brew install anchore/grype/grype
[FIX] Running: fgctl update
[FIX] Running: fgctl config init
```

### Exit codes

- `0` — all checks pass or warn only
- `1` — one or more checks failed

---

## 15. Debug Command

Collect diagnostics for a bug report:

```bash
fgctl debug           # human-readable
fgctl debug --json    # JSON for tooling
```

**Output includes:**
- Version, commit hash, build time
- OS and architecture
- Go runtime version
- Config file existence and size
- Signature store count and last-modified date
- API URL and reachability status
- Tool locations (grype, semgrep, trivy)
- `ANTHROPIC_API_KEY` presence (masked)
- Disk space free

---

## 16. Config Command

Manage `~/.forgeguardian/config.yaml`:

```bash
fgctl config show                       # print current config with file path
fgctl config set scan.workers=8         # update a key
fgctl config set scan.fail_on=high      # always fail on high in this environment
fgctl config set api_url=http://my-api  # point to a remote API
fgctl config init                       # write defaults to config file
```

**Valid keys:**

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `api_url` | string | `http://localhost:8080` | ForgeGuardian API base URL |
| `scan.workers` | int | `4` | Concurrent scan workers |
| `scan.timeout` | duration | `10m` | Scan timeout per cycle |
| `scan.fail_on` | string | `""` | Minimum severity to exit 2: `critical`/`high`/`medium`/`low` |
| `scan.min_severity` | string | `""` | Filter output to this severity and above |
| `signing.rekor_url` | string | `""` | Custom Rekor transparency log URL |

CLI flags always override config file values.

---

## 17. Web Dashboard

### Start the dashboard

**Minimal (API only):**
```bash
make up                              # postgres + redis + api
cd dashboard && npm ci && npm run dev
# Open http://localhost:3000
```

**Full dev stack:**
```bash
make dev
```

**Makefile shortcuts:**

| Command | Description |
|---------|-------------|
| `make up` | Start minimal docker stack |
| `make down` | Stop all docker services |
| `make logs` | Tail API container logs |
| `make health` | Check API reachability |
| `make dev` | Start full dev stack + dashboard |

### API offline banner

If the API is not running when you open the dashboard, an amber banner appears at the top:

```
ForgeGuardian API offline — Start the API: make api   OR   docker compose -f docker-compose.minimal.yml up -d
```

The banner auto-dismisses when the API comes back online (polls every 15s).

### Pages

The dashboard ships **28 routes** — all connected to live backend data:

| Page | What it does |
|---|---|
| **Dashboard** | SOC-style overview — risk heatmap, activity feed, 30-day recharts timeline |
| **Scan** | **Tab 1:** registry package scan — downloads real artifact, runs all 8 engines, shows engine status bar. **Tab 2:** drag-drop project archive (`.tar.gz`/`.zip`) for full scan |
| **Inventory** | Paginated package list with search and ecosystem filter |
| **Advisory** | AI-generated security advisory per package (needs `ANTHROPIC_API_KEY`) |
| **SBOM** | Generate and download CycloneDX / SPDX in 4 formats |
| **Sign / Verify** | Sigstore keyless signing + attestation verification |
| **Monitor** | Live SBOM monitoring — auto-reconnect with exponential backoff |
| **Intelligence** | Detection signatures list + manual refresh trigger |
| **Risks** | Risk heatmap with A–F grades, sortable |
| **Policy** | Policy rules display + last evaluation status |
| **Alerts** | Real-time security alerts from DB — severity filter + one-click dismiss |
| **Allowlist** | Add/remove trusted packages that bypass policy enforcement (full CRUD) |
| **Projects** | Risk posture grouped by package |
| **Dependency Drift** | 30-day vulnerability trend chart |
| **AI Agents** | Live SSE feed of autonomous patch agent sessions — session status badge, event log, clear button |
| **Webhooks** | Configure Slack/Discord alerts + test delivery |
| **CI/CD** | GitHub Actions, GitLab CI, Makefile integration snippets |
| **System Audit** | brew / gem / docker / PATH security audit |
| **Recursive Scan** | Multi-package scan — comma-separated packages, per-package results |
| **Exports** | SBOM format guide + generate links |
| **AI Security** | AI supply chain threat explainer |
| **Settings** | Config management |

### Scan engine status bar

Every scan result in the dashboard shows which engines ran:

```
✓ osv (3)    ✓ behavioral (1)    ✓ malware    ✗ grype (not installed)    ✗ semgrep (not installed)
artifact downloaded — full scan
```

`✓` = ran · number = findings · `✗` = skipped with reason

### File upload scan

```
Dashboard → Scan → Upload Project tab
Drop any .tar.gz / .zip / .jar / .gem / .whl
→ Uploads to POST /api/v1/scan/upload
→ Extracts to temp dir
→ Runs all 8 engines
→ Returns findings + engine status
```

### Live agent feed

`Dashboard → AI Agents` — connects to `GET /api/v1/agent/stream` (Server-Sent Events).  
When `fgctl patch` runs, events stream in real time: `start` → `step` → `patch` → `done`.  
No page refresh needed. Reconnects automatically on disconnect.

### Theme

Industrial SOC dark theme: `#0A0B0D` background · `#00FF87` green · `#FFAB40` amber · `#FF3D3D` red · JetBrains Mono for data.

### Node version

The dashboard requires Node 20 LTS (pinned in `.nvmrc`):
```bash
nvm use   # reads .nvmrc → uses Node 20
```

### API timeout

All API calls in the dashboard have a 30-second AbortController timeout. Hung requests surface as `"API request timed out after 30s"` rather than spinning indefinitely.

---

## 18. VS Code Extension

### Install

Install from the VS Code marketplace (search "ForgeGuardian") or build from source:

```bash
cd vscode-extension
npm install
npm run compile
# Load via "Install VSIX" in VS Code
```

### Features

**Inline diagnostics** — red/yellow squiggles on vulnerable packages in:
- `package.json` (npm)
- `requirements.txt` (pypi)
- `go.mod` (go)

**Hover cards** — hover over any package line to see:
- Top 3 findings by severity
- CVE IDs and source scanner
- Link to open the AI advisory

**CodeLens** — per-dependency scan status shown above each line:
- `🛡 ForgeGuardian: Click to scan` (unseen)
- `✓ No findings` (clean)
- `⚠ 2 findings (1 HIGH)` (issues found)

**Auto-scan on save** — enable in settings:
```json
{ "forgeguardian.autoScanOnSave": true }
```

**Commands (Ctrl+Shift+P):**
- `ForgeGuardian: Scan Current File`
- `ForgeGuardian: Scan Package`
- `ForgeGuardian: Generate AI Advisory`
- `ForgeGuardian: Generate SBOM`
- `ForgeGuardian: Verify Attestation`
- `ForgeGuardian: Open Dashboard`
- `ForgeGuardian: Refresh Intelligence Signatures`

### Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `forgeguardian.apiUrl` | `http://localhost:8080` | ForgeGuardian API server URL |
| `forgeguardian.autoScanOnSave` | `false` | Scan manifest files on save |
| `forgeguardian.failOnSeverity` | `high` | Minimum severity for error diagnostics |
| `forgeguardian.anthropicApiKey` | `""` | Anthropic API key (or use `ANTHROPIC_API_KEY` env var) |

---

## 19. Docker Profiles

### Minimal

```bash
make up
# postgres:5432  redis:6379  forgeguardian-api:8080
```

Use for: API development, local testing, low-resource environments.

### Dev

```bash
make dev
# + minio:9000/9001  prometheus:9090  grafana:3002  forgeguardian-worker
```

Use for: full local development with artifact storage and observability.

### Enterprise

```bash
make docker-enterprise
# + rekor-server:3001  trillian  dependency-track:8081  intel-agent
```

Use for: full stack with Sigstore transparency log, continuous monitoring, and automated intelligence.

### Environment variables for docker

```bash
# .env file (git-ignored)
POSTGRES_PASSWORD=devpassword
GRAFANA_PASSWORD=admin
ANTHROPIC_API_KEY=sk-ant-...
```

---

## 20. Test Environments

### Local machine (scan only — safe)

Read-only operations are safe on your local machine:

```bash
fgctl scan npm/chalk@5.3.0
fgctl sbom pypi/six@1.16.0
fgctl sign npm/chalk@5.3.0 --out=/tmp/chalk.att.json
```

### Docker sandbox (recommended for agent testing)

```bash
docker run --rm -it \
  -e ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY \
  golang:1.23-alpine sh

# Inside container:
go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest
go install github.com/mah3sec/forgeguardian/cmd/fg-agent@latest
fg-agent --recipe=npm --package=lodash --version=4.17.20 \
  --project-dir=/tmp/test-project --apply
```

### VM (full isolation for --apply testing)

| Setting | Value |
|---------|-------|
| OS | Ubuntu 22.04 LTS |
| RAM | 4 GB |
| Disk | 40 GB |
| Snapshot | Take snapshot before testing |

### GitHub Codespaces

`.devcontainer/devcontainer.json`:
```json
{
  "image": "mcr.microsoft.com/devcontainers/go:1.23",
  "postCreateCommand": "go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest",
  "containerEnv": { "ANTHROPIC_API_KEY": "${localEnv:ANTHROPIC_API_KEY}" }
}
```

---

## 21. Vulnerable Packages for Testing

> Run in a sandbox/VM. Do not install or execute these in production.

### npm

| Package | Version | CVE(s) | Type |
|---------|---------|--------|------|
| `lodash` | `4.17.20` | CVE-2021-23337, CVE-2020-28500 | Command injection, prototype pollution |
| `minimist` | `1.2.5` | CVE-2021-44906 | Prototype pollution |
| `axios` | `0.21.1` | CVE-2021-3749 | SSRF |
| `qs` | `6.5.2` | CVE-2022-24999 | Prototype pollution |

### PyPI

| Package | Version | CVE(s) | Type |
|---------|---------|--------|------|
| `PyYAML` | `5.3.1` | CVE-2020-14343 | Arbitrary code execution |
| `Pillow` | `8.3.1` | CVE-2021-34552 | Buffer overflow |
| `requests` | `2.25.0` | CVE-2023-32681 | Credential leak via redirect |

### Maven

| Package | Version | CVE(s) | Type |
|---------|---------|--------|------|
| `org.apache.logging.log4j:log4j-core` | `2.14.1` | CVE-2021-44228 | **Log4Shell RCE** (CVSS 10.0) |
| `org.springframework:spring-core` | `5.3.17` | CVE-2022-22965 | **Spring4Shell RCE** |

### Go

| Package | Version | CVE(s) | Type |
|---------|---------|--------|------|
| `github.com/gin-gonic/gin` | `v1.6.3` | GHSA-h395-qcrw-5vmq | HTTP request smuggling |
| `golang.org/x/crypto` | `v0.0.0-20200109152110` | CVE-2020-29652 | SSH panic |

### Recommended test progression

```bash
# 1. Clean package — green result
fgctl scan npm/chalk@5.3.0

# 2. Known CVEs with fix hints
fgctl scan npm/lodash@4.17.20

# 3. Famous Java CVE (Log4Shell)
fgctl scan maven/org.apache.logging.log4j:log4j-core@2.14.1

# 4. AI advisory (needs API key)
export ANTHROPIC_API_KEY=sk-ant-...
fgctl advisory npm/lodash@4.17.20

# 5. Autonomous patch agent dry-run
mkdir /tmp/test && echo '{"dependencies":{"lodash":"4.17.20"}}' > /tmp/test/package.json
fg-agent --recipe=npm --package=lodash --version=4.17.20 --project-dir=/tmp/test

# 6. SARIF output
fgctl scan npm/lodash@4.17.20 --format=sarif | jq '.runs[0].results | length'

# 7. Monitor mode
fgctl monitor --watch /tmp/test &
echo '{"dependencies":{"lodash":"4.17.20","express":"4.19.2"}}' > /tmp/test/package.json
# should see: NEW [CVE] express...

# 8. System audit
fgctl audit system
```

---

## 22. Full Walkthrough Examples

### Scan → SBOM → Sign → Verify (no API key)

```bash
PKG=chalk VER=5.3.0 ECO=npm

fgctl scan $ECO/$PKG@$VER
fgctl sbom $ECO/$PKG@$VER --format=cyclonedx-json --out=$PKG-sbom.json
fgctl provenance $ECO/$PKG@$VER --out=$PKG-provenance.json
fgctl sign $ECO/$PKG@$VER --out=$PKG-attestation.json
SHA=$(jq -r '.sha256' $PKG-attestation.json)
fgctl verify --attestation=$PKG-attestation.json --sha256=$SHA
```

### Full AI pipeline on a vulnerable package

```bash
export ANTHROPIC_API_KEY=sk-ant-...

mkdir /tmp/vuln-test
cat > /tmp/vuln-test/package.json << 'EOF'
{ "name": "test-app", "dependencies": { "lodash": "4.17.20" } }
EOF

# Scan (shows fix hint)
fgctl scan /tmp/vuln-test

# AI advisory
fgctl advisory npm/lodash@4.17.20

# Autonomous patch (dry-run first)
fg-agent --recipe=npm --package=lodash --version=4.17.20 \
  --project-dir=/tmp/vuln-test

# Apply
fg-agent --recipe=npm --package=lodash --version=4.17.20 \
  --project-dir=/tmp/vuln-test --apply

cat /tmp/vuln-test/package.json  # should show lodash@4.17.21
```

### CI with SARIF upload

```bash
# scan — exits 2 if HIGH+ finding, emits SARIF
fgctl scan . --fail-on=high --format=sarif > results.sarif || true

# upload to GitHub Code Scanning
# (done by github/codeql-action/upload-sarif in the workflow)
```

### Scan local Go project with compact output

```bash
cd ~/projects/my-go-app
fgctl scan . --compact --severity=high --fail-on=critical
```

### Audit all global tools

```bash
fgctl audit system
# reviews npm -g, pip, cargo, and $GOPATH/bin
```

### Collect a bug report

```bash
fgctl debug --json > fg-debug.json
# attach to GitHub issue
```

---

## 23. Troubleshooting

| Problem | Cause | Fix |
|---------|-------|-----|
| `ANTHROPIC_API_KEY not set` | Missing env var | `export ANTHROPIC_API_KEY=sk-ant-...` |
| `[WARN] ANTHROPIC_API_KEY` in doctor | Same | Same |
| `recipe "X" not found` | Typo | Use: `npm pypi maven go rubygems crates huggingface mcp` |
| `grype not installed — skipped` | Binary not in PATH | `brew install anchore/grype/grype` |
| `build failed: 404` | Package/version doesn't exist | Check on registry |
| `rekor_verified: false` | Rekor upload failed (no internet) | Normal — local signing still works |
| Dashboard shows amber offline banner | API not running | `make up` |
| Dashboard shows API errors in Monitor | Same | Same |
| `[SKIP]` in local scan | Version range (no pinned version) | Pin versions in manifest |
| `fgctl: command not found` | Not in PATH | `export PATH=$PATH:$(go env GOPATH)/bin` |
| Semgrep takes too long | Large package | `--timeout=20m` |
| `conflicting replacements` in go build | `go.work` edited incorrectly | `git checkout go.work` |
| VSCode diagnostics not appearing | API not running | Start `docker-compose.minimal.yml`, check `forgeguardian.apiUrl` |
| VSCode hover not working | `autoScanOnSave` disabled, no scan run | Run `ForgeGuardian: Scan Current File` command |
| `fgctl debug` shows config: not found | Config not initialised | `fgctl config init` |
| `fgctl monitor` exits immediately | Missing `--watch` flag | `fgctl monitor --watch .` |

---

## 24. Policy-as-Code

ForgeGuardian enforces security policy from `~/.forgeguardian/policy.yaml`. Policy findings are appended after every scan with a `[POLICY]` prefix and never suppress scan results.

### Default policy file

```yaml
version: 1
fail_on: ""                 # "" | CRITICAL | HIGH | MEDIUM | LOW
deny_packages: []           # exact package names to block
allow_licenses: [MIT, Apache-2.0, BSD-3-Clause, BSD-2-Clause, ISC]
max_package_age_days: 0     # 0 = disabled
block_typosquatting: false
block_abandoned: false
require_signing: false
```

### Initialise and configure

```bash
fgctl policy init                          # create ~/.forgeguardian/policy.yaml
fgctl policy show                          # print current policy
fgctl policy set fail_on=high              # block scans with high+ findings
fgctl policy set block_typosquatting=true  # block typosquatting packages
fgctl policy validate                      # check YAML syntax
```

### Policy finding IDs

| Finding ID | Trigger | Severity |
|-----------|---------|---------|
| `POLICY-DENIED-PACKAGE` | Package name in `deny_packages` | CRITICAL |
| `POLICY-TYPOSQUAT-BLOCKED` | `block_typosquatting=true` + behavioral typosquat signal | CRITICAL |

### CI integration

```yaml
# .github/workflows/scan.yml
- run: fgctl scan . --fail-on=high
# exit code 2 = policy violated, exit code 1 = scan error, exit code 0 = clean
```

---

## 25. Risk Score

Every scanned package receives a 0–100 risk score and an A–F letter grade surfaced in the dashboard and (when using `--compact`) in the CLI.

### Scoring model

| Factor | Weight | Trigger |
|--------|--------|---------|
| Vulnerability | 0–40 pts | CVE severity: CRITICAL=40, HIGH=20, MEDIUM=8, LOW=3 |
| Behavioral | 0–30 pts | Malware/behavioral findings: CRITICAL=30, HIGH=15 |
| Supply chain | 0–20 pts | Typosquatting, dependency confusion signals |
| Maintenance | 0–10 pts | Abandonment, extreme age signals |

### Grade thresholds

| Score | Grade | Meaning |
|-------|-------|---------|
| 0–20 | **A** | Low risk |
| 21–40 | **B** | Moderate risk |
| 41–60 | **C** | Elevated risk — review recommended |
| 61–80 | **D** | High risk — action required |
| 81–100 | **F** | Critical risk — block immediately |

### Dashboard views

- **Risks page** (`/risks`) — full prioritized table with grade column
- **Dashboard** — Top 5 active risks widget + ecosystem risk heatmap
- **Inventory** — per-package grade in expanded row

---

## 26. Webhook Notifications

ForgeGuardian fires webhook notifications after scans when findings meet the configured severity threshold.

### Configure

```bash
# Slack
fgctl config set notify.slack_webhook_url=https://hooks.slack.com/services/...
fgctl config set notify.on_severity=high

# Discord
fgctl config set notify.discord_webhook_url=https://discord.com/api/webhooks/...

# Generic HTTP endpoint
fgctl config set notify.webhook_url=https://your-siem.internal/ingest
```

Or edit `~/.forgeguardian/config.yaml` directly:

```yaml
notify:
  slack_webhook_url: "https://hooks.slack.com/services/..."
  discord_webhook_url: ""
  webhook_url: ""
  on_severity: "high"   # CRITICAL | HIGH | MEDIUM | LOW | "" (disabled)
```

### Test webhook

```bash
curl -X POST http://localhost:8080/api/v1/webhooks/test
```

### Payload format (generic webhook)

```json
{
  "event": "scan_finding",
  "package": "lodash@4.17.20",
  "severity": "HIGH",
  "finding_id": "CVE-2021-23337",
  "message": "lodash@4.17.20: [HIGH] CVE-2021-23337",
  "timestamp": "2026-05-24T12:00:00Z"
}
```

Notifications are best-effort — a failed webhook does not fail the scan.

---

## 27. Product Modes

ForgeGuardian adapts to five deployment contexts. See `MODES.md` for full setup guides.

| Mode | Primary User | Entry Point | Key Features |
|------|-------------|-------------|--------------|
| **Developer** | Individual engineer | `fgctl scan .` | IDE integration, inline hints, patch suggestions |
| **CI/CD** | DevOps / pipelines | `fgctl scan . --format=sarif` | SARIF output, policy gates, exit codes, signed SBOMs |
| **Monitoring** | Security engineer | `fgctl monitor --watch .` | Continuous re-scan, diff mode, webhook alerts |
| **Enterprise** | Security team / SOC | `docker-compose.enterprise.yml` | Full dashboard, risk scores, policy, Grafana metrics |
| **Offline / Air-gapped** | Classified / restricted | `--offline` flag | Local DB only, no outbound network, bundled signatures |

Quick-start by mode:

```bash
# Developer
fgctl scan .

# CI (fail on high or above, SARIF output)
fgctl scan . --fail-on=high --format=sarif > results.sarif

# SOC Enterprise stack
docker compose -f docker-compose.enterprise.yml up -d
open http://localhost:3000

# Air-gapped
fgctl scan . --offline
```

---

## 28. Trust & Privacy

ForgeGuardian is designed around a local-first, zero-trust architecture.

### What leaves your machine

| Data | Leaves machine? | Notes |
|------|----------------|-------|
| Package names + versions | Yes — when scanning | Sent to OSV API for CVE lookups; package names only, no source code |
| Source code | **Never** | All analysis is static/behavioral on the binary/manifest |
| Scan results | **Never** | Results stored in local PostgreSQL or printed to stdout only |
| Telemetry / analytics | **Never** | No usage tracking, no phone-home, no crash reports |
| AI prompts (advisory) | Yes — when using `fgctl advisory` | Finding data sent to Anthropic API; requires explicit invocation |
| Signing keys | **Never** | Ephemeral ECDSA keys generated per-signing event; never stored |

### Air-gapped operation

Run with `--offline` to disable all external API calls. Pre-download the OSV vulnerability database:

```bash
fgctl update --offline-bundle=/path/to/osv-bundle.zip
fgctl scan . --offline
```

### Self-hosted deployment

The full stack (API, dashboard, PostgreSQL, Rekor) runs entirely on-premises via docker-compose. No Anthropic API key required for non-AI features.

### ANTHROPIC_API_KEY usage

The key is only used when:
1. `fgctl advisory <package>` — AI-generated advisory
2. `fg-agent` autonomous patch agent
3. `intel-agent` signature generation

It is never sent to third parties other than Anthropic.

---

## 29. Competitive Landscape

See `COMPETITIVE.md` for detailed feature matrices. Summary:

## ForgeGuardian Capability Summary

| Capability | Status |
|---|---|
| Local-first scanning (no cloud required) | ✅ |
| AI-native triage and advisory | ✅ |
| HuggingFace / MCP ecosystem security | ✅ |
| Community detection signatures | ✅ |
| Policy-as-code (local enforcement) | ✅ |
| Behavioral analysis | ✅ |
| Monitor mode (live diff) | ✅ |
| SARIF + CycloneDX + SPDX output | ✅ |
| Sigstore / SLSA provenance | ✅ |
| Self-hostable | ✅ |
| Open source (Apache 2.0) | ✅ |

---

## 30. Changelog

### v2.0.0 — 2026-06-13 — Full Platform Integration

**Dashboard — scan now actually works:**
- `TriggerScan` fixed: downloads real artifact from registry, extracts to temp dir, runs all 8 engines — was previously OSV-only with empty `LocalPath`
- `POST /api/v1/scan/upload` — new endpoint: multipart file upload, extract, full scan
- ScanPage rewritten: 2 tabs (registry + file upload), engine status bar per-scan, sha256 display, graceful degradation message when download fails

**New API endpoints (8):**
- `GET/POST/DELETE /api/v1/allowlist` + `GET /api/v1/allowlist/check`
- `GET/POST /api/v1/alerts` + `POST /api/v1/alerts/:id/dismiss`
- `GET /api/v1/agent/stream` (SSE) + `POST /api/v1/agent/events`

**Dashboard pages wired to real data:**
- `AlertsPage` — real DB alerts, severity filter, dismissed toggle, one-click dismiss
- `AllowlistPage` — full CRUD (add with ecosystem/package/reason, delete by ID)
- `AgentsPage` — live SSE event feed, session status badge, scrollable log, clear button
- `RecursiveScanPage` — multi-package scan, comma-separated input, per-package results

**Community signatures — Nuclei-style toolkit:**
- `fgctl intel new` — interactive 5-question wizard, auto-generates ID, writes YAML
- `fgctl intel validate` — schema + regex + required fields, supports globs
- `fgctl intel test` — downloads real package, runs relevant engines, shows MATCH/NO MATCH
- `fgctl intel update` — pulls from community repo URL
- `fgctl intel list` — filterable by type and ecosystem
- 24 community signatures added (blocklisted ×9, typosquatting ×3, behavioral ×4, malware ×4, mcp ×2, ai-model ×2)
- `signatures/` directory with CI workflow (`validate.yml`) + bundle builder (`build_bundle.py`)

**Security & stability:**
- API key auth middleware (`X-Api-Key` / `Bearer`, constant-time compare, `/healthz` exempt)
- Per-IP token bucket rate limiter (60 rps, burst 20, auto-evict stale IPs)
- DB migration runner (embedded SQL, `schema_migrations` tracking, transactional per-file)
- Scraper `--watch` flag + `FG_SCRAPER_INTERVAL` continuous scheduling
- `FG_API_KEY` config field, dev-mode warning when unset
- Dashboard `VITE_API_KEY` env var wired into all API calls

**Tests: 43 total (was 0):**
- `internal/core` — 9 tests (ScoreFindings, grade boundaries)
- `internal/api/middleware` — 9 tests (auth ×6, rate limiter ×3)
- `internal/policy` — 15 tests (enforce, thresholds, save/load)
- `internal/notify` — 10 tests (webhook shape, threshold, HTTP errors)

### v1.4.0 — 2026-05-24 — Phase 15: Final Pre-Release Platform Hardening

**CLI & scanner:**
- Competitor branding removed from all marketing copy (README, COMPETITIVE.md, GUIDE.md, DOCS.md)
- INFORMATIONAL findings hidden by default; `--verbose` expands them
- Banner stats line: "Loaded: • N signatures • N heuristics • N engines"
- HIGH SevBadge color fix (FgHiRed)
- Scan timing footer: "Completed in X.Xs"
- Duplicate `stats` entry removed from help
- `auditBrewPackages`, `auditGemPackages`, `auditDockerImages`, `checkPATHHijack` added to `fgctl audit system`

**Dashboard:**
- TopBar with live engine/severity counts
- Sidebar: System Audit, Agents, Settings stub pages

**Docs & security:**
- `SECURITY.md` added ("How ForgeGuardian Secures Itself")
- npm audit CI gate

### v1.3.0 — 2026-05-24 — Phase 14: Production Polish + Operational Consistency

**CLI fixes & polish:**
- `preParseOutputFlags()` — scans `os.Args` before any `flag.Parse` call; `--format=sarif/json`, `--quiet`, `--ci`, `--no-banner` now suppress the banner before any subcommand runs, including when a positional argument precedes the flags (e.g. `fgctl scan . --format json`)
- `--quiet` early return — no stdout leaks; advisory hints no longer bleed through
- `fgctl sbom .` — auto-suppresses the banner when writing SBOM JSON to stdout; banner still prints when `--out` redirects to a file
- `severityOrd()` fix — returns `-1` for unknown/empty severity (was `0`); `--severity high` now correctly excludes MEDIUM, LOW, and INFORMATIONAL findings
- `--only-fixable` — also excludes INFORMATIONAL findings
- `--compact` — now calls `PrintCompactGrouped()`, printing one line per package with bracket-grouped counts: `axios@1.3.4  — 19 findings [CRIT:1 HIGH:1 MED:17]  fix: >=1.12.0`
- `fgctl stats` new command — signature runtime statistics table (total sigs, by type, ecosystems, last updated); supports `--json`
- `fgctl audit system` — prints an ecosystem-level summary table instead of per-package output; `--verbose` expands to per-package
- `fgctl monitor --watch` — every diff line now shows a `[HH:MM:SS]` timestamp, the package name, and is color-coded: `NEW` in red, `RESOLVED` in green
- Banner — shows "Loaded N detection signatures" line after the logo when signatures are present

**Dashboard fixes:**
- `api.ts` — Content-Type check before `res.json()`; prevents the cryptic `"Unexpected token '<'"` error when the API is down
- `ApiStatusBanner` — `retry: 3`, exponential backoff, `[RECONNECTING...]` amber state label
- `MonitorPage` — same retry/reconnect state as ApiStatusBanner

### v1.2.0 — 2026-05-24 — Phase 13: Production Hardening & Release Readiness

See CLAUDE.md Phase 13 entry for the full list of 20 improvements.

---

*ForgeGuardian is open source — community contributions welcome.*  
*Documentation last updated: 2026-06-13*
