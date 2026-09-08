# Schema redesign — handover brief

> **Superseded 8 September 2026. Do not act on this file.**
>
> This was a handover brief written before the V2 design was settled. It is kept only as a
> record of how the work started. **It has already caused one wrong build**: its "open signup"
> line was implemented as though it were a decision, when nobody had made that decision.
>
> The plan of record is `V2_PLAN.md`, and the schema of record is `SCHEMA_V2.sql`. Read those.
> Treat every statement below as a guess from before the questions were answered.

Written 7 September 2026. For whoever picks this up next.

Read this before you touch anything. The previous session wasted the owner's afternoon by
repeatedly anchoring on the current schema, spawning agents before understanding the
requirements, and re-asking questions he had already answered. Do not repeat that.

## What the owner wants

His words, over several days, in the order he stopped being patient:

1. **The catalog and the users are not tied to each other.** The only time they are tied is
   when a user makes a specific edit they want to keep. He has said this at least six times.
2. **Design the right database, from scratch.** Not a migration, not a patch. "IF THAT MEANS
   REDESIGNING THE WHOLE THING FINE." Every table is in scope, not only the catalog.
3. **Stop using the current code as the base.** He was explicit and angry about this. Design
   from the requirement. Do not reason about what to keep, change, or migrate.
4. **Multiple users by default.** Open signup. The invite gate currently in the code was a
   stopgap while the catalog was leaking licensed content — it is not the design.
5. **Self-sufficient and easy to use, above everything else.** He wants to publish this on
   Reddit for strangers to self-host, and eventually run a hosted version on its own domain.
6. **YAML is the source of defaults.** Its purpose is that deleting the database does not cost
   him the catalog. He does not want to re-author catalog items every time the database goes.
   Content lives in the database; the YAML seeds it. Users can also author their own content,
   and those items are gone if the database goes — he considers that correct.

## Constraints and freedoms

- **Existing production data may be discarded.** He said keeping it "would be nice, but not
  hugely important". Do not compromise the schema to ease a migration.
- **Project conventions are open to challenge with evidence**, including the rule in
  `.claude/agents/schema.md` that every model holding user data carries an `OwnerID`.
- **The catalog content gap is postponed.** Do not work on it. Do not design around it.

## Two questions he has not answered

Both are product decisions. Ask him, once, plainly, when it actually blocks you.

1. Should a user's own exercises be shareable with other users later, or private forever?
2. When a user reorders a shipped session or drops an exercise from it — is that a change they
   keep, or does it make the session theirs?

## Verified findings

Every item here was checked against code or measured against the running system. Nothing in
this section is inference. Where a number appears, it came from a command that was run.

### Defects in the current system

| | Evidence |
|---|---|
| **Anyone can inject content into shared catalog rows.** The child-add handlers take a parent id from the URL and never check it belongs to the caller. No `Preload` in the codebase filters children by owner — all 35 were read. With open signup this is a live vulnerability. | `http/server/activity_handlers.go:13`, `http/server/exercise_handlers.go:305` |
| **A second account gets a broken app.** Empty library page with working filters, 500 on Trial Run, 404 on export and on any library item's edit page, empty cycle-targets page. About 20 read sites ask for "my rows" where they needed "my rows plus the catalog". | `exercise_library.go:47`, `templates.go:526`, `export.go:78,191,300` |
| **Every restart leaks a generation of rows.** Two identical boots, no content change, doubled three tables: activities 16→32, exercises 130→260, media 75→150. Production holds 9,039 dead exercise rows against 1,119 live. It grows on every deploy. | measured with a three-line test |
| **Editing a session rewrites past history.** The history page counts exercises in the session as it stands today and uses that as the denominator for finished runs. Change a session from 19 exercises to 22 and every past run reads "12 / 22". Name and colour come from the live row too. | `http/server/history.go:96-102`, `:170` |
| **Dates carry a timezone.** `scheduled_date` stores local midnight with a UTC offset, in a column SQLite compares as text. A row written at `+05:30` does not match a same-day query bound at `+00:00`. Masked only because production writes one offset. | `db/models.go:404`, `training_cycles.go:1029` |
| **An invariant enforced by nothing.** Every one of 373,212 `exercise_media` rows has exactly one parent set — never both, never neither. Two nullable foreign keys, no `CHECK`. | schema read |
| **Run state sprawl.** Five booleans, six live combinations, two independent meanings of "draft" — one path sets `Status: draft` with the flag false, another sets the flag true with `Status: running`. | `db/models.go:424-434` |
| **`Visible()` is unenforceable.** It is called 15 times and never once from `http/server/`. Roughly 34 handler sites on catalog tables each had to remember the rule; about 20 did not. The failure mode is an empty page, not an error. | verified count |

