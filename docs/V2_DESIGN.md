# Passion V2 — design

What the app does and how its data is laid out. Read this before you design anything.

**Don't use the V1 database design.** Don't read `db/models.go` or `db/queries.go` on
`master`, and don't carry over GORM or SQLite shapes. If you think you need them, ask first
and wait.

**Do copy the V1 frontend.** This is a backend rewrite. V1's look, and the way its screens
work, are the target. See "The frontend follows V1".

---

## Start here

**What works.** Sign up, sign in on several devices, see your account, sign out. The Go binary
serves the Svelte client. API docs are at `/api/docs/`, generated from the handlers, and CI
fails if the spec goes stale. `make run` to try it, `make watch` to develop. There's nothing
about training yet.

**How it's built.** In five parts, in order. Each part holds everything needed to build it,
and the open decisions that come due there. We're on part 1.

1. Exercises
2. Session templates
3. Runs and the player
4. Cycles
5. History and metrics

**What's agreed.** The behaviour in every part, and in the schema only the exercise table.
Every other table below is a proposal nobody signed off. Don't write a migration from one.

**How to work here.** Agree a plan before each step, then write only that. One change per
commit. Never commit or push unless asked, right then. Don't widen what was asked for.

---

## What the app is

A self-hosted training app for climbers, built for about 1000 users, not one.

The loop, in the owner's words:

> create a cycle, run a scheduled session from a session template, or run an open session,
> log it, see metrics and history, repeat

Core: cycles, session templates, scheduled and open sessions, the player, logging, metrics
and history, and choices inside a session.

Later: planned against actual, adherence, hangboard load as a percent of bodyweight, a grade
pyramid.

---

## Part 1 — Exercises

### Steps

1. Migrate the exercise table.
2. Write the exercise queries and their tests: create, list, get, update, retire. A list
   holds the shipped rows and your own, and leaves out retired ones.
3. Add the API routes for the same.
4. Clean up the catalog files, as listed below, then load them.

### The exercise table

Agreed. A row holds what a catalog file holds, one column per field.

```sql
CREATE TABLE exercise (
    id                uuid        PRIMARY KEY DEFAULT uuidv7(),
    owner             uuid        NULL REFERENCES account (id) ON DELETE CASCADE,
    slug              text        NULL,

    name              text        NOT NULL,
    kind              text        NOT NULL
        CHECK (kind IN ('reps_and_sets', 'timed_reps', 'climbing', 'open')),
    notes             text,
    source            text,
    tags              text[]      NOT NULL DEFAULT '{}',

    sets              int,
    reps              int,
    set_rest_seconds  int,
    rep_seconds       int,
    rep_rest_seconds  int,
    prep_seconds      int,
    duration_seconds  int,

    video_url         text,
    thumbnail_url     text,

    retired_at        timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX exercise_shipped_slug ON exercise (slug)
    WHERE owner IS NULL AND slug IS NOT NULL;
CREATE UNIQUE INDEX exercise_owner_slug ON exercise (owner, slug)
    WHERE owner IS NOT NULL AND slug IS NOT NULL;
CREATE INDEX exercise_owner_idx ON exercise (owner);

CREATE TRIGGER exercise_touch BEFORE UPDATE ON exercise
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
```

- `owner NULL` means the app ships the row.
- `slug` is the key inside a catalog file: `bench_press` in `bench_press.yaml`. The loader uses
  it to find the row it wrote last time. An exercise typed into the app has none.
- Slug uniqueness takes two partial indexes, because Postgres treats NULLs as distinct. One
  `UNIQUE (owner, slug)` would let the shipped catalog hold the same slug twice.
- No unique index on `name`. Two unrelated exercises can share one (rule 8).
- `kind` picks the player screen. The numbers aren't tied to kind yet.
- The numbers are the protocol, the same for everyone. NULL means not set. Climbing has none,
  because each climb is logged on its own.
- No weight. What you lift belongs to the plan or the log, not the shared definition.
- One clip per exercise, because no catalog file has more.
- Exercises are retired with `retired_at`, never deleted.

Left for later, because nothing needs it yet:

- Your own version of a shipped exercise that keeps one history. An override table or a
  lineage column would both add to this table without changing it.
- A re-import that skips rows you edited by hand.
- Whether a number counts per side.

### Catalog cleanup

The catalog files are in V1's format. Before loading them:

