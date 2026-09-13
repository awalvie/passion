# The catalog YAML format

This is the spec of record for a catalog tree. `V2_PLAN.md` Part 2 states the decisions and
why they were made. This file states the keys.

**Every key is listed here.** The loader refuses a key it does not recognise, so an omission
in this file is a boot failure, not a documentation gap.

`format_version: 2`. Version 1 was the first draft of this format and was never used on real
data; the loader refuses it, and refuses a `menus/` directory, naming the fix.

---

## The tree

```
<tree>/
  catalog.yaml               format_version, and nothing else yet
  movements/<slug>.yaml
  blocks/<slug>.yaml
  sessions/<slug>.yaml
```

Three directories. A menu has no file: it is written inside the block that holds it.

## The two rules that shape everything else

**A row's identity is where its file sits.** The directory gives the kind, the filename gives
the name. Neither is repeated inside the file — there is no `slug:` key. Two files cannot
claim one identity, because two files cannot share a path.

A filename must be lower case letters, digits and underscores.

**A name only has to be unique within its kind.** A block called `drills` and a menu called
`drills` can both exist, because every reference names the kind it expects. The database
already worked this way: `ux_content_shipped (kind, slug)` and
`ux_content_mine (author_id, kind, slug)` both include the kind.

## References: two namespaces, no fallback

A bare name means the tree the file is in. A name with the `app:` prefix means the catalog the
app ships.

```yaml
items:
  - block: warm_up            # a block in this tree
  - block: app:drills         # a block the app ships
```

There is no fallback between the two. A bare name never reaches the app's catalog, and an
`app:` name never reaches this tree. So adding a row can never change what an existing
reference means.

This is the failure the two namespaces exist to prevent. Hugo shipped scoped reference lookup
and removed it in 0.45, writing that it is too easy for links to point at the wrong page, and
that adding a page can make an existing reference ambiguous so an originally correct link
points elsewhere with no warning. dbt refuses an ambiguous `ref()` for the same reason.

When a reference resolves to nothing, the error says which prefix to write, because with two
namespaces that is knowable:

```
blocks/warm_up.yaml: items[0]: no movement "half_crimp_hang" in this tree.
                     The app's catalog has one — write "app:half_crimp_hang"
```

**A file never names another account's content.** Sharing between accounts happens in the app,
by copying. So two namespaces is the whole of it, permanently.

## Rules for every file

1. **Quote a string that starts with `#`, and quote the rest by convention.**

   Only `#` is load-bearing: unquoted, it starts a YAML comment, so every `color` must be
   quoted. The exporter's marshaller does this on its own.

   Everything else is safe, because the loader decodes into typed fields. Tested against
   `gopkg.in/yaml.v3`: written unquoted, `90_90_hold`, `no`, `007` and `3_strike_repeat` all
   arrive as those exact strings in a `string` field. Only an untyped decode turns `007` into
   `7`, and nothing here does an untyped decode.

   Quote anyway in a hand-written file. It reads consistently and it survives another tool
   reading the tree less carefully.
2. Every number is optional unless the table says otherwise. Absent means NULL. `0` is a real
   value, so `reps: 0` is not the same as no `reps`.
3. `tags` is a list of slugs. Write whatever you like: a tag nothing has used before is
   created on first use. There is no list to add it to.

   A tag has to look like a slug — lower case, digits and underscores — so `warm_up` and
   `Warm Up` cannot become two tags for one idea. Its label in the app comes from the slug
   (`hip_mobility` shows as "Hip Mobility") and you can rename it there afterwards.
4. List order becomes `position`, renumbered from 0 by the importer.
5. One parent cannot hold one child twice. `ux_item_edge` is `UNIQUE (parent_id, child_id)`,
   which is also why a ladder is one reference with `per_set` rather than three references.

---

## `id:` — every file carries one

```yaml
# movements/half_crimp_20mm.yaml
id: 3f2a91c4-7b6e-4d15-9a03-1e8c5f2d4b7a
name: Half Crimp Hang, 20 mm
style: timed_reps
```

You do not type it. The first time the app imports a tree it writes one into every file that has
none. `passion catalog lint --fix <dir>` does the same without booting the app.

What it is for:

- The importer matches a file to its row on the `id`. Rename the file, move it to another
  directory, or change its `name`, and it is still the same row.
- An export writes it back out. So a tree can be exported, restored into an empty database, and
  still own the same rows. An id minted at import time cannot do that.

Three rules:

- **Copying a file into your own tree unchanged is fine.** Your copy of the app's
  `movements/pullup.yaml` keeps its id, and ids are unique per owner, not globally. Same id means
  the same `family`, so your history stays one chart.
- **Copying a file WITHIN one tree means changing the `id`, and then setting `family:` to the old
  one.** Without the id change the import is refused and the error names both files. Without the
  `family:` line the copy charts on its own from zero.
- **Never write an id by hand, and never derive one from the file name.** It is a random value on
  purpose, so that nothing about the file can change it.

---

## `movements/<slug>.yaml` → `content`, kind `movement`

| Key | Type | Required | Column |
|---|---|---|---|
| `id` | uuid | **yes**, written for you | `uuid` |
| `family` | uuid | no, only on a copy or a replacement | `family` |
| `name` | string | **yes** | `name` |
| `style` | `climbing` \| `open` \| `reps_and_sets` \| `timed_reps` | **yes** | `movement_style` |
| `aliases` | list of strings | no | see "Renaming" |
| `tags` | list of strings | no | `content_tag` |
| `notes` | string, markdown | no | `notes` |
| `source` | string | no | `source` |
| `per_side` | boolean, default false | no | `per_side` |
| `media` | list of `{url, thumb_url}` | no | `content_media` |
| `sets` | int | no | `d_sets` |
| `reps` | int | no | `d_reps` |
| `weight_kg` | decimal | no | `d_weight_kg` |
| `rep_seconds` | int | no | `d_rep_seconds` |
| `rep_rest_seconds` | int | no | `d_rep_rest_seconds` |
| `set_rest_seconds` | int | no | `d_set_rest_seconds` |
| `prep_seconds` | int | no | `d_prep_seconds` |
| `seconds` | int | no | `d_seconds` |
| `per_set` | list of `{reps, weight_kg, seconds}` | no | `content_set` |

**`style`, not `kind`.** The row's kind comes from the directory. This key says how the
movement is performed, which decides the run screen it gets. Two meanings under one name was
a trap: `plan_target.movement_kind` holds a content kind, and `content.movement_style` holds a
performance style, one join apart.

`open` means the movement carries no numbers at all — a journal entry, a session-long ride.
Not called `duration`, because there is no duration to look for.

**`per_side`** is set when the numbers are per side. Without it, "1 set of 6, per side" can
only be said in prose and the player counts 6 when you owe 12. 14 public movements carry it.

**`per_set` is a ladder**: 3 seconds, then 6, then 9, inside one set. Its entries are reps
inside one set, not sets — so `sets` is required alongside it and says how many times the
ladder is repeated, and `reps` is refused because `per_set` already gives each rep.

---

## `blocks/<slug>.yaml` → `content`, kind `block`

| Key | Type | Required | Column |
|---|---|---|---|
| `id` | uuid | **yes**, written for you | `uuid` |
| `family` | uuid | no, only on a copy or a replacement | `family` |
| `name` | string | **yes** | `name` |
| `aliases` | list of strings | no | see "Renaming" |
| `role` | `warmup` \| `main` \| `cooldown` | no | `block_kind` |
| `tags` | list of strings | no | `content_tag` |
| `notes` | string, markdown | no | `notes` |
| `source` | string | no | `source` |
| `items` | list, non-empty | **yes** | `content_item` |

`role` belongs to the block, not to a session's use of it, so a block cannot be a warm-up in
one session and the main event in another.

A block's `items` hold movements and menus. Nothing else.

---

## `sessions/<slug>.yaml` → `content`, kind `session`

| Key | Type | Required | Column |
|---|---|---|---|
| `id` | uuid | **yes**, written for you | `uuid` |
| `family` | uuid | no, only on a copy or a replacement | `family` |
| `name` | string | **yes** | `name` |
| `aliases` | list of strings | no | see "Renaming" |
| `color` | string, quoted | no | `color` |
| `needs` | string | no | `needs` |
| `tags` | list of strings | no | `content_tag` |
| `notes` | string, markdown | no | `notes` |
| `source` | string | no | `source` |
| `items` | list, non-empty | **yes** | `content_item` |

A session's `items` hold blocks. Only blocks.

That is `ck_item_pair` in the schema, and it is deliberate: allowing a session to hold a
movement would make every session renderer handle two child types across five templates.

---

## An item

Every item names its kind, with exactly one of three keys.

```yaml
items:
  - movement: app:half_crimp_hang
    sets: 4
  - block: warm_up
  - menu:
      name: Shoulder choice
      pick: 1
      of: [app:cuban_press, shoulder_ys]
```