Latent, not currently firing: planned sets are *moved* off a template rather than copied, so
the first run steals them permanently (harmless only because production has none); a manual
entry backdated to 1 September shows 1 September in the log and 7 September on the dashboard;
`exercise_burns` is a table in the database and in no Go file.

### Facts about the tooling, all tested

Do not re-derive these. Do not contradict them without running your own test.

- **AutoMigrate CAN create `CHECK` constraints and partial unique indexes** from struct tags,
  in this repo's exact GORM version (v1.31.1, driver/sqlite v1.6.0). `gorm:"check:name,expr"`
  emits `CONSTRAINT name CHECK (expr)`. `gorm:"uniqueIndex:name,where:..."` emits the partial
  index. Both enforce — the violating insert fails. A previous design's whole argument for
  abandoning AutoMigrate rested on this being false.
- **AutoMigrate cannot create triggers**, but `db.Exec("CREATE TRIGGER ...")` works and already
  sits beside AutoMigrate in `db/store.go`.
- **`BEFORE UPDATE OF col` fires syntactically, not by value.** GORM's `Save()` writes every
  column, so a naive immutability trigger aborts ordinary saves. A `WHEN OLD.x IS NOT NEW.x`
  guard fixes it.
- **SQLite has no deferrable unique constraints.** `UNIQUE(parent, position)` fails on a
  two-row swap, in one statement or inside a transaction. A temp-value detour works.
- **SQLite treats NULLs as distinct in unique indexes.** A single `UNIQUE(owner, slug)` will
  not stop two owner-less rows sharing a slug. Two partial unique indexes will.
- **A cycle generates every scheduled session for every week at creation time**, in one loop.
  There is no lazy generation. `http/server/training_cycles.go:356`.
- Foreign keys are enabled per-connection via `_foreign_keys=on` in the DSN. Soft deletes are
  `UPDATE`s and fire no cascade.

### Content shape, verified by independent re-derivation

| | public `catalog/` | private repo | total |
|---|---|---|---|
| library exercises | 88 | 96 | 184 |
| activity blocks | 10 | 14 | 24 |
| session templates | 4 | 13 | 17 |

All 178 distinct `ref:` targets resolve by slug; not one falls back to a display name. 24
library entries (13%) belong to no block or session. 11 of the 24 blocks wrap a single
"pick one" menu and nothing else. The public tree makes zero references into the private tree,
so it is coherent standalone. 11 or 12 exercises say "N reps per side" while the app counts N
in total — a real bug, and a morning's YAML edit.

## What was disproved

Roughly half of what the agents produced was wrong. The owner's instruction to have every
report independently verified is the only reason that is known. Do not skip that step.

- **Two quotations were fabricated** — sentences set in quotation marks and attributed to a
  source page where they do not appear. Two statistics were invented with false precision.
- **"No training app ships version reconciliation" is false.** Runna ships exactly it: a "New
  Plan Version Available" pill, a summary of changes, Update, and Revert to Previous Version.
  TrueCoach and Tredict have variants. Read Runna before designing this area.
