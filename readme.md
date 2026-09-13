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

The importer refuses to run, rather than warn, on two conditions:

- **`YAMLImport.OwnerID` names no existing account.** The import writes a whole catalog
  under that id and every catalog read is owner-scoped, so a stale id produces rows nobody
  can reach — and the next run's prune treats them as orphaned. A config left pointing at a
  deleted account did exactly that in production; see [docs/RECOVERY.md](docs/RECOVERY.md).
- **That owner has importer-created catalog rows with no slug.** Rows are matched by slug,
  so an unslugged row matches no YAML entry: the import would create a second copy of
  everything and prune the originals. Run `--backfill-slugs-dry-run`, read it, then
  `--backfill-slugs`.

The second one bites on any database predating the slug switch, local ones included. If
`make watch` exits with a "no slug" error, that is this check — run the backfill once.

---

## Administration

These flags run against the database and exit. They do not start the server.

| Command | What it does |
|---|---|
| `--mint-invites=N` | Print N new signup invite codes. Add `--invite-note="for X"` to record who each is for. |
| `--list-invites` | List every code and whether it has been used. |
| `--purge-orphans-dry-run` | Count exercises orphaned by the old importer bug. Changes nothing. |
| `--purge-orphans` | Delete those orphans, keeping any that run history still references. |
| `--delete-users-except=ID` | Show what deleting every other account would remove. Changes nothing on its own. Refuses if a victim owns catalog rows, or is the configured import owner. |
| `--i-have-a-backup` | Required with `--delete-users-except` or `--purge-ghost-catalog` to actually delete. There is no undo but the backup file. |
| `--purge-ghost-catalog=ID` | Show what removing every row owned by that id would take. Only works when no account has the id, and refuses if anything still points at the rows. |
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

Training content lives in version-controlled YAML files under `catalog/`, in the format
[docs/CATALOG_FORMAT.md](docs/CATALOG_FORMAT.md) specifies. Three directories, one row per
file:

```
catalog/movements/    one exercise per file
catalog/blocks/       one block per file. A menu is written inside the block that holds it
catalog/sessions/     one session per file
```

**Every file carries an `id:`,** and the app matches a file to its row on that id. So a file
can be renamed or moved and it still owns the same row. You never type the id: booting the
app writes one into every file that has none, and so does `make catalog-ids`.

A reference is written bare to name something in the same tree, and `app:<slug>` to name
something the app ships. There is no fallback between the two, so adding a file can never
change what an existing reference means.

Check a tree without a database or a server:

```
make catalog-lint
```

Content from paid programmes is not ours to redistribute, so it lives in a separate private
repository and is shipped to the server alongside this tree (see
`.github/workflows/deploy.yml`). Configure it as:

```yaml
catalog:
  import: true
  private:
    - dir: /opt/passion/catalog-private
      owner: you@example.com
```

`owner:` is an email, not an id. A tree whose owner has no account is skipped with a warning
and imports on a later boot.

### Editing a catalog item in the app

Three cases, three answers. None of them makes a hidden copy.

**Your own numbers on a movement the app ships.** Saved against your account and applied
every time you run it. No new row, and the app's improvements still reach you.

**A row of yours that came from a file.** Edited in place: same row, same id. The row is
detached from its file for good, and every later import names that file in its report and
badges the row in the library. Without that, you edit the YAML, restart, see no change, and
are told nothing.

**The structure of something the app ships.** An explicit "copy to my content" action, never
an implicit one. The copy gets a new id and keeps the family of the row it came from, so one
progression chart still covers both.

<details>
<summary>Movement example</summary>

```yaml
# catalog/movements/weighted_pull_ups.yaml
id: 3f2a91c4-7b6e-4d15-9a03-1e8c5f2d4b7a
name: Weighted Pull-ups
style: reps_and_sets
source: Power Company Climbing   # optional: the programme or coach the method is known by
tags: [strength, pulling]
sets: 5
reps: 5
rep_seconds: 5
set_rest_seconds: 120
weight_kg: 10
notes: Controlled tempo
```

</details>

<details>
<summary>Block example, with a menu</summary>

```yaml
# catalog/blocks/fingers.yaml
id: a97066ed-016b-45a2-b767-303a1d04a71d
name: Fingers
role: main
items:
  - movement: app:half_crimp_hang
    sets: 4
  - menu:
      name: Pick a hang
      pick: 1
      of: [half_crimp_hang, hangboard_ladder]
```

`pick: 0` means the menu may be skipped. That is where optionality lives, so no display name
ever has to end in "(optional)".

</details>

<details>
<summary>Session example</summary>

```yaml
# catalog/sessions/finger_day.yaml
id: c81d4f70-2f6a-4b8e-9d1c-0a4e7b3f5c21
name: Finger Day
color: "#ef4444"
needs: hangboard
items:
  - block: warm_up
  - block: fingers
```

</details>

Anything the format cannot express fails at boot, naming the file: an unknown key, a
reference to nothing, a tag that is not a slug, a file with no id, or two files
holding one id.

---

## Project structure

```
main.go                V2 entry point — holds the embeds, since a pattern cannot use ".."
store/                 V2 data layer — models, goose migrations, one store per area
web/                   V2 handlers, middleware and template rendering
cmd/passion/           V1 entry point — config loading, DB init, server startup
cmd/genmigrations/     Writes both dialects' migrations from docs/SCHEMA_V2.sql
cmd/convertcatalog/    One-time: rewrites a catalog tree into the V2 format
config/                12-factor config (YAML + env vars)
db/                    V1 GORM models, SQLite store, seed data, YAML importer
http/server/           V1 Chi router, all HTTP handlers, middleware
pages/                 V1 template compiling and rendering
templates/             Go HTML templates — pages, fragments, layouts
static/                CSS, JS (HTMX, Tailwind, Lucide), icons
catalog/               YAML movements, blocks and sessions — one row per file
docs/                  Screenshots, design documents, the canonical schema
scripts/               Utility scripts
```

---

## Developer guide

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

---

## License

Personal project. Not currently licensed for redistribution.