| Key | Meaning |
|---|---|
| `movement` | a reference to a movement |
| `block` | a reference to a block |
| `menu` | a whole menu, written here |

Writing the kind is not decoration. A block's child may be a movement or a menu, so the kind
cannot be worked out from position; the database's uniqueness is per `(kind, slug)`; and the
loader can then check the parent-and-child rule before it touches the database.

A `movement` or `block` item may also carry numbers, which override that child's defaults for
this one use:

| Key | Type | Column |
|---|---|---|
| `notes` | string | `content_item.notes` |
| `sets` | int | `sets` |
| `reps` | int | `reps` |
| `weight_kg` | decimal | `weight_kg` |
| `rep_seconds` | int | `rep_seconds` |
| `rep_rest_seconds` | int | `rep_rest_seconds` |
| `set_rest_seconds` | int | `set_rest_seconds` |
| `prep_seconds` | int | `prep_seconds` |
| `seconds` | int | `seconds` |
| `per_set` | list of `{reps, weight_kg, seconds}` | `content_item_set` |

**A shipped file may not set `weight_kg` on an item.** The importer refuses one. A slot beats
a person's own saved weight in `movement_pref`, so a weight here would override every account
with no way out short of copying the whole session. See the `movement_pref` comment in
`SCHEMA_V2.sql`.

A `menu` item carries no numbers of its own. A menu is a choice, not a thing you do, so the
numbers belong on the option they apply to.

---

## A menu

A menu is written inside its block. It becomes a `content` row with kind `menu`, and its slug
is `<block>_<position>` — `warm_up_2`. That name is never typed, never shown and never
referenced; it exists because every row needs one.

| Key | Type | Required | Column |
|---|---|---|---|
| `pick` | int, `>= 0` | **yes** | `pick_count` |
| `of` | list of options, non-empty | **yes** | `content_item` |
| `name` | string | no | `name`, else "Pick N" |
| `notes` | string, markdown | no | `notes` |
| `tags` | list of strings | no | `content_tag` |

**`pick` is the FEWEST options you must choose, not the most.** `pick: 0` means the menu may
be skipped, and it is the only place optionality can live. The alternative was the word
"optional" inside a display name, which nothing could act on.

`pick` may not exceed the number of options. The database enforces the rest:
`ck_content_pick` requires a count on a menu and forbids one on anything else, because "pick
NULL of these" has no runtime meaning.

**An option is a bare name, or a mapping.** A menu's only legal child is a movement, so the
usual spelling is a bare name. A mapping is how one option carries numbers while its siblings
stay bare, with no reflow of the file:

```yaml
of:
  - app:cuban_press
  - movement: shoulder_ys
    sets: 2
```

A mapping takes `movement` plus `notes` and the number keys above.

**A menu is never reused.** It belongs to one block. That is a product decision, and it is
what lets a menu have no name: nothing needs to point at it.

**A menu is deleted when it leaves its block**, where a movement, block or session is retired.
Nothing can reference a menu, its name is derived, and a log freezes a block's name as text
rather than pointing at the row — so there is nothing a retired menu would protect, and a
retired one would keep its derived name and block the next import of a shortened block.

---

## Renaming

Rename a file, change the directory it sits in, or change its `name`, and the importer still
finds the row it owns, because it matches on `id`. Nothing has to be declared, and nothing is
written to your history: a chart groups on the family, and a rename does not change one.

What a rename still costs is the references. Another file naming the old slug now names nothing,
and that has to be fixed in the same commit. The importer names the exact files that broke.

```yaml
# movements/half_crimp_20mm.yaml   -- was movements/half_crimp_hang.yaml
id: 3f2a91c4-7b6e-4d15-9a03-1e8c5f2d4b7a
name: Half Crimp Hang, 20 mm
```

The `id` did not move, so this is the same row it has always been.

### `aliases:` is a leftover, and it is not inert

Before ids lived in files, the importer matched a file to a row by name, so a rename had to be
declared with `aliases: [old_slug]`. The importer then rewrote the finished log rows that came
from that row. The `id` does the first job, and the family removes the need for the second.

**Until the key is removed, leaving one in a file still rewrites your history.**
`store/catalog_import.go` moves the row and then runs `UPDATE log_entry SET movement_slug` and
`UPDATE log SET session_slug` against finished logs. That write is the thing this design exists
to delete. Removing the key, and that code path, is tracked in `V2_PLAN.md` 1.15.

**Aliases never take part in resolving a reference.** A reference must name the current name.
While the key is still accepted, it only improves the error message:

