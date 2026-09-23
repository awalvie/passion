# Developer guide

How to add to the V2 server. For setup and the make targets, see [readme.md](../readme.md).
For what to build, see [V2_DESIGN.md](V2_DESIGN.md).

## Where things go

- `server/db/` holds every SQL statement. It writes no JSON.
- `server/api/` holds the handlers. It writes no SQL.
- `server/db/migrations/` holds the schema, one goose file per change.

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

## Before you commit

- `make db-up`, then `make test`. Tests share one database, so they run one package at a time.
- `gofmt` and `go vet` must be clean. CI checks both.
- `make openapi`, if you touched a handler or a request or response struct.
