# V2 plan — executability review

> **Snapshot, not the plan.** This records a review as it stood on its own date. Decisions
> have moved since — see `V2_PLAN.md` §1.10 (private trees as owned content, fork-on-edit,
> `source_tree`) and §1.11 (`content_key` replacing the log's text link). **Where this file
> disagrees with `V2_PLAN.md` or `SCHEMA_V2.sql`, those two win.** Nothing here is edited to
> keep up. That is what makes it a snapshot.

Written 8 September 2026. One question only: **can a competent Go developer execute
`V2_PLAN.md` in order, without stopping to invent a decision the plan should have made?**

Not a correctness review. I do not audit the schema, re-derive the numbers, or rank defects by
severity. Every count below has its command in the appendix.

---

## 1. Verdict

You could start on Monday and get about two hours in — `go get` for goose and the Postgres
driver — and then you would stop, because phase 0 writes `store/models.go` and
`001_init.sql` and the plan does not say whether GORM pluralizes the table names, whether a
nullable number is a pointer, or how a hangboard ladder is keyed. **I hit 13 stopping points
in phase 0 — one irreversible on SQLite, three more that reach every file written after them.** Phase 1 also contains a build error the
plan cannot compile past: `cmd/passionv2/main.go` cannot `//go:embed` `templates`, `static`
or `catalog`, because those trees are two directories above it.

---

## 2. Every stopping point

### Phase 0 — foundations

The file list I would create, in order, and where it stops.

| # | Where I stop | What the plan should say |
|---|---|---|
| **0-A** | `go.mod`. The plan says "decide cgo or pure-Go SQLite here" and does not decide | Name the driver and paste the DSN. If pure-Go, also change `.github/workflows/deploy.yml`, which builds `CGO_ENABLED=1` today, and the pragma spelling from `_foreign_keys=on` to that driver's form |
| **0-B** | `go.mod`. Which goose, and is the CLI required? The phase-0 gate says "`goose status` reports the same version on both engines", but migrations are embedded and applied by the binary | "goose is a library only. The gate is `passion --migrate-status` against each engine." Otherwise the developer installs a CLI that cannot read `embed.FS` |
| **0-C** | `config/`. `store.Open(ctx, cfg.Database)` needs a `Database` config section; no phase owns `config/`. §1.3 lists the changes and assigns them to nobody | Put `config/` in phase 0's file list, with the env-var names (`PASSION_DATABASE_ENGINE`, …). Phase 13's gate asserts the readme table matches the struct, so the names must exist from the start |
| **0-D** | `store/models.go`. **GORM pluralizes table names by default** (`NamingStrategy.SingularTable` is false in gorm v1.31.1). `SCHEMA_V2.sql` names every table singular | "`store.Open` sets `NamingStrategy{SingularTable: true}`." One line, and without it every query hits `accounts` / `contents` / `log_entries` and nothing works |
| **0-E** | `store/models.go`. "A NULL number means not asked" — so what Go type? `*int`, `sql.NullInt64`, or `int` | One sentence: "every nullable column is a pointer; `0` is a real value." This decision reaches all 19 structs, every template field and every form parse. It cannot be changed later cheaply |
| **0-F** | `store/models.go`. `CHAR(36)` keys on the four log tables. No UUID library is in `go.sum` | Name the library in phase 0's `go.mod` step, and say whether phase 3 accepts a client-supplied id or generates server-side |
| **0-G** | `store/models.go`. Dates are `VARCHAR(10)`; timestamps are `TIMESTAMP` | "`OnDate string`, `CreatedAt time.Time`, all UTC." State it once — 329 template field references depend on the formatting |
| **0-H** | `001_init.sql`. **The ladder decision is missing.** `V2_PLAN_REVIEW.md` §3.2.3 says pick option A, B or C *in phase 0*; the plan's §1.1 lists nine changes and this is not one of them. Part 2 says only "`per_set` replaces the `rungs` string", and its `per_set` example carries `reps:`, which cannot express three rungs inside one set. `content_item_set`'s primary key is `(content_item_id, set_index)` and SQLite cannot alter a primary key later | Decide A, B or C in the plan, and if A, write the two columns and two key changes into 001. **This is the worst gap in the document**: it is irreversible, it is absent from the list of irreversible things, and 3 private files plus `run.html`'s `RungSeconds` depend on it |
| **0-I** | `001_init.sql`. Who writes the SQLite variant, and from what? `SCHEMA_V2.sql` is PostgreSQL | "The postgres file is `SCHEMA_V2.sql` plus §1.1's changes, verbatim. The sqlite file differs only in the identity clause." Also delete the schema's "WHAT GORM CANNOT EXPRESS" section — nothing is AutoMigrated any more, so those notes now mislead |
| **0-J** | `001_init.sql`. goose file format: `-- +goose Up`, a `Down` block, and `StatementBegin/End` around anything with an inner semicolon | Paste a 10-line migration template. Say what `Down` holds when migrations are append-only and never run down |
| **0-K** | `store/migrate.go`. §1.5 step 4 says the embeds live in `main.go` "because embed patterns cannot contain `..`", which reads as *all* embeds | "`store/migrate.go` embeds `migrations` itself — it is inside `store/`, so no `..` is needed." A developer following 1.5 literally passes the FS in from `main` and then no test can migrate |
| **0-L** | The Postgres harness. No compose file, no container library, no CI test workflow exists | Say what `PASSION_TEST_POSTGRES` holds (a DSN), give the `docker run` line, add `make pg-up` / `pg-down` / `test-postgres`, and say whether parallel tests share one database or get a schema each |
| **0-M** | `store/BEHAVIOUR.md`. The gate depends on it; it does not exist and nothing says how to derive it | "Seed it from `grep -h '^func Test' db/*_test.go` (130 funcs), mark each ported / dropped / not-applicable with a phase, and write the file **before** any test." It is a phase-0 deliverable, not a note |

### Phase 1 — accounts and the web skeleton

| # | Where I stop | What the plan should say |
|---|---|---|
| **1-A** | `cmd/passionv2/main.go`. **This does not compile.** Go 1.26.3's `go doc embed`, verbatim: "Patterns may not contain '.' or '..' or empty path elements". `templates`, `static` and `catalog` are two directories above `cmd/passionv2/` | The v2 binary is the repo root `main.go`. The root has no `.go` file today, so the slot is free, and `STRUCTURE_REVIEW.md` already puts the final `main.go` there. The old binary stays at `cmd/passion`. Phase 12's gate line becomes "`cmd/passion` is gone" |
| **1-B** | `web/render.go`. `BaseParams`' field list. The 50 existing templates already name their chrome: `.Authenticated .AuthFormError .Email .InviteCode .InviteRequired .IsAuthPage .ProseMirror .Title .WideLayout` in the layout plus login and signup | "The params structs reproduce the field names the templates already use; 329 distinct `.Field` references are the contract, extracted with the command in the appendix." Or say the templates get renamed — but say which |
| **1-C** | `web/render.go`. `STRUCTURE_REVIEW.md` §5.4 recommends a typed render method per page plus an `executePlain` for fragments. The plan neither adopts nor rejects it | Adopt or reject in one line, and show one page and one fragment. Every later phase copies whichever shape phase 1 lands |
| **1-D** | `web/render.go`. 50 files and 26 `{{ define }}` blocks, with `{{ define "content" }}` in every page — parsing them all into one set makes the last one win | "One `template.Template` per page: layout + shared fragments + that page, built at boot into a map." Today `pages/pages.go` solves this; the developer must not have to re-derive it |
| **1-E** | `-seed` and `DevAuthBypass`. There is no owner sentinel, and `DevAuthBypass` auto-authenticates as **user 1** today (`passion.example.yaml`, `docs/DEVELOPMENT.md`) | "`-seed` creates `dev@localhost` with a fixed password, prints it, and is idempotent by email. `DevAuthBypass` resolves that account by email and refuses to boot if it is absent." With identity primary keys, id 1 may not exist |
| **1-F** | The gate's "all nine tables that reference an account". The plan drops the list; the schema alone gives a different set, because `log_entry.account_id` is frozen and its foreign key points at `log`, not `account` | Name the nine: `body_measurement`, `grade_milestone`, `content.author_id`, `plan`, `scheduled`, `calendar_event`, `place`, `log`, `log_entry` |
| **1-G** | `web/middleware.go`. The `token_epoch` JWT claim's name and comparison | "Claim `epc`; reject when `epc != account.token_epoch`." It is security-relevant and one line |
| **1-H** | `/static/*` and the cache-busting checksum. §1.5 step 8 computes it and nothing says how it reaches a URL | "`{{ .AssetVersion }}` appended as `?v=`, so `layouts/base.html` changes in phase 1." Otherwise phase 1 ships and the checksum is dead weight until someone notices |

### Phase 2a — format, importer, exporter, public tree

| # | Where I stop | What the plan should say |
|---|---|---|
| **2a-A** | The first converted file. **`label:` → `tags:` is not in the plan.** All 225 files carry `label: "a, b, c"` — a comma string. No file has `tags:`. `kind:` → `movement_kind:` is also unlisted, on 184 files | List four renames, not two: `label`→`tags` (string to list, 225 files), `kind`→`movement_kind` (184), `video_url`→`url`, `thumbnail_url`→`thumb_url` (112 files). `label`→`tags` is the one the importer hard-fails on |
| **2a-B** | `tags.yaml`. The plan says 29 entries. There are 29 distinct labels, but `shoulder` and `shoulders` both appear, so the vocabulary is **28** — and `lead` appears only in the private tree | "tags.yaml holds 28 rows, includes `lead` although no public file uses it, and one file's `shoulders` is rewritten to `shoulder`." Or: "a `Catalog.Dirs` tree may carry its own tags.yaml, merged before validation." Decide, because an unknown tag is a hard failure |
| **2a-C** | Two keys with no mapping in the format table: `session_duration_seconds` (1 file) and `rung_seconds` (3 files) | Map the first to `seconds:` / `d_seconds`. The second is the ladder decision (0-H) |
| **2a-D** | `movements/`. The plan's layout is flat. 29 public and 96 private exercise files live in source-named subdirectories (`ondra/`, `kettle/`, `paradigm/`, …) | "The importer walks `movements/` recursively and the subdirectories stay" — or "flatten, one `git mv` of 125 files". The deploy script has a comment about these subfolders, so flattening touches it too |
| **2a-E** | `content_media`. 2 media items carry `thumbnail_url` with no `video_url`, and `content_media.url` is `NOT NULL` | "A media item with no url is dropped by the importer" — and fix the 2 files, or the import fails on the private tree |
| **2a-F** | The format spec has no path. `docs/` has no `CATALOG_FORMAT.md` | Name the file. The private repo's README must point at it, and `format_version` is meaningless without a document that owns the number |
| **2a-G** | `format_version`. Value, type, and behaviour on absence are unspecified | "`format_version: 1`, an integer, required, refused if absent or unknown." Better: one `catalog.yaml` manifest per tree instead of a line in 271 files |
| **2a-H** | The exporter's output shape. With no inline children, a session exports as 1 + N + M files | Say whether export writes a directory, a tar, or a multi-document YAML. The round-trip gate cannot be written before this |
| **2a-I** | `retired_on`. Booting **without** `Catalog.Dirs` — the normal dev case — would retire 123 private slugs, and the next boot with the tree must un-retire them | "Retirement is computed only over trees that were actually loaded, and never when a configured directory is missing." The deploy workflow's own comment records that the old `pruneCatalogOrphans` deleted licensed content this exact way |
| **2a-J** | The importer's pass structure. Refs are bare and cross kinds; files arrive in directory order | "Two passes: parse everything and build a slug → (kind, id) map, then write rows and edges." One sentence saves a day |
| **2a-K** | The idempotency gate. `content.updated_at` is `NOT NULL` and a naive re-upsert moves it, so "import twice; every table's checksum unchanged" cannot pass | "The upsert skips rows whose content is unchanged" — or "the checksum excludes `updated_at`" |

### Phase 3 — run and log

| # | Where I stop | What the plan should say |
|---|---|---|
| **3-A** | `CurrentStep`. The plan says freeze the contract before touching `run.html` and does not give it. `run.html` names 20 distinct `CurrentStep.*` fields | Paste the 20 as a Go struct in the plan. Four change shape: `RungSeconds` (0-H), `SessionDurationSeconds` (renamed), `PlannedSets` (now `log_set` rows), `CatalogOptions` (the menu). And say what `ExerciseID` now holds — a content id or a `log_entry` id |
| **3-B** | The menu flow. §1.7 deletes the pending row on choose. Today's handler re-inserts and the choice is **re-editable** | Either keep the row and mark it resolved, or say plainly in §1.8 that changing your mind mid-session is dropped. Also say what a finished session with an unchosen menu shows |
| **3-C** | The five-source resolver. It needs a plan and a week number; an ad-hoc run has no plan | "Plan = `scheduled.plan_id`, else the plan whose date range covers the run date, else none. Week = `(on_date − plan.starts_on) / 7 + 1`, in the account's time zone." Otherwise phases 3 and 6b compute the week twice, differently |
| **3-D** | The progression query. It does not filter the log's state, so a `draft` manual entry appears in the "last five times" hint. The review flags it; the plan does not mention it | Decide here, because phase 3 writes the query: join `log`, or freeze `state` onto `log_entry` |
| **3-E** | `log_set` at freeze time. Nothing states the `set_mode` invariant | "`set_mode='simple'` implies zero `log_set` rows", enforced in the one write path |
| **3-F** | The gate's "drop every content table". With foreign keys on, `DROP TABLE content` is refused while `log.session_id` references it | Say the test drops in dependency order, or toggles `foreign_keys` off for the drop. As written the best gate in the plan does not run |
| **3-G** | The phase-3 route set. `run.html` is 1,566 lines of already-written `hx-post` targets; the review counts 23 `/runs/*` patterns split across phases 3, 9b and 10 | Enumerate the phase-3 subset. A missing route inside HTMX is a silent 404, not a compile error |

---

## 3. Step order inside phases 0, 1, 2a and 3

### Phase 0

| Step | Blocked by | Note |
|---|---|---|
| 1. `store/BEHAVIOUR.md` | nothing | It is a list. Write it first — it sizes the rest of the phase |
| 2. Decisions 0-A, 0-D, 0-E, 0-G, 0-H | nothing | All four are irreversible. None needs code |
| 3. `go.mod` / `go.sum` | 0-A | |
| 4. `config/` `Database` section | 0-A | `store.Open` names its type |
| 5. `store/models.go` | 0-D, 0-E, 0-F, 0-G | Cannot start before the naming and nullability rules |
| 6. `migrations/postgres/001` then `sqlite/001` | 0-H, 0-I, 0-J | Must agree with models.go column for column |
| 7. `store/store.go` (Open, DSN, WithTx) | 3, 4 | |
| 8. `store/migrate.go` + `//go:embed migrations` | 6, 7 | |
| 9. `newTestStore(t)` helper + the pragma test | 8, 0-L | Nothing else can be tested before this |
| 10. The ported behaviour suite | 1, 9 | |

Cannot be reordered: 5 and 6 both depend on the same four decisions, so a developer who starts
at step 5 on Monday morning writes structs twice.

### Phase 1

| Step | Blocked by | Note |
|---|---|---|
| 1. Fix 1-A — the binary's location | nothing | Do this before anything else in phase 1 |
| 2. `config/` auth + log + catalog sections | 0-C | |
| 3. root `main.go` with §1.5's order | 1, 2, phase 0 | Every browser gate from here on runs against it |
| 4. `web/render.go` — BaseParams, parse strategy, error→status | 1-B, 1-C, 1-D | |
| 5. `store/account.go` | phase 0 | Needs `token_epoch` |
| 6. `web/middleware.go` | 4, 5 | Cannot be written before the store reads `token_epoch` |
| 7. `web/auth.go` | 4, 5, 6 | |
| 8. `templates/signup.html` edit | 7 | The invite field is already behind `{{ if .InviteRequired }}`, so this is a deletion, not a rewrite |
| 9. `/static/*` + checksum + `layouts/base.html` | 1-H, 4 | |
| 10. `httptest` cascade and password-change tests | 1-F, 7 | |

### Phase 2a

| Step | Blocked by | Note |
|---|---|---|
| 1. The format spec document | 2a-A…2a-H | Freeze it before a single file is converted |
| 2. `catalog/tags.yaml` | 2a-B | |
| 3. YAML structs in `store/catalog.go` | 1 | |
| 4. The two-pass importer | 2, 3, 2a-I, 2a-J, 2a-K | |
| 5. `cmd/convert/` — the mechanical converter | 1 | See §5. Do not sed by hand |
| 6. Convert the public tree (102 → 107 files) | 1, 5 | |
| 7. The exporter | 3, 4 | |
| 8. Round-trip and failure tests | 4, 6, 7 | |

Cannot be built earlier: the round-trip test needs the exporter; the exporter needs the YAML
structs; the tree conversion needs the frozen spec. Converting before step 1 is the single
most expensive mistake available in this phase — 271 files get touched twice.

### Phase 3

| Step | Blocked by | Note |
|---|---|---|
| 1. The `CurrentStep` contract, written down | 3-A, 0-H | Before any Go and before `run.html` |
| 2. The week/plan rule | 3-C | Shared with 6b |
| 3. `store` five-source resolver | 2, phase 2a | |
| 4. `store/log.go` — freeze in one transaction | 3, 3-E, 0-F | |
| 5. Menu freeze and choose | 4, 3-B | |
| 6. `log_climb` writes | 4 | |
| 7. Progression query | 3-D | |
| 8. `web/run.go` + the phase-3 route set | 4, 5, 6, 3-G | |
| 9. `run.html` and its fragments | 1, 8 | Last. It is 1,566 lines and the timer fails silently |
| 10. History list | 4 | Independent of 5-9; can run in parallel |

---

## 4. The daily loop

### `make watch` and air

Air v1.65.3's own config struct names the keys `tmp_dir` (top level) and `include_dir`,
`exclude_dir`, `include_ext` under `[build]`. The repo's `.air.toml` uses `[watch] include`
and `[tmp] dir`, which are **not air keys** and are ignored by its TOML decode. So air runs
on its defaults today: `include_ext = ["go","tpl","tmpl","html"]`,
`exclude_dir = ["assets","tmp","vendor","testdata"]`, watching the whole root.

| Change | Why |
|---|---|
| `[build] cmd = "go build -o ./tmp/passion ."` | The v2 binary is the root `main.go`, not `./cmd/passion`. During the transition use a second file, `.air.v2.toml`, so `make watch` can still run master's binary |
| `[build] include_ext = ["go","html","yaml"]` | **`yaml` is missing today.** With `catalog/` embedded, a YAML edit needs a rebuild, and air will not notice one |
| `[build] exclude_dir = ["tmp","vendor","testdata","docs",".claude"]` | Replaces the ignored `[watch]` block with keys air reads |
| Add `make test`, `make test-postgres`, `make pg-up`, `make pg-down`, `make backup` | There are 4 Make targets today and **no `test` target**, although `.air.toml`'s comment refers to `make test`. Phase 13's gate is "every Make target runs" |
| `make reseed` drops `--exit-after-seed` | That flag is one of the 16 that die |

Templates keep working: `html` is watched, and a rebuild re-embeds them. The cost is that
every template and catalog edit is now a full Go build plus a restart plus a migrate plus a
271-file re-import. Measure that boot time in phase 2a; if it is over a second or two, add
`-skip-import` for the watch loop.

### Postgres locally

No compose file, Dockerfile, or container library exists anywhere in the repo. The plan says
"a throwaway container" and stops. What it should say:

```
make pg-up    docker run -d --name passion-pg -e POSTGRES_PASSWORD=passion \
                -e POSTGRES_DB=passion -p 5433:5432 postgres:17
make pg-down  docker rm -f passion-pg
PASSION_TEST_POSTGRES=postgres://postgres:passion@localhost:5433/passion?sslmode=disable
```

Also missing: `passion.postgres.yaml` as a committed example, and the note that
`DevAuthBypass` refuses to boot on Postgres (§1.3), so running the *app* against Postgres
means logging in for real — which means `-seed` has to have created an account.

### The private catalog in development

`Catalog.Dirs: ["../passion-private-catalog"]`. It exists as a sibling of this repo today.
Three things the plan must add:

1. What a `Catalog.Dirs` entry points at — a tree root holding `movements/ menus/ blocks/
   sessions/`, or a single kind directory. Today's config had one key per kind and all three die.
2. Whether such a tree carries its own `tags.yaml` and `format_version` (see 2a-B, 2a-G).
3. That air's root is `.`, so a sibling directory is **not watched**. Editing the private tree
   needs a manual restart, and there is no way around it while the tree is a second repository.

And `.github/workflows/deploy.yml` needs a phase: it ships `catalog`, `catalog-private`,
`templates` and `static` to `/opt/passion` as directories today, and V2 embeds three of the
four in the binary. The plan never mentions the deploy workflow.

### What `-seed` means now

Unanswerable from the plan. `DevAuthBypass` auto-authenticates as user 1, and
`SeedDevIfEmpty` keys off `DemoOwnerID`, which §1.3 kills. What it should say:

- `-seed` creates `dev@localhost` with a fixed printed password if no account exists, then
  seeds for that account. Idempotent by email; refuses on a database with other accounts.
- `DevAuthBypass` resolves that email, and refuses to boot if it is missing.
- The fixtures grow with the phases. `docs/DEVELOPMENT.md` lists what today's seed covers —
  45 runs, a 4-week cycle, 165 ticks, 4 venues, 3 boards, deliberately awkward long names.
  None of it can be written before phase 9. Say that each phase adds its own fixtures, or
  phase 9 inherits all of it at once.

### Switching engines without wiping

No. Two DSNs are two databases, and nothing in the plan moves rows between them: identity
primary keys need sequence resets on restore, and the four log tables use client-created
UUIDs. Write this down as a limitation:

> SQLite and Postgres hold separate databases during the build. Only the migration sequence
> is shared. Dev data is disposable.

And note the risk table's own consequence: if V2's SQLite file becomes the real training log
from phase 3, "disposable" stops being true, and `make backup` is a phase-3 deliverable that
nobody has written.

---

## 5. Are the gates enough?

Each gate is a good gate. What no gate anywhere in the plan catches:

| Loose end | The check that catches it |
|---|---|
| A migration written for one engine only | Phase 0's gate compares version *numbers*. A column present in postgres and absent in sqlite passes. Dump both schemas and compare normalised column lists |
| A table nothing queries | Insert and read one row in all 19 tables, once, in phase 0 |
| A template referencing a field nobody sets | `html/template` fails at execute time, not parse time. Execute every template against a zero-valued params struct — one test, all 50 files, every phase |
| A route registered but unimplemented | The named risk — 37 actions behind 7 catch-alls. Keep a checked-in route list and assert the router matches it. No phase has this gate |
| §1.5 and §1.6 promises | Nothing gates the two footgun warnings, `/healthz`, the request id on a 500 page, the newer-database refusal, or the 200 ms slow-query log. All are stated once and tested nowhere |
| Conversion completeness | Phase 2a's gate counts movements. It does not assert that every file's slug equals its filename stem, that all 65 public refs resolved, or that no old key survives. All are one-liners (§6) |

---

## 6. The conversion procedure

225 files today → **271** after conversion (public 102 → 107, private 123 → 164), plus
`tags.yaml`. Do it as three phases, per repository, public first.

### A. Mechanical — write a converter, not a sed pipeline

Put a throwaway `cmd/convert/main.go` in the repo and delete it after phase 2b. The format
will change while you convert; a script is re-runnable and a hand-edit is not.

1. `git mv exercises movements`, `activity_templates blocks`, `session_templates sessions`; `mkdir menus`.
2. `kind:` → `movement_kind:` in `movements/` only, and `movement_kind: "session"` → `"duration"` (56 files).
3. `video_url:` → `url:`, `thumbnail_url:` → `thumb_url:` (112 files carry media).
4. `label: "a, b"` → `tags: [a, b]` (all 225 files); rewrite the one `shoulders`.
5. `session_duration_seconds:` → `seconds:` (1 file).
6. `format_version: 1` at the top of every file.

### B. By hand — 46 items and 9 invented names

7. **24 menus.** Each is an inline item with `kind: "exercise_catalog"` and `children:`
   (public 3, private 21). Cut it to `menus/<slug>.yaml` with `name`, `notes`, `tags`, `pick`;
   leave `- ref: <slug>` behind. Needs a stated slug rule — lowercase, `[a-z0-9_]`, punctuation
   dropped: `"Easy campusing (optional)"` → `easy_campusing_optional`.
8. **22 inline blocks.** Each is `- type:` inside a session's list (18 `activity`, 2 `warmup`,
   2 `cooldown`; public 2, private 20). `type: activity` → `block_kind: main`. **9 have no
   name** — invent one each: 2 in `boulder_session.yaml` and
   `emil_submax_daily_fingerboard_routine.yaml`, 7 across `a_foundations.yaml`,
   `b_flow_and_power.yaml` and `c_reading_and_tactics.yaml`.