```
blocks/warm_up.yaml: items[0]: no movement "app:half_crimp_hang".
                     The app renamed it to "half_crimp_20mm"
```

**A person's movement and a shipped movement that share a name stay separate.** They are two
rows with two ids and two families, so they are two charts. That is the point of grouping history
on the family rather than on the name.

**A shipped name is public API.** It appears in people's files and in their logs. Renaming one
breaks their `app:` references, so it is avoided rather than mechanised.

---

## Two kinds of row, and what an edit does

A row carries `source_tree`: the name of the tree that owns it, or NULL when nothing does.

| | |
|---|---|
| A re-import | rewrites the rows whose `source_tree` matches the tree being imported |
| A row with `source_tree` NULL | is **skipped**, and named in the import report |
| Editing a file-owned row in the app | edits it in place, same id and name, and sets `source_tree` to NULL |

Skipped, not refused. Editing a file-owned row is ordinary, and a refusal would stop the whole
tree importing from then on, on every start, because of one edit.

The report is what stops the skip being silent. A person hand-edits these files as their main
way of working, so "I changed the YAML, restarted, nothing happened, nothing told me why" is
the failure to avoid.

Editing a row the app ships is a different thing: the app offers a copy instead, and the copy
**keeps the name**, because the two unique indexes never meet. Keeping the name is what keeps
one history in one series.

Wanting different numbers on a movement the app ships is neither: that is `movement_pref`, a
small per-account override. No copy, no second identity.

---

## What the conversion produced

Measured 2026-09-10, converting `catalog/` from the old three-folder layout:

| | |
|---|---|
| Files written | 106 — 89 movements, 12 blocks, 4 sessions, plus `catalog.yaml` |
| Rows imported | 108 — the 105 files above plus 3 menus written inside blocks |
| Names invented | 3 |
| `per_side` set from prose | 14 |
| `(optional)` dropped from a name | 1 |

The three invented names: `movements/journal.yaml`, which was written inside `journal.yaml`,
and `blocks/bouldering.yaml` and `blocks/hangs.yaml`, which were groups with no name at all
inside a session file.

**The old format's collisions mostly vanished rather than being resolved.** Under the old
rules, deriving names for content that had none produced 25 clashes. Kind-scoped names removed
one class of them, and menus losing their files removed most of the rest.

The conversion is verified three ways: the tree loads, it imports, a second import writes
nothing at all, and the export loads back.

---

## What is deliberately not in the format

Each of these is a limit that was seen and accepted, not an oversight.

**Intensity.** **23 movements** carry it as prose only — "~RPE 8", "one grade harder than
drill terrain", "well below your limit". For finger work that is the number that matters for
injury. Without a field, a cycle can progress volume and never intensity. Wants a key on the
movement and on an item.

**Edge size and grip type.** **20 names** bake the geometry in: `max_hangs_20_mm_edge`, six
`isometric_hang_*_{open,half_crimp}`, three `hangboard_ladder_*`, and the rest. Every new edge
needs a new file, so the file count grows on the wrong axis. Two keys would collapse those 20
toward about 6.

**Rest between items.** `set_rest_seconds` covers rest inside a movement. Nothing says "10
minutes before the strength block".

**Equipment as a real field.** `needs` is session-level free text on 5 files, while `tags`
already smuggles equipment in as `hangboard` (15) and `board` (11).

**A menu written as a movement.** At least two files are menus in intent but say otherwise:
`power_company/high_pressure_sends.yaml` says "choose one of these formats" and then describes
two protocols that already exist as files, and `paradigm/muscular_activation.yaml` says
"choose a couple of the exercises below".

---

## Licence

`catalog/` is published under the repository licence. A private tree is not.

**No `source:` is added to `catalog/exercises/bechtel/`.** Those five files carry no source
today and are generic barbell and bodyweight lifts in our own words. The folder is named
after a paid ebook, so adding one would newly assert paid-programme provenance in the
published tree for content that does not need it. Delete the coach folder instead.

The other 24 of the 29 public movements with no source — `ondra/` 15, `emil/` 6, `nelson/` 3 —
are publicly known free content, and CLAUDE.md permits naming them.

**A `source:` string cannot say whether a source may be published.** 11 of the 12 private
files with no `source:` are session templates, and a programme's session structure is the
protected part. So those are the files where the key would do the most warning work. A list
of known-paid sources, refused by the importer for the shipped tree, would make CLAUDE.md's
rule mechanical instead of a matter of vigilance across two repositories with no shared CI.
