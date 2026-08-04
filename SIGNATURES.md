# ForgeGuardian Signatures — Community Contribution Guide

> Spotted a malicious package? Found a typosquatting attempt? Noticed a suspicious pattern?  
> Write 10 lines of YAML. Everyone running ForgeGuardian is protected in the next release.

---

## Why Your Contribution Matters

Supply chain attacks don't get stopped by one team watching one feed. They get stopped by thousands of people watching thousands of packages — and sharing what they find.

When you contribute a signature to ForgeGuardian:

- **Every developer** who runs `fgctl scan` or uses the VS Code extension gets protected — automatically, in the next update
- **Every CI pipeline** using ForgeGuardian will flag that package before it ever gets merged
- **The AI intel-agent** learns from community signatures to generate better ones on its own
- **You don't need to be a security expert.** If you can spot something suspicious in a package, you can write a signature. We'll help with the rest.

This is how the open-source security community works at its best — the same way Nuclei's 9,000+ community templates protect millions of users. No single company can watch everything. But a community can.

---

## What is a Signature?

A signature is a small YAML file that tells ForgeGuardian's scanners what to look for. It's **detection only** — no exploit code, no proof-of-concept, just a precise description of a threat pattern.

There are 6 types. Each one maps to a specific scanner:

| Type | What it detects | Example |
|------|----------------|---------|
| `blocklisted_package` | A specific package version known to be malicious | `event-stream@3.3.6` backdoor |
| `typosquatting_target` | Similar names that trick developers into installing the wrong package | `lodahs` instead of `lodash` |
| `behavioral_rule` | Dangerous patterns in package lifecycle scripts | `postinstall` that makes network calls + reads env vars |
| `malware_pattern` | Byte or regex patterns found in known malware | base64-encoded reverse shell payload |
| `mcp_injection_pattern` | Prompt injection attempts inside MCP server tool descriptions | "ignore previous instructions" in a tool description |
| `pickle_rule` | Unsafe AI model configurations | `safe_serialization: false` in a HuggingFace model config |

Pick the type that fits what you found. If you're not sure, open an issue and ask — we'll help you figure it out.

---

## Creating a Signature — Step by Step

### Step 1 — Copy the right template

Pick the template that matches what you found and copy it:

---

#### Template A: Blocklisted Package

Use this when a specific package version is known to contain malicious code.

```yaml
id: FG-<ecosystem>-<short-name>
# Example: FG-npm-event-stream-backdoor
# Rules: lowercase, hyphens only, start with FG-<ecosystem>-

name: <Short human-readable name>
# Example: Backdoor in event-stream@3.3.6

type: blocklisted_package
ecosystem: npm   # npm | pypi | maven | go | rubygems | crates | huggingface | mcp
severity: CRITICAL   # CRITICAL | HIGH | MEDIUM | LOW

package: <package-name>
version_range: "=3.3.6"
# Version range format:
#   =1.2.3      exact version
#   <1.2.3      any version below
#   <=1.2.3     this version and below
#   >=1.2.3     this version and above
#   >=1.0,<2.0  range

description: |
  One or two sentences. What does this package do that's malicious?
  Be specific about the behavior (data exfiltration, backdoor, cryptominer, etc.)

cve: CVE-XXXX-XXXXX   # Remove this line if no CVE exists
references:
  - https://github.com/...   # link to the issue, report, or writeup
  - https://nvd.nist.gov/...

author: your-github-username
tags: [backdoor]   # choose from: backdoor, cryptominer, exfiltration, ransomware, typosquatting, obfuscation
```

---

#### Template B: Typosquatting Target

Use this when a popular package is a likely target for typosquatting attacks.

```yaml
id: FG-<ecosystem>-typosquat-<package-name>
# Example: FG-npm-typosquat-lodash

name: Typosquatting target — <package-name>

type: typosquatting_target
ecosystem: npm
severity: HIGH

target: <exact-package-name>
# The legitimate popular package to protect.

similar_names:
  - <misspelling-1>
  - <misspelling-2>
  - <misspelling-3>
# List of names that could trick a developer.
# Think: transposed letters, digit substitutions (l→1, O→0),
#        missing letters, added letters, different casing.

description: |
  <package-name> has X million weekly downloads.
  Any package published under these names should be treated as a
  potential typosquatting attempt and flagged for manual review.

references:
  - https://npmjs.com/package/<package-name>   # link to the legitimate package

author: your-github-username
tags: [typosquatting]
```

> **Tip for finding typosquatting names:** Think about common keyboard slip-ups.
> `lodash` → `lodahs` (transposed), `1odash` (l→1), `lodsh` (missing letter), `lodashh` (extra letter).

---

#### Template C: Behavioral Rule

