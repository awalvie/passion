# V2 build plan

Written 8 September 2026, revised after review. The backend is rewritten from scratch on
branch `v2`. Master keeps working and is never touched.

Companion documents: `SCHEMA_V2.sql` (the schema), `STRUCTURE_PROPOSAL.md` and
`STRUCTURE_REVIEW.md` (the package layout), `V2_PLAN_REVIEW.md` (the review this revision
answers — its §3.1 holds the full YAML format and its §7 the full route list).

## Two decisions that shape everything below

**Build the new packages alongside the old ones. Delete the old ones last.** `store/` and
`web/` do not collide with `db/` and `http/server/`, so both exist and the app keeps
compiling on `v2` throughout. `main.go` switches over in one commit near the end.

**A runnable binary exists from phase 1, and it lives at the repo root as `main.go`.**
Not under `cmd/` — `//go:embed` patterns cannot contain `..`, so a binary two directories
down cannot embed `templates`, `static` or `catalog`. The root has no `.go` file today, so
the slot is free, and that is where the final `main.go` belongs anyway. Phase 12 deletes
`cmd/passion/`, not this.

---

# Part 1 — decisions that must be made before any code

These were missing from the first draft. Four of them land in migration 001, and SQLite
cannot add a CHECK, a UNIQUE or a primary key to an existing table, so they cannot wait.

## 1.1 Migration 001 additions to `SCHEMA_V2.sql`

| # | Change | Why |
|---|---|---|
| 1 | Replace `content.d_rest_seconds` with `d_rep_rest_seconds`, `d_set_rest_seconds`, `d_prep_seconds` | A movement's defaults must mirror `content_item` exactly. The trees carry `set_rest_seconds` 43×, `prep_seconds` 35×, `rep_rest_seconds` 17× at movement level, and `run.html` reads all three |
| 2 | Add `t_prep_seconds`, `t_rep_rest_seconds`, `t_set_rest_seconds` to `log_entry` | The player needs timings and the log freezes none of them. The alternative — read timings live from content — means editing a template changes a running session's timers. Freeze them |
| 3 | Add `content.retired_on VARCHAR(10)` | A shipped slug has left the tree twice already (`e527218`, `3a0a6cd`). The importer marks it retired; the library hides it; plans and logs keep resolving |
| 4 | Add `account.time_zone VARCHAR(64) NOT NULL DEFAULT 'UTC'` | "Today" is currently the server's day. Every heatmap, streak and calendar depends on it |
| 5 | Add `account.token_epoch INTEGER NOT NULL DEFAULT 0` | See 1.2. Without it, changing a password does not end old sessions |
| 6 | Add `account.max_pull_ups INTEGER`, `account.max_hang_kg NUMERIC(5,2)` | Both are on the profile form today and have no column anywhere |
| 7 | Add index `content (author_id, kind, name)` | The library list. One line |
| 8 | Add index `log (account_id, state)` | History filtering. One line |
| 9 | ~~Make `forked_from_id` composite **with `ON DELETE RESTRICT`**~~ **REVERSED 2026-09-10, see 1.11.** The column is gone with `content_key`, and this row goes with it. The finding it recorded still holds and is worth keeping for the columns that DO use a composite key: **never `ON DELETE SET NULL` on one** — it nulls *every* column in the key, including `kind NOT NULL`, which made any referenced row permanently undeletable. Verified on both engines | The one place it still bites is `plan_target`, which keeps a composite FK to `content (id, kind)` and uses `NO ACTION DEFERRABLE` |
| 10 | `content_item_set` and `log_set` primary keys gain `rep_index`: `(item, set_index, rep_index)` | **A ladder's rungs are reps inside one set, not sets.** `(item, set)` cannot hold 3s/6s/9s. An ordinary set is rep_index 0. SQLite cannot alter a primary key later, so this is irreversible and it was missing from this list |
| 11 | Every composite FK into `content` that sits in an account's cascade path becomes `ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED` — `content_item.child`, `plan_slot`, `plan_target`, `scheduled` | **With `RESTRICT`, deleting your own account is refused on Postgres and succeeds on SQLite.** Identical DDL, opposite behaviour, verified. Deferring the check to commit makes both agree |
| 12 | ~~`content.content_key CHAR(36) NOT NULL`, and `log_entry` carries `movement_key`~~ **REVERSED 2026-09-10, see 1.11.** `content_key` and `forked_from_id` are dropped from `content`. `log_entry` carries `movement_id BIGINT NULL` + `movement_slug VARCHAR(64) NOT NULL` + `movement_name`, which is what `log` already did for the session one level up. `ix_entry_progression` becomes `ix_entry_movement (movement_slug, log_id)` | The key could not survive an export and a restore, which is the one thing history most needs it for. Its only other job was repairing a forced rename that no longer happens. **Reversed 2026-09-10** |
| 13 | `content.source_tree VARCHAR(64)` | 1.10 item 5. 1.10 says this can wait for 2a, and it did not: migration 001 was being regenerated for row 12 anyway, so a later migration would have cost more than one line now. **Done 2026-09-08** (`383259a`) |
| 14 | New table `content_set`, holding a movement's OWN per-rep numbers — the mirror of `content_item_set` one level up | A movement can then BE a ladder. "Hangboard Ladder: Half Crimp" is a 3s/6s/9s hang and its name, slug and notes all say so; without this the shape could only be written inside one block. **Researched first**: seven other apps keep their movement library plain and put non-uniform sets in the workout, which they can because they also have a library of named protocols. Here the equivalent needs a block inside a block, and `ck_item_pair` forbids that on purpose. **Table count 19 → 20. Done 2026-09-08** |
| 15 | `content.per_side BOOLEAN NOT NULL DEFAULT FALSE` | Movements that say "per side" or "per leg" in prose only, so the app undercounts them. `bulgarian_split_squats` is 1 set of 6 with "per side" in its notes, and the player counts 6 when you owe 12. 14 public movements carry it after conversion. **Done 2026-09-08** |
| 16 | Drop `log_entry.account_id`, `log_entry.on_date`, the composite `fk_entry_log`, and `ux_log_identity` on `log`. A plain FK on `log_id` replaces them | 1.11. Two frozen copies existed only so the progression query needed no join, kept in step by an `ON UPDATE CASCADE` composite key. One account holds a few hundred logs. Two columns that can disagree are worse than one that cannot. **Done 2026-09-10** |
| 17 | New table `movement_pref` — a person's own numbers for a movement they do not own | 1.13. It is what makes editing an app-shipped movement's weight need no copy at all: no new row, no new identity, and the app's improvements still reach you. **Table count 20 → 21. Done 2026-09-10** |
| 18 | Rename `content.movement_kind` to `movement_style`, and `log_entry.movement_kind` to `movement_style` | Three columns were called `movement_kind` and they meant two different things one join apart: a performance style on `content`, a content kind on `plan_target`. `movement_pref` would have made three. Free while nothing has shipped. **Done 2026-09-10** |
| 19 | `ck_content_pick` becomes "a menu must have a count, and nothing else may have one"; new `ck_content_style`, `ck_content_slug` (no colon), `ck_content_retired` and `ck_log_on_date` (length 10); `ck_item_position`; explicit `NOT NULL` on the two `CHAR(36)` primary keys | Every one is a CHECK or a NOT NULL, which SQLite cannot add later. A menu with no count has no runtime meaning. A slug holding a colon would make `app:thing` ambiguous. A short date sorts wrong for good, and every history sort compares these as text. SQLite allows NULL in a `CHAR(36) PRIMARY KEY`. **Done 2026-09-10** |
| 20 | `content.uuid CHAR(36) NOT NULL` with a unique index, and `content.family CHAR(36) NOT NULL` with a plain one. `log_entry` gains `movement_family CHAR(36) NOT NULL`, `movement_slug` widens to 128 and stops being the grouping key, and `ix_entry_movement` becomes `(movement_family, log_id)` | 1.11, rewritten 2026-09-13. The id lives in the YAML file, which is the one thing `content_key` could not do and the reason it was dropped. History groups on the family, so a rename writes nothing to the log, and two unrelated movements that share a slug no longer merge into one chart. `movement_slug VARCHAR(64)` was also narrower than `content.slug VARCHAR(128)` |
| 21 | **Review pass, 2026-09-13.** `ux_content_uuid` splits into two partial indexes scoped like the slug pair. Every `CHAR(36)` becomes `VARCHAR(36)` — the three new columns and the four log primary keys. New length CHECKs: `ck_content_uuid`, `ck_content_family`, `ck_entry_family`, `ck_log_session_family`, and the six date columns that had none (`body_measurement`, `grade_milestone`, `plan`, `scheduled`, `calendar_event` ×2). `NOT NULL` on `log_set.id` and `log_climb.id`. New `log.session_family VARCHAR(36) NOT NULL DEFAULT ''` with partial index `ix_log_family` | Two independent reviews. A global unique index on `uuid` means two accounts cannot both add the same published catalog, forever. `CHAR` is `bpchar` on PostgreSQL and blank-padded, so one Go `""` stores as two different values. `NOT NULL` accepts `''` on both engines, and a shared empty family charts every affected row as one movement. SQLite accepts NULL in a `CHAR(36)` primary key, tested. Sessions still grouped by name, so a person's Power and the app's Power counted as one — the movement fault, one level up. **Every one of these is a CHECK or a NOT NULL, which SQLite cannot add later** |

