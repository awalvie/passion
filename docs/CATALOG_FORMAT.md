# Catalog format

A catalog is a directory of YAML files. The app ships one in `catalog/`. A private catalog
uses the same format, and the config file says who owns it.

```
<tree>/
  movements/<slug>.yaml    one exercise per file
  blocks/<slug>.yaml       a group of exercises that sessions share
  sessions/<slug>.yaml     one session template per file
```

A directory inside any of the three stops the load. Anything else in the tree is ignored.

## A file

```yaml
# catalog/movements/repeaters_7_3_20_mm_edge.yaml, shortened
id: 693a8297-678c-4c14-8ee9-9b3582f4f588
name: Repeaters 7:3 (20 mm Edge)
style: timed_reps
tags: [endurance, fingers, hangboard]
notes: |-
    Submaximal. Keep 1-2 reps in reserve on the last set.
sets: 4
reps: 6
rep_seconds: 7
rep_rest_seconds: 3
set_rest_seconds: 180
prep_seconds: 10
media:
    - url: https://www.youtube.com/watch?v=...
      thumb_url: https://i.ytimg.com/vi/.../hqdefault.jpg
```

The file name is the slug: lower case letters, digits and underscores. A private file refers to
a shipped exercise by it.

| Key | Required | Holds |
|---|---|---|
| `id` | yes | a uuid. `make catalog-ids` writes it. Never type one by hand |
| `name` | yes | up to 200 characters |
| `style` | yes | `reps_and_sets`, `timed_reps`, `climbing` or `open`. It picks the player screen |
| `tags` | no | a list of words |
| `notes` | no | text, read on a phone mid-session, so keep it short |
| `source` | no | the coach or method it is known by |
| `sets`, `reps` | no | whole numbers, 0 or more |
| `set_rest_seconds`, `rep_seconds`, `rep_rest_seconds`, `prep_seconds` | no | seconds |
| `seconds` | no | how long an `open` exercise runs |
| `media` | no | a list of `{url, thumb_url}`. Either may be left out. Links must be http or https |
| `per_side` | no | `true` when the numbers count for each side, as in 6 reps per side |
| `per_set` | no | read, but not stored yet. A start logs how many files use it |

Any other key stops the load and names its line, so a misspelt key is never dropped. This
holds for blocks and sessions too.

## A block

```yaml
# an example block, built from shipped exercises
id: 0199c3a0-0000-7000-8000-000000000001
name: Drills
items:
    - movement: silent_feet
      sets: 2
    - menu:
        name: Drills
        notes: One drill per session.
        pick: 1
        of: [soft_hands, down_climbing]
```

A block exists only in files, so that several sessions can share one warm-up. The loader
copies it into each session that uses it, as a section. Edit the block file, and every one of
those sessions changes on the next start.

| Key | Required | Holds |
|---|---|---|
| `id` | yes | a uuid. `make catalog-ids` writes it |
| `name` | yes | the section's name, up to 200 characters |
| `notes` | no | the section's notes |
| `items` | yes | at least one `movement:` or `menu:` |
| `tags`, `source`, `role` | no | read, but not stored. A start logs how many files use them |

A `movement:` item names an exercise. It can also set `sets`, `reps`, `set_rest_seconds`,
`rep_seconds`, `rep_rest_seconds`, `prep_seconds` or `seconds`, which replace the exercise's
own for this use.

A `menu:` item is a choice. It has a `name`, optional `notes`, a `pick` and `of`, a list of
exercises. `pick` is the fewest to do. `pick: 0` means that you can skip the whole choice. A
menu carries no numbers of its own.

## A session

```yaml
# catalog/sessions/boulder_session.yaml, shortened
id: bfbfa010-1020-4c47-85ce-38d8cfec9ae0
name: Boulder Session
color: '#ef4444'
needs: bouldering wall
items:
    - block: warm_up
    - block: drills
```

| Key | Required | Holds |
|---|---|---|
| `id` | yes | a uuid. `make catalog-ids` writes it |
| `name` | yes | up to 200 characters |
| `items` | yes | at least one `block:`. A session holds only blocks |
| `notes`, `source`, `needs` | no | text. `needs` is what the session needs to run |
| `color` | no | written `'#rrggbb'`. Quote it, or YAML reads it as a comment |
| `tags` | no | a list of words |

Each step in the session holds its own copy of the exercise, taken from the exercise's file.

## Names

A block names exercises, and a session names blocks, by file name.

- A bare name means the catalog of the file that holds it. So the exercises in a shipped block
  are shipped ones.
- `app:<name>` means the catalog the app ships. The shipped catalog itself never uses it.
- A name that no file holds stops the load and names the file. If the app ships that name,
  the error says to write `app:<name>`.

## The id

The loader matches a file to its row by the `id`, so renaming or moving a file keeps its row
and its history.

- Run `make catalog-ids` on a new file. For another tree, add `CATALOG_DIR=<tree>`. It fills
  in only a missing `id`, and never changes one.
- To start a new exercise from an old file, delete the `id:` line in the copy first. Two files
  with one id stop the load.
- Two people who load the same tree each get their own rows, so a copy of someone's tree is
  fine as it is.

## Loading

Every start loads the shipped catalog, then each private catalog in the config for each of its
owners. All of one owner's catalogs load together, so two of them cannot share a file name or
an id.

- Exercises load first, then sessions.
- A file that did not change is not written. A session counts as changed when a block or an
  exercise it copies changes.
- A deleted file retires its exercise or session. It leaves the library, and still works in
  anything that used it. If the file comes back, so does the row.
- One bad file stops the whole load and names every bad file, so a half-loaded catalog never
  happens.
- An owner with no account yet is skipped until the first start after they sign up.
- An owner removed from the config has their exercises and sessions retired.

An edit made in the app to an exercise or a session from a catalog stays until its file
changes. Then the file wins. Edit the file instead.

## What belongs in `catalog/`

`catalog/` is published under the repository's licence. Content from a paid programme belongs
in a private catalog. See the rules in `CLAUDE.md`.
