# Repository structure — proposal for review

Written 7 September 2026. This is a proposal, not a decision. It exists to be reviewed.

## Context

Passion is being rewritten backend-first. The frontend's look and feel is kept; what it
currently displays is not a requirement. One developer. Go, HTMX, server-rendered
`html/template`. GORM. PostgreSQL and SQLite are the two supported engines, decided —
MySQL is out, because the schema depends on partial unique indexes which MySQL lacks.

Scale for reference: today's code is ~12,000 lines of handlers, 50 templates, 19 tables.

The owner's overriding constraint, repeated many times: **simple and easy for a human to
read and review.** Not clever. He will reject a tree with many packages on sight.

Project code style, from CLAUDE.md:
- No comments unless the WHY is non-obvious.
- No error handling for scenarios that cannot happen.
- Prefer fewer files over more.

## The proposal

```
passion/
├── main.go            flags, config, open store, migrate, serve
├── config/            settings struct and loading
├── store/             the domain structs, all queries, migrations, the dialect seam
│   ├── store.go         Open(), pool setup, pragmas, WithTx
│   ├── models.go        the structs that mirror the 19 tables
│   ├── migrate.go       schema creation and versioned migrations
│   ├── content.go       catalog reads and writes, forking
│   ├── log.go           runs and history
│   ├── plan.go          cycles, targets, scheduling
│   └── account.go       accounts, measurements, places
├── web/               one file per area, no nesting
│   ├── server.go        router, middleware, session handling
│   ├── render.go        template loading and the view helpers
│   ├── auth.go  library.go  session.go  run.go  plan.go  history.go  account.go
├── templates/         unchanged from today
├── static/            unchanged from today
└── catalog/           YAML that seeds the shipped content
```

Three Go packages plus `main`.

## The reasoning, and where it came from

**No `internal/`.** Nobody imports Passion as a library, so `internal/` enforces nothing
we need and lengthens every import path. listmonk puts `models/` at the top level
deliberately.

**No `pkg/`.** Russ Cox filed an issue against `golang-standards/project-layout` titled
"this is not a standard Go project layout".

**Fewer, larger packages.** Dave Cheney's guidance is titled "Consider fewer, larger
packages". Comparable projects: wtf is 11 packages at 6k lines, hetty 13 at 6k.

**The structs live in `store/`, not their own package.** `web` already imports `store` to
run queries, so there is no import cycle to avoid and no second package to justify. With
GORM the structs *are* the tables, so a separate domain-model layer would be a copy.

**One flat file per feature area in `web/`.** listmonk does this with 28 files in `cmd/`.
`web/run.go` says what is in it without being opened.

**GORM stays, and the dialect seam is GORM's own.** Its `Dialector` interface is eight
methods and already contains every difference measured between PostgreSQL and SQLite:
identity columns and date types in `DataTypeOf`, placeholders in `BindVarTo`, quoting in
`QuoteTo`, and the SQLite table rebuild needed to add a CHECK. Gotify — a small
self-hosted Go app on GORM with three engines — has a per-engine seam of about 15 lines in
one `switch`. Nine multi-engine Go projects were read; eight use one implementation with
small dialect switches, one keeps parallel implementations.

## Decisions this proposal makes, which the review should attack

1. **Three packages.** Is `store/` doing too much by holding structs, queries, migrations
   and connection setup in one package?
2. **No separate package for logic spanning aggregates.** An earlier draft had `app/` for
   operations touching several tables — forking a session writes `content` and
   `content_item`; starting a run writes `log` and `log_entry`. This proposal puts those in
   `store/` as exported methods that own their transaction. Is that right, or does it make
   `store/` a dumping ground?
3. **Transactions.** Exported store methods open the transaction; unexported helpers take
   `tx *gorm.DB` and never touch `s.db`. This is deliberate: GORM detects nesting on the
   handle, not globally, so a helper using `s.db` inside an outer transaction silently
   opens a second independent one, and both are the same type so nothing warns you.
4. **Context.** Every store method takes `ctx context.Context` first. Today's code threads
   none, and the audit flags it.
5. **SQLite connection.** WAL, `_busy_timeout`, `_foreign_keys=on` and
   `_txlock=immediate` in the DSN, plus `SetMaxOpenConns(1)`. Foreign keys are
   per-connection in SQLite and a pool opens many, so it must be in the DSN, not a PRAGMA.
6. **A later JSON API.** The plan is a fourth package beside `web/` over the same store.
   Does this structure actually keep that cheap, or does it only appear to?

## Known open question

Where do view models live? Today `pages/pages.go` holds 81 view structs and is the
frontend's contract. This proposal has `web/render.go` and says nothing more. That is
probably underspecified.
