// Package db owns the connection pool and every SQL statement the app runs.
package db

import (
	"context"
	"fmt"

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

// checkVersion reads the server_version_num form, where 18.6 is 180006.
func checkVersion(num int) error {
	if major := num / 10000; major < minMajorVersion {
		return fmt.Errorf("postgres %d is too old, the schema needs %d or newer", major, minMajorVersion)
	}
	return nil
}
