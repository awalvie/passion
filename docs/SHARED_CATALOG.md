# Shared catalog — where we got to, and what is next

Written 2026-09-03. Hand-off notes, so a fresh session can pick this up cold.

## The goal, in the owner's words

- Other people can self-host this.
- Later, he hosts it on a domain so others can use it.
- Content from paid programmes reaches his account only, not everyone's.
- One shared library. No copying the whole catalog for every user.
- People can edit. An edit becomes theirs and only they see it.

## The agreed design

Ownership does all the work.

| `owner_id` | Who sees it | What it is |
|---|---|---|
| `0` | everyone | the shared library, read-only |
| a real user | that user only | their edit, or their own imported catalog |

Four rules:

1. Exercises, activity templates and session templates have one shared copy.
2. Anyone can edit one.
3. Editing copies the row to that user. Their copy replaces the shared one in their own
   lists. Nobody else sees it.
4. A user's own imported catalog works the same way. Those rows are theirs from the start,
   so every existing owner-scoped query already hides them from other people.

Rules 3 and 4 are the same rule, which is why this is simpler than it looked.

### Do not match copies by name

The owner spotted this. When a user's copy replaces a shared row, the app has to know
*which* shared row it replaces.

- Two users picking the same name is harmless. Rows are owned, so they never see each
  other's.
- But within one user, name cannot tell these apart:
  - "this is my edited version of the shared Max Hangs" — must hide the shared row
  - "this is a different thing I happened to call Max Hangs" — must not hide anything

So store the shared row's id on the copy when the copy is made. Hiding is then exact: the
shared row disappears only when the user holds a copy pointing at it. It also means a user
can rename their copy freely and it still replaces the right original.

## What already shipped

Pushed and deployed. Production is healthy: service active, site 200, 102 published YAML
files and 123 private, no errors.

```
6da4fa1  docs: explain editing a catalog item
75dda75  db: test that the published catalog imports alone
e527218  templates: add sessions the public catalog can run
dd85836  library: add a warm-up and drills of our own
4e29e9e  templates: show which catalog items you edited
cc40e22  *: stamp catalog rows edited in the app
50021de  db: keep catalog rows a user has edited
```

In `passion-private-catalog`: `299054f catalog: rename the paradigm warm-up and drills`.

### The Edited flag

`CatalogEditedAt *time.Time` on `LibraryExercise`, `ActivityTemplate`, `SessionTemplate`.
A stamped row is skipped by the three `upsert*` functions and by `pruneCatalogOrphans`.
There is an "Edited" chip in the lists and a "Reset to catalog" button on the edit pages.

This fixed two live bugs: an edit made in the app was undone on the next restart, and
renaming a catalog row deleted it.

**Its job changes under the new design.** For shared rows there is nothing to stamp — a
user gets their own copy instead. It still earns its place for a user's *own* imported
rows, because the importer would otherwise overwrite those.

`http/server/catalog_edited_test.go` holds a table with one case per mutating route. Keep
it. Adding a route that touches a catalog row without adding a table entry is meant to
read as a visible gap.

### The published catalog

`Warm Up` and `Drills` now exist in `catalog/`, written from scratch. The warm-up specifies
12:38 of work — the programme one it replaces claimed 15 minutes and specified 30 to 40,
which is why it never got run.

Three published sessions: Boulder Session, Fingers & Strength, Mobility Day. Six new
exercises. `Strength Base Session` deleted.

The private blocks are now `Warm Up (Paradigm)` and `Drills (Paradigm)`, and the ten
sessions that referenced them were repointed. **His four training sessions still run the
programme content, unchanged.** Switching one to the new warm-up is a one-line `ref` edit.

`db/catalog_published_test.go` imports `catalog/` alone and asserts nothing came out empty.
That is what keeps self-hosting working.

## The four known blockers

Found by two reviews. All still true.

1. **Child rows are fetched by the reader's own id.** `GetTemplateWithGraph`
   (`db/queries.go:53-72`) and `GetActivityTemplateWithExercises` (`:207-223`) filter their
   Preload closures by `owner_id = ownerID`. The importer gives children the same owner as
   their parent. So widening only the top-level query makes a shared session open **with no
   activities and no exercises, and no error**. Same pattern in `http/server/export.go`
   (~`:189`, ~`:298`) and `exercise_library.go:236` for catalog children.

   This is the one that wastes a day if nobody knows. The failure is silence.

