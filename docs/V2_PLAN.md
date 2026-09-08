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
| 9 | Make `forked_from_id` composite **with `ON DELETE RESTRICT`** | Pins the fork's kind for free. **Never `SET NULL`** — on a composite key that nulls *every* column including `kind NOT NULL`, which makes any forked-from row undeletable. Verified on both engines |
| 10 | `content_item_set` and `log_set` primary keys gain `rep_index`: `(item, set_index, rep_index)` | **A ladder's rungs are reps inside one set, not sets.** `(item, set)` cannot hold 3s/6s/9s. An ordinary set is rep_index 0. SQLite cannot alter a primary key later, so this is irreversible and it was missing from this list |
| 11 | Every composite FK into `content` that sits in an account's cascade path becomes `ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED` — `content_item.child`, `plan_slot`, `plan_target`, `scheduled` | **With `RESTRICT`, deleting your own account is refused on Postgres and succeeds on SQLite.** Identical DDL, opposite behaviour, verified. Deferring the check to commit makes both agree |
| 12 | `content.content_key CHAR(36) NOT NULL`, and `log_entry` carries `movement_key` in place of `movement_id` + `movement_slug`, with `ix_entry_progression` regrouped onto it and a new `ix_content_key` | 1.11. **In 001 while it is free** — restructuring a NOT NULL column and an index on SQLite after real rows exist is exactly what is expensive later. **Done 2026-09-08** (`383259a`) |
| 13 | `content.source_tree VARCHAR(64)` | 1.10 item 5. 1.10 says this can wait for 2a, and it did not: migration 001 was being regenerated for row 12 anyway, so a later migration would have cost more than one line now. **Done 2026-09-08** (`383259a`) |

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
`movement_kind = 'menu'`, `movement_key` = the menu's key, `state = 'pending'`. When the
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
| `save-as-mine` on all three kinds | **Dropped.** Fork-on-edit replaces it. Three routes go |
| `reset-catalog` | **Kept**, as "delete your fork". Needs `forked_from_id` |
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
| Editing a row | forks | **forks.** Same rule, both trees |
| Re-import over an existing slug | refreshes it. The tree on disk is the truth | **refreshes it.** Same rule, both trees |

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
| 2 | **The ref resolver reads two scopes, mine first, then shipped.** With 0 slugs shared today the tie-break fires on nothing, so its test must construct a collision by hand rather than lean on the trees | 2a importer, `store/content.go` |
| 3 | **Import order is now correctness, not tidiness.** Shipped first. The old wording — extra trees "merged *before* the embedded catalog" — is backwards; it is also in `config/app.go`'s doc comment on `Catalog.Dirs`. Both to fix | 1.5, 2a |
| 4 | **A first boot cannot import a private tree.** No account exists, so the owner email resolves to nothing. Warn naming the email and the dir, keep booting, import on the next boot. Refusing to boot would deadlock — you cannot sign up before the listener opens | 1.5 step 7 |
| 5 | **A file-sourced row is read-only in the app; the import refreshes it.** Reversed 2026-09-08 after review. The earlier rule — imported rows edit in place, the import never overwrites — killed the file: once imported, editing the YAML did nothing for ever. So both trees are one-way inputs and behave identically. Editing any file-sourced row produces a copy that the import never touches. **This needs `content.source_tree` (see 1.11) — the row's own answer to "may the importer refresh me?"** | 2a, 1.11 |
| 6 | **Deleting the account erases its private catalog.** The cascade follows `content.author_id`; that is the design working, not a leak. Re-importable from disk next boot. Not worth engineering around — worth knowing before clicking | 1 gate, 2b |
| 7 | **"You may not edit this" stops meaning `author_id IS NULL`** and becomes `author_id IS NULL OR source_tree IS NOT NULL`. Your imported private content is read-only in the app for the same reason shipped content is: a file owns it. `ErrShipped`'s message needs rewording, since "this comes with the app" is wrong for a private tree | 2b, `statusFor` copy |
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

