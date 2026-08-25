<p align="center">
  <img src="dashboard/public/logo.png" alt="ForgeGuardian" width="80"/>
</p>

<h1 align="center">ForgeGuardian</h1>

<p align="center">
  <strong>Supply chain security scanner for every package you depend on.</strong><br/>
  8 engines. 9 ecosystems. 223+ detection signatures. Works offline.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/License-Apache%202.0-green?style=flat-square" alt="License"/>
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go" alt="Go"/>
  <img src="https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-blue?style=flat-square" alt="Platform"/>
  <img src="https://img.shields.io/badge/Signatures-223+-orange?style=flat-square" alt="Signatures"/>
</p>

<p align="center">
  <img src="docs/images/dashboard-overview.png" alt="ForgeGuardian Dashboard" width="900"/>
</p>

---

## Install

One command. No account needed.

```bash
curl -sSfL https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.sh | bash
```

<details>
<summary>Windows (PowerShell)</summary>

```powershell
irm https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.ps1 | iex
```
</details>

<details>
<summary>Docker</summary>

```bash
docker run -d --name forgeguardian -p 3000:3000 ghcr.io/mah3sec/forgeguardian
```
Open **http://localhost:3000**
</details>

<details>
<summary>Build from source</summary>

```bash
git clone https://github.com/Mah3Sec/ForgeGuardian.git
cd ForgeGuardian && make build
```
</details>

---

## Scan

```bash
fgctl scan .
```

That's it. Scans your project, shows findings with severity and fix versions.

---

## Dashboard

Start the web dashboard:

```bash
fgctl serve
```

Open **http://localhost:8080** — SOC-style overview with security posture grading, severity trends, real-time alerts, and risk heatmaps.

**Live demo:** [forgeguardian.mahendrapurbia.com](https://forgeguardian.mahendrapurbia.com)

### 30+ pages across 7 categories

**Analyze** — Multi-engine vulnerability scanner (registry + file upload + remote SSH), scan session history with JSON/CSV/HTML export, recursive directory scanning, system audit across all package managers

**AI-Powered** — AI security analysis, AI advisory with remediation guidance, autonomous patch agent

**Monitor** — Live monitoring with auto-quarantine, structured log viewer, dependency topology with attack surface mapping, dependency drift detection, alert timeline

<p align="center">
  <img src="docs/images/attack-surface.png" alt="Attack Surface — Dependency Topology" width="900"/>
</p>

**Inventory** — Multi-workspace project management, dependency inventory with risk grades, SBOM generation (CycloneDX + SPDX)

**Policy** — Policy-as-code rules, allowlist/blocklist management, Sigstore keyless signing + verification, provenance tracking, signature authoring with guided wizard

**Integrations** — Webhook alerts (Slack, Discord, HTTP), CI/CD pipeline config, report exports

**Tools** — Built-in web terminal for CLI commands, developer docs, API reference, settings

<p align="center">
  <img src="docs/images/scan-now.png" alt="Vulnerability Scanner" width="900"/>
</p>

---

## 8 Scan Engines

Every scan runs all available engines concurrently:

| Engine | Status | What it catches |
|---|---|---|
| **OSV** | always runs | Known CVEs via osv.dev |
| **Behavioral** | always runs | Malicious install scripts, typosquatting |
| **Malware** | always runs | Byte/regex pattern matching |
| **AI Model** | always runs | Unsafe HuggingFace weights |
| **MCP** | always runs | Prompt injection in tool descriptions |
| **Grype** | optional | Deep CVE scan of artifacts |
| **Trivy** | optional | Container + OS scanning |
| **Semgrep** | optional | SAST static analysis |

Missing an optional engine? Run `fgctl doctor --fix` to auto-install them.

---

## 9 Ecosystems

npm, PyPI, Go, Maven, RubyGems, Cargo/crates.io, NuGet, HuggingFace, GitHub Actions

---

## 223+ Community Signatures

ForgeGuardian ships with 223+ detection signatures covering real supply chain attacks (2016-2026):

| Type | What it catches | Count |
|---|---|---|
| `blocklisted_package` | Confirmed malicious packages (event-stream, XZ utils, Shai-Hulud worm...) | 80 |
| `behavioral_rule` | Install-time env harvest, SSH key theft, dep confusion | 42 |
| `malware_pattern` | Obfuscated loaders, crypto miners, RATs, credential stealers | 44 |
| `typosquatting_target` | Popular packages + known typosquat variants | 27 |
| `mcp_injection_pattern` | Tool shadowing, data exfil via MCP | 13 |
| `pickle_rule` | Unsafe AI model weights, missing model cards | 12 |

**Update signatures:**
```bash
fgctl update
```

**Create your own:**
```bash
fgctl intel new          # guided wizard
fgctl intel validate .   # validate schema
fgctl intel test .       # test against real package
```

---

## CI/CD

Drop into any GitHub Actions workflow:

```yaml
- name: Install ForgeGuardian
  run: |
    curl -sSfL https://raw.githubusercontent.com/Mah3Sec/ForgeGuardian/main/install.sh | bash
    echo "$HOME/.local/bin" >> $GITHUB_PATH

- name: Scan
  run: fgctl scan . --ci --fail-on=high --format=sarif > results.sarif

- name: Upload to GitHub Security
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: results.sarif
```

---

## Key Features

- **Offline-first** — all scans run locally, no data leaves your machine
- **AI triage** — optional AI advisory and patch agent (needs `ANTHROPIC_API_KEY`)
- **Multi-workspace** — organize projects into workspaces with independent scan histories
- **SBOM** — CycloneDX 1.5 + SPDX 2.3 generation
- **Sigstore signing** — keyless artifact signing + verification
- **Policy-as-code** — YAML policy rules, deny lists, threshold enforcement
- **Attack surface mapping** — dependency topology graph with risk visualization
- **Webhooks** — Slack, Discord, generic HTTP alerts
- **Risk scoring** — A-F letter grades per package
- **Scan sessions** — full history with JSON/CSV/HTML export
- **Web terminal** — run CLI commands from the dashboard
- **Self-hostable** — Docker one-liner, airgap-compatible
- **SLSA Level 3** — provenance for every release

---

## Free vs Pro

The engine, CLI, and community tools are **Apache 2.0, free forever**. Pro adds team features.

| | Community (Free) | Pro |
|---|---|---|
| CLI scan + all 8 engines | Yes | Yes |
| SBOM, signing, provenance | Yes | Yes |
| Community signatures | Yes | Yes |
| Dashboard (self-hosted) | Yes | Yes |
| Alerts, policies, webhooks | Yes | Yes |
| AI advisory + patch agent | Yes | Yes |
| Team management + RBAC | — | Yes |
| Cloud-hosted option | — | Yes |
| SLA + priority support | — | Yes |

---

## Privacy

- Zero telemetry — phones home for nothing
- AI features are opt-in (explicit `ANTHROPIC_API_KEY`)
- Self-hostable and airgap-compatible
- SBOMs and provenance published for every release

---

## Contributing

Fastest path: **write a detection signature** — no Go knowledge needed:

```bash
fgctl intel new    # guided wizard, ~10 minutes
```

For code contributions: fork, branch, PR. See [CONTRIBUTING.md](CONTRIBUTING.md).

Security issues: [SECURITY.md](SECURITY.md).

---

## License

Apache License 2.0 — [LICENSE](LICENSE)
