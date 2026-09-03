# Passion — launch plan, written as agent prompts

Each ship below is a self-contained prompt. Hand one to a fresh agent with no other
context and it has what it needs. Do them in order. Do not start a ship before the one
before it is merged and seen working.

Written 2026-09-03, from a full codebase review plus four specialist reviews.

## Why the order is what it is

The app was built for one person. Every shortcut that is safe at one user becomes a defect
at many. Ship 0 stops active harm. Ship 1 makes the catalog fit for more than one owner.
Ship 2 builds the account layer that lets strangers in. Ship 3 opens the doors.

---

## Standing rules for every ship

Give these to every agent, at the top of every prompt.

- Read `CLAUDE.md` first, then `docs/DESIGN.md` if the work touches any template or CSS.
- **Port 3000 is the owner's live dev server, holding his real training database.** Never
  stop it, never `pkill`, never run against `passion.db`. Use port 3001 and a temp file
  from `PASSION_DB_PATH`. `.air.toml` watches `cmd config http db pages static templates`,
  so editing those restarts his server.
- **Never run `git add`, `git commit` or `git push`.** Implement, then stop and report.
- Tests use standard Go testing, real SQLite in `t.TempDir()`, and `httptest` for
  handlers. No frameworks.
- Production is `ubuntu@140.245.198.72`, key `~/.ssh/stone`, database
  `/opt/passion/passion.db`. **Read-only. Never write to it.**
- When a UX question is ambiguous, ask. Do not pick a direction and build it.
- Write to the owner in simple English.

---

## Ship 0 — stop the bleeding

Nothing here is user-visible. Every item gets more expensive, or impossible, if it waits.

### Prompt

