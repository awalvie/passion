// Package dbtest hands a test a migrated, empty database.
//
// Tests share one database and run serially, truncating between them. Copying
// a template database per test is faster and brings real failure modes with
// it, and this suite is nowhere near slow enough to want them.
package dbtest

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/db"
)

// DSN returns the test connection string, or fails the test.
//
// Failing beats skipping. A skipped database test is one nobody notices has
// stopped running.
func DSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL is not set: run inside the nix shell, after `make db-up`")
	}
	return dsn
}

// Pool opens the test database, migrates it, and empties it. Migrating every
// time costs a millisecond once there is nothing to apply, and it means no
// test can leave the schema behind for the next one.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := DSN(t)
	ctx := context.Background()

	if err := db.Migrate(ctx, dsn); err != nil {
		t.Fatalf("migrate the test database: %v", err)
	}

	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open the test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := truncate(ctx, pool); err != nil {
		t.Fatalf("empty the test database: %v", err)
	}
	return pool
}

// truncate reads the table list rather than naming tables, so a new migration
// needs no change here.
func truncate(ctx context.Context, pool *pgxpool.Pool) error {
	var list *string
	err := pool.QueryRow(ctx, `
		SELECT string_agg(quote_ident(tablename), ', ')
		FROM pg_tables
		WHERE schemaname = current_schema()
		  AND tablename <> 'goose_db_version'`).Scan(&list)
	if err != nil {
		return fmt.Errorf("list tables: %w", err)
	}
	if list == nil {
		return nil
	}

	if _, err := pool.Exec(ctx, "TRUNCATE "+*list+" CASCADE"); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}
	return nil
}
