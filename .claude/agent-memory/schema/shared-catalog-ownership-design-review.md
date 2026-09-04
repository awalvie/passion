---
name: shared-catalog-ownership-design-review
description: Full relational review of the owner_id=0-vs-NULL question for the shared-catalog project — recommends a separate Shared/Visibility column instead of repurposing OwnerID, and flags that child-row OwnerID filtering (not the encoding) is the real hard part
type: project
---

Context: 2026-09-03 design review requested directly (not a code change), before the
migration work in `docs/SHARED_CATALOG.md` starts. That doc had already converged on
`owner_id = 0` meaning shared through two prior reviews. This review pushes back on the
encoding, not the overall shape.

## Recommendation: don't repurpose OwnerID at all

Both candidates on the table (`owner_id = 0` for shared, `owner_id NULL` for shared)
overload one column with two concerns: who authored a row, and who may read it. Splitting
them removes the dilemma instead of picking a side:

- Keep `OwnerID uint NOT NULL` exactly as it is today — never 0, never null, always a real
  user id. It keeps meaning "who owns/authored this."
- Add `Shared bool NOT NULL DEFAULT false` (or a `Visibility string` if more states than
  two are plausible — this codebase already validates enum-like strings in Go, see
  `NormalizeKind` in `db/yaml_import.go:518`) on the three catalog tables only
  (`LibraryExercise`, `ActivityTemplate`, `SessionTemplate`).
- Query becomes `WHERE owner_id = ? OR shared = ?` — two-valued logic throughout, no `<>`/
  `!=`/`NOT IN` landmine anywhere, and a bug that fails to set the new column defaults to
  `false` (fails closed: the row is just private, not suddenly public).

This is strictly better than both options in the brief:
- vs `owner_id = 0`: a bug that leaves `OwnerID` unset already produces Go's zero value
  today (`uint` zero is 0) — with the 0-means-shared scheme that bug now means "leaked to
  everyone" instead of "orphaned to nobody." Decoupling ownership from visibility removes
  this specific failure mode entirely, because OwnerID stays meaningful and non-magic.
- vs `owner_id NULL`: no 3VL. None of the ~76 inlined `WHERE owner_id = ?` call sites in
  `db/queries.go`, and however many more in `http/server/`, need auditing for NULL-skipping
  comparison operators.

## A specific footgun this avoids: account-deletion tooling

`db/delete_users.go` treats every `OwnerID` as a real, deletable `users.id` —
`ownerScopedTables()` bulk-deletes every owner-scoped table `WHERE owner_id IN
(victimIDs)`, and victim ids come from `SELECT * FROM users WHERE id <> keepUserID`. If the
shared catalog's identity were ever made a *real* `User` row (a "system account"
approach, which is the natural alternative to a bare sentinel integer), it becomes a
completely ordinary victim of that tooling — an operator running "delete every account
except mine" wipes the entire shared catalog for everyone, in one call, with the existing
code working exactly as designed. `Shared bool` (or plain `owner_id = 0`, which also isn't
a real `users.id`) both dodge this; a real system-user-row design does not, unless
`ownerScopedTables()`/`PlanDeleteAllUsersExcept` grow an explicit guard. Flag this if a
"make the shared library a real account" design ever resurfaces.

## The actually hard part is not the encoding

Regardless of which of the three encodings is picked, none of them fix the real blocker,
which `docs/SHARED_CATALOG.md` already found (its "Blocker 1"): `Activity`, `Exercise`, and
`ExerciseMedia` all carry their own `OwnerID` (models.go:88, 132, 104), fully redundant
with their parent chain (`Activity.SessionTemplateID`, `Exercise.ActivityID` /
`ActivityTemplateID` / `SessionRunID`, `ExerciseMedia.ExerciseID` /
`LibraryExerciseID`). Every `Preload` closure that loads them
(`GetTemplateWithGraph` db/queries.go:53, `GetActivityTemplateWithExercises` db/queries.go:
207, plus the equivalents in `http/server/export.go` and `exercise_library.go:236`) filters
children by `owner_id = ownerID` — the *reader's* id. That was harmless when every row's
owner always equaled its parent's owner (the private-copy-per-user world). It silently
returns zero children the moment a shared parent (owned by the publisher, or by sentinel 0,
or flagged `Shared`) is read by a different user, because the children's `OwnerID` is the
publisher's, not the reader's.

Fix: once a parent has been authorized (mine or shared), stop filtering its children by
`owner_id` at all — scope by the parent id only (`activity_id = ?`, etc.), which is already
sufficient since the parent check already gated access. This is mechanical but real work:
every one of the ~76+ inlined owner-filter call sites in `db/queries.go`, plus the ones in
handlers, needs the same audit regardless of which visibility encoding wins. **Do not
remove the `OwnerID` column from `Activity`/`Exercise`/`ExerciseMedia`** — it's load-bearing
for `db/delete_users.go`'s `ownerScopedTables()` bulk-delete-by-owner mechanism, and
reworking that to cascade through joins is a separate, larger, unforced migration. Keep the
column as a redundant provenance cache; just stop trusting it for read-visibility on shared
parents.

## Live gap found in passing, unrelated to the encoding question

`http/server/exercise_library.go:335-347` (`case "delete":`) soft-deletes a
`LibraryExercise` with no reference check at all — no check against `Exercise
.LibraryExerciseID`, `CycleExerciseOverride.LibraryExerciseID`, or
`CycleExerciseWeekOverride.LibraryExerciseID`. Contrast `pruneCatalogOrphans` in
`db/yaml_import.go:188-236`, which carefully reference-counts all three before ever
hard-deleting a `LibraryExercise`. The UI path has no such guard on its soft-delete. Low
severity today (soft-delete is recoverable and downstream consumers already null-check the
optional pointer, so it degrades to "lost the link back to the library entry" rather than
corrupting data) but worth closing before the shared catalog makes a `LibraryExercise` more
widely referenced than it is today.

## Also verified while reading db/yaml_import.go

YAML-imported `Exercise` rows created via `ref:` (`upsertSessionTemplate`,
`upsertActivityTemplate`) never set `Exercise.LibraryExerciseID` — they copy the field
values from the parsed YAML exercise, not a resolved `LibraryExercise` row id. Only the
UI "add from library" path (`http/server/templates.go:23,93`) sets it. This means
`CycleExerciseOverride`/`CycleExerciseWeekOverride`'s "preferred match key"
(`LibraryExerciseID`) is unpopulated for the large majority of catalog-sourced exercises,
so the `ExerciseName` fallback (models.go:295, 312) is load-bearing, not a rarely-hit edge
case — and it silently stops matching the moment a catalog exercise is renamed (no error,
the override just goes quiet). Same root cause likely affects `exercise_history.go`'s
"same movement across templates" grouping, which also prefers `LibraryExerciseID` and
falls back to name.
