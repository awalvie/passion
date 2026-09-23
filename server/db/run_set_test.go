package db_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/db"
	"passion/server/db/dbtest"
)

const s3 = "0199c3a0-0000-7000-8000-00000000000d"

// runWith starts an open run whose one section holds steps.
func runWith(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner string, steps ...db.RunStep) db.Run {
	t.Helper()
	run := startRun(t, ctx, pool, owner, db.RunStart{Name: "Open", StartedAt: time.Now()})
	return putBody(t, ctx, pool, owner, run, steps...)
}

func putBody(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner string, run db.Run, steps ...db.RunStep) db.Run {
	t.Helper()
	items := []db.RunItem{}
	for _, s := range steps {
		items = append(items, db.RunItem{Step: &s})
	}
	f, problems := db.RunFields{
		Name:      run.Name,
		Body:      db.RunBody{Sections: []db.RunSection{{Name: "Main", Items: items}}},
		StartedAt: run.StartedAt,
		LocalDate: run.LocalDate,
	}.Clean()
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	updated, err := db.UpdateRun(ctx, pool, owner, run.ID, f)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	return updated
}

func kinded(s db.RunStep, kind string) db.RunStep {
	s.Kind = kind
	return s
}

func TestReplaceSets(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	run := runWith(t, ctx, pool, ada, runStep(s1, e1, "Hang"), runStep(s2, e2, "Pull"))

	sets := []db.SetFields{
		{Seconds: ptr(10), WeightKG: ptr(-5.5)},
		{Seconds: ptr(10), WeightKG: ptr(0.0)},
		{Seconds: ptr(8)},
	}
	got, err := db.ReplaceSets(ctx, pool, ada, run.ID, s1, sets)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if len(got) != 3 || got[0].Number != 1 || got[2].Number != 3 || got[0].Exercise != e1 {
		t.Fatalf("sets %+v, want three numbered sets of the step's exercise", got)
	}
	if *got[0].WeightKG != -5.5 || *got[1].WeightKG != 0 || got[2].WeightKG != nil {
		t.Fatalf("weights %v %v %v, want assistance, bodyweight and none kept apart",
			*got[0].WeightKG, *got[1].WeightKG, got[2].WeightKG)
	}

	read, err := db.GetRun(ctx, pool, ada, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if st := read.Body.Sections[0].Items[0].Step.Status; st == nil || *st != db.StepDone {
		t.Fatalf("status %v, want a step with sets done", st)
	}
	if len(read.Sets) != 3 {
		t.Fatalf("the run reads %d sets, want 3", len(read.Sets))
	}

	// A retry writes the same rows, and a shorter list drops the rest.
	for range 2 {
		if got, err = db.ReplaceSets(ctx, pool, ada, run.ID, s1, sets[:2]); err != nil {
			t.Fatal(err)
		}
	}
	if len(got) != 2 {
		t.Fatalf("%d sets after two writes of two, want 2", len(got))
	}
}

func TestReplaceSetsRefuses(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	skipped := db.StepSkipped
	skip := runStep(s2, e2, "Mobility")
	skip.Status = &skipped
	run := runWith(t, ctx, pool, ada, runStep(s1, e1, "Hang"), skip, kinded(runStep(s3, e2, "Bouldering"), "climbing"))
	one := []db.SetFields{{Reps: ptr(5)}}

	var problem db.StepProblem
	for name, c := range map[string]struct {
		owner, step string
		want        func(error) bool
	}{
		"a step it does not hold": {ada, c1, func(err error) bool { return errors.Is(err, db.ErrNoStep) }},
		"a skipped step":          {ada, s2, func(err error) bool { return errors.As(err, &problem) }},
		"a climbing step":         {ada, s3, func(err error) bool { return errors.As(err, &problem) }},
		"someone else's run":      {bob, s1, func(err error) bool { return errors.Is(err, db.ErrNoRun) }},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := db.ReplaceSets(ctx, pool, c.owner, run.ID, c.step, one); !c.want(err) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestUpdateRunDropsSetsOfStepsItNoLongerKeeps(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	run := runWith(t, ctx, pool, ada, runStep(s1, e1, "Hang"), runStep(s2, e2, "Pull"), runStep(s3, e1, "Row"))
	for _, step := range []string{s1, s2, s3} {
		if _, err := db.ReplaceSets(ctx, pool, ada, run.ID, step, []db.SetFields{{Reps: ptr(5)}}); err != nil {
			t.Fatal(err)
		}
	}
	run, err := db.GetRun(ctx, pool, ada, run.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Keep s1, skip s2, and point s3 at another exercise.
	skipped := db.StepSkipped
	items := run.Body.Sections[0].Items
	hang, pull, row := *items[0].Step, *items[1].Step, *items[2].Step
	pull.Status = &skipped
	row.Exercise = e2
	got := putBody(t, ctx, pool, ada, run, hang, pull, row)

	var steps []string
	for _, s := range got.Sets {
		steps = append(steps, s.Step)
	}
	if !slices.Equal(steps, []string{s1}) {
		t.Fatalf("sets remain on %v, want only the kept step's", steps)
	}
}

func TestCheckSets(t *testing.T) {
	problems := db.CheckSets([]db.SetFields{{Reps: ptr(-1)}, {WeightKG: ptr(12000.0)}, {Seconds: ptr(10)}})
	want := []string{"sets[0].reps", "sets[1].weight_kg"}
	if got := slices.Sorted(maps.Keys(problems)); !slices.Equal(got, want) {
		t.Fatalf("problems %v, want %v", problems, want)
	}
}
