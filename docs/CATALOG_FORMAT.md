# Catalog format

A catalog is a directory of YAML files, one exercise per file. The app ships one in
`catalog/`. A private catalog uses the same format, and the config file says who owns it.

```
<tree>/
  movements/<slug>.yaml    one exercise per file
```

A directory inside `movements/` stops the load. Anything outside `movements/` is ignored for
now. `blocks/` and `sessions/` are read from Part 2 of [V2_DESIGN.md](V2_DESIGN.md) onwards.

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
| `per_side`, `per_set` | no | read, but not stored yet. A start logs how many files use them |

Any other key stops the load and names its line, so a misspelt key is never dropped.

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

- A file that did not change is not written.
- A deleted file retires its exercise. It leaves the library, and still works in anything that
  used it. If the file comes back, so does the exercise.
- One bad file stops the whole load and names every bad file, so a half-loaded catalog never
  happens.
- An owner with no account yet is skipped until the first start after they sign up.
- An owner removed from the config has their exercises retired.

An edit made in the app to an exercise from a catalog stays until its file changes. Then the
file wins. Edit the file instead.

## What belongs in `catalog/`

`catalog/` is published under the repository's licence. Content from a paid programme belongs
in a private catalog. See the rules in `CLAUDE.md`.
