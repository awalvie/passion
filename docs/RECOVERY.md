# Recovering from the 4 September 2026 catalog duplication

The Ship 1 deploy built a second complete catalog under an account that does not exist.
This is what happened, what is already fixed, and the order to fix the rest in.

Nothing in this document changes production before step 3, and every step says how it
reverses.

**Status: recovered on 7 September.** Steps 2 to 4 are done — the guards are deployed, the
1,172 ghost rows are gone, and every YAML slug matches an existing row. Step 5 re-enables
the import. Step 6, publishing the catalog, is the only thing left and is not recovery.

Two things worth recording, because both corrected an assumption made while diagnosing:

- **The slugs were already filled in.** `--backfill-slugs` reported "nothing to do", and
  the YAML-to-database comparison came back with 225 slugs on each side plus the seeded
  `open_session` template. The claim that user 4's rows were unslugged came from a copy of
  the database taken at 17:08 on 4 September, before the backfill was run. It was inference
  presented as fact, twice.
- **The purge only held because the guards went first.** A hand-run SQL cleanup on
  4 September was undone by the next restart, which re-imported into the deleted owner.

## What happened

The boot import used to loop over every account and ignore `YAMLImport.OwnerID`. Ship 1
changed it to import once, into that owner — which made a setting that had been dead for
months load-bearing overnight. The server's copy said `OwnerID: 1`. User 1 had been
deleted hours earlier the same day.

So the import wrote a full catalog — around 1,172 rows across six tables — under an id
with no account behind it. Every catalog read is owner-scoped, so those rows are
unreachable, and on the next import prune would have seen them as orphaned.

Two things made it possible, and only the second one is interesting:

1. The config was stale. That is ordinary.
2. **Nothing checked.** `PublishCatalog` had verified its owner existed since it was
   written. `ImportYAML` never did. That asymmetry is the whole incident.

The user's own data was never touched: 0 runs, 0 scheduled sessions and 0 completions
belonged to the ghost owner, and none of user 4's exercises referenced a ghost library row.

## What is already fixed in code

| | |
|---|---|
| `ImportYAML` refuses an owner id with no account | `db/yaml_import.go` |
| `ImportYAML` refuses to run against unslugged catalog rows | `db/yaml_import.go` |
| The block prune branch no longer hard-deletes exercises history points at | `db/yaml_import.go` |
| `--delete-users-except` refuses to delete the configured import owner | `db/delete_users.go` |
| The boot import is switched off in the deployed config | `passion.prod.yaml` |
| `--purge-ghost-catalog` removes rows owned by a non-existent account | `db/purge_ghost_catalog.go` |

The guard lives in `ImportYAML` rather than `main.go` because the importer has a second
caller: `resetCatalogRow` in `http/server/catalog_edited.go` runs a full import from an
HTTP request. A check in `main.go` would leave that path open.

It refuses rather than warns. A warning is what we already had — the setting was silently
ignored, which is exactly how it went stale.

### Why the import is switched off rather than pointed at the right account

Two traps, and the config sits between them:

- Leave `OwnerID: 1` and deploy the guard, and the service refuses to start. The systemd
  unit has `Restart=on-failure`, so it crash-loops. Data is safe, site is down.
- Change it to `OwnerID: 4` while the import is on, and if user 4's rows have no slugs the
  import matches nothing, creates a second catalog, and prune then treats his real rows as
  having left the YAML.

`Enabled: false` sidesteps both. It also closes the HTTP import path, because `main.go`
leaves `yamlImport` nil when the import is disabled and `resetCatalogRow` returns
`errCatalogImportDisabled`.

One thing to know before editing anything on the box: `.github/workflows/deploy.yml` copies
`passion.prod.yaml` over `/opt/passion/passion.yaml` on **every** deploy. A hand edit there
is reverted by the next push, silently. The config is fixed in git or not at all.

---

## 1. Establish the current state

Read-only. The service can stay up. Take the backup anyway — a deploy follows.

```sh
cd /opt/passion
STAMP=$(date +%Y%m%d-%H%M%S)
sqlite3 passion.db ".backup 'passion.db.bak-$STAMP'"
sqlite3 passion.db.bak-$STAMP "PRAGMA integrity_check;"
```

Expect `ok`, then copy it off the box.

**Who exists, and who owns catalog rows.** Grouped by owner rather than filtered on 1:
signup was open for a while and the count of accounts that ever existed is a fact to
establish, not assume. Live and soft-deleted are separated because a half-deleted parent
with live children is the shape of the old orphan bug.

