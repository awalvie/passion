package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/db"
	"passion/server/db/dbtest"
)

const (
	hangFile = "9555c81a-ce5e-4b30-8a0a-159d5852b976"
	pullFile = "11111111-1111-4111-8111-111111111111"
)

func file(id, slug, hash string) db.FileExercise {
	return db.FileExercise{FileID: id, Slug: slug, Hash: hash, Fields: db.ExerciseFields{Name: slug, Kind: "open"}}
}

func load(t *testing.T, pool *pgxpool.Pool, owner *string, files ...db.FileExercise) db.LoadResult {
	t.Helper()
	result, err := db.LoadExercises(context.Background(), pool, owner, files)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return result
}

type loadedRow struct {
	ID        string
	Slug      *string
	Name      string
	Retired   bool
	UpdatedAt time.Time
}

// row finds the row a file loaded for owner, or for the shipped catalog when
// owner is empty.
func row(t *testing.T, pool *pgxpool.Pool, owner, fileID string) loadedRow {
	t.Helper()
	var r loadedRow
	err := pool.QueryRow(context.Background(), `
		SELECT id, slug, name, retired_at IS NOT NULL, updated_at FROM exercise
		WHERE owner IS NOT DISTINCT FROM nullif($1, '')::uuid AND file_id = $2`, owner, fileID).
		Scan(&r.ID, &r.Slug, &r.Name, &r.Retired, &r.UpdatedAt)
	if err != nil {
		t.Fatalf("row %s: %v", fileID, err)
	}
	return r
}

func TestLoadWritesOnlyWhatChanged(t *testing.T) {
	pool := dbtest.Pool(t)

	if got := load(t, pool, nil, file(hangFile, "hang", "h1"), file(pullFile, "pull", "p1")); got.Written != 2 {
		t.Fatalf("first load wrote %d, want 2", got.Written)
	}
	pull := row(t, pool, "", pullFile)

	if got := load(t, pool, nil, file(hangFile, "hang", "h1"), file(pullFile, "pull", "p1")); got.Written != 0 {
		t.Fatalf("a load with nothing changed wrote %d", got.Written)
	}

	changedHang := file(hangFile, "hang", "h2")
	changedHang.Fields.Name = "Long Hang"
	if got := load(t, pool, nil, changedHang, file(pullFile, "pull", "p1")); got.Written != 1 {
		t.Fatalf("one changed file wrote %d", got.Written)
	}
	if got := row(t, pool, "", hangFile); got.Name != "Long Hang" {
		t.Fatalf("name %q", got.Name)
	}
	if got := row(t, pool, "", pullFile); !got.UpdatedAt.Equal(pull.UpdatedAt) {
		t.Fatal("the unchanged file's row was written")
	}
}

func TestLoadFollowsARename(t *testing.T) {
	pool := dbtest.Pool(t)
	load(t, pool, nil, file(hangFile, "hang", "h1"))
	before := row(t, pool, "", hangFile)

	load(t, pool, nil, file(hangFile, "long_hang", "h2"))
	after := row(t, pool, "", hangFile)
	if after.ID != before.ID || after.Slug == nil || *after.Slug != "long_hang" {
		t.Fatalf("got %+v, want the same row with the new slug", after)
	}
}

// Two files that swap names in one change must not clash half way through.
func TestLoadSwapsNames(t *testing.T) {
	pool := dbtest.Pool(t)
	load(t, pool, nil, file(hangFile, "a", "h1"), file(pullFile, "b", "p1"))
	load(t, pool, nil, file(hangFile, "b", "h2"), file(pullFile, "a", "p2"))

	if got := row(t, pool, "", hangFile); *got.Slug != "b" {
		t.Fatalf("hang slug %q", *got.Slug)
	}
}

func TestLoadRetiresAGoneFileAndBringsItBack(t *testing.T) {
	pool := dbtest.Pool(t)
	load(t, pool, nil, file(hangFile, "hang", "h1"))
	before := row(t, pool, "", hangFile)

	if got := load(t, pool, nil); got.Retired != 1 {
		t.Fatalf("retired %d, want 1", got.Retired)
	}
	gone := row(t, pool, "", hangFile)
	if !gone.Retired || gone.Slug != nil {
		t.Fatalf("got %+v, want it retired with its name given up", gone)
	}

	// A new file may take the name the gone one had.
	load(t, pool, nil, file(pullFile, "hang", "p1"))

	load(t, pool, nil, file(pullFile, "pull", "p2"), file(hangFile, "hang", "h1"))
	back := row(t, pool, "", hangFile)
	if back.ID != before.ID || back.Retired || *back.Slug != "hang" {
		t.Fatalf("got %+v, want the same row back in the library", back)
	}
}

// A row retired in the app stays retired while its file does not change.
func TestLoadKeepsARetirementMadeInTheApp(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")

	load(t, pool, &ada, file(hangFile, "hang", "h1"))
	if err := db.RetireExercise(ctx, pool, ada, row(t, pool, ada, hangFile).ID); err != nil {
		t.Fatal(err)
	}

	load(t, pool, &ada, file(hangFile, "hang", "h1"))
	if !row(t, pool, ada, hangFile).Retired {
		t.Fatal("an unchanged file brought back an exercise retired in the app")
	}

	// Deleting the file then still frees its name.
	if got := load(t, pool, &ada); got.Retired != 1 {
		t.Fatalf("retired %d, want 1", got.Retired)
	}
	if got := row(t, pool, ada, hangFile); !got.Retired || got.Slug != nil {
		t.Fatalf("got %+v, want it retired with its name given up", got)
	}
}

func TestLoadKeepsOwnersApart(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	typed := mustInsertExercise(t, pool, ada, "", "Typed in the app")

	load(t, pool, nil, file(hangFile, "hang", "h1"))
	load(t, pool, &ada, file(hangFile, "hang", "h1"))
	load(t, pool, &bob, file(hangFile, "hang", "h1"))

	// Emptying Ada's tree retires only her loaded row.
	if got := load(t, pool, &ada); got.Retired != 1 {
		t.Fatalf("retired %d, want 1", got.Retired)
	}
	if row(t, pool, "", hangFile).Retired || row(t, pool, bob, hangFile).Retired {
		t.Fatal("one owner's load retired another's row")
	}
	e, err := db.GetExercise(ctx, pool, ada, typed)
	if err != nil || e.RetiredAt != nil {
		t.Fatalf("the exercise typed into the app: %v, retired %v", err, e.RetiredAt)
	}
}

// A load waits while another one holds the lock.
func TestLoadWaitsForTheLock(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)

	other, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Rollback(ctx)
	if _, err := other.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('passion catalog load'))`); err != nil {
		t.Fatal(err)
	}

	waiting, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	if _, err := db.LoadExercises(waiting, pool, nil, []db.FileExercise{file(hangFile, "hang", "h1")}); err == nil {
		t.Fatal("the load ran while another held the lock")
	}
}
