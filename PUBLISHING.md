# Publishing ForgeGuardian — GitHub Push + Web Deployment

Two separate things covered here: (1) getting this code onto GitHub for the
first time, (2) putting the dashboard + API somewhere reachable on the
internet, with a real domain. Do them in that order — the domain/hosting
step assumes the repo already exists.

---

## Part 1 — Push to GitHub (first time)

### Prerequisites

- `gh` CLI installed (`brew install gh` on macOS) and authenticated (`gh auth login`).
- A clean local commit ready to push. If you've been working with an
  AI-assisted session, check `git status` and `git log` first — squash
  anything you don't want carrying the intermediate churn as separate
  commits.

### Steps

```bash
# 1. Confirm you're on the branch you actually want to push
git branch --show-current
git log --oneline

# 2. Create the GitHub repo (choose --public or --private)
gh repo create mah3sec/forgeguardian --public --source=. --remote=origin

# 3. Push
git push -u origin <your-branch>:main

# 4. Tag the release
git tag v2.0.0
git push origin v2.0.0
```

`gh repo create --source=.` both creates the repo on GitHub and wires up
the `origin` remote in one step — no separate `git remote add` needed.

### If the repo already exists (created via github.com web UI)

```bash
git remote add origin https://github.com/mah3sec/forgeguardian.git
git push -u origin <your-branch>:main
git tag v2.0.0 && git push origin v2.0.0
```

**Important**: if you created the repo via GitHub's web UI, make sure you
did *not* check "Add a README" / "Add .gitignore" / "Choose a license" —
any of those creates an initial commit on GitHub's side that conflicts
with your local history on push.

### Verify

```bash
gh repo view mah3sec/forgeguardian --web   # opens it in the browser
```

Check: README renders correctly, LICENSE is detected (GitHub shows an
"Apache-2.0" badge automatically), CI workflow (`.github/workflows/ci.yml`)
triggers on the push and passes.

### Releases

Once a `v*.*.*` tag is pushed, `.github/workflows/release.yml` runs
automatically — builds `fgctl`/`fg-agent`/`intel-agent` for
linux/macOS/windows via GoReleaser, signs the worker container image with
cosign, and publishes a GitHub Release with the binaries attached.

**Before your first real tag**: `.github/workflows/release.yml` requires a
repo secret `FG_LICENSE_SECRET` (Settings → Secrets and variables →
Actions) — the release job hard-fails without it, on purpose (see
`internal/license/license.go`'s doc comment for why: the license-gate
secret must never be the public dev-build default in a real release
binary). Generate one with:

```bash
openssl rand -hex 32
```

and paste it in as the secret value before tagging.

---

## Part 2 — Deploying to the web

ForgeGuardian has three independent pieces that can be deployed separately
or together: the **API** (Go, stateless, needs Postgres), the **dashboard**
(static React build, served by any static host or the API's own container),
and **fgctl** (a CLI binary — nothing to deploy, users install it locally).

### Option A — Minimal: API + dashboard on one small VM

Cheapest, simplest, good for internal/team use or a public demo.

1. **Provision a VM** (DigitalOcean droplet, Hetzner, AWS EC2 — any Linux box, 1-2 vCPU / 2GB RAM is enough to start).
2. **Point a domain at it.** Buy a domain (Namecheap, Cloudflare Registrar, Porkbun — any registrar). Add an A record:
   ```
   Type: A
   Name: @  (or a subdomain like "app")
   Value: <your VM's public IP>
   ```
   Propagation takes a few minutes to a few hours.
3. **On the VM**, clone the repo and build:
   ```bash
   git clone https://github.com/mah3sec/forgeguardian
   cd forgeguardian
   make docker-minimal   # postgres + redis + API, via docker-compose.minimal.yml
   make dashboard         # builds dashboard/dist/ (static files)
   ```
4. **Put a reverse proxy in front** (Caddy is the easiest — automatic HTTPS via Let's Encrypt with zero config):
   ```bash
   sudo apt install -y caddy
   ```
   `/etc/caddy/Caddyfile`:
   ```
   yourdomain.com {
       handle /api/* {
           reverse_proxy localhost:8080
       }
       handle {
           root * /path/to/forgeguardian/dashboard/dist
           try_files {path} /index.html
           file_server
       }
   }
   ```
   ```bash
   sudo systemctl reload caddy
   ```
   Caddy automatically gets a real TLS cert from Let's Encrypt the first
   time it starts with a real domain in the Caddyfile — no manual
   certbot steps.
5. **Set the dashboard's API URL at build time** (before step 3's `make dashboard`, or rebuild after):
   ```bash
   echo "VITE_API_URL=https://yourdomain.com/api" > dashboard/.env.production
   ```

### Option B — Container-based (more portable, easier to scale later)

Use the existing Dockerfiles (`internal/api/Dockerfile` for the API,
`dashboard/Dockerfile` for the dashboard — build from the repo root: `docker
build -f internal/api/Dockerfile -t forgeguardian-api .`, context matters,
the API Dockerfile needs the whole repo, not just its own directory) and
push images to a registry, then run on any container platform:

- **Fly.io** — `fly launch` in the repo root, point it at the API's
  Dockerfile, `fly deploy`. Fly auto-provisions a `*.fly.dev` domain and
  supports custom domains + automatic TLS out of the box.
- **Railway** — connect the GitHub repo directly in Railway's UI, it
  detects the Dockerfile and deploys on every push. Custom domain support
  built in.
- **AWS ECS / Kubernetes** — `infra/k8s/` and `infra/terraform/` already
  have a real starting point (EKS + RDS + S3 via Terraform, Kustomize
  base + prod overlay for the K8s manifests) — this is the path for an
  actual production deployment with autoscaling, not a quick demo.

### Domain DNS — the actual records you need, regardless of host

| Record | Purpose |
|---|---|
| `A` (or `CNAME` if your host gives you a hostname instead of an IP) | Points your domain at the server/service |
| `CNAME` `www` → your root domain | So `www.yourdomain.com` also works |
| `MX` (optional) | Only needed if you want `hello@yourdomain.com`-style email — most registrars/Cloudflare offer free email forwarding without a full mail server |

If you want a CDN/DDoS layer in front (recommended for anything public):
point your domain's nameservers at **Cloudflare** (free tier), then manage
the A/CNAME records inside Cloudflare's dashboard instead of your
registrar's — same records, extra layer of protection, plus free TLS.

### What's NOT yet built for a "real" hosted SaaS

Being direct about the gap: everything above deploys the *self-hosted*
product. A genuine multi-tenant SaaS (one deployment serving many separate
customers with billing) is not built — there's no tenant isolation, no
billing integration, no license-issuance backend (see `CLAUDE.md`'s "Free
vs Pro" section for the full detail on what's real vs. aspirational there).
The steps above get you *a* working deployment for *your own* team or a
public demo — not a commercial hosted offering.
