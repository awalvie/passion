package db_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	"passion/server/db"
	"passion/server/db/dbtest"
)

const (
	k1 = "0199c3a0-0000-7000-8000-0000000000c1"
	k2 = "0199c3a0-0000-7000-8000-0000000000c2"
	k3 = "0199c3a0-0000-7000-8000-0000000000c3"
	k4 = "0199c3a0-0000-7000-8000-0000000000c4"
)

func climb(t *testing.T, f db.ClimbFields) db.ClimbFields {
	t.Helper()
	f, problems := f.Clean()
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	return f
}

func boulder(step, grade string) db.ClimbFields {
	return db.ClimbFields{Step: step, Discipline: "boulder", Setting: "indoor", Grade: &grade, GradeSystem: ptr("font")}
}

func TestPutClimb(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	run := runWith(t, ctx, pool, ada, kinded(runStep(s1, e1, "Bouldering"), "climbing"))

	f := boulder(s1, "6c")
	f.Position, f.Outcome = 1, ptr("flash")
	got, err := db.PutClimb(ctx, pool, ada, run.ID, k1, climb(t, f))
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if got.ID != k1 || got.Exercise != e1 || got.GradeRank == nil || !got.Sent() {
		t.Fatalf("climb %+v, want a ranked send of the step's exercise, under the id the client chose", got)
	}

	// A retry writes the same climb.
	f.Outcome = ptr("working")
	for range 2 {
		if got, err = db.PutClimb(ctx, pool, ada, run.ID, k1, climb(t, f)); err != nil {
			t.Fatal(err)
		}
	}
	if got.Sent() {
		t.Fatal("a climb still being worked counts as a send")
	}

	lap := db.ClimbFields{Step: s1, Discipline: "boulder", Setting: "indoor", Grade: ptr("Traverse"), Outcome: ptr("flash")}
	if got, err = db.PutClimb(ctx, pool, ada, run.ID, k2, climb(t, lap)); err != nil {
		t.Fatal(err)
	}
	if got.Sent() || got.GradeRank != nil {
		t.Fatalf("climb %+v, want an ungraded lap with no rank that is never a send", got)
	}

	read, err := db.GetRun(ctx, pool, ada, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if st := read.Body.Sections[0].Items[0].Step.Status; st == nil || *st != db.StepDone {
		t.Fatalf("status %v, want a step with climbs done", st)
	}
	var ids []string
	for _, c := range read.Climbs {
		ids = append(ids, c.ID)
	}
	if !slices.Equal(ids, []string{k2, k1}) {
		t.Fatalf("climbs %v, want them in the order they were climbed", ids)
	}
}

func TestPutClimbRefuses(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	skipped := db.StepSkipped
	skip := kinded(runStep(s2, e2, "Limit"), "climbing")
	skip.Status = &skipped
	run := runWith(t, ctx, pool, ada, kinded(runStep(s1, e1, "Bouldering"), "climbing"), skip, runStep(s3, e2, "Pull"))
	other := runWith(t, ctx, pool, bob, kinded(runStep(s1, e1, "Bouldering"), "climbing"))
	if _, err := db.PutClimb(ctx, pool, ada, run.ID, k1, climb(t, boulder(s1, "6a"))); err != nil {
		t.Fatal(err)
	}

	var problem db.StepProblem
	for name, c := range map[string]struct {
		owner, run, climb, step string
		want                    func(error) bool
	}{
		"a step it does not hold":     {ada, run.ID, k2, c1, func(err error) bool { return errors.As(err, &problem) }},
		"a skipped step":              {ada, run.ID, k2, s2, func(err error) bool { return errors.As(err, &problem) }},
		"a step that logs sets":       {ada, run.ID, k2, s3, func(err error) bool { return errors.As(err, &problem) }},
		"someone else's run":          {bob, run.ID, k2, s1, func(err error) bool { return errors.Is(err, db.ErrNoRun) }},
		"an id another run holds":     {bob, other.ID, k1, s1, func(err error) bool { return errors.Is(err, db.ErrNoClimb) }},
		"a climb id that is not uuid": {ada, run.ID, "nope", s1, func(err error) bool { return errors.Is(err, db.ErrNoClimb) }},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := db.PutClimb(ctx, pool, c.owner, c.run, c.climb, climb(t, boulder(c.step, "6a"))); !c.want(err) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestDeleteClimb(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	run := runWith(t, ctx, pool, ada, kinded(runStep(s1, e1, "Bouldering"), "climbing"))
	if _, err := db.PutClimb(ctx, pool, ada, run.ID, k1, climb(t, boulder(s1, "6a"))); err != nil {
		t.Fatal(err)
	}

	if err := db.DeleteClimb(ctx, pool, bob, run.ID, k1); !errors.Is(err, db.ErrNoClimb) {
		t.Fatalf("someone else deleted it: %v", err)
	}
	if err := db.DeleteClimb(ctx, pool, ada, run.ID, k1); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteClimb(ctx, pool, ada, run.ID, k1); !errors.Is(err, db.ErrNoClimb) {
		t.Fatalf("deleted twice: %v", err)
	}
	if err := db.DeleteClimb(ctx, pool, ada, run.ID, "nope"); !errors.Is(err, db.ErrNoClimb) {
		t.Fatalf("an id that is not a uuid: %v", err)
	}
}

func TestUpdateRunDropsClimbsOfStepsItNoLongerKeeps(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	run := runWith(t, ctx, pool, ada,
		kinded(runStep(s1, e1, "Bouldering"), "climbing"),
		kinded(runStep(s2, e1, "Limit"), "climbing"),
		kinded(runStep(s3, e1, "Routes"), "climbing"),
		kinded(runStep(c1, e1, "Board"), "climbing"))
	for step, id := range map[string]string{s1: k1, s2: k2, s3: k3, c1: k4} {
		if _, err := db.PutClimb(ctx, pool, ada, run.ID, id, climb(t, boulder(step, "6a"))); err != nil {
			t.Fatal(err)
		}
	}
	run, err := db.GetRun(ctx, pool, ada, run.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Keep s1, skip s2, make s3 a step that logs sets, and point a fourth at
	// another exercise.
	skipped := db.StepSkipped
	items := run.Body.Sections[0].Items
	keep, skip, sets, swap := *items[0].Step, *items[1].Step, *items[2].Step, *items[3].Step
	skip.Status = &skipped
	sets.Kind = "open"
	swap.Exercise = e2
	got := putBody(t, ctx, pool, ada, run, keep, skip, sets, swap)

	if len(got.Climbs) != 1 || got.Climbs[0].Step != s1 {
		t.Fatalf("climbs %+v, want only the kept step's", got.Climbs)
	}
}

func TestCleanClimbFields(t *testing.T) {
	for name, c := range map[string]struct {
		f    db.ClimbFields
		want []string
	}{
		"a clean boulder": {boulder(s1, "7a"), nil},
		"a route on a boulder scale": {
			db.ClimbFields{Step: s1, Discipline: "sport", Setting: "outdoor", Grade: ptr("7a"), GradeSystem: ptr("font")},
			[]string{"grade_system"},
		},
		"a grade off its scale": {boulder(s1, "V4"), []string{"grade"}},
		"a grade with no system": {
			db.ClimbFields{Step: s1, Discipline: "boulder", Setting: "indoor", Grade: ptr("6a")},
			[]string{"grade_system"},
		},
		"a system with no grade": {
			db.ClimbFields{Step: s1, Discipline: "boulder", Setting: "indoor", GradeSystem: ptr("v")},
			[]string{"grade"},
		},
		"a board on a route": {
			db.ClimbFields{Step: s1, Discipline: "sport", Setting: "indoor", Board: ptr("moon")},
			[]string{"board"},
		},
		"every bad value": {
			db.ClimbFields{Step: "nope", Position: -1, Discipline: "ice", Setting: "space", RopeStyle: ptr("solo"),
				Outcome: ptr("sent"), Attempts: ptr(0), Seconds: ptr(-1), Stars: ptr(4)},
			[]string{"attempts", "discipline", "outcome", "position", "rope_style", "seconds", "setting", "stars", "step"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, problems := c.f.Clean()
			if got := slices.Sorted(maps.Keys(problems)); !slices.Equal(got, c.want) {
				t.Fatalf("problems %v, want %v", problems, c.want)
			}
		})
	}
}