- `kind: "session"` becomes `kind: "open"`, in 10 files.
- `label`, a comma-separated string, becomes a `tags` list.
- `session_duration_seconds` becomes `duration_seconds`, in `pulse_raiser.yaml`.
- `weight_kg` comes out of `weighted_pullups.yaml`.
- `media:` splits into `video_url` and `thumbnail_url`.
- About ten files sit in the wrong kind. For example, `traverse_circuit` is `climbing` but logs
  time, and `nelson/wall_crawl_static` is `climbing` but is timed holds.
- Per side is written three ways: only in the notes, in `reps`, or in `sets`. Pick one.
- The coach sources for the `ondra/`, `emil/` and `nelson/` files live only in the activity
  templates. Move them onto the exercises before the templates go.
- Check that `max_lifts_power` doesn't copy a paid programme's weekly structure.

### Open decisions

2. **One pull-up in the library, or several?** One gives one unbroken chart. Several matches
   other training apps. You can split later, but you can't merge.
6. **What does the library ship?** Answered: sets, reps and timings, and no weight.

---

## Part 2 — Session templates

### Steps

1. Agree the session template table. Decisions 3, 4, 7 and 8 come due here.
2. Rewrite the catalog's session templates to fit it. They `ref:` activity templates, which V2
   doesn't have.
3. Build the session template editor, with sections.

### What a session is

A session is a list of exercises. That's it.

**Sections.** A session splits into named parts: warm-up, drills, main, cooldown. A section
isn't a library item. It lives inside its session, and has a name, a position, and room for a
note or a target time.

**Choices.** A session can offer a choice: "pick two of these five". Not designed yet.

### Proposal, not agreed

A session is one thing, so it's one row with a `jsonb` body. Sections nest inside the body.
There's no table of sections and no table of steps.

```
session_template(id, owner, name, body jsonb)
```

```json
{
  "sections": [
    { "name": "Warm-up", "note": "go slow, it is cold",
      "steps": [
        { "exercise": "E3", "name": "Pulse raiser", "seconds": 300 },
        { "exercise": "E9", "name": "Shoulder circles", "sets": 2, "reps": 10 }
      ] },
    { "name": "Main",
      "steps": [ { "exercise": "E2", "name": "Limit boulders", "seconds": 3600 } ] }
  ]
}
```

### Open decisions

3. **Does a copied session template keep its identity?** If it does, your version and the
   shipped one share one "times completed" count.
4. **How does an improvement to a shipped exercise reach someone already using it?** Sessions
   hold copies, so nothing arrives by itself. A prompt, a manual sync, or nothing.
7. **Choices inside a session** are core and not designed yet. "Pick two of these five."
8. **Is `jsonb` the right home for sessions, cycles and runs?** The proposal assumes it. Plain
   tables for sections and steps are the other way. This decides the table count, so settle it
   together with decision 1.

---

## Part 3 — Runs and the player

### Steps

1. Agree the run table. Decisions 1 and 5 come due here.
2. Build the player: port the V1 behaviour below, and add swap, add, reorder and go back.

### What a run is

**During a run you can do four things**, whether the run came from a template or started
empty:

- swap an exercise for another
- add one that wasn't planned
- reorder what's left
- go back to a step you've finished

V1 only lets you skip. Closing that gap is the main job.

**A run never changes its template.** A template changes only when someone edits it on
purpose.

A run doesn't always have a template behind it. There are three cases:

- a planned session that was run
- an open session that started empty and grew
- a session written up later from memory, backdated

An unfinished write-up counts for nothing until it's finished.

### The V1 player: port it on purpose

V1's player is good and the owner wants it kept. These details were hard won:

- Timer state is mirrored to `localStorage`, because locking the phone reset the stopwatch.
- A two-tone sound and a double buzz when rest ends.
- Vibration even when the phone is muted, for shared gyms.
- A screen wake lock while a timer runs.
- The grade picker shows two grades either side of the last pick.
- Finishing early skips every step you never touched.

Five step kinds: picker, sets and reps, timer-driven reps, open timed block, climbing. Plus a
session timer, runs you can resume, notes per step, and an end-of-run journal for sleep,
energy and RPE.

### Proposal, not agreed

```
run(id, owner, template, date, started, finished, body jsonb)
```

The run body is the session template body plus results and timings, copied when the run
starts.

### Open decisions

1. **Where do logged results live?** In the run body, or in a flat `exercise_log` table with
   one row per set. What the exercise page shows decides it. A six-month chart of the heaviest
   set reads across every session, which the body can't do cheaply. The last five times can
   come straight from the body.
