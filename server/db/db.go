// Package db owns the connection pool and every SQL statement the app runs.
package db

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The schema defaults primary keys to uuidv7(), which Postgres added in 18.
const minMajorVersion = 18

// Open connects and refuses a server too old for the schema.
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	var num int
	err = pool.QueryRow(ctx, "SELECT current_setting('server_version_num')::int").Scan(&num)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("read the postgres version: %w", err)
	}

	if err := checkVersion(num); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}

// cleanID trims and lower-cases an id, and says whether it is a uuid.
func cleanID(id string) (string, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	return id, uuidText.MatchString(id)
}

// missing looks up ids, by path, in table, which is always a constant, and
// names each one the person cannot see with msg. Shipped rows and their own
// count, retired ones included, as a step keeps working on a retired row.
func missing(ctx context.Context, q querier, table, owner string, byPath map[string]string, msg string) (map[string]string, error) {
	if len(byPath) == 0 {
		return nil, nil
	}
	rows, err := q.Query(ctx, `SELECT id::text FROM `+table+` WHERE id = ANY($1::uuid[]) AND (owner IS NULL OR owner = $2)`,
		slices.Collect(maps.Values(byPath)), owner)
	if err != nil {
		return nil, fmt.Errorf("select %s ids: %w", table, err)
	}
	found, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("read %s ids: %w", table, err)
	}
	problems := map[string]string{}
	for path, id := range byPath {
		if !slices.Contains(found, id) {
			problems[path] = msg
		}
	}
	return problems, nil
}

// oneRow reads the one row a query returns. No row, or an id that is not a
// uuid, is notFound.
func oneRow[T any](rows pgx.Rows, notFound error, what string) (T, error) {
	var zero T
	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[T])
	if errors.Is(err, pgx.ErrNoRows) || isInvalidText(err) {
		return zero, notFound
	}
	if err != nil {
		return zero, fmt.Errorf("read %s: %w", what, err)
	}
	return row, nil
}

// execOne runs a statement that changes one row. No such row, or an id that
// is not a uuid, is notFound.
func execOne(ctx context.Context, pool *pgxpool.Pool, notFound error, what, sql string, args ...any) error {
	tag, err := pool.Exec(ctx, sql, args...)
	if isInvalidText(err) {
		return notFound
	}
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if tag.RowsAffected() == 0 {
		return notFound
	}
	return nil
}

// checkVersion reads the server_version_num form, where 18.6 is 180006.
func checkVersion(num int) error {
	if major := num / 10000; major < minMajorVersion {
		return fmt.Errorf("postgres %d is too old, the schema needs %d or newer", major, minMajorVersion)
	}
	return nil
}
