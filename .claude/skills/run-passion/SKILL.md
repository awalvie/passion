---
name: run-passion
description: Build, run, and drive the Passion climbing training app. Use when asked to start Passion, run it, take a screenshot, interact with the running app, or verify a UI change.
---

Passion is a Go + HTMX + SQLite web app served at `:3001`. Drive it by starting the Go server then running a Playwright Node script from the isolated sandbox at `/tmp/pw-sandbox/`.

All paths below are relative to the repo root (`passion/`).

## Prerequisites

No system packages needed. Go is already available. Playwright runs from an isolated sandbox:

```bash
mkdir -p /tmp/pw-sandbox
cd /tmp/pw-sandbox && npm init -y && npm install playwright
```

The Playwright Chromium browser installs to `~/.cache/ms-playwright/` on first use — it's already downloaded if this has been run before.

## Build

```bash
go build ./...
```

## Run (agent path)

**Start the server** (seeded data + auth bypass so no login needed):

```bash
PASSION_SEED=1 PASSION_DEV_AUTH_BYPASS=1 PASSION_ADDR=:3001 go run ./cmd/passion &
echo $! > /tmp/passion.pid
timeout 15 bash -c 'until curl -sf http://localhost:3001/ >/dev/null 2>&1; do sleep 0.5; done'
```

**Drive with Playwright** — write a script and run it from the sandbox:

```bash
mkdir -p /tmp/shots
cd /tmp/pw-sandbox && node -e "
const { chromium } = require('playwright');
(async () => {
  const browser = await chromium.launch({ args: ['--no-sandbox'] });
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });

  await page.goto('http://localhost:3001/dashboard');
  await page.waitForLoadState('networkidle');
  await page.screenshot({ path: '/tmp/shots/dashboard.png' });

  await browser.close();
})();
"
```

Screenshots land in `/tmp/shots/`.

**Stop the server:**

```bash
kill $(cat /tmp/passion.pid) 2>/dev/null; rm -f /tmp/passion.pid
```

Or if the pid file is gone: `pkill -f 'go run ./cmd/passion'`

## Run (human path)

```bash
make watch   # hot-reload dev server at :3000, opens at http://localhost:3000
```

Sign up at `/signup`, or skip auth entirely with `PASSION_DEV_AUTH_BYPASS=1 PASSION_SEED=1 make run`.

## Key routes

| Route | What it is |
|---|---|
| `/dashboard` | Weekly session agenda + month calendar |
| `/history` | Training log with heatmap and streaks |
| `/templates` | Session template list |
| `/activity-templates` | Activity template list |
| `/exercise-library` | Exercise library table |
| `/training-cycles` | Training cycles (**not** `/cycles`, which 404s) |
| `/training-cycles/{id}` | One cycle: goals, week grid, notes |
| `/training-cycles/{id}/targets` | Per-cycle exercise targets |
| `/training-cycles/new/guided` | The only cycle builder; `/training-cycles/new` redirects here |
| `/calendar` | Full calendar view |
| `/training-log` | Manual log entries |

## Test

```bash
go test ./...
```

## Gotchas

- **`chromium-cli` is not installed** — `npx playwright` is available, and the Playwright Chromium browser downloads to `~/.cache/ms-playwright/`. Use the Node script pattern above, not `chromium-cli`.
- **Sandbox isolation** — install `playwright` into `/tmp/pw-sandbox/` (not the repo, not system npm). Run `node` from that directory so `require('playwright')` resolves.
- **Port conflict** — if `:3001` is already bound from a previous run, `kill $(cat /tmp/passion.pid)` or `pkill -f passion` before relaunching. The server exits cleanly on SIGTERM.
- **Route is `/exercise-library` not `/exercises`** — the nav label says "Library" but the path is `/exercise-library`.
- **Auth bypass requires an existing user** — `PASSION_SEED=1` creates demo data and a user on an empty DB. With `PASSION_DEV_AUTH_BYPASS=1` the server auto-logs in as user 1. Both flags are needed together on a fresh DB.

## Troubleshooting

- **`Cannot find module 'playwright'`**: you ran `node` from the wrong directory. `cd /tmp/pw-sandbox` first, then run the script.
- **Server not ready after timeout**: check if the DB is locked by a previous process — `fuser passion.db` and kill the holder.
- **Blank screenshot / 404**: double-check the route path against the Key routes table above.
