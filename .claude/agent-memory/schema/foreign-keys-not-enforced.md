---
name: foreign-keys-not-enforced
description: SUPERSEDED 2026-08-27 — the pragma this file describes as OFF was turned ON in commit 7c95a4c ("db: enable sqlite foreign key enforcement"). See fk-pragma-rollout-verified.md for current behavior.
type: project
---

**SUPERSEDED as of 2026-08-27.** This file documented `PRAGMA foreign_keys` as OFF.
That has since changed: commit `7c95a4c` ("db: enable sqlite foreign key enforcement")
added `_foreign_keys=on` to the DSN in `db.NewSqlite` (`db/store.go`), and it is still
there as of the current `master`. The advisory review this file's sibling
(`fk-pragma-rollout-verified.md`) was written for was actually adopted.

**Do not cite this file's claim that "every constraint:OnDelete:CASCADE tag is
decorative" as current fact.** For the 9 relations listed in
`fk-pragma-rollout-verified.md`, CASCADE now fires for real on `Unscoped().Delete()` —
and, new implication not yet verified empirically: a hard delete of a *parent* row that
still has children referencing it through a plain (non-CASCADE, no explicit ON DELETE)
declared FK will now fail with "FOREIGN KEY constraint failed" instead of silently
orphaning them, IF that relation is one of the 9 with a real declared constraint. For
any of the many *undeclared* `uint`/`*uint` cross-references (RunID, ScheduledSessionID,
TrainingCycleID on CycleGoal, etc.) nothing changed — no REFERENCES clause was ever
emitted for those, so the pragma has no effect on them either way; explicit app-code
cleanup is still required exactly as before.

Original (now-outdated) text follows, kept for the empirical repro details:

---

Empirically verified (2026-07-10, reviewing the ManagedByCatalog/prune-on-import change):
`db.NewSqlite` opens the connection via `sqlite.Open(path)` with no DSN pragma params.
`PRAGMA foreign_keys` reads back as `0` (OFF) on a freshly-migrated DB.

This contradicts this agent's own "Database conventions" note that "GORM does this via
DSN params" — that is not true for this codebase as of this review. Confirmed by a live
repro: deleting a `SessionTemplate` row directly (`Unscoped().Delete(&SessionTemplate{}, id)`)
left its `Activity` row behind, even though `Activity.SessionTemplateID` is declared
`constraint:OnDelete:CASCADE` and `PRAGMA foreign_key_list(activities)` does show the
constraint in the schema DDL. The constraint exists as metadata but SQLite never enforces
it for this connection.

**Implications for every future schema review:**
- Every `constraint:OnDelete:CASCADE` tag anywhere in `db/models.go` is decorative only.
  Any code path that deletes a parent row and assumes children vanish automatically is
  wrong — children will be left behind as real orphans, silently, with no error.
- Conversely, self-referential/NO-ACTION-only FKs (e.g. `LibraryExercise.ParentLibraryExerciseID`)
  never block a delete either. Deleting a parent while a child still references it does
  NOT error — it silently leaves the child with a dangling FK value (confirmed by repro).
- Whenever reviewing a delete path, check that cleanup of dependent rows is done
  explicitly in application code (as `pruneCatalogOrphans` and `deleteTemplate` in
  `http/server/templates.go` both already do) — never credit a `constraint:OnDelete:CASCADE`
  tag as an actual safety net until this pragma situation changes.
- If asked to weigh in on turning `PRAGMA foreign_keys` ON: flag that this is a
  significant behavior change (some existing delete flows may currently rely, whether
  knowingly or not, on cascades NOT firing) and should be a deliberate, separately-reviewed
  decision, not a drive-by fix bundled into an unrelated change.
