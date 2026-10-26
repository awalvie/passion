# Develop

## Set up

The repository has a nix flake with all the tools. The flake also sets `DATABASE_URL` and
`TEST_DATABASE_URL` to a local Postgres in `.pgdata`.

1. Run `nix develop`.
2. Run `make watch`. This starts Postgres, Vite on port 5173 and Go on port 8080.
3. Run `make seed`. This adds sixteen weeks of sample training.
4. Open <http://localhost:5173>.
5. Sign in as `dev@passion.local` with the password `passion-dev`.

The tests erase the test database. Do not set `TEST_DATABASE_URL` to a database with data that
you want to keep.

| Target | What it does |
|---|---|
| `make run` | Starts Postgres, builds the client and serves on port 8080 |
| `make watch` | Runs the client and the server, and reloads them when you edit |
| `make seed` | Adds sample training. `SEED_FLAGS=-reset` starts again |
| `make test` | Runs the Go tests and the client tests |
| `make openapi` | Makes `server/api/swagger.json` again |
| `make db-up`, `make db-down` | Starts or stops Postgres |
| `make client` | Builds the client into `server/web/dist` |
| `make image` | Builds the Docker image |
| `make catalog-ids` | Gives an id to new catalog files. Set `CATALOG_DIR=<tree>` for a different tree |

## Where things go

```
server/api/       HTTP handlers. No SQL here
server/db/        all the SQL, and the migrations
server/catalog/   loads the catalog files at start
server/cmd/       passion, seed, catalogid and catalogcheck
client/           the SvelteKit app
catalog/          the exercises and sessions that come with the app
deploy/           the systemd unit, the release Dockerfile, and the maintainer's compose file
```

## Release

Push a tag such as `v0.1.0`. The release workflow builds the binaries and the image with
goreleaser, and publishes them. A tag such as `v0.2.0-rc1` makes a pre-release.

`.github/workflows/deploy.yml` deploys the maintainer's server. Its top lists what it needs.