**Decided 2026-09-08.** Replaces the log's text link to content.

Today `log_entry` carries two links to a movement — `movement_id BIGINT` and
`movement_slug VARCHAR(128)` — and `ix_entry_progression` groups by **the slug**. Both reasons
for that are now dead: SQLite row-id reuse was fixed by `AUTOINCREMENT`, and surviving a
deleted movement is what `content.retired_on` is for.

**`content.content_key CHAR(36)`, and a fork inherits its parent's.** `log_entry` keeps one
link — `movement_key` — replacing both columns, and progression groups by it.

Not called `uuid` on purpose: a fork and its original hold the same value, so the column is
not unique per row. Anyone who reads `uuid` eventually adds `UNIQUE` to it and breaks forking.

| Job | Key |
|---|---|
| One YAML file naming another (`ref:`) | slug — a uuid cannot be hand-written |
| Matching a file to its existing row on re-import | slug |
| A page's URL | slug |
| A finished run's link to its movement | **content_key** |
| Holding history together across a fork | **content_key** |

So a slug stops being identity and becomes the human handle. Renaming one is safe for history.

**Minted at random on first insert. Never written into a YAML file.** The alternative — deriving
it from `(kind, slug)` — is stable across a rebuild but makes the key a hashed name, which is
the thing this change exists to stop. Random is safe because content and log live in the same
database file and are destroyed together, with exactly one exception, which becomes a rule:

> **An engine move copies every table. It is never done by re-running the importer.** Re-importing
> mints fresh keys while the old `log_entry` rows keep the old ones, which splits history at the
> move. This is the only way to reach that bug, and it is a runbook line, not a schema problem.

**Still true after this change, and worth stating:** renaming a *file* is a delete plus a create.
The importer matches by slug, so the old row retires and a new row appears with a new key. History
stays on the retired row. `content_key` does not rescue this and is not meant to.

`plan_target`, `plan_slot` and `scheduled` keep their BIGINT ids — they are current state, not
history. `log.session_slug` is untouched: nothing groups or indexes on it. Both deliberate.

**This one does go in migration 001**, while it is free — restructuring `log_entry`'s NOT NULL
columns and an index after real rows exist is exactly what SQLite makes expensive.

---

# Part 2 — the catalog YAML format

**The spec of record is [CATALOG_FORMAT.md](CATALOG_FORMAT.md)**, which lists every key per
kind. This part states the decisions and why. The essentials:

```
catalog/
  tags.yaml                  the whole tag vocabulary, 28 entries
  movements/<slug>.yaml
  menus/<slug>.yaml
  blocks/<slug>.yaml
  sessions/<slug>.yaml
```

**Rules.** `slug` is required and equals the filename stem, so a rename is a `git mv` and
shows up in review. A slug is unique **across all four kinds**, which is what lets a bare
`ref:` name a target without naming its kind — all **252** existing refs keep their shape.
`tags` is a list validated against `tags.yaml`; an unknown tag is a hard failure, so a typo
cannot mint `shouldres`. Every number is optional; absent is NULL, and `0` is a real value.
List order is `position`, renumbered densely by the importer.

**No inline children.** Every content row is a file. This is the largest mechanical change:
the trees hold **27 inline exercises** (24 of them menus) and **22 inline blocks**, 9 with no
name. The conversion invents 22 block slugs, 24 menu slugs and 9 block names, once, by hand.
The file count goes from 225 to **274**. An earlier draft said 271: it counted the 22 blocks
and 24 menus and forgot the **3 inline movements**, which become files too.

**Two list keys, not one and not three.** A session's list and a block's list are both
`items:`; a menu's list is `options:`. The distinction is the one a coach actually reads for: a
session's list means "this, then this", a block's means "do all of these", and a menu's means
"choose from these". The first two read alike and can share a word. The third does not, and
putting the arity on the parent line only would make a reader look up a line to learn whether
the list below is do-all or choose-one. `children:` disappears.

