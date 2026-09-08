# V2 build plan — second review

> **Snapshot, not the plan.** This records a review as it stood on its own date. Decisions
> have moved since — see `V2_PLAN.md` §1.10 (private trees as owned content, fork-on-edit,
> `source_tree`) and §1.11 (`content_key` replacing the log's text link). **Where this file
> disagrees with `V2_PLAN.md` or `SCHEMA_V2.sql`, those two win.** Nothing here is edited to
> keep up. That is what makes it a snapshot.

Reviewing [V2_PLAN.md](V2_PLAN.md) as revised on 8 September 2026, against
[V2_PLAN_REVIEW.md](V2_PLAN_REVIEW.md) (the first review), [SCHEMA_V2.sql](SCHEMA_V2.sql)
and [STRUCTURE_REVIEW.md](STRUCTURE_REVIEW.md).

Fixed and not relitigated: Go, GORM, chi, HTMX, server-rendered templates; PostgreSQL and
SQLite only; three packages plus `main`; goose migrations; old production data disposable;
same feature set; one developer.

Every number below has the command beside it. Two schema defects were proved by running the
DDL — SQLite through `mattn/go-sqlite3 v1.14.22`, PostgreSQL through a throwaway
`postgres:17-alpine` container, both since removed. I did not read `db/` or `pages/` beyond
line counts, and did not open `passion.db`.

---

## 1. Verdict

The revision absorbed most of the first review, but it dropped the one decision that review
said could never be taken back — the ladder's rep index — and the one change it added in
answer to that review, the composite `forked_from_id`, is broken as written: I ran it, and on
both engines it makes any forked-from row impossible to delete. Two more defects are real and
proved: account deletion is refused on PostgreSQL and accepted on SQLite, and the hand-invented
catalog slugs collide four times in the trees as they stand today. Fix D1 through D3 in the
migration-001 table, take the rep-index decision, and add a naming rule for the invented
slugs; then phase 0 is a good place to start, and nothing else here needs to block it.

---

## 2. First-review findings: absorbed, partial, or dropped

A finding restated as prose with no mechanism is scored **dropped**.

| First review | Landed in | Score |
|---|---|---|
| 3.1 the YAML format itself | Part 2, condensed, pointing back at §3.1 | absorbed |
| 3.1 rule 1-3: slug is identity, = filename stem, unique across kinds | Part 2 Rules | absorbed |
| 3.1 rule 4: no inline children | Part 2 | absorbed |
| 3.1 rule 5: unknown tag is a hard failure | Part 2 | absorbed |
| 3.1 rule 6-7: absent is NULL, 0 is real, dense renumber | Part 2 | absorbed |
| 3.1 rule 8: importer upserts, **never deletes**, marks retired | `retired_on` is in 1.1 item 3; "never deletes" is nowhere | partial |
| 3.1 `tags.yaml` | Part 2 — but see D15, the count is wrong | partial |
| 3.1 `movement_kind: session` → `duration` | Part 2 — unenforced, see D4 | partial |
| 3.1 media key rename | Part 2 | absorbed |
| 3.1 ref item shape, `per_set`, `sets == len(per_set)` | Part 2 | absorbed |
| 3.1 menu file, `pick` means the fewest | Part 2 | absorbed |
| 3.1 block file, `block_kind` on the block | Part 2 + 1.8 | absorbed |
| 3.1 `needs` stays session-only, stated as a limitation | nowhere | dropped |
| 3.1 **per-use block name dropped**, 6 refs resolved by hand | nowhere. 1.8 covers the per-use *warmup label*, not the per-use *name* | dropped |
| 3.1 **the hangboard ladder: rep index, option A/B/C, in phase 0** | nowhere. `rung`, `rep_index` and `ladder` appear zero times in the plan | **dropped — see D2** |
| 3.1 export emits `slug` and `ref:` so an export re-imports as the same rows | the exporter moved to 2a; the sentence about what it emits did not | partial |
| 3.2.1 split `d_rest_seconds` three ways | 1.1 item 1 | absorbed |
| 3.2.2 freeze `t_prep/t_rep_rest/t_set_rest` | 1.1 item 2 — then defeated by 1.7, see D7 | partial |
| 3.2.3 rep index on `content_item_set` / `log_set` | nowhere | **dropped — see D2** |
| 3.2.4 `retired_on` | 1.1 item 3 | absorbed |
| 3.2 `account.time_zone` | 1.1 item 4 | absorbed |
| 3.2 `account.token_epoch` | 1.1 item 5 | absorbed |
| 3.2 `max_pull_ups`, `max_hang_kg` | 1.1 item 6 | absorbed |
| 3.3 keep stateless JWT + cookie contract + CSRF + secret validation | 1.2, all four points | absorbed |
| 3.4 config, key by key | 1.3 | absorbed |
| 3.4 `Server.Seed` kept as a dev-only flag | `-seed` survives 1.5; what it seeds is never said, see §6 | partial |
| 3.5 when migrations run, six rules | 1.4 | absorbed |
| 3.5 **"back up the SQLite file before an upgrade"** in the readme | nowhere. The risk table says the file "needs a backup story" and stops | dropped |
| 3.6 `main.go`, twelve steps in order | 1.5 | absorbed |
| 3.7 errors and logging, six rules | 1.6 | absorbed |
| 3.8 `cmd/passionv2` from phase 1 | the plan's second framing decision | absorbed |
| 3.9 the menu choice design | 1.7 — with three new defects, see D5 | partial |
| 3.10 removals stated on purpose | 1.8 | absorbed |
| §4 gates on every phase | 13 gate blocks for 19 phases, and two are prose | partial |
| §5 N1 `varies_by_week` = one all-NULL row per week | 6a gate | absorbed |
| §5 N2 composite `forked_from_id` | 1.1 item 9 — **broken as written, see D1** | partial |
| §5 N3 the progression query does not filter the log's state | nowhere. The phase 8 gate covers statistics, not the run screen's hint | dropped |
| §5 N5 `scheduled` needs an upsert and a test | 6a gate | absorbed |
| §5 N7 the two missing indexes | 1.1 items 7 and 8 | absorbed |
| §5 N9 first half: `sets` vs `content_item_set` | Part 2 invariant + 5b gate | absorbed |
| §5 N9 second half: `set_mode='simple'` implies zero `log_set` rows | nowhere | dropped |
| §5 N10 the sibling rewrite is `UPDATE`-only | 5b gate, in those words | absorbed |
| §5 N13 say in one sentence that V2 overrides `0393672` and `5d5fde8`, and get sign-off | 1.3 asserts the ownerless catalog; the two commits are never named | dropped |
| §5 D14d slug and email normalization in Go | 1.9 | absorbed |
| §6 the four out-of-order items | all four moved | absorbed |
| §6 the four phase splits | all four split | absorbed |
| §6 "13 phases is the wrong count" | corrected to 17; the real count is 19, see D9 | partial |
| §7 enumerate the 37 sub-actions with a phase or "drop" beside each | the risk table repeats the instruction and points at §7. The plan enumerates none | dropped |
| §8 orphaned templates named in a phase | `run.html` gets its own paragraph. `run_ticks`, both run hints, both session pickers and `layouts/base.html` are named in no phase | partial |
| §9 relative sizing, per phase | Part 4 carries the LOC — one line is misassigned, see D10 | partial |
| §10 the run player's JavaScript | risk row + phase 3 contract + one test per timer phase | absorbed |
| §10 cgo vs pure-Go | phase 0 step 0.1 + the bad-parent test | absorbed |
| §10 the Postgres suite must fail, not skip | Part 3 + phase 0 gate | absorbed |
| §10 the private catalog is a second repository | risk row + `format_version`. No CI, no slug rule, see D8 | partial |
| §10 two live databases while you train | risk row says "Decide which is the real log". No decision, no backup | partial |
| §10 no rollback policy | 1.4, append-only, never down | absorbed |
| §10 feature loss by omission | risk row, no enumeration | partial |

**Score: 33 absorbed, 15 partial, 8 dropped.**

---

## 3. New defects, ranked

### D1 — the composite `forked_from_id` cannot coexist with `ON DELETE SET NULL`. Proved.

1.1 item 9 makes the key composite and says nothing about the delete action.
`SCHEMA_V2.sql:94` carries `ON DELETE SET NULL`. Both engines then set **every** column of
the key to NULL, including `kind`, which is `NOT NULL`.

SQLite, `mattn/go-sqlite3 v1.14.22`:

```
DELETE the shipped session id=1     ERROR: NOT NULL constraint failed: content.kind
rows with id=1 remaining: 1
```

PostgreSQL 17 names the statement it generates:

```
ERROR:  null value in column "kind" of relation "content" violates not-null constraint
CONTEXT: UPDATE ONLY "public"."content" SET "forked_from_id" = NULL, "kind" = NULL ...
```

So no content row that anything has forked from can ever be deleted, on either engine, and
the error names `kind`, not the foreign key — a landmine with a misleading message.

The kind-pinning half **does** work, and that answers the question in the brief: it is the
intended semantics, not an accident. A row with `kind='movement'` pointing at a session is
refused on both engines (`Key (forked_from_id, kind)=(1, movement) is not present`). The fork
keeps its parent's kind for free, exactly as item 9 claims.

**Fix.** Item 9 becomes `FOREIGN KEY (forked_from_id, kind) REFERENCES content (id, kind)
ON DELETE RESTRICT`. I ran it: the fork inserts, deleting the parent while a fork exists is
refused with a plain `FOREIGN KEY constraint failed`, and deleting the fork first then the
parent both succeed. RESTRICT is also what item 9's own justification asks for — "reset to
default needs the target to survive". Do not reach for PostgreSQL 15's
`ON DELETE SET NULL (forked_from_id)`: it works (I ran it) and SQLite has no such syntax, so
it would split the two dialects on semantics rather than on spelling.

### D2 — the rep-index decision is dropped, and it is the one decision SQLite cannot take back.

First review §3.2.3 said pick option A, B or C in phase 0, because `content_item_set`'s
primary key is `(content_item_id, set_index)` and SQLite cannot alter a primary key. The
revision does not mention it. `rung`, `rep_index` and `ladder` appear zero times in
`V2_PLAN.md`, and Part 2 says only "`per_set` replaces the `rungs` string".

Three private files still carry the two-dimensional data:

```
../passion-private-catalog/exercises/bechtel/hangboard_ladder_open_hand.yaml:9:rung_seconds: "3,6,9"
../passion-private-catalog/exercises/bechtel/hangboard_ladder_full_crimp.yaml:9:rung_seconds: "3,6,9"
../passion-private-catalog/exercises/bechtel/hangboard_ladder_half_crimp.yaml:9:rung_seconds: "3,6,9"
```

Each is `sets: 3`, `reps: 3`, `rung_seconds: "3,6,9"` — three rungs **per rep**, not per set.
A flat `per_set` list cannot express it, and `run.html` reads `RungSeconds` off `CurrentStep`.
So the plan has silently chosen option B or C without saying which, and the choice is
unrecoverable after 001.

**Fix.** Add a tenth row to 1.1: option A —
`rep_index INTEGER NOT NULL DEFAULT 1` on `content_item_set` and `log_set`, primary key
`(content_item_id, set_index, rep_index)` and `ux_log_set (log_entry_id, set_index, rep_index)`.
If you prefer B or C, write the sentence "the three hangboard ladders lose their per-rep
shape" into 1.8, where removals live. Either way, decide it before 001, not after.

### D3 — account deletion succeeds on SQLite and is refused on PostgreSQL. Proved.

`fk_slot_session` and `fk_sched_session` are `ON DELETE RESTRICT`
(`SCHEMA_V2.sql:258,303`). Delete an account that has a cycle whose `plan_slot` points at
that account's own session, and the two engines disagree.

SQLite: `ok  account=0 content=0 plan_slot=0`.

PostgreSQL 17:

```
ERROR:  update or delete on table "content" violates foreign key constraint
        "fk_slot_session" on table "plan_slot"
DETAIL:  Key (id, kind)=(300, session) is still referenced from table "plan_slot".
```

Both rows would have been removed by the account's cascade; PostgreSQL checks RESTRICT before
it gets there. This is the phase 1 gate's own scenario, and Part 3 says handler tests run
"against a real store on SQLite" — so the one gate that exercises every delete action never
runs on the engine where it fails.

I tested three variants on both engines. Only one passes on both:

| Action | SQLite: delete the account | PostgreSQL: delete the account | Still blocks a bare session delete? |
|---|---|---|---|
| `RESTRICT` | ok | **refused** | yes |
| `NO ACTION` | ok | **refused** | yes |
| `NO ACTION DEFERRABLE INITIALLY DEFERRED` | ok | **ok** | yes, at commit |

**Fix.** In 001, spell `fk_slot_session`, `fk_sched_session` and `fk_item_child` as
`ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED`. Add to Part 3: the phase 1 cascade gate
runs on both engines, not SQLite alone.

### D4 — `movement_kind` has no CHECK, so Part 2's rename is unenforced, and 1.7 reintroduces the collision it was meant to remove.

`grep -n movement_kind docs/SCHEMA_V2.sql` shows `content.movement_kind VARCHAR(32)` with no
constraint. Part 2 renames the fourth value `session` → `duration` on the stated grounds that
"otherwise the string `session` means two different things in two columns" — and then nothing
stops a row from holding `session` anyway. SQLite cannot add a CHECK later, so this is forced
into 001 and the plan defers it.

Worse, 1.7 writes `movement_kind = 'menu'` into `log_entry.movement_kind`. That column
otherwise holds movement kinds (`climbing`, `timed_reps`, `duration`). Putting a *content
kind* in it is the same two-meanings-in-one-column defect, one table over.

**Fix.** 1.1 gains an item: `CONSTRAINT ck_content_movement_kind CHECK (movement_kind IS NULL
OR movement_kind IN ('reps_and_sets','timed_reps','climbing','duration'))`. For the menu
marker, use `log_entry.set_mode`-style honesty: add `'menu'` to a `ck_entry_movement_kind`
list explicitly, and say in 1.7 that `log_entry.movement_kind` carries one value that is not a
movement kind. The cleaner alternative is a dedicated `is_menu BOOLEAN NOT NULL DEFAULT FALSE`
on `log_entry`, which every progression and statistics query can filter on without knowing the
vocabulary. Either way it is a 001 change, because the CHECK cannot be added later.

### D5 — the menu design breaks re-choosing, orders the picks at random, and leaves menus in the progression index.

Attacking 1.7 in the five ways the brief asks:

| Case | What 1.7 gives you | Fix |
|---|---|---|
| **The runner re-chooses** | Impossible. The menu row was deleted on the first choose, so there is nothing left to replace. Today's behaviour is re-editable (first review §3.9: `handleRunExerciseChoose` deletes and re-inserts). This is a **feature removal missing from 1.8** | Keep the menu row, `state='pending'`, and hang the chosen rows off it. Re-choose deletes the chosen rows, not the menu row. Resolve the menu row to `state='done'` only when the run finishes |
| **`pick_count > 1`** | All picks land at the same `position`, so `ORDER BY position, id` orders them by UUID. One menu in the trees is "Choose a couple of the exercises below" | Renumber: freeze positions with gaps (×100), or renumber every later entry on choose. Positions have no unique constraint, and a run holds tens of rows |
| **`ORDER BY position, id` is a stable order** | Deterministic, yes; meaningful, no. `log_entry.id` is `CHAR(36)` invented on a phone. A v4 UUID sorts at random, so the picks appear in neither menu order nor choose order | If you want `id` as the tiebreak, mandate **UUIDv7** for all four log tables and say so in 1.7. Otherwise renumber positions and drop `, id` |
| **Never chosen, run finished** | The menu row survives with `movement_slug` = a *menu* slug. `ix_entry_progression (account_id, movement_slug, on_date DESC)` then serves "the last five times you did Drills" from menu rows | Say it: an unresolved menu row is skipped, `state='skipped'`, and every progression and statistics query filters `movement_kind <> 'menu'`. Add it to the phase 8 gate beside the `draft` assertion |
| **Abandoned run, unresolved menu** | Same row, and history renders a step named "Drills" with no targets. That is survivable but undocumented | One sentence in 1.7 |
| **The chosen movement is later forked** | Works by accident: the fork keeps the slug, so `movement_slug` still resolves. But **forking a menu is undefined** — `SCHEMA_V2.sql:527-531` lists session, block and movement, not menu, and no phase owns a menu editor for the 24 menus | Add menu to the fork list in 001's comment block, and name the menu editor in 5b |

### D6 — the five-source order does not hold column by column, so the phase 3 gate cannot pass as written.

`plan_target` carries only `sets, reps, weight_kg, rep_seconds` (`SCHEMA_V2.sql:274-277`).
`content_item_set` carries only `reps, weight_kg, seconds`. So after 1.1:

| Column | Sources that can supply it | Five-source order applies? |
|---|---|---|
| `reps`, `weight_kg` | 1, 2, 3, 4, 5 | yes |
| `sets` | 2, 3, 4, 5 — and `count(content_item_set)` is the truth per Part 2 | **undefined**: what wins when `plan_target.sets = 5` and the block has 3 per-set rows? |
| `rep_seconds` | 2, 3, 4, 5 | four sources |
| `seconds` | 1, 4, 5 | three |
| `prep_seconds`, `rep_rest_seconds`, `set_rest_seconds` | 4, 5 | **two**. A cycle cannot change a rest or a prep at all |

The phase 3 gate — "insert `plan_target` rows directly and assert each of the five sources
wins in turn" — is satisfiable for two columns out of eight.

**Fix.** Put that table in the plan beside the resolver. Restate the gate as "for each of the
eight columns, assert the winner from the table". State the `sets` rule explicitly: when
`content_item_set` rows exist, `count()` wins and `plan_target.sets` is ignored for that use.
State that a cycle cannot retime a movement, which is a real limitation of the targets page.

### D7 — 1.7 defeats 1.1 item 2.

Item 2's stated reason for freezing timings is that reading them live "means editing a
template changes a running session's timers". 1.7 then resolves a menu pick's targets **at
choose time** — mid-run, from live content. Every movement behind a menu is exempt from the
rule item 2 exists to enforce, and three of the four shipped public sessions contain a menu.

**Fix.** One sentence in 1.7: a menu pick's targets are resolved once, at choose time, and
frozen from then on — and the exemption window is from run start to choose, not the whole
run. Then say the same sentence in item 2 so the two do not read as contradictions.

### D8 — the hand-invented slugs already collide, four times, measured.

Part 2 calls "22 block slugs, 24 menu slugs and 9 block names" one-time handwork and gives no
naming rule. Slugifying the names that exist today collides:

| Collision | Where |
|---|---|
| `drills` (menu) × 2 | `catalog/activity_templates/drills.yaml`, `../passion-private-catalog/activity_templates/drills.yaml` — and the public block is **already** slug `drills`, so this is a cross-kind collision inside the public tree on day one |
| `endurance_method` (menu) × 2 | `../passion-private-catalog/session_templates/pcc_do_more.yaml`, `.../endurance.yaml` |
| `endurance_work` (block) × 2 | the same two private files |
| `movement_exploration` (block) | `../passion-private-catalog/session_templates/pcc_explore.yaml` — **the slug is already taken** by an existing row |

The importer catches these, which is the good news. The bad news is *when*: 1.5 step 7 imports
at boot and Part 2 makes a duplicate slug "a hard failure", so a colliding private tree means
the binary refuses to start. `.github/workflows/deploy.yml` replaces `catalog` and
`catalog-private` wholesale and then restarts the service, and there is no CI on either tree.
A slug invented in the private repo that matches one in the public repo is a production outage
with no test between the commit and the restart.

**Fix.** Three sentences in Part 2. (a) Invented block and menu slugs take the form
`<parent slug>_<name>` for blocks and `<parent slug>_menu` for menus, so the 46 are derived,
not invented, and cannot collide. (b) Every slug in an extra `Catalog.Dirs` tree is prefixed
`p_`, reserving the namespace. (c) A failed import logs and leaves the previously imported
content in place; it never stops the listener from opening. Then add a CI job in phase 13 that
runs `-import-catalog` against both trees, and say the private repo gets the same job.

### D9 — 19 phases, not 17; 13 gates, not 19; phase 4 is now empty.

`grep -c '^### Phase' docs/V2_PLAN.md` → **19**. Part 4's first line says 17. The first
review's own corrected list also has 19 entries and also calls it 17, so the error was
inherited.

`grep -c '^\*\*Gate\.\*\*' docs/V2_PLAN.md` → **13**. Phases **4, 5a, 6a, 7, 9a and 11** have
no gate of their own; five of them share their pair's gate, and phase 4 has none at all. The
brief's premise — "each phase now has one" — does not hold.

**Fix.** Correct the count to 19. Give 5a, 6a, 7, 9a and 11 their own gate, or merge each pair
back into one phase and stop claiming the split. Delete phase 4 (see §5) and keep 7c.

### D10 — phase 3's line count charges it the library.

Phase 3's sizing includes "504 library", which is `http/server/exercise_library.go` at 504
lines. The library list is phase 2b and the library editor is 5a, and the 5a+5b total of 2,263
excludes it. So 504 lines are counted in the run phase and in no other.

**Fix.** Move 504 to 5a. Phase 3 becomes 2,138 handler lines (844 + 388 + 151 + the 755-line
ticks store half) plus `run.html`; 5a+5b becomes 2,767.

### D11 — the 81 params structs are owned by no phase.

`STRUCTURE_REVIEW.md` §7 item 3 calls view models "the largest missing piece by volume — 81
structs" and §5 gives the whole answer: a params struct per page, in the handler's own file, a
shared `BaseParams` in `render.go`, a typed render method per page. The plan mentions none of
it. `view model`, `BaseParams` and `params struct` appear zero times in `V2_PLAN.md`.

**Fix.** One line in phase 1: `render.go` defines `BaseParams` with the account, the flash
pair and the static checksum, and every page params struct embeds it. One line in Part 4's
preamble: params structs live in the handler's file, never centralised.

### D12 — two invariants the first review asked for are gone.

N9's second half — `set_mode='simple'` implies zero `log_set` rows — and N3's second half —
the run screen's progression hint reads `log_entry` without filtering `log.state`, so a
`draft` manual entry appears in "the last five times". The phase 8 gate covers statistics; the
hint is a different query and phase 3 owns it.

**Fix.** Add to phase 3's gate: a `draft` log's entries appear in no progression hint, and
saving one entry with `set_mode='simple'` leaves zero `log_set` rows. Decide whether the hint
joins `log` (and say the index no longer answers it alone) or `log.state` is frozen onto
`log_entry`.

### D13 — the ownerless catalog still overrides two commits without saying so.

1.3 asserts "`author_id IS NULL` is shipped content" three times and never names `0393672`
("refuse to import a catalog into an account that does not exist") or `5d5fde8` ("remove a
catalog owned by nobody"). The first review flagged this as the one item needing sign-off.

**Fix.** One sentence in 1.3: "V2 overrides `0393672` and `5d5fde8`. Those commits made a
catalog owner mandatory so no deletion could orphan it; V2 removes the owner entirely, which
achieves the same end, and the two commits die with `db/`."

### D14 — `SCHEMA_V2.sql` was not updated, and it now contradicts the plan in three places.

The plan carries nine deltas in a table and leaves the schema file as it was. The file still
says (line 546) "AutoMigrate handles the tables, columns, plain indexes, and the partial
unique indexes", which Part 6 forbids; and (lines 9-12) that the identity column is "the ONE
construct in this file that cannot be spelled the same on both engines", which D1 shows is now
two.

**Fix.** Apply the nine — soon ten — changes to `SCHEMA_V2.sql` itself and delete the delta
table. One document, not two that disagree. Rewrite the "WHAT GORM CANNOT EXPRESS" section as
"what goose writes by hand", and correct the one-construct claim.

### D15 — `tags.yaml` is 28 rows, not 29.

29 distinct labels exist, and `shoulder` (2×) and `shoulders` (1×) collapse to one — which the
first review said and then still wrote 29. Part 2 repeats it. Small, but the phase 2a gate
counts rows.

### D16 — `training_cycles.go` is not the largest file in the repo.

Phase 6's row says "the largest single file in the repo". `pages/pages.go` (1,813),
`db/queries.go` (1,400) and `db/yaml_import.go` (1,330) are all larger. The first review said
"the largest single *handler* file", which is correct.

---

## 4. Numbers I spot-checked

Commands run from the repository root. The private tree is `../passion-private-catalog`.

| Claim | Command | Result | Held? |
|---|---|---|---|
| "all 203 existing refs keep their shape" | `grep -rhoE '\bref:' catalog ../passion-private-catalog --include='*.yaml' \| wc -l` | **252** (73 public, 179 private; 147 in activity templates, 105 in session templates) | **no — 252** |
| "59 of 184 movements are climbing" | `grep -rhoE '^kind: *"?[a-z_]+' catalog/exercises ../passion-private-catalog/exercises --include='*.yaml' \| sed 's/kind: *"*//' \| sort \| uniq -c` | **57** climbing, 56 session, 36 reps_and_sets, 35 timed_reps — sums to 184 | **no — 57** |
| "`tags.yaml`, 29 entries" | `grep -rhE '^\s*label:' catalog ../passion-private-catalog --include='*.yaml' \| sed 's/.*label: *//' \| tr -d '"' \| tr ',' '\n' \| sed 's/^ *//;s/ *$//' \| grep -v '^$' \| sort -u \| wc -l` | 29 distinct labels, but `shoulder` and `shoulders` both appear → **28 rows** | **no — 28** |
| "17, not 13" phases | `grep -c '^### Phase' docs/V2_PLAN.md` | **19** | **no — 19** |
| "each phase now has one [gate]" | `grep -c '^\*\*Gate\.\*\*' docs/V2_PLAN.md` | **13** for 19 phases | **no — 13** |
| "the largest single file in the repo" (6a/6b) | `find . -name '*.go' -not -name '*_test.go' \| xargs wc -l \| sort -rn` | `training_cycles.go` 1,087 is 4th; `pages/pages.go` 1,813 leads | **no** |
| "27 inline exercises (24 of them menus)" | `grep -rhcE '^\s+- name:' … \| paste -sd+ \| bc`; `grep -rhoE '^\s*children:' … \| wc -l` | 27 and 24 | yes |
| "22 inline blocks, 9 with no name" | python walk of `activities:` items lacking `ref:` | 22 blocks, 13 named, 9 unnamed | yes |
| "56 of 184" `movement_kind: session` | as the climbing command above | 56 of 184 | yes |
| "112 files that carry media" | `grep -rlE 'video_url\|thumbnail_url' catalog ../passion-private-catalog --include='*.yaml' \| wc -l` | 112 | yes |
| "1,021 lines of JS" in `run.html` | `awk '/<script/{f=1} f{c++} /<\/script>/{f=0} END{print c}' templates/run.html` | 1,021 (1,011 strictly inside, 5 script blocks) | yes |
| "2,263 handler lines" (5a+5b) | `find http/server -name '*.go' -not -name '*_test.go' \| xargs wc -l` → 822+574+456+249+137+25 | 2,263 | yes |
| "2,170" (9a+9b) | same → 874+661+635 | 2,170 | yes |
| "1,670" (6a+6b) | same → 1,087+397+186 | 1,670 | yes |
| "37 sub-actions behind 7 catch-alls" | `awk '/URLParam\(r, "action"\)/,/^}/' http/server/<each>.go \| grep -oE 'case "[a-z-]+"'` → 8+9+6+2+7+2+3 | **exactly 37** (19 distinct names) | yes |
| "7 catch-all route patterns", 94 routes | `grep -cE '^\s+(r\|pr)\.(HandleFunc\|Handle)\(' http/server/core.go`; `grep -nE '\{action\}' http/server/core.go` | 94 routes; **10** `{action}` patterns over **7** prefixes, so 7 is the prefix count, not the pattern count | partly |
| "19 structs" | `grep -c '^CREATE TABLE' docs/SCHEMA_V2.sql` | 19 | yes |
| "16 of today's 17 flags die" | `grep -cE 'flag\.(Bool\|String\|Int\|Uint)' cmd/passion/main.go` | 17 flags; `main.go` is 582 lines | yes |
| "`set_rest_seconds` 43×, `prep_seconds` 35×, `rep_rest_seconds` 17× at movement level" | `grep -rhcE '^<key>:' catalog/exercises ../passion-private-catalog/exercises --include='*.yaml' \| paste -sd+ \| bc` | 43, 35, 17 | yes |
| "the public tree's 102 slugs", "= 88", "= 184", "123 private" | `grep -rhoE '^slug: *"[^"]+"' <tree> --include='*.yaml' \| sort -u \| wc -l`; `find <dir> -name '*.yaml' \| wc -l` | 102 / 88 / 184 / 123, and all 225 slugs are distinct across kinds and trees | yes |
| "nine tables that reference an account" (phase 1 gate) | `grep -cE 'REFERENCES account\(id\)' docs/SCHEMA_V2.sql` | **8** foreign keys. `log_entry.account_id` is frozen with no FK to `account`, so it is the ninth only by intent | partly |
| "the public tree's 102 slugs converted" (phase 2a) | python count of public inline blocks and menus | 2 blocks + 3 menus are **created**, so 102 files become **107**. The gate only checks movements = 88 | partly |
| "nothing in `go.mod` can reach Postgres today" | `cat go.mod` | confirmed; `mattn/go-sqlite3 v1.14.22`, so the build is cgo | yes |
| the ~1,200-line file trigger (phase 4) | `grep -n '1,200' docs/STRUCTURE_REVIEW.md` | line 116, exists as claimed | yes |

**Six wrong: 203 refs, 59 climbing, 29 tags, 17 phases, one gate per phase, and "largest file
in the repo".** Three more are partly wrong: nine account tables (8 foreign keys), 102 files
converted (107 after), and 7 catch-all patterns (7 prefixes, 10 patterns).

---

## 5. Gates that cannot fail

| Phase | The part that cannot fail | What to say instead |
|---|---|---|
| **4** | The whole phase. `wc -l` against a ~1,200-line trigger, at a point where `store/` and `web/` hold five files each, cannot trip. And the grep tests four words in a *name* — a store method returning `HomeRows` or `CalendarMonth` passes it clean, which is the exact evasion `STRUCTURE_REVIEW.md` §8 warns about | **Delete phase 4.** Fold the `wc -l` line into 7c. Nothing between phases 0 and 3 answers a structure question |
| **7c** | "The dashboard renders with **zero store calls returning a screen-shaped type**." Prose, with no mechanism. This is the first review's phase 4 finding, moved rather than fixed | Make it countable, the way §8 states the rule: assert the dashboard handler makes **at least four** separate store calls, and assert that every exported type in `store/` either matches a table struct or is named after a query, listed in a test fixture that a new type must be added to |
| **0** | "`go test ./store/... -count=1` passes." An empty suite passes. The named list moved to `store/BEHAVIOUR.md` and no gate asserts the file has any lines in it | Add: `store/BEHAVIOUR.md` lists N named behaviours, and a test asserts one Go test function exists per line. Pick N when you read `db/`'s 130 test functions, and write it into phase 0 |
| **0** | "A test asserts both migration directories hold the same version numbers." This tests the test fixture, not the code — it is a lint on filenames | Keep it; it is cheap and it is honest. But do not count it as a gate. The real gate on 0 is the bad-parent insert and the two-boot checksum |
| **13** | "A test reflects over `config.Config` and asserts every key in the readme table exists." One-directional. Add a config key and never document it, and this passes | Assert **set equality** between the readme table and the reflected struct, both ways |
| **6a/6b** | "Two whole-cycle rows for one movement are refused." That tests `ux_target`, which 001 either wrote or did not | Fine once, as a schema lint. The gate's weight is on the week-3 override and the `scheduled` upsert, which can genuinely fail |

The phase 3 gate remains the best in the plan — an action, an observation and a destructive
check — except for the five-source line, which D6 shows cannot pass as written.

---

## 6. What is still missing

| Missing | Evidence | What the plan should say |
|---|---|---|
| **Backups** | The word appears once, in a risk row: the file "needs a backup story". `passion.db` is 599 MB, and 1.4 forbids down migrations | A `-backup` flag that runs `VACUUM INTO` before migrating, and an unconditional copy on the first boot after a version bump. Phase 0, five lines. Without a down migration, the backup *is* the rollback |
| **What `-seed` seeds** | `Server.DemoOwnerID` dies in 1.3; `-seed` survives 1.5; `config/config.go:184` currently rejects `DemoOwnerID = 0` and `cmd/passion/main.go:151` calls `EnsureSeedUser(demoOwnerID, "demo@passion.local", …)` | `-seed` creates `demo@passion.local` by email, takes the id back, and writes demo rows under it. Say it in 1.5 |
| **What `DevAuthBypass` authenticates as** | 1.3 keeps `Auth.DevAuthBypass` and kills `DemoOwnerID`, which was its target account | `DevAuthBypass` logs in as the lowest-id account, or refuses to boot when none exists. One line in 1.2 |
| **Rate limiting on open signup** | `grep -rniE 'rate.?limit\|throttle\|golang.org/x/time'` over the repo returns nothing. 1.8 opens signup and drops the invite table, which was the only thing limiting account creation | Say it plainly: open signup has no rate limit, and that is accepted for a one-user app behind a known hostname. Or add a per-IP token bucket in `middleware.go` — 20 lines, no storage. Do not leave it unstated, because dropping invites *is* the change that makes it matter |
| **`embed` versus on-disk, and hot reload** | `.air.toml` watches `["cmd","config","http","db","pages","static","templates"]` — three of those are deleted in phase 12 and neither `store` nor `web` is listed. `deploy.yml` scps `templates`, `static`, `catalog` and `catalog-private` to `/opt/passion/`, so today they are on disk, not embedded | Phase 0 edits `.air.toml` to add `store` and `web`; phase 12 removes `db`, `pages`, `http`. Hot reload survives, because air's `cmd` is `go build`, so an embedded template is re-embedded on save — say that, because it is not obvious and it is the reason no dev-mode disk fallback is needed |
| **The deploy workflow and `passion.prod.yaml`** | `.github/workflows/deploy.yml` and `passion.prod.yaml` are named in no phase. Once `catalog` is embedded, its tar is dead weight; `catalog-private` must become `Catalog.Dirs: [/opt/passion/catalog-private]` | Phase 12 owns `deploy.yml` and `passion.prod.yaml`. Name them |
| **CI** | `.github/workflows/` holds only `deploy.yml`, which is push-to-master with no test step | Phase 13 adds a `test.yml` that runs `go build ./...`, the SQLite suite, the Postgres suite in a service container, and `-import-catalog` against both trees. The private repo gets the import job too — that is the missing mechanism behind "no shared CI" |
| **How the private catalog loads in development** | 1.3 says `Catalog.Dirs` is the mechanism, and nothing says what a developer puts there | `Catalog.Dirs: ["../passion-private-catalog"]` in `passion.yaml`, absent from `passion.example.yaml`, and the importer skips a directory that does not exist rather than failing. One line in 1.3 |
| **The 81 params structs** | See D11 | |
| **Flash messages** | `STRUCTURE_REVIEW.md` §7 item 9 puts them on `BaseParams`, populated from the session. 1.2 removes server-side session storage | Either a short-lived signed cookie or nothing. Say which |
| **The 37 sub-actions, enumerated** | The risk row says to enumerate them and points at the first review's §7, which groups by prefix rather than listing 37 rows | Put the 37 rows in the plan, one per line, with a phase or the word "drop". I have measured that the total is exactly 37, so the list is closeable |
| **Five orphaned templates** | `fragments/run_ticks.html` (534 lines), `run_ticks_readonly.html`, both run hints, both session pickers and `layouts/base.html` are named in no phase | Copy the first review's §8 table into Part 4 |
| **What happens to production's 599 MB file** | "Old production data is disposable" plus a risk row saying that stops being true at phase 3 | Decide now: V2's SQLite file becomes the real log at phase 3, and the old file is archived once and never read again. That is a decision, not a risk |
