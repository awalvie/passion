<p align="center">
  <strong>Passion</strong><br>
  <sub>A self-hosted climbing training app. Go and Postgres on the back, Svelte on the front.</sub>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.26">
  <img src="https://img.shields.io/badge/PostgreSQL-18-4169E1?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL 18">
  <img src="https://img.shields.io/badge/Svelte-5-FF3E00?style=flat-square&logo=svelte&logoColor=white" alt="Svelte 5">
  <img src="https://img.shields.io/badge/Tailwind_CSS-4-06B6D4?style=flat-square&logo=tailwindcss&logoColor=white" alt="Tailwind CSS 4">
</p>

---

Passion is being rewritten as a JSON API with a separate client, so that a web app and a
mobile app can both talk to the same server. [docs/V2_DESIGN.md](docs/V2_DESIGN.md) says
what the app will do and how the data is arranged. This file says what works today.

## What works today

Accounts, authentication, an exercise library, session templates and runs, through the API.
The client is a phone app with three tabs. Today shows the day's planned sessions and starts
one, or an open session. Library holds the exercises and the session templates. History
lists finished sessions and what each one logged. While a session runs, the client logs sets,
per-side sets, climbs, and timed reps with prep, hang and rest. It picks from a choice, adds
an exercise, and finishes with the journal. Writes wait on the phone when there is no signal.
The look is a placeholder.

- Sign up with an email address and a password, hashed with argon2id.
- Sign in. Each sign-in mints its own bearer token, so a phone and a laptop hold different
  ones and signing out of either leaves the other working.
- A token lasts thirty days by default and slides forward when it is used.
- Read your own account.
- Sign out one device.
- List your exercise library, and create, edit and retire exercises of your own.
- The shipped exercises load at startup, and so does any private catalog in the config. A
  start writes only the files that changed. If you edit an exercise from your catalog in the
  app, the edit stays until its file changes, and then the file wins. Edit the file instead.
- List session templates, and create, edit and retire your own. A session holds named
  sections of exercises and choices such as "pick 1 of these 3". Each step keeps its own copy
  of the exercise, so you can change its numbers for that session only.
- The shipped session templates and any private ones load at startup too, from `blocks/` and
  `sessions/` beside `movements/`. See [docs/CATALOG_FORMAT.md](docs/CATALOG_FORMAT.md).
- Start a run from a session template, or an open run that starts empty. The run copies the
  template when it starts, so a later edit to the template never reaches it. A write-up of a
  day already gone names that day.
- Change anything in a run while you train or long after: its steps, their order, a pick from
  a choice, notes and the end-of-run journal. Finish it, and every step not reached counts as
  skipped.
- Log a step's sets, and a climbing step's climbs one at a time. A retried write counts once.
  A climb is a send when it is graded and was an onsight, flash or redpoint.
- Grades come in Font and V for boulders, French and YDS for routes. Your account says which
  the client offers first.
- Plan a cycle: a block of days that repeats between two dates, with sessions on some of its
  days. The app places each session on the dates it falls, from today on. Move one day, take
  one out, or schedule a one-off by hand. A day gone by with no run shows as missed.
- Start a run from a scheduled day, and the day counts as done once the run is finished.
- Read an exercise's history: every finished run that logged it, with its sets or climbs.

The Go binary serves everything: the API, the Svelte client, and browsable API
documentation. It migrates its own database on the way up, so an upgrade never needs a
second command.

## Quick start

The repository ships a nix flake with every tool it needs. With `direnv`, `cd` into the
directory and the shell is ready. Without it, run `nix develop`.

```sh
make run     # build the client, start postgres, serve on http://localhost:8080
```

Then open <http://localhost:8080/login> to sign up.

For development, `make watch` reloads both halves. Vite serves the client on
<http://localhost:5173> and proxies `/api` to the Go server. Open that one, not 8080.

## On an iPhone

1. Serve Passion over HTTPS. Over plain http the app works, but the screen dims during a
   timer, because Safari keeps the wake lock for secure pages.
2. Open the address in Safari, tap Share, then Add to Home Screen.
3. Open Passion from the Home Screen and sign in there. The Home Screen app keeps its own
   storage, so a sign-in in Safari does not carry over.

Timer tones play once you tap Start. Turn them off in Settings, from the gear on Today.

## API

Interactive documentation lives at `/api/docs/` on a running server. It is generated from
the handlers and the OpenAPI document is at `/api/openapi.json`.

