# ForgeGuardian — Competitive Positioning

> An honest comparison. We acknowledge where other tools are stronger.

> **Note:** This is an engineering reference document. Comparisons are factual observations, not marketing claims.

---

## Executive Summary

ForgeGuardian is the only open-source supply chain security tool designed from the ground up to cover the full modern software supply chain: traditional package ecosystems (npm, PyPI, Maven, Go, RubyGems, crates.io) **and** the emerging AI/agent ecosystem (HuggingFace model weights, MCP server packages, ONNX model hubs). It adds an AI-native triage and patching layer, a developer-first CLI experience, local-first operation with no mandatory cloud account, and a community signature ecosystem built by and for the security community.

Established tools like Trivy, Grype, Snyk, and Dependency-Track each excel in their domain. ForgeGuardian is not a replacement for all of them — it is the right choice when you need: (1) a single CLI that works locally without an account, (2) AI model and MCP security coverage that no other tool provides, (3) AI-powered triage and autonomous patching, or (4) a community-driven detection layer that extends beyond CVE databases.

---

## Feature Comparison Matrix

| Feature | ForgeGuardian | Trivy | Grype | Snyk | Dependency-Track |
|---|---|---|---|---|---|
| **Deployment** | Local CLI + optional SaaS | Local CLI | Local CLI | SaaS (free tier) | Self-hosted web app |
| **Account required** | No | No | No | Yes (free tier) | No |
| **Open source** | Yes (Apache 2.0) | Yes (Apache 2) | Yes (Apache 2) | No (SSSE license) | Yes (Apache 2) |
| **CVE / GHSA scanning** | Yes (OSV + Grype + Trivy) | Yes | Yes | Yes | Yes (via SBOM import) |
| **HuggingFace model security** | Yes | No | No | No | No |
| **MCP server security** | Yes | No | No | No | No |
| **Behavioral analysis** | Yes | No | No | No | No |
| **Malware / blocklist detection** | Yes | Partial | No | Partial (commercial) | No |
| **Typosquatting detection** | Yes | No | No | No | No |
| **Dependency confusion detection** | Yes | No | No | No | No |
| **Lifecycle script analysis** | Yes | No | No | No | No |
| **AI triage advisory** | Yes | No | No | Partial (fix suggestions) | No |
| **Autonomous patch agent** | Yes | No | No | No | No |
| **SBOM generation** | Yes (CycloneDX + SPDX) | Yes (CycloneDX + SPDX) | No | Yes | Yes |
| **Sigstore / SLSA signing** | Yes | Partial | No | No | No |
| **Community signatures** | Yes (community-format) | No | No | No | No |
| **Policy-as-code (local)** | Yes | Partial (OPA) | No | Yes (cloud) | No |
| **Monitor / live diff** | Yes | No | No | No | No |
| **SARIF output** | Yes | Yes | Yes | Yes | Partial |
| **Pure stdout JSON/SARIF (no banner leak)** | Yes (v1.4.0) | Yes | Yes | Partial | N/A |
| **VS Code extension** | Yes | Partial | No | Yes | No |
| **Container image scanning** | Partial (Dockerfile) | Yes (best-in-class) | Yes | Yes | No |
| **K8s misconfiguration** | No | Yes | No | Yes | No |
| **Risk score / grade** | Yes (A–F) | No | No | Yes (priority score) | Yes |
| **Offline / airgap support** | Yes | Yes | Yes | No | Partial |
| **Self-hostable** | Yes | Yes | Yes | No | Yes |

---

## ForgeGuardian vs Trivy

Trivy is one of the most complete open-source security scanners available. It excels at container image scanning, Kubernetes misconfiguration detection, and has a mature vulnerability database with broad OS package coverage. For teams primarily focused on container and infrastructure security, Trivy is an excellent choice.

ForgeGuardian adds capabilities Trivy does not have:

| Capability | ForgeGuardian | Trivy |
|---|---|---|
| AI triage & advisory | Yes — Claude-powered, per-finding | No |
| Autonomous patch agent | Yes — proposes + validates upgrades | No |
| HuggingFace / MCP security | Yes — first-in-class | No |
| Behavioral / script analysis | Yes | No |
| Typosquatting detection | Yes | No |
| Community signatures | Yes — community signature ecosystem | No |
| Policy-as-code (local YAML) | Yes | Partial (OPA config policies) |
| Autonomous monitor + diff | Yes | No |