Use this when you've noticed a dangerous pattern in package scripts — not a specific package, but a class of behavior.

```yaml
id: FG-<ecosystem>-behavioral-<short-name>
# Example: FG-npm-behavioral-postinstall-env-harvest

name: <Short description of the behavior>

type: behavioral_rule
ecosystem: npm
severity: HIGH

rule: |
  Describe the rule in plain English. What combination of factors makes this dangerous?
  Example:
    Package has a postinstall script
    AND the script source contains process.env
    AND the script source contains any of: http, https, net, dns, fetch, axios, request
# The scanner uses this description to match against behavioral analysis output.

description: |
  Explain why this pattern is dangerous.
  What is the attacker trying to do?
  What is the impact if exploited?

author: your-github-username
tags: [postinstall, exfiltration]   # tag with the relevant behaviors
```

---

#### Template D: Malware Pattern

Use this when you have a specific string, regex, or byte pattern that reliably identifies malicious code.

```yaml
id: FG-malware-<short-name>
# Example: FG-malware-base64-reverse-shell-npm

name: <Short name of the malware pattern>

type: malware_pattern
ecosystem: npm   # or: pypi | go | rubygems | crates | any
severity: CRITICAL

pattern: "<regex or string pattern>"
# Use regex for flexible matching. Keep it specific to avoid false positives.
# Examples:
#   "eval\\(Buffer\\.from\\(['\"][A-Za-z0-9+/]{20,}['\"],\\s*['\"]base64['\"]\\)"
#   "require\\(['\"]child_process['\"]\\)\\.exec\\("
#   "0x[0-9a-fA-F]{8,}\\s*,\\s*0x[0-9a-fA-F]{8,}"   # shellcode-like byte sequence

description: |
  What does this pattern match? Where did you find it?
  Give a brief explanation of why this is malicious, not just unusual.

references:
  - https://...   # link to writeup, malware sample, or analysis

author: your-github-username
tags: [malware, obfuscation]
```

---

#### Template E: MCP Injection Pattern

Use this when a specific string or pattern in an MCP server's tool description or code is used to manipulate LLM behavior.

```yaml
id: FG-mcp-<short-name>
# Example: FG-mcp-ignore-previous-instructions

name: <Short name of the injection pattern>

type: mcp_injection_pattern
ecosystem: mcp
severity: CRITICAL

pattern: "<regex pattern to match in tool descriptions or code>"
# Example: "(?i)(ignore|disregard|forget).{0,30}(previous|prior|above|system|instruction)"
# Example: "(?i)(output|print|reveal|expose).{0,30}(secret|api.?key|token|password|credential)"

description: |
  Explain what this injection attempts to do.
  Which part of the MCP server does it appear in? (tool description, parameter description, tool handler)
  What LLM behavior does it try to induce?

references:
  - https://...

author: your-github-username
tags: [prompt-injection, mcp]
```

---

#### Template F: Pickle / AI Model Rule

Use this when an AI model has a configuration or file structure that makes it unsafe to load.

```yaml
id: FG-ai-<short-name>
# Example: FG-ai-pickle-no-safetensors

name: <Short description>

type: pickle_rule
ecosystem: huggingface
severity: HIGH

rule: |
  Describe what configuration or file pattern makes this model unsafe.
  Example:
    Model config has safe_serialization: false
    AND no .safetensors file is present
    AND a .pkl or .bin file is present

description: |
  Why is this unsafe? What can happen when a user loads this model?
  Mention the attack vector (arbitrary code execution via pickle, etc.)

references:
  - https://huggingface.co/docs/...
  - https://nvd.nist.gov/...   # if a CVE exists

author: your-github-username
tags: [pickle, unsafe-deserialization, ai-model]
```

---

### Step 2 — Fill it in

Fill in every field. The required fields are:

