# ForgeGuardian Community Signatures

Detection signatures for supply chain attacks — contributed by the community, protecting everyone.

## Quickstart — contribute a signature in 5 minutes

```bash
# 1. Create a signature interactively
fgctl intel new

# 2. Validate it
fgctl intel validate ./FG-npm-my-sig.yaml

# 3. Test it against a real package
fgctl intel test ./FG-npm-my-sig.yaml \
  --ecosystem=npm --package=evil-package --version=1.0.0

# 4. Fork this repo, place the file in the right folder, open a PR
```

That's it. CI validates automatically. A maintainer reviews within 48–72 hours.

---

## Repository layout

```
signatures/
├── blocklisted/           Specific package versions confirmed malicious
│   ├── npm/
│   ├── pypi/
│   ├── go/
│   ├── crates/
│   ├── maven/
│   └── rubygems/
├── typosquatting/         Popular packages to protect from name-squatting
│   ├── npm/
│   └── pypi/
├── behavioral/            Dangerous install script patterns (regex)
│   ├── npm/
│   └── pypi/
├── malware/               Byte/regex patterns in malicious code (any ecosystem)
├── mcp/                   MCP server prompt injection patterns
├── ai-model/              Unsafe AI model configurations
├── scripts/
│   └── build_bundle.py    Builds dist/signatures.json from all YAML files
└── dist/
    └── signatures.json    Built bundle — loaded by fgctl intel update
```

---

## File naming

`<id>.yaml` where `id` matches the `id:` field exactly.

Examples:
- `FG-npm-event-stream-backdoor.yaml`
- `FG-pypi-typosquat-requests.yaml`
- `FG-malware-base64-eval-reverse-shell.yaml`

---

## Required fields

| Field | Required | Example |
|---|---|---|
| `id` | ✅ | `FG-npm-evil-pkg` |
| `name` | ✅ | `Backdoor in evil-pkg@1.0.0` |
| `type` | ✅ | `blocklisted_package` |
| `ecosystem` | ✅ | `npm` |
| `severity` | ✅ | `CRITICAL` |
| `description` | ✅ | Multi-line explanation |
| `author` | ✅ | Your GitHub username |
| `package` | If `blocklisted_package` | `evil-pkg` |
| `target` | If `typosquatting_target` | `lodash` |
| `pattern` | If `malware_pattern` or `mcp_injection_pattern` | Regex string |
| `rule` | If `behavioral_rule` or `pickle_rule` | Rule description |
| `cve` | Optional | `CVE-2024-3094` |
| `references` | Optional | List of URLs |

---

## PR title format

```
[sig] FG-<id> — <short name>
```

Example: `[sig] FG-npm-polyfill-io-cdn-hijack — polyfill.io CDN domain hijack 2024`

---

Full guide: [SIGNATURES.md](../SIGNATURES.md)
