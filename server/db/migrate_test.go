package db_test

import (
	"context"
	"sync"
	"testing"

	"passion/server/db"
	"passion/server/db/dbtest"
)

func TestMigrateIsRepeatable(t *testing.T) {
	ctx := context.Background()
	d := dbtest.DSN(t)

	if err := db.Migrate(ctx, d); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := db.Migrate(ctx, d); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	pool, err := db.Open(ctx, d)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()

	for _, table := range []string{"account", "auth_token"} {
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT to_regclass('public.'||$1) IS NOT NULL", table).Scan(&exists)
		if err != nil {
			t.Fatalf("look up %s: %v", table, err)
		}
		if !exists {
			t.Fatalf("table %s is missing after migrating", table)
		}
	}
}

// The advisory lock exists so that two servers starting at once do not race.
// Starting from an empty schema is the point: every racer has real work to do.
func TestMigrateIsSafeInParallel(t *testing.T) {
	ctx := context.Background()
	d := dbtest.DSN(t)

	pool, err := db.Open(ctx, d)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_, err = pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	pool.Close()
	if err != nil {
		t.Fatalf("empty the schema: %v", err)
	}

	const racers = 8
	errs := make([]error, racers)

	var wg sync.WaitGroup
	for i := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = db.Migrate(ctx, d)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("racer %d: %v", i, err)
		}
	}

	pool, err = db.Open(ctx, d)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, `
		SELECT version_id, count(*) FROM goose_db_version
		WHERE version_id > 0
		GROUP BY version_id
		HAVING count(*) > 1`)
	if err != nil {
		t.Fatalf("count applied migrations: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var version, applied int64
		if err := rows.Scan(&version, &applied); err != nil {
			t.Fatalf("read applied migrations: %v", err)
		}
		t.Errorf("migration %d applied %d times, want exactly 1", version, applied)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read applied migrations: %v", err)
	}
}