## 1.2 Auth — stateless JWT, plus one integer

Keep the HS256 JWT in the `passion_auth` cookie. **No session table.** The schema has none,
a per-request read would contend with every write under `SetMaxOpenConns(1)`, and logout
already works by clearing the cookie.

- `account.token_epoch` is a JWT claim, compared on every request. Changing a password bumps
  it, which invalidates every existing cookie. This is the one thing stateless JWT cannot do
  and one integer fixes it. The default TTL is 30 days, so it matters.
- Cookie contract: `HttpOnly`, `SameSite=Lax`, `Secure` unless `InsecureCookies`, `Path=/`,
  `MaxAge` = TTL, HS256 only with an explicit `alg` check.
- CSRF stays an Origin / `Sec-Fetch-Site` check on unsafe methods. No CSRF token, so nothing
  needs server-side storage.
- Keep the existing secret validation — the placeholder list and 32-character minimum.

## 1.3 Config

| Today | V2 |
|---|---|
| `Server.Addr`, `Auth.*`, `InsecureCookies` | keep, including the existing secret validation |
| `Server.DBPath` | dies → `Database.Engine` + `Database.DSN` |
| `Server.DemoOwnerID` | dies — no owner sentinel exists |
| `YAMLImport.Enabled` | → `Catalog.Import`, default **true**. The import is idempotent and ownerless now |
| `YAMLImport.*Dir` (three keys) | die. One directory per kind was a consequence of the old three-table split |
| `YAMLImport.OwnerID` | **replaced, not dropped** → `Catalog.Private[].Owner`, an email. See 1.10 |

New: `Database.Engine` (`sqlite` \| `postgres`, anything else refuses to boot),
`Database.DSN`, `Database.MaxOpenConns` (forced to 1 on SQLite),
`Migrate` (`up` \| `off`), `Catalog.Private` (a list of `{dir, owner}` — **this is how the
private catalog loads**, since a second repository cannot be embedded; see 1.10), `Log.Level`,
`Log.Format`, `Defaults.TimeZone`.

`Catalog.Private` is a list of structs, so it has no `PASSION_*` env override. Env overrides
stay for scalars only; a private tree is declared in the file.

`DevAuthBypass` refuses to boot when `Engine = postgres`.

## 1.4 When migrations run

- **At boot, by default.** One binary, one unit, nothing for a self-hoster to forget.
- `Migrate: off` skips. `--migrate-status` prints current and target and exits.
  `--migrate-only` applies and exits.
- **Refuse to boot when the database is newer than the embedded migrations.** That is a
  rolled-back binary meeting a migrated database. The message names both versions.
- **Never automatically down. Migrations are append-only after the first boot.**
- Two directories, one numbering: `store/migrations/sqlite/` and `.../postgres/`. goose's
  dialect is process-wide and a `.sql` file has no per-dialect branch, but `goose.Up` takes a
  directory. A test asserts both directories hold the same version numbers.

## 1.5 What `main.go` does, in order

1. Parse flags: `-config`, `-migrate-status`, `-migrate-only`, `-import-catalog`, `-seed`,
   `-version`. **16 of today's 17 flags die** — every `-purge-*`, `-backfill-*`,
   `-publish-catalog`, `-delete-users-except`, `-mint-invites` and the rest exist to repair
   data that is being discarded. `make reseed` calls `--exit-after-seed` and needs updating.
2. Load config, apply env overrides, validate. Exit non-zero naming the offending setting.
3. Set up `slog` from `Log.Level` / `Log.Format`.
4. `//go:embed templates static catalog` — here, because embed patterns cannot contain `..`.
5. `store.Open(ctx, cfg.Database)`.
6. Migrate per 1.4, or print status and exit.
7. Import the catalog when `Catalog.Import`, **shipped tree first, then each private tree**:
   `ImportShipped(ctx, embedded)`, then `ImportOwned(ctx, tree.Dir, tree.Owner)` per entry.
   The order is load-bearing, not cosmetic — 39 of the private tree's refs resolve to public
   slugs, so the shipped rows must exist first. A private tree whose owner email matches no
   account logs a warning naming both and is skipped; it imports on a later boot. See 1.10.
8. Compute the static asset checksum for cache-busting.
9. `web.New(...)` — parse templates once, build the router.
10. Warn once per enabled footgun: `InsecureCookies`, `DevAuthBypass`.
11. Start `http.Server` with the existing 15s / 60s / 120s timeouts.
12. `signal.Notify` on SIGINT/SIGTERM, `Shutdown` with a 10s context, close the store.

**There are no background jobs.** `scheduled` rows are written when a cycle is created and
read on demand. Nobody should build a job runner.

## 1.6 Errors and logging

1. No `store/` signature names an HTTP type, and no error it returns carries a status code or
   user-facing prose. Store returns `gorm.ErrRecordNotFound`, `gorm.ErrDuplicatedKey` (via
   `TranslateError: true`) and its own sentinels: `store.ErrNotYours`, `store.ErrShipped`.
2. **One function maps error to status**, in `web/render.go`. Not found → 404, duplicate →
   409, not yours → **404, never 403** (a 403 confirms the row exists), else 500.
3. Every error is handled exactly once. A function logs it or returns it, never both.
4. A request-id middleware puts a short id in the context, the log line and the 500 page.
5. One log line per request: method, path, status, duration_ms, account_id, request_id.
6. `GET /healthz` returns 200 and the schema version. GORM's logger goes to `slog` and warns
   on any query over 200 ms.

## 1.7 The run's menu choice

A menu has `pick_count` and nothing records which option was chosen. Three of the four
shipped public sessions contain a menu, so this blocks phase 3.

**The design:** a menu freezes at run start as one `log_entry` with
`movement_style = 'menu'`, `movement_slug` = the menu's derived slug, `state = 'pending'`. When the
runner chooses, that row is deleted and one row per chosen movement is inserted at the same
`position`, with targets resolved **at choose time**.

Two consequences to write down: `position` is not unique on `log_entry`, so every read is
`ORDER BY position, id`; and the schema's "frozen at run creation" sentence gains this
exception.

