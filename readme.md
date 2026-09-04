<p align="center">
  <strong>Passion</strong><br>
  <sub>A structured climbing training app built with Go, HTMX, and SQLite.</sub>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.25">
  <img src="https://img.shields.io/badge/HTMX-1.9-3366CC?style=flat-square" alt="HTMX">
  <img src="https://img.shields.io/badge/SQLite-003B57?style=flat-square&logo=sqlite&logoColor=white" alt="SQLite">
  <img src="https://img.shields.io/badge/Tailwind_CSS-06B6D4?style=flat-square&logo=tailwindcss&logoColor=white" alt="Tailwind CSS">
</p>

---

<!-- SCREENSHOT: dashboard with a week of scheduled sessions and the month calendar visible -->
<!-- Replace the placeholder below with your actual screenshot -->
![Dashboard](docs/screenshots/dashboard.png)

---

## What it does

Passion helps you plan, schedule, and track climbing training sessions. Build session templates from a library of exercises, organize them into multi-week training cycles, and follow guided workout runs with built-in timers.

**Core features:**

- Session templates with warmup / activity / cooldown structure
- Exercise library with Markdown notes, sets/reps/timers, and embedded video
- Training cycles that auto-generate weekly schedules
- Guided run player with countdown timers, rep tracking, completion logging, and climbing tick logging (grade chip strip, outcome quick-actions, live session header)
- Training history with heatmaps, streak tracking, per-template breakdowns, and climbing analytics (grade pyramids, hardest-sent, send rate)
- Weekly + monthly calendar dashboard
- YAML-driven exercise and template catalog for version-controlled training plans
- Multi-user auth (JWT) with in-app password change, dark/light theme, fully server-rendered with HTMX

---

## Screenshots

<!-- Take these screenshots and drop them into docs/screenshots/ -->

### Dashboard

![Dashboard](docs/screenshots/dashboard.png)

### Guided Run

![Run](docs/screenshots/run.png)

### History

![History](docs/screenshots/history.png)

### Template Editor

![Template](docs/screenshots/template-edit.png)

### Exercise Library

![Library](docs/screenshots/exercise-library.png)

<!-- SCREENSHOTS TO TAKE:
  1. docs/screenshots/dashboard.png    — Dashboard with a populated week (sessions scheduled, month calendar showing colored dots)
  2. docs/screenshots/run.png          — Guided run mid-workout (timer counting, exercise card visible, playlist sidebar open)
  3. docs/screenshots/history.png      — History page with some completed workouts (heatmap, weekly chart, table with data)
  4. docs/screenshots/template-edit.png — Template editor with a few activities expanded showing exercises
  5. docs/screenshots/exercise-library.png — Exercise library grid with a few entries
  Use dark mode for all screenshots for the best contrast on GitHub's dark default.
  Resize your browser to ~1200px wide for consistent framing.
-->

---

## Quick start

```sh
# Clone and run
git clone https://github.com/<you>/passion.git
cd passion
make watch        # hot-reload dev server at :3000
```

Sign up at `/signup`. For faster local dev, skip auth entirely and load demo data:

```sh
PASSION_SEED=1 PASSION_DEV_AUTH_BYPASS=1 make run
```

---

## Configuration

Copy [`passion.example.yaml`](passion.example.yaml) to `passion.yaml` or configure via environment variables. Env vars always override the YAML file.

