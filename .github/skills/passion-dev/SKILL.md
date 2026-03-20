---
name: passion-dev
description: 'Passion climbing training app development. Use for: adding handlers, modifying DB models, creating templates, understanding project architecture, running/testing the app, YAML import features.'
---

# Passion Development

Passion is a climbing training web application built with Go, HTMX, and Tailwind CSS. It manages session templates, training cycles, exercise libraries, and guided workout runs.

## Architecture

- **Go 1.25** with `chi` router, `gorm` ORM (SQLite), `goldmark` markdown, `golang-jwt` auth
- **Server-side rendered** HTML templates with HTMX for interactivity
- **Single binary** with embedded static assets

## Project Structure

```
cmd/passion/main.go       – Entry point, config loading, DB init, graceful shutdown
config/config.go          – 12-factor config: YAML file + env overrides
db/models.go              – GORM model definitions (all entities)
db/store.go               – Store struct, AutoMigrate
db/seed.go                – Dev seed data
db/yaml_import.go         – Startup YAML catalog import (exercises + templates)
http/server/core.go       – Server struct, template parsing, routing, middleware
http/server/auth.go       – JWT auth, login/signup handlers
http/server/dashboard.go  – Dashboard handler (week view, scheduled sessions)
http/server/templates.go  – Session template CRUD, activity/exercise management
http/server/exercise_library.go – Library exercise CRUD
http/server/training_cycles.go  – Training cycle CRUD + session generation
http/server/runs.go       – Guided run player (complete/skip exercises)
http/server/form_helpers.go     – Shared form parsing (formInt, formFloat)
http/server/errors.go     – Centralized error response helper
http/server/template_color.go  – Color normalization for template cards
http/server/time_helpers.go    – Date/time utilities
http/server/youtube_embed.go   – YouTube URL → embed conversion
http/server/markdown.go        – Markdown preview endpoint
templates/                     – Go HTML templates (pages, fragments, layouts)
static/passion.css             – Custom CSS (Tailwind loaded via CDN)
catalog/                       – YAML exercise/template definitions for import
```

## Key Conventions

### Handler Pattern

All HTTP handlers are methods on `*Server`. Pattern:

```go
func (s *Server) handleFoo(w http.ResponseWriter, r *http.Request) {
    ownerID, ok := s.currentUserID(r)
    if !ok {
        s.unauthorizedRedirect(w, r)
        return
    }
    // ... business logic with s.store.DB ...
    // Full page: s.renderPage(w, PageData{...})
    // HTMX fragment: s.renderFragment(w, "fragment_name", data)
}
```

### HTMX Redirects

After successful POST mutations, set `HX-Redirect` header + `w.WriteHeader(http.StatusOK)`:

```go
w.Header().Set("HX-Redirect", "/target-path")
w.WriteHeader(http.StatusOK)
```

### Error Responses

- **Client errors (4xx):** `http.Error(w, "human message", http.StatusBadRequest)`
- **Server errors (5xx):** `s.serverError(w, r, err)` — logs internally, returns generic message
- Never expose GORM/internal error messages to the client

### Form Parsing

Use shared helpers from `form_helpers.go`:

```go
sets := formInt(r, "sets")
weight := formFloat(r, "weight_kg")
```

### Database Models

- All entities have `OwnerID uint` for multi-user scoping
- Always filter queries by `owner_id` for authorization
- Use `gorm.Model` (provides ID, CreatedAt, UpdatedAt, DeletedAt)
- Exercise kinds: `"reps_and_sets"` (default), `"session"`, `"exercise_catalog"`
- Media is stored in `ExerciseMedia` rows (polymorphic: ExerciseID or LibraryExerciseID)

### Templates (HTML)

- Layouts: `templates/layouts/base.html` wraps all pages
- Fragments: `templates/fragments/` for HTMX partial responses
- Use `{{ .Field }}` Go template syntax
- HTMX attributes: `hx-post`, `hx-target`, `hx-swap`

### Config

Config loads from YAML then env vars override (12-factor). Key env vars:

- `PASSION_ADDR` – listen address
- `PASSION_DB_PATH` – SQLite file path
- `PASSION_SEED` – enable dev seeding
- `PASSION_JWT_SECRET` – JWT signing key
- `PASSION_DEV_AUTH_BYPASS` – skip auth in development

## Build & Run

```sh
make run              # go run ./cmd/passion
make build            # go build ./...
make watch            # hot-reload with air
go test ./...         # run all tests
go vet ./...          # static analysis
```

## Testing

- Tests use standard `testing` package
- Config tests use `t.TempDir()` + `t.Setenv()` for isolation
- HTTP handler tests use `httptest.NewRecorder()` + `httptest.NewRequest()`
- DB tests create in-memory SQLite via `NewSqlite(":memory:")`