## 1.8 Features removed on purpose

Removal is a feature change. These are deliberate, not oversights.

| Feature | V2 |
|---|---|
| Invite-only signup | **Dropped, and 1.10 is the reason.** The gate existed because the catalog held paid-programme content; under 1.10 it does not — a new account sees the 102 public slugs and nothing else. The invite table and both flags go. `signup.html` carries an invite field and the line "Passion is invite-only for now" — **the template must be edited in phase 1**, contradicting the first draft's "reuse as-is" |
| `save-as-mine` on all three kinds | **Kept, and it is the whole story now.** Revised 2026-09-10: 1.13 replaced fork-on-edit with an explicit "Copy to my content" button, which is what these three routes already were |
| `reset-catalog` | **Kept**, as "delete your copy". Needs no provenance column: per 1.11 a copy keeps its name, so it and the row it came from are found by `(kind, slug)` across the two unique indexes |
| Per-use warmup/cooldown labelling | **Lost by design.** `block_kind` is on the block, not the use |
| Cycle per-week targets | **Kept.** See phase 6b |

## 1.9 Slug and email normalization

Slugs are lowercase ASCII, `[a-z0-9_]`, normalized in Go before every read and write. Same
for email. Never left to the database collation — MySQL's default is case-insensitive and
Postgres' is not, and we may want MySQL one day.

## 1.10 Two kinds of catalog tree

**Decided 2026-09-08.** This replaces "the private catalog is just a second tree that also
imports as shipped content", which is what 1.3 and 1.5 said before.

A private tree imports as **one account's own content**, not as content the app ships.

| | Shipped tree | Private tree |
|---|---|---|
| Where | embedded `catalog/` | on disk, named in config |
| `author_id` | `NULL` | the account named by `owner:` |
| Who reads it | every account | that account, only |
| Unique index it lands in | `(kind, slug) WHERE author_id IS NULL` | `(author_id, kind, slug) WHERE author_id IS NOT NULL` |
| Editing a row | the app offers a copy | **edits in place and detaches the row.** See 1.13 |
| Re-import over an existing slug | refreshes it. The tree on disk is the truth | **refreshes it,** unless the row was edited in the app, in which case it is skipped and reported |

```yaml
catalog:
  import: true
  private:
    - dir: ../passion-private-catalog
      owner: awalvie@example.com
```

`owner:` is an **email, not an id.** The dead `YAMLImport.OwnerID` named an id; the account it
named was deleted, and the import wrote its rows under an id nothing pointed at. An email
cannot go stale silently — the importer resolves it at boot and says so when it cannot.

### The measurements this rests on

Counted 2026-09-08 across both trees:

| | |
|---|---|
| Public slugs / private slugs | 102 / 123 |
| Slugs present in **both** trees | **0** |
| Distinct refs the private tree makes | 127 |
| ...resolving inside the private tree | 88 |
| ...**resolving to a public slug** | **39** — 33 under `exercises/`, 6 under `activity_templates/` |
| ...dangling | 0 |
| Refs the public tree makes into the private tree | 0 |

### What follows from it

| # | Implication | Lands in |
|---|---|---|
| 1 | **A `content_item` edge routinely crosses the author boundary** — an owned parent, a shipped child. Those 39 refs are exactly that. The schema already permits it (nothing on `content_item` compares authors) and it is what you want: my session points at the shipped `weighted_pull_ups`, not a private copy of it. The reverse can never occur, because the shipped import only ever writes ownerless rows | schema note + 2b gate |
| 2 | ~~**The ref resolver reads two scopes, mine first, then shipped.**~~ **Reversed 2026-09-10 — see 1.12.** A first-then-fallback resolver means adding a row can silently change what an existing reference means. A reference now says which namespace it wants, and there is no fallback at all | 1.12, 2a importer |
| 3 | **Import order is now correctness, not tidiness.** Shipped first. The old wording — extra trees "merged *before* the embedded catalog" — is backwards; it is also in `config/app.go`'s doc comment on `Catalog.Dirs`. Both to fix | 1.5, 2a |
| 4 | **A first boot cannot import a private tree.** No account exists, so the owner email resolves to nothing. Warn naming the email and the dir, keep booting, import on the next boot. Refusing to boot would deadlock — you cannot sign up before the listener opens | 1.5 step 7 |
| 5 | ~~**A file-sourced row is read-only in the app.**~~ **Reversed again 2026-09-10 — see 1.13.** It is editable, the edit happens in place, and it clears `source_tree`, which detaches the row from its file for good. The import then SKIPS that row and names it in the report. `content.source_tree` is still the mechanism and still the row's own answer to "may the importer refresh me" | 1.13, 2a |
| 6 | **Deleting the account erases its private catalog.** The cascade follows `content.author_id`; that is the design working, not a leak. Re-importable from disk next boot. Not worth engineering around — worth knowing before clicking | 1 gate, 2b |
| 7 | ~~**"You may not edit this" becomes `author_id IS NULL OR source_tree IS NOT NULL`.**~~ **Reversed 2026-09-10 — see 1.13.** It goes back to `author_id IS NULL`: everything you own is editable. `Content.Editable()` is that one line. `FromAFile()` stays, but it now tells a caller to warn that editing detaches the row, rather than to refuse | 1.13, 2b |
| 8 | **A stranger can take the private catalog, and this is accepted.** Signup is open and `owner:` is resolved against `account.email` at every boot. While no account holds that email — a first boot, or after the owner deletes their account — whoever signs up with it first becomes the import target and receives the tree. **You chose to keep this on 2026-09-08.** It only bites where other people can sign up; on a single-user machine the window shuts at your own signup and never reopens. Mitigation is one log line, not a design change: log the resolved binding at boot with both the email and the account id, so the moment it happens is visible | 1.5 step 7, risks |
| 9 | **Every content read carries `WHERE author_id IS NULL OR author_id = ?`.** Index `content (author_id, kind, name)` will not serve both arms of that `OR` well. At a couple of hundred rows per account it does not matter; written down so it is not met later as a mystery | 2b, schema review |

Mechanical changes this implies: `Catalog.Dirs []string` becomes `Catalog.Private
[]PrivateTree{Name, Dir, Owner}` — **`Name` as well as `Dir`, because `source_tree` stores it
and a filesystem path is not stable across machines**; `ImportCatalog` splits into
`ImportShipped` and `ImportOwned`; `main.go`'s `catalogTrees()` returns the typed list.

**`content.source_tree VARCHAR(64) NULL`.** NULL means a person made this row in the app, or
it is a copy — either way the importer never touches it. Non-NULL is the tree name that owns
it (`shipped`, or a `Catalog.Private[].Name`), and a re-import rewrites exactly the rows whose
`source_tree` matches the tree being imported. One nullable column, not a boolean plus a name,
because two columns can disagree and one cannot. It is a plain `ADD COLUMN` with no constraint,
so it does **not** have to be in migration 001 — it can land with the importer in 2a.

## 1.11 Identity: what a finished run points at

**Rewritten 2026-09-13**, after reading wger's source. This section has now been decided three
times. The record of all three is at the end, because the same mistake caused the first two.

### The three columns

| Column | Job |
|---|---|
| `content.id BIGINT` | The local key. Every foreign key in this schema points here. It may differ between two installs of the app, and it never leaves the machine |
| `content.uuid CHAR(36)` | The global key. It is written in the YAML file. The importer matches a file to a row on it. It is never a foreign key target |
| `content.family CHAR(36)` | Which family of rows this one belongs to. Read by history and by nothing else. Flat, and never followed |

`log_entry` mirrors the last one. `movement_family` is what a progression chart groups on, and
`ix_entry_movement (movement_family, log_id)` answers it.

### The case this exists for

