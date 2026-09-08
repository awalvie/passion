# Repository structure — review of the proposal

Written 7 September 2026. Reviews [STRUCTURE_PROPOSAL.md](STRUCTURE_PROPOSAL.md) against
[GO_STRUCTURE.md](GO_STRUCTURE.md), [SCHEMA_V2.sql](SCHEMA_V2.sql), and four Go apps read
for this review: **listmonk**, **tangled/core**, **wakapi** and **miniflux**.

Every project claim below comes from a clone read at a named commit, or from a page fetched
and quoted verbatim. Nothing here is from memory.

| Project read | Commit | Non-test Go LOC |
|---|---|---|
| [knadh/listmonk](https://github.com/knadh/listmonk) | `594b740` (4 Sep 2026) | 20,913 total |
| [tangled.org/core](https://tangled.org/tangled.org/core/tree/master) | `4c1c689` (1 Sep 2026) | 200,153 total |
| [muety/wakapi](https://github.com/muety/wakapi) | `9de2b03` (31 Aug 2026) | 29,553 total |
| [miniflux/v2](https://github.com/miniflux/v2) | shallow clone, 7 Sep 2026 | 44k (per GO_STRUCTURE §1.5) |

---

## 1. Verdict

**Buildable as-is on the package question, not buildable as-is on the file question.** The
three-package shape is right and is confirmed by three of the four apps read; do not add
`app/`, do not add `internal/`, do not split `store/`. But the tree has one defect that
stops the build outright — `//go:embed` cannot reach `../templates` from `web/`, so
templates, static and catalog must be embedded in `main.go` and passed down as `fs.FS`. And
it is silently underspecified in five places that will each cost a day: middleware, the
`session.go` name collision, the catalog importer, background jobs, and view models.

Fix those, keep the three packages, and ship it. Five of the six decisions are upheld.

---

## 2. The final tree

Three Go packages plus `main`, unchanged in count. `[NEW]`, `[CHANGED]` and `[SPLIT]` mark
every difference from the proposal.

```
passion/
├── main.go            flags, config, //go:embed templates static catalog, open store, migrate, start jobs, serve   [CHANGED]
├── config/            Config struct and loading from env, flags, file
├── store/             domain structs, all queries, migrations, catalog import, the dialect seam
│   ├── store.go         Open(), DSN switch, pool limits, WithTx, TranslateError
│   ├── models.go        the 19 table structs with GORM tags
│   ├── migrate.go       goose runner + the engine value divergent steps switch on            [CHANGED]
│   ├── migrations/      embedded numbered .sql, one sequence for both engines                 [NEW]
│   ├── content.go       catalog reads and writes, forking
│   ├── catalog.go       YAML structs and the shipped-content importer                         [NEW]
│   ├── log.go           runs and history
│   ├── plan.go          cycles, targets, scheduling
│   └── account.go       accounts, measurements, places
├── web/               one package, one file per screen area, no nesting
│   ├── server.go        Server struct, New(fs.FS, *store.Store, *config.Config), route table
│   ├── middleware.go    auth check, HTTP session/cookie, CSRF, request log, panic recover     [NEW]
│   ├── render.go        parse templates from fs.FS, funcmap, BaseParams, execute helpers, error→status
│   ├── auth.go          login, signup, logout
│   ├── dashboard.go     the landing page                                                      [NEW]
│   ├── library_exercise.go  library_activity.go  library_session.go                           [SPLIT]
│   ├── run.go           the open session and its fragments
│   ├── plan.go          cycles and targets
│   ├── calendar.go      scheduling and calendar events                                        [NEW]
│   ├── history.go       training log and summaries
│   └── account.go       profile, measurements, places
├── templates/         unchanged; embedded in main.go, injected as fs.FS                       [CHANGED]
├── static/            unchanged; embedded in main.go                                          [CHANGED]
└── catalog/           YAML; embedded in main.go, injected into store.ImportCatalog            [CHANGED]
```

Later, `api/` becomes a fourth package beside `web/` over the same store. See §4.6.

### Why each change

| Change | Reason | Source |
|---|---|---|
| `//go:embed` moves to `main.go` | **This is a build error, not a preference.** `go doc embed`, Go 1.26.3, verbatim: "The patterns are interpreted relative to the package directory containing the source file… Patterns may not contain '.' or '..' or empty path elements". So `//go:embed ../templates` from `web/` does not compile. The repo root is the only package that can see all three directories. | Go's own `embed` docs; [wakapi `main.go`](https://github.com/muety/wakapi/blob/master/main.go) carries `//go:embed static` and `//go:embed version.txt` at the root for exactly this reason |
| `web/middleware.go` added, `web/session.go` removed | `session.go` in a training app reads as the training-session feature, not HTTP sessions. That is a name collision on the single most-opened file in the package. Auth, cookie session, CSRF and recover are ~400 lines and belong in one file named for what they are. | [listmonk](https://github.com/knadh/listmonk/tree/master/internal/auth) keeps this in a separate 802-line `internal/auth` package; [wakapi](https://github.com/muety/wakapi/tree/master/middlewares) has an 8-file `middlewares/` package |
| `library.go` split three ways; `dashboard.go`, `calendar.go` added | The proposal lists 7 handler files for ~12,000 lines — an average of 1,700 lines per file. The largest file in the largest single-package comparable is [listmonk `cmd/init.go`](https://github.com/knadh/listmonk/blob/master/cmd/init.go) at 1,198 lines, and its largest *handler* file is `cmd/subscribers.go` at 906. Passion has 29 page templates and 20 fragments; 12-14 files puts each around 800-900 lines. Files are the cheap unit — splitting one is invisible to callers. | Google's guide, via GO_STRUCTURE §1.6: "it is usually not a good idea to have a single file with many thousands of lines in it" |
| `store/migrations/` + goose | The proposal's `migrate.go` doing "schema creation and versioned migrations" is a hand-rolled runner, which GO_STRUCTURE §2.8 already ruled against for good reasons. Keep the proposal's own research. | GO_STRUCTURE §2.8, §5.2 |
| `store/catalog.go` | The YAML importer is a multi-table transactional write, which decision 2 puts in `store/`. It also needs a **third family of structs** — the YAML shapes, which are not the table structs — and the proposal says nothing about where those live. Unexported, in this file. | Decision 2, applied consistently. Escape hatch: [listmonk `internal/subimporter`](https://github.com/knadh/listmonk/blob/master/internal/subimporter/importer.go) is 765 lines in its own package; take that only if ours passes ~400 |
| `config/` kept | Both comparables that separate config make it a package: [wakapi `config/`](https://github.com/muety/wakapi/tree/master/config) and [tangled `appview/config/`](https://tangled.org/tangled.org/core/tree/master/appview/config). If it lived in `main`, `web.New` could not name its type. | — |

**Do not add a `queries/` directory of `.sql` files.** listmonk has one because it uses
sqlx and has no ORM — its [`queries/`](https://github.com/knadh/listmonk/tree/master/queries)
is 10 files of named SQL bound to `*sqlx.Stmt` fields in
[`models/queries.go`](https://github.com/knadh/listmonk/blob/master/models/queries.go).
Under GORM, hand-written SQL is the **one thing that bypasses the dialect seam** —
`BindVarTo` and `QuoteTo` are only reached through GORM's builder (GO_STRUCTURE §2.3). A
`queries/` directory would move Passion's per-engine risk from zero to everywhere.

---

## 3. Handler and store scale — the evidence for one package each

Measured from the clones (non-test lines, top level of each package only):

| Package | Files | Non-test LOC | Packages | Shape |
|---|---|---|---|---|
| listmonk `cmd/` | 26 | **9,426** | **1** | `package main`; handlers, router, wiring, installer, all flat |
| miniflux `internal/ui` | 93 | 6,127 | 1 | one file per route |
| tangled `appview/state/` | 19 | 5,962 | 1 | one file per area, + sibling packages for big areas |
| wakapi `routes/` | 13 | 3,279 | 1 | one file per page, each with its own `RegisterRoutes` |
| stdlib `net/http` | 71 | 30,114 | 1 | GO_STRUCTURE §1.5 |
| tangled `appview/db/` | 40 | 12,313 | 1 | one file per entity |
| listmonk `internal/core` | 12 | 2,549 | 1 | one file per aggregate; **this is its store** |

**listmonk is the answer to question 3.** 9,426 lines of handlers, router, wiring and an
installer, in one package, which is also `package main`, maintained by one principal
developer. Passion at ~12,000 is the same order. One `web/` package holds.

The one project that split handlers into packages is tangled, and it did so at
**~18,000 lines of handler code across nine packages** (`state/` 5,962, `repo/` 5,822,
`pulls/` 3,849, `settings/` 1,148, `issues/` 1,065, and four smaller). That is a threshold
Passion is nowhere near.

**The trigger to revisit, stated so it is checkable:** when one *file* in `web/` passes
~1,200 lines, split the file. When one *area* passes ~4,000 lines, and only then, consider
a sibling package. Not before.

Same for `store/`. Ours will be ~2,500-3,500 lines. tangled splits `models/` (4,042) from
`db/` (12,313) — at 4x our size. listmonk and miniflux do not split at all.

---

## 4. The six decisions, ruled

### 4.1 Decision 1 — three packages, `store/` holding structs + queries + migrations + connection — **UPHELD**

`store/` is not overloaded. It will be around 3,000 lines, which is smaller than
[listmonk's `internal/core`](https://github.com/knadh/listmonk/tree/master/internal/core)
plus [`models/`](https://github.com/knadh/listmonk/tree/master/models) combined (3,819) and
a quarter of tangled's `db/` + `models/` (16,355).

The only split with real precedent is structs-out-of-store, and both projects that do it are
much larger: tangled `appview/models/` (4,042 lines, 44 files) and listmonk `models/` (1,270
lines). **The test for splitting is Google's: must a caller import both to use either?**
Here yes — `web` imports `store` to run queries and to name the types the queries return. So
they stay together.

The one refinement: **`store/migrations/` is a directory, not a package** — embedded `.sql`
plus a goose runner in `migrate.go`. That is not a fourth Go package.

I would change my mind if `api/` needed to name a domain type without querying. It will
query, so it never bites.

### 4.2 Decision 2 — no `app/` package; multi-table logic in `store/` as methods owning their transaction — **UPHELD, and GO_STRUCTURE §5.1 is overturned**

GO_STRUCTURE recommended `internal/app/` and flagged it as "the package most likely to earn
deletion" (§5.3 item 7). Delete it before it is written. The census at our scale:

| Project | Layer above the store? | Evidence |
|---|---|---|
| tangled `appview` (69k lines) | **No.** Handlers call `db/` directly | [`appview/state/`](https://tangled.org/tangled.org/core/tree/master/appview/state), `repo/`, `issues/` import `appview/db` and there is no `services/` |
| listmonk (20.9k) | **No.** `internal/core` *is* the store | [`core.go`](https://github.com/knadh/listmonk/blob/master/internal/core/core.go) holds `db *sqlx.DB` and its own doc says it "primarily provides data (DB / CRUD) operations to the app" |
| miniflux (44k) | **No.** `internal/ui` → `internal/storage` | GO_STRUCTURE §1.5 |
| wakapi (29.5k) | **Yes** — [`services/`](https://github.com/muety/wakapi/tree/master/services), 5,093 lines over [`repositories/`](https://github.com/muety/wakapi/tree/master/repositories), 1,928 | — |

Three of four have no such layer. And look at what the fourth paid for it:
[`services/services.go`](https://github.com/muety/wakapi/blob/master/services/services.go)
declares **nineteen producer-side interfaces named `IAggregationService`,
`IHeartbeatService`, `IUserService`…**, handlers hold them as fields
(`userSrvc services.IUserService` in
[`routes/summary.go`](https://github.com/muety/wakapi/blob/master/routes/summary.go)), and
the repo carries an 18-file `mocks/` directory to feed them. That is precisely the shape
Go's Code Review Comments calls "interfaces on the implementor side of an API 'for mocking'"
(GO_STRUCTURE §2.4). One layer bought thirteen interfaces and a mock package.

`store/` does not become a dumping ground, because of §5's rule.

### 4.3 Decision 3 — exported methods open the transaction, unexported helpers take `tx *gorm.DB` — **UPHELD, and strengthened from convention to compiler-enforced**

The proposal states the rule but leaves it as discipline: "unexported helpers take
`tx *gorm.DB` and never touch `s.db`". Nothing stops a helper being a method on `*Store`,
and then nothing stops it reaching `s.db`.

**Make the helpers package-level functions, not methods.** A function with no receiver has
no syntactic path to `s.db`. The trap becomes unrepresentable rather than merely discouraged.
This is what Ben Johnson actually does — `createDial(ctx, tx, dial)` and
`insertDialValue(ctx, tx, …)` are package-level funcs in
[wtf/sqlite](https://github.com/benbjohnson/wtf/blob/main/sqlite/dial.go) (GO_STRUCTURE
§2.5). It is one word of difference in the rule and it converts a code-review item into a
compiler guarantee.

Independent confirmation of the shape: tangled declares an
[`Execer` interface](https://tangled.org/tangled.org/core/tree/master/appview/db/db.go) with
the eight `Query`/`Exec`/`Prepare` methods common to `*sql.DB` and `*sql.Tx`, so its store
functions accept either handle. Same idea, plain-SQL spelling.

### 4.4 Decision 4 — `ctx context.Context` first on every store method — **UPHELD**

Settled by Go's own `context` docs, Code Review Comments, Google's style guide and
`go.dev/blog/context-and-structs`, all quoted in GO_STRUCTURE §2.6.

**But the proposal should stop claiming this is what comparable apps do, because listmonk is
a flat counter-example.** Measured: `internal/core` has **106 `*Core` methods and zero of
them take a context** — e.g.
[`func (c *Core) GetLists(typ, status string, getAll bool, permittedIDs []int) ([]models.List, error)`](https://github.com/knadh/listmonk/blob/master/internal/core/lists.go).
tangled takes `ctx`. So it is 50/50 among the closest comparables and the decision rests on
the language's own documentation, which is a stronger place to rest it anyway.

### 4.5 Decision 5 — SQLite DSN pragmas plus `SetMaxOpenConns(1)` — **UPHELD, one addition**

Independently confirmed by a project the prior research had not read.
[tangled `sqlite/sqlite.go`](https://tangled.org/tangled.org/core/tree/master/sqlite/sqlite.go)
is a 22-line package whose whole content is the DSN:

```go
var dsnOpts = []string{
	"_foreign_keys=1", "_journal_mode=WAL", "_synchronous=NORMAL",
	"_auto_vacuum=incremental", "_busy_timeout=5000", "_txlock=immediate",
}
```

Four of the proposal's five settings, reached independently, including `_txlock=immediate`.
**Add `_synchronous=NORMAL`** — it is the standard WAL companion and every project that
tunes SQLite sets it.

Two honest notes. tangled puts `_busy_timeout` **fifth**, after `_journal_mode`, which
contradicts GO_STRUCTURE §2.7's "order matters, busy_timeout first". I am not overturning
that finding — it has three sources — but the ordering is evidently survivable in practice.
And tangled sets **no pool cap at all**, so `SetMaxOpenConns(1)` remains the conservative
choice GO_STRUCTURE §5.3 item 3 says it is, not a consensus one.

**The best citation for the cap, though, is wakapi — because it is a GORM app that reached
the proposal's exact conclusion for the proposal's exact reason.** Verbatim from
[`config/config.go`](https://github.com/muety/wakapi/blob/master/config/config.go):

> `Log().Warn("with sqlite, only a single connection is supported")` `// otherwise 'PRAGMA foreign_keys=ON' would somehow have to be set for every connection in the pool`

One more thing to take from it: wakapi runs GORM on the **pure-Go** driver
(`github.com/glebarez/sqlite`, over `modernc.org/sqlite`) with `sqlite3` as its default
dialect, alongside `gorm.io/driver/postgres` and `gorm.io/driver/mysql`. That is
GO_STRUCTURE §5.3 item 5 — decide cgo or pure-Go *before* writing the DSN, because the
parameter names differ and a wrong name fails silently. wakapi decided pure-Go.

### 4.6 Decision 6 — a JSON API as a fourth package beside `web/` — **UPHELD, and it is proven shipping**

[wakapi](https://github.com/muety/wakapi) is the structure the proposal describes, already
running: [`routes/`](https://github.com/muety/wakapi/tree/master/routes) renders HTML (3,279
lines) and [`routes/api/`](https://github.com/muety/wakapi/tree/master/routes/api) serves
JSON (1,243 lines) as sibling packages over the same data layer. Go, GORM,
`html/template`, single binary, SQLite and PostgreSQL. That is the closest structural match
to Passion found anywhere in this review.

**The specific thing that goes wrong, with a named real-world example.** listmonk's data
layer returns HTTP errors. Verbatim from its own
[`internal/core/core.go`](https://github.com/knadh/listmonk/blob/master/internal/core/core.go)
package comment:

> All such methods return an echo.HTTPError{} (which implements error.error) that can be
> directly returned as a response to HTTP handlers without further processing.

And in the same file, verbatim:

> `ErrNotFound = echo.NewHTTPError(http.StatusNotFound, "not found")`

That is convenient for listmonk because both its HTML pages and its JSON API run on echo.
It is a trap for Passion, because a store error carrying a status code and a rendered
message has already decided how the *other* transport must answer.

**The one rule that prevents it:** *no signature in `store/` names an HTTP type, and no
error returned by `store/` carries a status code or a user-facing message.* Store returns
`gorm.ErrRecordNotFound`, `gorm.ErrDuplicatedKey` (via `TranslateError: true`) and its own
sentinel `var ErrNotYours = errors.New(...)` values. Mapping error to status and to prose is
one function in `web/render.go`, and later a second in `api/render.go`. It is checkable by
reading a signature.

There is a **second** failure mode, and it is the one I would actually bet on. See §7.

---

## 5. The view-model answer

This was the proposal's stated weak point. It is the best-evidenced answer in this review,
because all four projects were read specifically for it.

### What server-rendered Go apps actually do

| Project | Approach | Where | Count |
|---|---|---|---|
| **tangled** appview | typed `XxxParams` struct **+ a typed render method per page** | [`appview/pages/pages.go`](https://tangled.org/tangled.org/core/tree/master/appview/pages/pages.go), one file | **119 structs, 2,123 lines** |
| **wakapi** | typed `XxxViewModel` embedding a shared base and the domain types | [`models/view/`](https://github.com/muety/wakapi/tree/master/models/view), one file per page | 22 structs, 504 lines |
| **listmonk** | typed `xxxTpl` embedding a `publicTpl` base | in the handler file — [`cmd/public.go`](https://github.com/knadh/listmonk/blob/master/cmd/public.go), `cmd/auth.go` | 9 structs |
| **miniflux** | untyped `map[string]any`, chrome seeded in a constructor | [`internal/ui/view/view.go`](https://github.com/miniflux/v2/blob/main/internal/ui/view/view.go) | 0 structs |

**Three of four use typed structs, and the two closest to Passion in shape both do.** So the
answer to "does every page need one" is yes, and Passion's 81 structs are not a smell —
tangled has 119 at 5x the scale and keeps them all in one file.

This also **overturns GO_STRUCTURE §3.3 and §5.3 item 8**, which said "render domain structs
in templates, and do not build a view-model layer yet". That advice mistakes a params struct
for a DTO layer. It is not one. It is the template's argument list.

### The concrete answer

**1. Where they live.** In `web/`, in the same file as the handlers that render them. Not in
one central file. listmonk does exactly this — `unsubTpl`, `optinTpl`, `msgTpl`,
`subFormTpl` sit in `cmd/public.go` with the four handlers that use them, and
`loginTpl`, `forgotPasswordTpl`, `resetPasswordTpl`, `twofaTpl` sit in `cmd/auth.go`. It also
matches the proposal's own stated principle: "`web/run.go` says what is in it without being
opened". tangled centralises only because its `pages` package must *export* them across a
package boundary; `web/` is one package, so we get to co-locate.

**2. Who maps store types to them.** **Nobody. There is no mapping step.** Every typed
project embeds the store types directly. wakapi's
[`models/view/summary.go`](https://github.com/muety/wakapi/blob/master/models/view/summary.go),
verbatim:

```go
type SummaryViewModel struct {
	SharedLoggedInViewModel
	*models.Summary
	*models.SummaryParams
	AvailableFilters AvailableFilters
	AvatarURL        string
	// …
}
```

tangled's [`TimelineParams`](https://tangled.org/tangled.org/core/tree/master/appview/pages/pages.go)
is the same: `BaseParams` embedded, then `Timeline []models.TimelineGroup`,
`Repos []models.Repo`, `Issues []models.Issue` — store types verbatim — then view-only
computed fields like `ShowNewsletter bool` and `CanFocus bool` alongside. listmonk's
`unsubTpl` embeds `models.Subscriber` and `[]models.Subscription`.

So the params struct has three kinds of field and no functions:
- the embedded base (chrome)
- store types, by value or pointer, unmodified
- view-only values the handler computed (a formatted label, a flag, a colour map)

**3. The shared base.** One struct, in `render.go`, embedded by every page params struct.
It carries what the layout needs on every request. From what the four apps put in theirs:

| Field | Who has it |
|---|---|
| the logged-in account | tangled `BaseParams.LoggedInUser`, wakapi `SharedLoggedInViewModel.User` |
| flash success / error | wakapi `Messages{Success, Error}`, miniflux `flashSuccessMessage` / `flashErrorMessage` |
| CSRF token | miniflux `"csrf"` |
| static asset checksum for cache-busting | miniflux `theme_checksum` / `app_js_checksum`, listmonk `AssetVersion`, wakapi `getCacheBuster` |
| site name / root URL | listmonk `tplData` |

Copy wakapi's flash trick — it is the neatest thing in any of these four repos. It declares
`type Messages struct { Success, Error string }` with `SetError` / `SetSuccess` methods and a
one-method `BasicViewModel` interface, so **any** params struct that embeds the base can be
handed to a generic "render with this error" helper.

**4. The render method.** Copy tangled's one non-obvious idea: pair every params struct with
a typed method, so handlers never name a template string.

```go
type RunParams struct { BaseParams; Log store.Log; Entries []store.LogEntry }
func (r *Render) Run(w io.Writer, p RunParams) error { return r.page("run", w, p) }
```

tangled does this 119 times —
`func (p *Pages) Timeline(w io.Writer, params TimelineParams) error`, then
`return p.execute("timeline/timeline", w, params)`. The template name appears once in the
codebase, and the compiler enforces that a page gets the right data. For HTMX fragments it
has a second helper, `executePlain`, that skips the layout — Passion needs the same, since
20 of its 50 templates are fragments.

**5. Do fragments need their own params struct?** Yes, and a small one. tangled gives
`SignupSuccessParams` its own two-field struct with no `BaseParams`, rendered by
`executePlain`. A fragment reply has no layout, so it must not carry chrome.

**6. What must NOT happen.** No `store.DashboardData`. See §7.

---

## 6. What listmonk and tangled actually do, corrected

### listmonk — the close comparable, and two of the four claims need correcting

| Claim | Verdict | Reality |
|---|---|---|
| "28 flat handler files in `cmd/` beside `main.go`" | **Corrected** | **26 non-test `.go` files** including `main.go`, 9,426 lines, all `package main`, all flat. Roughly two-thirds are handler files; the rest are wiring — `init.go` (1,198), `install.go`, `upgrade.go`, `handlers.go` (the route table), `utils.go`, `manager_store.go`. There are **zero** `_test.go` files in the whole repository. |
| "`models/` at top level rather than internal" | **Confirmed** | 9 files, 1,270 lines. But see below — this does not support "no `internal/`". |
| "SQL in `queries/` as `.sql` files" | **Confirmed, do not copy** | 10 `.sql` files bound to `*sqlx.Stmt` struct fields in `models/queries.go` via `query:"get-lists"` tags, prepared at boot in `cmd/init.go`. This exists because listmonk has no ORM. Under GORM it would break the dialect seam. |
| "`internal/` used only for genuine internals" | **Corrected — this is backwards** | listmonk has **20 packages under `internal/`**, and the largest is `internal/core` (12 files, 2,549 lines) — **its entire data layer**. Only `models/` and `cmd/` sit outside `internal/`. So citing listmonk for a no-`internal/` tree is wrong: its actual choice is one public `models/` package and everything else private. The honest support for a flat, `internal/`-free tree is **wakapi** — 29,553 lines, zero `internal/`, all packages top-level — and gotify (GO_STRUCTURE §1.5). |

**Its DB layer, since that was asked.** Three parts, not one: `queries/*.sql` holds the SQL
text; `models/queries.go` holds a `Queries` struct of `*sqlx.Stmt` handles named by tag;
`internal/core` holds `Core{ db *sqlx.DB; q *models.Queries; … }` and one file per aggregate
executing them. No interface. No context, anywhere.

**Its handler signature.** `func (a *App) GetLists(c echo.Context) error` — a method on one
big `App` struct defined in `cmd/main.go` with 20 dependency fields (`db`, `queries`,
`core`, `manager`, `auth`, `media`, `bounce`, `captcha`, `i18n`, `pg`, `events`, `log`…).
Handlers call `a.core.GetLists(...)` and return `c.JSON(...)` or `c.Render(...)`.

**What to copy:**
- Flat handler files in one package at ~9,400 lines. It works, at our scale, with one dev.
- One route table in one function — `initHTTPHandlers(e *echo.Echo, a *App)` in
  [`cmd/handlers.go`](https://github.com/knadh/listmonk/blob/master/cmd/handlers.go), 452
  lines, with authenticated and public groups visibly separated in brace blocks.
- Per-page view structs in the handler file, embedding a shared base.
- The importer as a self-contained thing —
  [`internal/subimporter`](https://github.com/knadh/listmonk/blob/master/internal/subimporter/importer.go),
  765 lines — a good precedent for Passion's YAML catalog importer if `store/catalog.go`
  outgrows a file.

**What to avoid:**
- **The data layer returning `echo.HTTPError`.** §4.6.
- **A one-struct god object** with 20 dependency fields plus `sync.Mutex`, `needsRestart` and
  `needsUserSetup`. Passion's `web.Server` needs four fields.
- **No tests at all.** Zero `_test.go` files.
- `stuffbin` for asset embedding — that predates `embed`. Use `//go:embed`.
- No `context` anywhere.

### tangled/core — the judgement was right, but two things are worth taking

**Not comparable, confirmed with numbers.** 200,153 Go lines, 1,030 Go files, 53 top-level
directories, plus Rust (`Cargo.toml`, `crates/`) and four Go services — `appview` (69,188),
`spindle` (61,777), `knotserver` (15,065), `knotmirror` (6,829). 16x Passion. Its top-level
shape answers a monorepo problem we do not have.

**The `orm/` and `sqlite/` packages, since those were the question:**

- [`orm/orm.go`](https://tangled.org/tangled.org/core/tree/master/orm/orm.go) is **154 lines
  and is not an ORM.** It is three unrelated things: `IsUniqueViolation(err)`, a
  `RunMigration(conn, logger, name, fn)` that checks a `migrations` table by name inside a
  transaction, and a `Filter` type — `FilterEq`, `FilterIn`, `FilterLike`, `FilterContains`,
  `FilterInSubquery` — that compiles predicates to `key = ?` / `key in (?, ?, ?)`. **Nothing
  to take.** GORM already does all of it, and its `FilterIn` on an empty slice returning
  `"1 = 0"` is a hand-rolled edge case we get for free.
- [`sqlite/sqlite.go`](https://tangled.org/tangled.org/core/tree/master/sqlite/sqlite.go) is
  **22 lines**: a DSN string list and `Open`. **Take it as confirmation of decision 5**, per
  §4.5. Note it is a whole package for 22 lines — do not copy that; ours is a function in
  `store.go`, as gotify does it (GO_STRUCTURE §2.7).

**The one thing genuinely worth copying, and it is significant:** its `appview` — one
service of 69k lines, Go + HTMX + `html/template`, no SPA — is the best available worked
example of the view-model question, and it answers it in exactly the shape §5 recommends. It
is also structural evidence for decision 2: at 69,188 lines it still has **no service layer**
between handlers and `db/`.

Two smaller items: `appview/pages/htmx.go` is a 57-line file of `hx-swap-oob` helpers —
`Notice(w, id, msg)` writes `<span id="…" hx-swap-oob="innerHTML">…</span>` — which Passion
will want in `render.go`. And its `//go:embed templates/* static legal` sits inside
`appview/pages/`, which is the alternative fix to §2's embed problem if you would rather move
`templates/` under `web/` than embed from `main.go`.

### The third and fourth data points

**wakapi** is the single closest structural match found: Go, **GORM**, `html/template`,
single binary, SQLite + PostgreSQL + MySQL, HTML routes and a JSON API side by side, no
`internal/`, `main.go` in the root, 29,553 lines. Read it before writing any of this. Its
`models/view/` package and its `routes/` + `routes/api/` split are the two things to steal.
Its `IXxxService` interfaces and `mocks/` are the two things not to.

**miniflux** is the dissenting view-model answer — `map[string]any` and a fluent
`view.New(tpl, r).Set("entries", entries).Render("unread")`. Rejected: it gives up compile-time
checking of the frontend contract, which is the one thing Passion's 81 structs currently buy.
Worth reading only for the list of chrome fields its constructor seeds.

---

## 7. What the proposal is missing

Ordered by what will cost the most.

| # | Missing | The answer |
|---|---|---|
| 1 | **`embed` cannot reach `../templates`** | Build error, not a preference. `//go:embed templates static catalog` in `main.go`; pass `fs.FS` to `web.New` and `store.ImportCatalog`. Alternative: move `templates/` and `static/` under `web/` as tangled does. Pick one before writing a line. |
| 2 | **Middleware has no home, and `session.go` is a name collision** | `web/middleware.go` — auth check, cookie session, CSRF, request log, panic recover. Delete `session.go` from the file list; in a training app that name means training sessions. |
| 3 | **View models** | §5. This is the largest missing piece by volume — 81 structs. |
| 4 | **The YAML catalog importer** | `store/catalog.go`: the YAML structs (a third struct family, unexported) and one exported `ImportCatalog(ctx, fs.FS) error` owning its transaction. It is a multi-table write, so decision 2 puts it in `store/`. |
| 5 | **Background jobs** | Scheduled sessions need a ticker. It is not HTTP, so it does not belong in `web/`; it needs the store, so it does not belong in `config/`. Start it from `main.go`; if it passes ~200 lines, `jobs.go` in the root `package main`. Precedent: [listmonk `internal/manager`](https://github.com/knadh/listmonk/tree/master/internal/manager) (1,226 lines, own package, started from `cmd/init.go`). |
| 6 | **Tests and their placement** | Same package, same directory, `_test.go` beside the code — [wakapi](https://github.com/muety/wakapi/blob/master/routes/login_test.go) puts a 910-line `login_test.go` in `routes/`, tangled puts `repos_test.go` in `appview/db/`. Real SQLite file in `t.TempDir()`, no store mocks (GO_STRUCTURE §4.2). Note listmonk ships **zero** tests — do not take that from it. |
| 7 | **Static asset serving and cache-busting** | All four projects version their assets. Compute a checksum of the embedded `static/` at boot, put it in `BaseParams`, serve from the embedded FS with a far-future `Cache-Control`. One field and ~15 lines in `render.go`. |
| 8 | **Errors becoming status codes** | Nothing says where `gorm.ErrRecordNotFound` becomes a 404 and `ErrDuplicatedKey` becomes a 409. One function in `web/render.go`. This is the mechanism that lets §4.6's rule hold. |
| 9 | **Flash messages and CSRF** | Fields on `BaseParams`, populated by `middleware.go` from the session. §5 point 3. |
| 10 | **`migrate.go` contradicts GO_STRUCTURE §2.8** | The proposal describes a hand-rolled runner; its own research says goose, embedded, one numbered sequence, `AutoMigrate` never in production. Use the research. |

### Two claims in the proposal's reasoning that are inaccurate

Both are citations, not decisions, so no decision changes — but the reasoning should not be
reviewed later on a false basis.

1. > "No `internal/`. … listmonk puts `models/` at the top level deliberately."

   listmonk has **20 packages under `internal/`**, including its whole data layer. It is not
   evidence for a tree without `internal/`. Cite wakapi (29,553 lines, no `internal/`)
   instead. The conclusion — Passion does not need `internal/` — still stands, on the
   proposal's own better argument that nobody imports Passion as a library.

2. > "listmonk does this with 28 files in `cmd/`."

   26 non-test files, 9,426 lines, `package main`, and several of those are wiring rather
   than handlers. The number that actually supports the argument is the LOC, and it supports
   it strongly. Use it.

---

## 8. The single biggest risk

Not the package count. It is this:

> **A page-shaped read gets added to `store/`, and after that the store's return types are
> the HTML's shape.**

The dashboard needs data from six tables. The path of least resistance is
`store.GetDashboard(ctx, accountID) (DashboardData, error)` — one call, one struct, one
query, and it feels like exactly what decision 2 sanctions, since it spans several tables.
Then the run summary gets one. Then history. Within a month `store/` has a dozen methods
whose return types are named after screens, and:

- **decision 2 collapses** — `store/` is the dumping ground the proposal asked about, and
  it is `app/` under a different name, without `app/`'s honesty
- **decision 6 collapses** — the JSON API cannot reuse `GetDashboard`, because its shape was
  chosen by a template. It re-implements the reads, and now there are two of everything
- **§4.6's rule does not catch it** — `DashboardData` names no HTTP type. It passes the
  signature test cleanly

**The rule that prevents it, and it is the load-bearing rule of this whole tree:**

> A `store/` method returns rows shaped like tables, or slices of them, or a scalar. It never
> returns a type named after a screen. Assembling a page is the params struct's job, done in
> the handler by calling several store methods.

Six store calls in a handler is the correct shape, not a smell. That is what all four
projects read for this review actually do, and it is what makes the params struct in §5 the
right place for the frontend's contract: the params struct is allowed to know about the
screen, and the store is not.

The one exception, and write it down as an exception: a genuine aggregate — a JOIN or a
window function that has to run in the database because it cannot be assembled in Go without
N+1 queries. That returns a named row struct in `store/`, and it is named after the query,
not the page.