- `id` — must be unique. Check the [signatures index](https://github.com/forgeguardian/forgeguardian-signatures/blob/main/INDEX.md) to make sure yours doesn't already exist.
- `name` — short, clear, human-readable
- `type` — one of the 6 types above
- `ecosystem` — which package ecosystem
- `severity` — your best judgment. Reviewers will adjust if needed.
- `description` — what the threat is and why it's dangerous
- `author` — your GitHub username (gets credited in the release notes)

Optional but encouraged:
- `cve` — if a CVE exists
- `references` — links to writeups, issues, advisories

---

### Step 3 — Where to save the file

Save your file under the correct directory in the `forgeguardian-signatures` repo:

```
forgeguardian-signatures/
├── blocklisted/
│   ├── npm/
│   │   └── FG-npm-event-stream-backdoor.yaml
│   └── pypi/
│       └── FG-pypi-ctx-cryptominer.yaml
├── typosquatting/
│   ├── npm/
│   │   └── FG-npm-typosquat-lodash.yaml
│   └── pypi/
│       └── FG-pypi-typosquat-requests.yaml
├── behavioral/
│   └── npm/
│       └── FG-npm-behavioral-postinstall-env-harvest.yaml
├── malware/
│   └── FG-malware-base64-reverse-shell.yaml
├── mcp/
│   └── FG-mcp-ignore-previous-instructions.yaml
└── ai-model/
    └── FG-ai-pickle-no-safetensors.yaml
```

**File naming:** use the same value as your `id` field, with `.yaml` extension.

---

### Step 4 — Test your signature locally (optional but appreciated)

```bash
# Validate your YAML schema
fgctl intel validate ./my-signature.yaml

# Dry-run against the signature store — see if it triggers on a known bad package
fgctl intel test --signature=./my-signature.yaml --recipe=npm --package=event-stream --version=3.3.6
```

If you don't have ForgeGuardian set up locally, skip this step — CI will validate it automatically when you open the PR.

---

### Step 5 — Submit a Pull Request

1. Fork [github.com/forgeguardian/forgeguardian-signatures](https://github.com/forgeguardian/forgeguardian-signatures)
2. Add your `.yaml` file in the right directory
3. Open a PR with the title: `[sig] FG-<your-id> — <short name>`
4. In the PR description, briefly answer:
   - Where did you find this? (package name, registry URL, writeup link, etc.)
   - How confident are you? (seen in the wild, theoretical pattern, confirmed malicious)
   - Any edge cases or false positive risk?

That's it. A maintainer will review it within a few days.

---

## What Happens After You Submit

```
PR opened
    ↓
Automated CI checks (takes ~2 minutes):
  ✓ YAML schema valid
  ✓ Required fields present
  ✓ No executable code in the file
  ✓ ID is unique (no duplicates)
  ✓ Ecosystem is valid
  ✓ Severity is valid
    ↓
Maintainer review:
  - Is the threat real?
  - Is the description accurate?
  - False positive risk?
  - Severity appropriate?
    ↓
Merged → included in next signatures release
    ↓
Users pull it automatically:
  fgctl intel update          (manual pull)
  intel-agent --loop          (auto-pulled on next cycle)
```

**Review turnaround:** we aim for 48–72 hours. For CRITICAL findings (active exploitation), we fast-track same-day.

---

## What We Accept and What We Don't

### We accept

- Specific malicious packages with evidence (links, writeups, CVEs)
- Typosquatting patterns for packages with significant download counts
- Behavioral patterns based on observed attack techniques
- MCP injection patterns based on real prompt injection research
- AI model safety rules based on documented deserialization vulnerabilities

### We don't accept

- Proof-of-concept exploit code of any kind
- Signatures targeting packages based on political/personal reasons with no technical evidence
- Patterns so broad they would cause significant false positives
- Duplicate signatures (check the index first)
- Signatures without a description or references

If you're not sure whether your finding qualifies, open an issue first and describe what you found. We'll tell you if it fits.

---

## Recognition

Every merged signature credits the author in:

- The YAML file itself (`author:` field)
- The GitHub release notes for that signatures bundle
- The `CONTRIBUTORS.md` file in the signatures repo

Top contributors (by merged signatures per quarter) are highlighted in the ForgeGuardian community newsletter and GitHub README.

---

## Quick Reference Card

```
Found a malicious package?        → type: blocklisted_package
Popular package being typosquatted? → type: typosquatting_target
Suspicious postinstall pattern?   → type: behavioral_rule
Regex matching known malware?     → type: malware_pattern
MCP tool description injection?   → type: mcp_injection_pattern
Unsafe AI model config?           → type: pickle_rule

Severity guide:
  CRITICAL  Confirmed exploitation in the wild, or RCE/backdoor
  HIGH      Strong evidence of malice, or commonly exploited class
  MEDIUM    Suspicious pattern, elevated risk, not confirmed malicious
  LOW       Informational, best-practice violation

File location: signatures/<type>/<ecosystem>/<id>.yaml
PR title:      [sig] FG-<id> — <short name>
```

---

## Getting Help

- **Not sure which type to use?** → [Open an issue](https://github.com/forgeguardian/forgeguardian-signatures/issues) with `[question]` in the title
- **Found something critical?** → Flag it with `[critical]` in the PR title for fast-track review
- **Want to discuss a pattern before writing the signature?** → [GitHub Discussions](https://github.com/forgeguardian/forgeguardian/discussions)

The bar for contributing is intentionally low. If you spotted something suspicious, that instinct is valuable. We'll help you turn it into a proper signature.

---

*ForgeGuardian Signatures — MIT License — Community Maintained*