| Variable | Default | Description |
|---|---|---|
| `PASSION_ADDR` | `:3000` | Listen address |
| `PASSION_DB_PATH` | `passion.db` | SQLite database path |
| `PASSION_JWT_SECRET` | `change-me-in-production` | JWT signing secret — **change this in production** |
| `PASSION_JWT_TTL_HOURS` | `168` | Token lifetime in hours (7 days) |
| `PASSION_SEED` | off | Populate demo data on empty DB |
| `PASSION_DEV_AUTH_BYPASS` | off | Auto-login as user 1 (dev only) |
| `PASSION_INSECURE_COOKIES` | off | Allow the session cookie over plain HTTP (dev only) |
| `PASSION_YAML_IMPORT_ENABLED` | off | Import catalog YAML at startup |
| `PASSION_YAML_EXERCISES_DIR` | `catalog/exercises` | Exercise YAML directory |
| `PASSION_YAML_SESSION_TEMPLATES_DIR` | `catalog/session_templates` | Session template YAML directory |
| `PASSION_YAML_ACTIVITY_TEMPLATES_DIR` | `catalog/activity_templates` | Activity template YAML directory |
| `PASSION_YAML_IMPORT_OWNER_ID` | `1` | Owner ID for imported records |

Pass `1`, `true`, `yes`, or `on` for boolean flags.

The server refuses to start on an unsafe `JWTSecret`. It rejects the example value
`change-me-in-production`, the deploy template's `__JWT_SECRET__` placeholder, and anything
shorter than 32 characters. Generate one with `openssl rand -base64 48`. Dev auth bypass
skips the check, because it short-circuits token verification anyway. `DemoOwnerID` and
`YAMLImport.OwnerID` must not be `0` — that id is the "no owner" sentinel across the data
layer.

---

## Administration

These flags run against the database and exit. They do not start the server.

| Command | What it does |
|---|---|
| `--mint-invites=N` | Print N new signup invite codes. Add `--invite-note="for X"` to record who each is for. |
| `--list-invites` | List every code and whether it has been used. |
| `--purge-orphans-dry-run` | Count exercises orphaned by the old importer bug. Changes nothing. |
| `--purge-orphans` | Delete those orphans, keeping any that run history still references. |
| `--delete-users-except=ID` | Show what deleting every other account would remove. Changes nothing on its own. |
| `--i-have-a-backup` | Required with `--delete-users-except` to actually delete. There is no undo. |
| `--backfill-runs-dry-run` / `--backfill-runs` | Give past runs their own copy of the exercises their records point at. |
| `--backfill-slugs-dry-run` / `--backfill-slugs` | Derive a slug for every catalog row that has none. Run before the importer matches on slug. |
| `--publish-catalog-dry-run` / `--publish-catalog=ID` | Flag that owner's importer-created rows as the shared catalog every account reads. |
| `--unpublish-catalog=ID` | Take those rows back into private ownership. Reverses `--publish-catalog` exactly. |

Stop the server before running any of these. Two writers on one SQLite file give
`database is locked`. After a purge or a deletion, run `VACUUM` to reclaim the space — the
rows go immediately but the file does not shrink on its own.

### Invite-only signup

Signup requires a valid invite code. The one exception is the very first account on a fresh
install, so a new self-hosted instance can create its owner without one. After that every
signup needs a code:

```sh
./passion --mint-invites=3 --invite-note="climbing club"
```

Codes look like `K7PM-3XQD-9RTB`. Case and dashes do not matter when one is typed in. A
code works once.

---

## The shared catalog

The catalog belongs to the app, not to a user. One account owns the rows and they are
flagged `shared`, so every other account reads them and nobody edits them in place. Saving
your own version copies that one row to you, and from then on it is an ordinary row you own.

A new account gets nothing copied to it. It reads the catalog on first login because the
catalog was always there.

See [docs/SHIP_1_RUNBOOK.md](docs/SHIP_1_RUNBOOK.md) for switching this on.

## YAML catalog

Training plans live in version-controlled YAML files under `catalog/`. On startup (when
import is enabled), Passion upserts exercises and templates **by slug**, once, into the
account that holds the catalog.

Every entry needs an explicit `slug:`. It is the row's identity, so the display `name:` is
free to change without the row being deleted and recreated. A missing slug is a hard error
rather than something derived from the name — deriving it would mean the first rename
silently produced a new slug, deleted the row and created another. `ref:` names its target
by slug for the same reason.