The app ships a movement, Pull Up. A person logs it 40 times. Then they edit it, which gives
them a copy: a new row with their account on it.

| id | uuid | family | slug | owner |
|---|---|---|---|---|
| 12 | `3f2a…` | `3f2a…` | `pullup` | the app |
| 987 | `c81d…` | `3f2a…` | `pullup` | the person |

The 40 old entries carry `movement_family = 3f2a…`. Every entry after the edit carries the same
value, because the copy inherited it. One chart, 41 points.

A new row's family is its own uuid. A copy's family is the family of the row it came from. That
is the whole rule, and nothing ever resolves a family value back to a row.

### Why the id has to be in the file

This is what killed `content_key`, the 2026-09-08 design. That key was minted by the database.
Export a tree, restore it into a fresh database, and every key is new, so every old `log_entry`
points at nothing. History breaks exactly when the export feature is used.

A file-borne id has no such failure. The file carries it, the export writes it, the import reads
it back.

The same fault is on record in other projects. Roam's exporter left the id out, so its round trip
was silently lossy. Strapi and Contentful both mint a new id at import, so neither survives its
own export. Hugo derives its "unique id" from the file path, so moving a file makes it a
different thing.

### Why history groups on the family and not on the slug

The 2026-09-10 design grouped on `log_entry.movement_slug`. It worked, and it had two faults.

**Two unrelated movements can share a slug.** A private tree can hold `pullup` and so can the
app's. Both are yours to run. Grouping on the name merges them into one chart, and the version of
this section being replaced recorded that as a consequence to accept.

**A rename had to rewrite the log.** `aliases:` in a file renamed a row and then rewrote
`log_entry.movement_slug` for that row's entries. That is a destructive write against history,
made only to keep a chart whole. A family never changes, so there is nothing to rewrite.

### A copy never keeps the original's uuid

Every comparable project that allowed that lost a row to it. Anki, Logseq, Grafana, Pulumi and
Argo CD all document the same failure: two things sharing one id read as one thing in two states,
and the next sync deletes one of them.

wger is the closest working example. It is a self-hosted fitness app where exercises ship from
upstream, sync by uuid, and users log history against them. It runs the same split — uuid to
match across installs, integer pk for every join, and the uuid is never a foreign key target. Its
`variation_group` is our `family`. It reached that shape by deleting a `Variation` table in
migration 0037, because a local group row cannot survive a sync.

### What we do not take from wger

`DeletionLog.replaced_by`. Upstream says "this exercise is gone, use that one", and the importer
moves the user's log rows onto the replacement. It needs a guaranteed import order and a fallback
for a missing target. wger gets both wrong, and its own test asserts the result: the user's logs
are deleted when no replacement is present locally. We drop a shipped movement about once a year,
`retired_on` already keeps its history working, and the family already merges the old chart with
the new one.

### What did NOT change

`plan_target`, `plan_slot` and `scheduled` keep their `BIGINT` ids. They are current state, not
history. `log.session_slug` and `log_entry.block_name` are untouched.

`log_entry.movement_slug` and `movement_name` stay. They are the frozen record of what the thing
was called that day, which is what lets the whole of history render with every content table
dropped. Neither is read to group anything.

**Two columns left `log_entry`** on 2026-09-10 and have not come back. It froze `account_id` and
`on_date` from its parent, kept in step by a composite FK with `ON UPDATE CASCADE`, purely so the
progression query needed no join. One account holds a few hundred logs, so the join costs
nothing.

### The three decisions, in order

| Date | Decision | Why it fell |
|---|---|---|
| 2026-09-08 | `content.content_key CHAR(36)`, minted by the database, inherited by a copy | It could not survive an export and a restore into a fresh database |
| 2026-09-10 | No key at all. History groups on `log_entry.movement_slug`, and a copy keeps its slug | Two unrelated movements sharing a slug merge into one chart, and a rename has to rewrite the log |
| 2026-09-13 | `uuid` in the file, `family` inherited by a copy, history groups on the family | — |

The same mistake was behind the first two: the id was minted by whatever read the file, instead
of being written in the file. Once it is in the file, the export carries it and a rename costs
nothing.

## 1.12 References: two namespaces, no fallback

**Decided 2026-09-10.** This is the whole answer to a naming argument that ran for most of a day
and produced three proposals that were dropped.

**A bare reference means the tree the file is in. `app:` means the catalog the app ships. There
is no fallback between them.**

```yaml
items:
  - block: warm_up            # this tree
  - block: app:drills         # the app's
```

**The bug this removes.** The importer resolved a reference by trying the account's own rows and
then the app's. So a private row silently shadowed a shipped one of the same name, and adding a
row could change what an existing reference meant, with no message. Hugo shipped that and removed
it in 0.45 for that reason; dbt refuses an ambiguous reference and requires a qualifier to cross a
package boundary.

**Every reference also names the kind it expects** — `movement:`, `block:`, `menu:` — instead of a
bare `ref:`. A block's child may be a movement or a menu, so the kind cannot be worked out from
position; both unique indexes on `content` already include the kind; and the loader can check
`ck_item_pair` before touching the database.

**A name is unique only within its kind.** A block `drills` and a menu `drills` can coexist. This
is free: the database already worked that way, and only the loader was stricter.

**Config gains one validation: two private trees may not name the same owner.** With two, "bare
means my tree" has no single answer, and the silent-shadowing bug comes back in through the
config file.

### Three proposals that were dropped

| Proposal | Why it went |
|---|---|
| Auto-qualify a menu's name with its parent's | Measured: it bought exactly one name across 22 menus. And a menu can have two parents as far as the schema is concerned, so "its parent" is not always one thing. Superseded entirely by menus losing their files (1.14) |
| Make the same name in both trees a hard error | It forbids a person from ever using a name the app shipped, and that gets worse with every release. The problem was never the name; it was the reference |
| A generated id in the file, or a folder path in a reference | Rejected on readability. Hand-editing YAML is the authoring surface, permanently |

## 1.13 Editing, and a person's own numbers

**Decided 2026-09-10.** This replaces copy-on-edit, which 1.10 item 5 set out and which the
schema called FORKING.

Three cases, three answers. None of them makes a hidden copy.

| What you edit | What happens |
|---|---|
| A row you own that came from a file | Edited **in place**: same id, same name, `source_tree` set to NULL. The NULL detaches the row from its file for good |
| Only the numbers on a movement the app ships | `movement_pref`, a per-account override. No new row, no new identity |
| The structure of something the app ships | An explicit **"Copy to my content"** button. The copy **keeps the name** |

**A detached row is reported, twice.** The import result names every file it skipped, and the
library badges the row. Without that, a person edits the YAML, restarts, sees no change, and is
told nothing — and hand-editing those files is this project's main way of working, so that
failure is certain rather than hypothetical.

**Why in-place editing is right.** R1 is that an edit must not change a row *for anybody else*. A
row with `author_id = you` is shared with nobody, so copy-on-write protected no one. All the edit
had to do was survive the next re-import, which is `source_tree`, which already existed.

**Why a copy keeps its name.** Your names and the app's live in different unique indexes and never
meet, so there is no clash to work around. Keeping the name is also what keeps your history in one
series — see 1.11.

**One consequence to put in the copy dialog:** your YAML files still say `app:<name>`, so a session
you run from a file keeps using the app's row. There is no fallback by design, so nothing silently
switches to the copy.

### `movement_pref`

New table, migration 001. One row per account per movement, every number nullable, NULL meaning
"no opinion". Keyed by `movement_id`, because the importer updates a matched row rather than
replacing it, so an id survives every re-import and every rename.

Resolution order for a number, weakest first:

1. the movement's own `d_*` default
2. `movement_pref` for the account running the session
3. the `content_item` override on the slot being run
4. what the person enters while running

