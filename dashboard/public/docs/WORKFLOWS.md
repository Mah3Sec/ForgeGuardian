# ForgeGuardian Workflows

> Five end-to-end workflows covering every persona that uses ForgeGuardian — from a developer on their laptop to a compliance officer generating audit artifacts.

---

## Table of Contents

1. [Developer Workflow](#1-developer-workflow)
2. [DevOps / CI/CD Workflow](#2-devops--cicd-workflow)
3. [Security Team Workflow](#3-security-team-workflow)
4. [SOC Workflow](#4-soc-workflow)
5. [Compliance Workflow](#5-compliance-workflow)

---

## 1. Developer Workflow

**Use case:** Catch vulnerable or malicious dependencies before they reach a commit. Get AI-generated fix guidance without leaving the terminal.

**Persona:** A developer working locally — on any OS — who wants fast feedback without context-switching to a web UI.

### Flow

```
  Edit code / add dependency
         │
         ▼
   fgctl scan .                ← scan all manifests in current project
         │
    findings?
   ┌─────┴─────┐
   No          Yes
   │           │
   Commit      ▼
           fgctl advisory npm/lodash@4.17.20
                │
                ▼
           fgctl patch . --project-dir=.
                │
                ▼
           Review proposed changes
                │
                ▼
              Commit
```

### Step-by-Step

**Step 1 — Install and initialize (once)**

```bash
go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest
fgctl doctor --fix     # auto-installs missing tools, validates env; prints each repair command before running
fgctl update           # download community signatures
fgctl stats            # verify signatures are loaded
fgctl stats --json     # machine-readable for scripting
```

**Step 2 — Scan the project**

```bash
$ fgctl scan .

  ███████╗ ...  ForgeGuardian v1.3.0

  Scanning /home/alice/projects/myapp...

  package.json  (npm)
  ─────────────────────────────────────────────────────
  lodash@4.17.20  [grade D · 2 findings]
  ├─ HIGH      CVE-2021-23337  Prototype Pollution          → fix: >= 4.17.21
  └─ MEDIUM    CVE-2019-10744  Prototype Pollution (assign) → fix: >= 4.17.12

  express@4.18.1  [grade C · 1 finding]
  └─ MEDIUM    CVE-2024-29041  Open Redirect                → fix: >= 4.19.2

  ─────────────────────────────────────────────────────
  3 findings  ·  0 CRITICAL  ·  1 HIGH  ·  2 MEDIUM  ·  0 LOW

  Run 'fgctl advisory npm/lodash@4.17.20' for AI remediation.
  Use --verbose to expand all findings.
```

**Step 3 — Compact mode for a quick glance**

`--compact` groups output by package (one line per package with severity bracket counts):

```bash
$ fgctl scan . --compact

  lodash@4.17.20      [1 HIGH, 1 MEDIUM]    fix: 4.17.21
  express@4.18.1      [1 MEDIUM]            fix: 4.19.2
```

**Step 4 — Get an AI advisory for the critical finding**

```bash
$ fgctl advisory npm/lodash@4.17.20

Advisory for lodash@4.17.20 (npm)
  Severity:   HIGH (confidence 94%)
  Advisory:   CVE-2021-23337 is a prototype pollution vulnerability in lodash's
              merge(), mergeWith(), and defaultsDeep() functions. An attacker
              who can control a deep merge operation can pollute Object.prototype,
              potentially enabling denial-of-service or remote code execution
              depending on how the application uses the merged result downstream.
  Action:     Upgrade to lodash@4.17.21 immediately. The fix is non-breaking
              (patch release). Run: npm install lodash@4.17.21
```

**Step 5 — Run the autonomous patch agent**

```bash
$ fgctl patch . --project-dir=.
  fg-agent applying patches for /home/alice/projects/myapp
  ✓ lodash: 4.17.20 → 4.17.21 (compatibility: high, breaking changes: none)
  ✓ express: 4.18.1 → 4.19.2 (compatibility: high, breaking changes: none)
  Patch applied. Review changes: git diff
```

**Step 6 — Only show fixable findings**

```bash
fgctl scan . --only-fixable --compact
```

### Pre-Commit Hook

Add this to `.git/hooks/pre-commit` to block commits with critical findings:

```bash
#!/bin/sh
# ForgeGuardian pre-commit hook
if command -v fgctl &>/dev/null; then
  fgctl scan . --quiet --fail-on=critical
  if [ $? -eq 2 ]; then
    echo "ForgeGuardian: CRITICAL vulnerability detected. Commit blocked."
    echo "Run 'fgctl scan .' for details."
    exit 1
  fi
fi
```

Make it executable: `chmod +x .git/hooks/pre-commit`

---

## 2. DevOps / CI/CD Workflow

**Use case:** Enforce a security policy gate on every PR. Upload SARIF results to GitHub Code Scanning. Sign release artifacts.

**Persona:** A DevOps or platform engineer who owns the CI/CD pipeline.

### Flow

```
  PR opened
      │
      ▼
  fgctl update          ← fetch latest signatures
      │
      ▼
  fgctl scan .          ← multi-engine scan
  --fail-on=high
  --format=sarif
      │
  ┌───┴───────────────┐
  pass (exit 0)       fail (exit 2)
  │                   │
  ▼                   ▼
  fgctl sign          Block merge + annotate PR
  artifact            with SARIF findings
  │
  ▼
  Publish artifact
```

### CI Shortcut Flag

`--ci` is a single-flag shortcut equivalent to `--quiet --format=sarif --fail-on=high`. As of v1.3.0, `--ci` produces **pure stdout SARIF** with zero human text — no banner, no ANSI codes. The v1.2.0 bug where the banner appeared even with `--ci` is fixed.

```bash
fgctl scan . --ci            # equivalent to: --quiet --format=sarif --fail-on=high
fgctl scan . --ci --fail-on=critical  # override the threshold
```

The individual flags remain available for fine-grained control (see examples below).

### GitHub Actions — Full Example

```yaml
name: ForgeGuardian Supply Chain Scan

on:
  pull_request:
  push:
    branches: [main]

jobs:
  supply-chain-scan:
    runs-on: ubuntu-latest
    permissions:
      security-events: write
      contents: read

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"

      - name: Install ForgeGuardian
        run: go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest

      - name: Fetch community signatures
        run: fgctl update

      - name: Run supply chain scan
        # v1.3.0: --format=sarif produces pure stdout SARIF — no banner or ANSI contamination.
        # Alternatively, use --ci which sets --quiet --format=sarif --fail-on=high automatically.
        run: |
          fgctl scan . \
            --format=sarif \
            --fail-on=high \
            --timeout=15m \
            > forgeguardian.sarif
        continue-on-error: true  # let the SARIF upload happen even on policy failure

      - name: Upload SARIF to GitHub Code Scanning
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: forgeguardian.sarif

      - name: Enforce policy gate
        run: fgctl scan . --quiet --fail-on=high

      - name: Sign release artifact (main branch only)
        if: github.ref == 'refs/heads/main'
        env:
          COSIGN_EXPERIMENTAL: 1
        run: |
          fgctl sign \
            --recipe=npm \
            --package=my-package \
            --version=${{ github.sha }} \
            --out=attestation.json

      - name: Upload attestation
        if: github.ref == 'refs/heads/main'
        uses: actions/upload-artifact@v4
        with:
          name: supply-chain-attestation
          path: attestation.json
```

### GitLab CI — Example

```yaml
forgeguardian-scan:
  stage: security
  image: golang:1.23-alpine
  before_script:
    - go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest
    - fgctl update
  script:
    # v1.3.0: --format=sarif produces pure stdout SARIF with no banner contamination
    - fgctl scan . --format=sarif --fail-on=high > gl-sast-report.sarif
  artifacts:
    reports:
      sast: gl-sast-report.sarif
  allow_failure: false
```

### Exit Codes

| Exit Code | Meaning |
|---|---|
| 0 | Clean scan — no findings at or above `--fail-on` threshold |
| 1 | Scan error (tool failure, network error, invalid args) |
| 2 | Policy violation — findings at or above `--fail-on` threshold |

Use exit code 2 to distinguish a security failure from an infrastructure failure.

---

## 3. Security Team Workflow

**Use case:** Centralized review of risk across all projects. Investigate high-severity findings. Generate advisories. Measure remediation velocity.

**Persona:** An AppSec engineer or security lead responsible for supply chain risk across a portfolio of services.

### Flow

```
  Open ForgeGuardian dashboard
         │
         ▼
  Review Risk Score grades        ← A–F per project
         │
         ▼
  Investigate CRITICAL findings   ← Scan + Advisory pages
         │
         ▼
  Generate AI advisories          ← fgctl advisory or dashboard Advisory tab
         │
         ▼
  Track remediation in Monitor    ← MonitorPage, live polling
         │
         ▼
  Review intelligence coverage    ← Intelligence tab, community sigs
         │
         ▼
  Audit system-wide               ← fgctl audit system
```

### Step-by-Step

**Step 1 — Audit all globally installed packages**

`fgctl audit system` outputs a grouped ecosystem summary table (not a per-package flood), making it practical for large global installs:

```bash
$ fgctl audit system

  Auditing globally installed packages...

  Ecosystem   Packages   Findings   Critical   High   Medium   Low
  ─────────────────────────────────────────────────────────────────
  npm          34         2          0          1      1        0
  pip          127        1          0          0      1        0
  ─────────────────────────────────────────────────────────────────
  Total        161        3          0          1      2        0

  Run 'fgctl audit system --verbose' to expand per-package findings.
```

**Step 2 — Generate an advisory for a critical finding**

```bash
$ fgctl advisory npm/event-stream@3.3.6

Advisory for event-stream@3.3.6 (npm)
  Severity:   CRITICAL (confidence 99%)
  Advisory:   event-stream@3.3.6 is a confirmed supply chain attack. The
              flatmap-stream dependency injected into this version harvested
              Bitcoin wallet private keys. This package should be removed
              immediately and treated as a full compromise of the host.
  Action:     Remove event-stream entirely. The original maintainer has not
              re-assumed control. Use the 'streams' package as a replacement.
```

**Step 3 — Review the intelligence module**

```bash
$ fgctl update
  Added 12 new signatures (847 → 859 total).

# Verify loaded signature count after an update
$ fgctl stats
# Machine-readable output for scripting / CI health checks
$ fgctl stats --json
```

Monitor what new threats the community has published. The Intelligence tab in the dashboard shows all active signatures by type, ecosystem, and severity.

**Step 4 — Check the risk score across a project**

The ForgeGuardian Risk Score aggregates all findings into a single letter grade:

| Grade | Description | Action |
|---|---|---|
| A | No actionable findings | No immediate action |
| B | Only low-severity findings | Schedule review |
| C | Medium severity present | Address within sprint |
| D | High severity present | Address within 48 hours |
| F | Critical severity present | Address immediately |

**Step 5 — Review historical scan trends**

Open the dashboard at `http://localhost:3000`. The Dashboard page shows a 30-day timeline chart of findings by severity, a risk score trend, and a breakdown by ecosystem.

---

## 4. SOC Workflow

**Use case:** Real-time detection of new malicious package introductions. Alert on suspicious dependency changes. Investigate and respond via policy.

**Persona:** A SOC analyst monitoring a production software development environment.

### Flow

```
  fgctl monitor --watch .      ← polling every 3s, baseline comparison
         │
         ▼
  Manifest change detected     ← package.json, requirements.txt, etc.
         │
         ▼
  Auto-rescan triggered
         │
  ┌──────┴──────────────┐
  No new findings        New findings
  │                      │
  Log "no change"        ▼
                    Diff displayed
                         │
                    ┌────┴──────────────────────────────┐
                    Informational                    HIGH/CRITICAL
                    │                                    │
                    Log                            Webhook alert sent
                                                        │
                                                        ▼
                                               Investigate via dashboard
                                                        │
                                                        ▼
                                               Acknowledge or add to
                                               policy.yaml deny_packages
```

### Step-by-Step

**Step 1 — Start continuous monitoring**

```bash
$ fgctl monitor --watch . --interval=5s

  ForgeGuardian monitor watching /home/alice/myapp
  Press Ctrl+C to stop.

  [14:02:01] Initial scan: 0 finding(s)
  [14:03:45] package.json changed — rescanning...
  [14:03:47] NEW [CRITICAL]  event-stream@3.3.6 — Known supply chain attack (blocklisted)
  [14:03:47] 1 new, 0 resolved
```

**Step 2 — Configure webhook notifications**

Add to `~/.forgeguardian/config.yaml`:

```yaml
notifications:
  slack_webhook: https://hooks.slack.com/services/T.../B.../xxx
  discord_webhook: https://discord.com/api/webhooks/.../xxx
  min_severity: high   # only alert on HIGH and CRITICAL
```

Slack alert format:
```
[CRITICAL] ForgeGuardian Alert
Project: /home/alice/myapp
Finding: event-stream@3.3.6 — Known supply chain attack
Action: Remove this package immediately
```

**Step 3 — Investigate via the MonitorPage**

Open `http://localhost:3000/monitor`. The MonitorPage polls the API every 10 seconds and shows:
- Live finding count with severity breakdown
- Recent scan timeline
- Per-manifest finding diffs

**Step 4 — Block a package via policy**

Once confirmed malicious, add it to the local policy to gate future installs:

```yaml
# ~/.forgeguardian/policy.yaml
deny_packages:
  - npm/event-stream
  - npm/flatmap-stream
fail_on: critical
```

**Step 5 — Run a full system audit after an incident**

```bash
fgctl audit system     # enumerate all globally installed packages
fgctl debug --json     # dump diagnostic state for incident report
```

`fgctl debug` output:

```json
{
  "version": "1.3.0",
  "os": "linux",
  "arch": "amd64",
  "signatures": "859 sigs, modified 2026-05-24",
  "api_status": "OK (200)",
  "anthropic_api_key": "SET (sk-ant-...key)",
  "disk_free": "142.3 GB"
}
```

---

## 5. Compliance Workflow

**Use case:** Generate SBOMs, sign artifacts, produce SLSA provenance, and verify attestations for audits (SOC 2, FedRAMP, ISO 27001 supply chain controls).

**Persona:** A compliance officer, auditor, or security architect who needs cryptographic proof of artifact integrity and provenance.

### Flow

```
  Build artifact
       │
       ▼
  fgctl sbom                  ← generate CycloneDX or SPDX SBOM
       │
       ▼
  fgctl scan . --format=sarif ← generate vulnerability evidence
       │
       ▼
  fgctl sign                  ← keyless sign via Sigstore + upload to Rekor
       │
       ▼
  fgctl provenance            ← generate SLSA v1.0 provenance statement
       │
       ▼
  fgctl verify                ← verify attestation at any time
       │
       ▼
  Archive: sbom.json,
           sarif-report.sarif,
           attestation.json,
           provenance.json
```

### Step-by-Step

**Step 1 — Generate a CycloneDX SBOM**

```bash
$ fgctl sbom \
    --recipe=npm \
    --package=lodash \
    --version=4.17.21 \
    --format=cyclonedx-json \
    --out=sbom-lodash-4.17.21.json

# or SPDX format for NTIA compliance
$ fgctl sbom \
    --recipe=npm \
    --package=lodash \
    --version=4.17.21 \
    --format=spdx-json \
    --out=sbom-lodash-4.17.21.spdx.json
```

Supported formats: `cyclonedx-json`, `cyclonedx-xml`, `spdx-json`, `spdx-tv`

**Step 2 — Generate vulnerability evidence (SARIF)**

```bash
$ fgctl scan \
    --recipe=npm \
    --package=lodash \
    --version=4.17.21 \
    --format=sarif \
    > vuln-report-lodash-4.17.21.sarif
```

**Step 3 — Sign the artifact and upload to Rekor**

```bash
$ fgctl sign \
    --recipe=npm \
    --package=lodash \
    --version=4.17.21 \
    --out=attestation-lodash-4.17.21.json

  INFO signing artifact sha256=abc123...
  INFO uploaded to Rekor log_id=24296fb24b8ad77a index=12345678
```

The attestation JSON contains:
- Artifact SHA256
- Build timestamp
- Rekor log ID and index (for public verifiability)
- SLSA provenance fields

**Step 4 — Generate SLSA provenance**

```bash
$ fgctl provenance \
    --recipe=npm \
    --package=lodash \
    --version=4.17.21 \
    --out=provenance-lodash-4.17.21.json
```

The provenance document conforms to SLSA v1.0 and includes:
- Builder identity
- Build invocation parameters
- Dependency inputs (from SBOM)
- Build timestamp

**Step 5 — Verify an attestation**

```bash
$ fgctl verify \
    --attestation=attestation-lodash-4.17.21.json \
    --sha256=<expected-sha256>

{
  "valid": true,
  "sha256_match": true,
  "rekor_verified": true,
  "timestamp": "2026-05-24T14:03:22Z",
  "rekor_log_id": "24296fb24b8ad77a",
  "rekor_index": 12345678
}
```

**Step 6 — Archive compliance artifacts**

For each release, archive:

```
release-v1.2.3/
├── sbom.cyclonedx.json          # CycloneDX SBOM
├── sbom.spdx.json               # SPDX SBOM (NTIA minimum elements)
├── vulnerability-report.sarif   # SARIF vulnerability evidence
├── attestation.json             # Sigstore attestation + Rekor reference
└── provenance.json              # SLSA v1.0 provenance
```

Rekor entries are permanent and publicly verifiable at `https://search.sigstore.dev`.

### Self-Hosted Rekor (Airgapped)

For environments that cannot use the public Sigstore infrastructure:

```bash
# Start the local Rekor server
docker compose --profile enterprise up rekor-server

# Configure fgctl to use it
fgctl config set signing.rekor_url=http://localhost:3001

# All subsequent sign/verify operations use local Rekor
fgctl sign --recipe=npm --package=lodash --version=4.17.21 --out=att.json
```

### SLSA Level 3 Compliance

ForgeGuardian's release pipeline achieves SLSA Level 3 for its own artifacts:
- Hermetic builds via GitHub Actions
- Two-party review (PR approval)
- Signed provenance uploaded to public Rekor
- SBOMs published with each release

Use `fgctl verify --attestation=<fg-release-att.json> --sha256=<hash>` to verify any ForgeGuardian binary you downloaded.