9. **The duplicate-name rule.** Two different blocks are both named "Endurance Work"
   (`endurance.yaml`, `pcc_do_more.yaml`) and each holds a menu named "Endurance Method" —
   with different notes and different option lists. They cannot merge. State the rule:
   identical inline blocks share one slug; different ones get distinct slugs, and per-use
   prose moves to `content_item.notes`.
10. **6 per-use block names** beside a `ref:` in 3 private session files: rename the block,
    clone it, or move the text to `notes:`. One decision each.
11. **3 `rung_seconds` files** per the ladder decision (0-H).
12. **2 thumbnail-only media items** per 2a-E.

### C. Verify — all scriptable, and it is the acceptance test

```sh
# slug equals filename stem
for f in $(find catalog -name '*.yaml' ! -name tags.yaml); do
  s=$(sed -n 's/^slug: *"*\([a-z0-9_]*\).*/\1/p' $f); [ "$s" = "$(basename $f .yaml)" ] || echo "MISMATCH $f"
done
# slugs unique across all four kinds and across both repos
grep -rhE '^slug:' catalog | sed 's/slug: *"*//;s/"*$//' | sort | uniq -d
comm -12 <(pub slugs) <(priv slugs)          # must be empty
# every ref resolves in the merged set
comm -13 <(all slugs) <(all refs)            # must be empty
# every tag exists in tags.yaml; no old key survives
grep -rlE '^(label|kind):|video_url|thumbnail_url|children:|- type:' catalog   # must be empty
# counts
find catalog -name '*.yaml' | wc -l          # 107 public, 164 private
```

