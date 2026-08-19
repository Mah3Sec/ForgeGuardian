```
   ███████╗ ██████╗  ██████╗  ██████╗ ███████╗
   ██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝
   █████╗  ██║   ██║██████╔╝██║  ███╗█████╗
   ██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝
   ██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗
   ╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝
              GUARDIAN · local-first AI-native supply chain security
```

# ForgeGuardian

**Local-first, AI-native Software Supply Chain Security Platform**

> Community-driven detection. AI-native triage. Full 8-engine scanning. Works offline.

![Docker](https://img.shields.io/badge/Docker-One%20Command%20Install-2496ED?style=flat-square&logo=docker)
![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat-square&logo=go)
![License](https://img.shields.io/badge/License-Apache%202.0-green?style=flat-square)
![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-blue?style=flat-square)
![Signatures](https://img.shields.io/badge/Community%20Signatures-24-orange?style=flat-square)

**Not a developer?** → [Executive Summary](EXECUTIVE_SUMMARY.md) — what this does and why it matters, no CLI or code.

---

## What It Is

ForgeGuardian is a supply chain security platform that works at three levels:

| Level | What you get |
|---|---|
| **CLI** (`fgctl`) | Scan any project in 2 commands. Works offline. No account needed. |
| **Dashboard** | Web UI with full 8-engine scan, file upload, live agent feed, alerts, allowlist |
| **API** | 42 REST endpoints — scan, SBOM, sign, advisory, allowlist, alerts, SSE stream |

One tool. 9 ecosystems. AI triage. Community signatures. SLSA Level 3 provenance.

---

## Get Started

Pick one:

### Option A — Full Platform (API + Web Dashboard)

Requires: [Docker](https://docs.docker.com/get-docker/)

```bash
docker run -d --name forgeguardian -p 3000:3000 ghcr.io/mah3sec/forgeguardian
```

Open **http://localhost:3000** and log in:

| | |
|---|---|
| **Email** | `admin@forgeguardian.local` |
| **Password** | `changeme123` |

That's it. Scan packages, view results, generate SBOMs, browse alerts — all from the browser.

To set your own credentials:
```bash
docker run -d --name forgeguardian -p 3000:3000 \
  -e FG_ADMIN_EMAIL=you@example.com \
  -e FG_ADMIN_PASSWORD=YourSecurePass \
  ghcr.io/mah3sec/forgeguardian
```

### Option B — CLI Only

No Docker needed. One command installs `fgctl` to `~/.local/bin`:

```bash
curl -sSfL https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.sh | bash
```

Then scan:
```bash
fgctl scan .                             # scan your current project
fgctl scan npm/lodash@4.17.20            # scan a specific package
fgctl advisory npm/lodash@4.17.20        # AI security advisory (needs ANTHROPIC_API_KEY)
```

No config file. No account. No API keys for scanning.

<details>
<summary>Other install methods</summary>

```bash
# Go install (requires Go 1.23+)
go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest

# Build from source
git clone https://github.com/Mah3Sec/ForgeGuardian.git
cd ForgeGuardian && make build
```

**Windows:** Download the `.zip` from [Releases](https://github.com/Mah3Sec/ForgeGuardian/releases), extract, and add to PATH.
</details>

### Sample Output

```
$ fgctl scan .

axios@1.3.4  [grade F · 19 findings]
├─ CRITICAL  CVE-2023-45857  Header Injection   → fix: >= 1.12.0
├─ HIGH      CVE-2022-1214   SSRF               → fix: >= 1.7.4
└─ +17 more  (use --verbose to expand)

Completed in 3.2s  •  3 packages  •  Loaded: 24 signatures • 8 engines
```

---

## Free vs Pro

ForgeGuardian is **open-core** — the engine, CLI, scanner, and community tools are Apache 2.0, free forever. Pro adds AI-powered features and team capabilities.

| Feature | Community (Free) | Pro |
|---|---|---|
| `fgctl scan .` — local project scan | ✅ | ✅ |
| 8 scan engines (OSV + Behavioral + Malware + AI Model + MCP) | ✅ | ✅ |
| SBOM generation (CycloneDX + SPDX) | ✅ | ✅ |
| Sigstore keyless signing + verification | ✅ | ✅ |
| Community signatures (contribute + use) | ✅ | ✅ |
| `fgctl intel new/validate/test/update` | ✅ | ✅ |
| Policy-as-code enforcement | ✅ | ✅ |
| Self-hostable + airgap-compatible | ✅ | ✅ |
| Basic dashboard | ✅ | ✅ |
| Dashboard: allowlist, advisory, monitor, alerts, agents (patch feed), projects, webhooks | ✅ | ✅ |
| `fgctl advisory` / `fgctl patch` / `fgctl monitor` — AI features via CLI | 🔒 needs `FG_LICENSE_KEY` | ✅ |
| The same 3 features via the dashboard/API (needs `ANTHROPIC_API_KEY` only) | ✅ | ✅ |
| Team management + RBAC | — | ✅ |
| SLA + priority support | — | ✅ |
| Cloud-hosted option | — | ✅ |

> **Why open-core?** The engine stays free, community signatures stay community-owned, revenue from Pro funds continued development. You'll never lose access to what you have today. Pro doesn't exist yet as a shipped product with actual billing — the CLI checks for `FG_LICENSE_KEY` today, but the dashboard and API don't enforce this at all, so the split above is aspirational, not yet consistently enforced.

Interested in Pro? Watch this repo — a signup link goes here once it ships.

---

## What's Included (zero extra installs for core features)

| Capability | Built-in | Notes |
|---|---|---|
| Local manifest scanner | ✅ | npm, PyPI, Go, Maven, Ruby, Rust, Cargo |
| OSV vulnerability scan | ✅ | Uses osv.dev API |
| Behavioral analysis | ✅ | Postinstall scripts, env harvest, typosquat |
| Malware pattern scan | ✅ | Regex + signature matching |
| AI model weight scan | ✅ | HuggingFace pickle / safetensors |
| MCP server scan | ✅ | Prompt injection, tool shadowing |
| SBOM generation | ✅ | CycloneDX 1.5 + SPDX 2.3 |
| Sigstore signing | ✅ | Keyless, no GPG setup needed |
| AI triage + patch | ✅ | Needs `ANTHROPIC_API_KEY` |
| Policy enforcement | ✅ | YAML policy file, local only |
| Webhook alerts | ✅ | Slack, Discord, generic HTTP |
| Community signatures | ✅ | 24 signatures, `fgctl intel update` to refresh |
| **Deep CVE scan (Grype)** | ⚡ optional | `brew install anchore/grype/grype` |
| **Container scan (Trivy)** | ⚡ optional | `brew install trivy` |
| **SAST (Semgrep)** | ⚡ optional | `pip install semgrep` |

---

## The 8 Scan Engines

Every `fgctl scan` and dashboard scan runs all available engines concurrently:

```
OSV          → Known CVEs via osv.dev API          (always runs)
Behavioral   → Malicious install scripts, typosquatting (always runs)
Malware      → Byte/regex pattern matching          (always runs)
AI Model     → HuggingFace weight safety            (always runs)
MCP          → Prompt injection in tool descriptions (always runs)
Grype        → Deep CVE scan of artifact files      (if installed)
Trivy        → Container + OS CVE scanning          (if installed)
Semgrep      → SAST static analysis                 (if installed)
```

Dashboard scan shows per-engine status: ✓ ran / ✗ skipped (with reason).

---

## Dashboard

Already running if you used **Option A** above. If not:

```bash
docker run -d --name forgeguardian -p 3000:3000 ghcr.io/mah3sec/forgeguardian
```

Open **http://localhost:3000**. CLI and dashboard share the same API — scan from the CLI, see results in the dashboard, and vice versa.

Live preview: [forgeguardian.mahendrapurbia.com](https://forgeguardian.mahendrapurbia.com)

<details>
<summary>Advanced: multi-container setup with Postgres persistence</summary>

For production use with database persistence:
```bash
fgctl setup                    # interactive — writes .env with your credentials
docker compose up -d           # starts postgres + redis + API + dashboard
```

Or manually:
```bash
cp .env.example .env
# Edit .env — set FG_ADMIN_EMAIL, FG_ADMIN_PASSWORD, FG_SESSION_SECRET
docker compose up -d
```
</details>

**28 routes. All connected to live backend when self-hosted.**

| Page | What it does |
|---|---|
| Dashboard | SOC-style overview — risk heatmap, activity feed, timeline chart |
| Scan | **Tab 1**: registry package scan (downloads real artifact, runs all 8 engines) **Tab 2**: drag-drop project archive **Tab 3**: remote host scan over SSH |
| Inventory | Paginated package list with search + ecosystem filter |
| Advisory | AI-generated security advisory per package |
| SBOM | Generate and download CycloneDX / SPDX |
| Sign / Verify | Sigstore keyless signing + attestation verification |
| Provenance | SLSA provenance generation + inspection |
| Monitor | Live SBOM monitoring with reconnect/backoff |
| Intelligence | Detection signatures list + manual refresh |
| Signature Authoring | Guided wizard to write + test a new detection signature |
| Risks | Risk heatmap with letter grades |
| Policy | Policy rules display |
| Alerts | Real-time security alerts — severity filter + one-click dismiss |
| Allowlist | Add/remove trusted packages that bypass policy |
| Projects | Risk posture by package |
| Dependency Drift | 30-day vulnerability trend chart |
| AI Agents | **Live SSE feed** of autonomous patch agent sessions |
| Webhooks | Configure Slack/Discord alerts + test delivery |
| Integrations | Scan engine + CI/CD + webhook status overview |
| CI/CD | GitHub Actions, GitLab, Makefile integration snippets |
| System Audit | brew / gem / docker / PATH security audit |
| Attack Surface | Exposed/reachable dependency surface view |
| Recursive Scan | Multi-package scan with per-package results |
| Exports | SBOM format guide |
| AI Security | AI supply chain threat explainer |
| Docs / API Docs | In-app documentation + API reference |
| Settings | Config management |

---

## Community Signatures — Nuclei-style Contribution

ForgeGuardian uses a community detection library. Contributing takes **10 minutes**:

```bash
# 1. Create a signature with the interactive wizard
fgctl intel new

# 2. Validate schema + regex
fgctl intel validate ./FG-npm-my-sig.yaml

# 3. Test against a real package
fgctl intel test ./FG-npm-my-sig.yaml \
  --ecosystem=npm --package=evil-package --version=1.0.0

# 4. Fork → place in signatures/ → open PR
# CI auto-validates. Maintainer reviews logic only.
```

**24 signatures included** — loaded automatically from this repo's `signatures/` directory when you run `fgctl` from inside a git clone (no setup needed). `fgctl update` pulls newer community signatures once they're published to [forgeguardian-signatures](https://github.com/mah3sec/forgeguardian-signatures):
```bash
fgctl update
fgctl intel list --type=malware_pattern
```

| Type | What it catches | Count |
|---|---|---|
| `blocklisted_package` | Confirmed malicious (event-stream, XZ utils, polyfill.io…) | 9 |
| `typosquatting_target` | Popular packages + variant names (lodash×15, react×17, requests×16) | 3 |
| `behavioral_rule` | Postinstall env harvest, SSH key theft, dep confusion, setup.py exec | 4 |
| `malware_pattern` | base64-eval, discord token, ELF dropper, CI secret exfil | 4 |
| `mcp_injection_pattern` | Tool shadowing, data exfil via output | 2 |
| `pickle_rule` | Unsafe AI model configs, missing model cards | 2 |

Use `fgctl intel new` to create a signature with the interactive wizard.

---

## Scan Flags

```
fgctl scan [path|ecosystem/package@version] [flags]

Output:
  --format=text|json|sarif    Output format (default: text)
  --compact                   One line per finding
  --summary                   Severity table only
  --quiet                     Suppress output, exit code only
  --verbose                   Expand all grouped findings
  --executive                 Executive summary

Filtering:
  --severity=critical|high|medium|low   Minimum severity to show
  --only-fixable                         Only findings with a known fix
  --prod-only / --exclude-dev            Exclude dev dependencies
  --debug                                Show engine errors + raw metadata

Policy / CI:
  --fail-on=critical|high|medium|low    Exit 2 on threshold breach
  --ci                                   CI mode: quiet + SARIF + fail-on=high
  --no-banner  --no-color
```

---

## GitHub Actions Integration

```yaml
- name: Install ForgeGuardian
  run: |
    curl -sSfL https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.sh | bash
    echo "$HOME/.local/bin" >> $GITHUB_PATH

- name: Update signatures & scan
  run: |
    fgctl intel update
    fgctl scan . --format=sarif --fail-on=high > fg.sarif || true

- name: Upload to GitHub Code Scanning
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: fg.sarif
```

Findings appear in the GitHub Security tab with file + line annotations.

---

## API — 42 Endpoints

```
GET  /healthz                               liveness probe
GET  /metrics                               Prometheus metrics

POST /api/v1/scan                           scan registry package (downloads + all engines)
POST /api/v1/scan/upload                    scan uploaded archive (multipart)
POST /api/v1/scan/remote                    scan a remote host over SSH (manifests pulled, nothing installed remotely)
GET  /api/v1/scan/:eco/:name/:ver           get persisted scan results
GET  /api/v1/jobs/:id                       poll async scan job status/result
GET  /api/v1/packages                       list packages (paginated)
GET  /api/v1/packages/:eco/:name            package detail
GET  /api/v1/packages/:eco/:name/versions   version list
POST /api/v1/advisory                       AI advisory
GET  /api/v1/sbom/:eco/:name/:ver           get SBOM
POST /api/v1/sign                           sign artifact
POST /api/v1/verify                         verify attestation
POST /api/v1/provenance                     generate SLSA provenance
GET  /api/v1/dashboard/stats                aggregate stats
GET  /api/v1/dashboard/recent               recent scan activity
GET  /api/v1/dashboard/timeline             daily finding counts
GET  /api/v1/dashboard/graph                dependency graph data
GET  /api/v1/dashboard/activity             event feed
GET  /api/v1/intelligence/signatures        list signatures
POST /api/v1/intelligence/signatures        author a new signature
POST /api/v1/intelligence/refresh           trigger intel agent
POST /api/v1/intelligence/validate          validate signature YAML
POST /api/v1/intelligence/test              test a signature against a real package
GET  /api/v1/risks                          active risk items
GET  /api/v1/policy/status                  policy evaluation status
PUT  /api/v1/policy                         save policy
GET  /api/v1/audit/stats                    system audit statistics
POST /api/v1/webhooks/test                  test webhook delivery
GET  /api/v1/agent/stream                   live SSE agent event stream
POST /api/v1/agent/events                   publish agent event
GET  /api/v1/allowlist                      list allowlist entries
POST /api/v1/allowlist                      add allowlist entry
DELETE /api/v1/allowlist/:id               remove entry
GET  /api/v1/allowlist/check               check if package is allowlisted
GET  /api/v1/alerts                         list alerts (paginated, filtered)
POST /api/v1/alerts                         create alert
POST /api/v1/alerts/:id/dismiss            dismiss alert
POST /api/v1/auth/login                     dashboard login (session cookie)
POST /api/v1/auth/logout                    dashboard logout
GET  /api/v1/auth/me                        current session status
```

Two auth models, independent of each other:
- **CLI/API clients**: `X-Api-Key: <key>` or `Authorization: Bearer <key>`. Set `FG_API_KEY` env var. Empty = dev mode (no auth).
- **Dashboard login**: session cookie via `/api/v1/auth/login`, enabled by setting `FG_ADMIN_EMAIL` + `FG_ADMIN_PASSWORD` + `FG_SESSION_SECRET` on the API server.

---

## Policy-as-Code

```yaml
# ~/.forgeguardian/policy.yaml
version: 1
fail_on: high
deny_packages:
  - event-stream
  - requests-dmarc
block_typosquatting: true
require_signing: false
```

```bash
fgctl policy check        # evaluate current project against policy
fgctl policy show         # display active policy
fgctl policy set deny=lodash@4.17.20   # add package to blocklist
```

---

## Risk Score

Every package gets a letter grade (A–F) from a composite score:

| Factor | Weight | Signal |
|---|---|---|
| Vulnerability | 0–40 | CVE severity distribution |
| Behavioral | 0–30 | Malware / install script signals |
| Supply Chain | 0–20 | Typosquatting / confusion |
| Maintenance | 0–10 | Abandonment / age |

| Grade | Score | Meaning |
|---|---|---|
| A | 0–20 | Clean |
| B | 21–40 | Low risk |
| C | 41–60 | Review recommended |
| D | 61–80 | High risk — upgrade |
| F | 81–100 | Critical — block |

---

## Self-Hosted Deployment

```bash
docker run -d --name forgeguardian -p 3000:3000 \
  -e FG_ADMIN_EMAIL=you@example.com \
  -e FG_ADMIN_PASSWORD=YourSecurePass \
  ghcr.io/mah3sec/forgeguardian
```

Everything runs locally — no cloud dependency, no telemetry, no data leaves your machine.

---

## Architecture

```
┌─── CLIENTS ──────────────────────────────────────────────────────┐
│  fgctl CLI  •  Dashboard  •  Browser  •  GitHub Actions  │
└──────────────────────────┬───────────────────────────────────────┘
                           │ HTTPS
┌─── AWS VPC ──────────────▼───────────────────────────────────────┐
│  Route 53 → ALB :443                                             │
│                                                                  │
│  ┌── EKS Cluster ───────────────────────────────────────────┐   │
│  │  api :8080  │  worker  │  dashboard :3000                 │   │
│  │  intel-agent (CronJob) │  fg-agent (AI patch)            │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌── Data ──────────────────────────────────────────────────┐   │
│  │  RDS PostgreSQL 16  │  ElastiCache Redis  │  S3 artifacts │   │
│  └──────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
                           │
┌─── EXTERNAL ─────────────▼───────────────────────────────────────┐
│  Anthropic Claude API  •  Sigstore/Rekor  •  OSV+OpenSSF feeds   │
│  npm • PyPI • Go • crates • RubyGems • Maven • HuggingFace • MCP │
└──────────────────────────────────────────────────────────────────┘
```

---

## Trust & Privacy

- All scans run locally — no data sent to external servers by default
- AI features (`advisory`, `patch`, `intel`) require explicit `ANTHROPIC_API_KEY` — fully opt-in
- Zero telemetry — ForgeGuardian phones home for nothing
- Self-hostable and airgap-compatible
- SLSA Level 3 provenance published for every release
- SBOMs published for every release via Sigstore/Rekor

---

## Contributing

Fastest path: **write a detection signature** — no Go knowledge required, takes 10 minutes.

```bash
fgctl intel new    # guided wizard
```

For code: fork → branch → PR. All PRs run Semgrep + unit tests.

- [CONTRIBUTING.md](CONTRIBUTING.md) — development setup
- [SECURITY.md](SECURITY.md) — vulnerability disclosure

---

## Changelog

### v2.0.0 — 2026-06-13
- Full 8-engine scan from dashboard (downloads real artifact, all engines run)
- `POST /api/v1/scan/upload` — scan any uploaded archive via dashboard
- Dashboard ScanPage — 2 tabs (registry + file upload) + engine status bar
- Allowlist API + full CRUD dashboard page
- Alerts API + real-time dashboard page with dismiss
- Live SSE agent feed (`/api/v1/agent/stream`) + AgentsPage
- `fgctl intel` — full Nuclei-style toolkit: `new` / `validate` / `test` / `update` / `list`
- 24 community signatures (blocklisted, typosquatting, behavioral, malware, MCP, AI model)
- API key auth middleware + rate limiter (60 rps / burst 20)
- DB migration runner (embedded SQL, transactional, `schema_migrations` tracking)
- Scraper scheduler `--watch` flag + `FG_SCRAPER_INTERVAL`
- 43 unit tests across core, middleware, policy, notify

### v1.4.0 — 2026-05-24
- Enterprise dashboard UX, brew/gem/docker/PATH audit, SECURITY.md

### v1.3.0 — 2026-05-24
- Machine output correctness, `fgctl stats`, compact grouped mode, filter fixes

---

## License

Apache License 2.0 — see [LICENSE](LICENSE).

ForgeGuardian is free to use, self-host, and fork. Commercial features (SaaS hosting, enterprise SSO, team management) fund continued open-source development.
