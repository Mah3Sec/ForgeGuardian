# ForgeGuardian — Vulnerable Test Environment

This directory provides everything needed to test ForgeGuardian against intentionally vulnerable targets.

---

## Structure

```
testenv/
├── targets/                          ← Standalone vulnerable packages (scan targets)
│   ├── npm-malicious/evil-pkg/       ← Simulated malicious npm (postinstall exfil, prototype pollution, eval)
│   ├── npm-typosquatted/             ← Typosquatted 'expres' (vs 'express') — behavioral scanner
│   ├── pypi-vuln-app/                ← Python: yaml.load RCE, pickle, subprocess shell=True, SQL injection
│   ├── ai-model-unsafe/              ← HuggingFace model: pickle format, missing model card fields
│   └── mcp-server-injection/         ← MCP server: prompt injection, excessive permissions, direct exec
│
├── projects/                         ← Realistic apps with vulnerable deps (for patch agent)
│   ├── vuln-node-app/package.json    ← lodash@4.17.20, minimist@1.2.5, axios@0.21.1, qs@6.5.2
│   ├── vuln-python-app/requirements.txt  ← PyYAML@5.3.1, Pillow@8.3.1, requests@2.25.0, Django@2.2.0
│   └── vuln-java-app/pom.xml         ← log4j-core@2.14.1 (Log4Shell), Spring4Shell, commons-collections
│
├── docker-compose.testenv.yml        ← Isolated test network with all services
├── Dockerfile.testenv                ← ForgeGuardian container with grype + semgrep + osv-scanner
├── Dockerfile.vuln-python            ← Vulnerable Flask API container
├── run-tests.sh                      ← Automated test suite (8 groups, 20+ assertions)
└── README.md                         ← This file
```

---

## Quick Start — Local Machine (No Docker)

### Prerequisites
```bash
go build -o fgctl ./cmd/fgctl/main.go
go build -o fg-agent ./agent/main.go

# Optional but recommended — adds grype + osv-scanner coverage:
brew install anchore/grype/grype
pip3 install semgrep
go install github.com/google/osv-scanner/cmd/osv-scanner@latest
```

### Run the automated test suite
```bash
# From repo root
bash testenv/run-tests.sh
```

### Run individual tests manually

**1. Clean baseline (expect no findings):**
```bash
./fgctl scan --recipe=npm --package=chalk --version=5.3.0
```

**2. Known CVE — lodash (prototype pollution, CVSS 7.4+):**
```bash
./fgctl scan --recipe=npm --package=lodash --version=4.17.20 --json | jq '.summary'
```

**3. Log4Shell (CVSS 10.0) — the most famous supply chain CVE:**
```bash
./fgctl scan \
  --recipe=maven \
  --package=org.apache.logging.log4j:log4j-core \
  --version=2.14.1 \
  --json | jq '.findings[] | select(.severity=="CRITICAL")'
```

**4. Python dangerous patterns:**
```bash
./fgctl scan --recipe=pypi --package=PyYAML --version=5.3.1
```

**5. AI Advisory (requires ANTHROPIC_API_KEY):**
```bash
export ANTHROPIC_API_KEY=sk-ant-...
./fgctl advisory --recipe=npm --package=lodash --version=4.17.20
```

**6. Full patch agent on vulnerable Node project — dry run:**
```bash
export ANTHROPIC_API_KEY=sk-ant-...
./fg-agent \
  --recipe=npm \
  --package=lodash \
  --version=4.17.20 \
  --project-dir=testenv/projects/vuln-node-app
```

**7. Full patch agent — apply (writes package.json):**
```bash
# Copy first so you can restore
cp testenv/projects/vuln-node-app/package.json /tmp/package.json.bak

./fg-agent \
  --recipe=npm \
  --package=lodash \
  --version=4.17.20 \
  --project-dir=testenv/projects/vuln-node-app \
  --apply

# Verify the version was bumped
cat testenv/projects/vuln-node-app/package.json

# Restore
cp /tmp/package.json.bak testenv/projects/vuln-node-app/package.json
```

---

## Docker — Fully Isolated Environment

Recommended for `--apply` testing and running the vulnerable webapp targets.

```bash
# Start all services (ForgeGuardian + vuln targets)
docker compose -f testenv/docker-compose.testenv.yml up -d

# Interactive shell in the ForgeGuardian container
docker compose -f testenv/docker-compose.testenv.yml exec forgeguardian bash

# Inside the container:
fgctl scan --recipe=npm --package=lodash --version=4.17.20
bash /run-tests.sh

# Stop and clean up
docker compose -f testenv/docker-compose.testenv.yml down -v
```

**Accessible services (localhost only):**
| Service | URL | Notes |
|---------|-----|-------|
| Vulnerable webapp (Mutillidae) | http://localhost:8888 | SQL injection, XSS, etc. |
| OWASP Juice Shop | http://localhost:3333 | Realistic vulnerable Node app |
| Vulnerable Python API | http://localhost:5555 | yaml.load + shell injection endpoints |

---

## What Each Target Tests

### npm-malicious/evil-pkg
Tests behavioral + Semgrep scanners for:
- `postinstall` script accessing `process.env` (env var harvesting pattern)
- `child_process` usage in lifecycle scripts
- `eval()` with user input
- Prototype pollution via unsafe `for..in` assignment
- Base64-encoded payload (obfuscation indicator)
- SQL injection via string concatenation

### npm-typosquatted
Tests behavioral scanner for:
- Package name Levenshtein distance ≤ 2 from high-popularity package (`express`)
- Module-load side-effects (env var access at require time)

### pypi-vuln-app
Tests Semgrep + OSV scanners for:
- `yaml.load()` without `Loader` (CVE-2020-14343, CVSS 9.8)
- `pickle.loads()` on untrusted data
- `subprocess` with `shell=True` + user input
- `eval()` on user-controlled expression
- SQL injection via f-string
- Hardcoded AWS credentials + passwords
- `os.system()` with unsanitized input

### ai-model-unsafe
Tests AI model scanner for:
- `safe_serialization: false` in model config
- Pickle format (no safetensors alternative)
- Model card missing required safety sections

### mcp-server-injection
Tests MCP scanner for:
- Prompt injection in tool descriptions (instructions to ignore prior context)
- Overly broad filesystem permissions (`filesystem:read:/`)
- Excessive permission combinations (`shell:exec + network:outbound + filesystem:write:/`)
- `child_process.execSync()` called directly from tool handler

### projects/vuln-node-app
Tests the **patch agent** end-to-end. Contains lodash, minimist, axios, qs — all with known CVEs. Agent should:
1. Detect vulnerabilities in the dependencies
2. Plan version upgrades
3. Write updated versions to `package.json` (with `--apply`)

### projects/vuln-java-app
Tests Maven scanning against Log4Shell (CVE-2021-44228, CVSS 10.0) and Spring4Shell (CVE-2022-22965, CVSS 9.8).

---

## Recommended Test Progression

```
Level 1 — No API key needed
  → chalk@5.3.0 (clean baseline)
  → lodash@4.17.20 (known CVEs, JSON output)
  → log4j-core@2.14.1 (Log4Shell)
  → SBOM generation
  → Sign + Verify

Level 2 — With ANTHROPIC_API_KEY
  → AI advisory on lodash
  → Patch agent dry-run on vuln-node-app

Level 3 — Docker (isolated)
  → Patch agent --apply on vuln-node-app
  → Full test suite via run-tests.sh
  → Explore vulnerable webapp at localhost:8888
```