> You are working on Passion, a self-hosted Go + Chi + GORM/SQLite + HTMX climbing
> training app. It is about to be hosted publicly. Today it is a single-user app and its
> signup page is open to anyone who finds it. Your job is to stop three kinds of active
> harm before strangers arrive. Read the standing rules above first.
>
> **Task 1 — close the licensed-content leak.**
> `http/server/auth.go:117` imports the whole YAML catalog for every new signup, and
> `cmd/passion/main.go:93-106` re-runs the same import for **every existing user on every
> server restart**. Production config points that import at both `catalog/` and
> `catalog-private/`, and the private tree holds content from paid programmes. So every
> stranger who has ever signed up holds licensed content, and gets a fresh copy on every
> deploy.
>
> Add invite codes to signup. A new model with a code, who created it, who used it, and an
> expiry. A field on the signup form. A check in `handleSignup` that rejects a missing,
> unknown, used, or expired code before creating the user. A way for the owner to mint
> codes — a flag on the binary is enough, no admin UI.
>
> **Invite codes stop new exposure. They do not close the leak.** Say so plainly wherever
> this work is described. Every account that already signed up still holds private-catalog
> rows under its own `owner_id`, and nothing here removes them.
>
> Finishing that is a separate job with a hard part: **nothing records which catalog
> directory a row came from.** `ManagedByCatalog` says "the importer made this", not "from
> the licensed tree". Any audit has to join on row *names* against the private catalog's
> current file list, and that join is fragile — the private blocks were already renamed
> once (`Warm Up (Paradigm)`, `Drills (Paradigm)`), so a name join against today's list
> silently misses rows imported under yesterday's names. That is what the `Origin` field in
> `docs/SHARED_CATALOG.md` is for, and it argues for building it sooner.
>
> Then it needs a decision that is the owner's, not yours: erase those rows, which breaks
> any run history referencing them, or leave the existing few accounts alone because the
> population is small and known. Ask. Do not delete anything without being told to.
>
> **Task 2 — the importer leak. The fix has shipped; the cleanup has not.**
> `db/yaml_import.go:817` deletes a session template's activities before recreating them:
> `tx.Where(...).Delete(&Activity{})`. `Activity` embeds `gorm.Model`, so this is a soft
> delete — an `UPDATE` of `deleted_at`. The `constraint:OnDelete:CASCADE` tag on
> `Activity.Exercises` only fires on a hard `DELETE` at SQL level. So the child `Exercise`
> rows are never touched. They stay live and orphaned, and a fresh set is created on every
> import, which runs on every restart.
>
> **The fix is done.** The delete now retires the leaf rows by name — media, then
> exercises, then the activities — mirroring `upsertActivityTemplate:647-650`. Three
> regression tests in `db/yaml_import_reimport_test.go` cover it, and they were checked
> against the old code first: 3 exercises became 12 over three unchanged re-imports.
>
> **Do not "improve" this into a hard delete.** `RunExerciseCompletion`, `ClimbingTick`,
> `ManualExerciseSetLog`, `ExercisePlannedSet`, `RunExerciseChoice` and
> `ClimbingExerciseMeta` all carry `exercise_id` with **no `REFERENCES` clause** — verified
> against `sqlite_master`. So `_foreign_keys=on` cannot protect them, and a hard delete on
> a path that runs every restart would strand completed runs permanently. The soft delete
> keeps retired rows recoverable. `TestReimportKeepsRetiredExercisesRecoverable` guards
> this.
>
> The other upsert paths do **not** have the bug. `upsertActivityTemplate:647-650` and
> `upsertLibraryExercise:785` soft-delete the leaf table directly, which is correct. The
> rule: a soft delete only removes rows from the table it names. Soft-deleting a parent and
> expecting `ON DELETE CASCADE` to follow is silent corruption, because the cascade never
> fires on an `UPDATE`.
>
> **What is left: the one-off cleanup.** Measured on the dev database after the fix landed:
> 122,180 live exercises, of which **118,059 are orphans** — 97% of the table. Of those,
> exactly **102 are referenced by run history and must survive**. 117,957 are safe to
> purge.
>
> Orphan candidates:
>
> ```sql
> exercises.deleted_at IS NULL
>   AND exercises.activity_id IS NOT NULL
>   AND exercises.activity_id NOT IN (SELECT id FROM activities WHERE deleted_at IS NULL)
> ```
>
> Must-keep — check all six tables, and deliberately **without** filtering their own
> `deleted_at`, because history rows are meant to be immutable facts:
>
> ```sql
> EXISTS (SELECT 1 FROM run_exercise_completions c WHERE c.exercise_id = e.id)
>   OR EXISTS (SELECT 1 FROM climbing_ticks t WHERE t.exercise_id = e.id)
>   OR EXISTS (SELECT 1 FROM manual_exercise_set_logs l WHERE l.exercise_id = e.id)
>   OR EXISTS (SELECT 1 FROM exercise_planned_sets p WHERE p.exercise_id = e.id)
>   OR EXISTS (SELECT 1 FROM run_exercise_choices ch
>              WHERE ch.parent_exercise_id = e.id OR ch.chosen_exercise_id = e.id)
>   OR EXISTS (SELECT 1 FROM climbing_exercise_meta m WHERE m.exercise_id = e.id)
> ```
>
> Delete the safe set in **one bulk statement**, not a loop. SQLite checks foreign keys at
> the end of a statement, so removing an `exercise_catalog` parent and its
> `parent_exercise_id` children together succeeds in one statement and fails in a loop.
> `ExerciseMedia` follows automatically through its real cascade. Afterwards, soft-deleted
> activities with no remaining children can go too — but only those with none, or the
> cascade takes a kept exercise with them.
>
> **Two tests before this runs anywhere near real data:**
> `TestCleanupOrphanedExercises_DeletesTrueOrphans` and
> `TestCleanupOrphanedExercises_PreservesExercisesReferencedByRunHistory`. Then a
> count-only dry run against a **copy** of the production database. It is a command the
> owner runs and watches, never a boot step.
>
> **The history bug is a separate job — do not bundle it.** Fixing the leak stops new
> orphans. It does not change that `buildRunSteps` (`http/server/runs.go:729`) renders a
> past run from whatever the template says *today*. The real fix is for a run to own its
> exercises: `MaterialiseTemplateExercises` (`db/queries.go:1123`) already does the copy but
> runs lazily, only when a log is first edited, and `buildRunSteps` never checks
> `ExercisesMaterialised`. That needs eager materialisation at run start plus a backfill
> over past runs, and the backfill re-points ticks and logs by `exercise_id`, so it carries
> its own data-loss surface and deserves its own review.
>
> **One thing I got wrong, recorded so nobody re-derives it:** the `exercise_media` triples
> are not duplication. They are three distinct `video_url` values on one exercise, which is
> what the YAML asked for. The media bloat is orphaned exercises dragging their media
> along, not a separate bug.
>
> **Task 3 — make a bad config fail at startup, not in production.**
> This is done; it is here so you know not to redo it. `config.Validate()` now rejects the
> placeholder secrets `change-me-in-production` and `__JWT_SECRET__`, rejects any secret
> under 32 characters, and rejects `Server.DemoOwnerID == 0` and a zero
> `YAMLImport.OwnerID`. Dev auth bypass skips the secret checks because it short-circuits
> verification anyway. `passion.example.yaml` no longer ships a secret or a live bypass.
>
> **Definition of done.** `go build ./...`, `go vet ./...` and `gofmt -l .` are clean. All
> tests pass. Signup refuses a request with no invite code. A second import creates no new
> `Exercise` rows — assert the count, that is the test the old suite was missing. The
> cleanup leaves every `RunExerciseCompletion` still pointing at a live exercise. Update
> `readme.md` for the new config rules and the invite flow.

