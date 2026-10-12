// Package db owns the connection pool and every SQL statement the app runs.
package db

import (
	"context"
	"errors"
	"fmt"

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
