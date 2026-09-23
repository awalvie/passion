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

Accounts, authentication and an exercise library, through the API. The client has only the
sign-in screen so far.

- Sign up with an email address and a password, hashed with argon2id.
- Sign in. Each sign-in mints its own bearer token, so a phone and a laptop hold different
  ones and signing out of either leaves the other working.
- A token lasts thirty days and slides forward when it is used.
- Read your own account.
- Sign out one device.
- List your exercise library, and create, edit and retire exercises of your own. The shipped
  catalog isn't loaded yet.

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

## API

Interactive documentation lives at `/api/docs/` on a running server. It is generated from
the handlers and the OpenAPI document is at `/api/openapi.json`.

| Method | Path | What it does |
|---|---|---|
| `POST` | `/api/v1/accounts` | Sign up. Returns the account and its first token |
| `POST` | `/api/v1/tokens` | Sign in. Returns a new token |
| `GET` | `/api/v1/accounts/me` | The signed-in account |
| `DELETE` | `/api/v1/tokens/current` | Sign out this device |
| `GET` | `/api/v1/exercises` | Your library: shipped exercises and your own, not retired |
| `POST` | `/api/v1/exercises` | Create an exercise of your own |
| `GET` | `/api/v1/exercises/{id}` | One exercise, retired ones included |
| `PUT` | `/api/v1/exercises/{id}` | Replace every field of one of your own |
| `POST` | `/api/v1/exercises/{id}/retire` | Take one of your own out of your library |
| `GET` | `/healthz` | Liveness, and whether the database answers |

Authenticate with `Authorization: Bearer <token>`.

Every failure answers in one shape: an error object with a code, a human message, and on a
validation failure a `fields` map that names what to fix.

## Configuration

Two environment variables, both read at startup.

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | none, required | Postgres connection string. The server exits without it |
| `PASSION_ADDR` | `:8080` | Listen address |

The nix shell sets `DATABASE_URL` and `TEST_DATABASE_URL` to a cluster in `.pgdata`, so
local development needs no configuration at all.

## Make targets

| Target | What it does |
|---|---|
| `make run` | Build the client and serve on :8080 |
| `make watch` | Hot reload. Vite on :5173, air rebuilding the server |
| `make test` | Run the tests. Start the database first with `make db-up` |
| `make openapi` | Regenerate `server/api/swagger.json` from the handler annotations |
| `make db-up` / `make db-down` | Start or stop the local postgres cluster |
| `make image` | Build the docker image |

CI checks formatting, `go vet`, that the OpenAPI document is not stale, and the tests.

## Project structure

```
server/api/         handlers, routing, middleware, the openapi spec and the docs page
server/db/          pgx queries and goose migrations
server/password/    argon2id hashing
server/token/       opaque bearer tokens
server/web/         serves the client, embedded with //go:embed
server/cmd/passion/ entry point
client/             sveltekit, built into server/web/dist
catalog/            exercise and session YAML. Not imported yet
docs/               design and requirements
```

The stack is deliberately small: `net/http` with no router library, `pgx` with no ORM,
`goose` for migrations, and `golang.org/x/crypto` for argon2id. Three direct dependencies.

## Docker

```sh
make image
docker run -e DATABASE_URL=... -p 8080:8080 passion:dev
```

The image is a static binary on Alpine, running as an unprivileged user.

## Licence

[PolyForm Noncommercial 1.0.0](LICENSE).