5. **Can a session run with numbers left empty?**

---

## Part 4 — Cycles

### Steps

1. Agree the cycle table.
2. Build cycles: create one, place sessions in its block, and schedule them.

### What a cycle is

A cycle has a total length and a block of N days that repeats. Six weeks with an eight-day
block is fine, so neither number is tied to a week.

Exercises can scale over a cycle. By default one setting covers the whole cycle. You can set a
week on its own, but a week carries a whole setting, not a patch on a few fields. Nothing
merges field by field, so nothing can half-override.

### Proposal, not agreed

```
cycle(id, owner, name, starts, weeks, body jsonb)
```

```json
{
  "block_days": 7,
  "days": [ { "day": 1, "template": "T1" }, { "day": 4, "template": "T2" } ],
  "scaling": [ { "exercise": "E1", "week": 1, "weight": 20 },
               { "exercise": "E1", "week": 3, "weight": 25 } ]
}
```

---

## Part 5 — History and metrics

### Steps

1. Agree what each history screen reads.
2. Build exercise history, then session history.

### What history shows

**Exercise history and session history are separate.** Exercise history shows progress on
one exercise and nothing else. Session history is where planned against actual goes later.

Every metric comes from records. None reads the catalog or the plan.

**Keep charts minimal.** Show only what matters at first. Don't ship the row of four or five
metric toggles that Hevy and Strong have.

| Exercise kind | What to show |
|---|---|
| Sets, reps and weight | Heaviest weight per session, labelled with the set: "70 kg × 5" |
| Hangboard | Signed added weight |
| Open timed block | No chart. A personal best and a list |
| Climbing | No chart. One number: highest grade sent in the last 90 days |

Estimated 1RM is out because it drifts at 5 to 10 reps. Total volume is out because it
rewards junk reps.

---

## Applies everywhere

### Rules

- **Copy, don't reference.** Dragging an exercise into a session copies its name and numbers.
  Nothing in a session reads the library live.
- **Identity travels with the copy.** Every copy carries a value that means "same movement as
  that one". It's never regenerated, and charts group on it.
- **Only the owner changes a record.** A record freezes what it shows when it's written.
- **Retire, never delete.** A library row that leaves drops out of the picker and keeps working
  wherever it's already used.
- **Deletes follow ownership only.** Account to its rows, parent to children. Nothing in the
  library or the plan can reach into a record.

### Units, grades, time and the body

From research into how Crimpd, Hevy, MacroFactor, Lattice, Kilter and Strava model a person.
Full notes in `.claude/agent-memory/scout/project_person_model_v2.md`.

- **Store one unit.** Kilograms, named in the column. Convert on the way in and on the way out.
  Never a unit column per row: it breaks every sort and every total.
- **Hangboard load is signed.** A band-assisted hang is negative added weight, and that's where
  most people start.
- **Snapshot whatever a past number was worked out from.** Bodyweight goes on the hang row.
  Otherwise weighing yourself today quietly rewrites last year's percentages. TrainingPeaks and
  Garmin users complain about exactly this when they change FTP.
- **Never store a best as a profile column.** A best is a query over the log, and it depends on
  edge, grip, arm, angle and bodyweight at the time.
- **Store date of birth, not age.** Store span and height, and work out the ape index.
- **Grades take two settings, not one.** Boulders and routes use different scales, and Font
  boulders with French routes is normal. Store the grade in the system it was logged in, plus a
  number for sorting. Converting loses detail, so never overwrite what was stored. A board
  grade means nothing without the angle.
- **Timezone is an IANA name on the account, never an offset.** On every event store the UTC
  time, the zone, and the local date. The local date is part of the record, not a display of
  it. Otherwise a session logged abroad moves a day when you get home, and breaks a streak.
- **The profile is almost all optional.** You must be able to log a session with nothing but
  an account. Don't make people fill in a profile first.

---

## Reference

### The code today

Auth is done, tested and pushed. It signs you up, signs you in on several devices, tells you
who you are, and signs one device out.

```
server/api/        handlers, routing, middleware, the openapi spec and the docs page
server/db/         pgx queries, goose migrations, the test helper
server/password/   argon2id hashing
server/token/      opaque bearer tokens
server/web/        serves the embedded client
server/cmd/passion entry point
client/            sveltekit, built to server/web/dist and embedded in the binary
```

Seven routes, all in `server/api/api.go`:

