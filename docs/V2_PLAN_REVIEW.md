# V2 build plan — review

> **Snapshot, not the plan.** This records a review as it stood on its own date. Decisions
> have moved since — see `V2_PLAN.md` §1.10 (private trees as owned content, fork-on-edit,
> `source_tree`) and §1.11 (`content_key` replacing the log's text link). **Where this file
> disagrees with `V2_PLAN.md` or `SCHEMA_V2.sql`, those two win.**
>
> **This one file is not a clean snapshot.** Before adopting the rule above I edited it in
> place twice: `movement_kind: session` now reads `open`, and §3.1's menu paragraph was
> rewritten after review found two false claims in it. Both corrections also live in
> `V2_PLAN.md`, which is where they belong. No other review file was touched.

Reviewing [V2_PLAN.md](V2_PLAN.md) as it stands on 8 September 2026, against
[SCHEMA_V2.sql](SCHEMA_V2.sql), [SCHEMA_REVIEW_2.md](SCHEMA_REVIEW_2.md),
[STRUCTURE_PROPOSAL.md](STRUCTURE_PROPOSAL.md) and [STRUCTURE_REVIEW.md](STRUCTURE_REVIEW.md),
plus the live route table, the 50 templates and both catalog trees.

Fixed and not relitigated: Go, GORM, chi, HTMX, `html/template`; PostgreSQL and SQLite only;
`config/` `store/` `web/` plus `main`; build beside the old packages and delete them last;
goose owns migrations; old data is disposable; same feature set; one developer.

Every number below comes from a command shown beside it. Nothing is from memory. I did not
read `db/` or `pages/` beyond line counts, so nothing here judges the current implementation;
where a question needs the old importer's behaviour, I say that I could not check it.

---

## 1. Verdict

The phase order is sound and the wall between content and history is the right spine, but
this is not yet a plan you can start on, because four decisions it defers to later phases
have to be made **inside migration 001** and SQLite cannot take them back afterwards. The
blocking gap is the catalog format: the plan says "convert both trees to the new format" and
no such format exists, and writing it down surfaces three columns the schema does not have
(`d_prep_seconds`, a split rep/set rest, and a rung index) plus one table it does not have
(the run's menu choice). Write section 3 into the plan, settle section 3.2 before a line of
DDL, re-cut the phases per section 6, and then phase 0 is a good place to start.

---

## 2. What I measured

| Claim | Measured | Command |
|---|---|---|
| "225 slugs" | **225** — 184 movement files, 24 activity templates, 17 sessions | `grep -rhoE '^slug: "[^"]+"' catalog ../passion-private-catalog --include='*.yaml' \| sort -u \| wc -l` → 102 public + 123 private |
| "184 exercises" | **184** | Go walk of both trees, counting files under `exercises/` |
| "~12,000 lines of handlers" | **11,694** non-test | `find http/server -name '*.go' -not -name '*_test.go' \| xargs wc -l` |
| "50 templates" | **50** — 27 pages, 20 fragments, 3 layout | `find templates -name '*.html' \| wc -l` |
| "39 existing behavioural tests" | **unverifiable.** `db/` holds **130** `func Test` across 23 files; the whole repo holds **292** | `grep -rhoE '^func Test[A-Za-z0-9_]+' db/*_test.go \| wc -l` |
| Total Go being replaced | **20,543** non-test + **13,477** test | `find . -name '*.go' -not -name '*_test.go' \| xargs wc -l` |
| Registered route patterns | **94**, of which 7 are `{action}` catch-alls hiding **37** distinct actions | `grep -cE '^\s+(r\|pr)\.(HandleFunc\|Handle)\(' http/server/core.go` |

Two things that fall out of this and matter later.

**The plan's "12,000 lines of handlers" undercounts the rewrite by about 40%.** `pages/pages.go`
is 1,813 lines and 81 view structs, and `db/` is 6,170 non-test lines. All three are replaced.

**Column widths are safe.** Longest values across both trees: source 22, name 45, slug 43,
label 47, media URL 123, needs 55. Every one fits `SCHEMA_V2.sql` as written. Nothing to do.

---

## 3. The missing specifications

### 3.1 The catalog YAML format — the format itself

This is the biggest gap. Below is the whole format, meant to be pasted into the plan.

#### Layout

One file per content row. The directory gives `kind`.

```
catalog/
  tags.yaml                  the whole tag vocabulary
  movements/<slug>.yaml      kind = movement
  menus/<slug>.yaml          kind = menu
  blocks/<slug>.yaml         kind = block
  sessions/<slug>.yaml       kind = session
```

#### Rules that hold for every file

1. `slug` is required, is identity, and must equal the filename stem. The importer fails if
   it does not. A rename is then a `git mv` and is visible in review.
2. `name` is required and must be non-empty.
3. A shipped slug is unique **across all four kinds**. The importer fails on a collision.
   This is what lets a bare `ref:` name a target without naming its kind.
4. **No inline children.** Every content row is a file. This is the largest mechanical change:
   the trees today hold **27 inline exercises** (24 of them menus) and **22 inline blocks**,
   9 of which have no name at all. The conversion invents 22 block slugs, 24 menu slugs and
   9 block names, once, by hand.
5. `tags` is a list of tag slugs. Every one must exist in `tags.yaml`. An unknown tag is a
   hard failure, so a typo cannot silently mint `shouldres`.
6. Every number is optional. Absent means NULL, "not asked". `0` is a real value — 0 kg is
   bodyweight.
7. List order is `content_item.position`, 1-based, renumbered densely by the importer.
8. The importer upserts on `(kind, slug)` where `author_id IS NULL`, never deletes, and
   marks a slug that has left the tree as retired (see 3.2.4).

#### `tags.yaml`

The 29 distinct labels in the two trees become 29 rows. `shoulder` and `shoulders` both
appear today; they collapse to one.

```yaml
- slug: fingers
  name: Fingers
- slug: hangboard
  name: Hangboard
- slug: shoulder
  name: Shoulder
```

#### A movement file

| key | column | note |
|---|---|---|
| `name` | `content.name` | required |
| `slug` | `content.slug` | required, = filename stem |
| `movement_kind` | `content.movement_kind` | `reps_and_sets`, `timed_reps`, `climbing`, `open` |
| `source` | `content.source` | ≤ 64 chars |
| `tags` | `content_tag` | list of tag slugs |
| `notes` | `content.notes` | markdown block scalar |
| `sets` | `content.d_sets` | the default when nothing more specific says |
| `reps` | `content.d_reps` | |
| `weight_kg` | `content.d_weight_kg` | |
| `rep_seconds` | `content.d_rep_seconds` | |
| `rep_rest_seconds` | `content.d_rep_rest_seconds` | **column does not exist yet** — see 3.2.1 |
| `set_rest_seconds` | `content.d_set_rest_seconds` | **column does not exist yet** |
| `prep_seconds` | `content.d_prep_seconds` | **column does not exist yet** |
| `seconds` | `content.d_seconds` | whole-step duration |
| `media` | `content_media` | list of `{url, thumb_url}` |

`movement_kind: session` — today's fourth value, on **56 of 184** entries — is renamed
`open`. Keeping it would put the string `session` in both `content.kind` and
`content.movement_kind` meaning two different things.

Media keys are renamed to match the columns: `video_url` → `url`, `thumbnail_url` → `thumb_url`.
That is one sed across the **112** files that carry media.

#### A ref item — the same shape in every list

```yaml
- ref: weighted_pull_ups
  sets: 4
  reps: 5
  weight_kg: 10
  rep_seconds: 7
  rep_rest_seconds: 60
  set_rest_seconds: 180
  prep_seconds: 10
  seconds: 600
  notes: "cue for this use only"
  per_set:
    - reps: 5
    - reps: 3
    - reps: 3
```

Every numeric key maps 1:1 to a `content_item` column of the same name. `per_set` writes
`content_item_set` rows with `set_index` 1..n.

**The invariant SCHEMA_REVIEW_2 N9 asked for, made loud:** if `per_set` is present, `sets`
must be absent or equal to `len(per_set)`, and the importer fails otherwise. `content_item_set`
is the truth; `content_item.sets` is a cache of its length.

#### A menu file

```yaml
name: "Drills"
slug: drills_menu
pick: 1
tags: [technique, footwork, boulder]
notes: |
  One drill per session. Carry it through the whole warm-up ladder, easiest grades first.
options:
  - ref: silent_feet
  - ref: eyes_feet_hips_hands
  - ref: down_climbing
```

`pick` → `content.pick_count`, default 1. **Its meaning is "the fewest options you must
choose", not the most.** The min reading is right, and the "pick one or more of these" menu in
`pcc_go_hard.yaml` is the case that proves it.

**Corrected 2026-09-08.** Two claims above this line were wrong.

*"All 24 state their arity in prose."* They do not. **Three state nothing at all**: "Easy
campusing (optional)" and "Endurance Method" in `endurance.yaml`, "Lead Focus" in `lead.yaml`,
and "Endurance Method" in `pcc_do_more.yaml`. Those need a decision each, not a sed.

*"one 'choose a couple of the exercises' (`pick: 2`)."* That phrase is in
`exercises/paradigm/muscular_activation.yaml`, which is `kind: "session"` — a plain movement,
not a menu. **There is no pick-2 menu in either tree.** (That file is arguably a menu written
as a movement, which is a separate thing to catch during the conversion.)

**Optionality has nowhere to live.** Four menus put it in the name: "Wall Crawls (optional)",
"Strength (optional)", "Prehab (optional)", "Easy campusing (optional)". Under the min reading
`pick: 0` says "you may skip this", so state that 0 is legal and drop the four suffixes.
Otherwise the one fact the app could act on stays buried in a display name. Ranges need no key
today; add `pick_max` when a file wants one.
It also matches the handler that exists today, which already accepts a list —
`child_exercise_ids[]` in `handleRunExerciseChoose` at `http/server/runs.go:479`.

Verify: `grep -rhioE 'pick (one|two|a couple)[^.]{0,20}|choose (one|a couple)[^.]{0,20}' catalog ../passion-private-catalog --include='*.yaml'`

#### A block file

```yaml
name: "Synovial & Fascia Warm-Up"
slug: synovial_fascia_warm_up
block_kind: warmup            # warmup | main | cooldown, default main
source: "Adam Ondra"
tags: [warmup, mobility]
items:
  - ref: thumbs_up_down
  - ref: wrist_fist_rolls
  - ref: drills_menu          # a menu ref is spelled the same way
```

`block_kind` lives on the **block**, not on the edge. Today `type:` sits on the session's
activity list item, so it is per-use — but only the 22 inline blocks carry one (18 `activity`,
2 `warmup`, 2 `cooldown`), and none of the **49** `ref:`-ed blocks does. The plan must say
plainly: **the warmup/main/cooldown label is a property of the block, so the same block
cannot be a warm-up in one session and the main event in another.** No `content_item` column
carries it and none should be added.

Verify: `for f in catalog/session_templates/*.yaml ../passion-private-catalog/session_templates/*.yaml;
do awk '/^( *)- ref:/{r=1;next} /^ *- /{r=0} r && /^ *[a-z_]+:/{sub(/^ */,"");sub(/:.*/,"");print}' $f; done | sort | uniq -c`
— the only keys any `ref:` item carries today are `name` (6×) and one item's four numbers.

#### A session file

```yaml
name: "Boulder Session"
slug: boulder_session
color: "#ef4444"
needs: "bouldering wall"
tags: [boulder, power, technique]
blocks:
  - ref: warm_up
  - ref: drills
  - ref: boulder_projects_block
  - ref: antagonist_prehab
  - ref: cooldown_stretch
  - ref: journal
```

`needs` stays session-only and free text. Blocks and movements cannot declare equipment.
State that as an accepted limitation.

**A per-use block name is dropped.** Six block refs in three private session files carry a
`name:` beside the `ref:` — `strength_stretching.yaml` renames `mobility` to "Mobility — done
fresh, before the lifting", and two more rename a block to "Workout". `content_item` has no
`name` column and should not gain one: a block's name is its identity and history freezes it
into `log_entry.block_name`. The six become either a renamed block or a second block with its
own slug. Per-use *prose* still has a home — `content_item.notes`. (I did not read
`db/yaml_import.go`, so I cannot say whether the importer honours these six today; either way
they must be resolved by hand during the conversion.)

#### A full worked example — the hangboard ladder

This is the case the plan gets wrong. Verbatim from
`passion-private-catalog/exercises/bechtel/hangboard_ladder_full_crimp.yaml`:
`sets: 3`, `reps: 3`, `prep_seconds: 10`, `rung_seconds: "3,6,9"`, `rep_rest_seconds: 60`.

The rungs are **per rep, not per set**. `templates/run.html:998`, verbatim:

```js
if (rungArr.length > 0) return rungArr[(repNow - 1) % rungArr.length];
```

`content_item_set` is keyed `(content_item_id, set_index)`. There is no rep index, so
"ladders become per-set rows" is not expressible. Three sets of three rungs needs a
two-level key. Pick one of these **in phase 0**:

| Option | What it costs | What it buys |
|---|---|---|
| **A (recommended)** `rep_index INTEGER NOT NULL DEFAULT 1` on `content_item_set` and `log_set`; PK becomes `(content_item_id, set_index, rep_index)` and `ux_log_set` becomes `(log_entry_id, set_index, rep_index)` | two columns and two key changes in migration 001; impossible to add later without a SQLite table rebuild | the ladder is stored exactly as the player already renders it; per-rep planning works everywhere |
| **B** flatten to 9 sets of 1 rep, `seconds` 3,6,9,3,6,9,3,6,9 | the rung rest (60 s) and the round rest can no longer differ — `content_item_set` has no rest column | no schema change |
| **C** accept the loss on all 3 ladders and record it | 3 private entries change meaning | nothing to build |

With option A the file becomes:

```yaml
name: "Hangboard Ladder: Full Crimp"
slug: hangboard_ladder_full_crimp
movement_kind: timed_reps
source: "Logical Progression"
tags: [hangboard, fingers, strength]
sets: 3
reps: 3
prep_seconds: 10
rep_rest_seconds: 60
notes: |
  Our own words describing the ladder go here.
```

and the ladder shape lands on the **use**, inside the block that prescribes it:

```yaml
name: "Hangboard Ladders"
slug: hangboard_ladders
block_kind: main
items:
  - ref: hangboard_ladder_full_crimp
    sets: 3
    reps: 3
    per_set:
      - set: 1
        reps:
          - {rep: 1, seconds: 3}
          - {rep: 2, seconds: 6}
          - {rep: 3, seconds: 9}
```

That last nesting is the one place the format is not flat, and it exists only because a
ladder is genuinely two-dimensional. If you take option B or C, delete `per_set`'s inner
list and the format is flat everywhere.

#### Export must be written with the importer, not in phase 11

`http/server/export.go` writes this same YAML — it is the format's second implementation.
Today it emits **no `slug`** and **no `ref`**: it inlines every child (`exportActivityTemplate`
expands `childrenByParent` into `Children`). Under V2 that re-imports as duplicate movements,
because slug is identity. Two sentences the plan needs: **export emits `slug` on every row
and `ref:` for every child, so an export re-imports as the same rows**; and **the exporter
is written in phase 2 beside the importer, and only its route lands later.**

### 3.2 Four decisions the format forces into migration 001

SQLite cannot add a CHECK, a UNIQUE or a primary key to an existing table, which
`SCHEMA_V2.sql:557-559` already says. So these cannot wait for the phase that needs them.

| # | Missing from the schema | Evidence | What migration 001 should say |
|---|---|---|---|
| 3.2.1 | `content` has one `d_rest_seconds` and **no** `d_prep_seconds`, while `content_item` has all three | `grep -n 'd_rest_seconds\|prep' docs/SCHEMA_V2.sql`; `templates/run.html:789-791` reads `RepRestSeconds`, `SetRestSeconds` and `PrepSeconds` off `CurrentStep`; the trees carry `set_rest_seconds` 43×, `prep_seconds` 35×, `rep_rest_seconds` 17× at movement level | replace `d_rest_seconds` with `d_rep_rest_seconds`, `d_set_rest_seconds`, `d_prep_seconds`, so a movement's defaults mirror `content_item` exactly |
| 3.2.2 | `log_entry` freezes `t_sets, t_reps, t_weight_kg, t_rep_seconds, t_seconds` — no prep, no rest | same grep | either add `t_prep_seconds`, `t_rep_rest_seconds`, `t_set_rest_seconds`, or write down that **the player reads timings live from content while targets are frozen**, and accept that editing a template mid-run changes its timers. Decide; do not leave it silent |
| 3.2.3 | no rep index on `content_item_set` / `log_set` | 3.1's ladder | option A, B or C above |
| 3.2.4 | no retirement marker | `git log --diff-filter=D --name-only -- 'catalog/**'` shows `strength_base_session.yaml` deleted in `e527218` and 20+ files moved out in `3a0a6cd` | add `retired_on VARCHAR(10)` to `content`. The importer sets it when a shipped slug leaves the tree; the library hides retired rows; plans and logs keep resolving. This is SCHEMA_REVIEW_2 N8 and it has now happened twice |

Two more columns, cheap and nullable, so they can technically wait — but there is no reason
to make two migrations out of one:

- `account.time_zone VARCHAR(64) NOT NULL DEFAULT 'UTC'` (SCHEMA_REVIEW_2 N12). "Today" is
  the server's day right now: `http/server/time_helpers.go:41` computes `localDate(time.Now())`.
  Every heatmap, streak and calendar in phases 7 and 8 depends on it.
- `account.token_epoch INTEGER NOT NULL DEFAULT 0` — see 3.3.
- `account.max_pull_ups INTEGER` and `account.max_hang_kg NUMERIC(5,2)`. Both are on the
  profile form today (`templates/profile.html:79-85`) and have no column anywhere in the 19
  tables. This is SCHEMA_REVIEW_2 D5's "hole", still open. Phase 10 hits it.

### 3.3 Auth — keep stateless JWT, and add one column

Today: HS256 JWT in a cookie named `passion_auth`, TTL from config, refreshed when less
than half remains, `Secure` unless `InsecureCookies`, and CSRF by Origin match
(`http/server/auth.go:19,246,281`; `csrfMiddleware` at `:193`).

**Keep it. Do not add a session table.** Reasons, in order of weight: the schema has no such
table and adding one buys a per-request read plus an expiry sweep; `SetMaxOpenConns(1)` on
SQLite makes that read contend with every write; the app has one real user and will have
few; and logout already works by clearing the cookie.

What the plan must add either way:

1. `account.token_epoch INTEGER NOT NULL DEFAULT 0`, carried as a JWT claim and compared on
   every request. Password change bumps it. **Without this, "change password" — which the
   plan lists in step 1.3 — does not end the old sessions, and the default TTL is 30 days.**
   That is the one thing stateless JWT cannot do, and one integer fixes it.
2. The cookie contract, written down: `HttpOnly`, `SameSite=Lax`, `Secure` unless
   `InsecureCookies`, `Path=/`, `MaxAge` = TTL, HS256 only with an explicit `alg` check.
3. CSRF stays an Origin / `Sec-Fetch-Site` check on unsafe methods. **No CSRF token**, so
   nothing needs server-side storage and `BaseParams` needs no token field.
4. Keep the existing secret validation — the placeholder list and the 32-character minimum
   in `config/config.go` are already right and cost nothing to carry over.

### 3.4 Config — what lives, what dies, what is new

| Setting today | V2 |
|---|---|
| `Server.Addr` | keep |
| `Server.DBPath` | **dies**, replaced by `Database.Engine` + `Database.DSN` |
| `Server.Seed` | keep as a dev-only flag, not a config key |
| `Server.DemoOwnerID` | **dies** — no owner sentinel exists any more |
| `Auth.JWTSecret`, `Auth.JWTTTLHours` | keep, with the existing validation |
| `Auth.DevAuthBypass` | keep, and refuse to boot if it is set while `Engine = postgres` |
| `Auth.InsecureCookies` | keep |
| `YAMLImport.Enabled` | becomes `Catalog.Import`, default **true** — the import is idempotent and ownerless now, so there is nothing to protect against |
| `YAMLImport.ExercisesDir` / `SessionTemplatesDir` / `ActivityTemplatesDir` | **die.** One directory per kind is a consequence of the old three-table split. `catalog/` is embedded in the binary |
| `YAMLImport.OwnerID` | **dies.** `author_id IS NULL` is shipped content |

New:

| Setting | Default | Why |
|---|---|---|
| `Database.Engine` | `sqlite` | `sqlite` or `postgres`; anything else refuses to boot |
| `Database.DSN` | `passion.db` | SQLite path or Postgres URL. The pragmas are appended by `store.Open`, never by the user |
| `Database.MaxOpenConns` | 0 (auto) | forced to 1 on SQLite regardless of what is set |
| `Migrate` | `up` | `up` or `off`. See 3.5 |
| `Catalog.Dirs` | empty | extra on-disk trees merged before import, refs resolving across all of them. **This is how the private catalog is loaded**, because a separate repository cannot be `//go:embed`-ed from this one |
| `Log.Level` / `Log.Format` | `info` / `json` | today both are hard-coded in `main.go` |
| `Defaults.TimeZone` | `UTC` | the value new accounts get |

### 3.5 When migrations run

The plan says goose owns the schema and never says when it runs. Say this:

- **At boot, by default.** `Migrate: up` applies every pending migration before the listener
  opens. One binary, one systemd unit, no second command for the self-hoster to forget.
- `Migrate: off` skips it, for the case where someone wants to run the migration separately.
- `--migrate-status` prints the current and target version and exits. `--migrate-only`
  applies and exits.
- **On a version mismatch the binary refuses to boot** when the database is *newer* than the
  embedded migrations. That is a rolled-back binary meeting a migrated database, and running
  it would write rows the old code cannot read. The message names both versions.
- **Never automatically down.** Migrations are append-only after the first boot. If phase 5
  needs a column, it is migration 002.
- Two dialect directories, one numbering: `store/migrations/sqlite/` and
  `store/migrations/postgres/`. `SCHEMA_V2.sql:9-12` names identity columns as the one
  construct that cannot be spelled the same. goose's dialect is a process-wide setting and a
  `.sql` migration has no per-dialect branch, while `goose.Up(db, dir)` takes the directory,
  so two directories with one numbering is the mechanism. `goose.SetBaseFS(embedded)` plus `goose.SetDialect(engine)` plus the matching
  directory. A test asserts the two directories hold the same version numbers.
- The self-hoster's story, in the readme: put a DSN in `passion.yaml`, start the binary, it
  migrates itself. Back up the SQLite file before an upgrade; there is no down migration.

### 3.6 What `main.go` does, in order

The plan never says. It should:

1. Parse flags. `-config`, `-migrate-status`, `-migrate-only`, `-import-catalog`, `-seed`,
   `-version`. **16 of today's 17 flags die** — every `-purge-*`, `-backfill-*`,
   `-publish-catalog`, `-unpublish-catalog`, `-delete-users-except`, `-purge-ghost-catalog`,
   `-mint-invites`, `-list-invites`, `-invite-note`, `-i-have-a-backup`, `-exit-after-seed`.
   They exist to repair data that is being thrown away. Say so, and update `make reseed`,
   which calls `--exit-after-seed`.
2. Load config, apply env overrides, validate. Exit non-zero with the offending setting named.
3. Set up `slog` from `Log.Level` / `Log.Format`.
4. `//go:embed templates static catalog` — in `main.go`, because `embed` patterns cannot
   contain `..` and the repo root is the only package that sees all three.
5. `store.Open(ctx, cfg.Database)` — dialect switch, DSN pragmas, pool limits.
6. Run migrations per 3.5, or check status and exit.
7. `store.ImportCatalog(ctx, embeddedFS, cfg.Catalog.Dirs)` when `Catalog.Import`.
8. Compute the static asset checksum for cache-busting.
9. `web.New(embeddedTemplates, embeddedStatic, checksum, store, cfg)` — parse templates once,
   build the router.
10. Warn once for each footgun that is on: `InsecureCookies`, `DevAuthBypass`.
11. Start the `http.Server` with the existing 15 s / 60 s / 120 s timeouts.
12. `signal.Notify` on SIGINT and SIGTERM; `Shutdown` with a 10 s context; close the store.

There are no background jobs. `STRUCTURE_REVIEW.md` item 5 assumed scheduled sessions need a
ticker; they do not — `scheduled` rows are written when a cycle is created and read on demand.
Say that explicitly so nobody builds a job runner.

### 3.7 Errors, logging, observability

The plan says nothing. Six rules, all checkable by reading a signature or a log line:

1. **No `store/` signature names an HTTP type, and no error it returns carries a status code
   or a user-facing message.** Store returns `gorm.ErrRecordNotFound`, `gorm.ErrDuplicatedKey`
   (via `TranslateError: true`) and its own sentinels — `store.ErrNotYours`, `store.ErrShipped`.
2. **One function maps error to status and prose**, in `web/render.go`. Not found → 404,
   duplicate → 409, not yours → 404 (never 403 — a 403 confirms the row exists), anything
   else → 500.
3. **Every error is handled exactly once.** A function either logs it or returns it, never both.
4. **A request id middleware** puts a short id in the context, in the request log line, and on
   the 500 page, so the user can quote it.
5. **One log line per request**: method, path, status, duration_ms, account_id, request_id.
   4xx logs no body. 5xx logs the error with the request id. Panic recover logs the stack.
6. **`GET /healthz`** returns 200 and the schema version. GORM's logger goes to `slog` and
   warns on any query over 200 ms.

### 3.8 There is no runnable binary until phase 12

`main.go` switches to the new stack in phase 12, and `templates`, `static` and `catalog`
are embedded there. But the phase 1 gate is "two accounts sign up and log in, **in a real
browser**". Nothing can serve a browser until phase 12.

**Add to phase 1: `cmd/passionv2/main.go`, deleted in phase 12.** It carries the embeds and
the startup order from 3.6, and it is what every browser gate in phases 1 through 11 is run
against. Without it, every gate that says "in a browser" is untestable and quietly becomes an
`httptest` assertion.

### 3.9 The run's menu choice has no table and no phase

`SCHEMA_V2.sql` gives a menu a slug and a `pick_count` and then stops. There is no place to
record **which option the runner chose**. Today there is a table for it:
`handleRunExerciseChoose` (`http/server/runs.go:458-573`) deletes and re-inserts
`db.RunExerciseChoice` rows keyed by run and parent, and the choice is re-editable.

This is not an edge case. **Three of the four public sessions contain a menu** —
`boulder_session` refs `drills` and `cooldown_stretch`, `fingers_strength` refs `wall_crawls`,
`mobility_day` refs `cooldown_stretch`. Only the Emil fingerboard session is menu-free. So
phase 3 cannot run a shipped session end to end without deciding this.

The gap is sharper than a missing table, because `log_entry.movement_slug` is `NOT NULL` and
`SCHEMA_V2.sql:506` says targets are "copied into `log_entry.t_*` at the moment the run is
created". An unresolved menu cannot be written at run start.

**What the plan should say.** A menu is frozen at run start as one `log_entry` with
`movement_kind = 'menu'`, `movement_slug` = the menu's slug and `state = 'pending'`. When the
runner chooses, that row is deleted and one row per chosen movement is inserted at the same
`position`, with its targets resolved **at choose time**, not at run start. History then shows
what was done and the menu disappears from it. Two consequences to write down beside it:
`position` is not unique on `log_entry`, so every read is `ORDER BY position, id`; and the
sentence about freezing at run creation gains the exception.

### 3.10 Features the plan removes without saying so

The constraint is "same feature set, no new features". Removal is a feature change too, and
three of these are removals by omission.

| Feature today | Evidence | Status in V2 |
|---|---|---|
| Invite-only signup | `http/server/auth.go:111-155`, a `signup_invite` table, `--mint-invites`, `--list-invites` | Plan step 1.3 says "open signup, no invite codes" — and step 1.4 says "reuse `templates/login.html` and `signup.html` as they are", while `signup.html:15-19` carries an invite field and the line "Passion is invite-only for now". **The two steps contradict each other.** Decide, and if signup opens, say that the invite table and the two flags are dropped on purpose |
| `save-as-mine` on all three kinds | `templates.go:279`, `activity_template_handlers.go:104`, `exercise_library.go:248` | **Drop.** Fork-on-edit replaces it. The three routes go |
| `reset-catalog` on all three kinds | `templates.go:299`, `activity_template_handlers.go:137`, `exercise_library.go:338` | **Keep**, re-implemented as "delete your fork", which needs `forked_from_id` to survive — see section 5 |
| Per-use warmup/cooldown labelling | `type:` on a session activity | **Lost by design.** `block_kind` is on the block. Say so |
| Menus | 24 in the trees | **Unspecified.** See 3.9 |
| Retirement of a shipped slug | happened in `e527218` and `3a0a6cd` | **Unspecified.** See 3.2.4 |

---

## 4. Gate by gate

The phase 3 gate is the model: it names an action, an observation and a destructive check.
Six of the other gates are not testable as written.

| Phase | Gate as written | Real test? | Rewrite |
|---|---|---|---|
| 0 | "All 39 green on SQLite and on Postgres. Two consecutive boots against one catalog file change no row count. `AutoMigrate` is not used" | **Half.** "39" matches nothing measurable; `db/` holds 130 test funcs. "AutoMigrate is not used" is unfalsifiable as prose | `go test ./store/... -count=1` passes with `PASSION_TEST_POSTGRES` set and unset, and the named list of ported tests is in `store/BEHAVIOUR.md`. `grep -rn AutoMigrate store/` returns nothing — as a test, not a promise. `goose status` reports the same version on both engines. Boot twice against a temp DB; `SELECT count(*) FROM content, content_item, content_item_set, content_media, content_tag` is identical |
| 1 | "Two accounts sign up and log in, in a real browser. Deleting one removes all of its data and leaves the other untouched" | **No** — nothing serves a browser until phase 12 (3.8). The cascade half is real | Add 3.8's `cmd/passionv2`. Then: a `httptest` test signs up two accounts, writes a row in every account-owned table for each, deletes one, and asserts `count(*) = 0` for that account in all 9 tables that reference an account directly — `body_measurement`, `grade_milestone`, `content.author_id`, `plan`, `scheduled`, `calendar_event`, `place`, `log`, `log_entry` — and unchanged for the other. Browser check is a manual smoke, named as such. Add: changing account A's password rejects A's old cookie and leaves B's working |
| 2 | "Two accounts both see all 184 exercises. Editing a shipped session forks it and leaves the original intact. Two boots change nothing. One exercise is one row no matter how many sessions use it" | **Half.** 184 needs the private tree converted, which risk row 1 defers until after the public one — the gate cannot pass when the phase ends. "Editing a shipped session" has no editor until phase 5 | Split. (a) `SELECT count(*) FROM content WHERE kind='movement' AND author_id IS NULL` = 88 after the public tree, 184 after the private one. (b) Fork is a **store** test at this phase: `ForkSession` leaves the original row byte-identical, copies its blocks and edges, keeps the slug, sets `forked_from_id`, and creates zero new `movement` rows. (c) Import twice; every table's checksum is unchanged. (d) `SELECT count(*) FROM content WHERE kind='movement' AND slug='silent_feet'` = 1 while `SELECT count(*) FROM content_item WHERE child_id=(that id)` > 1. (e) Import a tree with a duplicate slug across kinds and an unknown tag; both fail loudly |
| 3 | "Run a session, log sets against a target, see the disparity in history. Then drop every content table and confirm history still renders correctly. Rename a session and a gym, and confirm the old record is unchanged" | **Yes.** Keep it verbatim. It is the best gate in the plan | Two additions, because as written it cannot exercise the five-source order: insert a `plan` and `plan_target` rows directly in the test and assert each of the five sources wins in turn; and run a session containing a menu and a climbing movement, since three of four shipped sessions have a menu and 59 of 184 movements are `climbing` |
| 4 | "A fourth package, or a store method that can only return a page-shaped type, means the three-package split is wrong" | **No.** Nothing is measured; the four questions are answered by opinion | Make it a script. `find store web -name '*.go' \| xargs wc -l` printed per file, with the ~1,200-line trigger from `STRUCTURE_REVIEW.md` §3 named. `grep -nE 'func \(s \*Store\).*\) \(.*(Page\|View\|Dashboard\|Summary)' store/*.go` returns nothing. And **the real question cannot be answered at phase 4** — the page-shaped read arrives with the dashboard, which is phase 7. Add a second checkpoint there |
| 5 | none | **Missing** | Reorder a block's items and assert `content_item_set` rows survive — the rewrite is `UPDATE`-only, never delete-and-insert, because the cascade would take the planned sets (SCHEMA_REVIEW_2 N10). Editing a shipped block forks the block and not its movements. `sets` equals `count(content_item_set)` after every save, add, delete and clear |
| 6 | none | **Missing** | The five-source order, through the UI this time: set a whole-cycle target, then a week-3 target touching only `reps`, and assert week 3 shows the new reps and the cycle's sets. Two whole-cycle rows for one movement are refused by `ux_target`. Creating the same cycle twice does not double `scheduled` — `SCHEMA_V2.sql:310` makes that the app's job, so it needs a test |
| 7 | none | **Missing** | The dashboard renders with 0 store calls that return a screen-shaped type. `/dashboard`, `/calendar` and `/history` each answer in under 200 ms against a database with 500 logs and 5,000 log entries, and the query count per page is asserted, not eyeballed |
| 8 | none | **Missing** | Heatmap, streak, pyramid and send rate each computed from a fixture of known logs, with the expected numbers written in the test. A `draft` log appears in none of them |
| 9 | none | **Missing** | A manual entry backdated to the 1st shows the 1st on the log page *and* in the exercise history — the bug the composite `ON UPDATE CASCADE` exists to prevent. An open session with per-set planning saves and reloads. A trial run writes no `scheduled` row |
| 10 | none | **Missing** | Rename a gym; every past log line still shows the old name. Delete a gym; the name stays and `place_id` is NULL. A tick with no grade is accepted and does not appear in the pyramid |
| 11 | none | **Missing** | Export a session, re-import it into an empty database, and assert the row and edge counts match the original. That is the only test that proves the format round-trips |
| 12 | none | **Missing** | `grep -rn 'passion/db\|passion/pages\|http/server' --include='*.go' .` returns nothing. `go build ./...` and the full suite pass. `cmd/passionv2` is gone |
| 13 | none | **Missing** | Every config key in the readme table exists in `config.Config`, asserted by a test that reflects over the struct. Every Make target runs |

---

## 5. Open schema defects the plan does not schedule

From `SCHEMA_REVIEW_2.md`, only the ones still open in `SCHEMA_V2.sql`. Nine of its fourteen
new defects are genuinely fixed — N1's key, N3's composite FK with `ON UPDATE CASCADE`, N4's
enumerated pairs (the `menu` kind makes a cycle structurally impossible, which is better than
what was asked for), N6's frozen `place_name`, N11 by choosing goose, and most of N7's indexes.
These are not.

| Defect | Still open because | Phase that must own it | Plan today |
|---|---|---|---|
| **N1, second half** — `varies_by_week` has no home | The schema derives it from "a `week > 0` row exists", so the toggle cannot be on before a value is typed. The old code comment at `training_cycle_overrides.go:174-185` says why a bare flag row must exist | 6 | **ignored.** Fix in the phase, not the schema: the toggle inserts one all-NULL `plan_target` row per week, which `ux_target` permits. Write that down |
| **N2, second half** — `forked_from_id` is a bare single-column FK, `ON DELETE SET NULL` | `SCHEMA_V2.sql:94`. A fork can descend from a different kind, and "reset to default" loses its target if the shipped row goes | 2 | **ignored.** Either make it composite — `FOREIGN KEY (forked_from_id, kind) REFERENCES content (id, kind)`, which pins the kind for free — or accept it and test that reset fails safely |
| **N3, second half** — the progression query does not filter the log's state | `SCHEMA_V2.sql:511-518` shows the query joining nothing, so a `draft` manual entry appears in the "last five times" hint | 3 | **ignored.** Either freeze the log's state onto `log_entry`, or join `log` and say the index no longer answers it alone. Do not leave the comment claiming both |
| **N5** — `scheduled` has no natural key | `SCHEMA_V2.sql:310` decides "duplicate prevention is the app's job" | 6 | **ignored.** A cycle writes every week's rows in one loop, so this is the exact shape of the doubling bug. It needs the upsert *and* a test, named in the phase |
| **N7, partial** — two indexes | `content (author_id, kind, name)` for the library list, and nothing on `(account_id, state)` for the log | 2 and 8 | **ignored.** Both are one line in migration 001 |
| **N8** — the importer has no retirement | no `retired_on` | 0 and 2 | **ignored.** See 3.2.4 |
| **N9** — `content_item.sets` vs `content_item_set` rows; `log_entry.set_mode` vs `log_set` rows | Both pairs are in the schema with no stated truth | 2, 5 and 9 | **ignored.** See 3.1's `per_set` rule; for `set_mode`, write the invariant "`set_mode='simple'` implies zero `log_set` rows" and enforce it in the one write path |
| **N10** — the sibling rewrite must be `UPDATE`-only | `SCHEMA_V2.sql:30-32` says reorders rewrite siblings and does not say how | 5 | **ignored.** With `content_item_set` a child of `content_item`, delete-and-insert takes the planned sets through the cascade |
| **N12** — no per-account time zone | no column | 0 | **ignored.** See 3.2 |
| **N13** — ownerless catalog reverses two shipped commits | `SCHEMA_V2.sql:87-89` makes `author_id IS NULL` shipped, which is what `0393672` and `5d5fde8` on master were written to refuse and clean up | 2 | **ignored, and it is the one that needs your sign-off.** The schema's reason is sound and better than the old one — nothing owns the catalog, so no deletion can reach it, and the import needs no account to exist. But it reverses a decision made three commits ago with evidence. The plan should say, in one sentence, that V2 overrides those commits and why |
| **D5 hole** — `max_pull_ups`, `max_hang_kg` | on the profile form, in no table | 0 | **ignored.** See 3.2 |
| **D14d** — slug normalization in Go | not stated | 2 | **ignored.** One sentence: slugs are lowercase ASCII, `[a-z0-9_]`, normalized before every read and write, same as email |

---

## 6. Corrected phase order

The order is mostly right — content before the log is correct, and the log freezing content
is what makes phase 3's gate possible. Four things are genuinely out of order, and four
phases are two phases each.

**Out of order:**

1. **The four migration-001 decisions are in phase 2 and must be in phase 0** (3.2). SQLite
   cannot add a CHECK, a UNIQUE or a PK later. The plan's own step 0.3 says so and then
   defers the decisions that depend on it.
2. **The runnable binary is in phase 12 and must be in phase 1** (3.8). Otherwise no gate
   from 1 to 11 that says "in a browser" can run.
3. **Export is in phase 11 and belongs in phase 2** (3.1). It is the format's second
   implementation and the only thing that proves the format round-trips.
4. **Climbing ticks are in phase 10 and the run player is in phase 3.** `boulder_session` is
   a shipped public session and its third block holds `boulder_projects`, `kind: climbing`.
   59 of 184 movements are climbing. Phase 3 needs the `log_climb` **store** writes; only the
   tick UI belongs in 10.

**Phases that are two phases:**

| Phase | Split into |
|---|---|
| 2 | **2a** the format spec, `tags.yaml`, the importer, the exporter, and the public tree (102 slugs). **2b** the private tree (123 slugs), `content.go`, and the library pages |
| 5 | **5a** the movement library editor. **5b** blocks and sessions, which is where the reorder and per-set editors live |
| 6 | **6a** cycles and scheduling. **6b** the Exercise targets page, which is the five-source resolution order with a UI on it |
| 9 | **9a** manual log entry. **9b** open sessions and trial runs |

**The corrected order.** 17 phases:

```
0   foundations + all of 3.2's migration-001 decisions
1   accounts, web skeleton, middleware, render, cmd/passionv2
2a  catalog format + importer + exporter + public tree
2b  private tree + content.go + library pages
3   run and log — including menus, climbing writes, and the five-source resolver
4   structure checkpoint (narrowed to the two questions phases 0-3 can answer)
5a  movement library editor
5b  blocks and sessions: building, editing, reorder, per-set
6a  cycles and scheduling
6b  the Exercise targets page
7   dashboard and calendar
7c  second structure checkpoint — the page-shaped-read rule, which only the dashboard tests
8   full history
9a  manual log entry
9b  open sessions and trial runs
10  ticks UI, venues and boards, profile and body stats
11  markdown preview and the remaining fragments
12  switch main.go, delete db/, http/server/, pages/, cmd/passionv2
13  readme and DEVELOPMENT.md
```

**Say plainly: 13 phases is the wrong count.** Not because the work is different, but because
four of the thirteen are two phases wearing one row, and phase 4's real question cannot be
answered where it sits.

---

## 7. Route accounting

94 registered patterns in `http/server/core.go`, of which 7 are `{action}` catch-alls hiding
37 distinct actions. Counts below are registered patterns; the actions are named where they
change phase.

| Prefix | Patterns | Phase | Notes |
|---|---|---|---|
| `/`, `/login`, `/signup`, `/logout` | 4 | **1** | `/signup` keeps or drops the invite field per 3.10 |
| `/static/*` | 1 | **1** | changes from `http.Dir` to the embedded FS with a cache-busting checksum. The plan never mentions it |
| `/exercise-library`, `/exercise-library/new`, `/exercise-library/{id}/{action}` | 5 | **2b** list and detail, **5a** the editor | actions: `edit`, `update`, `delete`, `save-as-mine` (**drop**), `reset-catalog` (keep as "delete your fork"), `export` (**move to 2a**) |
| `/exercise-library/{id}/history` | in the 5 above | **8** | |
| `/templates`, `/templates/new`, `/templates/{id}/{action}[/{sub}]` | 4 | **5b** | actions: `edit`, `activities`, `update`, `delete`, `save-as-mine` (**drop**), `reset-catalog`, `export` (**2a**), `trial` (**9b**) |
| `/activity-templates` × 4 | 4 | **5b** | actions: `edit`, `update`, `delete`, `save-as-mine` (**drop**), `reset-catalog`, `export` (**2a**), `exercises` + `reorder` + `from-library` |
| `/activities/{id}/{action}[/{sub}]` | 2 | **5b** | actions: `exercises`, `delete` |
| `/exercises/{id}/{action}` | 8 | **5b** | `update`, `delete`, `planned-sets` ×4. `history-hint` → **3**, `history-popup` → **8**, `divergence-hint` → **3** |
| `/training-cycles` × 7 | 7 | **6a** / **6b** | 7 sub-actions: `move`, `add`, `remove`, `override-save`, `override-clear`, `details-save`, `delete`. `week-override-save` and `week-override-toggle` → **6b** |
| `/scheduled-sessions` × 2 | 2 | **6a** | 3 actions: `preview`, `start`, `move` |
| `/calendar`, `/calendar-events` × 3 | 4 | **7** | |
| `/dashboard`, `/dashboard/start-template`, `/fragments/start-session-picker` | 3 | **7** | |
| `/history` | 1 | **3** list, **8** stats | the one route the plan splits across two phases without saying so |
| `/runs/*` | 23 | mixed | `/runs/{id}` and `.../complete`, `.../skip`, `/stop`, `/delete`, `/summary`, `/journal`, `/session-notes` → **3**. `/runs/{id}/exercises/{id}/choose` → **3** (3.9). `/runs/open` and the 10 `/runs/{id}/open/*` → **9b**. The 4 `.../ticks*` → **3** store, **10** UI |
| `/training-log/*` | 18 | **9a** | `/training-log`, `/new`, `/quick`, `/for-run/{id}`, `/{id}`, `/{id}/edit`, `/{id}/delete`, and the 11 `/draft/{id}/*` |
| `/profile*` | 7 | **10** | `/profile`, `/profile/password` (**1**, since the plan puts password change in 1.3), `/profile/venues` ×3, `/profile/boards` ×2 |
| `/preview/markdown` | 1 | **11** | |

**Orphaned by the plan:** none, once phases 5–11 are read generously — but `/static/*`,
`/profile/password`, `/history`'s split, and the 37 hidden actions are named in no phase, and
the four `save-as-mine`/`export` reassignments above are decisions the plan has not made.

**To drop, not rebuild:** the 3 `save-as-mine` routes (fork-on-edit replaces them). Nothing
else. Every other route maps to a feature that exists.

---

## 8. Template accounting

50 files. `templates/layouts/base.html` and its two fragments are needed by every page from
phase 1 onward.

| Phase | Templates |
|---|---|
| **1** | `login.html`, `signup.html`, `layouts/base.html`, `layouts/fragments/topbar.html`, `layouts/fragments/footer.html` |
| **2b** | `exercise_library.html` |
| **3** | `run.html`, `run_summary.html`, `fragments/run_summary.html`, `fragments/run_ticks.html`, `fragments/run_ticks_readonly.html`, `fragments/exercise_history_hint.html`, `fragments/exercise_divergence_hint.html`, `fragments/journal_form.html`, `history.html` (list only) |
| **5a** | `new_exercise_library.html`, `edit_exercise_library.html` |
| **5b** | `templates.html`, `new_template.html`, `template_edit.html`, `activity_templates.html`, `new_activity_template.html`, `activity_template_edit.html`, `fragments/activities_container.html`, `fragments/exercises_container.html`, `fragments/activity_template_exercises_container.html`, `fragments/planned_sets.html`, `fragments/preview_container.html` |
| **6a** | `training_cycles.html`, `new_cycle_guided.html`, `training_cycle_detail.html`, `fragments/scheduled_session_preview.html` |
| **6b** | `cycle_targets.html` |
| **7** | `dashboard.html`, `calendar.html`, `fragments/schedule_session_picker.html`, `fragments/start_session_picker.html` |
| **8** | `history.html` (stats), `exercise_history.html`, `fragments/exercise_history_popup.html` |
| **9a** | `training_log.html`, `training_log_new.html`, `training_log_quick.html`, `training_log_summary.html`, `fragments/manual_exercises.html` |
| **9b** | `open_session.html`, `fragments/open_exercise_panel.html`, `fragments/open_template_panel.html` |
| **10** | `profile.html`, `fragments/venues_list.html`, `fragments/boards_list.html` |
| **11** | — the markdown editor is `static/markdown-editor.js`, not a template |

**Templates no phase in the plan reaches**, in order of how much it costs:

| Template | Why it is orphaned |
|---|---|
| `layouts/base.html`, `layouts/fragments/topbar.html`, `layouts/fragments/footer.html` | Phase 1.4 says "reuse `templates/login.html` and `signup.html` as they are". Neither renders without the layout, and `topbar.html` holds the nav for every route, so it changes in almost every phase. Name it in phase 1 and say it grows as routes land |
| `fragments/run_ticks.html` (534 lines), `fragments/run_ticks_readonly.html` | Ticks are phase 10; the run player is phase 3. See section 6 item 4 |
| `fragments/exercise_history_hint.html`, `fragments/exercise_divergence_hint.html` | The run screen's two live hints. Phase 3 says only "the player, reusing `run.html`". The divergence hint is the one that gets *easier* in V2 — `log_set` holds asked and done on one row, so it stops reading across the wall |
| `fragments/exercise_history_popup.html` | Phase 8's only unnamed piece |
| `fragments/schedule_session_picker.html`, `fragments/start_session_picker.html`, `fragments/preview_container.html`, `fragments/scheduled_session_preview.html` | "The remaining fragments" in phase 11 is the only row they could land in, which puts the dashboard's session pickers four phases after the dashboard |

**`run.html` deserves its own line in the plan.** It is 1,566 lines, of which **1,021 are
inside `<script>`**, and lines 789-793 read `RepRestSeconds`, `SetRestSeconds`, `PrepSeconds`
and `RungSeconds` straight off `CurrentStep`. Three of those four change shape in V2. "Reusing
`run.html`" is a phase of its own, not a step.

---

## 9. Relative sizing

Handler lines are non-test, measured per file and grouped by the phase that owns them.

| Phase | Handler LOC it replaces | Size | Underestimated? |
|---|---|---|---|
| 0 | — (19 structs, ~560 lines of DDL × 2 dialects, `store.go`, the ported behaviour suite) | **L** | **Yes.** The plan reads as a warm-up. It is where every irreversible decision lives |
| 1 | 340 auth + 214 core + 178 helpers, plus `render.go` from a 1,813-line `pages.go`, plus `cmd/passionv2` | **L** | **Yes** if 3.8 is included, M if not |
| 2a+2b | — (the importer is 1,330 lines today; 274 files converted by hand; the exporter) | **XL** | **Yes, badly.** Four table rows for the phase the plan itself calls "the heart of the redesign" |
| 3 | 844 runs + 388 run_actions + 151 run_journal + 504 library + 755 ticks (store half) | **XL** | **Yes.** Plus `run.html`'s 1,021 lines of JavaScript and the unbuilt menu flow |
| 4 | — | **S** | No |
| 5a+5b | 822 exercise_handlers + 574 templates + 456 activity_templates + 249 activities + 137 catalog_edited + 25 color = **2,263** | **XL** | **Yes.** One table row. `fragments/exercises_container.html` alone is 1,452 lines |
| 6a+6b | 1,087 training_cycles + 397 overrides + 186 scheduled = **1,670** | **XL** | **Yes.** The largest single handler file in the repo, plus the five-source resolver's UI |
| 7 | 476 dashboard + 142 calendar + 168 calendar_events = **786** | **L** | No |
| 8 | 492 history + 250 exercise_history = **742** | **L** | Slightly — four separate statistics, each needing a fixture |
| 9a+9b | 874 training_log + 661 manual + 635 open_session = **2,170** | **XL** | **Yes.** One table row for 19% of the handler code |
| 10 | 755 ticks (UI half) + 181 profile = **936** | **L** | No |
| 11 | 382 export + 53 markdown + 73 youtube = **508** | **M**, or **S** once export moves to 2a | No |
| 12 | 582-line `main.go`, 17 flags to triage, three packages to delete | **M** | Slightly |
| 13 | readme + DEVELOPMENT.md | **M** | No |

**The number that matters.** Phases 5 through 11 own **9,075 of the 11,694 handler lines —
78%** — and the plan describes them in seven table rows of one line each. Phases 0 through 4
get 42 lines of detail for the remaining 22%. The plan is inverted: the detail is where the
thinking already happened, and absent where it has not.

Verify: `find http/server -name '*.go' -not -name '*_test.go' | xargs wc -l | sort -rn`.

---

## 10. Missing risks

The five rows in the plan's table are real. These are missing, in the order I would bet on
them hurting.

| Risk | Why it is not covered | What to do about it |
|---|---|---|
| **The run player's JavaScript** | The plan's template row says "each phase touches only the templates it needs", which is right for HTML and wrong for `run.html`: 1,021 lines of timer state machine whose four inputs change shape. A mistake here does not throw — the timer just counts the wrong number of seconds, mid-session, on a phone | Freeze `CurrentStep`'s field list as a contract in phase 3 before touching the template, and add one test per timer phase (prep, work, rep rest, set rest) asserting the seconds |
| **cgo** | `go.mod` pins `github.com/mattn/go-sqlite3 v1.14.22`, so the build is cgo. The plan's step 0.4 lists `_busy_timeout`, `_foreign_keys=on`, `_txlock=immediate` — mattn's parameter names. The pure-Go driver spells them differently, and **an unrecognised DSN parameter is accepted silently**, so the wrong name gives you a database with no foreign keys and a green suite | Decide cgo or pure-Go in step 0.1, and add a test that inserts a child row with a bad parent and asserts it is refused. That is the only way to know the pragma took |
| **Postgres in the suite is aspirational** | The plan says the container runs "in the same suite, so it is one command". `go.mod` has no Postgres driver and no container library, and the repo has no compose file. If the container does not come up, every Postgres test skips and "runs on both" quietly becomes "runs on SQLite" | Add `gorm.io/driver/postgres` in step 0.1 — the plan's 0.1 forgets it. Make the Postgres suite **fail** when `PASSION_TEST_POSTGRES` is set and unreachable, and print a skip count when it is not set |
| **The private catalog is a second repository** | Risk row 1 covers the content question and not the coupling. Two trees, one format, no shared CI. If the format changes after 2b, both trees re-convert | Convert both in the same week, and put a `format_version` in one `catalog.yaml` at each tree root so a stale tree fails loudly instead of importing wrong — **revised 2026-09-08 from "every file": it is one constant per tree, and 271 copies is 271 chances to disagree, while a `git mv` does not touch it** |
| **Two live databases while you train** | The plan's order exists so you can train with V2 before it is finished. Master keeps running on production against the old database. Sessions logged in V2 during phases 3-11 live nowhere else | Say which database is the real log during the build. If it is V2's, then "old production data is disposable" stops being true from phase 3 onward, and the SQLite file needs a backup story |
| **No rollback policy** | Goose owns the schema and nothing says migrations are append-only, or what happens if phase 5 needs a column that SQLite cannot add | 3.5's rules, written into the plan |
| **Feature loss by omission** | 37 sub-actions behind 7 catch-all patterns. A route dropped by accident and a route dropped on purpose look identical | Enumerate the 37 in the plan, with a phase or the word "drop" beside each. Section 7 is the start of that list |
