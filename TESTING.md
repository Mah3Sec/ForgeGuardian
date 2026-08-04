# ForgeGuardian — Verification Guide

How to confirm ForgeGuardian actually works on your machine, from a clean
checkout, with real commands and real expected output. Every command below
was run against the current `main` before writing this doc.

If a step doesn't match its expected result, that's a bug — file an issue
with the command, the output you got, and your OS/Go version.

---

## 0. Prerequisites

```bash
go version      # need 1.23+
git rev-parse HEAD
```

Optional (unlocks extra scan engines — not required for core verification):
```bash
brew install anchore/grype/grype
brew install aquasecurity/trivy/trivy
pip3 install semgrep
```

---

## 1. Build

```bash
go build ./...                 # compiles every package, no output = pass
go vet ./...                   # static checks, no output = pass
go build -o fgctl ./cmd/fgctl
go build -o fg-agent ./cmd/fg-agent
```

**Pass:** both binaries exist and are executable.
```bash
./fgctl --version
# fgctl dev (none) built unknown   ← version string prints, exit 0
```

---

## 2. Unit tests

```bash
go test ./...
```

**Pass:** every package prints `ok` or `[no test files]`. Any `FAIL` is a
real regression — do not proceed until it's green.

---

## 3. Automated smoke suite (the real integration test)

This is the fastest way to know the whole pipeline works end to end. It
scans known-vulnerable real packages and asserts on the findings.

```bash
FGCTL=./fgctl AGENT=./fg-agent bash testenv/run-tests.sh
```

Needs internet (fetches real packages from npm/PyPI/Maven). Takes ~30-60s.

**Pass — expected output shape:**
```
[PASS] binary exists: ./fgctl
[PASS] binary exists: ./fg-agent
[PASS] chalk@5.3.0: 0 critical findings (expected)
[PASS] lodash@4.17.20: N findings (expected >= 1)
[PASS] minimist@1.2.5: N findings (expected >= 1)
[PASS] axios@0.21.1: N findings (expected >= 1)
[PASS] qs@6.5.2: N findings (expected >= 1)
[PASS] Pillow@8.3.1: N findings (expected >= 1)
[PASS] PyYAML@5.3.1: N findings (expected >= 1)
[PASS] requests@2.25.0: N findings (expected >= 1)
[PASS] org.apache.logging.log4j:log4j-core@2.14.1: N findings (expected >= 1)
[PASS] lodash@4.17.20 --fail-on=high exits 2 (exit 2)
[PASS] SBOM: valid CycloneDX JSON produced
[PASS] sign: attestation produced
[PASS] verify: attestation valid
[SKIP] AI advisory (reason: ANTHROPIC_API_KEY not set)
[SKIP] patch agent (reason: ANTHROPIC_API_KEY not set)

  Results: 14 PASS  0 FAIL  2 SKIP
```

The 2 SKIPs are expected without `ANTHROPIC_API_KEY` set — see §6.

A single package occasionally failing with a network/TLS error
(`stream error`, `context deadline exceeded`) is registry/CDN flakiness, not
a ForgeGuardian bug — rerun that one scan manually to confirm:
```bash
./fgctl scan --recipe=pypi --package=Pillow --version=8.3.1 --json
```

---

## 4. Manual spot checks (no API key needed)

**Clean package — expect 0 critical findings:**
```bash
./fgctl scan --recipe=npm --package=chalk --version=5.3.0 --json | jq '.summary'
```

**Known CVE — lodash prototype pollution:**
```bash
./fgctl scan --recipe=npm --package=lodash --version=4.17.20 --json | jq '.findings[].id'
```

**Log4Shell (CVSS 10.0):**
```bash
./fgctl scan --recipe=maven --package=org.apache.logging.log4j:log4j-core --version=2.14.1 --json \
  | jq '.findings[] | select(.severity=="CRITICAL")'
```

**Local project scan (dot-notation + path form):**
```bash
mkdir -p /tmp/fgtest && cd /tmp/fgtest
echo '{"name":"test","dependencies":{"lodash":"4.17.20"}}' > package.json
/path/to/fgctl scan .
/path/to/fgctl scan npm/lodash@4.17.20
```

**CI-friendly exit codes:**
```bash
./fgctl scan --recipe=npm --package=lodash --version=4.17.20 --fail-on=high
echo "exit: $?"    # expect 2 — findings at/above HIGH present
```

**SBOM generation:**
```bash
./fgctl sbom --recipe=npm --package=chalk --version=5.3.0 --format=cyclonedx-json --out=/tmp/sbom.json
jq '.bomFormat' /tmp/sbom.json   # expect "CycloneDX"
```

**Sign + verify (Sigstore keyless):**
```bash
./fgctl sign --recipe=npm --package=chalk --version=5.3.0 --out=/tmp/att.json
SHA=$(jq -r '.sha256' /tmp/att.json)
./fgctl verify --attestation=/tmp/att.json --sha256=$SHA
echo "exit: $?"   # expect 0
```

**Doctor self-check:**
```bash
./fgctl doctor
./fgctl doctor --fix
```

---

## 5. Vulnerable target fixtures (deeper engine coverage)

`testenv/targets/` and `testenv/projects/` contain intentionally vulnerable
fixtures for each scanner. See `testenv/README.md` for the full breakdown of
what each target exercises (behavioral, malware, AI model, MCP scanners).

```bash
./fgctl scan testenv/targets/npm-malicious/evil-pkg
./fgctl scan testenv/targets/pypi-vuln-app
./fgctl scan testenv/targets/ai-model-unsafe
./fgctl scan testenv/targets/mcp-server-injection
```

**Pass:** each produces findings matching the vulnerability class described
in `testenv/README.md` for that target (e.g. the malicious npm target should
flag postinstall exfiltration + eval + prototype pollution).

---

## 6. AI features (needs `ANTHROPIC_API_KEY`)

```bash
export ANTHROPIC_API_KEY=sk-ant-...

./fgctl advisory --recipe=npm --package=lodash --version=4.17.20 --json | jq '.severity'

./fg-agent --recipe=npm --package=lodash --version=4.17.20 \
  --project-dir=testenv/projects/vuln-node-app
# dry-run by default — nothing written. Add --apply to actually patch.
```

Rerun the smoke suite with the key set to cover these two groups too:
```bash
ANTHROPIC_API_KEY=sk-ant-... FGCTL=./fgctl AGENT=./fg-agent bash testenv/run-tests.sh
```

---

## 7. Dashboard + API (full stack)

```bash
docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d
curl -sf localhost:8080/api/v1/health && echo "API OK"
cd dashboard && npm run dev   # http://localhost:5173
```

**Pass:** `API OK` prints, dashboard loads without console errors, scan
triggered from UI shows up in the live feed.

```bash
docker compose down
```

---

## Known-flaky, not a bug

- Individual `pypi` package downloads occasionally hit
  `stream error: stream ID 1; NO_ERROR` or `context deadline exceeded` — this
  is PyPI CDN / local network behavior, not ForgeGuardian. Retry the single
  command before reporting.
- `TOOL-NOT-INSTALLED` / `TRIVY-NOT-INSTALLED` / `SEMGREP-NOT-INSTALLED`
  findings are expected when the optional binaries (§0) aren't installed —
  they're informational, not failures.
