---
name: foreign-keys-not-enforced
description: PRAGMA foreign_keys is OFF on the app's SQLite connection, so declared OnDelete:CASCADE tags are inert
type: project
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