**The slot beats the person's own number**, because a slot is a prescription: a session that ramps
60/70/80% must not collapse into one figure. That only stays fair while a shipped file does not put
an absolute weight on a slot, so **the importer refuses one**. Measured: no shipped file does today,
and making it a rule is what stops that being a fact about today rather than a guarantee.

Overrides are movement-only. Blocks and sessions carry no numbers, and a menu's `pick_count` is
deliberately not overridable — changing a shipped session's structure is what the copy button is
for.

## 1.14 A menu has no file

**Decided 2026-09-10**, after the owner confirmed a menu is written per session and never reused.

A menu is written inside the block that holds it, under `menu:`. It is still a `content` row with
kind `menu`, so `ck_item_pair`, the tree walk and the exporter are unchanged. Its slug is
`<block>_<position>`, never typed and never shown.

**What this bought.** The old format's 25 name collisions were mostly menus wanting a file whose
name their parent block already had. `drills` the block holding `drills_menu` the menu was the
shape of it. Twenty-two files and twenty-two names stop existing.

**A menu is upserted by its derived slug, not rebuilt.** A hard delete and recreate on every start
would be simpler and would break the rule that a second import of an unchanged tree writes nothing
at all. A menu absent from the file is DELETED rather than retired, because nothing can reference
one and a retired one would keep its derived slug and block the next import of a shortened block.
---

## 1.15 Linting a catalog tree

**Decided 2026-09-13.** One command, in the app's own binary:

```
passion catalog lint ~/my-catalog            # reports, writes nothing, exits 1 on a problem
passion catalog lint --fix ~/my-catalog      # writes missing ids into the files, then reports
passion catalog lint ./catalog ~/my-catalog  # two trees, so app: references are checked too
```

It reads a directory. No database, no server, no config.

| Problem | What it prints |
|---|---|
| A file with no `id:` | `movements/front_lever.yaml: no id` |
| Two files with one id | `movements/pullup_weighted.yaml: id 3f2a… is also in movements/pullup.yaml` |
| A reference to nothing in this tree | `blocks/warmup.yaml: movement "scapular_pulls" is not in this tree` |
| A tag that is not a slug | `sessions/power.yaml: tag "Explosive Power" is not a slug` |
| A `family:` naming nothing in the trees given | `movements/front_lever.yaml: family 3f2a… matches no id in this tree` |
| Everything `Load` already refuses | An unknown key, a menu with no pick count, a slug holding a colon |

Run against one tree, an `app:` reference is accepted without being checked, because the app's
tree is not there to check it against. Pass both trees and it is checked.

This lands on code that exists. `Load(fsys, name, known map[Ref]bool)` in `store/catalog.go`
already takes the set of references another tree provides, which is the two-tree case.

**Where it runs.**

1. Not on your laptop. Booting the app writes missing ids into the files itself, and the import
   already refuses everything else in the table. The command is for checking without booting.
2. `deploy.yml`, before Build, over `catalog` and `catalog-private`. A bad file stops the deploy
   instead of stopping the server after the binary is already copied.
3. `passion-private-catalog`, its own new workflow, over one tree. That repo is where the file is
   written, so that is where the mistake is made.

**Boot refuses when it cannot fix.** If a file has no id and the directory cannot be written, the
app does not start, and the error names the file. In practice this never fires, because you boot
locally before you commit. It exists so that a server can never mint an id that disagrees with
the one in the file.

Two different messages, because two different things are wrong. A writable private tree that
somehow could not be fixed names `passion catalog lint --fix`. The shipped tree is embedded with
`//go:embed`, so it can never be written at runtime and `--fix` is not an answer for it — a
missing id there is a build fault, caught by item 2 above, and the message says so.

### What the review pass settled, and what is still open

**Settled: sessions group on a family too.** `log.session_family` was added on 2026-09-13. A
person's session called Power and the app's session called Power are two workouts, and "times
completed" counted them as one. It had to be now: adding a `NOT NULL` column to a table that
already holds rows is refused by SQLite, and backfilling it means the log reading `content`,
which the log must never do. Empty means the run had no session behind it, which is what an open
session is.

**Settled: sync is phone to this server and back, never device to device.** So a log row may
point at content by the local integer `movement_id`. Recorded in the log section of
`SCHEMA_V2.sql`. If that ever changes, `log_entry` needs the content uuid beside the family, and
that is another migration-001-or-never column.

**Settled: `aliases:` is gone, and `rewriteHistory` with it.** Its two jobs were to let the
importer recognise a renamed file and to rewrite `log_entry.movement_slug` afterwards. The uuid
does the first, and the family removes the need for the second. No import writes to finished
history any more.

**Open, and safe: a menu has no file, so no id in one.** Per 1.14 a menu is written inline and
its slug is derived from its block and its position. Its uuid is minted at insert and preserved
by the upsert. Nothing logs a menu — `log_entry` records the movement that was run — so history
never reads it. One thing to write down where the upsert lives: the derived slug counts over all
items, so inserting a movement at the top of a block renumbers every menu below it, and those
menus are deleted and recreated with fresh uuids. Harmless while nothing points at a menu, and
the reason a menu's uuid can never be treated as stable.

## 1.16 Tags are whatever you write

**Decided 2026-09-13.** This reverses the closed vocabulary that stood since 2026-09-08.

A file writes `tags: [fingers, strength]`. A tag nothing has used before is created on import.
There is no list, and `catalog/tags.yaml` is deleted.

**Why the list went.** It existed to catch one thing: a typo minting `shouldres`. It was
shipped inside the binary, so somebody self-hosting Passion with their own catalog could not
tag their own movement `yoga` without editing our source and rebuilding. A gate on the user's
own files, held in a file the user cannot reach, for a feature whose whole point is bringing
your own content.

**What still holds.** A tag must look like a slug — lower case, digits and underscores — so
`warm_up` and `Warm Up` cannot become two tags for one idea. `tag.name` is derived from the
slug on creation (`hip_mobility` becomes "Hip Mobility") and is never overwritten afterwards,
so renaming a tag in the app sticks.

**What was given up.** A typo is now a tag used once rather than a failed import. The library
showing a usage count is what surfaces it, and that is phase 5a.

The 2026-09-08 clean-up is not undone: the converter still merges `shoulders` into `shoulder`,
`stretching` into `mobility`, `lead` into `route` and `prehab` into `antagonist`, and still
drops `squat`, `hinge`, `pressing` and `rest`. The shipped tree carries 21 tags. Nothing
enforces that number now.

# Part 2 — the catalog YAML format

**The spec of record is [CATALOG_FORMAT.md](CATALOG_FORMAT.md)**, which lists every key per
kind. This part states the decisions and why they were made.

**Revised 2026-09-10, and the revision is large.** The format went to `format_version: 2` and
three of Part 2's original decisions were reversed. What is written below is the current state,
with each reversal named. Version 1 never touched real data.

```
catalog/
  catalog.yaml               format_version, and nothing else yet
  movements/<slug>.yaml
  blocks/<slug>.yaml
  sessions/<slug>.yaml
```

## The three reversals

**1. There is no `slug:` key.** Reversed 2026-09-10. It was required and had to equal the
filename, with the importer failing when they differed — 46 of 225 files disagreed and would
have become 46 renames.

The key is gone. The directory gives the kind, the filename gives the name, and a row's
identity is where its file sits. Two files cannot claim one identity because two files cannot
share a path, so the check the key existed to enable has nothing left to check. It also deletes
one line from every file and one thing to keep in step.

Renaming is now a plain `git mv`. The importer matches the file by its `id:`, so the row is found
whatever the file is called, and history does not move because it groups on the family — 1.11.

**2. A menu has no file.** Reversed 2026-09-10, see 1.14. Every content row was to be a file,
which made the conversion invent 22 block names, 24 menu names and 9 block names by hand and
took the file count from 225 to 274.