Trivy is stronger than ForgeGuardian in:
- **Container image scanning depth** — Trivy has the most complete OS package CVE coverage for container images (Debian, Alpine, RHEL, Ubuntu) and scans container layer by layer
- **Kubernetes misconfiguration** — Trivy's `trivy k8s` is purpose-built for this; ForgeGuardian has no equivalent
- **Infrastructure-as-code scanning** — Trivy scans Terraform, CloudFormation, Helm charts; ForgeGuardian does not

**Recommendation:** Use ForgeGuardian for application dependency and supply chain scanning (npm, PyPI, Go, AI/MCP). Use Trivy for container and infrastructure scanning. They complement each other well.

---

## ForgeGuardian vs Grype

Grype is a fast, focused CVE scanner for SBOMs and container images. Its strength is speed and database freshness — it auto-updates its vulnerability database and produces clean, well-formatted output. It is widely used in CI pipelines that already produce SBOMs separately.

ForgeGuardian adds capabilities Grype does not have:

| Capability | ForgeGuardian | Grype |
|---|---|---|
| Behavioral analysis | Yes | No |
| Malware / blocklist detection | Yes | No |
| Typosquatting detection | Yes | No |
| AI triage advisory | Yes | No |
| Autonomous patch agent | Yes | No |
| Monitor mode (live diff) | Yes | No |
| Community signatures | Yes | No |
| SBOM generation | Yes | No (consumes SBOMs) |
| HuggingFace / MCP security | Yes | No |

Grype is stronger than ForgeGuardian in:
- **CVE database update speed** — Grype's vulnerability DB updates more frequently and has a mature caching mechanism
- **SBOM consumption** — Grype accepts SBOMs as input directly, making it easy to integrate into pipelines that already generate SBOMs with other tools (Syft, etc.)

**Recommendation:** ForgeGuardian replaces Grype as the primary scanner for application supply chain security. If your pipeline uses Syft + Grype specifically for SBOM-in → CVE-out workflows, ForgeGuardian's `sbom` + `scan` commands cover the same flow.

---

## ForgeGuardian vs Snyk

Snyk is a mature commercial SCA (Software Composition Analysis) platform with strong PR automation, a large vulnerability database, and license scanning. Its free tier is generous but requires an account and sends dependency data to Snyk's cloud.

Key differences:

| Dimension | ForgeGuardian | Snyk |
|---|---|---|
| Account required | No | Yes |
| Data sent to cloud | No (local-first) | Yes (dependency data sent to Snyk) |
| Open source | Yes (Apache 2.0) | No (source-available, SSSE license) |
| AI triage advisory | Yes (Claude, opt-in) | Partial (DeepCode AI suggestions) |
| Autonomous patch agent | Yes | No |
| HuggingFace / MCP security | Yes | No |
| Community signatures | Yes | No |
| Local enforcement | Yes | Requires Snyk CLI + account |
| Self-hostable | Yes | No (requires Snyk cloud) |
| PR automation | No | Yes (strong integration) |
| License scanning | Basic | Yes (comprehensive) |
| Enterprise support | No | Yes |

Snyk is stronger than ForgeGuardian in:
- **PR automation** — Snyk Fix PRs are the gold standard for automated dependency upgrade PRs at scale, with compatibility testing
- **License scanning** — Snyk's license compliance scanning is more comprehensive and enterprise-ready
- **Enterprise integrations** — Snyk integrates with Jira, ServiceNow, and enterprise ticketing systems out of the box

**Recommendation:** For teams that need local-first, open-source, or AI/MCP security coverage, ForgeGuardian is the better fit. For teams that need enterprise PR automation and license compliance at scale, Snyk's commercial offering may justify the subscription cost.

---

## ForgeGuardian vs Dependency-Track

Dependency-Track is a mature, OWASP-maintained platform for continuous SBOM monitoring. It is excellent at enterprise BOM management, policy evaluation, and compliance reporting across large portfolios.

Key differences:

