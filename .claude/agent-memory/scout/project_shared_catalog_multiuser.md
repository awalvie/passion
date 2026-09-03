---
name: Shared catalog vs per-user copies (multi-user)
description: Consultation on whether the exercise library / activity templates / session templates should be one shared catalog or duplicated per user
type: project
---

# Question (2026-09-02, RECOMMENDED — not yet decided by owner)

Passion is going multi-user. Today every table is `OwnerID`-scoped and each
new signup gets a full duplicate of `catalog/` (exercises, activity_templates,
session_templates) via `ImportYAML(OwnerID: n)`. Owner asked: one shared
library everyone reads, plus private user items on top?

## Key code facts that shaped the answer

- `Exercise` rows are already **copies**. A session template's activity owns its
  own `Exercise` rows and only *links back* via `LibraryExerciseID`.
- All run history (`RunExerciseCompletion`, `ExercisePlannedSet`,
  `ManualExerciseSetLog`, `ClimbingExerciseMeta`) references `Exercise`, never
  `LibraryExercise`. So the library is a **picker**, not a history FK.
  → The "history breaks if the shared item changes" argument is already
  defused by the existing structure.
- Layered override resolution already exists:
  `CycleExerciseWeekOverride → CycleExerciseOverride → template default`.
  Per-user parameter customisation therefore already has a home *downstream*
  of the library. The library does not need per-user defaults.
- `ImportYAML` matches on `(owner_id, name)` and prunes `ManagedByCatalog`
  rows that nothing references.

## Recommendation given

Shared read-only catalog (`OwnerID = 0` / `IsCatalog`), plus private per-user
additions, plus a tiny per-user hide flag. Do NOT build per-user field
overrides on library rows — cycle overrides cover it.

Industry precedent: Strong / Hevy / JEFIT / MacroFactor / RP / Juggernaut all
ship one canonical library + user customs. TrainingPeaks and Boostcamp copy
program content onto the calendar at apply time (same as Passion's
`Exercise` copy). No mainstream app duplicates its whole database per user.

## The named failure mode

Identity churn on shared rows: the importer keys on `name`, so a rename in
YAML looks like "delete old + create new". With a shared catalog that breaks
*other* users' private templates and overrides. Fix = stable `slug` as the
import key, and soft-retire (`RetiredAt`) instead of delete.

Second flag raised: the current catalog contains the owner's personal coaching
notes and cycle-1-specific content. Sharing it makes one climber's opinions
everyone's defaults.

## Update 2026-09-03 — the design drifted, and I pushed back

`docs/SHARED_CATALOG.md` now settles on **copy-on-edit** (editing a shared row
copies it to the user, storing the shared row's id on the copy). That is a move
away from the recommendation above.

I argued to move partway back. See
[project_public_launch_readiness.md](project_public_launch_readiness.md) for the
full case. Short version: `CycleExerciseOverride` already owns every numeric
field, so copy-on-edit uniquely buys only name/notes/out-of-cycle defaults — all
cheaper as one `UserExercisePref` table. And a copy freezes that user out of
catalog improvements permanently, which is the `CatalogEditedAt` bug made
permanent. No mainstream app forks on edit; they make built-ins immutable and
make **Duplicate** an explicit action.

The identity-churn failure named above is now a **hard blocker**, not a nice-to-have:
`CycleExerciseOverride.LibraryExerciseID` is an unenforced `*uint`, so a YAML
rename under a shared catalog silently dangles other users' overrides.
