---
name: ship1-decisions-1-and-2
description: Decided — owner encoding is Shared bool (not nullable OwnerID or a system User row); slug comes from an explicit YAML field, hard error if missing, DB backfill-by-name must precede the slug-based lookup switch
type: project
---

Context: 2026-09-04, asked directly to decide (not just review) the two open questions in
`docs/SHIP_1_PLAN.md` before any Phase B/C code lands. Supersedes the "which encoding"
part of `shared-catalog-ownership-design-review.md` and `ship1-plan-adversarial-review.md`
by picking a side; the hazards those two documents found are still accurate and are the
justification below.

## Decision 1: `Shared bool`, not nullable `OwnerID`, not a system `User` row

`OwnerID uint NOT NULL` stays exactly as it is on every model, including
`Activity`/`Exercise`/`ExerciseMedia`. Add `Shared bool `gorm:"not null;default:false;index"``
to `LibraryExercise`, `ActivityTemplate`, `SessionTemplate` only. Nothing else changes type.

Why this wins over nullable `OwnerID`:
- `unique(owner_id, slug)` keeps working with zero changes — no partial index. All shared
  rows share one `owner_id` (the catalog account), so per-owner uniqueness already implies
  global uniqueness among shared rows. This also answers the plan's open question 2
  ("slug unique per owner or globally for catalog rows") — per-owner is enough, for free.
- No 3VL. `WHERE owner_id = ? OR shared = ?` needs no audit of `<>`/`NOT IN` call sites.
- A forgotten `OwnerID` (Go zero-value 0) still produces an unreachable row, same as the
  nullable scheme — but fails closed (invisible to everyone) instead of failing open
  (nullable scheme risked "0 means catalog" style bugs if anyone ever chose that spelling).
- Zero blast radius on `config.Validate()`'s zero-DemoOwnerID/YAMLImport.OwnerID checks or
  `http/server/auth.go`'s zero-UserID check — `OwnerID`'s type and meaning never change.

Why not a system `User` row: it doesn't remove the `delete_users.go` hazard, it just moves
it — any real account that owns shared rows is an ordinary victim of
`--delete-users-except` (`db/delete_users.go`'s `ownerScopedTables()` bulk-deletes by
`owner_id IN victimIDs`) whether that account is dressed up as a "system user" or is just
"the account the importer happens to run as." The fix is the same either way: guard
`delete_users.go` explicitly. A fake login-capable row buys nothing and adds a new class
of thing every user-list/admin path must special-case.

**delete_users.go must change before any row is ever flagged `Shared = true`:**
1. The account configured as the catalog owner (today `cfg.YAMLImport.OwnerID`) must never
   be a deletion victim — thread it into `PlanDeleteAllUsersExcept`/`DeleteAllUsersExcept`
   as a second protected id alongside `keepUserID`, refuse or auto-exclude it.
2. Defense in depth: the three catalog tables' bulk-delete condition becomes
   `owner_id IN ? AND shared = 0`, not bare `owner_id IN ?`.
3. Their children (`Activity`, `Exercise`, `ExerciseMedia` — no `Shared` column of their
   own) need a parent-shared exclusion subquery, e.g. for `Activity`:
   `owner_id IN ? AND session_template_id NOT IN (SELECT id FROM session_templates WHERE shared = 1)`.
   `Exercise` has three possible parents (`activity_id`, `activity_template_id`,
   `session_run_id`); only the first two ever chain to a `Shared` row.
4. Add a regression test: flag a non-kept account's row `Shared = true`, run
   `DeleteAllUsersExcept` naming a different keep id, assert the shared row and its
   children survive. `TestOwnerScopedTablesCoversEveryTableWithAnOwner` (delete_users_test.go:198)
   guards the table list; nothing today guards the shared-row exclusion.

**Migration mechanics (Phase C), once Decision 2's slugs are stable:** promotion is a
single `Update("shared", true)` per chosen row (column form, not `Updates(struct{})` —
the struct form's zero-value skip would silently no-op a `false`→`true`... actually the
zero-value trap bites the opposite direction, skipping a *false* write; using column form
throughout avoids ever having to reason about it). Because `OwnerID` never moves, promotion
never creates a parent/child owner mismatch — the exact "donor account deleted, children
orphaned" scenario the adversarial review found under the nullable-OwnerID design does not
exist under `Shared bool`, because children were never repointed in the first place.

**Read helper** (only for the three catalog models — private-only models keep plain
`owner_id = ?`):
```go
func CatalogVisible(ownerID uint) func(*gorm.DB) *gorm.DB {
    return func(tx *gorm.DB) *gorm.DB {
        return tx.Where("owner_id = ? OR shared = ?", ownerID, true)
    }
}
```
Children (`Activity`, `Exercise`, `ExerciseMedia`) drop `owner_id = ownerID` from every
`Preload` closure entirely — scope by parent id only, per
`shared-catalog-ownership-design-review.md`'s original finding. That finding is still the
hard part regardless of encoding; ~76+ call sites in `db/queries.go` plus handlers need
the same audit no matter which of the three candidates won.

**Write guard**: every UPDATE/DELETE on the three catalog models adds `AND shared = 0` to
its WHERE and turns a 0-rows-affected result into a visible error, not a silent no-op.
Confirmed two live silent-no-op sites that need this:
`http/server/exercise_library.go`'s `case "delete":` (~337-353) and
`http/server/catalog_edited.go`'s `stampCatalogEdited`/`resetCatalogRow` (~106-118).

## Decision 2: slug is an explicit YAML field, hard error if missing; DB backfill-by-name must run before the lookup switch

