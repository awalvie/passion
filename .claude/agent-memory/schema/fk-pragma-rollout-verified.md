---
name: fk-pragma-rollout-verified
description: Empirical results of testing `_foreign_keys=on` against this schema — which relations actually have declared constraints, and how soft-delete interacts with them. UPDATE 2026-08-27 — this was adopted for real in commit 7c95a4c; findings below are now live production behavior, not a proposal.
type: project
---

**UPDATE 2026-08-27:** this file's findings were written for an advisory review, but the
proposal was subsequently adopted — commit `7c95a4c` ("db: enable sqlite foreign key
enforcement") added `_foreign_keys=on` to the DSN in `db/store.go`, still present on
`master` as of `4f54aaa`. Everything below is now real, live connection behavior, not a
hypothetical. Practical consequence for future reviews: adding a *new* GORM association
(belongs_to/has_many with a real foreign key relationship, not a bare `uint` column) now
means AutoMigrate will emit a real `REFERENCES` clause that SQLite enforces — inserts
with a dangling value on that column will now fail loudly, and hard-deleting a parent
row that still has children on a non-CASCADE relation will now fail instead of silently
orphaning them. Prefer plain undeclared `uint`/`*uint` columns (no GORM association tag)
for new cross-references unless real DB-level enforcement is actually wanted — that
keeps the new column's behavior consistent with the vast majority of existing
cross-references in this schema (RunID, ScheduledSessionID, TrainingCycleID on the
override/goal tables, etc.), none of which are FK-declared.

Context: 2026-07-10 advisory review of "enable SQLite foreign keys" proposal. Built a
throwaway harness (`gorm.Open` against a temp file, `db.Store.AutoMigrate()`, then
`PRAGMA foreign_key_list(<table>)` for every table) to see what AutoMigrate actually
puts in the DDL, then a second harness with `?_foreign_keys=on` to test real delete/insert
behavior. Findings, in case this proposal resurfaces or a new model is added:

**Only 9 FK constraints exist in the schema at all** (everything else — `run_id`,
`scheduled_session_id` on cycle/journal/tick/log tables, `owner_id` everywhere, etc. —
is a plain `uint`/`*uint` column with no GORM association declared, so AutoMigrate never
emits a `REFERENCES` clause for it, and no pragma can ever enforce something that isn't
in the DDL):
- `activities.session_template_id → session_templates.id` CASCADE
- `exercises.activity_template_id → activity_templates.id` CASCADE
- `exercises.activity_id → activities.id` CASCADE
- `exercises.parent_exercise_id → exercises.id` NO ACTION (self-ref)
- `library_exercises.parent_library_exercise_id → library_exercises.id` NO ACTION (self-ref)
- `exercise_media.exercise_id → exercises.id` CASCADE
- `exercise_media.library_exercise_id → library_exercises.id` CASCADE
- `training_cycle_weekday_mappings.training_cycle_id → training_cycles.id` CASCADE
- `scheduled_sessions.session_template_id → session_templates.id` CASCADE

**Soft-deletes never trigger FK checks, pragma or no pragma** — GORM's default
`.Delete()` on a `gorm.Model` issues `UPDATE ... SET deleted_at = ?`, not a `DELETE`.
SQLite only checks FK constraints on real `DELETE`/key-changing `UPDATE` statements.
This means turning the pragma on changes behavior **only** at the handful of
`Unscoped().Delete(...)` call sites (`http/server/templates.go` deleteTemplate,
`http/server/activity_template_handlers.go` deleteActivityTemplate, and
`db/yaml_import.go` pruneCatalogOrphans/deleteExercisesAndMedia) — every other delete
handler in `http/server/` (activity, exercise, run, library-exercise, calendar, tick,
manual-log deletes) is a soft delete and is completely unaffected by the pragma.

**Verified empirically (throwaway harness, not committed):**
- A single bulk `DELETE FROM exercises WHERE activity_id = ?` that removes both a
  self-referencing parent and child row *in the same statement* succeeds with the
  pragma on — SQLite doesn't choke on same-statement self-FK removal. This matches
  the existing bulk-delete pattern in `deleteTemplate`/`deleteActivityTemplate`.
- Hard-deleting an exercise via `Unscoped()` while its `exercise_media` rows still
  exist now cascades and removes them automatically (0 left over) — this is a genuine
  **improvement**: today `deleteTemplate` and `deleteActivityTemplate` never clean up
  `ExerciseMedia` for the exercises they hard-delete, so those rows are orphaned right
  now, silently, on every template/activity-template delete.
- Deleting a single "parent" exercise via `Unscoped()` while a child still references
  it through `parent_exercise_id` (self-ref, NO ACTION) **fails** with "FOREIGN KEY
  constraint failed" once the pragma is on. No current code path does this in isolation
  (both cascade-delete call sites remove parent+children together in one statement, and
  `pruneCatalogOrphans` explicitly counts child rows before deleting a `LibraryExercise`
  parent), but any future code that hard-deletes a single exercise-catalog parent without
  its children would need to handle this new error.
- Flipping the pragma on an **already-inconsistent** existing DB file is safe at the
  connection level: opening, reading, and updating unrelated columns on rows with a
  dangling FK value all succeed with no error. SQLite does not retroactively validate
  existing data when the pragma is enabled (unlike e.g. Postgres `ADD CONSTRAINT
  VALIDATE`). `PRAGMA foreign_key_check` can be run at any time, pragma on or off, to
  list existing violations without erroring the connection — this is the right tool for
  a pre-flip audit, but it only covers the 9 relations above; everything else needs a
  hand-written `NOT EXISTS` query.
- The one real behavior change to plan for: **new inserts** with a dangling FK value on
  one of the 9 constrained columns start failing immediately once the pragma is on
  (previously silent). This is desirable (turns future silent corruption into a loud
  bug) but means any latent ordering bug in create paths would now surface as a 500.
