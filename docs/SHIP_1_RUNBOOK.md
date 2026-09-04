# Ship 1 — one shared catalog, and how to switch it on

The code is written, tested, and rehearsed against a copy of production. This is the
sequence to run it for real. Nothing here changes production until step 3.

## What changes

Before: every account held a private copy of the whole catalog. Signup imported it, and
the boot import re-ran for every account on every restart. That is how one instance reached
122,180 exercise rows for 8 accounts, and how content licensed to one person reached
everyone else.

After: one catalog, owned by one account and flagged shared. Every other account reads it
and nobody edits it in place. Saving your own version copies that one row to you.

```
shared = true   →  the catalog. Everyone reads it, nobody edits it.
shared = false  →  yours. Only you see it.
```

## Rehearsed numbers

Taken from a read-only copy of production on 4 September 2026.

| | |
|---|---|
| Rows given a slug | 226 |
| Rows published | **225** — the hidden open-session template is held back |
| Rows a new account owns | **0** |
| Library exercises a new account sees | 184 |
| Sessions a new account sees | 17 |
| Blocks a new account sees | 24 |
| A session opens with | 3 blocks, 19 exercises |
| Editing a published row | refused |
| Owner's runs, completions, ticks | 16 / 124 / 48, unchanged |
| Every other table count | unchanged |

Four foreign key violations exist in production already and are **not** caused by this: one
scheduled session pointing at a deleted template, three exercises pointing at deleted
activities. They are the `deleteTemplate` orphaning from the June audit. Worth fixing, not
part of this.

---

## The runbook

### 1. Back up

```sh
sudo systemctl stop passion
cd /opt/passion
STAMP=$(date +%Y%m%d-%H%M%S)
sqlite3 passion.db ".backup 'passion.db.bak-$STAMP'"
sqlite3 passion.db.bak-$STAMP "PRAGMA integrity_check;"
```

Expect `ok`. Copy it off the box as well — a backup on the same disk does not survive
losing the disk.

### 2. Deploy

Push both repos. **The private catalog must go first**: the code refuses to start on an
entry with no slug, so its slugs have to be on the server before the code that requires
them.

```sh
git -C ~/code/lamp/passion-private-catalog push origin master
git -C ~/code/lamp/passion push origin master
```

The service is stopped, so the deploy's restart is what brings it back. The boot import
will run once, for the catalog owner, matching on slug.

### 3. Fill in the slugs, then publish

```sh
cd /opt/passion
sudo systemctl stop passion

./passion -config passion.yaml --backfill-slugs
./passion -config passion.yaml --publish-catalog-dry-run
./passion -config passion.yaml --publish-catalog=4

sudo systemctl start passion
```

Expect 226 rows slugged and 225 published. **Read the dry run before the real one** — it
lists what it would publish and what it leaves private.

Order matters. Publishing before the backfill would work, but the importer matches on slug,
and a switch against an empty column makes every row match nothing — prune then treats the
whole catalog as orphaned.

### 4. Check

```sh
sqlite3 passion.db "
  SELECT 'shared library rows: ' || COUNT(*) FROM library_exercises WHERE shared=1;
  SELECT 'shared sessions: '     || COUNT(*) FROM session_templates WHERE shared=1;
  SELECT 'system session shared (must be 0): ' || COUNT(*)
    FROM session_templates WHERE shared=1 AND is_system=1;
  SELECT 'your runs: '  || COUNT(*) FROM session_runs;
  SELECT 'your ticks: ' || COUNT(*) FROM climbing_ticks;
  PRAGMA foreign_key_check;"
curl -s -o /dev/null -w "site: %{http_code}\n" https://passion.awalvie.me/
```

Then open the app and check a session still opens with its exercises. And mint an invite
code, sign up a second account, and confirm it sees the catalog and owns nothing:

```sh
./passion -config passion.yaml --mint-invites=1 --invite-note="shared catalog test"
```

### If it goes wrong

Publishing changes nothing else about a row, so it reverses exactly:

```sh
./passion -config passion.yaml --unpublish-catalog=4
```

Or restore the backup:

```sh
sudo systemctl stop passion
cp passion.db.bak-<stamp> passion.db
sudo systemctl start passion
```

---

## What is left after this

**The UI.** Nothing calls `save-as-mine` yet, so the copy feature works but has no button.
A catalog row also has no visual marker, and its Edit button still reads Edit even though
the write is refused. That wants a design pass — see `docs/DESIGN.md`, and ask pixel before
inventing a chip or a badge.

**`CatalogEditedAt` is now redundant.** Nobody edits a catalog row in place, so there is
nothing to stamp. `catalog_edited.go` and its 20 call sites can go, along with the Edited
chip and Reset to catalog. That is a deletion, not a feature, and it is the cleanest thing
left on the pile.

**A unique index on `(owner_id, slug)`.** Deliberately deferred to its own deploy: adding it
in the same one as the backfill risks `AutoMigrate` failing at boot, and a failed
`AutoMigrate` exits the process before anything loads.

**The four foreign key violations** noted above.