```sh
sqlite3 -readonly passion.db "
SELECT 'live users: ' || group_concat(id) FROM users WHERE deleted_at IS NULL;
SELECT 'all users:  ' || group_concat(id) FROM users;
SELECT t, owner_id, live, soft FROM (
  SELECT 'library_exercises' t, owner_id, SUM(deleted_at IS NULL) live,
         SUM(deleted_at IS NOT NULL) soft FROM library_exercises GROUP BY owner_id
  UNION ALL SELECT 'activity_templates', owner_id, SUM(deleted_at IS NULL),
         SUM(deleted_at IS NOT NULL) FROM activity_templates GROUP BY owner_id
  UNION ALL SELECT 'session_templates', owner_id, SUM(deleted_at IS NULL),
         SUM(deleted_at IS NOT NULL) FROM session_templates GROUP BY owner_id
  UNION ALL SELECT 'activities', owner_id, SUM(deleted_at IS NULL),
         SUM(deleted_at IS NOT NULL) FROM activities GROUP BY owner_id
  UNION ALL SELECT 'exercises', owner_id, SUM(deleted_at IS NULL),
         SUM(deleted_at IS NOT NULL) FROM exercises GROUP BY owner_id
  UNION ALL SELECT 'exercise_media', owner_id, SUM(deleted_at IS NULL),
         SUM(deleted_at IS NOT NULL) FROM exercise_media GROUP BY owner_id
) ORDER BY owner_id, t;"
```

`exercise_media` is in that list deliberately — it is not one of the three catalog models,
so anything that only walks those undercounts.

**The two answers that decide whether a restart is safe.** These matter more than the ghost
count:

```sh
sqlite3 -readonly passion.db "
SELECT 'unslugged lib:   ' || COUNT(*) FROM library_exercises
  WHERE owner_id=4 AND deleted_at IS NULL AND managed_by_catalog=1 AND (slug IS NULL OR slug='');
SELECT 'unslugged block: ' || COUNT(*) FROM activity_templates
  WHERE owner_id=4 AND deleted_at IS NULL AND managed_by_catalog=1 AND (slug IS NULL OR slug='');
SELECT 'unslugged sess:  ' || COUNT(*) FROM session_templates
  WHERE owner_id=4 AND deleted_at IS NULL AND managed_by_catalog=1 AND (slug IS NULL OR slug='');
SELECT 'shared rows: ' ||
  ((SELECT COUNT(*) FROM library_exercises WHERE shared=1)
  +(SELECT COUNT(*) FROM activity_templates WHERE shared=1)
  +(SELECT COUNT(*) FROM session_templates WHERE shared=1));"
```

**Is anything of user 4's pointing at a ghost row:**

```sh
sqlite3 -readonly passion.db "
SELECT 'ex→ghost lib: ' || COUNT(*) FROM exercises e
  JOIN library_exercises l ON l.id = e.library_exercise_id
  WHERE e.owner_id = 4 AND l.owner_id <> 4;
SELECT 'cyc ov→ghost lib: ' || COUNT(*) FROM cycle_exercise_overrides o
  JOIN library_exercises l ON l.id = o.library_exercise_id
  WHERE o.owner_id = 4 AND l.owner_id <> 4;
SELECT 'cyc wk ov→ghost lib: ' || COUNT(*) FROM cycle_exercise_week_overrides o
  JOIN library_exercises l ON l.id = o.library_exercise_id
  WHERE o.owner_id = 4 AND l.owner_id <> 4;
SELECT 'lib parent→ghost: ' || COUNT(*) FROM library_exercises c
  JOIN library_exercises p ON p.id = c.parent_library_exercise_id
  WHERE c.owner_id = 4 AND p.owner_id <> 4;
SELECT 'sched→ghost tpl: ' || COUNT(*) FROM scheduled_sessions s
  JOIN session_templates t ON t.id = s.session_template_id
  WHERE s.owner_id = 4 AND t.owner_id <> 4;
SELECT 'cyc map→ghost tpl: ' || COUNT(*) FROM training_cycle_weekday_mappings m
  JOIN session_templates t ON t.id = m.session_template_id
  WHERE m.owner_id = 4 AND t.owner_id <> 4;"
```

**Does run history reference a ghost exercise** — the six tables that carry `exercise_id`
with no `REFERENCES` clause, so nothing in the database would stop a delete stranding them:

```sh
sqlite3 -readonly passion.db "
SELECT 'completions→ghost ex: ' || COUNT(*) FROM run_exercise_completions c
  JOIN exercises e ON e.id = c.exercise_id WHERE e.owner_id <> 4;
SELECT 'ticks→ghost ex: ' || COUNT(*) FROM climbing_ticks t
  JOIN exercises e ON e.id = t.exercise_id WHERE e.owner_id <> 4;
SELECT 'manual logs→ghost ex: ' || COUNT(*) FROM manual_exercise_set_logs l
  JOIN exercises e ON e.id = l.exercise_id WHERE e.owner_id <> 4;
SELECT 'planned sets→ghost ex: ' || COUNT(*) FROM exercise_planned_sets p
  JOIN exercises e ON e.id = p.exercise_id WHERE e.owner_id <> 4;
SELECT 'choices→ghost ex: ' || COUNT(*) FROM run_exercise_choices ch
  JOIN exercises e ON e.id IN (ch.parent_exercise_id, ch.chosen_exercise_id)
  WHERE e.owner_id <> 4;
SELECT 'climb meta→ghost ex: ' || COUNT(*) FROM climbing_exercise_meta m
  JOIN exercises e ON e.id = m.exercise_id WHERE e.owner_id <> 4;"
```

**Foreign keys:**

```sh
sqlite3 -readonly passion.db "PRAGMA foreign_key_check;"
```

Four rows are the known baseline and predate all of this — see
[SHIP_1_RUNBOOK.md](SHIP_1_RUNBOOK.md). A fifth row, or a different shape, is new. Do not
fix any of them here.

Reverses: nothing to reverse.

## 2. Deploy the guards

This branch. The import is off in the shipped config, so the deploy's restart is a no-op
for the catalog and the site comes back on the same data.

```sh
git -C ~/code/lamp/passion push origin <branch>
```

Verify:

```sh
journalctl -u passion -n 50 --no-pager | grep -i "yaml import"
curl -s -o /dev/null -w "site: %{http_code}\n" https://passion.awalvie.me/
```

Expect no "yaml import enabled" line at all, and a 200.

Reverses: revert the commit and push. For an immediate rollback without a deploy, restore
the backup from step 1.

## 3. Remove the ghost rows

**Step 2 must be deployed first, and this is a hard dependency, not a preference.** Until
it is, the server still runs the old binary with `Enabled: true` and `OwnerID: 1`, so
**every restart rebuilds the ghost catalog**. Purging before the deploy is wasted work: the
rows come back the next time the service bounces, including the bounce the deploy itself
performs. Confirmed on 7 September — a hand-run SQL cleanup on 4 September was undone
exactly this way, and owner 1 still holds a full catalog.

`--purge-ghost-catalog=ID`. Dry run by default, following `--delete-users-except`: a dry
run you have to remember to ask for is a dry run somebody skips.

```sh
cd /opt/passion
sudo systemctl stop passion
STAMP=$(date +%Y%m%d-%H%M%S)
sqlite3 passion.db ".backup 'passion.db.bak-$STAMP'"

# The ghost slugs are the only record of what the importer wrote there. Cheap insurance.
sqlite3 -readonly -csv passion.db "
  SELECT 'library_exercises', id, owner_id, slug, name FROM library_exercises WHERE owner_id=1
  UNION ALL SELECT 'activity_templates', id, owner_id, slug, name FROM activity_templates WHERE owner_id=1
  UNION ALL SELECT 'session_templates', id, owner_id, slug, name FROM session_templates WHERE owner_id=1;
" > ghost-slugs-$STAMP.csv

./passion -config passion.yaml --purge-ghost-catalog=1                      # dry run
./passion -config passion.yaml --purge-ghost-catalog=1 --i-have-a-backup

sudo systemctl start passion
```

It refuses, rather than narrowing, on any of:

- `ID == 0` — the "no owner" sentinel across the data layer.
- An account with that id exists, checked `Unscoped()` so a soft-deleted one counts.
  This is the command's entire safety property.
- Any of its rows is flagged `shared` — that is the app's catalog, whatever the account
  state.
- Run history references any of its exercises. It reuses the same
  `referencedByHistoryWhere` predicate as `--purge-orphans`, not a copy.
- Anything not owned by that id points at one of its rows — seven joins, including the
  `exercises.library_exercise_id` shape production was checked for by hand.

