# The catalog YAML format

This is the spec of record for a catalog tree. `V2_PLAN.md` Part 2 states the decisions and
why they were made. This file states the keys.

**Every key is listed here.** The importer refuses a key it does not recognise, so an
omission in this file is a boot failure, not a documentation gap. That is why the table comes
before the conversion.

Counts below are measured across both trees on 2026-09-08: 184 movement files, 24 activity
templates, 17 session templates.

---

## The tree

```
<tree>/
  catalog.yaml               format_version, and nothing else yet
  tags.yaml                  the whole tag vocabulary — shipped tree only
  movements/<slug>.yaml
  menus/<slug>.yaml
  blocks/<slug>.yaml
  sessions/<slug>.yaml
```

A slug is unique across all four kinds. That is what lets `ref: "silent_feet"` name a target
without naming its kind.

## Rules for every file

1. `slug` is required and equals the filename without `.yaml`. The importer fails when they
   differ. The key stays because it is the row's identity, and the check stops the two
   drifting apart. **46 of 225 files disagree today** and become 46 renames.
2. **Quote a string that starts with `#`, and quote the rest by convention.** Corrected
   2026-09-08 after measuring, because an earlier draft of this file overstated it.

   Only `#` is load-bearing: unquoted, it starts a YAML comment, so every `color` must be
   quoted. The exporter's marshaller does this on its own.

   Everything else is safe, because the loader decodes into typed fields. Tested against
   `gopkg.in/yaml.v3`: written unquoted, `90_90_hold`, `no`, `007` and `3_strike_repeat` all
   arrive as those exact strings in a `string` field. Only an untyped decode turns `007` into
   `7`, and nothing here does an untyped decode. Five slugs start with a digit and every one
   has letters in it, so none is even a candidate.

   Quote anyway in a hand-written file. It reads consistently and it survives another tool
   reading the tree less carefully. The exporter writes what the marshaller thinks is safe,
   which is unquoted for most values.
3. Every number is optional unless the table says otherwise. Absent means NULL. `0` is a
   real value, so `reps: 0` is not the same as no `reps`.
4. `tags` is a list, validated against `tags.yaml`. An unknown tag fails the import.

   **There is one `tags.yaml`, in the shipped tree**, and it holds **21 entries**. A private
   tree needs none: after the merges recorded in that file, every tag either tree uses is in
   the list. The only tag once unique to a private tree was `lead`, now `route`. If a private
   tree ever wants a tag of its own, give it its own `tags.yaml` whose entries are merged with
   the shipped one — do not copy the shipped list, or the two will drift.

   The list went from **29 tags to 21** on 2026-09-08. Four merges — `shoulders` into
   `shoulder`, `stretching` into `mobility`, `lead` into `route`, `prehab` into `antagonist`.
   Four drops — `squat`, `hinge` and `pressing`, which were one movement-pattern tag per file
   and not enough to be an axis, and `rest`, whose one meaning is simply gone. `mental` was
   kept on one file, `fall_practice`, because falling practice is a mental thing and no other
   tag says that. `tags.yaml` records every one with the files it touched.
5. List order becomes `position`, renumbered from 0 by the importer.
6. **No inline children.** Every row is a file. See "What the conversion creates".

---

## `movements/<slug>.yaml` → `content`, kind `movement`

| key | column | required | files |
|---|---|---|---|
| `name` | `content.name` | yes | 184 |
| `slug` | `content.slug` | yes | 184 |
| `kind` | `content.movement_kind` | yes | 184 |
| `tags` | `content_tag` rows | yes | 184 |
| `notes` | `content.notes` | no | 183 |
| `source` | `content.source` | no | 106 |
| `media` | `content_media` rows | no | 112 |
| `per_side` | `content.per_side` | no | 24 need it |
| `sets` | `content.d_sets` | no | 70 |
| `reps` | `content.d_reps` | no | 70 |
| `weight_kg` | `content.d_weight_kg` | no | 1 |
| `rep_seconds` | `content.d_rep_seconds` | no | 32 |
| `rep_rest_seconds` | `content.d_rep_rest_seconds` | no | 17 |
| `set_rest_seconds` | `content.d_set_rest_seconds` | no | 43 |
| `prep_seconds` | `content.d_prep_seconds` | no | 35 |
| `seconds` | `content.d_seconds` | no | 1 |
| `per_set` | `content_set` rows | no | 3 |

