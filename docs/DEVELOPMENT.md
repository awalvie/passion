# Developer guide

How to add to the V2 server. For setup and the make targets, see [readme.md](../readme.md).
For what to build, see [V2_DESIGN.md](V2_DESIGN.md).

## Where things go

- `server/db/` holds every SQL statement. It writes no JSON.
- `server/api/` holds the handlers. It writes no SQL.
- `server/db/migrations/` holds the schema, one goose file per change.
- `server/catalog/` reads catalog files and loads them through `db`. It writes no SQL.
- `server/config/` reads the config file.

## Add to the catalog

1. Write `catalog/movements/<slug>.yaml` for an exercise, `catalog/blocks/<slug>.yaml` for a
   block, or `catalog/sessions/<slug>.yaml` for a session. The keys are in
   [CATALOG_FORMAT.md](CATALOG_FORMAT.md).
2. Run `make catalog-ids` to give it an id.
3. Run `make test`. `TestShippedCatalogReads` fails on a bad file, before any server does.

## Add a table

1. Add the next numbered file to `server/db/migrations/`, for example `00002_exercise.sql`,
   with `-- +goose Up` and `-- +goose Down` sections.
2. If the table has `updated_at`, attach the trigger that already exists:

   ```sql
   CREATE TRIGGER exercise_touch BEFORE UPDATE ON exercise
       FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
   ```

3. That's it. The binary migrates itself when it starts, and the test helper empties every
   table on its own, so nothing else needs to know about the new table.

## Add a query

Put it in `server/db/`, one file per table, like `account.go`.

- Read rows with `RETURNING *` or `SELECT *` and `pgx.RowToStructByName`. The struct needs a
  `db:` tag for every column, so a new column means a new field in the same commit.
- A nullable column scans into a pointer, like `*int`.
- Give each expected failure a named error, like `ErrNoAccount`, and wrap everything else with
  `fmt.Errorf("what failed: %w", err)`.

Test it in the same package with `dbtest.Pool(t)`. That gives you a migrated, empty database.
See `account_test.go`.

## Add an endpoint

1. Write the handler as a method on `Server`. If it needs a signed-in person, give it the
   `authenticatedHandler` signature so it receives `db.Authenticated`.
2. Register it in `Routes` in `server/api/api.go`, wrapped in `s.authenticated(...)` if it
   needs sign-in.
3. Read the body with `decodeJSON`. It rejects unknown fields, so a typo in a field name is an
   error.
4. Collect validation problems in a `map[string]string` and send them with
   `writeFieldErrors`. Send success with `writeJSON`, and anything unexpected with
   `writeInternal`, which logs the reason and keeps it out of the response.
5. Document it for OpenAPI:
   - a `swagger:route` comment on the handler, listing its responses
   - `swagger:model` on the request and response structs, with `example:` and `required:` on
     each field
   - a `swagger:parameters` wrapper for the body and a `swagger:response` wrapper for each
     response, in `server/api/openapi.go`
6. Run `make openapi` and commit `server/api/swagger.json`. CI fails if it's stale.

Test handlers with `httptest` against `newTestServer(t)`, from `auth_test.go`. See
`api_test.go`.

## Add a screen

1. Put a page that needs sign-in under `client/src/routes/(app)/`. Its layout sends a
   signed-out visitor to `/login`. A page with the tab bar goes in `(app)/(tabs)/`, and a
   Library page in `(app)/(tabs)/(library)/`. None of these groups change the URL.
2. Call the API with `request` from `client/src/lib/api.ts`. A failure throws
   `RequestFailed`, and `describe` turns it into one line for a form.
3. A change to a run goes through `openRun` in `client/src/lib/runState.svelte.ts`, not
   `request`. It changes the run on the phone first and queues the write, so a gym with no
   signal loses nothing.
4. For filters a list keeps in the URL, use `urlFilters` from `client/src/lib/filters.svelte.ts`.
5. Style it with the token utilities. See the next section.
6. Run `pnpm --dir client check`.

Under `make watch`, a class used for the first time in a new file can be missing from the dev
stylesheet. Run `touch client/src/app.css` and Vite rebuilds it. The production build is not
affected.

## Style a screen

The look comes from `passion-design/final/designs/direction`. Every screen, the Library too,
uses only its tokens. The classes in [DESIGN.md](DESIGN.md) are V1's, and V2 screens do not
use them.

- `client/src/tokens.css` holds the tokens, light under `:root` and dark under
  `:root[data-theme='dark']`. `app.html` always sets `data-theme`, so a token needs no media
  query. A new colour goes in both blocks.
- `client/src/app.css` maps them into Tailwind in `@theme inline`. A new token needs a line
  there before it has a utility.
- Colours: `bg-ground` for the page, `bg-surface` for cards, `bg-well` for fields and
  wells, `text-ink`, `text-ink-2` and `text-ink-3` for text, `border-line` for hairlines.
  `bg-tint` with `text-on-tint` is lime, for Start, Log and the active tab. `bg-live` with
  `text-on-live` is apricot, for a session that is running. `bg-hero` with `text-on-hero`
  and `text-on-hero-2` is the forest card. `prep`, `hang` and `rest` colour the timer phases.
  `text-bad` is for errors.
- `text-tint` is not for text. Lime has too little contrast on a light page. Use
  `text-link` for a link or a text button. `text-tint` is only for an icon or a mark on a
  dark ground, such as the forest card.
- Shadows: `shadow-card` for a card, `shadow-card-sm` for a pill or a small button,
  `shadow-tint` and `shadow-live` under a lime or an apricot button.
- Type: `text-xs` (12 px) for labels, `text-[15px]` for body text, `text-xl` (20 px) for
  card titles and big buttons, `text-[32px]` for the page title. Use weights 600 to 800.
  `font-sans` asks for Figtree and falls back to the system font, because the font is not
  self-hosted yet.
- `dark:` follows `data-theme`, not the system setting. Use it only when a token cannot
  say it, for example a shadow that differs by theme.
- Use `Button`, `NavBar`, `Menu`, `TabBar` and `FormError` from `client/src/lib/`, and the
  `.input` class for every field and `<select>`.

## Before you commit

- `make db-up`, then `make test`. Tests share one database, so they run one package at a time.
  `make test` also runs the client's `*.test.ts` files with `node --test`. They cover pure
  modules only, so they need no browser and no extra packages.
- `gofmt` and `go vet` must be clean. CI checks both.
- `make openapi`, if you touched a handler or a request or response struct.
