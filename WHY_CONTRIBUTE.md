# Why Contribute to ForgeGuardian Signatures

## The Problem Nobody Can Solve Alone

A new malicious npm package gets published. It typosquats `lodash`. It sits
there for 6 hours before anyone notices. In those 6 hours, it gets installed
inside 400 CI pipelines across the world, harvesting environment variables and
sending them to an attacker's server.

This keeps happening — not because the tools are bad, but because no single
team can watch every package, in every ecosystem, every hour of the day.

**But a community can.**

---

## What ForgeGuardian Does for You

ForgeGuardian is a free, open-source, local-first AI-native software supply chain security platform that scans every package you use before it enters your pipeline:

- Checks it against known CVEs (8 engines, running in parallel)
- Detects typosquatting, suspicious lifecycle scripts, and malware patterns
- Scans AI model weights for pickle exploits and unsafe configurations
- Scans MCP servers for prompt injection and excessive permissions
- Generates a signed, cryptographic attestation for every clean package
- Uses Claude to write you an advisory — not just a CVE ID, but an actual
  explanation of what's exploitable and how to fix it
- Assigns every package an A–F risk grade, synthesizing CVE severity, behavioral signals, supply-chain patterns, and maintenance health
- Enforces security policy-as-code from `~/.forgeguardian/policy.yaml` — block packages, set fail thresholds, and auto-notify your team on Slack or Discord

It's the kind of tool that, a few years ago, only the largest companies could afford to build internally. Now it's free for everyone.

---

## What Happens When You Contribute a Signature

You spot something suspicious. Maybe you caught a typosquatted package before
it tricked you. Maybe you saw a writeup about a backdoor. Maybe you noticed a
MCP server tool description trying to get the LLM to leak secrets.

You write 10 lines of YAML. You open a PR.

Within 48 hours, that signature is reviewed, merged, and shipped in the next
signatures release. From that moment:

- **Every developer** running `fgctl scan` or using the VS Code extension
  gets warned if they encounter that package or pattern
- **Every CI pipeline** integrated with ForgeGuardian blocks it automatically
- **The AI intel-agent** learns from your signature to generate related ones

You turned a one-time catch into permanent, community-wide protection.

---

## You Don't Need to Be a Security Expert

The templates are designed so that anyone who can read a `package.json` can
write a signature. If you spotted something suspicious, that instinct is the
hard part. We've made the YAML as simple as possible.

Examples of contributions from real community members in similar projects:

- "I searched npm for packages with names similar to `react` and found `reakt` with a
  suspicious postinstall script" → typosquatting signature, 20 minutes of work
- "I saw this writeup about a backdoored Python package" → blocklisted_package
  signature, copy-paste from the article, 10 minutes
- "I noticed MCP servers can hide instructions in parameter descriptions, not
  just tool descriptions" → new mcp_injection_pattern, opened discussion first

None of these required deep security knowledge. They required paying attention.

---

## The Bigger Picture

ForgeGuardian's goal is to be the community's immune system for software supply
chains — the same way Nuclei's 9,000+ community templates protect web
applications at scale.

AI agents (Intel-Agent) handle the volume — pulling feeds, generating bulk
signatures automatically. The community handles the quality — catching what the
feeds miss, verifying edge cases, flagging emerging attack patterns.

Neither works as well without the other.

Every signature you contribute makes the tool more useful. More useful means
more users. More users means more eyes. More eyes means faster detection for
everyone — including you.

---

**Ready to contribute?** See [SIGNATURES.md](SIGNATURES.md) for the step-by-step guide.  
It takes less time than you think.