| Dimension | ForgeGuardian | Dependency-Track |
|---|---|---|
| Primary interface | CLI-first | Web UI |
| SBOM ingestion | Generates SBOMs | Consumes SBOMs |
| AI triage | Yes | No |
| Developer workflow | Yes (local scan, pre-commit) | No (no CLI for local dev) |
| HuggingFace / MCP security | Yes | No |
| Enterprise portfolio management | No | Yes |
| Policy evaluation | Local YAML | Sophisticated web UI |
| Compliance reporting | Basic | Strong (SOC 2, ISO 27001) |
| Metrics & dashboards | Prometheus / Grafana | Built-in (Grafana-style) |

Dependency-Track is stronger than ForgeGuardian in:
- **Enterprise BOM management** — tracking SBOMs across hundreds of projects, projects/versions/components hierarchy
- **Compliance reporting** — purpose-built for SOC 2, FedRAMP, and ISO 27001 supply chain controls
- **Policy sophistication** — Dependency-Track's policy conditions cover license groups, CVSSv3 attributes, and component age

**Recommendation:** ForgeGuardian and Dependency-Track are complementary. Use ForgeGuardian at the developer and CI layer to scan before commit. Feed SBOMs generated by `fgctl sbom` into Dependency-Track for enterprise portfolio monitoring. The docker-enterprise compose profile ships Dependency-Track pre-integrated for exactly this workflow.

---

## ForgeGuardian vs OSV Scanner

The OSV Scanner (by Google) is a focused tool that queries the OSV database for vulnerable packages. It is fast, accurate, and well-maintained.

| Dimension | ForgeGuardian | OSV Scanner |
|---|---|---|
| CVE coverage | OSV + Grype + Trivy (3 engines) | OSV only |
| Behavioral analysis | Yes | No |
| Malware detection | Yes | No |
| Typosquatting | Yes | No |
| AI model security | Yes | No |
| MCP security | Yes | No |
| SBOM generation | Yes | No |
| Signing | Yes | No |
| Monitor mode | Yes | No |

**Recommendation:** ForgeGuardian uses OSV as one of its three CVE engines. If all you need is fast CVE checking via OSV, the OSV Scanner is lighter-weight. If you need the full supply chain security picture, ForgeGuardian is the right choice.

---

## Key Differentiators

Five things only ForgeGuardian does:

**1. AI Model & MCP Server Security (first-in-class)**
No other open-source tool scans HuggingFace model weights for pickle exploits, checks `config.json` for `trust_remote_code`, detects MCP prompt injection, or identifies MCP tool shadowing. As AI agents become a primary attack surface, ForgeGuardian is the only tool covering this space.

**2. Autonomous AI Patching with Compatibility Validation**
`fgctl patch .` delegates to the `fg-agent` binary, which uses Claude's tool-use API to propose dependency upgrades, validate compatibility, and apply changes. This is not a simple "update to latest" operation — the agent reasons about breaking changes and proposes the safest upgrade path.

**3. Policy-as-Code with Local Enforcement — No Cloud Required**
`~/.forgeguardian/policy.yaml` lets you define `fail_on`, `deny_packages`, and `min_score` rules that are evaluated locally on every scan. No cloud account, no policy server, no network call. Works in airgapped environments.

**4. Community Signature Ecosystem (like community detection signatures, but for supply chain)**
The community can publish detection signatures for new threats — malicious packages, typosquatting targets, behavioral rules, MCP injection patterns — and every user gets them with `fgctl update`. This creates a crowd-sourced threat intelligence layer that grows faster than any single vendor's research team can maintain.

**5. Full Supply Chain Pipeline in One Tool**
`scan → advisory → patch → sbom → sign → verify → monitor` — from vulnerability detection to cryptographic attestation, all in one CLI. No need to glue together six separate tools with incompatible output formats.

---

## What ForgeGuardian Does Not Do

Honesty about limitations:

- **No runtime security** — ForgeGuardian is a static analysis and signature-matching tool. It does not monitor running processes or network connections at runtime.
- **No PR automation (yet)** — Unlike Snyk, ForgeGuardian does not open pull requests automatically. `fgctl patch` applies changes locally; PR creation is left to the user.
- **No enterprise license compliance** — ForgeGuardian does not have Snyk's or FOSSA's comprehensive license compliance scanning.
- **No Kubernetes misconfiguration scanning** — Trivy's `trivy k8s` is the right tool for this.
- **No deep container layer analysis** — ForgeGuardian scans Dockerfiles and docker-compose files but does not scan container image layers like Trivy does.
- **No commercial support** — ForgeGuardian is community-supported. Enterprise support SLAs are not yet available.