2. **`ImportYAML` refuses owner 0.** `db/yaml_import.go:99-102`. Change the guard, do not
   delete it — it is what stops a missing config value importing into owner 0 by accident.

3. **The migration would abort.** `LibraryExercise.ParentLibraryExerciseID`
   (`db/models.go:185`) is a real enforced foreign key with `NO ACTION`, and
   `_foreign_keys=on` is set in `db/store.go`. Deleting a parent while a child points at it
   fails and, inside one transaction, takes the whole run down. `pruneCatalogOrphans`
   already guards this exact case by counting children first — copy that.

4. **A config value can create a colliding user.** `EnsureSeedUser` (`db/seed.go:204-220`)
   force-sets the primary key from `cfg.Server.DemoOwnerID`, and `Config.Validate()` never
   checks it is non-zero. So `DemoOwnerID: 0` makes a real, log-in-able user with id 0.
   Reject zero in `Validate()`.

## Order of work

Both reviewers said the same thing: do not build the migration in the same pass.

**Step 1 — the read side.** Widen every read of the three models to "mine or shared",
including **every Preload closure and detail fetch**. Keep every write at `owner_id = me`,
and add an explicit `if row.OwnerID != ownerID` guard after each fetch-then-save. Today
that safety rests on three handlers each spelling out their own WHERE
(`templates.go:484`, `activity_template_handlers.go:179`, `exercise_library.go:292`), and
GORM's `Save()` writes whatever row it is handed.

Both reviewers independently suggested collapsing every read of these three models behind
the query helpers, so no handler writes its own owner filter. Then there is one place per
model to get right instead of dozens.

**Then look at it.** Open a shared session in a browser, run it, export it. Blocker 1
fails silently, so it has to be seen, not just tested.

**Step 2 — the import.** Published files load once as the shared owner. Private files load
once as the entitled account. Signup stops importing entirely (`http/server/auth.go:117`) —
a new user reads the shared library, so there is nothing to copy. **This is where the live
leak closes.**

**Step 3 — copy-on-edit.** When a user edits a shared row, copy it to them and store the
shared row's id on the copy. Lists prefer the copy and hide the original.

**Step 4 — the migration.** A command he runs and watches, not a boot step. Never
unsupervised: production holds his real training history. Skip rows with `CatalogEditedAt`
set — those are his. Repoint every foreign key that references a `LibraryExercise`:
`Exercise`, `CycleExerciseOverride`, `CycleExerciseWeekOverride`, `ExerciseMedia`, and
`LibraryExercise.ParentLibraryExerciseID` (blocker 3).

### Two things that are already settled

**No history backfill.** `RunExerciseCompletion.ExerciseID`,
`ManualExerciseSetLog.ExerciseID` and `ClimbingTick.ExerciseID` all reference `Exercise`,
never `LibraryExercise`. History is already independent. Verified twice.

**Materialisation has to move.** `MaterialiseTemplateExercises` (`db/queries.go:1126`) runs
lazily, only when the log is first edited (`http/server/training_log.go:429`). Until then a
run points straight at the session template's own exercises. Once templates are shared, an
import could change a run in progress. Move the copy to run creation.

## The live leak, still open

`https://passion.awalvie.me/signup` works for anyone who finds it, and signup imports every
configured directory. Production holds the private tree. So a new account gets Paradigm,
Power Company and Kettle content today. Step 2 closes it.

## Showing where a row came from — designed, not built

`scratchpad` note is gone with the session; the decisions are here.

Nothing records which tree a row came from. `listYAMLFiles` (`db/yaml_import.go:376-409`)
flattens every configured directory before parsing, so the origin is lost.

**Two reviewed decisions, both against my first proposal:**

- Use `Origin string` (`"app" | "public" | "private"`), not a boolean. The axis is already
  three-valued and a fourth is plausible. A boolean needs a second boolean beside it, and
  two booleans can disagree.
- **Tag the YAML file, not the folder.** Working the origin out from the directory has a bad
  failure: if a private directory is *present but empty* — a blank config key, a failed
  checkout — the keep-set holds only published names, and `pruneCatalogOrphans` hard-deletes
  every private row. The whole licensed catalog, silently. A *missing* directory is safe
  (the walk errors and the import aborts); an empty one is not. Per-file tagging has one
  list and one keep-set, so there is nothing to guard.

  Cost: a new private file could be added without the tag. Add a test that every file in
  the private tree carries it.

