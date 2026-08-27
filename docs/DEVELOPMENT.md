# Developer guide

## Prerequisites

- Go 1.25+
- [`air`](https://github.com/air-verse/air) for hot-reload (installed automatically by `make watch`)

## Make targets

| Target | Description |
|---|---|
| `make watch` | Hot-reload dev server (rebuilds on code/template changes) |
| `make run` | Run without hot-reload |
| `make build` | Compile all packages |
| `make reseed` | Delete `passion.db` and re-initialize with seed data |

## Adding a feature

**Handler** — create a new file in [../http/server/](../http/server/) following the existing naming convention (e.g. `my_feature_handlers.go`), then register routes in [../http/server/core.go](../http/server/core.go).

**Template** — create a `.html` file in [../templates/](../templates/). Templates are compiled by [../pages/pages.go](../pages/pages.go) and available to handlers via the `Pages` struct.

**Database model** — add or modify structs in [../db/models.go](../db/models.go). GORM auto-migrates on startup; add explicit migrations in [../db/store.go](../db/store.go) if needed.

**YAML catalog** — extend [../db/yaml_import.go](../db/yaml_import.go) to parse new YAML structures and upsert them into the DB.

## HTMX patterns

- All interactive fragments use `hx-boost`, `hx-get`/`hx-post`, and `hx-swap`.
- Lazy-loaded divs inside forms **must** set `hx-target="this"` — HTMX inherits `hx-target` from ancestor elements, which causes requests to swap into the wrong container.
- Partial responses return named template fragments (e.g. `pages.MyFragment`), not full pages.

## Auth

- JWT tokens are stored in a cookie and validated by middleware in [../http/server/middleware.go](../http/server/middleware.go).
- `PASSION_DEV_AUTH_BYPASS=1` skips validation and auto-authenticates as user ID 1 — **never enable in production**.

## Database

- SQLite via GORM. The DB file is `passion.db` by default.
- `make reseed` is the fastest way to reset state during development. **It deletes
  `passion.db`** — point `DB_PATH` elsewhere if the current one holds real training data:
  `make reseed DB_PATH=/tmp/demo.db`.
- Seed data lives in [../db/seed.go](../db/seed.go); YAML import in [../db/yaml_import.go](../db/yaml_import.go).
- `SeedDevIfEmpty` only runs when the owner has **no session templates**. The YAML
  importer creates templates from `catalog/` on every boot, so on a database that has
  booted once the seed will never run again — reseed rather than expecting it to top up.

### What the seed covers

The fixtures aim to give every screen something to render, including states that only
appear when something unusual happened:

| Area | What you get |
|---|---|
| Templates | 3 built-in templates with labels, sources and `needs`, plus 3 activity templates |
| History | ~45 completed runs over 14 weeks, journals, ~165 climbing ticks |
| Cycle | A 4-week cycle mid-flight, with goals, weekday mappings and future scheduled sessions |
| Run states | Running, completed, finished-early with skipped steps, open session, manual draft, manual saved |
| Climbing | 4 venues, 3 boards, board context on a run, per-set planned targets and logs |
| Calendar | Trip, injury, deload and competition events |
| Awkward content | Very long names, 7-label rows, missing sources, and a template with no activities |

That last row is deliberate: every mobile overflow bug found so far only reproduced with
content that long. Keep it when adding fixtures.
