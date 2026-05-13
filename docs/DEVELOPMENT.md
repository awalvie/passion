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
- `make reseed` is the fastest way to reset state during development.
- Seed data lives in [../db/seed.go](../db/seed.go); YAML import in [../db/yaml_import.go](../db/yaml_import.go).