A menu is written inside its block. The conversion now writes **106 files** and invents **3**
names. The three are `movements/journal.yaml`, plus `blocks/bouldering.yaml` and
`blocks/hangs.yaml`, which were groups with no name at all inside a session file.

The `options:` versus `items:` distinction goes with it: a menu's list is `of:`, sitting under
`menu:`, so the "choose from these" reading is carried by the key above it rather than by a
third list word.

**3. A reference names its kind, and says which namespace it wants.** Reversed 2026-09-10, see
1.12. A bare `ref:` named a target without naming its kind, which only worked while a name meant
one thing across all four kinds.

```yaml
items:
  - movement: app:half_crimp_hang
    sets: 4
  - block: warm_up
  - menu:
      pick: 1
      of: [app:cuban_press, shoulder_ys]
```

## What did not change

**Rules.** `tags` is a list of slugs, created on first use — see 1.16. Every number is
optional; absent is NULL, and `0` is a real value. List order is `position`, renumbered
densely by the importer.

**All nine unnamed blocks get named. None are deleted.** A review suggested letting a session
hold a movement directly, which would delete two of them. **Rejected, and the schema is why.**
`ck_item_pair` permits session→block, block→movement|menu and menu→movement, and nothing else.
Loosening it would make a session's child list heterogeneous, so every renderer of a session —
run screen, preview, log — would have to handle "this child is a movement, not a block". The
Emil wrapper holds six movements, so it wants a name whatever the schema allows; the boulder
wrapper holds one, and "Bouldering" is a fine name for it.

Six of the nine turned out to be menus, which under 1.14 need no name at all. Three remain, and
the converter carries them.

**`per_set` replaces the `rungs` string**, and a ladder's entries are reps inside ONE set. So
`sets` is required alongside it and says how many times the ladder is repeated, and `reps` is
refused because `per_set` already gives each rep.

**Renames from the old format.** `label:` → `tags:`, a comma string becoming a list, in all 225
files, and the one the importer hard-fails on. `kind:` → `style:` in the movement files, which
is 1.1 row 18 — an earlier draft said `movement_kind:` and that name is now taken by a different
meaning on `plan_target`. Then `style: session` — on 56 of 184 entries — becomes **`open`**,
because otherwise the string `session` means two different things in two columns. Media keys
`video_url` → `url` and `thumbnail_url` → `thumb_url`, across the 112 files that carry media.

**Not `duration`.** Of those 56, exactly **one** carries `session_duration_seconds`. The other
55 carry no dose key at all, as do all 57 `climbing` movements. The style does not mean "one
long stretch of time" — it means "no numbers, just do the thing". `duration` would send a reader
hunting for a number that is not there and tempt the importer author into requiring one.

**`per_side: true`.** The highest-value gap a review found, and a live bug rather than a format
nicety. Movements say "per side" or "per leg" in prose only, and the numbers are already
ambiguous in the data: `bulgarian_split_squats` is `sets: 1, reps: 6` with "per side" in the
notes, so the player counts 6 when you owe 12. One boolean fixes them and lets the run timer say
left and right. The converter detects it from the notes and reports every one for checking: 14
in the public tree, after two false positives were ruled out by hand and one pattern bug was
fixed. That bug is worth recording — the pattern had no word boundaries, so "up**per arm**s"
matched "per arm".

**`pick` means the fewest options you must choose, not the most**, and **`pick: 0` means "you
may skip this"**. That is where optionality lives. Four menus put it in the display name
instead — "Wall Crawls (optional)", "Strength (optional)", "Prehab (optional)", "Easy campusing
(optional)" — so 0 says it properly and the suffix comes off. Otherwise the one fact the app
could act on stays buried in a name. The database now requires a count on a menu and forbids one
on anything else, because "pick NULL of these" has no runtime meaning. Ranges need no key today;
add `pick_max` when a file asks for one.

**The complete key table per kind is published before any file is converted.** A review found
this fatal rather than untidy: the importer hard-fails on an unknown key, and six keys that
existed in the data appeared in no draft of this spec — `source:` (124 files), `media:` (112),
`color:` (17), `needs:` (5), `rung_seconds:` (3), `session_duration_seconds:` (1). Taken
literally, every session file would have refused to import on `color:`.

The private tree is a separate repository with its own git and no shared CI, so adding one key to
the format breaks that tree on the next boot until it is converted too. Convert both in the same
pass.

**The quoting rule was overstated, and is corrected in CATALOG_FORMAT.md.** An earlier draft here
said every string must stay quoted because `90_90` parses as the integer 9090. Measured against
`gopkg.in/yaml.v3`: into a typed `string` field, `90_90_hold`, `no`, `007` and `3_strike_repeat`
all arrive as those exact strings. Only an untyped decode turns `007` into `7`, and nothing does
an untyped decode. **Only `#` is genuinely load-bearing**, because unquoted it starts a comment —
so every `color` must be quoted. Quote the rest by convention.

**`format_version` in one `catalog.yaml` at each tree root, not in every file.** It is one
constant per tree, and copying it into every file creates that many chances to disagree while a
`git mv` does not touch it.

