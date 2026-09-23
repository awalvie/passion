package db_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/db"
	"passion/server/db/dbtest"
)

func TestExerciseHistory(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	logged := func(owner string, finish bool, steps ...db.RunStep) db.Run {
		t.Helper()
		run := runWith(t, ctx, pool, owner, steps...)
		for _, s := range steps {
			var err error
			if s.Kind == "climbing" {
				_, err = db.PutClimb(ctx, pool, owner, run.ID, newUUID(t, pool), climb(t, boulder(s.ID, "6a")))
			} else {
				_, err = db.ReplaceSets(ctx, pool, owner, run.ID, s.ID, []db.SetFields{{Reps: ptr(5)}, {Reps: ptr(4)}})
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		if finish {
			if _, err := db.FinishRun(ctx, pool, owner, run.ID); err != nil {
				t.Fatal(err)
			}
		}
		return run
	}
	first := logged(ada, true, runStep(s1, e1, "Pull"), runStep(s2, e2, "Row"))
	second := logged(ada, true, runStep(s1, e1, "Pull"))
	logged(ada, false, runStep(s1, e1, "Pull"))
	logged(bob, true, runStep(s1, e1, "Pull"))
	laps := logged(ada, true, kinded(runStep(s3, e2, "Laps"), "climbing"))

	history, err := db.ExerciseHistory(ctx, pool, ada, e1)
	if err != nil {
		t.Fatal(err)
	}
	var runs []string
	for _, h := range history {
		runs = append(runs, h.Run)
		if len(h.Sets) != 2 || h.Sets[0].Exercise != e1 || len(h.Climbs) != 0 {
			t.Fatalf("run %s holds %+v, want the two sets of Pull alone", h.Run, h)
		}
	}
	if !slices.Equal(runs, []string{second.ID, first.ID}) {
		t.Fatalf("runs %v, want the two finished ones of ada's, newest first", runs)
	}

	climbs, err := db.ExerciseHistory(ctx, pool, ada, e2)
	if err != nil {
		t.Fatal(err)
	}
	if len(climbs) != 2 || climbs[0].Run != laps.ID || len(climbs[0].Climbs) != 1 || len(climbs[1].Sets) != 2 {
		t.Fatalf("history %+v, want the climb run, then the row sets", climbs)
	}

	if none, err := db.ExerciseHistory(ctx, pool, ada, c1); err != nil || len(none) != 0 {
		t.Fatalf("history %v, %v, want an empty list for an exercise never logged", none, err)
	}
	if _, err := db.ExerciseHistory(ctx, pool, ada, "nope"); !errors.Is(err, db.ErrNoExercise) {
		t.Fatalf("got %v, want ErrNoExercise", err)
	}
}

func newUUID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `SELECT gen_random_uuid()::text`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
