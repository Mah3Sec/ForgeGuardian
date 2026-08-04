# ForgeGuardian — Trust & Privacy Architecture

> This document explains exactly what data ForgeGuardian accesses, what (if anything) leaves your machine, and how to run it with zero external connections. No marketing language. Just the facts.

---

## Philosophy

ForgeGuardian was designed with a local-first architecture. The default assumption is that your code, your dependencies, and your development environment are sensitive. We made zero telemetry and zero mandatory cloud calls the baseline, not an enterprise-tier option.

The corollary: AI features require an external API call because LLMs don't run locally at production quality yet. Those features are fully opt-in and clearly documented below.

---

## Data Flow by Command

The table below summarizes every external call ForgeGuardian makes, by command. "External" means a network connection that leaves your machine.

| Command | External Calls | What Is Sent | To Whom |
|---|---|---|---|
| `fgctl scan .` | None | Nothing | Nobody |
| `fgctl scan npm/lodash@4.17.20` | Yes | Package name + version | OSV API, Grype/Trivy (local DB) |
| `fgctl advisory` | Yes (opt-in) | Finding summary | Anthropic API |
| `fgctl patch` | Yes (opt-in) | Finding summary + upgrade plan | Anthropic API |
| `fgctl update` | Yes | None (download only) | GitHub Releases |
| `fgctl sign` | Yes | Artifact hash | Rekor (Sigstore) or self-hosted |
| `fgctl verify` | Yes | Artifact hash | Rekor (Sigstore) or self-hosted |
| `fgctl sbom` | None | Nothing | Nobody |
| `fgctl provenance` | None | Nothing | Nobody |
| `fgctl monitor --watch .` | None | Nothing | Nobody |
| `fgctl audit system` | None | Nothing | Nobody |
| `fgctl doctor` | None | Nothing | Nobody |
| `fgctl debug` | None | Nothing | Nobody |
| `fgctl config` | None | Nothing | Nobody |
| `fgctl sig validate` | None | Nothing | Nobody |
| Web dashboard | None | Nothing | Nobody |

---

## Per-Command Data Handling

### `fgctl scan .` — Local Project Scan

**No external calls.**

The local scanner (`internal/localscanner`) walks your project directory, parses manifest files, and runs all scan engines locally. The OSV engine queries a locally cached vulnerability database if available. Grype and Trivy use locally downloaded databases (updated with `grype db update` / `trivy image --download-db-only`).

Nothing leaves your machine. This command works completely offline.

### `fgctl scan npm/lodash@4.17.20` — Package Scan (dot-notation)

**Makes external calls.**

When you scan a specific package by name, ForgeGuardian:
1. Downloads the package archive from the registry (npm, PyPI, crates.io, etc.)
2. Queries the OSV API (`api.osv.dev`) with the package name and version
3. Runs Grype and Trivy against the downloaded archive using local databases

What is sent to OSV API: `{"package": {"name": "lodash", "ecosystem": "npm"}, "version": "4.17.20"}` — package name and version only. No file contents, no source code, no environment information.

The package archive is downloaded to a temporary directory and deleted after the scan.

### `fgctl advisory` — AI Advisory

**Makes external calls. Requires ANTHROPIC_API_KEY. Fully opt-in.**

When you run an AI advisory, ForgeGuardian sends a finding summary to the Anthropic Claude API. The summary includes:
- Package name, ecosystem, and version
- Finding IDs (CVE IDs, finding titles)
- Severity levels
- Brief descriptions from the vulnerability database