`kind` is one of four values. It says how you count the movement:

| value | files | meaning |
|---|---|---|
| `climbing` | 57 | opens the tick logger |
| `open` | 56 | no numbers, just do the thing |
| `reps_and_sets` | 36 | counted in reps |
| `timed_reps` | 35 | counted in seconds |

`open` is today's `session`, renamed. Of those 56 files, **55 carry no dose key at all**, and
neither do any of the 57 `climbing` files. So the value does not mean "one long stretch of
time", and naming it `duration` would send a reader hunting for a number that is not there.

`media` is a list, and stays a list: 112 files carry 120 entries between them, and 6 of those
files carry more than one. Each entry is `{url, thumb_url}` and both are optional — 3 entries
have no thumbnail and 2 have no video.

`seconds` is today's `session_duration_seconds`, renamed to match its column.

**`per_side` is new, and it fixes a live bug.** **24 movements** say "per side", "per leg",
"per arm", "each side", "each leg" or "each hand" in prose only. A wider pattern that also
catches "each way", "both sides" and "swap legs" finds 27. Review claimed 33; I could not
reproduce that with either pattern, so treat 24 as the floor and settle the exact list during
the conversion.

Both examples are measured. `bulgarian_split_squats` is `sets: 1, reps: 6` with "per side" in
the notes, so the player counts 6 when you owe 12. `heel_hook_isometric_pull_60_90_120_degrees`
is `sets: 6, reps: 1` with "per leg", and no reader can tell whether that is six total or six
each.

**`rung_seconds` becomes `per_set`, and it stays on the movement.** Three files carry it, as
the string `"3,6,9"`. It is a ladder: one set is a 3-second hang, then 6, then 9.

```yaml
name: "Hangboard Ladder: Half Crimp"
kind: "timed_reps"
sets: 3
per_set:
  - seconds: 3
  - seconds: 6
  - seconds: 9
rep_rest_seconds: 60
```

An earlier draft moved the ladder to the block that uses it. **That was wrong.** The name, the
slug and the notes of those three files all describe the ladder, so moving it out would leave
three movements named after a shape they no longer held, and no movement could ever be a
ladder again.

`per_set` rows land in `content_set`, a table added for this. Research first: seven other apps
keep their movement library plain and put non-uniform sets in the workout. They can, because
they also ship a library of named protocols — Crimpd has 200 of them. Here that would need a
block inside a block, and the schema forbids it on purpose, because the fixed chain is what
makes a loop impossible.

A block that uses a ladder writes only `- ref: "hangboard_ladder_half_crimp"` and inherits the
shape, the same way it inherits `sets`. It can still override with its own `per_set`.

---

## `menus/<slug>.yaml` → `content`, kind `menu`

| key | column | required | notes |
|---|---|---|---|
| `name` | `content.name` | yes | all 24 have one |
| `slug` | `content.slug` | yes | invented by the conversion |
| `pick` | `content.pick_count` | yes | default 1 |
| `options` | `content_item` rows | yes | the list of choices |
| `notes` | `content.notes` | no | all 24 have one |
| `tags` | `content_tag` rows | no | none carry one today |

**`pick` is the fewest you must choose, not the most.** `pick: 0` means you may skip the
menu. Four menus currently put that in the display name — "Wall Crawls (optional)", "Strength
(optional)", "Prehab (optional)", "Easy campusing (optional)" — so those four become `pick: 0`
and lose the suffix.

**Three menus state no arity at all** and need a decision each, not a rule: "Easy campusing
(optional)" and "Endurance Method" in `endurance.yaml`, "Lead Focus" in `lead.yaml`, and
"Endurance Method" in `pcc_do_more.yaml`.

The list key is `options` and not `items`. A session's list means "this, then this". A
block's means "do all of these". A menu's means "choose from these". The first two read alike
and share a word. The third does not.

---

## `blocks/<slug>.yaml` → `content`, kind `block`

| key | column | required | files |
|---|---|---|---|
| `name` | `content.name` | yes | 24 |
| `slug` | `content.slug` | yes | 24 |
| `tags` | `content_tag` rows | yes | 24 |
| `items` | `content_item` rows | yes | 24 |
| `source` | `content.source` | no | 13 |
| `role` | `content.block_kind` | no | 4 need it |
| `notes` | `content.notes` | no | 4 |

