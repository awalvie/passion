# Catalog format

A catalog is a folder of YAML files. The app comes with one catalog in `catalog/`. A private
catalog has the same layout:

```
movements/   one exercise in each file (required)
blocks/      sections that sessions share
sessions/    one session in each file
```

A file name has only lower case letters, digits and underscores. Other files use the name
without `.yaml` to refer to the file.

If a file has a key that the loader does not know, the load stops. The error gives the line
number.

## Exercises

```yaml
id: 693a8297-678c-4c14-8ee9-9b3582f4f588
name: Repeaters 7:3 (20 mm Edge)
style: timed_reps
tags: [endurance, fingers, hangboard]
notes: Keep 1-2 reps in reserve on the last set.
sets: 4
reps: 6
rep_seconds: 7
rep_rest_seconds: 3
set_rest_seconds: 180
```

An exercise must have an `id`, a `name` and a `style`. The `style` is `reps_and_sets`,
`timed_reps`, `climbing` or `open`. It sets the screen that the exercise uses during a session.

These keys hold numbers: `sets`, `reps`, `seconds`, `rep_seconds`, `rep_rest_seconds`,
`set_rest_seconds` and `prep_seconds`.

If each rep of a `timed_reps` exercise has a different time, use `ladder: [3, 6, 9]`. Do not
use `rep_seconds` with a ladder.

If the numbers are for each side, set `per_side: true`.

The keys `tags`, `notes`, `source` and `media` are optional.

## Blocks

A block is a section that more than one session uses, for example a warm-up. When the
catalog loads, each session gets its own copy of the block.

```yaml
id: f8830077-4e8a-4088-9b6b-2a3da7d29538
name: Drills
items:
    - movement: silent_feet
      sets: 2
    - menu:
        name: Drills
        pick: 1
        of: [soft_hands, down_climbing]
```

A `movement` gives the name of an exercise. It can change the numbers of the exercise for
this block only.

A `menu` gives a choice of exercises. It must have `pick`, which is the number to do. If
`pick` is 0, the person can skip the menu.

## Sessions

```yaml
id: bfbfa010-1020-4c47-85ce-38d8cfec9ae0
name: Boulder Session
color: '#ef4444'
icon: mountain
items:
    - block: warm_up
    - block: drills
```

A session is a list of blocks. It can also have `notes`, `needs`, `source`, `tags`, `color`
and `icon`. Put quotes around the `color`. If you do not, YAML reads it as a comment.

## Ids and names

To give a new file its id, run `make catalog-ids`. Do not write an id yourself. If you copy
a file to make a new one, delete the `id` line first.

The id connects a file to its row in the database. Thus you can rename or move a file and
keep its history.

A name without a prefix refers to a file in the same catalog. In a private catalog,
`app:down_climbing` refers to the exercise that comes with the app.

If you delete a file, its exercise or session is retired. It goes out of the library, but the
sessions that use it continue to work.

If you edit a catalog item in the app, the edit stays until its file changes. Then the file
replaces the edit.

To examine a private catalog without the server, run
`go run ./server/cmd/catalogcheck <tree>...`.

## What can go in `catalog/`

The repository's licence covers `catalog/`, so all of it is public:

- Write notes in your own words.
- Do not copy the sessions of a paid programme.
- You can use sets, reps and rest times, because they are facts.

Put all other content in a private catalog.