| Method | Path | What it does |
|---|---|---|
| `POST` | `/api/v1/accounts` | Sign up. Returns the account and its first token |
| `POST` | `/api/v1/tokens` | Sign in. Returns a new token |
| `GET` | `/api/v1/accounts/me` | The signed-in account |
| `PUT` | `/api/v1/accounts/me/grades` | Choose the grade scales the client offers first |
| `DELETE` | `/api/v1/tokens/current` | Sign out this device |
| `GET` | `/api/v1/exercises` | Your library: shipped exercises and your own, not retired |
| `POST` | `/api/v1/exercises` | Create an exercise of your own |
| `GET` | `/api/v1/exercises/{id}` | One exercise, retired ones included |
| `PUT` | `/api/v1/exercises/{id}` | Replace every field of one of your own |
| `POST` | `/api/v1/exercises/{id}/retire` | Take one of your own out of your library |
| `GET` | `/api/v1/exercises/{id}/history` | Every finished run that logged it, newest first |
| `GET` | `/api/v1/session-templates` | Shipped session templates and your own, not retired |
| `POST` | `/api/v1/session-templates` | Create a session template of your own |
| `GET` | `/api/v1/session-templates/{id}` | One session template, retired ones included |
| `PUT` | `/api/v1/session-templates/{id}` | Replace every field of one of your own, sections included |
| `POST` | `/api/v1/session-templates/{id}/retire` | Take one of your own out of your list |
| `GET` | `/api/v1/runs` | Your runs, newest day first |
| `POST` | `/api/v1/runs` | Start a run from a template, or an open run |
| `GET` | `/api/v1/runs/{id}` | One run with its sets and climbs |
| `PUT` | `/api/v1/runs/{id}` | Replace everything but the plan, the template and the finish |
| `POST` | `/api/v1/runs/{id}/finish` | Finish a run. Steps not reached count as skipped |
| `DELETE` | `/api/v1/runs/{id}` | Delete one of your runs with everything in it |
| `PUT` | `/api/v1/runs/{id}/steps/{step}/sets` | Replace one step's sets |
| `PUT` | `/api/v1/runs/{id}/climbs/{climb}` | Write one climb, under an id the client chose |
| `DELETE` | `/api/v1/runs/{id}/climbs/{climb}` | Remove one climb |
| `GET` | `/api/v1/grades` | Every grade scale, easiest grade first |
| `GET` | `/api/v1/cycles` | Your cycles, latest start first |
| `GET` | `/api/v1/cycles/{id}` | One cycle |
| `PUT` | `/api/v1/cycles/{id}` | Create or replace a cycle, under an id the client chose |
| `DELETE` | `/api/v1/cycles/{id}` | Delete a cycle and the days it placed. Runs stay |
| `GET` | `/api/v1/scheduled-sessions?from=&to=` | Your calendar, with each day's status |
| `POST` | `/api/v1/scheduled-sessions` | Schedule a one-off session |
| `PUT` | `/api/v1/scheduled-sessions/{id}` | Move a session to another day |
| `DELETE` | `/api/v1/scheduled-sessions/{id}` | Take a session off its day |
| `GET` | `/healthz` | Liveness, and whether the database answers |

Authenticate with `Authorization: Bearer <token>`.

Every failure answers in one shape: an error object with a code, a human message, and on a
validation failure a `fields` map that names what to fix.

## Configuration

Settings come from an optional YAML file, read at startup. Set `PASSION_CONFIG` to its
path. [passion.example.yaml](passion.example.yaml) lists every key with its default. A
misspelt key or a bad value stops the server and names the line.

| Key | Default | Description |
|---|---|---|
| `server.addr` | `:8080` | Listen address |
| `database.url` | none, required | Postgres connection string. The server exits without it |
| `auth.token_life` | `720h` | How long a sign-in lasts unused. Must be longer than `24h` |
| `log.level` | `info` | `debug`, `info`, `warn` or `error` |
| `log.format` | `text` | `text` or `json` |
| `catalog.private` | none | Catalog trees of your own, each a `location` and a list of `owner` emails. An owner with no account yet is skipped until the next start |

Two environment variables win over the file, so a deploy can set them and need no file.

| Variable | Sets |
|---|---|
| `DATABASE_URL` | `database.url` |
| `PASSION_ADDR` | `server.addr` |

The nix shell sets `DATABASE_URL` and `TEST_DATABASE_URL` to a cluster in `.pgdata`, so
local development needs no configuration at all.

## Make targets

| Target | What it does |
|---|---|
| `make run` | Build the client and serve on :8080 |
| `make watch` | Hot reload. Vite on :5173, air rebuilding the server |
| `make test` | Run the Go tests and the client's unit tests. Start the database first with `make db-up` |
| `make openapi` | Regenerate `server/api/swagger.json` from the handler annotations |
| `make db-up` / `make db-down` | Start or stop the local postgres cluster |
| `make image` | Build the docker image |
| `make catalog-ids` | Give each file in `catalog/` that has no `id:` one of its own. `CATALOG_DIR=<tree>` for another tree |

CI checks formatting, `go vet`, that the OpenAPI document is not stale, and the tests.

## Project structure

```
server/api/         handlers, routing, middleware, the openapi spec and the docs page
server/catalog/     catalog trees. Gives each file the id the loader will match it on
server/config/      the YAML config file and the environment variables that override it
server/db/          pgx queries and goose migrations
server/grades/      the grade scales a climb is logged in
server/password/    argon2id hashing
server/token/       opaque bearer tokens
server/web/         serves the client, embedded with //go:embed
server/cmd/         passion, the server, and catalogid, which gives catalog files an id
client/             sveltekit, built into server/web/dist
catalog/            the shipped catalog, embedded and loaded at startup. See docs/CATALOG_FORMAT.md
docs/               design and requirements
```

The stack is deliberately small: `net/http` with no router library, `pgx` with no ORM,
`goose` for migrations, `golang.org/x/crypto` for argon2id, and `go.yaml.in/yaml/v3` for
the config file. Four direct dependencies.

## Docker

```sh
make image
docker run -e DATABASE_URL=... -p 8080:8080 passion:dev
```

The image is a static binary on Alpine, running as an unprivileged user.

## Licence

[PolyForm Noncommercial 1.0.0](LICENSE).
