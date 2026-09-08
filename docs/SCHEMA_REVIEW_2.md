# Schema v2 — second review

> **Snapshot, not the plan.** This records a review as it stood on its own date. Decisions
> have moved since — see `V2_PLAN.md` §1.10 (private trees as owned content, fork-on-edit,
> `source_tree`) and §1.11 (`content_key` replacing the log's text link). **Where this file
> disagrees with `V2_PLAN.md` or `SCHEMA_V2.sql`, those two win.** Nothing here is edited to
> keep up. That is what makes it a snapshot.

Reviewing `docs/schema-v2-journey.html` as it stands on 7 September 2026, against
`docs/SCHEMA_REVIEW.md` (first review), `docs/DB_PRINCIPLES.md` and `docs/DEFAULTS_PATTERN.md`.

Fixed context accepted without argument: PostgreSQL and SQLite only, GORM plus a little explicit
SQL, old data disposable, current feature set is the target, and the three rules are decided.

Two engine claims below were tested on this machine, with output, in
`/tmp/claude-1000/-home-awalvie-code-lamp-passion/64a38945-d9fd-4d93-9093-e3fc8b9da59d/scratchpad/fkcheck`
(gorm v1.31.1, driver/sqlite v1.6.0, `_foreign_keys=on`). Everything else about engine behaviour
comes from `DB_PRINCIPLES.md`, which I treated as established. Feature claims come from the routes
and handlers named beside them. Nothing here is quoted from memory.

---

## 1. Verdict

The three new tables are the right three, and the reasons written beside them are correct: the
schema now covers the cycle-targets page and the per-set editor, which it could not before. But the
page is no longer a schema — it is a nineteen-row inventory with no columns, no keys, no indexes and
no referential actions, so most of the first review's defects are not fixed, they are simply no
longer written down anywhere. Do not build from this page: the new tables need one page of real DDL
first, and four of the new defects below (N1, N2, N3, N4) will otherwise be built in.

---

## 2. First-review defects — fixed, half-fixed, or dropped

"Silent" means the page states nothing either way. For a document a builder works from, silent is
open, not fixed.

| | Defect | Status | Evidence |
|---|---|---|---|
| D1 | Partial unique indexes do not port; GORM drops the predicate on MySQL | **Moot, not fixed** | MySQL and MariaDB are out, so partial unique indexes are available on both remaining engines (DB_PRINCIPLES §1.2, with PG and SQLite doc URLs). The brief already verified this repo's GORM emits and enforces them. The page still relies on `author IS NULL` (footer) but names neither index — the two indexes that carry the whole ownership model are unrecorded |
| D2 | `author_id IS NULL` gives absence a meaning; `visibility` can disagree with it | **Half-fixed** | The disagreeing column is gone: sharing was removed, and `visibility` and `content_share` appear nowhere in the 19 tables. The other half got sharper. With no `visibility` column, ownerless now means world-readable with nothing able to contradict it, so one `ON DELETE SET NULL` on `content.author_id` publishes a deleted user's private content. The page names no referential action anywhere |
| D3 | The `content` supertype was Class Table Inheritance with thin subtypes | **Fixed** | One `content` table, `kind` says which, and `movement`/`block`/`session` are gone. This is the largest real improvement in the revision |
| D4 | The hot-path index cannot serve the hot-path query | **Half-fixed** | The task brief says `log_entry` now freezes `account_id` and `on_date`; the page does not say so, and names no index. The query also filters the parent's state — `session_runs.status = 'completed'` in `exerciseHistoryItems` (`http/server/exercise_history.go:44-48`) — which the two frozen columns do not cover. See N3 |
| D5 | `account_metric` was EAV | **Fixed, with a hole** | `body_measurement` and `grade_milestone` are separate tables, and "a number beside it so it sorts" is the ordinal the review asked for. Hole: `max_pull_ups` and `max_hang_kg` are saved on the profile today (`http/server/profile.go:62-63`) and have no table in the 19. Nor does the page carry discipline (boulder and route are separate today) or the two grade systems (`boulder_grade_system`, `route_grade_system`) |
| D6 | Calendar dates need one date-only Go type, `date` on PG and TEXT on SQLite | **Dropped** | The page names no column type |
| D7 | `CHECK` is not a dependable primary mechanism | **Half-moot** | Both remaining engines enforce `CHECK` (DB_PRINCIPLES §1.4), so the version matrix is gone. The bigger half is live and unmentioned: SQLite's `ALTER TABLE` cannot add a `CHECK` or a `UNIQUE`, so every constraint must exist in `CREATE TABLE` and `AutoMigrate` will never add one later (DB_PRINCIPLES §1.16). See N11 |
| D8 | `ON DELETE CASCADE` is not sound enough to be the account-deletion mechanism | **Dropped** | The page says nothing about deletion, cascades, or a `DeleteAccount` path. SQLite still enforces no foreign key unless every pooled connection enables it |
| D9 | Comma-separated `labels`, `needs`, `source` | **Fixed** | `tag` and `content_tag` are two of the 19. `tag.kind` still has to carry gear as a second vocabulary, and `source` should stay one text column — neither is stated |
| D10 | Every foreign key is unindexed | **Dropped** | The page names no index at all. PostgreSQL does not create one for a foreign key (https://www.postgresql.org/docs/current/ddl-constraints.html). See N7 for the minimum set |
| D11 | The frozen log is not immutable, does not freeze units, and its slug is ambiguous | **Dropped, one part improved** | No `finalized_at`, no immutability rule, no `grade_system` on `log_climb`, no weight unit, no catalog key. Improved instead: `log_set` now holds asked and done on one row, which is worth more than the units gap costs |
| D12 | The read-time fallback should be resolved at author time; one definition only | **Regressed** | There were two sources. There are now five, and the page's own footer still says "A number comes from two places only … No override table." See N2, N14 and section 4 |
| D13 | The tree needs a depth bound, a cycle guard, and an FK instead of an exclusive-or `CHECK` | **One third fixed** | The exclusive-or is gone: `content_item` has one child column and `content_media` one parent, so the 373,212-row unenforced invariant in `exercise_media` becomes structural. Depth bound and cycle guard: still absent, and now harder — see N4 |
| D14a | `position` uniqueness and the swap problem | **Fixed by decision** | Non-unique `position` plus a whole-sibling rewrite is what DB_PRINCIPLES §3.9 chose, and it matches the UI, which already posts the entire order (`handleTrainingLogReorderExercises`). Two things still missing: the rewrite must be `UPDATE`-only, and `ORDER BY position, id`. See N10 |
| D14b | A generator needs a natural unique key | **Half-fixed** | Content has slug identity implied. `scheduled` has none, and its plan id must be nullable. See N5 |
| D14c | No optimistic locking (`version`) | **Dropped** | Not on the page. Two concurrent first edits still produce two forks |
| D14d | Collation differs per engine | **Mostly moot** | With MySQL out, both remaining engines are case-sensitive by default (DB_PRINCIPLES §1.11). Slug normalization in Go is still needed for case and accents, and is unstated |
| D14e | No `created_at` / `updated_at` on content or log | **Dropped** | Not on the page. "Sort my library by newest" and "what did the last import change" stay unanswerable |
| D14f | Missing keys: `content_share` PK, `account.email` unique | **Half-fixed** | `content_share` is gone with sharing. `account.email` uniqueness is silent, and it is the key the login path depends on |
| D14g | JSON "free merges" contradicted the rest of the document | **Fixed** | `content_item_set` is a table, and the page states the reason. Four routes confirm it: `/exercises/{id}/planned-sets`, `…/{setIndex}/save`, `…/{setIndex}/delete`, `…/clear` |
| D14h | An abandoned run is indistinguishable from a journal entry | **Half-fixed** | `log.state` exists now, so running and completed are separable. The page names only `draft`, never the full value set, and the app also needs a per-entry state — `/runs/{runID}/exercises/{exerciseID}/skip` is a live route and `log_entry` has nowhere to record a skip |
| D14i | Retired catalog items have nowhere to go | **Dropped, and it matters more now** | See N8: the importer hard-deletes |
| D14j | `account.height_cm` is not immutable | **Dropped** | The page repeats it: "the body facts that never move". Height is editable on the profile page today |
| D14k | Table count is not a design metric | **Not taken** | The page's subtitle is now the table count |

Sharing: cleanly gone. No `content_share`, no `visibility`, no `is_public`, and the read rule in the
footer has no sharing branch. Nothing in the 19 tables assumes it. The one consequence to notice is
in D2 above — with `visibility` gone, ownerless is unconditionally readable.

---

## 3. New defects, by severity

### N1 — `plan_target`'s unique key cannot be over a nullable `week`. `CORRECTNESS`

A unique key containing a nullable column does not constrain the rows where it is NULL. Both engines
treat NULLs as distinct (DB_PRINCIPLES §1.3, with doc URLs for each). Tested here on SQLite:
`UNIQUE (plan_id, movement_id, week)` accepted two whole-cycle rows for the same movement in the
same plan — `rows with week IS NULL = 2`.

**Consequence.** Two rows claim the same whole-cycle target. The targets page auto-saves silently
(`handleCycleOverrideSave` writes with no user confirmation), so the duplicate is invisible, and
which one wins depends on the plan the engine picks.

**How it should be keyed.**

```sql
week    INTEGER NOT NULL DEFAULT 0,          -- 0 = the whole cycle
CONSTRAINT ck_pt_week CHECK (week >= 0),
CONSTRAINT ux_pt UNIQUE (plan_id, movement_id, week)
```

Tested: the duplicate is refused, and `week = -1` is refused. This is DB_PRINCIPLES §1.2's own
answer — put a `NOT NULL` value in the key so the key is total. It also makes resolution one
statement: `WHERE week IN (0, :n) ORDER BY week DESC LIMIT 1`. A pair of partial unique indexes
would work now that MySQL is out, but it costs two indexes and keeps NULL meaning something.
`CHECK (week <= plan.weeks)` is not available on either engine — it reads another row — so the upper
bound stays in Go.

**Second half of this defect: `varies_by_week` has no home.** The targets page has a per-exercise
toggle that opens the per-week grid (`/training-cycles/{id}/week-override-toggle`), and it must work
before any week value is typed. Today that is a flag on the cycle-level row, and the code comment at
`http/server/training_cycle_overrides.go:176-185` says exactly why a bare flag row must be allowed to
exist. Merging two tables into one `plan_target` leaves the flag nowhere. Put it on the `week = 0`
row and write down that it is meaningless on any other row.

**Third: the design must say NULL, not zero, means "not asked".** Today zero means unset for sets,
reps and rep-seconds, while 0 kg legitimately means bodyweight — the comment at those lines
records that trap and the workaround. Nullable target columns remove it. Non-nullable integers
rebuild it.

### N2 — Fork provenance is missing, so two shipped features cannot be built. `CORRECTNESS`

The page's rule 2 says an edit gives you your own copy. It never says what records where the copy
came from, and no column on any of the 19 tables does.

**Consequence, two live features.** `reset-catalog` is a route on all three content kinds
(`exercise_library.go:338`, `templates.go:299`, `activity_template_handlers.go:137`). Without an
origin, reset cannot find what to restore. The library also shows whether a row was edited
(`catalog_edited_at`); with fork-on-edit, "edited" is "a fork exists", which again needs the link.

**Worse: the log's join key is the slug, and nothing says a fork keeps it.** The progression feature
groups history by exercise identity (`exerciseHistoryItems` plucks every exercise with the same
library id, else the same name). In the new design the equivalent is `log_entry.movement_slug`. If a
fork mints `deadhang-2`, every user's history splits in two the first time they edit an exercise, and
the split is silent.

**Fix.** State both invariants and enforce what can be enforced. A fork keeps the origin's slug —
`UNIQUE (author_id, kind, slug)` permits it, because the owner differs. Carry the origin as a value,
not only as a foreign key: `origin_slug VARCHAR NOT NULL` plus a nullable `forked_from` id.
DEFAULTS_PATTERN §5 recommends the value form for this exact reason — a deleted shipped row must not
take the provenance with it. If you keep `forked_from` as an FK, this pins the kind for free and
needs no extra column:

```sql
FOREIGN KEY (forked_from, kind) REFERENCES content (id, kind) ON DELETE SET NULL
```

A fork can then only descend from the same kind.

### N3 — Nothing keeps the frozen `account_id` and `on_date` in step with `log`, and the index does not answer the query alone. `INTEGRITY` `PERFORMANCE`

The denormalization is right and the first review asked for it. But two copies of one fact need a
stated invariant (DB_PRINCIPLES §3.7), and the page states none. A manual entry's date is editable
(`training_log_manual.go:212` seeds the date field), so the parent's `on_date` does change after the
children exist. If it diverges, the log page shows one date and the exercise history shows another —
which is the bug the brief already recorded once ("a manual entry backdated to 1 September shows 1
September in the log and 7 September on the dashboard").

**Fix, and it is structural, not disciplinary.** Tested on SQLite:

```sql
-- on log
CONSTRAINT ux_log_frozen UNIQUE (id, account_id, on_date)
-- on log_entry
CONSTRAINT fk_le_log FOREIGN KEY (log_id, account_id, on_date)
  REFERENCES log (id, account_id, on_date) ON UPDATE CASCADE ON DELETE CASCADE
```

Verified: a child with a mismatched `account_id` is refused; changing the parent's date propagates to
the child; an update that makes the child diverge is refused. Divergence becomes impossible rather
than merely tested. `ON UPDATE CASCADE` is required — without it, editing a draft's date fails.

**On the index.** `log_entry (account_id, movement_slug, on_date DESC)` is the right index, but the
first review's claim that it answers the query "with no join" is wrong. The query also needs the
log's state — the current query filters `session_runs.status = 'completed'`, and drafts must not
appear in a progression hint. The engine can walk the index in date order and check the parent per
row, which is cheap for a limit of five, but the join does not disappear. Say that, or freeze the
state too.

### N4 — The composite `(id, kind)` foreign key is necessary and not sufficient. `INTEGRITY`

Section 5 enumerates every reference. The short version: `(id, kind)` pins each end of an edge to a
kind. It cannot constrain the *pair*, the depth, or a cycle.

This is not theoretical here, because the real tree is four levels, not three. A "pick one" menu is a
movement whose children are movements — `exercises.parent_exercise_id`, chosen at run time by
`/runs/{runID}/exercises/{exerciseID}/choose` with `child_exercise_ids[]`. So `content_item` must
permit `(movement, movement)`. Once that edge is legal, a movement can be its own ancestor, and the
render loops. `CHECK` cannot see it: neither engine allows a subquery or another row in a `CHECK`
(DB_PRINCIPLES §1.4).

**Fix.** Three cheap parts. Constrain the legal pairs with one row-level check —
`CHECK ((parent_kind, child_kind) IN (('session','block'), ('block','movement'), ('movement','movement')))`
— written as an `IN` over the two columns, which is single-row and portable. Add
`CHECK (parent_id <> child_id)` to stop the one-row loop. Then a depth column maintained by the one
write path, plus a hard cap in the renderer, for the rest.

**Also missing from the 19 tables:** where "pick N of these" lives, and where the run's chosen option
is recorded. Both are live features and neither appears in any table description.

### N5 — `scheduled` has no natural key, and its plan id must be nullable. `INTEGRITY`

An ad-hoc scheduled session is created with `TrainingCycleID: nil`
(`handleAddScheduledSession`), so `scheduled.plan_id` is nullable. The first review's suggested key,
`UNIQUE (plan_id, on_date, session_id)`, therefore constrains nothing for ad-hoc rows — same NULL
rule as N1. And a cycle writes every week's rows in one loop at creation
(`training_cycles.go:356`), which is the shape of the worst bug in the brief: two identical runs
double the table.

**Fix.** Same sentinel treatment: `plan_id BIGINT NOT NULL` with a reserved "no plan" row is ugly, so
prefer two partial unique indexes here, now that they are available on both engines:
`UNIQUE (plan_id, on_date, session_id) WHERE plan_id IS NOT NULL` and
`UNIQUE (account_id, on_date, session_id) WHERE plan_id IS NULL`. Upsert against them.

### N6 — The log reads `place` at render time, so the wall has a hole. `CORRECTNESS`

Rule 3 says a finished session never reads content, and the page's test is "drop every content table
and your history still reads correctly". `place` is not a content table, and the log reads it live:
`training_log.go:408-412` and `:558-562` load the venue row to get `params.VenueName`, and the error
is ignored. Boards are the same (`run_journal.go:101`).

**Consequence.** Rename a gym and every past session's location changes. Delete a gym and the
location silently disappears from history.

**Fix.** Freeze `place_name` on `log`, keep `place_id` as `ON DELETE SET NULL`. This is
DB_PRINCIPLES' own rule for a history record — copy an immutable human label so the line still reads
after the FK goes NULL.

### N7 — No index is named anywhere in the design. `PERFORMANCE`

The page names 19 tables and zero indexes. PostgreSQL creates none for a foreign key
(https://www.postgresql.org/docs/current/ddl-constraints.html), and account deletion cascades into
every child table. The minimum set for these 19 tables:

`content (author_id, kind, name)`; `content (id, kind)` unique, needed as the FK target;
`content.forked_from`; `content_item (parent_id, position)`; `content_item.child_id`;
`content_item_set (item_id, set_index)` unique; `content_media.content_id`;
`content_tag (tag_id, content_id)`; `plan.account_id`; `plan_slot.plan_id`;
`plan_target (plan_id, movement_id, week)` unique; `scheduled (account_id, on_date)`;
`scheduled.session_id`; `calendar_event (account_id, on_date)`; `place.account_id`;
`log (account_id, on_date)`; `log.state` where it is filtered with the account;
`log_entry (log_id, position)`; `log_entry (account_id, movement_slug, on_date DESC)`;
`log_set (log_entry_id, set_index)` unique; `log_climb.log_entry_id`; `account.email` unique.

Two aggregate queries also want checking before they ship. The history page loads every run and every
completion for the account and counts in Go (`history.go:95-202`) — the new denominator is a count
over `log_entry` per log, which needs `log_entry (log_id)` as a leading column. And nothing in the
design supports "sort my library by newest", because there is no timestamp (D14e).

### N8 — The importer hard-deletes shipped rows, and the guard is hand-written. `INTEGRITY`

`pruneCatalogOrphans` (`db/yaml_import.go:256`) deletes a catalog row whose slug left the YAML,
after counting references by hand across four tables. That guard is already one table short of this
design — `plan_target` and `content_item_set` did not exist when it was written — and it will fall
behind again.

**Fix.** Make the database refuse it: `ON DELETE RESTRICT` on every reference into `content`, and a
`retired_at` marker so the library can hide a dropped item while a plan that still points at it keeps
resolving. This is the one exception to "no soft delete", and the first review already argued it; it
is now more urgent, not less, because the design deleted the marker and kept the delete.

Related and unstated: what happens to a fork whose origin is deleted or renamed. DEFAULTS_PATTERN §4
is unambiguous that a shipped slug is identity, and wger's answer is a deletion log with
`replaced_by` that repoints children in one transaction. The design needs one sentence choosing a
behaviour. "A renamed shipped slug creates a new row and orphans every fork of the old one" is an
acceptable answer, but only if it is written down and tested.

### N9 — Two derived facts are stored beside their source with no invariant. `NORMALIZATION`

`content_item` carries "its numbers", including `sets`, while `content_item_set` holds one row per
set. Today `SyncExerciseSetsCount` is called after every planned-set add, delete and clear to keep
them equal. Nothing in the design says which wins when they disagree, or that they must agree.

The same shape appears twice more. `log_entry.set_mode` is a new flag, and today the mode is not
stored at all: `handleTrainingLogSetMode` turns it on by inserting set 1 and off by deleting every
set row, so "mode on" *is* "a set row exists". A flag is defensible only if you want "mode on with no
sets" to be a state; then write the invariant that `set_mode = false` implies zero `log_set` rows.
`varies_by_week` is the third (N1).

Each of these needs one line: which column is the truth, and where it is maintained.

**Asked directly: is the `content_item_set` / `log_set` overlap a smell? No — it is the wall doing
its job.** They hold the same shape and different facts. `content_item_set` is the plan, and it stays
editable forever. `log_set` is what one day asked and what was done, and it must never change again.
That is the order-line-price argument DB_PRINCIPLES makes for the whole log (§Problem B, §3.7), and
the same reasoning that justifies `log_entry` copying the movement's name.

The app also proves the split is needed. Planned sets are edited both on a template exercise
(`/exercises/{id}/planned-sets/…`) and on a run's exercise
(`/runs/{id}/open/exercises/{id}/planned-sets/…`), and today both routes write the same table through
the same `UpsertExercisePlannedSet` call, keyed by an exercise row that is sometimes a template and
sometimes a run. That conflation is the latent bug the brief already recorded: planned sets are moved
off a template rather than copied, so the first run takes them permanently. Two tables is the fix,
not the smell.

### N10 — The sibling rewrite is right, and two details make it correct. `CORRECTNESS`

The page admits `position` has no uniqueness because SQLite cannot defer. That is the right choice.
Two things it does not say.

**Never rewrite by delete-and-insert.** The first review proposed "one `DELETE` plus one `INSERT`
inside a transaction". With `content_item_set` now a child of `content_item`, that recipe deletes
every planned set through the cascade. The rewrite must be `UPDATE`s only.

**The concurrent case that still breaks.** A full rewrite is computed from a list the client read
earlier. If another request inserts a sibling in between, the rewrite renumbers the rows it knew
about and never sees the new one, so two rows share a position. Nothing conflicts, so nothing errors,
and with a non-unique position the render order is then arbitrary. Two mitigations, both cheap: take
the parent row for update at the start of the rewrite so inserts serialize behind it, and always
write `ORDER BY position, id` so a duplicate at least renders the same way twice. The current
reorder handler does neither — it runs one `UPDATE` per row outside any transaction.

### N11 — "The same SQL on both engines" is not achievable for the constraints this design needs. `PORTABILITY`

The composite foreign keys in section 5 cannot be expressed by GORM association tags, so they need
explicit SQL. But SQLite's `ALTER TABLE` cannot add a constraint at all (DB_PRINCIPLES §1.16, with
the sqlite.org URL), so on SQLite they must be inside `CREATE TABLE`, which `AutoMigrate` owns. The
same statement is an `ALTER TABLE … ADD CONSTRAINT` on PostgreSQL. One of the fixed assumptions has
to give: either those tables are created from raw per-engine DDL, or the composite FKs are dropped
and the kind guarantees become `CHECK`s plus code. Decide it before writing models, because on SQLite
it cannot be added later without a table rebuild.

### N12 — There is no per-account time zone, so "today" is the server's day. `CORRECTNESS`

`db.LocalDate` keeps the incoming value's own location, and every caller passes `time.Now()`, so
"today" is the server process's zone (`http/server/time_helpers.go:42`, `db/queries.go:22`). Storing
dates as `DATE` is right and fixes the old offset bug. It does not answer *which* day it is for a
user in another zone, and the design targets a hosted version. One column: `account.time_zone` with
an IANA name and a default, and compute the day from it in Go.

### N13 — Rule 1 contradicts a decision already shipped in this repo. `INTEGRITY`

Noting it once, not relitigating it. "Shipped content has no owner. No account holds it" is the exact
state the last three commits on master made illegal and cleanable: `0393672` refuses an import into
an account that does not exist, after a deploy wrote about 1,172 catalog rows under an id with no
account behind it; `5d5fde8` adds a command to remove a catalog owned by nobody. DB_PRINCIPLES
§Problem A also chose the opposite (a reserved system account, `owner_id NOT NULL`). The page and the
repo cannot both be right. Whichever way it goes, the page should say that it overrides those, and
say what now plays the part of "the row that has no live account behind it is a bug".

### N14 — `plan_target` on a movement is one target per cycle, whatever the schedule does. `CORRECTNESS` (accept, but write it down)

Asked directly: does one row cover a movement that appears in two blocks in one session, or in two
sessions in one cycle? Yes, one row covers all of them. That is also what the app does today —
`buildCycleExerciseOverrides` walks the cycle's mapped session templates, collects every exercise,
and de-duplicates by library id, falling back to name. So the new design matches the shipped
behaviour and is not a regression.

Is it right? For the app as it stands, yes, and it is the cheapest thing that serves the page. It
will surprise a user who has the same exercise in a power session and a volume session in one cycle:
setting the target on one changes both, with no warning. That is a product limitation to state, not a
schema bug to fix here.

---

## 4. The five-source resolution order

**First, plainly: is `plan_target` the override table the design says it does not have?** The
document contradicts itself; the schema does not. The footer still reads "A number comes from two
places only: `content_item`, and your own log shown beside it. No override table." That sentence is
now false and must be rewritten — `plan_target` is a per-user target, because a `plan` always belongs
to one account. But the thing the design rejected and the thing it just added are not the same thing.
A per-user override on a *movement* shadows the catalog everywhere and forever, needs a reset story,
and is what makes a library row mean different numbers to different people. A `plan_target` is scoped
to one cycle that the user built and will end, and it is a prescription inside content the user
already owns. That is the same class of fact as `content_item`, and it is the pattern the brief
verified in Crimpd: change the volume in the plan, never in the workout definition. Keep the table.
Fix the sentence.

Write the order once, in one Go function, and let every page call it. It is a per-field resolution, not a
per-row one.

Given a movement, a `content_item` (its use inside a block), and a date:

1. **`plan_target` for this week.** Find the plan through the `scheduled` row for that date. The week
   is `floor((on_date − plan.start_date) / 7) + 1`. If a row exists for `(plan, movement, week)`,
   each of its non-NULL fields wins.
2. **`plan_target` for the whole cycle** — the `week = 0` row. Each of its non-NULL fields wins next.
3. **`content_item_set` rows.** They define the shape: one row per set, each with its own reps and
   weight. The number of rows is the number of sets.
4. **`content_item`'s own numbers.** The last source.
5. **Nothing.** A field still NULL after all four is "not asked". Render it blank.

**The log is not a fifth source.** It is shown beside the target and never resolved into it. Keeping
that true is what makes the wall real, and it is the one thing the page's footer still gets right.

**The conflict the order does not settle.** Sources 2 and 3 are specific on different axes: a
whole-cycle "3 × 5" and a per-set list of "5, 3, 3" cannot both be obeyed. Pick one rule and enforce
it in the one write path — the least surprising is that a movement whose item carries per-set rows is
not eligible for a `plan_target` at all, so the targets page does not offer it. The alternative,
per-week per-set rows, is a feature nobody asked for.

**Where it will surprise a user.**

- The same exercise in two sessions of one cycle shares one target (N14).
- A session dragged onto the calendar outside a cycle has no plan, so sources 1 and 2 vanish and the
  numbers silently drop back to the template's. This is the surprise most likely to be reported as a
  bug.
- Zero versus NULL. If a target column is a non-nullable integer, "0 sets" and "not set" become the
  same value and the fallback fires when the user meant zero — while 0 kg has to keep meaning
  bodyweight (N1, third part).
- A per-week row for week 3 that touches only `reps` still shows the whole-cycle `sets`. That is
  correct per-field resolution and it looks like a half-saved form. The page should show which field
  came from where.

---

## 5. Every foreign key into `content`, and what constrains its kind

`UNIQUE (id, kind)` on `content` is what makes any of this possible; it is not on the page and must
be. On SQLite the parent columns of a composite FK need that unique index — verified by the test
above.

| Reference | Kind it must be | What constrains it | Gap |
|---|---|---|---|
| `content_item.parent_id` | `session`, `block`, or `movement` (a pick-one menu) | `parent_kind` column + FK `(parent_id, parent_kind) → content (id, kind)` + `CHECK parent_kind IN (…)` | The FK pins the kind of the row, not the legality of the pair (N4) |
| `content_item.child_id` | `block` or `movement` | `child_kind` column + composite FK + `CHECK` | Same. Also nothing bounds depth or stops a cycle, and `(movement, movement)` is a legal edge |
| `content_item_set.item_id` | n/a — points at `content_item`, not `content` | plain FK, `ON DELETE CASCADE` | Nothing stops per-set rows hanging off a block-child item, where they are meaningless |
| `content_media.content_id` | should be `movement` only | nothing at all today | Add `kind` + composite FK, or accept media on a session |
| `content_tag.content_id` | any kind, deliberately | plain FK is correct here | none |
| `plan_slot.session_id` | `session` | needs `kind` column pinned to `'session'` + composite FK | Not stated. Without it a plan can schedule a movement |
| `plan_target.movement_id` | `movement` | needs `kind` pinned to `'movement'` + composite FK | Not stated |
| `scheduled.session_id` | `session` | needs `kind` pinned to `'session'` + composite FK | Not stated |
| `content.forked_from` | the same kind as the referencing row | `FOREIGN KEY (forked_from, kind) → content (id, kind)` — the row's own `kind` column pins it, no extra column needed | The column is not on the page at all (N2) |
| `log.session_id`, if provenance is kept | `session` | composite FK, and `ON DELETE SET NULL` — never `CASCADE`, which would delete history | Not stated. Must be nullable: open sessions and journal entries have no session |
| `log_entry.movement_id`, if kept | `movement` | composite FK, `ON DELETE SET NULL` | Not stated. The slug is the real key; this is provenance only |

Two general notes. A pinned kind needs `CHECK (kind_col = 'session')` as well as the FK — the FK
alone lets the column say `'movement'` and then find a movement. And every one of these FKs must be
indexed on the referencing side (N7).

---

## 6. What is right

- **One `content` table with a `kind` discriminator.** The first review's largest structural
  objection is answered in full, and the cross-hierarchy list the app shows on every library page is
  now one indexed scan.
- **`content_item_set` promoted back to a table, with the reason written beside it.** The reason is
  correct and I verified it: four routes save, delete, clear and append single sets. A JSON column
  cannot serve a UI that edits one element at a time.
- **`log_set` holding asked and done on the same row.** This is the best of the five changes. It
  makes "what did the plan ask that day" answerable after the template moves, and it makes the
  divergence hint computable inside the log — today that handler reads the live template
  (`handleExerciseDivergenceHint` compares the log against `ex.Sets, ex.Reps, ex.WeightKg`), which is a read
  across the wall this change removes.
- **`plan_target` restored.** It is the only place the cycle-targets page can live, and putting the
  numbers in the plan rather than in the workout definition is the pattern the brief verified in
  Crimpd.
- **One media parent instead of two nullable ones.** The 373,212-row invariant that nothing enforced
  becomes a `NOT NULL` foreign key.
- **`tag` and `content_tag`.** The delimited lists are gone, and with them the leading-wildcard
  `LIKE` scan and the rename-by-string-surgery.
- **`body_measurement` and `grade_milestone` as real tables, with a sortable number.** Grade
  progression becomes an `ORDER BY`.
- **One `place` for gyms, crags and boards.** Two tables and a pair of nullable foreign keys collapse
  into one reference.
- **A slug, not an id, as the log's key back to a movement.** SQLite reuses rowids after a delete
  (DB_PRINCIPLES §1.9), so a frozen numeric id would eventually point at someone else's exercise.
  This is the right call for the right reason.
- **Non-unique `position` with a whole-sibling rewrite.** Matches DB_PRINCIPLES §3.9 and the UI,
  which already posts the whole order.
- **The wall.** Still the best idea in the design, and every defect above leaves it standing — once
  `place` is on the right side of it (N6).