`role` is `warmup`, `main` or `cooldown`. It sits on the block and not on a session's use of
it. Four inline blocks carry `type: "warmup"` or `type: "cooldown"` today, and no block that
is referenced by slug carries one. So no block is a warm-up in one session and the main event
in another, and nothing is lost.

A block's `items` may hold a movement or a menu. Not a session and not another block.

---

## `sessions/<slug>.yaml` → `content`, kind `session`

| key | column | required | files |
|---|---|---|---|
| `name` | `content.name` | yes | 17 |
| `slug` | `content.slug` | yes | 17 |
| `color` | `content.color` | yes | 17 |
| `tags` | `content_tag` rows | yes | 17 |
| `items` | `content_item` rows | yes | 17 |
| `source` | `content.source` | no | 5 |
| `needs` | `content.needs` | no | 5 |
| `notes` | `content.notes` | no | 0 |

`color` is required because all 17 files carry one. All 10 distinct values are `#rrggbb`
strings, so the key must be quoted: an unquoted `#` starts a YAML comment.

**A session's `items` may hold blocks only.** Not movements and not menus. `ck_item_pair` in
the schema permits session→block, block→movement|menu and menu→movement, and nothing else.

Two consequences follow, and both were proposed as savings during review and both are
refused by that one rule:

- A session cannot point straight at a movement, so the two unnamed wrapper blocks in
  `boulder_session.yaml` and `emil_submax_daily_fingerboard_routine.yaml` get names.
- A session cannot point straight at a menu, so the **11 blocks that wrap exactly one menu**
  stay as files.

`needs` is free text about equipment, on 5 files. Equipment is really a property of a
movement, not of a session. Left as it is for V2 and recorded under "Known limits".

---

## A reference, in any `items` or `options` list → `content_item`

```yaml
- ref: "weighted_pull_ups"
  sets: 4
  reps: 5
  weight_kg: 10
  rep_rest_seconds: 60
  per_set:
    - reps: 5
    - reps: 3
    - reps: 3
```

| key | column | required | uses today |
|---|---|---|---|
| `ref` | resolves to `child_id` | yes | 252 |
| `sets` | `content_item.sets` | no | 1 |
| `reps` | `content_item.reps` | no | 1 |
| `weight_kg` | `content_item.weight_kg` | no | 0 |
| `rep_seconds` | `content_item.rep_seconds` | no | 1 |
| `rep_rest_seconds` | `content_item.rep_rest_seconds` | no | 0 |
| `set_rest_seconds` | `content_item.set_rest_seconds` | no | 1 |
| `prep_seconds` | `content_item.prep_seconds` | no | 0 |
| `seconds` | `content_item.seconds` | no | 0 |
| `notes` | `content_item.notes` | no | 0 |
| `per_set` | `content_item_set` rows | no | 0, and 3 arrive from `rung_seconds` |

A number here overrides the movement's default for this use only. One reference uses this
today: `heavy_feet` in `a_foundations.yaml` sets `sets: 3, reps: 1`.

`per_set` is a ladder. Its entries are reps inside one set, which is why the key on
`content_item_set` is `(item, set_index, rep_index)`. Invariant, enforced by the importer:
when `per_set` is present, `sets` equals its length.

**`name` on a reference is dropped.** Six references carry one, as a per-use display name.
The name belongs to the row being referenced.

**A reference may point outside its own tree, and usually does.** A private tree leans on
the shipped movements rather than carrying copies of them: the real one does this 39 times.
So loading a tree needs the index of every tree loaded before it, and the shipped tree is
always loaded first.

**A reference resolves in this order: this tree, then your own content, then the app's.** So a
movement you own shadows a shipped one with the same slug.

**A file cannot take a slug you already used.** If you built something in the app and a file
in the tree has its slug, the import is refused and names both. It cannot overwrite your row.
It also cannot quietly leave it, because then every file pointing at that slug would get your
row instead of the one the tree describes, and the import would report success while the tree
meant something else. Rename yours, or rename the file.

---

## What the conversion creates

The trees hold 49 items written inside another file. Each becomes its own file.