Then the real one: import into an empty database, export, re-import, and compare row and edge
counts — plus render all 17 sessions and assert every tree resolves to a leaf.

**How much is scriptable?** Steps 1-6 and all of C — that is every file touch except 46, and
those 46 are cut-and-paste plus 9 names and 10 judgement calls. Budget a day per tree for B
and an hour for A and C once the converter exists.

One warning about verification: the plan says "all 203 existing refs keep their shape". There
are 252 `ref:` lines and 178 distinct targets. Neither is 203, so a checklist cannot be
written against that number. Pick one and label it.

---

## 7. The five things I would want added

| # | What | Why it is first |
|---|---|---|
| 1 | **A one-page decisions sheet, before Monday**: the SQLite driver and its exact DSN; `SingularTable: true`; nullable columns are pointers; the ladder option; and a 10-line goose file template | Five of these are irreversible on SQLite, and four of them are not in §1.1's list of irreversible things |
| 2 | **A compiling phase-0/1 scaffold**: root `main.go` with the embeds and §1.5's order, `store/store.go`, `store/migrate.go` with `//go:embed migrations`, `newTestStore(t)`, and one page plus one fragment through `render.go` | It settles 1-A, 0-K, 1-C and 1-D by existing, and every later phase copies it |
| 3 | **The extracted template contract**: 329 `.Field` references grouped by template, and `CurrentStep`'s 20 fields written as a Go struct, with the command to regenerate both | It is the frontend's real contract, it already exists in the templates, and phase 3's worst failure mode is a timer that counts wrong instead of throwing |
| 4 | **`cmd/convert/` plus the §6C verification script** | 271 files, two repositories, and a format that will move while you convert |
| 5 | **`store/BEHAVIOUR.md` seeded from `db/*_test.go`, and `make test` / `test-postgres` / `pg-up` / `pg-down` / `backup`** | Phase 0's gate depends on a file that does not exist, and there is no `make test` and no CI test workflow at all |