**Known gap:** the ~200 existing rows cannot be labelled correctly. Nothing can work out
their origin after the fact. They start unlabelled and self-heal on the next import — except
rows with `CatalogEditedAt` set, which the importer skips, so those stay wrong until Reset.

### The UI, from the pixel review

Two facts, not four chips. Origin (built-in / Private / Custom — only one can be true) and
Modified (Edited or not). At most two icons ever. Custom and Edited can never both be true,
because the importer only stamps rows it created.

- **`pencil` is wrong for the Edited chip.** `docs/DESIGN.md:157` maps it to the edit
  action. Use `rotate-ccw`, matching the Reset button.
- **The mobile exercise library shows nothing at all** — no origin, no Edited chip, not even
  Source. This is a plain bug. `templates/exercise_library.html:72-92` is a two-line grid
  (`static/passion.css:3708-3719`). Add a third grid column for bare icons. Do **not** copy
  the other two cards' shape; the library's "no Source on mobile" was a measured width call
  and the template says so at line 70.
- **Give status its own slim column on desktop** (~2.25rem) and take the Edited chip out of
  the Source cell. Two chips in one fixed-width cell wrap and make the row taller than its
  neighbours.
- No chip for the ordinary built-in case. Absence already means default here.
- A fourth filter reading "All origins", with no "Built-in" option.
- **The words need a copy pass.** "Edited" shipped without one, which `CLAUDE.md` requires.
  "Custom" reads as "customized" sitting next to "Edited" when it should mean "you made
  this".

Two incidental finds: `docs/DESIGN.md` never lists `rotate-ccw` though three pages use it,
and the comment at `templates/exercise_library.html:94` gives column widths that no longer
match the `colgroup` below it.

## Everything else on the backlog

**Login work, needed before other people use it.** No password reset, no email
verification, no limit on login attempts, no way to list or manage accounts. One SQLite file
on one box, which decides hosting.

**Licensing and docs.** No `LICENSE` for the code, no separate content licence for
`catalog/`, no third-party notice. The notice must cover DOMPurify, which is bundled inside
`static/toastui-editor-all.min.js`, Google Fonts as a runtime dependency, and a provenance
decision on `static/drag-drop-touch.js`.

**A rule in `CLAUDE.md`** saying not to put content from paid programmes in the catalog.
Nothing written down says it. That is how all of this started.

**The 13 house-style exercise notes** the owner confirmed he did not write. They carry real
doses, so keep the name, dose and media and replace the prose.

**Filename as the row id.** The importer matches on display name, so renaming an exercise
reads as "delete the old, create a new". Using the filename fixes it — 215 files already
have unique basenames, so it costs nothing. Needed before renames are safe. Separate job.

**Licensed files are still in git history.** Removing them from the tree did not remove them
from the 269 commits before it. Zero forks, so `git filter-repo` would be clean. The owner
said this is fine for now.

**Small things.** Four agent-memory files are uncommitted. `templates.html:48` prints a
literal `template(s)` instead of computing the singular. Transferring the repo to a GitHub
organisation, so the logo can be the avatar, was explicitly tabled.

## Working notes for the next session

- **Port 3000 is his live dev server holding the real `passion.db`.** Never touch it, never
  `pkill`. Kill anything you start by PID from `ss -ltnp`. Use port 3001.
- `.air.toml` watches `cmd config http db pages static templates`. Editing those restarts
  his server, which re-imports against the real database. It does **not** watch `catalog`.
- The env var is `PASSION_DB_PATH`, not `PASSION_DB`. Verify isolation by comparing
  `stat -c %Y passion.db` before and after.
- The private repo is at `/home/awalvie/code/lamp/passion-private-catalog`.
- **A name defined in both trees is a hard error at boot.** So a change that moves a name
  between the repos must be pushed to the private repo first, or his server will not start.
- Production is `ubuntu@140.245.198.72`, key `~/.ssh/stone`, database
  `/opt/passion/passion.db`. Inspect read-only.
- Never commit or push without being asked. He reviews first.
- Write to him in simple English.