**Two gaps recorded and deliberately not fixed in V2**, so they are decisions rather than
oversights. **Intensity**: 23 movements carry it only as prose ("~RPE 8", "well below your
limit"), which for finger work is the safety-relevant number — without a field a cycle can
progress volume and never intensity. **Edge and grip**: 20 names bake the geometry in
(`max_hangs_20_mm_edge`, six `isometric_hang_*_{open,half_crimp}`, three `hangboard_ladder_*`),
so every new edge needs a new file and the file count grows on the wrong axis. Both want a field
on the movement, overridable on an item. Neither is a V2 feature; both are written here so the
next person knows the limit was seen.

**No `source:` is added to `catalog/exercises/bechtel/`.** A review caught this against our own
proposal, and it is a licence matter, not a style one. Those five files carry no `source:` today
and are generic barbell and bodyweight lifts in our own words. The folder is named after a paid
ebook, so adding `source: "Logical Progression"` would **newly assert paid-programme provenance
in the published tree** for content that does not need it. Delete the coach folder and let a
deadlift be a deadlift. The other 24 of the 29 public exercises missing a source (`ondra/` 15,
`emil/` 6, `nelson/` 3) are publicly known free content and CLAUDE.md permits naming them.

---

# Part 3 — testing

| | |
|---|---|
| Store tests | Real database. SQLite in `t.TempDir()`; the same suite against Postgres in a throwaway container. No mocks, ever. |
| Handler tests | `httptest` against a real store, **on both engines**. Not SQLite only: deleting an account behaved differently per engine and a SQLite-only gate would never have seen it |
| Both engines | Every store test runs against both. **The Postgres suite fails, not skips, when `PASSION_TEST_POSTGRES` is set and unreachable**, and prints a skip count when unset. Otherwise "runs on both" quietly becomes "runs on SQLite" |
| The pragma test | Insert a child row with a bad parent and assert it is refused. An unrecognised SQLite DSN parameter is **accepted silently**, so this is the only way to know `_foreign_keys=on` took |
| Ported behaviour suite | The named list lives in `store/BEHAVIOUR.md`. The first draft said "39 tests", which matches nothing measurable |

---

# Part 4 — the phases

19, not 13. Four of the original thirteen were two phases wearing one row, and phase 4's
question could not be answered where it sat.

Sizing is relative. Handler LOC is what that phase replaces, measured non-test.

### Phase 0 — foundations · **L**

Every irreversible decision lives here. Not a warm-up.

`go.mod`: add `goose` **and `gorm.io/driver/postgres`** — nothing in `go.mod` can reach
Postgres today. Decide **cgo or pure-Go SQLite** here: the DSN parameter names differ between
drivers and a wrong name is accepted silently.

Three more decisions that reach every file written after them, and were missing:

- **`gorm.Config{NamingStrategy: schema.NamingStrategy{SingularTable: true}}`.** GORM
  pluralizes table names by default, so `Content` becomes `contents` and stops matching the
  migrations.
- **`TranslateError: true`**, so `gorm.ErrDuplicatedKey` is returned rather than a driver
  string. §1.6 depends on it.
- **Every nullable column is a pointer** — `*int`, `*float64`, `*string`. A NULL number means
  "not asked" and `0` is a real value (0 kg is bodyweight), so a non-pointer int cannot tell
  them apart.

Then `store/models.go` (19 structs), `store/migrations/{sqlite,postgres}/001_init.sql` with
**all nine of 1.1's changes**, `store/store.go` with per-engine DSN and `WithTx`, and the
ported behaviour suite.

Store helpers inside the package are **package-level functions taking `tx *gorm.DB`, not
methods.** A function with no receiver cannot reach `s.db`, so GORM's same-handle transaction
trap becomes unwritable rather than discouraged. Every exported method takes `ctx` first.

**Gate.** `go test ./store/... -count=1` passes with `PASSION_TEST_POSTGRES` set and unset.
`grep -rn AutoMigrate store/` returns nothing, as a test. `goose status` reports the same
version on both engines, and a test asserts both migration directories hold the same version
numbers. Boot twice against a temp database; the row counts across all content tables are
identical. The bad-parent insert is refused.

### Phase 1 — accounts and the web skeleton · **L**

`store/account.go`. `web/server.go`, `middleware.go`, `render.go`. `web/auth.go` — signup,
login, logout, password change. **`main.go` at the repo root**, carrying 1.5's startup order.
Edit `signup.html` to remove the invite field and its copy.

**Gate.** An `httptest` test signs up two accounts, writes a row in every account-owned table
for each, deletes one, and asserts zero rows for it across all nine tables that reference an
account, and unchanged counts for the other. Changing A's password rejects A's old cookie and
leaves B's working. Browser check is a manual smoke, named as such.

### Phase 2a — the catalog format, importer, exporter, public tree · **XL**

The format spec, the loader, the importer, **the exporter** (moved here from
phase 11 — it is the format's second implementation and the only thing that proves a round
trip), and the public tree converted.

**Gate, revised 2026-09-10.** Numbers changed because a menu stopped being a file (1.14) and
because inline movements became files.

- `count(*) FROM content WHERE author_id IS NULL` = **108** — 89 movements, 12 blocks, 4
  sessions, 3 menus. The movement count is 89 rather than 88 because `journal`, written inside
  a session file, is now a file of its own.
- Import twice; **every table's checksum unchanged.** This is what proves a menu is matched by
  its derived slug rather than deleted and rebuilt, which is the obvious implementation and the
  wrong one.
- Import a tree with an unknown tag, one with a `menus/` directory, one with `format_version: 1`,
  and one with a key the spec does not list. All four fail loudly, naming the file.
- Export the tree, load the export, import it into an empty database: the catalog means exactly
  the same thing. Compared as meaning — slugs, names, numbers, tags and edges named by the slugs
  at each end — not as rows, because a fresh database issues new row ids.
- Export twice, byte-identical, or a tree kept in git churns on every run.
- Per 1.12: a bare reference does **not** resolve to the app's catalog, at the loader and at the
  importer separately. The loader can be handed the wrong index; the importer is the one that
  touches rows.
- Per 1.10 item 4: a private tree naming an unknown owner email is skipped with a warning and
  the boot continues; the same tree imports once the account exists.
- Per 1.13: a re-import **refreshes** a row whose `source_tree` matches the importing tree, and
  **skips and reports** a row whose `source_tree` is NULL — and a skipped row keeps its own
  children, which one earlier implementation got wrong because a single map served both
  reference resolution and the decision about what to write.
- Per 1.11: renaming a file moves the row it owns, found by `id:`, and creates no second row.
  History is untouched, because it groups on `movement_family` and a rename does not change one.
  A personal name and a shipped name that are the same stay two separate charts.
- Per 1.13: a shipped file setting `weight_kg` on a slot is refused; the same file in a private
  tree is accepted.

**Status: reopened and redone 2026-09-13** for the identity design in 1.11 and 1.15.

What changed in the code: the importer matches a file to its row by `uuid` rather than by
`(kind, slug, author)`; `aliases:` and `rewriteHistory` are deleted, so no import writes to
finished history; a slug held by two things is refused by name instead of by a constraint;
the exporter writes `id:` back out, and `family:` only on a copy; `store.MintIDs` and
`store.MissingIDs` write and report a missing id; `passion catalog lint [--fix]` checks a
tree with no database and no server; `main.go` imports the shipped tree and then each
private tree, minting ids into a writable one and refusing when it cannot; `config.Catalog`
takes `private: [{dir, owner}]` in place of `dirs`.

Verified on the real converted tree: 108 rows in, a second import writes nothing, the export
round-trips, and every identity survives an export into an empty database. Whole suite green
on SQLite and Postgres 17.

**Both trees are written.** `catalog/` now holds 107 files — 89 movements, 12 blocks, 4
sessions and `catalog.yaml` — and `passion-private-catalog` holds 145 in the
same shape, plus a `block_names.yaml` the converter reads and a workflow that runs
`passion catalog lint` on push. Measured on a boot with both: 108 shipped rows, 164 owned
rows, and 64 edges from owned content into the app's catalog, which is the cross-author edge
1.10 item 1 predicted. A second import of either tree writes nothing.

The seven groups the old private tree left unnamed are named in
`passion-private-catalog/block_names.yaml` rather than in `cmd/convertcatalog`, because they
describe a licensed programme's session structure and that is not ours to publish here. The
converter takes them with `-names`.

**This branch breaks V1.** `catalog/exercises`, `catalog/session_templates` and
`catalog/activity_templates` are gone, and `cmd/passion` reads all three. Nothing to fix —
V1 dies at phase 12 — but master must not take this branch before then.

### Phase 2b — the private tree, content reads, the library · **XL**

The private tree imported **as one account's own content** per 1.10, `store/content.go` (the one
visibility function, tree read, copy), and the library pages.

**Gate, revised 2026-09-10.**

- The public count does not move when the private tree imports. That is what proves the private
  tree did not leak into shipped.
- A second account sees the public tree and none of the private one.
- `CopyToMine` leaves the original row byte-identical, copies its blocks and edges, **keeps the
  slug**, and creates **zero** new movement rows. Per 1.11 there is no `forked_from_id` to set.
- `slug='silent_feet'` is one row while its `content_item` count is greater than one.
- Per 1.10 items 1 and 6: an owned session holds edges to shipped children, and deleting the
  owner removes the session and its edges while every shipped child survives.
- Per 1.11: a progression query spanning a copy returns one series rather than two, and two
  unrelated movements with different names do not merge.
- Per 1.13: editing a row with `source_tree` set **succeeds**, sets `source_tree` to NULL, keeps
  the row's id and slug, and the next import names that file in its skipped list. The library
  badges the row as detached from its file.
- Per 1.13: `movement_pref` resolves in the documented order, and a pref on a row the person does
  not own shows up inside a session the app ships.
- **A regression test asserts the content picker never returns another account's private
  content** — `content_item` has no author-comparing constraint, so nothing in the database will
  catch that if the picker regresses. The exporter refuses such an edge rather than writing it,
  which is the second half of the same guard.

### Phase 3 — run and log · **XL**

`store/log.go` — start a run (freeze in one transaction), record sets, finish. The menu flow
from 1.7. **The `log_climb` store writes** (moved here — `boulder_session` is shipped and 57
of 184 movements are climbing). The five-source resolver. `web/run.go` and a history list.

