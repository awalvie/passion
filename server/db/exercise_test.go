package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/db"
	"passion/server/db/dbtest"
)

func ptr[T any](v T) *T { return &v }

func newAccount(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	account, err := db.CreateAccount(context.Background(), pool, email, "hash", "Ada", "UTC")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return account.ID
}

// insertExercise writes a row directly, because only the catalog loader will
// write shipped rows and it does not exist yet. An empty owner means shipped.
func insertExercise(ctx context.Context, pool *pgxpool.Pool, owner, slug, name string) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO exercise (owner, slug, name, kind)
		VALUES (nullif($1, '')::uuid, nullif($2, ''), $3, 'open')
		RETURNING id`, owner, slug, name).Scan(&id)
	return id, err
}

func mustInsertExercise(t *testing.T, pool *pgxpool.Pool, owner, slug, name string) string {
	t.Helper()
	id, err := insertExercise(context.Background(), pool, owner, slug, name)
	if err != nil {
		t.Fatalf("insert %s: %v", name, err)
	}
	return id
}

func TestCreateExercise(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")

	created, err := db.CreateExercise(ctx, pool, ada, db.ExerciseFields{
		Name:           "Repeaters",
		Kind:           "timed_reps",
		Sets:           ptr(4),
		RepSeconds:     ptr(7),
		RepRestSeconds: ptr(0),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if created.ID == "" || created.Owner == nil || *created.Owner != ada {
		t.Fatalf("id %q, owner %v, want an id owned by %s", created.ID, created.Owner, ada)
	}
	if created.Slug != nil {
		t.Fatalf("slug %q, want none on an exercise typed into the app", *created.Slug)
	}
	if created.Sets == nil || *created.Sets != 4 {
		t.Fatalf("sets %v, want 4", created.Sets)
	}
	if created.RepRestSeconds == nil || *created.RepRestSeconds != 0 {
		t.Fatalf("rep rest %v, want 0 kept apart from not set", created.RepRestSeconds)
	}
	if created.Reps != nil {
		t.Fatalf("reps %v, want not set", *created.Reps)
	}
	if created.Tags == nil || len(created.Tags) != 0 {
		t.Fatalf("tags %#v, want an empty list", created.Tags)
	}
}

func TestListExercises(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	mustInsertExercise(t, pool, "", "max_hangs", "Max Hangs")
	mustInsertExercise(t, pool, ada, "", "bench press")
	mustInsertExercise(t, pool, bob, "", "Bob's Squat")
	retired := mustInsertExercise(t, pool, ada, "", "Old Drill")
	if err := db.RetireExercise(ctx, pool, ada, retired); err != nil {
		t.Fatalf("retire: %v", err)
	}

	list, err := db.ListExercises(ctx, pool, ada)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	var names []string
	for _, e := range list {
		names = append(names, e.Name)
	}
	want := []string{"bench press", "Max Hangs"}
	if len(names) != len(want) || names[0] != want[0] || names[1] != want[1] {
		t.Fatalf("listed %q, want %q", names, want)
	}
}

func TestGetExercise(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	shipped := mustInsertExercise(t, pool, "", "max_hangs", "Max Hangs")
	own := mustInsertExercise(t, pool, ada, "", "Bench Press")
	bobs := mustInsertExercise(t, pool, bob, "", "Bob's Squat")

	for name, id := range map[string]string{"shipped": shipped, "own": own} {
		t.Run(name, func(t *testing.T) {
			if _, err := db.GetExercise(ctx, pool, ada, id); err != nil {
				t.Fatalf("get: %v", err)
			}
		})
	}

	t.Run("retired, still found", func(t *testing.T) {
		if err := db.RetireExercise(ctx, pool, ada, own); err != nil {
			t.Fatalf("retire: %v", err)
		}
		got, err := db.GetExercise(ctx, pool, ada, own)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.RetiredAt == nil {
			t.Fatal("retired_at is not set")
		}
	})

	for name, id := range map[string]string{"someone else's": bobs, "not a uuid": "nope"} {
		t.Run(name, func(t *testing.T) {
			if _, err := db.GetExercise(ctx, pool, ada, id); !errors.Is(err, db.ErrNoExercise) {
				t.Fatalf("got %v, want ErrNoExercise", err)
			}
		})
	}
}

func TestUpdateExercise(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	own := mustInsertExercise(t, pool, ada, "", "Bench Press")
	shipped := mustInsertExercise(t, pool, "", "max_hangs", "Max Hangs")
	bobs := mustInsertExercise(t, pool, bob, "", "Bob's Squat")

	fields := db.ExerciseFields{
		Name: "Bench Press",
		Kind: "reps_and_sets",
		Sets: ptr(5),
		Reps: ptr(5),
		Tags: []string{"strength"},
	}

	got, err := db.UpdateExercise(ctx, pool, ada, own, fields)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.Kind != "reps_and_sets" || got.Sets == nil || *got.Sets != 5 {
		t.Fatalf("kind %q, sets %v, want reps_and_sets and 5", got.Kind, got.Sets)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "strength" {
		t.Fatalf("tags %q", got.Tags)
	}
	if !got.UpdatedAt.After(got.CreatedAt) {
		t.Fatal("updated_at did not move")
	}

	for name, id := range map[string]string{"shipped": shipped, "someone else's": bobs, "not a uuid": "nope"} {
		t.Run(name, func(t *testing.T) {
			if _, err := db.UpdateExercise(ctx, pool, ada, id, fields); !errors.Is(err, db.ErrNoExercise) {
				t.Fatalf("got %v, want ErrNoExercise", err)
			}
		})
	}
}

func TestRetireExercise(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	own := mustInsertExercise(t, pool, ada, "", "Bench Press")
	shipped := mustInsertExercise(t, pool, "", "max_hangs", "Max Hangs")
	bobs := mustInsertExercise(t, pool, bob, "", "Bob's Squat")

	if err := db.RetireExercise(ctx, pool, ada, own); err != nil {
		t.Fatalf("retire: %v", err)
	}
	first, err := db.GetExercise(ctx, pool, ada, own)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if err := db.RetireExercise(ctx, pool, ada, own); err != nil {
		t.Fatalf("retire again: %v", err)
	}
	second, err := db.GetExercise(ctx, pool, ada, own)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !second.RetiredAt.Equal(*first.RetiredAt) {
		t.Fatalf("retiring twice moved the date from %v to %v", first.RetiredAt, second.RetiredAt)
	}

	for name, id := range map[string]string{"shipped": shipped, "someone else's": bobs, "not a uuid": "nope"} {
		t.Run(name, func(t *testing.T) {
			if err := db.RetireExercise(ctx, pool, ada, id); !errors.Is(err, db.ErrNoExercise) {
				t.Fatalf("got %v, want ErrNoExercise", err)
			}
		})
	}
}

func TestExerciseSlugs(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	mustInsertExercise(t, pool, "", "bench_press", "Bench Press")

	t.Run("two shipped rows cannot share a slug", func(t *testing.T) {
		_, err := insertExercise(ctx, pool, "", "bench_press", "Bench Press")
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != "exercise_shipped_slug" {
			t.Fatalf("got %v, want the shipped slug index to refuse it", err)
		}
	})

	t.Run("each person can reuse a shipped slug", func(t *testing.T) {
		mustInsertExercise(t, pool, ada, "bench_press", "Bench Press")
		mustInsertExercise(t, pool, bob, "bench_press", "Bench Press")
	})

	t.Run("one person cannot use a slug twice", func(t *testing.T) {
		_, err := insertExercise(ctx, pool, ada, "bench_press", "Bench Press")
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != "exercise_owner_slug" {
			t.Fatalf("got %v, want the owner slug index to refuse it", err)
		}
	})

	t.Run("exercises with no slug never collide", func(t *testing.T) {
		mustInsertExercise(t, pool, ada, "", "Hang")
		mustInsertExercise(t, pool, ada, "", "Hang")
	})
}
