---
name: run-forgeguardian
description: Build, launch, and drive the ForgeGuardian API + dashboard for manual testing — start the Go API server and Vite dev server, log in, navigate pages, click buttons, take screenshots. Use when asked to "run ForgeGuardian", "start the dashboard", "test the app", "screenshot a page", or "check if X works in the browser".
---

ForgeGuardian is a Go API server (`internal/api`) paired with a React/Vite
dashboard (`dashboard/`). Both run locally on fixed ports; the dashboard's
dev server proxies `/api/` to the Go server. Driving it means launching
both and controlling a headless Chromium via the Playwright driver in this
skill directory — not `npm start` and waiting for a human to look at a
window.

Paths below are relative to the repo root (`<unit>/`), same level as
`go.mod`.

## Prerequisites

Playwright is **not** in `dashboard/node_modules` — install it separately
(this project doesn't ship a browser-automation dependency):

```bash
cd /tmp && npm init -y >/dev/null 2>&1 && npm install playwright
```

Run the driver with `NODE_PATH=/tmp/node_modules` so it resolves from
there (see Run section) — do not `npm install playwright` inside
`dashboard/`, it doesn't belong in that package's dependencies.

## Build

```bash
go build ./...              # Go backend + all cmd/ binaries
cd dashboard && npm run build   # dashboard production bundle → dashboard/dist/
```

Both must be clean before considering any change done.

## Run (agent path)

1. **Free the ports** (idempotent — safe even if nothing is listening):
   ```bash
   lsof -ti:3000 -sTCP:LISTEN | xargs -r kill
   lsof -ti:8095 -sTCP:LISTEN | xargs -r kill
   ```

2. **Start the API** on :8095. No database is required — it starts fine
   without one; every DB-backed endpoint (dashboard stats, packages,
   inventory, etc.) just returns 503 instead of crashing:
   ```bash
   FG_SESSION_SECRET=x nohup env PORT=8095 go run ./internal/api > /tmp/fg-api.log 2>&1 &
   disown
   sleep 3 && cat /tmp/fg-api.log   # confirm "api server listening" with no fatal error
   ```
   `FG_SESSION_SECRET` is required only if `FG_ADMIN_EMAIL`/`FG_ADMIN_PASSWORD`
   are also set (enables login) — the server refuses to start with weak/no
   session signing in that case. Leave all three unset for the default
   open-access dev mode (no login screen, dashboard loads directly).

   To test the authenticated flow (login screen, onboarding wizard,
   sidebar logout button), set real values instead:
   ```bash
   FG_ADMIN_EMAIL=admin@test.com FG_ADMIN_PASSWORD=testpass123 FG_SESSION_SECRET=x \
     nohup env PORT=8095 go run ./internal/api > /tmp/fg-api.log 2>&1 &
   disown
   ```

3. **Start the dashboard dev server** on :3000 (proxies `/api/` to :8095
   per `dashboard/.env.local`'s `VITE_API_URL`):
   ```bash
   cd dashboard && nohup npm run dev > /tmp/fg-dev.log 2>&1 &
   disown
   sleep 2 && cat /tmp/fg-dev.log   # confirm "ready in" — no port-conflict error
   cd ..
   ```

4. **Drive it** with the Playwright driver in this skill directory:
   ```bash
   NODE_PATH=/tmp/node_modules node .claude/skills/run-forgeguardian/driver.mjs nav /
   NODE_PATH=/tmp/node_modules node .claude/skills/run-forgeguardian/driver.mjs nav / --login
   NODE_PATH=/tmp/node_modules node .claude/skills/run-forgeguardian/driver.mjs click "Scan Now" / --login
   NODE_PATH=/tmp/node_modules node .claude/skills/run-forgeguardian/driver.mjs nav /welcome --mobile
   ```
   Each invocation launches a fresh browser, navigates, waits for
   network-idle, screenshots to `/tmp/fg-driver-shot.png`, and prints any
   `pageerror`/console-error lines. `--login` fills and submits the login
   form (`admin@test.com` / `testpass123` — only works if the API was
   started with those `FG_ADMIN_*` env vars) then skips the onboarding
   wizard via `localStorage.setItem('fg_onboarded','true')` before
   navigating. `--mobile` / `--tablet` set the viewport to 390×844 /
   768×1024 (default 1440×950).

   Full command reference is in the comment header of `driver.mjs` — read
   it before extending the driver rather than guessing at its args.

5. **Stop** when done:
   ```bash
   lsof -ti:3000 -sTCP:LISTEN | xargs -r kill
   lsof -ti:8095 -sTCP:LISTEN | xargs -r kill
   ```

## Run (human path)

`go run ./internal/api` in one terminal, `npm run dev` in `dashboard/` in
another, open `http://localhost:3000` in a real browser. Same ports, same
env vars as above. Useless in a headless container — use the driver path.

## Gotchas

- **Env vars leak across tool calls in some harnesses.** If you see
  `"dashboard login enabled"` in the API log when you expected open-access
  mode (or vice versa), a previous command in the same session may have
  exported `FG_ADMIN_EMAIL`/`FG_ADMIN_PASSWORD` and it's still set. Check
  with `env | grep FG_` before assuming your unset env vars took effect —
  `unset` in one tool call doesn't reliably propagate to the next in every
  harness. Prefix the launch command with explicit values (or
  `env -u FG_ADMIN_EMAIL -u FG_ADMIN_PASSWORD ...`) rather than relying on
  ambient shell state.
- **`FG_SESSION_SECRET` is silently required, not optional, once auth env
  vars are set.** The API exits immediately with
  `"FG_ADMIN_EMAIL/FG_ADMIN_PASSWORD are set but FG_SESSION_SECRET is not"`
  if you set the admin credentials without it. Always pass all three
  together.
- **`vite.config.ts`'s proxy key must be `/api/` with the trailing slash**,
  not `/api` — a bare `/api` prefix-matches `/api-docs` too and proxies
  that client-side route to the Go backend, breaking the page. Already
  fixed in this repo; don't reintroduce it if touching `vite.config.ts`.
- **403/401 on `/scan/upload` or `/sbom/...` under auth mode** used to be
  a real bug (missing `credentials: 'include'` on two hand-rolled
  `fetch()` calls in `dashboard/src/lib/api.ts`) — already fixed. If you
  see it again, check those two functions haven't regressed.
- **No `chromium-cli` tool is available in this environment** — always
  drive via `driver.mjs` (or extend it) rather than looking for that
  binary.
- **`localStorage.setItem('fg_onboarded', 'true')` must run before the
  `page.reload()`**, not after — the onboarding gate is checked once on
  mount from `localStorage`, so navigating without reloading first still
  shows the wizard.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `Error: listen tcp :8095: bind: address already in use` | `lsof -ti:8095 -sTCP:LISTEN \| xargs -r kill`, then relaunch. |
| Driver throws `Cannot find package 'playwright'` | You forgot `NODE_PATH=/tmp/node_modules` on the `node driver.mjs` invocation, or never ran the `npm install playwright` step in Prerequisites. |
| Every dashboard page shows only zeros / "No data yet" | Expected with no `DATABASE_URL` — this is not a bug, it's the documented no-DB degrade mode. Confirmed via `/tmp/fg-api.log` showing `"DATABASE_URL not set — DB-backed endpoints will return 503"`. |
| `driver.mjs nav /some-path --login` lands back on the login screen | The API wasn't started with `FG_ADMIN_EMAIL`/`FG_ADMIN_PASSWORD` set, so there's no account to log into — `--login` silently no-ops when the email input isn't present, and the page you actually asked for never loads because the gate never lifts. Relaunch the API with those two vars (+ `FG_SESSION_SECRET`). |