| | count | of those |
|---|---|---|
| Inline blocks | 22 | 13 already named, **9 need a name** |
| Inline menus | 24 | all named |
| Inline movements | 3 | all named |

**File count goes from 225 to 274.**

Slugs for the new files come from names, in this order:

1. **Has a name → slug from the name.** Covers all 24 menus, all 3 movements, and 13 blocks.
2. **Name collides → qualify with the parent's name.** The trees already do this by hand:
   `drills_paradigm`, `warm_up_paradigm`. **There are 8 collisions, not the 4 an earlier
   draft claimed.** Derived from names, these appear twice each: `drills`, `endurance_method`,
   `endurance_work`, `journal`, `movement_practice_board`, `movement_practice_endurance`,
   `movement_practice_lead`, `workout`. Most are the wrap-one-thing pattern — a file and the
   single item inside it share a name — so the parent's name is exactly the right qualifier.
3. `<parent_slug>__<position>` only for something genuinely unnameable. Expected uses: zero.
   A positional slug breaks when a list is reordered, which is why it is last.

The 9 unnamed blocks are the warm-up, main and cool-down of one session each. Three or four
words apiece.

**The 3 inline movements need a judgement, not a rule.** Found by structure, they are:

| name | in | kind |
|---|---|---|
| "Journal" | `catalog/activity_templates/journal.yaml` | `session` |
| "Apply: Mantra Routes" | `private/session_templates/a_foundations.yaml` | `climbing` |
| "Apply: Dynamic Choice" | `private/session_templates/b_flow_and_power.yaml` | `climbing` |

Decide during the conversion whether each becomes a new movement file or should reference one
that already exists. Note that "Journal" sits inside a block also called Journal, which is the
same wrap-one-thing pattern as `drills.yaml`.

---

## Renames from today's format

| today | becomes | files touched |
|---|---|---|
| `label: "a, b"` | `tags: ["a", "b"]` | 225 |
| `kind:` on a movement | `kind:` — unchanged | 184 |
| `kind: "session"` | `kind: "open"` | 56 |
| `video_url` | `url` | 112 |
| `thumbnail_url` | `thumb_url` | 112 |
| `session_duration_seconds` | `seconds` | 1 |
| `activities:` on a session | `items:` | 17 |
| `exercises:` on a block | `items:` | 24 |
| `children:` on a menu | `options:` | 24 |
| `type:` | gone | 22 |
| `rung_seconds: "3,6,9"` | `per_set` on the reference | 3 |
| `format_version` in every file | one `catalog.yaml` per tree | — |

`type:` needs no replacement. A reference is one because it has `ref:`. A menu is one because
it has `pick:`. Anything else is a plain group. `type:` currently reaches the screen as the
literal words "activity", "warmup" and "cooldown" in five templates, so naming the 9 blocks
improves what a reader sees.

---

## What is deliberately not in the format

Each of these is a limit that was seen and accepted, not an oversight.

**Intensity.** **23 movements** carry it as prose only — "~RPE 8", "one grade harder than
drill terrain", "well below your limit". For finger work that is the number that matters for
injury. Without a field, a cycle can progress volume and never intensity. Wants a key on the
movement and on a reference. (Review claimed 49. My pattern finds 23, and I could not
reproduce the higher figure.)

**Edge size and grip type.** **20 slugs** bake the geometry in: `max_hangs_20_mm_edge`, six
`isometric_hang_*_{open,half_crimp}`, three `hangboard_ladder_*`, and the rest. Every new edge
needs a new file, so the file count grows on the wrong axis. Two keys would collapse those 20
toward about 6.

**Rest between items.** `set_rest_seconds` covers rest inside a movement. Nothing says "10
minutes before the strength block".

**Equipment as a real field.** `needs` is session-level free text on 5 files, while `tags`
already smuggles equipment in as `hangboard` (15) and `board` (11).

**A menu written as a movement.** At least two files are menus in intent but `kind:` says
otherwise: `power_company/high_pressure_sends.yaml` says "choose one of these formats" and
then describes two protocols that already exist as files, and
`paradigm/muscular_activation.yaml` says "choose a couple of the exercises below". So the menu
count is 24 by `kind:` and at least 26 in intent. Catch these during the conversion.

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