Each directory is scanned recursively, so files can be grouped into subfolders however you like — `catalog/exercises/` uses one folder per program (`ondra/`, `emil/`, `bechtel/`, `nelson/`) with unsourced and one-off entries at the top level. Layout is purely for humans: an entry's identity is its `name`, not its path, so moving a file between folders changes nothing in the database.

Each of the three settings takes one directory or a list of them, so the catalog can span
several trees and `ref:` resolves across all of them. Content from paid programmes is not
ours to redistribute, so it lives in a separate private repository and is shipped to the
server alongside this tree (see `.github/workflows/deploy.yml`). A name defined in two
trees is a hard error rather than a silent shadow.

### Editing a catalog item in the app

The importer overwrites the row it matches by name, and for blocks and sessions it
replaces the child rows outright. So editing a catalog item in the UI would be undone on
the next restart, and renaming one would delete it.

Editing one now stamps `CatalogEditedAt` on it. From that point the importer skips the row
completely and never prunes it, the lists show an **Edited** chip, and the edit page offers
**Reset to catalog** — which clears the stamp and re-imports, restoring the original.

An edited row stops receiving catalog fixes. That is the trade, and the chip is there to
say so.

Two things this deliberately does not cover. Per-cycle numbers already have their own
place — `CycleExerciseOverride` and `CycleExerciseWeekOverride` hold your sets, reps,
weight and rep seconds for a cycle or a single week, and the importer never touches them,
so changing your numbers needs no stamp at all. And **deleting** a catalog item leaves no
row to carry a stamp, so the next import recreates it.

<details>
<summary>Exercise example</summary>

```yaml
# catalog/exercises/pullups.yaml
name: "Weighted Pull-ups"
source: "Power Company Climbing"   # optional: program or coach it comes from
label: "strength, pulling"          # optional: comma-separated tags shown as chips
kind: "reps_and_sets"
sets: 5
reps: 5
rep_seconds: 5
set_rest_seconds: 120
weight_kg: 10
notes: "Controlled tempo"
```

</details>

<details>
<summary>Session template example</summary>

```yaml
# catalog/session_templates/strength_base.yaml
name: "Strength Base Session"
source: "Power Company Climbing"   # optional: program or coach it comes from
label: "strength, bouldering"       # optional: comma-separated tags shown as chips
color: "#ef4444"
activities:
  - type: "warmup"
    exercises:
      - name: "Row + Band Prep"
        sets: 2
        reps: 12
        rep_seconds: 4
        rep_rest_seconds: 30
        set_rest_seconds: 30
  - type: "activity"
    exercises:
      - ref: "Weighted Pull-ups"    # references a library exercise by name
      - name: "Ring Rows"
        sets: 4
        reps: 10
        rep_seconds: 4
        set_rest_seconds: 90
```

</details>

Import behavior:

- Runs on startup only when `PASSION_YAML_IMPORT_ENABLED` is set
- Upserts by `owner_id + slug` — safe to re-run
- Template updates replace activities/exercises to preserve ordering
- Skips any row a user has edited in the app (`catalog_edited_at` set) — neither
  overwritten nor pruned
- Prunes catalog rows that dropped out of the YAML (e.g. after a rename): only rows the
  importer created are removed, the system open-session template is left alone, and a
  session template is never deleted while it still has scheduled sessions or cycle
  mappings (so logged runs are never orphaned)
- Unknown `ref` or invalid YAML fails startup fast

---

## Project structure

```
cmd/passion/           Entry point — config loading, DB init, server startup
config/                12-factor config (YAML + env vars)
db/                    GORM models, SQLite store, seed data, YAML importer
http/server/           Chi router, all HTTP handlers, middleware
pages/                 Compiles and renders all Go HTML templates
templates/             Go HTML templates — pages, fragments, layouts
static/                CSS, JS (HTMX, Tailwind, Lucide), icons
catalog/               YAML exercise and template definitions
docs/                  Screenshots and documentation
scripts/               Utility scripts
```

---

## Developer guide

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

---

## License

Personal project. Not currently licensed for redistribution.