### Also in Ship 0, done by the owner, not an agent

- **Backups.** One SQLite file on one box with no snapshot and nothing offsite. Litestream
  to object storage, or nightly `.backup` plus restic. **Plus one tested restore** — an
  untested backup is not a backup. Cheapest item on this list and the one that would hurt
  most.
- **Git history: decided against a rewrite.** Licensed files are gone from the tree but
  live in all 280 commits. The plan is to archive this repository and start a new one under
  an organisation instead, which leaves no history to clean.

  If that changes, two things are already known. All 123 private files exist in this
  repository's history, and the catalog was reorganised into program folders once, so the
  same content also sits at older flat paths — removing only today's paths would miss them.
  And **do not match by filename to find the old paths.** `drills.yaml` and `warmup.yaml`
  exist in both trees: the public ones are the replacements written from scratch, and a
  basename match would erase them from history too.
- **Rename `Final Exam I/II`** in the private catalog. Invented names attributed to a real
  named coach is misattribution, and worse than plain copying.
- **`LICENSE`** — done. PolyForm Noncommercial 1.0.0, chosen so the app can be self-hosted
  and modified but not run for profit. Note it is source-available, not open source.
- **The `CLAUDE.md` rule** on paid-programme content — done.

---

## Ship 1 — the shared catalog

Today every user gets a full copy of the whole catalog. That is what makes hosting
expensive and what leaks licensed content. The plan is in `docs/SHARED_CATALOG.md`. Two
independent reviews said the same thing: **Step 1 is right, Steps 2 to 4 need rework.**

### Prompt

> Read `docs/SHARED_CATALOG.md` in full, then the standing rules above. You are converting
> Passion from "every user owns a copy of the catalog" to "one shared catalog at
> `owner_id = 0`, which everyone reads". Do not do the whole document. Do this:
>
> **Widen the read side, and only the read side.** Every read of `SessionTemplate`,
> `ActivityTemplate` and `LibraryExercise` becomes "mine or shared". This is safe to ship
> alone because nothing exists at owner 0 yet.
>
> **The trap is the Preload closures.** They filter children by the reader's own id, and
> the importer gives children the same owner as their parent. So widening only the
> top-level query makes a shared session open with **no activities, no exercises, and no
> error**. Six known sites: `db/queries.go:53-72` and `:207-223`,
> `http/server/export.go:190` and `:301`, `http/server/dashboard.go:190-199`,
> `http/server/history.go:74-84`, `http/server/exercise_handlers.go:46-56`, and
> `http/server/exercise_library.go:236`. Search for more; that list came from two reviews
> and may still be short.
>
> Collapse every read of these three models behind the query helpers, so no handler writes
> its own owner filter. One place per model to get right, instead of dozens.
>
> **Keep every write at `owner_id = me`,** with an explicit `if row.OwnerID != ownerID`
> guard after each fetch. GORM's `Save()` writes whatever row it is handed.
>
> **Also handle the silent no-op writes.** Some routes scope their `WHERE` directly with
> no read step — `http/server/exercise_library.go:337-353` and
> `http/server/catalog_edited.go:106-118`. Once reads widen, a user will see a shared row,
> press Delete or Reset, get a success redirect, and the row will still be there. Make
> these fail visibly instead.
>
> **Then open a shared session in a browser and run it.** This failure is silent. It has
> to be seen, not just tested.
>
> **Do not build the import split or the migration in this pass.** When you get to the
> import split, know this: it wakes `pruneCatalogOrphans` (`db/yaml_import.go:188-236`)
> against the owner's own account. The keep-set would hold private names only, but the
> query pulls every catalog row he owns, so the published ones fail the test and get
> hard-deleted, unsupervised, at the next boot. Disable the prune for that transition
> import first.
>
> **Ship Duplicate, not copy-on-edit.** A product review found that no comparable app
> (Strong, Hevy, Crimpd, Lattice, MacroFactor) makes Edit copy a shared row. They keep the
> shared tier read-only and make Duplicate an explicit button. The decisive argument is in
> this codebase: `CycleExerciseOverride` and `CycleExerciseWeekOverride` already hold sets,
> reps, weight and rep seconds per cycle and per week, so every number a user would change
> already has a home. Copy-on-edit only adds the name, the notes, and out-of-cycle
> defaults — one additive `UserExercisePref` table covers those, and it fits the existing
> resolution chain: week, then cycle, then user default, then catalog.
>
> Copy-on-edit also has a cost the design doc does not name: once a user holds a copy,
> catalog fixes never reach them again. `CatalogEditedAt` exists because an edit was undone
> on restart. Copy-on-edit makes that trap permanent.
>
> **Add filename-as-row-id in this ship.** The importer matches on display name, so a
> rename reads as delete-then-create. `CycleExerciseOverride.LibraryExerciseID` is a plain
> indexed pointer, not an enforced key, so a rename silently dangles other users'
> overrides. 215 files already have unique basenames, so it is free now and expensive
> later.