```
GET    /healthz
GET    /api/openapi.json
GET    /api/docs/
POST   /api/v1/accounts          sign up
POST   /api/v1/tokens            sign in
GET    /api/v1/accounts/me       who am i
DELETE /api/v1/tokens/current    sign out
```

Two tables, in `server/db/migrations/00001_init.sql`: `account` and `auth_token`. Primary keys
are `uuid DEFAULT uuidv7()`. A `touch_updated_at` trigger keeps `updated_at` right, because
without an ORM no one place in Go sees every UPDATE.

The whole stack: Go 1.26, `net/http` with no router, Postgres 18, `pgx/v5` with no ORM,
`goose` for migrations, `golang.org/x/crypto` for argon2id. Three direct dependencies. The
client is SvelteKit with `adapter-static`, embedded in the binary with `//go:embed`. API docs
use Scalar, vendored so an offline install still works.

Stdlib first. A dependency has to earn its place. `docs/DEVELOPMENT.md` shows how to add a
table, a query and an endpoint.

### How to run it

`direnv` loads the nix flake. It brings go, nodejs, pnpm, postgresql 18, air and go-swagger,
and sets `DATABASE_URL` and `TEST_DATABASE_URL` to a cluster in `.pgdata`.

| Command | What it does |
|---|---|
| `make run` | Build the client, start postgres, serve on http://localhost:8080 |
| `make watch` | Vite on http://localhost:5173 proxying `/api` to the go server. Both halves reload |
| `make test` | `go test ./... -count=1 -p 1`. Start the database first with `make db-up` |
| `make openapi` | Regenerate `server/api/swagger.json` from the handler annotations |
| `make db-up` / `make db-down` | Start or stop the local postgres cluster |
| `make image` | Build the docker image |

The binary migrates itself when it starts, so upgrading is one command.

CI checks gofmt, `go vet`, a stale OpenAPI spec, and the tests. Green on 2026-09-22.

### The frontend follows V1

This is a backend rewrite. Nobody asked for a new look, and people like V1's. A new screen
should feel like the same app. Its markup doesn't have to match.

Already carried over:

- `client/src/passion.css` is `static/passion.css`, byte for byte: 4,397 lines, about 82 KB
  built. All the tokens and the `card`, `input`, `btn` and `muted` classes come with it.
- The sign-in screen is a port of `templates/login.html`, with the same classes.

Not carried over, and it mustn't be: the HTML templates, HTMX, and anything about how V1
talked to its server. Screens are rebuilt as Svelte components on the JSON API.

For the look and how a screen behaves, read:

- `docs/DESIGN.md`: colour tokens, type, cards, badges, icons and layout. Follow it instead of
  inventing.
- `templates/`: how each screen was laid out.
- `docs/DESIGN_UX_AUDIT.md`, `docs/ux-review-followups.md`, `docs/IMPLEMENTATION_PROMPT.md`
  and `docs/plans/`: audits and fixes already done once. Don't repeat the mistakes they record.

Left out on purpose for now: the Inter webfont isn't vendored, so the client falls back to
the system sans-serif.

### Documents that still describe V1

These are out of date and will mislead you. Don't take a data shape from any of them.

- **`docs/REQUIREMENTS.md`** is useful, but it isn't a spec. See the next section.
- **`CLAUDE.md`**: its HTMX and `<select>` rules are V1, and it lists a `schema` agent that no
  longer exists.
- **`.claude/agents/qa.md`** writes Go tests against SQLite in a temp folder.
- **`catalog/`** is 102 YAML files in V1's three-level shape, where a session template has
  `activities:` that `ref:` an activity template. Part 1 cleans up the exercises, and part 2
  rewrites the session templates.

### How to read `docs/REQUIREMENTS.md`

It's worth reading, but **it isn't gospel**. It was written before the rewrite, it states one
person's habits as if they were rules, and nobody has checked it against this design. Where
the two disagree, this file wins. Raise the disagreement instead of settling it alone.

It has no plan and no order of work. Its five parts are a six-week walkthrough, what people do
with the app, numbered rules a design must keep, what the app doesn't do today, and where it's
going. The order of work lives in the five parts of this file, and nowhere else.

The code cites its rule numbers, for example "Rule 51" in `server/cmd/passion/main.go`. Never
renumber them.

Three corrections went in on 2026-09-22, and more are likely:

- It said "SQLite by default and PostgreSQL optionally". It's Postgres only now.
- It made activities a level of the catalog, with a session as a group of activities. That's
  reversed: a section belongs to its session.
- The catalog counts in its own part 1 were V1's.
