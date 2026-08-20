# Contributing to ForgeGuardian

ForgeGuardian is open-core — the engine, CLI, scanner, and signature tools are Apache 2.0 open source. Contributions make the whole community safer.

**Fastest path: write a detection signature — no Go knowledge required, 10 minutes.**

---

## Easiest: Write a Detection Signature

A signature is a small JSON file that tells ForgeGuardian about a known threat. When you submit one, it protects every person who uses the tool.

### Step 1 — Create your signature file

```json
{
  "id": "FG-npm-your-package-name",
  "type": "blocklisted_package",
  "ecosystem": "npm",
  "severity": "critical",
  "title": "evil-pkg steals environment variables",
  "description": "evil-pkg runs a postinstall script that harvests process.env and sends it to an external server.",
  "package": "evil-pkg",
  "source": "manual"
}
```

**Signature types:**

| type | Use when |
|---|---|
| `blocklisted_package` | A specific package is confirmed malicious |
| `typosquat_target` | A fake package mimics a real one (set `target` to the real name) |
| `behavioral_rule` | Suspicious code pattern (set `rule` to a regex) |
| `malware_pattern` | Known byte/string pattern in malware (set `pattern`) |
| `mcp_injection_pattern` | AI agent tool with hidden instructions (set `rule`) |
| `pickle_rule` | Unsafe Python pickle or AI model pattern (set `rule`) |

Full field reference: [SIGNATURES.md](SIGNATURES.md)

### Step 2 — Validate it

```bash
fgctl sig validate ./my-signature.json
```

This tells you exactly what's wrong before you submit.

### Step 3 — Submit a PR

```bash
git checkout -b sig/FG-npm-evil-pkg
git add my-signature.json
git commit -m "[sig] FG-npm-evil-pkg — steals environment variables"
git push origin sig/FG-npm-evil-pkg
```

Open a pull request. Title format: `[sig] FG-<id> — <one line description>`

**Rules for signatures:**
- Detection only — no exploit code, no PoCs, no working attack payloads
- One signature per PR
- Must pass `fgctl sig validate` before submitting

---

## Not sure what to write? Report it instead

If you found something suspicious but don't want to write the JSON yourself, just open an issue:

- [Report a malicious package](https://github.com/mah3sec/forgeguardian/issues/new?template=report-malicious-package.yml)
- [Report typosquatting](https://github.com/mah3sec/forgeguardian/issues/new?template=report-typosquatting.yml)

We'll write the signature from your report.

---

## Code Contributions

### Dev setup

```bash
git clone https://github.com/mah3sec/forgeguardian
cd forgeguardian
bash scripts/bootstrap.sh   # validates Go ≥1.25, Node ≥20, optional tools; runs go build + npm ci
make build                  # → bin/fgctl  bin/fg-agent  bin/intel-agent
make test                   # all unit tests
make smoke-test             # integration tests against real packages (needs internet)

# Verify your environment
./bin/fgctl doctor          # health check
./bin/fgctl doctor --fix    # auto-repair: installs missing tools, creates config/policy files (prints each repair command before running it)
./bin/fgctl debug           # full diagnostic dump

# Start the dev stack
make up                     # postgres + redis + api (docker required)
cd dashboard && npm run dev # dashboard at http://localhost:5173
```

Code standards: `gofmt` · `go vet` · no `any` in TypeScript · 70%+ coverage on `internal/core/`

### Add a new scan engine

Implement the `core.Scanner` interface in `internal/scanner/yourengine/`:

```go
type Scanner interface {
    Name() string
    Scan(ctx context.Context, artifact core.BuiltArtifact) ([]core.Finding, error)
}
```

Then register it in `internal/scanner/orchestrator.go`.

**Note on local scanning:** if `artifact.LocalPath == ""`, the artifact was synthesized for a local manifest scan (no downloaded archive). Archive-based engines should return early in this case. Name+version-based engines (OSV, behavioral, malware) should still run.

### Add a new ecosystem

1. Add a scraper in `internal/scrapers/yourecosystem/` implementing `core.RegistryScraper`
2. Add a manifest parser in `internal/localscanner/manifest.go` (add case to `ParseManifest` and entry to the walker's `manifestNames` map)
3. Add a build recipe in `internal/build/recipes/yourecosystem/`

### Add a new CLI command

All CLI output goes through `internal/ui/printer.go`. Use `p.Success()`, `p.Warn()`, `p.Error()`, `p.PrintMissingToolsWarning()`, `p.PrintFindingCompact()`, `p.PrintSummaryTable()`, `p.Spinner()`, and `p.Divider()` — never raw `fmt.Printf` for user-facing output.

Add your command to the `switch` in `cmd/fgctl/main.go`. Implementations for `patch`, `monitor`, `audit`, `debug`, `config`, and `policy` live in `cmd/fgctl/commands.go` — put new multi-function commands there, keep main.go as the dispatcher. The `policy` command (`runPolicy()` in `commands.go`) is a good example of a multi-subcommand implementation (show/init/set/validate).

For SARIF output use `ui.WriteSARIF()` from `internal/ui/sarif.go`. For config persistence use `loadConfig()`/`saveConfig()` from `commands.go`.

### Dashboard (React)

```bash
cd dashboard
nvm use          # uses Node 20 LTS from .nvmrc
npm ci
npm run dev      # http://localhost:5173
npm run build    # production build
```

All new UI components go in `src/components/ui/` (shadcn/ui style — use the existing `cn()` helper from `utils.ts`). New pages go in `src/pages/` and need routes in `App.tsx` and nav entries in `Sidebar.tsx`.

---

## Pull Request checklist

- [ ] `go build ./cmd/fgctl/` passes (and `go build ./...` for other modules)
- [ ] `go vet ./...` passes
- [ ] `fgctl doctor` shows no new FAILs from my change
- [ ] `fgctl debug` shows expected output for any new command
- [ ] For dashboard changes: `tsc --noEmit` passes in `dashboard/` and `npm run build` succeeds
- [ ] Tests added for new logic in `internal/`
- [ ] No secrets committed (`.env` files, API keys)
- [ ] New scan flags added to both `cmd/fgctl/main.go` (flag definition) and the scan filter struct; also update `localScanOpts` struct, `filterResults()`, `runLocalScan()`, and the `printUsage()` help text
- [ ] Grouped output: new flags that affect output mode should update `PrintGroupedFindings()` or `p.Verbose`/`p.Debug` in `internal/ui/printer.go`
- [ ] New findings sources populate `core.Finding.FixedVersion` when a fix version is available