---

## Ship 2 — the account layer

This is what actually lets strangers in. Everything here is missing today.

### Prompt

> Read the standing rules above. Passion is behind an invite gate with a small number of
> real users. Build the account layer that lets the gate come off.
>
> - **Wire SMTP once.** Password reset, email verification and deletion confirmations all
>   ride on it. Buy a provider; do not build a mail server.
> - **Password reset.** With none, a user who forgets is locked out permanently and cannot
>   delete their own account either. `handleProfilePassword` already covers the logged-in
>   path. Only the logged-out path is missing.
> - **Login rate limiting.** `handleLogin` (`http/server/auth.go:50`) does unbounded email
>   and bcrypt checks with no throttle. An in-memory per-IP and per-email bucket needs no
>   new dependency.
> - **Explicit HTTP methods on every route.** 2 of 93 routes in `http/server/core.go` name
>   a method. The rest answer any method, so the only thing stopping a state-changing GET
>   is whether each handler author remembered a guard. Several forgot —
>   `http/server/calendar_events.go` has no `r.Method` check at all. Register methods at
>   the router so a missing guard fails closed.
> - **Account deletion, as a real transactional cascade.** `deleteTemplate` already orphans
>   `SessionRun` rows, so a naive user delete will orphan everything. The complete cascade
>   already exists in `db.DeleteDraftRun` (`db/queries.go:563`) — the user-facing paths
>   just do not call it.
> - **Privacy policy and terms.** The owner is a controller under UK/EU law the moment a
>   stranger's email is in the database. One page each. Say what is collected, why, where
>   it is stored, for how long, and how to exercise rights, with a contact address.
> - **Fix the week-override bug** (`docs/TECHNICAL_AUDIT.md` §2.4). It is an advertised
>   cycle-builder feature that silently does nothing.
>
> Then hand out codes freely. Aim for 20 to 50 people. That is a closed beta in the real
> sense: actual strangers, a bounded number, all of them known to you.

---

## Ship 3 — open the doors

### Prompt

> Read the standing rules above. Passion has a closed beta of real users and a working
> account layer. Remove the invite requirement.
>
> - **Email verification.** Its jobs are making reset trustworthy and stopping signup spam.
>   Below reset in priority, because invite codes covered spam until now.
> - **Data export.** `http/server/export.go` exports catalog YAML, not history. A user
>   needs a JSON dump of their runs, ticks, journals and cycles. This is a legal right, not
>   a feature.
> - **A JWT token version, checked per request and bumped on password change and
>   logout-all.** Today a 30-day sliding token means changing a password does not kill a
>   stolen session.
> - Then drop the invite check from signup.

### Deliberately waiting

Admin UI — `sqlite3` until it hurts. Postgres — SQLite plus Litestream is fine to several
hundred active users on this write pattern, and migrating early is the classic pre-launch
time sink. The History metrics rework. Security headers and a body size limit, which are
cheap and can ride along with any ship.

And everything on the "explicitly not building" list. Public users will ask for a social
feed. The answer stays no. That list is the reason this app is good.