Confirmed live: none of the 102 public `catalog/*.yaml` files currently set `slug:` (all
grep as slug-less); the field is parsed into the `yamlExercise`/`yamlSessionTemplate`/
`yamlActivityTemplate` structs today and silently ignored — safe to add slugs to YAML
right now with zero behavior change, independent of any Go code change.

**Hard error, not soft fallback**, confirmed correct: a soft fallback (derive from `name`
when `slug:` is blank) would recompute a *different* slug the moment someone renames the
one field this whole feature exists to free from being an identity — silently reproducing
delete-then-recreate for exactly the files nobody bothered to tag, discovered only when
someone renames one, not when the file is added. Hard error catches it at file-add time
(CI-visible) instead of rename time (invisible drift).

**Child matching** (`yamlSessionExercise` has no slug field):
- `ref:` children/exercises: match by `(parent_id, LibraryExerciseID)`. Requires actually
  setting `Exercise.LibraryExerciseID` when resolving a `ref:` — confirmed today this is
  **never** set by the importer (only the UI "add from library" path sets it,
  `http/server/templates.go`), so `upsertActivityTemplate`/`upsertSessionTemplate` need
  this line added alongside the matching-key switch. This also fixes the pre-existing,
  already-flagged bug where `CycleExerciseOverride`/`CycleExerciseWeekOverride`'s
  preferred match key is unpopulated for catalog-sourced exercises.
- Inline (non-ref) children: match by `(parent_id, Name)`. Verified today: zero duplicate
  exercise names within any one activity or activity template, so this is safe now. It is
  a **known, accepted limitation**, not a fix: renaming an inline child is still
  indistinguishable from delete+create. Deliberately not solving this with an authored
  `key:` field per child — doubles the YAML diff (225 files → every child block) for a
  low-frequency edge case, when the high-value fix (ref'd children, the majority in a
  catalog built on a shared exercise library) is cheap and available now.

**Exact deploy sequence** (the two repos are on independent timelines; the public repo's
`.github/workflows/deploy.yml` is the only thing that pulls both together, on every push
to this repo's `master`, and imports both trees in one boot):

1. Slug-backfill YAML in `passion-private-catalog` (123 files) — data only, any time,
   no dependency on this repo, safe under today's code because `slug:` is parsed and
   ignored.
2. Slug-backfill YAML in this repo's `catalog/` (102 files) — same, any time, either order
   relative to step 1.
3. **Verify, before step 4, that both trees are 100% slugged with zero collisions.** This
   is a manual/CI gate, not something the public repo's own test suite can see for the
   private tree — `TestPublishedCatalogImportsOnItsOwn` (db/catalog_published_test.go)
   only imports `catalog/`, not `catalog-private/`.
4. One deploy that, in this internal order inside `AutoMigrate`/boot:
   a. Backfills the DB: `UPDATE <table> SET slug = <slugify(name)> WHERE slug = ''`,
      same slugify function that generated the committed YAML slugs, matched by the
      *current* `Name` (still 1:1 today — verified zero collisions). Pattern precedent:
      `Store.backfillCatalogManaged` in `db/store.go`.
   b. Switches `upsertLibraryExercise`/`upsertActivityTemplate`/`upsertSessionTemplate`'s
      lookup from `Where("owner_id = ? AND name = ?")` to `Where("owner_id = ? AND slug = ?")`,
      and `pruneCatalogOrphans`'s keep-set from Name to Slug.
   c. Adds hard-error validation for a blank slug.
   d. Sets `Exercise.LibraryExerciseID` on ref resolution; switches ref'd-child matching
      to `(parent_id, LibraryExerciseID)`.
   Because (a) ran first, every existing row already carries the same slug the YAML now
   declares, so (b)'s first slug-aware import matches and updates in place — no new rows,
   no id churn.
5. Watch: row counts for the three tables unchanged before/after. Spot-check a few ids.
6. **Only in a separate, later deploy**, add `uniqueIndex` to `Slug` (composite with
   `OwnerID`) and let `AutoMigrate` create it — confirmed live: adding the index before
   the backfill fails, and `AutoMigrate` failure exits the process at startup
   (`db/store.go:28` → `NewSqlite` → `main.go`'s unconditional `os.Exit(1)` on error).
   Isolating the index creation from the cutover deploy means an unexpected collision
   (if the zero-collision verification missed something) fails a low-stakes follow-up
   deploy, not the cutover itself.

**What breaks if the order is wrong** (highest severity first):
- Step 4 shipped before step 3 confirms the private repo is fully slugged: the very next
  boot hard-errors on the private tree's un-slugged files, `ImportYAML` returns an error,
  `main.go` calls `os.Exit(1)`, and the systemd unit (`Restart=on-failure`, `RestartSec=5`)
  crash-loops every 5 seconds until someone fixes the private repo and redeploys. The
  public repo's own CI cannot catch this — the private repo's readiness is an external,
  unverified precondition, not something `go test` here sees.
- Step 4a (DB backfill) shipped in a *later* deploy than 4b (slug-based lookup): the first
  slug-aware boot finds `slug = ''` in the DB against non-empty slugs in YAML, matches
  nothing for any of the 1,619 rows, and either mass-creates 1,619 duplicate rows (if
  nothing else stops it) or fails outright — the single worst-case instance of the exact
  bug this whole project exists to fix, applied to the entire catalog at once instead of
  one renamed row at a time.
- Step 6 (uniqueIndex) done before step 4a (backfill): confirmed fact — `AutoMigrate`
  tries to build a unique index over ~1,619 rows nearly all sharing `slug = ''`, fails
  immediately, and the process never reaches `ImportYAML` — worse than the validation
  failure case, because it blocks boot before even serving existing users' unrelated data.