- **Three separate agents read `/home/awalvie/code/lamp/passion/passion.db` as production.**
  It is a stale snapshot from before the 4–7 September recovery: 599 MB, 1,472 catalog rows,
  every slug empty, nothing shared. **Production is 13 MB**, 184 library rows all shared and
  correctly slugged, 33 runs, 134 completions, 48 ticks. **Do not read that file.**
- One "per side" count was inflated 2.5×.
- The two-list asymmetry in `db/backfill_runs.go` is deliberate and documented, not a bug.

## What survived from the industry survey

Verified word-for-word against the sources. Useful because it is convergent prior art.

- **Hevy** prompts to update a routine **only** on structural change — reordering, adding or
  removing exercises or sets. It never prompts for a change to reps or weight.
- **Strong** offers four choices on finishing a workout: Update Template, Update Values Only,
  Update Template and Values, Keep Original Template.
- **Strong** hides built-in exercises rather than deleting them, and restores one through the
  history of a workout that used it.
- **TrainingPeaks** dynamic plans propagate to athletes' calendars for today and future dates
  only. Past dates never move.
- **Crimpd** edits weekly volume in the plan, never in the workout definition.

The pattern all five share: **numbers live downstream of the definition, not as an edit to
it.** Structure asks. Identity is blocked, and duplication is a deliberate menu action.

## Suggested shape

Offered as a starting point, not a decision. The owner has seen this much and pushed back on
nothing in it. He explicitly rejected putting shipped content and user content in separate
tables — "just add a column to show the source" — and he was right: the source marker was
never what broke. Ownership was.

**Content, one set of tables.** `exercise`, `block`, `session`. Each carries `slug` and
`created_by INTEGER NULL REFERENCES account(id)`. `created_by IS NULL` means the app ships it:
nobody owns it, no account holds it, there is nothing to publish. Two partial unique indexes,
because of the NULL-distinctness rule above:

```sql
CREATE UNIQUE INDEX ux_exercise_shipped ON exercise(slug) WHERE created_by IS NULL;
CREATE UNIQUE INDEX ux_exercise_mine    ON exercise(created_by, slug) WHERE created_by IS NOT NULL;
```

**Seeding.** At boot the app writes the YAML into `WHERE created_by IS NULL` and nothing else.
Rows with an author are outside its reach by definition. It matches by slug and updates in
place rather than deleting and rebuilding, so a second boot changes nothing — test that by
booting twice against one file and asserting every row count is unchanged.

**A kept edit** is one row naming the account, the content row, the field and the value.
Everything not overridden still follows the YAML, so a shipped fix reaches a user who changed
something unrelated. Reset is deleting one row.

**History** stores what a run asked of you, once, as its own rows, and never reads content
again. A finished run must render identically with the entire catalog absent.

**Visibility** is `created_by IS NULL OR created_by = :me`. That rule cannot be enforced in
SQL. It has to be enforced by code shape — one place that builds every content read, with no
direct database access from handlers. Say honestly how far you get: structurally impossible,
compiler-caught, test-caught, or still down to discipline. The current code's failure was
claiming a chokepoint it did not have.

## Method notes for whoever continues

- **Do not spawn agents before you understand the requirement.** The previous session launched
  eleven and then sent three rounds of corrections to the last two, which is what exhausted
  the owner's patience.
- **Ask questions in prose, one or two at a time.** He answers directly and dislikes being
  asked things he has already answered.
- **Any load-bearing technical claim must be tested with the output shown.** This morning's
  designs were confident and wrong; three commands found the truth. The single most useful
  finding of the day — that two boots double three tables — took three lines and 1.2 seconds.
- **Write in plain, short sentences.** He has asked for this repeatedly.
- **Never commit or push without him asking.** He runs every production command himself.

## Where things stand

Production is healthy: 13 MB, catalog published, four commits on master today. The recovery
from the 4 September incident is complete and documented in [RECOVERY.md](RECOVERY.md).

Nothing about the redesign has been built. No code was written, no schema changed, nothing
committed. This document and the audit at
`https://claude.ai/code/artifact/815ec313-f5a2-47cc-8ab4-9d7a99527e18` are the whole output.