Hard deletes, not soft. Soft-deleting a parent does not cascade, so it would leave the
children live and unreachable — creating the orphan bug rather than clearing it. A
soft-deleted row also keeps occupying its `(owner_id, slug)` identity.

### Rehearsed against real data

Run on a local copy of production with user 1's row deleted and its rows left in place:

| | |
|---|---|
| Refused while user 1 still existed | yes, naming the account |
| Refused once its rows carried run history | yes — 4 exercises |

That second refusal is the one to understand. **If the real run refuses on history, stop.**
It means the id was a working account whose runs were logged, not a catalog written under a
dead id — and this is the wrong tool for it. Production's owner 1 should have no history at
all, because `--delete-users-except` hard-deleted everything it owned before the deploy
recreated the catalog rows. Step 1's history query is what confirms that; if it comes back
non-zero, bring the numbers back rather than reaching for `--i-have-a-backup`.

Verify by re-running the step 1 queries: the ghost owner's groups gone, every owner-4
count byte-identical, and `foreign_key_check` still exactly the four known rows.

Reverses: `cp passion.db.bak-<stamp> passion.db`. There is no finer undo, and the command
says so before it runs.

## 4. Fill in the slugs

With the service stopped. The importer now refuses to run against unslugged rows, so this
has to happen before step 5 either way.

```sh
./passion -config passion.yaml --backfill-slugs-dry-run
./passion -config passion.yaml --backfill-slugs
```

Read the dry run, and read the collision list in particular. Then check the result against
the slugs in the YAML before trusting it — a backfilled slug that does not match its YAML
entry duplicates that row on the next import, which is this whole incident again, one row
at a time.

## 5. Turn the import back on

One commit: `OwnerID: 4` and `Enabled: true` in `passion.prod.yaml`, together. Either one
alone is one of the two traps described above.

## 6. Publish the catalog

The unfinished Ship 1 step, and not part of the recovery. Commands, expected numbers and
the rollback are in [SHIP_1_RUNBOOK.md](SHIP_1_RUNBOOK.md) step 3 — which was corrected at
the same time as this document, because its old ordering would now crash-loop the service.

---

## Deliberately not part of this

**The ownership redesign.** Making shared catalog rows ownerless — `owner_id = 0` or NULL —
was proposed while diagnosing this and rejected on evidence. Zero is already the "no owner"
sentinel in four places, and `GuardWritable` grants a zero-owner caller write access to a
row not yet flagged shared. NULL is worse: SQLite treats NULLs as distinct, so the planned
unique index on `(owner_id, slug)` would enforce nothing. A permanent system account to own
the catalog fails too — `--delete-users-except` takes victims from `WHERE id <> keep`, so it
would be an ordinary victim. `db/models.go` already carries a comment rejecting exactly
this: "a trap this project already hit once, in a production cleanup script." `Shared bool`
stays, and the real fix was the one missing check.

**A blocker worth recording for whoever adds the unique index on `(owner_id, slug)`.** Two
things fail today, both verified:

1. No UI path sets `Slug`, so every user-created row sits at the default `''`. Two such rows
   for one owner violate the index, and `''` is a value — SQLite's NULL-distinctness escape
   does not apply. `AutoMigrate` adding the index would fail at boot, which exits the
   process before anything loads.
2. The three upsert lookups in `db/yaml_import.go` are scoped, so a soft-deleted row is
   invisible to them and the insert beside it collides.

Either set slugs on the UI paths, make `Slug` a `*string`, or use a partial index
(`WHERE managed_by_catalog = 1` — SQLite supports it, but GORM's tag cannot express it, so
it needs a post-`AutoMigrate` `Exec`).

**Everything else already listed** in [SHIP_1_RUNBOOK.md](SHIP_1_RUNBOOK.md) under "What is
left after this": the save-as-mine UI, retiring `CatalogEditedAt`, the unique index itself,
and the four pre-existing foreign key violations. One correction to that list — the
save-as-mine *handlers* are wired (`templates.go`, `exercise_library.go`,
`activity_template_handlers.go` all have a `save-as-mine` case). What is missing is a button
and a way to tell a catalog row from your own.

**A media leak in the prune, pre-existing.** `deleteExercisesAndMedia` collects media with
a soft-delete-scoped `Find`, then hard-deletes the exercises unscoped — so media rows on a
retired exercise generation are orphaned rather than removed on every import. Small, old,
and not what tonight was about.

**Automated backups.** There are none. Every step above depends on one taken by hand.
