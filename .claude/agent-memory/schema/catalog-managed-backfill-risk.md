---
name: catalog-managed-backfill-risk
description: One-time bool backfills that assume "every existing row was created by process X" are unsafe once a UI path for creating that row already exists
type: feedback
---

Context: commit 8062a10 added `ManagedByCatalog bool` to `LibraryExercise`,
`ActivityTemplate`, `SessionTemplate`, plus a one-time `backfillCatalogManaged` migration
(`db/store.go`) that runs `UPDATE <table> SET managed_by_catalog = 1` unconditionally for
all three tables the first time the column is added — reasoning "every such row was
importer-created at that point." `pruneCatalogOrphans` (`db/yaml_import.go`) then hard-deletes
any `managed_by_catalog = true` row whose name isn't in the current YAML (subject to
reference-check guards).

The flaw: the app already has UI routes that create these exact rows outside the importer
— `/exercise-library/new`, `/templates/new`, `/activity-templates/new`
(`http/server/exercise_library.go`, `templates.go`, `activity_template_handlers.go`) — none
of which set `ManagedByCatalog`. So on any real (non-empty) prod DB, the backfill cannot
actually distinguish "importer-created" from "user-authored via UI"; it marks both, making
UI-authored content eligible for silent hard-deletion on the next import if its name isn't
in the YAML and (for ActivityTemplate especially, which has zero reference-check guard) it
isn't otherwise referenced.

**Rule for future reviews:** when a migration's correctness depends on an assumption like
"every row that exists right now was created by process X," check whether any *other*
code path can create that same row type. If yes, the assumption needs either (a) a
narrower backfill condition (e.g. matching known catalog names/sources rather than "all
rows"), or (b) an explicit opt-in flag that UI-creation always sets to false AND that
already existed before this migration (not newly added alongside it) — otherwise pre-existing
UI rows are indistinguishable from the thing being backfilled.

Also note: "diff the pruned DB against a from-scratch clean import, expect 0 stale
survivors" is not a valid verification method for this kind of bug — user-authored content
absent from the YAML is *also* absent from a clean import, so it looks identical to a
correctly-pruned rename-orphan in that diff. This kind of migration needs a targeted check
(e.g. list every `managed_by_catalog=true` row whose name never appeared in the YAML at
any historical commit, or cross-check against known catalog source YAML at the time of
first deploy) rather than a diff-based one.