**Invented slugs come from names, and positional is the last resort.** Revised 2026-09-08
after review. A real YAML parse settles the split: of the **49** inline items, **40 already
carry a `name:`** and only **9 do not**. So the earlier `<parent_slug>__menu<position>` rule
would have thrown away 40 perfectly good names to avoid four collisions, and a positional slug
breaks the moment a list is reordered.

The rule, in order:

1. **Has a name → slug from the name.** Covers all 24 menus and all 13 named inline blocks.
2. **Name collides → qualify with the parent's name**, which is what the trees already do by
   hand (`drills_paradigm`, `warm_up_paradigm`). This clears all four known collisions.
3. **`<parent_slug>__<position>` only for something genuinely unnameable.** Expected uses: zero.

**All nine unnamed blocks get named. None are deleted.** Review suggested letting a session
hold a movement directly, which would delete two of them — `boulder_session.yaml`'s wrapper
holds a single ref, and `emil_submax_daily_fingerboard_routine.yaml`'s wrapper *is* the whole
session. **Rejected, and the schema is why.** `ck_item_pair` permits session→block,
block→movement|menu and menu→movement, and nothing else. Loosening it would make a session's
child list heterogeneous, so every renderer of a session — run screen, preview, log — would
have to handle "this child is a movement, not a block".

The two cases do not earn that. The Emil wrapper holds **six** movements, so it wants a name
whatever the schema allows. The boulder wrapper holds one, and "Bouldering" is a fine name for
it. **So: 9 blocks to name by hand**, in one sitting — not the 49 items an earlier count in
conversation claimed, and not the 7 an earlier draft of this paragraph claimed.

**A ref item, the same shape in every list:**

```yaml
- ref: weighted_pull_ups
  sets: 4
  reps: 5
  weight_kg: 10
  rep_rest_seconds: 60
  per_set:
    - reps: 5
    - reps: 3
    - reps: 3
```

`per_set` replaces the `rungs` string. Invariant: `sets == len(per_set)` when `per_set` is
present, enforced by the importer.

**Four renames, not two.** `label:` → `tags:` — a comma string becomes a list, in **all 225
files**, and it is the one the importer hard-fails on. `kind:` → `movement_kind:` in the 184
movement files. Then: `movement_kind: session` — on **56 of 184** entries — becomes **`open`**,
because otherwise the string `session` means two different things in two columns. Media keys
`video_url` → `url` and `thumbnail_url` → `thumb_url`, one sed across the **112** files that
carry media.

**Not `duration`.** Revised 2026-09-08: of those 56, exactly **one** carries
`session_duration_seconds`. The other 55 carry no dose key at all, as do all 57 `climbing`
movements. The kind does not mean "one long stretch of time" — it means "no numbers, just do
the thing". `duration` would send a reader hunting for a number that is not there and tempt the
importer author into requiring one. (113 of 184 movements have no numbers at all, so this kind
is doing little work; the real distinction is that `climbing` opens the tick logger.)

**New key: `per_side: true`.** The highest-value gap review found, and a live bug rather than a
format nicety. **33 movements say "per side" or "per leg" in prose only**, and the numbers are
already ambiguous in the data: `bulgarian_split_squats` is `sets: 1, reps: 6` with "per side" in
the notes, so the player counts 6 when you owe 12; `heel_hook_isometric_pull` is `sets: 6,
reps: 1` "per leg", and no reader can tell whether that is six total or six each. One boolean
fixes all 33 and lets the run timer say left and right.

