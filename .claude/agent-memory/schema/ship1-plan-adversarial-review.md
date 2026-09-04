---
name: ship1-plan-adversarial-review
description: Adversarial review of docs/SHIP_1_PLAN.md before any code shipped — position-matching for children is unsafe, nullable owner_id has a SQLite NULL-uniqueness hole and a blank-slug collision hazard in the Phase C migration, and Phase E deletes the only import path with no replacement
type: project
---

Context: 2026-09-03, `docs/SHIP_1_PLAN.md` reviewed against the real code before any of it
was implemented (Phase A schema-only edits — `Slug` fields — landed on disk mid-review).
Supersedes nothing; complements `shared-catalog-ownership-design-review.md` (same day,
which recommended a `Shared bool` instead of nullable `OwnerID` — the plan proceeds with
nullable `OwnerID` anyway, so the 3VL hazards that review predicted are now concrete below).

## 1. Position-matching for children is unsafe, not merely imperfect

`upsertSessionTemplate` (`db/yaml_import.go:794-1112`) and `upsertActivityTemplate`
(`:628-747`) currently do **no matching at all** — full delete-then-recreate of every
child on every import. The plan's fix ("match by position") is worse than what it
replaces for one specific reason: delete+recreate orphans loudly (a detectable gap);
position-matching silently reattaches an existing row's stable id — and everything
pointing at it (completions, ticks, planned sets, choices) — to a *different* exercise's
content the moment the YAML reorders, inserts, or removes an item mid-list. No error.

Correct key: for a `ref:` entry, match on the referenced `LibraryExercise`'s `Slug` (or
`ActivityTemplate.Slug` for an activity `ref:`), via `(parent_id, library_exercise_id)`.
This requires a schema change the plan's Phase A bullet list doesn't include:
`Exercise.LibraryExerciseID` is **never set** when a row is created by resolving YAML
`ref:` (confirmed by grep — the only `LibraryExerciseID` hit in yaml_import.go is
`ExerciseMedia`'s polymorphic-owner field). Inline (non-`ref`) entries have **no stable
key at all** in the schema, planned or otherwise — the plan needs to either state that
limitation explicitly or add an authored `key:` YAML field.

## 2. Nullable owner_id — concrete breakage the plan misses

- **SQLite NULL-uniqueness hole**: a composite `unique(owner_id, slug)` index does **not**
  constrain rows where `owner_id IS NULL` — SQL treats NULL as never equal to NULL for
  uniqueness. Two different catalog rows could share a slug and the DB accepts both,
  defeating Phase A's core promise. Fix: a partial unique index,
  `CREATE UNIQUE INDEX ... WHERE owner_id IS NULL`, as raw SQL alongside `AutoMigrate`
  (the project already does this pattern in `db/store.go:87-146`). No `uniqueIndex` tag
  exists yet on the new `Slug` columns (`db/models.go:68,187,216`).
- **GORM zero-value trap for Phase C**: `Updates(struct{OwnerID: nil, ...})` (struct form,
  not map, not `Update(col,val)`, not `Save`) silently skips the nil pointer as a
  zero-value field — the promotion step would not actually null the column, no error.
  Must use `Update("owner_id", nil)` or a map. Today's upserts use `Save()` (safe), but a
  bulk promotion loop is likely to reach for `Updates(struct)` instead.
- **`delete_users.go` cross-phase hazard, critical**: `ownerScopedTables()`
  (`db/delete_users.go:21-47`) includes `Activity`/`Exercise`, which the plan keeps as
  `OwnerID uint NOT NULL` even after their parent is promoted to `owner_id = NULL`
  (`SHIP_1_PLAN.md:71-73`, "keep the column"). So a promoted `SessionTemplate`'s children
  still carry the donor account's real owner id. Deleting that donor via
  `--delete-users-except` (`cmd/passion/main.go:34,85-91`) hard-deletes
  (`Unscoped()`, `delete_users.go:133-137`) the shared catalog's own children out from
  under every other user, while the parent (owner_id NULL) survives pointing at nothing.
  Phase C must also repoint/neutralize children's owner_id under a promoted parent.
- Every owner-filtered call site (queries.go: ~12 functions; http/server/: exercise_
  library.go, activity_template_handlers.go, templates.go, export.go) needs auditing —
  the plan's "one read helper" is aspirational, not enumerated. No regression-guard test
  exists for this the way `catalog_edited_test.go` guards mutating routes.

## 3. Phase C migration — the real "safe to promote" predicate

The plan's framing ("rows a user created that happen to share a slug with a catalog row",
`:116-119`) understates the hazard. A UI-created row has `Slug == ""` (the not-null
default) — **every** untagged row across every user shares that literal blank value. A
naive "group by slug, promote one" collapses all of them into one slug-group and
discards the rest: mass, cross-account data loss, not a rare coincidence.

Concrete predicate:
- Include in a slug group at all: `ManagedByCatalog == true AND Slug != ""`.
- Must stay owned unconditionally: `ManagedByCatalog == false` OR `Slug == ""` OR
  `CatalogEditedAt != NULL`.

Also unhandled by the plan: if unstamped candidates for one slug disagree in content
(possible under partial rollout or differing import configs), diff/report rather than
picking one arbitrarily; if *zero* unstamped candidates exist for a slug (all 8 accounts
independently stamped it), there's nothing to promote and the plan doesn't say what
happens — that slug silently vanishes from the catalog.

## 4. Ordering — Phase E has no replacement import mechanism

`ImportYAML` rejects `opts.OwnerID == 0` (`db/yaml_import.go:103-105`) and every upsert
needs a concrete owner. Phase E retires both call sites that ever invoke it (signup,
boot re-import) with "there is nothing to import" — but nothing in the plan describes
*any* path that ever writes `owner_id = NULL` rows, including the initial seeding Phase
C's migration presupposes. Once E ships, there is no mechanism left to land new or
edited catalog YAML going forward, contradicting the project's own recent history of
ongoing catalog content changes. Needs its own designed mechanism before E is safe.

Phase B "ships alone" is true but inert: with zero NULL-owner rows pre-C, `CatalogVisible`
and the write guard are unreachable in practice — safe, but untestable against real data
until C runs. State this rather than implying B delivers value on its own.

## Also flagged

- `CopiedFromID` is used in prose (`:91,115`) but doesn't exist in the codebase and isn't
  in Phase A's schema-change list — needs its own line item.
- The plan says slug comes from the YAML filename (`:54-56`); the code already on disk
  (mid-review) uses an explicit `slug:` field instead, with a correct justification
  (duplicate basenames across public/private trees; some files hold lists). Plan text is
  already stale against its own implementation.