**What is NOT sent:**
- Your source code
- File contents of the scanned package (beyond what's in the finding summary)
- Environment variables
- Any other project files

The Anthropic API is only called if `ANTHROPIC_API_KEY` is set in your environment. If the variable is not set, the command fails with an explicit error rather than silently using an anonymous API.

ForgeGuardian does not store or log the API key. It is passed directly from the environment variable to the HTTP request.

### `fgctl patch` — Autonomous Patch Agent

**Makes external calls. Requires ANTHROPIC_API_KEY. Fully opt-in.**

The patch agent (`fg-agent`) uses Claude's tool-use API to reason about compatible dependency upgrades. Like `fgctl advisory`, it sends finding summaries and upgrade candidates to the Anthropic API. It does not send source code.

### `fgctl update` — Signature Update

**Downloads data from the internet. Sends nothing.**

`fgctl update` downloads the latest community signatures from `https://github.com/mah3sec/forgeguardian/releases/latest/download/signatures.json`. This is a one-way download — no data from your machine is uploaded. The HTTP request contains only a standard User-Agent header.

Downloaded signatures are stored at `~/.forgeguardian/signatures.json`.

### `fgctl sign` / `fgctl verify` — Sigstore / Rekor

**Makes external calls. Uploads to Rekor.**

`fgctl sign` submits a **hash** (SHA256 of the artifact) to the Rekor transparency log. The following is uploaded to Rekor:
- SHA256 hash of the artifact
- Attestation metadata (build timestamp, SLSA provenance fields)
- Keyless signing certificate (ephemeral, from Sigstore OIDC)

The artifact binary is **never uploaded** to Rekor. Only the hash.

Rekor entries are public and permanent by design — this is the transparency guarantee. If you want a private log, configure a self-hosted Rekor instance (see below).

---

## Telemetry Policy

**ForgeGuardian collects zero telemetry.**

There is no anonymous usage tracking, no crash reporting, no "improve ForgeGuardian by sharing data" checkbox, no phone-home mechanism, and no background process that reports anything. This is enforced by the codebase — there is no telemetry code. You can verify this by searching the source:

```bash
grep -r "telemetry\|analytics\|mixpanel\|segment\|amplitude\|datadog" internal/ cmd/
# Returns: nothing
```

---

## AI Feature Data Handling

When AI features are used (`advisory`, `patch`), the following policy applies:

**What goes to Anthropic:**
- Finding summaries: CVE IDs, package names, versions, severity levels
- Triage context: brief descriptions from public vulnerability databases
- The upgrade plan context: current version, candidate upgrade versions

**What never goes to Anthropic:**
- Source code
- File contents of packages
- Environment variables
- Credentials or secrets
- Personal information of any kind
- Internal network topology or IP addresses

You have complete control. If you never set `ANTHROPIC_API_KEY`, AI features are completely disabled — no data ever reaches Anthropic. Setting the key is a conscious opt-in.

Anthropic's data handling for API calls is governed by [Anthropic's privacy policy](https://www.anthropic.com/privacy). API-tier data is not used to train models by default; consult Anthropic's current terms for enterprise data residency requirements.

---

## Offline / Airgapped Support

The following ForgeGuardian features work without any internet connection:

| Feature | Offline | Notes |
|---|---|---|
| Local project scan (`fgctl scan .`) | Yes | Uses cached OSV DB if available |
| Behavioral / malware scan | Yes | All pattern matching is local |
| Community signatures (after download) | Yes | Cached at `~/.forgeguardian/signatures.json` |
| SBOM generation | Yes | No network calls |
| Provenance generation | Yes | No network calls |
| Monitor mode | Yes | No network calls |
| System audit | Yes | No network calls |
| Web dashboard | Yes | Reads from local postgres |
| AI advisory (`fgctl advisory`) | No | Requires Anthropic API |
| Sigstore signing (public Rekor) | No | Requires rekor.sigstore.dev |
| Sigstore signing (self-hosted Rekor) | Yes | With local Rekor configured |
| Package scan by name | Partial | OSV query fails; Grype/Trivy use local DBs |
| Signature update (`fgctl update`) | No | Requires GitHub access |

### Pre-caching for Airgapped Environments

Before entering an airgapped environment:

```bash
# 1. Download community signatures
fgctl update

# 2. Update Grype's vulnerability database
grype db update

# 3. Update Trivy's vulnerability database
trivy image --download-db-only

# 4. All databases are now cached locally
#    fgctl scan . will work offline using cached data
```

Grype databases are cached at `~/.cache/grype/db/`.
Trivy databases are cached at `~/.cache/trivy/`.
ForgeGuardian signatures are cached at `~/.forgeguardian/signatures.json`.

---

## Self-Hosted Deployment

For organizations that require all data to stay within their network perimeter:

**Start the full enterprise stack:**

```bash
git clone https://github.com/mah3sec/forgeguardian
cd forgeguardian
docker compose -f docker-compose.enterprise.yml up -d
```

This starts:
- PostgreSQL (scan results, SBOM storage)
- Redis (job queue)
- MinIO (artifact store — S3-compatible, fully local)
- Rekor server (local Sigstore transparency log)
- Dependency-Track (continuous SBOM monitoring)
- Prometheus + Grafana (metrics)
- ForgeGuardian API server
- ForgeGuardian worker

**Configure fgctl to use your local stack:**

```bash
fgctl config set api_url=http://your-server:8080
fgctl config set signing.rekor_url=http://your-rekor:3001
```

In self-hosted mode, `fgctl sign` uploads attestations to your local Rekor instance. Nothing reaches the public Sigstore infrastructure.

For the Anthropic API in self-hosted mode: set `ANTHROPIC_API_KEY` to a key from your organization's Anthropic account. If your organization uses a private AI gateway or proxy, set `ANTHROPIC_BASE_URL` to your proxy URL before running advisory commands.

---

## Open Source Transparency

ForgeGuardian's privacy posture is verifiable because the code is public:

- **Source code:** `https://github.com/mah3sec/forgeguardian` — audit every outbound HTTP call
- **Reproducible builds:** The release pipeline uses hermetic builds via GitHub Actions + SLSA Level 3
- **Published SBOMs:** Every release includes a CycloneDX SBOM listing all dependencies
- **Sigstore attestations:** Every release binary is signed and the attestation is in the public Rekor log

To verify a ForgeGuardian binary you downloaded:

```bash
# Download the attestation for your version
curl -L https://github.com/mah3sec/forgeguardian/releases/latest/download/fgctl-linux-amd64.att.json \
     -o fgctl.att.json

# Compute the SHA256 of your binary
sha256sum ./bin/fgctl

# Verify
fgctl verify --attestation=fgctl.att.json --sha256=<hash-from-above>
```

---

## Security of ForgeGuardian Itself

We apply the same security standards to ForgeGuardian as we recommend for your dependencies:

- **Signed releases** — every release binary is signed with Sigstore keyless signing
- **SLSA Level 3 provenance** — provenance documents published for every release
- **Published SBOMs** — CycloneDX and SPDX SBOMs published for the ForgeGuardian tool itself
- **Semgrep CI** — every PR is scanned with Semgrep (`auto` config) for code quality and security issues
- **Distroless runtime images** — production Docker images use `distroless/static-debian12`, no shell
- **No secrets in source** — all credentials are environment variables; the `.env` file is git-ignored
- **Minimum privilege containers** — containers run as `nonroot:nonroot` with read-only root filesystems in Kubernetes manifests

To verify that the ForgeGuardian binary you are running is the one we published:

```bash
fgctl version    # prints version, commit hash, and build time
fgctl debug      # prints diagnostic state including signature freshness
```
