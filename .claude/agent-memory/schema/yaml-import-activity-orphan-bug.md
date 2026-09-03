---
name: yaml-import-activity-orphan-bug
description: db/yaml_import.go:817 soft-deletes Activity on every restart without cascading to its Exercises/Media — verified against the real dev DB (80k+ orphaned live exercise rows). Sibling delete sites in the same file do NOT have this bug despite also being soft-deletes.
type: project
---

**Verified 2026-09-03** against the live dev `passion.db` (173MB at time of review).

## The bug

`upsertSessionTemplate` at `db/yaml_import.go:817` does:

```go
tx.Where("owner_id = ? AND session_template_id = ?", ownerID, template.ID).Delete(&Activity{})
```

No `Unscoped()` → soft delete (`UPDATE activities SET deleted_at = ?`). `Activity.Exercises`
carries `constraint:OnDelete:CASCADE`, but that constraint is a SQL-level `ON DELETE CASCADE`
clause — it only fires on a real `DELETE`, never on this `UPDATE`. The loop right after this
line creates a **fresh** set of Activities+Exercises every restart. Old Exercise rows (and
their ExerciseMedia) are never touched: they stay live, still pointing at a now-soft-deleted
(or, once a template gets pruned, entirely gone) `activity_id`.

Confirmed via direct SQL against the dev DB: of ~76-80k live `exercises` rows tied to an
`activity_id`, only ~2,750 resolve to a live Activity. The rest (~97%) are dead weight from
past restarts. Growth is roughly one full copy of every managed template's exercise tree per
restart — this DB was re-imported dozens of times over about 6 weeks.

Second-order symptom: `http/server/runs.go` `buildRunSteps` (called from `renderRun`) walks
`ScheduledSession.SessionTemplate.Activities.Exercises` via `db.GetScheduledSessionWithTemplate`
(`db/queries.go:30`), which `Preload`s with the default soft-delete scope. When the Activity a
past run's completions point at has been soft-deleted (by a later reimport), that run's
`RunExerciseCompletion` rows survive but the per-exercise detail renders empty — confirmed 84
of 113 live completions affected at review time.

## Why the sibling delete sites in the same file do NOT have this bug

- `upsertActivityTemplate` (`db/yaml_import.go:647-650`): `Delete(&Exercise{})` targets the
  Exercise rows **directly** (not via a parent's cascade). Soft-delete correctly marks exactly
  the rows that need removing. No orphaning — confirmed 0 live orphans under activity_template_id
  in the dev DB. (It does still leak unbounded soft-deleted rows over time — bloat, not
  corruption — and it never calls `createExerciseMedia` for these exercises at all, so there's
  no media to leak here either.)
- `upsertLibraryExercise` (`db/yaml_import.go:785`): same pattern, `Delete(&ExerciseMedia{})`
  targets the media rows directly. No orphaning. (Missing an `owner_id` filter on this one query
  — harmless in practice since `library_exercise_id` values are already owner-unique, but
  inconsistent with every other query in the file.)
- `pruneCatalogOrphans`/`deleteExercisesAndMedia` (same file, the once-per-name-removal path):
  already uses `Unscoped()` hard deletes, child-first, and reference-counts before deleting a
  `LibraryExercise`/`SessionTemplate` parent. This path is correct and is the pattern the
  restart-time replace-children code at :817 should have followed.

**Rule of thumb for this codebase:** a soft-delete is only safe when it targets the exact table
whose rows must disappear. A soft-delete of a *parent* row, relying on `constraint:OnDelete:CASCADE`
to clean up children, is silent corruption — the cascade never fires. This is the same lesson as
`foreign-keys-not-enforced.md`/`fk-pragma-rollout-verified.md` (soft-deletes are inert to FK
checks) applied one level up: it's inert to GORM's own cascade tags too, not just SQLite's FK
enforcement.

## Cleanup predicate (for the one-off backfill, not yet applied)

Orphan candidate: `exercises.deleted_at IS NULL AND activity_id IS NOT NULL AND activity_id NOT IN
(SELECT id FROM activities WHERE deleted_at IS NULL)` — covers both soft-deleted-activity and
activity-row-entirely-gone cases.

Must NOT hard-delete an orphan candidate if it's referenced by ANY of: `run_exercise_completions.exercise_id`,
`climbing_ticks.exercise_id`, `manual_exercise_set_logs.exercise_id`, `exercise_planned_sets.exercise_id`,
`run_exercise_choices.parent_exercise_id` OR `chosen_exercise_id`, `climbing_exercise_meta.exercise_id`.
None of these six tables have a declared FK to `exercises` (bare `uint` columns, consistent with
the rest of the schema per `fk-pragma-rollout-verified.md`), so `_foreign_keys=on` will not catch
or block a bad delete here — the application code is the only thing protecting these rows.
At review time this predicate found ~80k safe-to-purge candidates vs. ~100 that must be kept.

Bulk-delete the safe set in one `Unscoped().Where("id IN (?)", ids).Delete(&Exercise{})` statement
(not a row-by-row loop) — `fk-pragma-rollout-verified.md` confirms a single bulk statement that
removes both `exercise_catalog` parents and their `parent_exercise_id` children together succeeds
even though that self-ref FK is `NO ACTION`; a row-by-row loop would need children-before-parents
ordering to avoid a constraint failure. ExerciseMedia cleans up automatically via the existing
`exercises.id → exercise_media.exercise_id ON DELETE CASCADE`.