---

## Appendix — every count, with its command

Run from `/home/awalvie/code/lamp` unless noted. `P=passion/catalog`, `V=passion-private-catalog`.

| Claim | Command | Result |
|---|---|---|
| 102 public / 123 private YAML files | `find $P -name '*.yaml' \| wc -l` ; same for `$V` | 102 / 123 |
| 88 + 96 movement files | per-directory `find … -maxdepth 1 -type f \| wc -l` | 88 / 96 |
| 184 movement `kind:` values | `grep -rhE '^kind:' $P $V --include='*.yaml' \| sort \| uniq -c` | climbing 57, reps_and_sets 36, session 56, timed_reps 35 |
| 24 menus | `grep -rhE '^ +children:' $P $V --include='*.yaml' \| wc -l` | 24 (public 3, private 21) |
| 22 inline blocks | `grep -rhE '^ +- type:' $P $V --include='*.yaml' \| sort \| uniq -c` | 18 activity, 2 warmup, 2 cooldown |
| 9 unnamed inline blocks | awk walk of `session_templates/*.yaml` printing each `- type:` item's name | 9 |
| 29 labels, 28 after collapsing | `grep -rhE '^label:' … \| tr ',' '\n' \| sort -u` | 29, incl. both `shoulder` and `shoulders` |
| `lead` is private-only | `comm -13 <(public labels) <(private labels)` | `lead` |
| 112 files with media, 120 items, 2 with no video | `grep -rlE '^media:'` ; `grep -rhE '^ *- (video_url\|thumbnail_url)'` | 112 / 120 / 2 |
| 3 `rung_seconds`, 1 `session_duration_seconds` | `grep -rn 'rung_seconds\|session_duration_seconds' $P $V` | 3 / 1 |
| 252 ref lines, 178 distinct targets | `grep -rhE '(^\| )ref:' … \| wc -l` ; `grep -rhoE 'ref: *"?[a-z0-9_]+' … \| sort -u \| wc -l` | 252 / 178 |
| Public tree is self-contained; no cross-tree slug collision | `comm -13 pub_slugs pub_refs` ; `comm -12 pub_slugs priv_slugs` | both empty |
| 50 templates, 26 defines, 329 field references | `find templates -name '*.html' \| wc -l` ; `grep -rhoE '\{\{ *define *"[^"]+"' templates \| sort -u \| wc -l` ; `grep -rhoE '\{\{[^}]*\}\}' templates \| grep -oE '\.[A-Z][A-Za-z0-9_]*' \| sort -u \| wc -l` | 50 / 26 / 329 |
| `run.html` 1,566 lines, 20 `CurrentStep` fields | `wc -l templates/run.html` ; `grep -ohE 'CurrentStep\.[A-Za-z0-9_]+' templates/run.html \| sort -u` | 1566 / 20 |
| embed cannot use `..` | `go doc embed` (go1.26.3) | "Patterns may not contain '.' or '..' or empty path elements" |
| No root `.go` file; one binary | `ls *.go` ; `find cmd -type f` | none ; `cmd/passion/main.go` |
| GORM pluralizes by default | `gorm@v1.31.1/schema/naming.go:44-49` | `SingularTable` false → `inflection.Plural` |
| air's real keys and defaults | `air@v1.65.3/runner/config.go:34-41,101-113,336-374` | `tmp_dir`, `[build] include_dir/exclude_dir/include_ext`; defaults `["go","tpl","tmpl","html"]` |
| No compose file, no goose/postgres/uuid/container dep | `find . -iname '*compose*' -o -iname 'Dockerfile*'` ; `grep -iE 'goose\|postgres\|modernc\|testcontainers' go.sum` | none ; none |
| 4 Make targets, no `test` | `grep -E '^[a-z_-]+:' Makefile` | run, build, watch, reseed |
| Deploy ships four directories, cgo on | `.github/workflows/deploy.yml` | `tar czf - catalog catalog-private templates static`; `CGO_ENABLED=1` |