**Two gaps recorded and deliberately not fixed in V2**, so they are decisions rather than
oversights. **Intensity**: 49 movements carry it only as prose ("~RPE 8", "well below your
limit"), which for finger work is the safety-relevant number — without a field a cycle can
progress volume and never intensity. **Edge and grip**: 19 slugs bake the geometry in
(`max_hangs_20_mm_edge`, six `isometric_hang_*_{open,half_crimp}`, three `hangboard_ladder_*`),
so every new edge needs a new file and the file count grows on the wrong axis. Both want a field
on the movement, overridable on a ref. Neither is a V2 feature; both are written here so the
next person knows the limit was seen.

**`pick_count` means the fewest options you must choose, not the most.** It matches the
handler that already accepts a list, and the "pick one or more of these" menu in
`pcc_go_hard.yaml` is the case that proves the min reading is right.

**It does not cover all 24 menus, and two claims in the earlier draft were false.** Corrected
2026-09-08 after review:

- *"All 24 state their arity in prose."* **Three state nothing at all** — "Easy campusing
  (optional)" and "Endurance Method" in `endurance.yaml`, "Lead Focus" in `lead.yaml`, and
  "Endurance Method" in `pcc_do_more.yaml`. Each needs a decision, not a sed.
- *"One menu says choose a couple, so `pick: 2`."* That phrase is in
  `exercises/paradigm/muscular_activation.yaml`, which is `kind: "session"` — a plain movement,
  not a menu. **No pick-2 menu exists in either tree.** That file is arguably a menu written as
  a movement, which is a separate thing to catch during the conversion. So is
  `power_company/high_pressure_sends.yaml`, whose notes say "choose one of these formats" and
  then describe two protocols that already exist as their own files.

**`pick_count: 0` means "you may skip this", and that is where optionality lives.** Four menus
currently put it in the display name: "Wall Crawls (optional)", "Strength (optional)", "Prehab
(optional)", "Easy campusing (optional)". Under the min reading, 0 says it properly, so state
that 0 is legal and drop the four suffixes. Otherwise the one fact the app could act on stays
buried in a name. Ranges need no key today; add `pick_max` when a file asks for one.

**The complete key table per kind is published before any file is converted.** Added
2026-09-08 after review, which found this fatal rather than untidy: the importer hard-fails on
an unknown key, and six keys that exist in the data appear in no draft of this spec —
`source:` (124 files), `media:` (112), `color:` (17), `needs:` (5), `rung_seconds:` (3),
`session_duration_seconds:` (1). Taken literally, **every session file would refuse to import
on `color:`**. The hard-error rule makes any omission a boot failure, so the table is a
prerequisite for 2a, not documentation written after it.

The private tree is a separate repository with its own git and no shared CI, so adding one key
to the format breaks that tree on the next boot until it is converted too. Convert both in the
same pass.

**Every string stays quoted, and the reason goes in the spec.** All 252 refs already are. This
is load-bearing, not house style: tested against `gopkg.in/yaml.v3`, `90_90` parses as the
integer **9090** and `007` as **7**. Five slugs start with a digit. Someone will eventually
remove the quotes as tidying, so the spec has to say why they are there.

**`format_version` in one `catalog.yaml` at each tree root, not in every file.** Revised
2026-09-08: it is one constant per tree, and copying it into 271 files creates 271 chances to
disagree while a `git mv` does not touch it. `tags.yaml` already sets the precedent for a
tree-level file. Same guarantee — a stale private tree fails loudly — for one line instead of 271.

**No `source:` is added to `catalog/exercises/bechtel/`.** Review caught this against our own
proposal, and it is a licence matter, not a style one. Those five files carry no `source:`
today and are generic barbell and bodyweight lifts in our own words. The folder is named after
a paid ebook, so adding `source: "Logical Progression"` would **newly assert paid-programme
provenance in the published tree** for content that does not need it. Delete the coach folder
and let a deadlift be a deadlift. The other 24 of the **29** public exercises missing a source
(`ondra/` 15, `emil/` 6, `nelson/` 3) are publicly known free content and CLAUDE.md permits
naming them. Earlier drafts said 26; the count is 29.

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

The format spec, `tags.yaml`, the importer, **the exporter** (moved here from phase 11 — it
is the format's second implementation and the only thing that proves a round trip), and the
public tree's 102 slugs converted.

**Gate.** `count(*) FROM content WHERE kind='movement' AND author_id IS NULL` = 88. Import
twice; every table's checksum unchanged. Import a tree with a duplicate slug across kinds, and
one with an unknown tag; both fail loudly. Export a session, re-import into an empty database,
row and edge counts match — **counts, not keys: 1.11 mints a fresh `content_key` on insert, so
asserting key equality across a re-import would fail by construction.** Per 1.10: a private
tree naming an unknown owner email is skipped with a warning and the boot continues; the same
tree imports once the account exists. Per 1.10 item 5: a re-import **refreshes** a row whose
`source_tree` matches the importing tree, and leaves a row with `source_tree IS NULL`
untouched. A file whose only change is a key the spec does not list fails the import, naming
the key and the file.

### Phase 2b — the private tree, content reads, the library · **XL**

The private tree's 123 slugs imported **as one account's own content** per 1.10,
`store/content.go` (the one visibility function, tree read, fork), and the library pages.

**Gate.** Movement count = 184, and it splits: `author_id IS NULL` is still 88 after the
private import — that count not moving is what proves the private tree did not leak into
shipped. A second account sees 102 slugs and none of the 123. `ForkSession` leaves the original
row byte-identical, copies its blocks and edges, keeps the slug, sets `forked_from_id`, and
creates **zero** new movement rows. `slug='silent_feet'` is one row while its `content_item`
count is greater than one. Per 1.10 items 1 and 6: an owned session holds edges to shipped
children, and deleting the owner removes the session and its edges while every shipped child
survives. Per 1.11: a fork's `content_key` equals its parent's, two unrelated movements that
share a slug across the shipped and owned indexes do **not** share a key, and a progression
query spanning a fork returns one series rather than two. Per 1.10 item 5: editing a row with
`source_tree` set is refused with a message that does not say "comes with the app" when the
row came from a private tree. **A regression test asserts the content picker never returns
another account's private content** — `content_item` has no author-comparing constraint, so
nothing in the database will catch that if the picker regresses.

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
planned sets. Editing a shipped block forks the block and not its movements. `sets` equals
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
| **The private catalog is a second repository.** Two trees, one format, no shared CI | Convert both in the same week. `format_version` in every file |
| **Two live databases while I train.** Master runs production; V2 runs alongside from phase 3 | Decide which is the real log during the build. If it is V2's, "old data is disposable" stops being true from phase 3, and the SQLite file needs a backup story |
| **Feature loss by omission.** 37 sub-actions hide behind 7 catch-all route patterns; a route dropped by accident looks identical to one dropped on purpose | Enumerate all 37 with a phase or the word "drop" beside each. `V2_PLAN_REVIEW.md` §7 is the start |
| **The catalog conversion is your content** | Public tree first. Show one file's diff and get agreement before touching the private repo |
| **The owned import has a dependency the shipped import never had** (1.10): an account must exist, and 39 refs need the shipped rows already in place | Fixed import order. Skip-with-warning on an unresolved owner. A gate that boots empty, signs up, boots again, and finds the tree |
| **A stranger can claim the private catalog** by signing up with the configured `owner:` email while no account holds it (1.10 item 8) | **Accepted, knowingly, 2026-09-08.** Not mitigated by design. Log the resolved binding at boot with email and account id. Revisit only if this instance ever takes signups from other people |
| **The tag vocabulary is five axes in one flat list** — body part, equipment, discipline, quality, session role, skill — with 10 tokens used once or twice, and hard validation would freeze `hinge` and `squat` as permanent vocabulary | Clean `tags.yaml` before turning the hard failure on: collapse `shoulder`/`shoulders`, settle `antagonist` versus `prehab`, drop or promote the singletons |
| **The `web/` file list is a guess** | Expected to move at phase 4. Not a commitment |

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
