# How ForgeGuardian Secures Itself

ForgeGuardian is a supply chain security tool — it must hold itself to the same standards it enforces on others.

---

## Dependency Audit Pipeline

Every pull request and push to `main`/`develop` runs `npm audit --audit-level=moderate` inside the `dashboard/` directory as part of the `ts-ci` GitHub Actions job. Any moderate, high, or critical vulnerability in a direct or transitive dependency **blocks the build**.

For Go dependencies, `go vet ./...` runs on every CI invocation.

To run dependency audits locally:

```bash
# Frontend
cd dashboard && npm audit --audit-level=moderate

# Go
go vet ./...
```

---

## Lockfile Integrity

Both `package-lock.json` (npm) and `go.sum` (Go modules) are committed and validated on every CI run.

- `npm ci` (not `npm install`) is used in CI — it fails if `package-lock.json` is out of sync with `package.json`.
- Go module checksums are verified by the Go toolchain against the sum database (`sum.golang.org`).

**Never** use `--legacy-peer-deps` or `--force` when installing frontend dependencies without a documented reason.

---

## Production Build Hardening

The dashboard is compiled with Vite in production mode (`vite build`), which applies:
- Dead-code elimination and minification (esbuild)
- Source map generation is disabled by default in the release zip
- No `eval()` or `new Function()` constructs (lint-enforced)

The Go CLI binary is built with:
```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath ./cmd/fgctl/
```

`-s -w` strips debug symbols and DWARF info. `-trimpath` removes local filesystem paths from the binary.

Container images use `distroless/static-debian12` — no shell, no package manager, no debug utilities in production.

---

## Development Isolation

Development secrets (API keys, DB passwords) are stored in `.env` (git-ignored). The `.gitignore` explicitly excludes:
- `.env`, `.env.*`
- `*.key`, `*.pem`, `*.p12`
- `node_modules/`, `dist/`, `coverage/`

The `docker-compose.yml` dev stack uses fixed development-only credentials (`devpassword`, `minioadmin`) that are never reused in production.

---

## Static Analysis

Every PR runs:
- **Semgrep** (`auto` config) — catches OWASP Top 10 patterns, injection vulnerabilities, hardcoded secrets, and insecure function calls
- **go vet** — Go static analysis (type safety, unreachable code, suspicious constructs)
- **TypeScript strict mode** — `"strict": true` in all `tsconfig.json` files; `no-any` lint rule enforced

---

## Container Security

- All containers run as `nonroot:nonroot` (UID 65532)
- Root filesystems are mounted read-only in Kubernetes manifests (`readOnlyRootFilesystem: true`)
- No `privileged: true` or `CAP_NET_ADMIN` in any workload spec
- SBOM and provenance attestations are generated for every Docker image via `docker/build-push-action` with `sbom: true` and `provenance: mode=max`

---

## Vulnerability Reporting

If you discover a security vulnerability in ForgeGuardian:

1. **Do not open a public GitHub issue.**
2. Email the maintainers directly (see `package.json` or GitHub repository contacts).
3. Include: CVE ID if known, affected version, reproduction steps, and impact assessment.

We aim to acknowledge reports within 48 hours and release a patch within 14 days for confirmed critical issues.

---

## Supply Chain Provenance

ForgeGuardian release artifacts are signed with [Sigstore](https://sigstore.dev) keyless signing via the `signer/` module. Every release includes:
- A `forgeguardian-vX.Y.Z.zip` archive
- A Sigstore bundle (`forgeguardian-vX.Y.Z.zip.sigstore.json`) for verification
- A CycloneDX SBOM (`forgeguardian-vX.Y.Z.sbom.json`)

Verify a release:
```bash
cosign verify-blob \
  --bundle forgeguardian-vX.Y.Z.zip.sigstore.json \
  forgeguardian-vX.Y.Z.zip
```
