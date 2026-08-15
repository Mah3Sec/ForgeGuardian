# ForgeGuardian — Deep Technical Guide  v2.0.0

> How every module works, data flows, internal logic, and all command flags.  
> Last updated: 2026-08-14

---

## Table of Contents

1. [How the Pipeline Works — End to End](#1-how-the-pipeline-works--end-to-end)
2. [Module Deep Dives](#2-module-deep-dives)
   - [2.1 CLI Foundation — ui package + doctor command](#21-cli-foundation--ui-package--doctor-command)
   - [2.2 Local Project Scanner](#22-local-project-scanner)
   - [2.3 Registry Scrapers](#23-registry-scrapers)
   - [2.4 Build Engine](#24-build-engine)
   - [2.5 Scanner Orchestrator — 8 engines](#25-scanner-orchestrator--8-engines)
   - [2.6 SBOM Generator](#26-sbom-generator)
   - [2.7 AI Triage Engine](#27-ai-triage-engine)
   - [2.8 Autonomous Patch Agent](#28-autonomous-patch-agent)
   - [2.9 Sigstore Signer](#29-sigstore-signer)
   - [2.10 Continuous Monitor](#210-continuous-monitor)
   - [2.11 Intelligence Module](#211-intelligence-module)
3. [Data Flow Diagrams](#3-data-flow-diagrams)
4. [CLI Reference — Complete Flag Listing](#4-cli-reference--complete-flag-listing)
5. [API Endpoint Reference](#5-api-endpoint-reference)
6. [Database Schema](#6-database-schema)
7. [How AI Components Work](#7-how-ai-components-work)
8. [Dashboard Architecture](#8-dashboard-architecture)
9. [Docker Compose Profiles](#9-docker-compose-profiles)
10. [Release Engineering](#10-release-engineering)
11. [How to Integrate into CI/CD](#11-how-to-integrate-into-cicd)
12. [Core Types Reference](#12-core-types-reference)
13. [Community Signatures](#13-community-signatures)

---

## 1. How the Pipeline Works — End to End

### Single-package scan (`fgctl scan npm/lodash@4.17.20`)

```
fgctl scan npm/lodash@4.17.20
              │
              ▼  ui.New() → p.Banner() → spinner starts
  parseDotNotation("npm/lodash@4.17.20")
              │  eco="npm", pkg="lodash", ver="4.17.20"
              ▼
  scrapers/npm.FetchSource(ctx, "lodash", "4.17.20")
              │  GET registry.npmjs.org/lodash/4.17.20
              │  Downloads tarball to /tmp/fg-{uuid}/
              │  Computes SHA256, compares against registry
              ▼
  build/runner.Build(ctx, recipe, sourceArtifact)
              │  Extracts tarball into sandbox workdir
              │  Runs recipe commands (npm install --ignore-scripts, npm pack)
              │  Returns BuiltArtifact{LocalPath, SHA256, BuildLog}
              ▼
  scanner/orchestrator.Scan(ctx, artifact) — 8 goroutines in parallel
              │
              ├─ goroutine 1: grype      → subprocess grype <path> -o json
              ├─ goroutine 2: osv        → HTTP api.osv.dev/v1/query
              ├─ goroutine 3: semgrep    → subprocess semgrep --config=auto
              ├─ goroutine 4: trivy      → subprocess trivy fs <path> -f json
              ├─ goroutine 5: behavioral → typosquat, lifecycle script checks
              ├─ goroutine 6: malware    → byte-pattern matching
              ├─ goroutine 7: ai_model   → only for huggingface recipe
              └─ goroutine 8: mcp        → only for mcp recipe
              │
              WaitGroup.Wait() → MergeFindings() deduplicates by (source, id)
              │  sort: CRITICAL > HIGH > MEDIUM > LOW
              ▼
  p.PrintFindings(pkg, findings, summary)   ← colored terminal output
              │
              ▼  exit 0 (clean) or 2 (findings ≥ --fail-on threshold)
```

### Local project scan (`fgctl scan .`)

```
fgctl scan .
              │  isLocalPath(".") == true
              ▼
  localscanner.Walk(rootDir)
              │  Skips: node_modules, vendor, .git, __pycache__, target
              │  Returns: []ManifestFile{Path, Ecosystem}
              ▼
  For each ManifestFile:
    localscanner.ParseManifest(path)
              │  Returns: []ManifestEntry{Name, Version, Line, FilePath}
              │  normalizeVersion strips ^, ~=, >=, ~ prefixes
              │  Pure ranges (">=4,<5") → Version="" → [SKIP]
              ▼
  localscanner.Scanner.Scan(ctx, rootDir, progress)
              │  Semaphore: Workers goroutines (default 4)
              │  For each ManifestEntry with Version != "":
              │      synthesizeArtifact(entry) → BuiltArtifact{LocalPath=""}
              │      scanner.Orchestrator.Scan(ctx, artifact)
              │      LocalPath="" → grype/semgrep/trivy skip gracefully
              │      OSV, behavioral, malware still produce findings
              ▼
  p.PrintLocalScanHeader, p.PrintManifestHeader, p.PrintLocalFinding
  p.PrintScanSummary → total packages, findings by severity
```

---

## 2. Module Deep Dives

### 2.1 CLI Foundation — ui package + doctor command

**Location:** `internal/ui/printer.go`, `cmd/fgctl/doctor.go`

#### Printer

The `ui.Printer` is the single output layer for all CLI commands. It respects `--no-color` and `--json` flags, and prints the ASCII banner exactly once via `sync.Once`.

```go
type Printer struct {
    NoColor  bool
    JSON     bool
    Out      io.Writer  // os.Stdout
    Err      io.Writer  // os.Stderr
    version  string
    sigCount int        // set by SetSigCount; printed in Banner() when > 0
}

func New(noColor, jsonMode bool, version string) *Printer
func (p *Printer) SetSigCount(n int)                // stores n; Banner() prints "Loaded N detection signatures"
func (p *Printer) Banner()                          // sync.Once, suppressed in JSON mode; prints sigCount line when > 0
func (p *Printer) Spinner(msg string) *spinner.Spinner
func StopSpinner(s *spinner.Spinner)                // nil-safe
func SeverityColor(sev core.Severity) *color.Color
func (p *Printer) PrintFindings(pkg string, findings []core.Finding, sum scanner.ScanSummary)
func (p *Printer) PrintLocalScanHeader(rootDir string)
func (p *Printer) PrintManifestHeader(manifestPath, ecosystem string)
func (p *Printer) PrintLocalFinding(name, version string, f core.Finding)
func (p *Printer) PrintLocalScanSkip(name, raw string)
func (p *Printer) PrintScanSummary(sum scanner.ScanSummary)
func (p *Printer) PrintCompactGrouped(results []localscanner.LocalScanResult)
    // Groups findings by (name, version), accumulates severity counts,
    // prints one line per package:
    //   axios@1.3.4  — 19 findings [CRIT:1 HIGH:1 MED:17]  fix: >=1.12.0
    // Used when --compact flag is set.
func (p *Printer) Success/Warn/Error(format string, args ...any)
func (p *Printer) Divider()
```

Severity colors: `CRITICAL` → red+bold · `HIGH` → hi-red · `MEDIUM` → yellow · `LOW` → cyan

#### preParseOutputFlags

`preParseOutputFlags()` is called at the very top of `main()`, **before** any subcommand's `flag.Parse` call. This is necessary because Go's `flag` package stops parsing at the first non-flag positional argument, meaning `fgctl scan . --format json` would leave `--format json` unparsed.

```go
func preParseOutputFlags() (suppressHuman bool)
```

**How it works:**
- Iterates over `os.Args[1:]` looking for flag patterns
- Detects `--quiet`, `--no-banner`, `--ci` directly
- Detects `--json` and `--format=json/sarif/cyclonedx-json/spdx-json` (both `--format=value` and space-separated `--format value` via peek-ahead at the next arg)
- Detects `sbom` as the first non-flag argument with no `--out` flag present (SBOM stdout mode)
- Returns `suppressHuman bool`

The returned value is used to gate:
- `p.Banner()` — suppressed when `suppressHuman == true`
- `firstRunCheck()` — suppressed when `suppressHuman == true`
- `p.JSON` field — set to `true` when `suppressHuman == true`

This ensures that **no human-readable output leaks into machine-readable stdout streams**, regardless of argument ordering.

#### Doctor

`fgctl doctor` checks your environment health and prints a structured health report. Pass `--fix` to automatically repair failing checks (installs missing tools where possible, creates config and policy files if absent):

```
fgctl doctor — ForgeGuardian dev
──────────────────────────────────────────────────
[PASS] Go runtime            go1.25.0
[WARN] grype                 not found in PATH — some scan engines disabled
[PASS] semgrep               found at /usr/local/bin/semgrep  (1.161.0)
[PASS] trivy                 found at /usr/local/bin/trivy  (Version: 0.73.0)
[WARN] ANTHROPIC_API_KEY     not set — advisory and auto-fix commands unavailable
[PASS] signatures            24 detection signatures loaded
[WARN] signatures freshness  signatures.json not found — run 'fgctl update'
[PASS] disk space            611.5 GB free in /tmp
[WARN] API server            not reachable at http://localhost:8080 (CLI works offline)
[WARN] docker                docker info failed: Client: Docker Engine - Community
[PASS] node.js               v24.14.0
[PASS] dashboard build       dashboard/dist present
[WARN] config.yaml           not found — run 'fgctl config init' to create
──────────────────────────────────────────────────
  0 failure(s), 6 warning(s).
  Run 'fgctl doctor --fix' to attempt automatic repair.
```

The `[PASS] signatures` line above only shows real signatures because this
was run from inside a git clone — `checkSignatures` uses the same local
`signatures/` fallback the scanner itself uses (see `LoadStoreWithLocalFallback`
in `internal/intelligence/store.go`), not just `signatures.json`. A binary
run outside any checkout, with no `fgctl update` ever run, would correctly
WARN here instead. The separate "signatures freshness" check only looks at
`signatures.json`'s mtime — it stays WARN either way, since freshness can't
be determined from a directory of hand-authored YAML with no download
timestamp.

Checks run: Go runtime version, `grype`/`semgrep`/`trivy` in PATH, `ANTHROPIC_API_KEY` set, signatures loaded (JSON store or local fallback), signatures freshness (warn if >7 days old, JSON store only), disk space >500 MB (`syscall.Statfs`), API reachable at `$FG_API_URL`, Docker available (non-blocking warn), Node version ≥20, `dashboard/dist/` present, `config.yaml` valid YAML. `--fix` attempts to repair whichever of these it can (installs missing tools via brew/pip where possible, creates `config.yaml` if absent) and re-checks each fixed item before printing the final tally.

`--fix` prints the exact repair command before executing it (e.g. `→ running: brew install grype`) so the action is always transparent.

Exit: `0` if no failures · `1` if any `[FAIL]`

---

### 2.2 Local Project Scanner

**Location:** `internal/localscanner/`

Three files cooperate to implement `fgctl scan .`:

#### walker.go

```go
var manifestNames = map[string]string{
    "package.json":     "npm",
    "requirements.txt": "pypi",
    "pyproject.toml":   "pypi",
    "go.mod":           "go",
    "Cargo.toml":       "crates",
    "pom.xml":          "maven",
    "Gemfile":          "rubygems",
}

var skipDirs = map[string]bool{
    "node_modules": true, "vendor": true, ".git": true,
    "__pycache__": true, "target": true, ".tox": true,
    "dist": true, "build": true,
}

func Walk(rootDir string) ([]ManifestFile, error)
```

#### manifest.go

```go
type ManifestEntry struct {
    Ecosystem string
    Name      string
    Version   string   // normalizeVersion() output; "" if range-only
    FilePath  string
    Line      int      // 0-indexed line number
    Raw       string   // original raw version string e.g. "^4.17.21"
}

func ParseManifest(path string) ([]ManifestEntry, error)
func normalizeVersion(raw string) string  // strips ^, ~=, >=, ~, *, etc.
```

Sub-parsers (stdlib only — no external dependencies):
- `parsePackageJSON` — encoding/json
- `parseRequirements` — bufio line scanner + regexp
- `parsePyprojectTOML` — bufio + regexp (no TOML library dependency)
- `parseGoMod` — bufio line scanner, handles `require (...)` blocks
- `parseCargoToml` — bufio + regexp for `[dependencies]` sections
- `parsePomXML` — encoding/xml
- `parseGemfile` — bufio + regexp for `gem 'name', 'version'`

#### scanner.go

```go
type LocalScanResult struct {
    Entry    ManifestEntry
    Findings []core.Finding
    Skipped  bool   // true when Version == ""
    Err      error
}

type ProjectScanResult struct {
    RootDir      string
    Manifests    []ManifestFile
    Results      []LocalScanResult
    MissingTools []string            // unique, sorted tool names not installed (e.g. ["grype","semgrep"])
    Summary      scanner.ScanSummary
}

type Scanner struct {
    IntelStorePath string
    Workers        int  // default 4
}

func New(intelStorePath string) *Scanner
func (s *Scanner) Scan(ctx context.Context, rootDir string, progress func(done, total int)) (*ProjectScanResult, error)
```

`synthesizeArtifact` creates a `core.BuiltArtifact` with `LocalPath = ""` intentionally. Archive-based scanners (grype, semgrep, trivy) check for empty LocalPath and return immediately. Name+version-based scanners (OSV, behavioral typosquat/version-anomaly, malware blocklist) run normally and produce real findings.

**TOOL-NOT-INSTALLED deduplication:** after all goroutines complete, findings with `ID == "TOOL-NOT-INSTALLED"` are stripped from every `LocalScanResult.Findings` and the source tool name is collected into `ProjectScanResult.MissingTools`. The caller (`cmd/fgctl/main.go`) prints one consolidated warning via `p.PrintMissingToolsWarning(result.MissingTools)` before the findings loop — not once per package.

#### filterResults and severityOrd

`filterResults(results []LocalScanResult, minSev string, onlyFixable bool)` post-processes scan output before printing:

```go
func severityOrd(s core.Severity) int {
    switch s {
    case core.SeverityCritical:      return 4
    case core.SeverityHigh:          return 3
    case core.SeverityMedium:        return 2
    case core.SeverityLow:           return 1
    case core.SeverityInformational: return 0
    default:                         return -1   // unknown/empty — always excluded
    }
}
```

**Key behaviors (Phase 14 fixes):**
- `severityOrd` returns `-1` for unknown or empty severity strings (was `0` in v1.2.0). This means findings with no recognized severity tag no longer accidentally pass through `--severity` filters.
- When `--severity` is set to any value (even `low`), INFORMATIONAL findings are excluded unless the user explicitly types `--severity=informational`.
- `--only-fixable` also excludes INFORMATIONAL findings, in addition to findings with no `FixedVersion`.
- `--quiet` triggers an early return before any printing loop, ensuring zero stdout leakage (including advisory hint lines).

---

### 2.3 Registry Scrapers

**Location:** `internal/scrapers/`  
**Interface:** `core.RegistryScraper`

```go
type RegistryScraper interface {
    Poll(ctx context.Context, since time.Time) ([]PackageVersion, error)
    FetchSource(ctx context.Context, pkg, version string) (*SourceArtifact, error)
    VerifyIntegrity(ctx context.Context, artifact *SourceArtifact) error
}
```

| Ecosystem | Registry API | Format |
|-----------|-------------|--------|
| npm | `registry.npmjs.org/{pkg}/{version}` | JSON |
| PyPI | `pypi.org/pypi/{pkg}/{version}/json` | JSON |
| Maven | `search.maven.org/solrsearch/select` | JSON |
| Go | `proxy.golang.org/{module}/@v/list` | Text |
| RubyGems | `rubygems.org/api/v1/versions/{gem}.json` | JSON |
| crates.io | `crates.io/api/v1/crates/{name}/versions` | JSON |
| HuggingFace | `huggingface.co/api/models?sort=lastModified` | JSON |
| MCP | `smithery.ai/api/v1/servers` | JSON |
| OCI | Docker Registry HTTP API v2 | JSON |

---

### 2.4 Build Engine

**Location:** `internal/build/`

#### Sandbox

`internal/build/sandbox/` implements process-level isolation:
- Clean environment (no parent env vars leaked into build process)
- Fresh temp dir per build, cleaned on exit
- Network disabled after fetch phase
- Configurable timeout (default 10 minutes)

#### Runner

```go
func (r *Runner) Build(ctx context.Context, recipe Recipe, artifact *SourceArtifact) (*BuiltArtifact, error)
```

1. Extract archive to sandbox workdir
2. Run recipe commands
3. Capture stdout/stderr as `BuildLog`
4. Compute SHA256 of output artifact
5. `FG_VERIFY_REPRODUCIBLE=true` → repeat steps 1–4, compare hashes → `Reproducible=true/false`

#### Recipes

| Recipe | Commands run |
|--------|------------|
| npm | `npm install --ignore-scripts`, `npm pack` |
| pypi | `pip install . --no-binary :all: --no-deps` |
| maven | `mvn clean package -DskipTests` |
| go | `go mod download`, `go build ./...` |
| rubygems | `gem build *.gemspec` |
| crates | `cargo build --release` |
| ai_model | Extracts model files, validates config.json format |
| mcp_server | `npm install`, inspects tool definitions |

---

### 2.5 Scanner Orchestrator — 8 engines

**Location:** `internal/scanner/orchestrator.go`

All 8 engines fan out concurrently with `sync.WaitGroup`. Total scan time = slowest engine, not the sum.

`MergeFindings()` deduplicates by `(source, id)` — the same CVE from Grype and OSV appears only once.

#### Engine internals

**Grype** — runs `grype <path> -o json`, parses Grype JSON → `[]core.Finding`. Skips if `LocalPath=""` or binary not in PATH.

**OSV** — HTTP POST to `api.osv.dev/v1/query` with `{package: {name, ecosystem}, version}`. No binary needed. Works for local scans.

**Semgrep** — runs `semgrep --config=auto <source-dir> --json`. Only runs on extracted source (skips if `LocalPath=""`).

**Trivy** — runs `trivy fs <path> -f json`. Skips if `LocalPath=""`.

**Behavioral** — pure Go, no external binary:
- Typosquatting: Levenshtein distance ≤2 vs top-1000 packages per ecosystem
- Lifecycle scripts: flags `postinstall`/`preinstall` accessing `process.env` or `child_process`
- Scope confusion: `@` packages mimicking legitimate scopes
- Also queries `IntelligenceStore` for `behavioral_rule` signatures

**Malware** — byte-pattern matching, base64 payload detection, hardcoded IP detection, obfuscated eval chains. Queries `IntelligenceStore` for `malware_pattern` signatures.

**AI Model** — only activates for `huggingface` recipe. Checks `config.json` for `safe_serialization: false`, detects `.pkl` files, validates model card completeness. Queries `IntelligenceStore` for `pickle_rule` signatures.

**MCP** — only activates for `mcp` recipe. Flags prompt injection in tool `description` fields, overly broad permissions, `child_process.execSync` without sandboxing. Queries `IntelligenceStore` for `mcp_injection_pattern` signatures.

---

### 2.6 SBOM Generator

**Location:** `internal/sbom/`

#### Supported formats

| Format | Standard | File ext |
|--------|---------|---------|
| `cyclonedx-json` | CycloneDX 1.5 | `.cdx.json` |
| `cyclonedx-xml` | CycloneDX 1.5 | `.cdx.xml` |
| `spdx-json` | SPDX 2.3 | `.spdx.json` |
| `spdx-tv` | SPDX 2.3 Tag-Value | `.spdx` |

#### AI/MCP extensions (custom properties)

```
fg:finding:critical_count    fg:finding:high_count
fg:finding:cve_ids           fg:ai:safe_serialization
fg:ai:format                 fg:ai:model_card_complete
fg:mcp:permission_scope      fg:mcp:tool_count
```

---

### 2.7 AI Triage Engine

**Location:** `internal/agent/triage/`

Single-turn Claude call with structured output. Prompt caching on system prompt + package metadata (5-minute TTL). Average 2–4 seconds, ~$0.002 per advisory with caching.

**Output:**

```go
type Advisory struct {
    Package                 string
    Version                 string
    Severity                Severity  // CRITICAL | HIGH | MEDIUM | LOW
    Confidence              float64   // 0.0–1.0
    AdvisoryText            string
    ExploitabilityRationale string
    AgenticRisk             string    // unique to ForgeGuardian
    RecommendedAction       string
    PatchSuggestion         string    // e.g. "lodash@4.17.21"
    GeneratedAt             time.Time
}
```

**Agentic risk scoring** is unique to ForgeGuardian. Beyond CVSS, Claude assesses: "If this package is used inside an AI agent tool-use pipeline, how dangerous is this?" — critical context for MCP servers and LangChain/LlamaIndex tools.

---

### 2.8 Autonomous Patch Agent

**Location:** `internal/agent/`

Multi-turn Claude tool-use loop. Does not apply patches unless `--apply` is passed. Even then, a second Claude call reviews the plan before any file is written.

**Tools Claude can call:**

```
read_file(path)         → read manifest contents
list_files(dir)         → find manifest files
search_pattern(pattern) → grep for package references
write_patch(file, diff) → propose a version change
```

**Manifest files the agent can patch:**

| Ecosystem | Files |
|-----------|------|
| npm | `package.json`, `package-lock.json` |
| PyPI | `requirements.txt`, `pyproject.toml`, `setup.py`, `Pipfile` |
| Go | `go.mod`, `go.sum` |
| Ruby | `Gemfile`, `Gemfile.lock` |
| Rust | `Cargo.toml`, `Cargo.lock` |
| Java | `pom.xml`, `build.gradle`, `build.gradle.kts` |

---

### 2.9 Sigstore Signer

**Location:** `internal/signer/`

Keyless signing: ephemeral ECDSA P-256 keypair generated in memory → signed → uploaded to Rekor → private key discarded. No key files on disk.

The resulting `Attestation` JSON includes:
- `sha256` of built artifact
- `signature` (base64 DER ECDSA)
- `public_key` (PEM)
- `rekor_log_id` + `rekor_log_index`
- Full SLSA v1.0 provenance statement

---

### 2.10 Continuous Monitor

**Location:** `internal/monitor/`

Uploads CycloneDX SBOMs to Dependency-Track, polls for new CVE findings over time. Complements the point-in-time scanner — if a new CVE is published tomorrow for a package scanned last week, Dependency-Track alerts you.

---

### 2.11 Intelligence Module

**Location:** `internal/intelligence/`, `cmd/intel-agent/`

`intel-agent` polls three feeds, runs a Claude multi-turn tool-use loop to generate signatures, merges them into `~/.forgeguardian/signatures.json`:

1. **OSV** (`api.osv.dev/v1/query`) — new vulnerabilities last 24h
2. **OpenSSF Malicious Packages** (`github.com/ossf/malicious-packages`) — confirmed malicious packages
3. **npm/PyPI popularity** — top-1000 packages per ecosystem (typosquatting targets)

Claude tool: `emit_signature(type, ecosystem, pattern, severity, title, description)`

Runs every 6h with `--loop --interval=6h` (default in the enterprise docker-compose profile).

---

## 3. Data Flow Diagrams

### Full stack (server mode)

```
Package Registry → Scraper Orchestrator → PostgreSQL (packages)
                         │
                         ▼
                   Redis Job Queue
                         │
                         ▼
                   Build Worker ────────→ MinIO (artifacts)
                         │
                         ▼
                   Scanner Orchestrator → PostgreSQL (scan_results)
                         │
                   ┌─────┴──────┐
                   ▼            ▼
              SBOM Generator  AI Triage → PostgreSQL (advisories)
              MinIO (sboms)        │
                                   ▼
                             Patch Agent
                                   │
                                   ▼
                             Sigstore Signer → Rekor (transparency log)
                                               PostgreSQL (attestations)
                                   │
                                   ▼
                          Dependency-Track Monitor
                                   │
                                   ▼
                           REST API ←→ React Dashboard
```

### CLI mode (no server needed)

```
fgctl scan npm/lodash@4.17.20
    │
    ▼  (no Redis, no PostgreSQL, no MinIO)
Scraper → Build → Scan → ui.PrintFindings()
    │
    all runs in-process, tmp files cleaned on exit
```

---

## 4. CLI Reference — Complete Flag Listing

### fgctl scan

```
fgctl scan [target] [flags]

Target:
  .               Scan current directory (local project scan)
  ./path          Scan a specific directory
  npm/pkg@1.2.3   Dot-notation: ecosystem/package@version
  --recipe + --package + --version flags (explicit form)

Flags:
  --recipe        string    Ecosystem: npm|pypi|maven|go|rubygems|crates|huggingface|mcp
  --package       string    Package name
  --version       string    Package version
  --format        string    Output format: text (default)|json|sarif
  --json                    Alias for --format=json
  --compact                 One line per finding (no descriptions)
  --summary                 Print only the summary table, no individual findings
  --quiet                   No output; use exit code only (CI use)
  --severity      string    Filter: only show findings at this level+: critical|high|medium|low
  --ecosystem     string    Filter: only scan packages from this ecosystem (local scan)
  --only-fixable            Only show findings where a fix version is known
  --verbose                 Expand all grouped findings (default: top 3 per package + grade)
  --debug                   Show engine errors and raw metadata
  --prod-only               Exclude devDependencies from local project scan
  --exclude-dev             Alias for --prod-only
  --ci                      CI mode: --quiet --format=sarif --fail-on=high
  --executive               Executive summary: severity table only
  --no-banner               Suppress the ASCII logo banner
  --no-color                Disable ANSI color output (also: NO_COLOR env var)
  --fail-on       string    Exit 2 if any finding ≥ this severity: critical|high|medium|low
  --timeout       duration  Build + scan timeout (default: 10m)
  --workers       int       Concurrent package scan workers for local scan (default: 4)
  --skip-build              Skip hermetic build phase, scan source archive directly
  --engines       string    Comma-separated engines to run (default: all)
                            Options: grype,osv,semgrep,trivy,behavioral,malware,ai_model,mcp
  --sig-store     string    Path to intelligence signatures (default: ~/.forgeguardian/signatures.json)
  --no-color                Disable ANSI colors
```

### fgctl patch

```
fgctl patch [project-dir] [ecosystem/pkg@ver]

  Delegates to fg-agent with --apply.
  Finds fg-agent in PATH, then bin/ relative to fgctl, then ./bin/.

  --project-dir string    Directory with manifests (default: first arg or ".")
```

### fgctl monitor

```
fgctl monitor [dir] [flags]

  Without --watch: one-shot scan (same as fgctl scan .).
  With --watch:    polls manifest mtimes and re-scans on change.

  --watch               Enable watch mode
  --dir         string  Directory to monitor (default: "." or first arg)
  --interval    duration Poll interval (default: 3s)
  --workers     int     Scan workers (default: 4)
  --timeout     duration Scan timeout per cycle (default: 5m)
```

**Watch mode UX (Phase 14 polish):**

Every diff line is prefixed with a `[HH:MM:SS]` wall-clock timestamp and includes the affected package name. Delta lines are color-coded:
- `NEW` lines — printed in red
- `RESOLVED` lines — printed in green

Example:
```
  [14:32:17] NEW      express@4.19.2  [HIGH] CVE-2024-29041 — Open redirect
  [14:35:42] RESOLVED express@4.19.2  CVE-2024-29041 — Open redirect
```

### fgctl audit

```
fgctl audit system

  Enumerates globally installed packages from:
    npm -g list --json --depth=0
    pip list --format=json  (falls back to pip3)
    cargo install --list
    $GOPATH/bin directory listing

  Prints an ecosystem-level summary table (not per-package output).
  --verbose expands to per-package output.
  TOOL-NOT-INSTALLED and INFORMATIONAL findings are silently dropped.
```

### fgctl stats

```
fgctl stats [flags]

  Reads the intelligence.SignatureStore from the configured store path,
  counts signatures by intelligence.SignatureType, and prints a formatted
  statistics table.

  --json              Output as a JSON object instead of a table
  --store  string     Path to signature store (default: ~/.forgeguardian/signatures.json)

  Output includes:
    - Total signature count
    - Last updated timestamp
    - Breakdown by type:
        malware_pattern, typosquat_target, behavioral_rule,
        blocklisted_package, mcp_injection_pattern, pickle_rule
    - Ecosystems covered (unique ecosystem values across all signatures)
```

**Implementation note (`runStats` in `cmd/fgctl/main.go`):**
- Opens `intelligence.SignatureStore` at the resolved store path
- Iterates all signatures, increments a `map[intelligence.SignatureType]int` counter
- Collects a `map[string]struct{}` of ecosystems
- Prints table via `p.PrintStatsTable(...)` or marshals to JSON with `--json`

### fgctl debug

```
fgctl debug [flags]

  Prints a diagnostic report for bug reports:
    version, commit, build time, OS/arch, Go runtime
    config file: path + size
    signatures: count + last-modified
    API URL + reachability
    grype / semgrep / trivy locations
    ANTHROPIC_API_KEY presence (masked)
    disk free (GB)

  --json    Output as machine-readable JSON
```

### fgctl config

```
fgctl config show
fgctl config set <key>=<value>
fgctl config init

  Config file: ~/.forgeguardian/config.yaml

  Valid keys:
    api_url                    string   ForgeGuardian API base URL
    scan.workers               int      Concurrent scan workers
    scan.timeout               string   e.g. "10m"
    scan.fail_on               string   critical|high|medium|low
    scan.min_severity          string   Filter output below this level
    signing.rekor_url          string   Custom Rekor URL
    notify.slack_webhook_url   string   Slack incoming webhook URL
    notify.discord_webhook_url string   Discord webhook URL
    notify.webhook_url         string   Generic HTTP webhook URL
    notify.on_severity         string   Min severity to fire: critical|high|medium|low
```

### fgctl policy

```
fgctl policy show        # print ~/.forgeguardian/policy.yaml
fgctl policy init        # create with defaults
fgctl policy set key=value  # update one field (fail_on, block_typosquatting, etc.)
fgctl policy validate    # validate YAML syntax
```

### fgctl doctor

```
fgctl doctor [flags]

  Checks: grype/semgrep/trivy in PATH, ANTHROPIC_API_KEY set,
          signatures.json exists+populated, API reachable,
          disk space >500MB, Go version, signatures freshness (>7 days → WARN),
          Docker available (non-blocking WARN), Node version >=20,
          dashboard/dist/ present, config.yaml valid YAML

  --fix  Auto-repair failing checks: install missing tools, create config/policy files.
         Prints the exact repair command before executing it.

  Exit codes:
    0  — all checks pass or warn only
    1  — one or more checks failed
```

### fgctl advisory

```
fgctl advisory [target] [flags]

  Target same format as fgctl scan.

  --api-key     string    Anthropic API key (default: $ANTHROPIC_API_KEY)
  --skip-scan             Skip scan phase, generate advisory without findings
  --json                  Output as JSON
  --timeout     duration  (default: 10m)
```

### fgctl sbom

```
fgctl sbom [target] [flags]

  --format      string    cyclonedx-json|cyclonedx-xml|spdx-json|spdx-tv (default: cyclonedx-json)
  --out         string    Write to file (default: stdout)
  --timeout     duration  (default: 10m)
```

### fgctl sign

```
fgctl sign [target] [flags]

  --rekor-url   string    Rekor instance URL (default: https://rekor.sigstore.dev)
  --out         string    Write attestation JSON to file (default: stdout)
  --timeout     duration  (default: 5m)
```

### fgctl verify

```
fgctl verify [flags]

  --attestation string    Path to attestation JSON file (required)
  --sha256      string    Expected SHA256 hex digest (required)
```

### fgctl provenance

```
fgctl provenance [target] [flags]

  --out         string    Write SLSA provenance JSON to file (default: stdout)
  --timeout     duration  (default: 5m)
```

### fgctl update

```
fgctl update

  Downloads signatures.json from GitHub releases, merges into local store.
  Prints: "Updated: +N signatures (total: M)"
```

### fgctl sig validate

```
fgctl sig validate <path>

  Validates a community signature JSON file.
  Exit 0: valid — prints git commands to submit PR.
  Exit 2: invalid — prints specific field errors.
```

### fg-agent

```
fg-agent [flags]

  --recipe      string    Ecosystem (required)
  --package     string    Package name (required)
  --version     string    Package version (required)
  --apply                 Apply patches (default: dry-run)
  --project-dir string    Directory with manifest files (default: .)
  --api-key     string    Anthropic API key (default: $ANTHROPIC_API_KEY)
  --json                  Output full result as JSON
  --timeout     duration  (default: 15m)
  --max-turns   int       Max Claude tool-use turns (default: 10)
  --skip-scan             Skip scan phase
```

### intel-agent

```
intel-agent [flags]

  --store       string    Signature store path (default: ~/.forgeguardian/signatures.json)
  --loop                  Run continuously
  --interval    duration  Poll interval in loop mode (default: 6h)
  --api-key     string    Anthropic API key (default: $ANTHROPIC_API_KEY)
  --ecosystems  string    Comma-separated ecosystems (default: npm,pypi,go,rubygems,crates)
  --dry-run               Print without saving to disk
  --skip-ai               Only pull feeds, skip AI signature generation
  --verbose               Verbose logging
```

---

## 5. API Endpoint Reference

Base URL: `http://localhost:8080`

### Health

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/healthz` | Returns `{"status":"ok"}` |
| `GET` | `/metrics` | Prometheus metrics |

### Packages

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/packages` | List packages (paginated) |
| `GET` | `/api/v1/packages/:eco/:name` | Package detail |
| `GET` | `/api/v1/packages/:eco/:name/versions` | All versions |

### Scanning

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/scan` | Trigger scan |
| `GET` | `/api/v1/scan/:eco/:name/:version` | Get scan results |

**POST body:** `{ "ecosystem": "npm", "package": "lodash", "version": "4.17.20" }`

### Advisory

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/advisory` | Generate AI advisory |

### SBOM

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/sbom/:eco/:name/:version` | Get SBOM (`?format=cyclonedx-json`) |

### Signing

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/sign` | Sign artifact |
| `POST` | `/api/v1/verify` | Verify attestation |

### Dashboard

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/dashboard/stats` | Aggregate stats |
| `GET` | `/api/v1/dashboard/recent` | Recent scan activity |
| `GET` | `/api/v1/dashboard/timeline` | 30-day findings timeline (`?days=30`) |
| `GET` | `/api/v1/dashboard/activity` | Recent activity events (`?limit=20`) |
| `GET` | `/api/v1/risks` | Prioritized active risks with A–F grade |
| `GET` | `/api/v1/policy/status` | Current policy enforcement status |
| `POST` | `/api/v1/webhooks/test` | Send a test webhook payload |

**Timeline response:**
```json
{
  "days": 30,
  "points": [
    { "date": "2026-04-23", "critical": 2, "high": 7, "medium": 14, "low": 5, "total": 28 }
  ]
}
```

### Intelligence

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/intelligence/signatures` | List signatures |
| `POST` | `/api/v1/intelligence/refresh` | Trigger intel-agent run |

---

## 6. Database Schema

All tables in `internal/api/db/schema.sql`. Queries in `internal/api/db/sqlc/` (generated by sqlc).

```sql
CREATE TABLE packages (
    id        BIGSERIAL PRIMARY KEY,
    ecosystem TEXT NOT NULL,
    name      TEXT NOT NULL,
    UNIQUE(ecosystem, name)
);

CREATE TABLE package_versions (
    id           BIGSERIAL PRIMARY KEY,
    package_id   BIGINT REFERENCES packages(id),
    version      TEXT NOT NULL,
    source_url   TEXT,
    checksum     TEXT,
    published_at TIMESTAMPTZ,
    UNIQUE(package_id, version)
);

CREATE TABLE scan_results (
    id                 BIGSERIAL PRIMARY KEY,
    package_version_id BIGINT REFERENCES package_versions(id),
    sha256             TEXT,
    status             TEXT DEFAULT 'pending',  -- pending|running|complete|error
    total_findings     INT  DEFAULT 0,
    critical_findings  INT  DEFAULT 0,
    high_findings      INT  DEFAULT 0,
    medium_findings    INT  DEFAULT 0,
    low_findings       INT  DEFAULT 0,
    highest_severity   TEXT,
    findings_json      JSONB,
    scanned_at         TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE attestations (
    id                 BIGSERIAL PRIMARY KEY,
    package_version_id BIGINT REFERENCES package_versions(id),
    sha256             TEXT,
    signature          TEXT,
    public_key         TEXT,
    rekor_log_id       TEXT,
    rekor_index        BIGINT,
    attestation_json   JSONB,
    signed_at          TIMESTAMPTZ DEFAULT NOW()
);
```

---

## 7. How AI Components Work

ForgeGuardian uses Claude (Anthropic) in three ways:

| Component | Model | Mode | Avg latency |
|-----------|-------|------|------------|
| Triage (advisory) | claude-sonnet-4-20250514 | Single-turn + prompt caching | 2–4s |
| Patch planner | claude-sonnet-4-20250514 | Multi-turn tool-use (max 10 turns) | 15–30s |
| Intelligence generation | claude-sonnet-4-20250514 | Multi-turn tool-use (`emit_signature`) | 30–60s per batch |

**The agent will NOT apply patches automatically** unless you pass `--apply`. Even then, a second Claude reviewer call approves before any file is written.

**Confidence scoring** (0.0–1.0): reflects advisory certainty. <0.7 = ambiguous findings. >0.9 = well-documented CVEs with clear exploit paths.

---

## 8. Dashboard Architecture

**Location:** `dashboard/`  
**Stack:** React 18 + TypeScript 5 + Vite + Tailwind CSS + shadcn/ui + Zustand + React Query

The dashboard has 28 routes (`dashboard/src/App.tsx`'s router). Full up-to-date
page-by-page list: see [README.md's Dashboard section](README.md#dashboard) —
not duplicated here to avoid the two drifting apart again. A few worth
calling out for how they're actually built:

| Page | Route | Description |
|------|-------|-------------|
| Dashboard | `/` | Stats, 30-day recharts AreaChart timeline, ActivityFeed, RiskHeatmap, dependency force graph |
| Monitor | `/monitor` | Live stats polling every 10s via `GET /api/v1/dashboard/stats` |
| Scan | `/scan` | 3 tabs: registry package, file upload, remote SSH host — trigger scans, view findings |
| Advisory | `/advisory` | AI-generated advisory viewer |
| SBOM | `/sbom` | SBOM viewer and download |
| Sign/Verify | `/sign` | Sigstore attestation management |
| Intelligence | `/intelligence` | Signature browser and refresh |
| Risks | `/risks` | Prioritized active risks with A–F grade |
| Inventory | `/inventory` | Full package inventory browser |
| Policy | `/policy` | Policy-as-code viewer and editor |

### shadcn/ui components (`dashboard/src/components/ui/`)

`utils.ts` (cn helper) · `button.tsx` · `badge.tsx` (severity variants: critical/high/medium/low/safe) · `card.tsx` · `table.tsx` · `tabs.tsx` · `tooltip.tsx`

### Feature components

`ActivityFeed` — live stream of recent scan and advisory events, fed by `GET /api/v1/dashboard/activity`.  
`RiskHeatmap` — ecosystem-risk heatmap grid showing risk distribution by ecosystem and severity grade (A–F).

### DependencyGraph

`DependencyGraph.tsx` uses `react-force-graph-2d` (canvas). Node colors map to worst finding severity. Cleanup calls `graphRef.current?.pauseAnimation()` on unmount.

### CSS/Tailwind token strategy

`:root` defines both ForgeGuardian raw tokens (`--color-safe`, `--bg-base`, etc.) and shadcn/ui HSL tokens (`--primary`, `--background`, etc.) as aliases. Do not replace the ForgeGuardian tokens — shadcn/ui components reference the HSL ones; custom components reference the hex ones.

---

## 9. Docker Compose Profiles

### Minimal (`docker-compose.minimal.yml`)

3 services: `postgres` + `redis` + `forgeguardian-api`

Resource limits per service: `cpus: 0.5`, `memory: 512M`

```bash
docker compose -f docker-compose.minimal.yml up -d
make docker-minimal   # Makefile shortcut
```

### Dev (`docker-compose.dev.yml`)

Minimal + `minio` + `prometheus` + `grafana` + `forgeguardian-worker`

```bash
make docker-dev
```

### Enterprise (`docker-compose.enterprise.yml`)

Dev + `rekor-server` + `trillian-log-server` + `dependency-track` + `intel-agent`

```bash
make docker-enterprise
```

All compose files extend `docker-compose.base.yml` which defines shared networks and volumes.

**Worker build context fix:** all compose files use `context: .`, `dockerfile: cmd/fg-agent/Dockerfile`. The old `./build-engine` context path was incorrect after the single-module migration.

---

## 10. Release Engineering

### GoReleaser

`.goreleaser.yaml` builds 3 binaries (`fgctl`, `fg-agent`, `intel-agent`) for Linux/macOS/Windows × amd64/arm64. Archives include `README.md`, `LICENSE`, and `GUIDE.md`.

A `brews:` stanza auto-updates the Homebrew formula in `mah3sec/homebrew-forgeguardian` on every tagged release.

### Homebrew formula

`Formula/forgeguardian.rb` — platform-aware SHA256 placeholders filled by goreleaser at release time. Installs all 3 binaries.

### Curl installer (`install.sh`)

```bash
curl -sSfL https://raw.githubusercontent.com/mah3sec/forgeguardian/main/install.sh | bash
```

- Detects OS + arch
- Resolves latest tag from GitHub API (5-second timeout; on failure, auto-switches to local build mode with a clear error message and recovery hint)
- Downloads tar.gz, SHA256 verifies
- Installs to `/usr/local/bin` (or `$INSTALL_DIR`)
- Respects `FORGEGUARDIAN_VERSION` env var to pin a specific version

#### Install modes (`FORGEGUARDIAN_INSTALL_MODE`)

```
FORGEGUARDIAN_INSTALL_MODE=auto|local|offline|minimal|full|dev
  auto    — try GitHub release, fall back to local build on network failure
  local   — go install from source (requires Go)
  offline — use pre-placed binaries from ./bin/
  minimal — fgctl only (no npm, no docker)
  full    — fgctl + signatures + npm install
  dev     — full + docker compose up --build
```

#### Air-gapped installs (`FORGEGUARDIAN_LOCAL_BUNDLE`)

```
FORGEGUARDIAN_LOCAL_BUNDLE=/path/to/forgeguardian-bundle.tar.gz
  Extract binaries from local tarball — for air-gapped CI environments.
  Checked before any GitHub download attempt.
```

Example:
```bash
FORGEGUARDIAN_LOCAL_BUNDLE=/opt/bundles/forgeguardian-bundle.tar.gz bash install.sh
```

### GitHub Actions release workflow

`.github/workflows/release.yml` triggers on `v*.*.*` tags:

1. GoReleaser — builds binaries, creates GitHub release, pushes Homebrew formula
2. Docker build + push for `forgeguardian-api` and `forgeguardian-worker` to GHCR
3. Cosign keyless signing of both images
4. `$GITHUB_STEP_SUMMARY` with release links and all 3 install methods

---

## 11. How to Integrate into CI/CD

### GitHub Actions — scan project on PR

```yaml
name: Supply Chain Security
on:
  pull_request:
    paths: ['package.json', 'requirements.txt', 'go.mod', 'pom.xml', 'Cargo.toml']

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Install fgctl
        run: go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest

      - name: Update signatures
        run: fgctl update

      - name: Scan project (SARIF for Code Scanning)
        run: fgctl scan . --fail-on=high --format=sarif > results.sarif

      - name: Upload SARIF to GitHub Code Scanning
        uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: results.sarif

      - name: Also save JSON artifact
        run: fgctl scan . --format=json > scan.json

      - uses: actions/upload-artifact@v4
        with:
          name: scan-results
          path: scan.json
```

**Exit codes:** `0` = clean · `2` = findings ≥ `--fail-on` threshold · `1` = error

### Pre-commit hook

```bash
#!/usr/bin/env bash
# .git/hooks/pre-commit
if git diff --cached --name-only | grep -qE "package\.json|requirements\.txt|go\.mod"; then
    fgctl scan . --fail-on=critical
    if [ $? -eq 2 ]; then
        echo "ForgeGuardian: critical vulnerability found — commit blocked"
        exit 1
    fi
fi
```

---

## 12. Core Types Reference

```go
// Shared across all modules (internal/core/)

type PackageVersion struct {
    Ecosystem   string
    Name        string
    Version     string
    SourceURL   string
    Checksum    string            // SHA256 from registry
    PublishedAt time.Time
    Metadata    map[string]string
}

type SourceArtifact struct {
    PackageVersion
    LocalPath string
    SHA256    string
}

type BuiltArtifact struct {
    SourceArtifact
    OutputPath   string
    SHA256       string
    BuildLog     string
    Reproducible bool
}

type Finding struct {
    ID           string
    Severity     Severity
    Type         string         // "cve" | "sast" | "behavioral" | "malware" | "ai-model" | "mcp"
    Title        string
    Description  string
    Source       string         // "grype" | "osv" | "semgrep" | "trivy" | "behavioral" | ...
    FixedVersion string         // known safe version, e.g. "4.17.21"; "" if unknown
    Metadata     map[string]any
}

type Severity string
const (
    SeverityCritical      Severity = "CRITICAL"
    SeverityHigh          Severity = "HIGH"
    SeverityMedium        Severity = "MEDIUM"
    SeverityLow           Severity = "LOW"
    SeverityInformational Severity = "INFORMATIONAL"
)
```

---

## 13. Community Signatures

Community signatures work as detection signatures for supply chain threats. Every `fgctl update` merges the latest community signatures into your local store.

### Signature format

```json
{
  "id": "FG-npm-evil-pkg",
  "type": "blocklisted_package",
  "ecosystem": "npm",
  "severity": "critical",
  "title": "evil-pkg steals environment variables",
  "description": "Runs a postinstall script that harvests process.env and exfiltrates it.",
  "package": "evil-pkg",
  "source": "manual"
}
```

### Signature types

| type | Extra required field | What it detects |
|---|---|---|
| `blocklisted_package` | `package` | Specific confirmed-malicious package name |
| `typosquat_target` | `target` | Real package name being typosquatted |
| `behavioral_rule` | `rule` | Regex matched against source code |
| `malware_pattern` | `pattern` | Byte/string pattern found in malware |
| `mcp_injection_pattern` | `rule` | Prompt injection in MCP tool descriptions |
| `pickle_rule` | `rule` | Unsafe pickle/AI model patterns |

### Validate before submitting

```bash
fgctl sig validate ./my-signature.json
# ✓ Signature FG-npm-evil-pkg is valid and ready to submit.
#   git checkout -b sig/FG-npm-evil-pkg
#   git add ./my-signature.json
#   git commit -m "[sig] FG-npm-evil-pkg — steals environment variables"
#   git push origin sig/FG-npm-evil-pkg
```

Full format reference: [SIGNATURES.md](SIGNATURES.md)

---

*ForgeGuardian is open source — community contributions welcome.*  
*Guide last updated: 2026-05-24*
