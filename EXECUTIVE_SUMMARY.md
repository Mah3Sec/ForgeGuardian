# ForgeGuardian — Executive Summary

**One-line pitch:** ForgeGuardian checks every open-source package your software depends on for known vulnerabilities, malware, and supply-chain attacks — before that code runs in your product.

## Why this matters

Modern software is built almost entirely out of other people's code — open-source packages pulled in from public registries (npm, PyPI, and similar). A typical application depends, directly or indirectly, on hundreds of these packages. Any one of them can be compromised: a maintainer's account gets hijacked, a malicious update gets published, or a package with a name one letter off from a popular one gets uploaded to trick developers into installing it. This has already happened to real companies — the [xz-utils backdoor](https://en.wikipedia.org/wiki/XZ_Utils_backdoor) in 2024 nearly compromised SSH access on a huge share of Linux servers worldwide, planted by a single trusted-looking maintainer over years.

ForgeGuardian scans for this class of risk automatically, as part of normal development — not as a one-time audit, but continuously, every time code changes.

## What it actually does

- **Scans dependencies** across 9 ecosystems (JavaScript, Python, Go, Ruby, Rust, Java, and AI-specific ones: HuggingFace models, MCP tool servers) for known vulnerabilities and confirmed-malicious packages.
- **Catches what vulnerability scanners alone miss**: typosquatting (fake packages with confusingly similar names), suspicious install-time behavior, and unsafe AI model files — not just "is there a CVE."
- **Explains findings in plain language** using AI, including how exploitable a given issue realistically is — not just a severity number.
- **Proves what shipped**: generates a verifiable, cryptographically signed record (SBOM + provenance) of exactly what went into a release, so "what's actually running in production" is a fact, not a guess.
- **Runs everywhere**: a command-line tool for engineers, a web dashboard for the team, and a CI integration that blocks a bad build automatically — all against the same engine, same results.

## Why it's different from "yet another vulnerability scanner"

Most tools in this space (Snyk, Dependency-Track, etc.) either require sending your dependency data to someone else's cloud, or only catch the narrow "known CVE" case. ForgeGuardian:

- **Runs entirely on your own infrastructure by default** — nothing about your codebase or dependencies is sent anywhere unless you explicitly turn on an AI feature (and even then, only the specific package being analyzed goes to the AI provider, never your source code).
- **Is free at the core, permanently** — the scanning engine, CLI, and community detection rules are open source (Apache 2.0) and stay that way. There's no bait-and-switch where core features get pulled behind a paywall later.
- **Covers AI supply chain risk**, which general-purpose scanners don't — increasingly relevant as companies adopt AI models and tools as dependencies, with the same blind trust that caused problems in traditional package ecosystems.

## Where this fits for different roles

| Role | What you get out of it |
|---|---|
| **Security engineer** | A real multi-engine scanner (8 detection methods running together) with a policy layer you control — block builds on your own rules, not someone else's defaults. |
| **Developer** | Findings show up with the exact fixed version to upgrade to — not just "this is vulnerable," but "here's the one-line fix." Runs locally, no waiting on a central team. |
| **Executive / budget owner** | No mandatory per-seat SaaS cost for the core capability. Self-hosted means your dependency data never leaves your network unless you choose otherwise — relevant for regulated industries and IP-sensitive teams. |
| **Client / partner evaluating a vendor's supply chain posture** | Every release can carry a signed, independently-verifiable record of what's in it (SBOM + provenance) — a concrete artifact to point to, not a claim. |

## Honest caveats (as of this writing)

- The commercial "Pro" tier referenced in project materials is not yet a shipped, billable product — no signup flow, no billing integration exists today. Everything described above is what's real and working right now, at the free/open-source tier.
- The project's source code is not yet published publicly on GitHub — that's a near-term step, not a technical limitation of the product itself.

## Learn more

- [README.md](README.md) — full feature list, installation, CLI reference
- [COMPETITIVE.md](COMPETITIVE.md) — honest, feature-by-feature comparison against Trivy, Grype, Snyk, Dependency-Track
- [THREAT_MODEL.md](THREAT_MODEL.md) — what this tool defends against, and what it explicitly does not
- [PRIVACY.md](PRIVACY.md) — exactly what data leaves your network, per command, and when
- [SECURITY.md](SECURITY.md) — how ForgeGuardian secures itself, and how to report a vulnerability in it