**Freeze `CurrentStep`'s field list as a contract before touching `run.html`.** It holds 1,021
lines of timer state machine whose inputs change shape, and a mistake there does not throw —
the timer counts the wrong number of seconds, mid-session, on a phone. One test per timer
phase (prep, work, rep rest, set rest) asserting the seconds.

**Gate.** Run a session, log sets against a target, see the disparity. **Drop every content
table; history still renders.** Rename a session and a gym; the old record is unchanged.
Insert `plan_target` rows directly and assert each of the five sources wins in turn. Run a
session containing a menu, and one containing a climbing movement.

### Phase 4 — structure checkpoint · **S**

Narrowed to what phases 0–3 can actually answer. `find store web -name '*.go' | xargs wc -l`
per file, against the 1,200-line trigger.
`grep -nE 'func \(s \*Store\).*\) \(.*(Page|View|Dashboard|Summary)' store/*.go` returns
nothing. The page-shaped-read question moves to 7c, because only the dashboard tests it.

### Phase 5a — the movement library editor · **L**
### Phase 5b — blocks and sessions · **XL**

Together they replace **2,263** handler lines. `fragments/exercises_container.html` alone is
1,452 lines.

**Gate.** Reorder a block's items and assert `content_item_set` rows survive — the sibling
rewrite is **`UPDATE`-only, never delete-and-insert**, because the cascade would take the
planned sets. Copying a shipped block copies the block and not its movements. `sets` equals
`count(content_item_set)` after every save, add, delete and clear.

### Phase 6a — cycles and scheduling · **L**
### Phase 6b — the Exercise targets page · **L**

Together **1,670** handler lines, including the largest single file in the repo.

**Gate.** Set a whole-cycle target, then a week-3 target touching only `reps`; week 3 shows
the new reps and the cycle's sets. Two whole-cycle rows for one movement are refused. Creating
the same cycle twice does not double `scheduled` — the schema makes that the app's job, so it
needs an upsert and a test. The "varies by week" toggle inserts one all-NULL `plan_target` row
per week, which the unique key permits.

### Phase 7 — dashboard and calendar · **L**
### Phase 7c — second structure checkpoint · **S**

**Gate.** The dashboard renders with zero store calls returning a screen-shaped type.
`/dashboard`, `/calendar` and `/history` each answer under 200 ms against 500 logs and 5,000
log entries, with the query count per page asserted, not eyeballed.

### Phase 8 — full history · **L**

**Gate.** Heatmap, streak, pyramid and send rate each computed from a fixture of known logs,
with expected numbers in the test. A `draft` log appears in none of them.

### Phase 9a — manual log entry · **XL**
### Phase 9b — open sessions and trial runs · **L**

Together **2,170** lines — 19% of the handler code.

**Gate.** A manual entry backdated to the 1st shows the 1st on the log page *and* in the
exercise history. An open session with per-set planning saves and reloads. A trial run writes
no `scheduled` row.

### Phase 10 — ticks UI, places, profile · **L**

**Gate.** Rename a gym; every past log line shows the old name. Delete a gym; the name stays
and `place_id` is NULL. A tick with no grade is accepted and stays out of the pyramid.

### Phase 11 — markdown preview and remaining fragments · **S**
### Phase 12 — switch over · **M**

**Gate.** `grep -rn 'passion/db|passion/pages|http/server' --include='*.go' .` returns
nothing. `go build ./...` and the full suite pass. `cmd/passion/` is gone.

### Phase 13 — docs · **M**

**Gate.** A test reflects over `config.Config` and asserts every key in the readme table
exists. Every Make target runs.

---

# Part 5 — risks

| Risk | What I do |
|---|---|
| **`run.html`'s 1,021 lines of timer JavaScript.** A mistake does not throw; the timer just counts wrong, mid-session | Freeze the field contract in phase 3 before touching the template. One test per timer phase |
| **cgo vs pure-Go SQLite.** The two drivers spell DSN parameters differently and **an unknown parameter is accepted silently** — the wrong name gives a database with no foreign keys and a green suite | Decide in 0.1. Test it with a bad-parent insert |
| **The Postgres suite is aspirational.** No driver in `go.mod`, no container library, no compose file | Add the driver in 0.1. Make the suite fail, not skip, when the container is unreachable |
| **The private catalog is a second repository.** Two trees, one format, no shared CI | Convert both in the same week. `format_version` at each tree root, so a stale tree fails loudly on the next boot |
| **Two live databases while I train.** Master runs production; V2 runs alongside from phase 3 | Decide which is the real log during the build. If it is V2's, "old data is disposable" stops being true from phase 3, and the SQLite file needs a backup story |
| **Feature loss by omission.** 37 sub-actions hide behind 7 catch-all route patterns; a route dropped by accident looks identical to one dropped on purpose | Enumerate all 37 with a phase or the word "drop" beside each. `V2_PLAN_REVIEW.md` §7 is the start |
| **The catalog conversion is your content** | Public tree first. Show one file's diff and get agreement before touching the private repo |
| **The owned import has a dependency the shipped import never had** (1.10): an account must exist, and 39 refs need the shipped rows already in place | Fixed import order. Skip-with-warning on an unresolved owner. A gate that boots empty, signs up, boots again, and finds the tree |
| **A stranger can claim the private catalog** by signing up with the configured `owner:` email while no account holds it (1.10 item 8) | **Accepted, knowingly, 2026-09-08.** Not mitigated by design. Log the resolved binding at boot with email and account id. Revisit only if this instance ever takes signups from other people |
| **The tag vocabulary is five axes in one flat list** — body part, equipment, discipline, quality, session role, skill — with 10 tokens used once or twice, and hard validation would freeze `hinge` and `squat` as permanent vocabulary | **Closed 2026-09-13 by removing the vocabulary.** The conversion still applies the 2026-09-08 merges and drops, so the shipped tree carries 21 tags, but nothing enforces a list any more. See 1.16 |
| **The `web/` file list is a guess** | Expected to move at phase 4. Not a commitment |
| **Two rounds of redesign landed on the identity and reference scheme** (1.11 to 1.14), and the second round deleted most of the first. Design churn on the schema is expensive once anything has shipped | **All of it is inside migration 001, where a CHECK, a UNIQUE and a primary key are still free to change on SQLite.** That window closes at the first real boot on real data. Anything after this is a new migration and a data move |
| **A person hand-edits a file, restarts, and nothing happens** — because the row was edited in the app once and is detached from its file for good (1.13) | The import result names every skipped file, and the library badges the row. This is the failure mode the reporting exists for, and it is certain rather than hypothetical: hand-editing YAML is how this project is worked on |
| **A person's history for a movement merges with the app's**, when they hold a movement with the same name as one the app ships and run both (1.11) | **Accepted.** It is almost always the same exercise, copied down to be edited. The merge is a read-time grouping over frozen columns, so a chart can always split it apart again. Guard: warn once when a name that already has logged history is created |
| **A future release adds `weight_kg` to a slot in a shipped file**, silently overriding every account's own saved weight for that movement (1.13) | The importer refuses it for the shipped tree. Measured as zero occurrences today, and turned into a rule so it stays that way |

---

# Part 6 — what I will not do

- No commit or push without being asked. Not once.
- Nothing on master. Nothing on production.
- No `AutoMigrate`. Goose owns the schema, append-only, never automatically down.
- No new features. V2 is the same app on a better database.
- No mocks in place of a real database.
- No paid-programme content in `catalog/`.
- No background job runner.

# Where this starts

Phase 0, step 0.1 — `go.mod`, and the cgo-versus-pure-Go decision. Everything after it
depends on that choice, and a wrong DSN parameter name fails silently.
