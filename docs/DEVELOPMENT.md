# Develop

## Set up

The repository has a nix flake with all the tools. The flake also sets `DATABASE_URL` and
`TEST_DATABASE_URL` to a local Postgres in `.pgdata`.

1. Run `nix develop`. If you use direnv, it loads the flake for you.
2. Run `make watch`. This starts Postgres, Vite on port 5173 and Go on port 8080.
3. Run `make seed`. This adds sixteen weeks of sample training.
4. Open <http://localhost:5173>.
5. Sign in as `dev@passion.local` with the password `passion-dev`.

The tests erase the test database. Do not set `TEST_DATABASE_URL` to a database with data that
you want to keep.

If a screen needs data that the seed does not have, add it to `server/cmd/seed/main.go`.

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
deploy/           the compose file that the deploy workflow uses
```

The server has four dependencies: pgx, goose, x/crypto and yaml. Do not add a fifth without
a strong reason.

## The data

The migrations in `server/db/migrations/` give the full schema. This is a summary:

- An exercise is one movement with its numbers, for example sets, reps and rest.
- A session template is a list of sections. Each step keeps its own copy of the numbers.
- A run is one session that you did. It keeps a copy of the template from its start.
- Sets and climbs are in their own tables.
- A cycle is a block of days that repeats between two dates.
- When you save a cycle, the server makes a `scheduled_session` row for each day.
- A centre is a place where you train.

Exercises and templates are not deleted. They are retired, because old runs point to them. A
run that is not finished does not count in any chart.

## Add a table

1. Add the next numbered file to `server/db/migrations/`.
2. If the table has an `updated_at` column, attach the `touch_updated_at()` trigger. Copy it
   from another table.

## Add a query

1. Put the query in `server/db/`, in the file for its table.
2. Read rows with `pgx.RowToStructByName`. Give each column a `db:` tag.
3. If a column can be null, use a pointer for its field.
4. Test the query with `dbtest.Pool(t)`. This gives a real, empty database.

## Add an endpoint

1. Write the handler as a method on `Server`.
2. Add it to `Routes` in `server/api/api.go`.
3. If the endpoint needs a signed-in user, wrap it in `s.authenticated`.
4. Read the request body with `decodeJSON`.
5. Reply with `writeJSON`, `writeFieldErrors` or `writeInternal`.
6. Add the `swagger:` comments, then run `make openapi`. CI fails if `swagger.json` is old.
7. Test the handler with `httptest` and `newTestServer(t)`.

## Add a screen

1. If the page needs a signed-in user, put it under `client/src/routes/(app)/`.
2. If the page has the tab bar, put it under `(app)/(tabs)/`.
3. Call the API with `request` from `lib/api.ts`.
4. To change a running session, use `openRun` from `lib/runState.svelte.ts`. Do not use
   `request`. `openRun` saves on the phone first.
5. Keep the filters of a list in the URL with `urlFilters`.

If a new class does not show under `make watch`, run `touch client/src/app.css`.

## Add to the catalog

1. Write the file. [CATALOG_FORMAT.md](CATALOG_FORMAT.md) gives the format.
2. Run `make catalog-ids`.
3. Restart the server.

Content from a paid programme goes in a private catalog. Do not put it in `catalog/`.

## Style a screen

The colour tokens are in `client/src/tokens.css`. Each token has a light value and a dark
value. A new colour needs both.

- Backgrounds: `bg-ground` for the page, `bg-surface` for cards, `bg-well` for fields.
- Text: `text-ink`, `text-ink-2` and `text-ink-3`. Lines use `border-line`.
- Lime (`bg-tint`) shows Start, Log and the current tab. Do not use lime for text.
- Apricot (`bg-live`) shows a running session. Dark green (`bg-hero`) is for the forest card.
- Errors use `text-bad`. Charts use `chart`.
- Type: `text-xs` for labels, `text-[15px]` for body text, `text-xl` for card titles.
- The page title uses `page-title`. Numbers use `digits`. Notes from the user use `reading`.
- Each field and `<select>` uses the `.input` class.

Use the parts in `client/src/lib/` before you make a new one:

- `Section` for a heading above a card.
- `Segmented` to change between views of the same data.
- `Tabs` to move between parts of a list.
- `LibraryFilters` and `FilterLine` to filter a list.
- `Button`, `Sheet`, `Menu`, `Icon`, `Checkbox` and `FormError`.

Layout rules:

- Each control is at least 44 px. If a control looks smaller, make its tap area larger.
- Rows in a list use the `list-row` class for the line between them. Do not use `border-t` or
  `divide-y`.
- A row that opens a different page ends with a chevron.
- A chart card starts with a label and one number. The chart comes after them.
- Draw charts with `Chart`, `Bars` and `Dots`.
- On a phone, the tabs are at the bottom. From `md` (768 px), they are in a rail on the left.
- From `lg` (1024 px), Today uses a wide layout. The other pages stay in one column.
- The pages of a running session are on `RunField`. They look the same in light and dark
  mode.

## Before you commit

1. Run `make test`.
2. Run `gofmt` and `go vet`.
3. If you changed a handler, run `make openapi`.

CI does all of these on each push.
