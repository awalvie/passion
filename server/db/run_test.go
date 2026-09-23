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

const (
	s1 = "0199c3a0-0000-7000-8000-00000000000a"
	s2 = "0199c3a0-0000-7000-8000-00000000000b"
	c1 = "0199c3a0-0000-7000-8000-00000000000c"
)

func runStep(id, exercise, name string) db.RunStep {
	return db.RunStep{ID: id, Step: step(exercise, name)}
}

func startRun(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner string, s db.RunStart) db.Run {
	t.Helper()
	s, problems := s.Clean()
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	run, err := db.StartRun(ctx, pool, owner, s)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	return run
}

func TestStartRunFromTemplate(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	account, err := db.CreateAccount(ctx, pool, "ada@example.com", "hash", "Ada", "Pacific/Auckland")
	if err != nil {
		t.Fatal(err)
	}
	ada := account.ID

	hang := mustInsertExercise(t, pool, "", "max_hangs", "Max Hangs")
	raise := mustInsertExercise(t, pool, ada, "", "Y raises")
	tpl, err := db.CreateSessionTemplate(ctx, pool, ada, template(t, "Power", hang, raise))
	if err != nil {
		t.Fatal(err)
	}

	// 20:00 in London on New Year's Day is already the 2nd in Auckland.
	started := time.Date(2026, 1, 1, 20, 0, 0, 0, time.UTC)
	run := startRun(t, ctx, pool, ada, db.RunStart{Template: &tpl.ID, StartedAt: started})

	if run.Name != "Power" || run.Template == nil || *run.Template != tpl.ID || run.Owner != ada {
		t.Fatalf("name %q, template %v, owner %s", run.Name, run.Template, run.Owner)
	}
	if got := run.LocalDate.Format(time.DateOnly); got != "2026-01-02" || run.Timezone != "Pacific/Auckland" {
		t.Fatalf("local date %s in %s, want 2026-01-02 in Pacific/Auckland", got, run.Timezone)
	}
	if run.Plan == nil || len(run.Plan.Sections[0].Items) != 2 {
		t.Fatalf("plan %+v, want the template's body", run.Plan)
	}
	if run.FinishedAt != nil {
		t.Fatal("a new run is finished")
	}

	items := run.Body.Sections[0].Items
	hangStep, choice := items[0].Step, items[1].Choice
	if hangStep == nil || hangStep.ID == "" || hangStep.Exercise != hang || hangStep.Status != nil {
		t.Fatalf("step %+v, want Max Hangs with an id and no status", hangStep)
	}
	if choice == nil || choice.ID == "" || choice.ID == hangStep.ID || len(choice.Options) != 1 {
		t.Fatalf("choice %+v, want the offer with an id of its own", choice)
	}

	// Editing the template afterwards never reaches the run.
	edited := template(t, "Renamed", hang, raise)
	if _, err := db.UpdateSessionTemplate(ctx, pool, ada, tpl.ID, edited); err != nil {
		t.Fatal(err)
	}
	again, err := db.GetRun(ctx, pool, ada, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Name != "Power" {
		t.Fatalf("name %q after the template changed, want Power", again.Name)
	}
}

func TestStartOpenRun(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")

	day := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	run := startRun(t, ctx, pool, ada, db.RunStart{Name: " Saturday ", StartedAt: time.Now(), LocalDate: &day})

	if run.Name != "Saturday" || run.Template != nil || run.Plan != nil || len(run.Body.Sections) != 0 {
		t.Fatalf("run %+v, want an empty open run named Saturday", run)
	}
	if got := run.LocalDate.Format(time.DateOnly); got != "2026-03-07" {
		t.Fatalf("local date %s, want the day the write-up names", got)
	}
}

func TestStartRunRefuses(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	if _, problems := (db.RunStart{Name: " "}).Clean(); problems["name"] == "" {
		t.Fatalf("problems %v, want an open run to need a name", problems)
	}
	if _, problems := (db.RunStart{Template: ptr("nope")}).Clean(); problems["template"] == "" {
		t.Fatalf("problems %v, want a bad template id refused", problems)
	}

	bobs := insertSessionTemplate(t, pool, bob, "Bob's")
	_, err := db.StartRun(ctx, pool, ada, db.RunStart{Template: &bobs, StartedAt: time.Now()})
	if !errors.Is(err, db.ErrNoSessionTemplate) {
		t.Fatalf("got %v, want someone else's template refused", err)
	}
}

func TestCleanRunBody(t *testing.T) {
	bad := "stopped"
	_, problems := db.RunBody{Sections: []db.RunSection{{
		Name: "Main",
		Items: []db.RunItem{
			{Step: ptr(runStep(s1, e1, "Hang"))},
			{Step: ptr(runStep(s1, e1, "Hang again"))},
			{Step: &db.RunStep{ID: "nope", Step: step(e1, "Pull"), Status: &bad}},
			{Choice: &db.RunChoice{ID: c1, Choice: db.Choice{Name: "Shoulders"}}},
			{},
		},
	}}}.Clean()

	want := []string{
		"sections[0].items[1].step.id",
		"sections[0].items[2].step.id",
		"sections[0].items[2].step.status",
		"sections[0].items[3].choice.options",
		"sections[0].items[4]",
	}
	if got := slices.Sorted(maps.Keys(problems)); !slices.Equal(got, want) {
		t.Fatalf("problems %v, want %v", problems, want)
	}
}

func TestCleanRunFields(t *testing.T) {
	f, problems := db.RunFields{
		Name:    " Power ",
		Body:    db.RunBody{Sections: []db.RunSection{}},
		Place:   ptr(" "),
		Journal: db.Journal{Sleep: ptr(6), RPE: ptr(10), Focus: ptr("vibes"), Setting: ptr("indoor")},
	}.Clean()

	if f.Name != "Power" || f.Place != nil {
		t.Fatalf("name %q, place %v", f.Name, f.Place)
	}
	want := []string{"focus", "sleep"}
	if got := slices.Sorted(maps.Keys(problems)); !slices.Equal(got, want) {
		t.Fatalf("problems %v, want %v", problems, want)
	}
}

func TestUpdateRun(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	run := startRun(t, ctx, pool, ada, db.RunStart{Name: "Open", StartedAt: time.Now()})

	done := db.StepDone
	f, problems := db.RunFields{
		Name: "Open session",
		Body: db.RunBody{Sections: []db.RunSection{{
			Name: "Main",
			Items: []db.RunItem{
				{Step: &db.RunStep{ID: s1, Step: step(e1, "Hang"), Status: &done, RunNotes: ptr("right shoulder tight")}},
				{Step: ptr(runStep(s2, e2, "Pull"))},
			},
		}}},
		StartedAt: run.StartedAt,
		LocalDate: run.LocalDate,
		Place:     ptr("The Castle"),
		Journal:   db.Journal{Sleep: ptr(3), Energy: ptr(4), RPE: ptr(7), WentWell: ptr("left hand felt solid")},
	}.Clean()
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}

	got, err := db.UpdateRun(ctx, pool, ada, run.ID, f)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	items := got.Body.Sections[0].Items
	if len(items) != 2 || *items[0].Step.Status != db.StepDone || *items[0].Step.RunNotes != "right shoulder tight" {
		t.Fatalf("body %+v", got.Body)
	}
	if *got.Place != "The Castle" || *got.Sleep != 3 || *got.RPE != 7 || *got.WentWell != "left hand felt solid" {
		t.Fatalf("run %+v", got)
	}

	if _, err := db.UpdateRun(ctx, pool, bob, run.ID, f); !errors.Is(err, db.ErrNoRun) {
		t.Fatalf("got %v, want someone else's run refused", err)
	}
}

func TestFinishRun(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")

	run := startRun(t, ctx, pool, ada, db.RunStart{Name: "Open", StartedAt: time.Now()})
	done := db.StepDone
	f, _ := db.RunFields{
		Name: "Open",
		Body: db.RunBody{Sections: []db.RunSection{{
			Name: "Main",
			Items: []db.RunItem{
				{Step: &db.RunStep{ID: s1, Step: step(e1, "Hang"), Status: &done}},
				{Step: ptr(runStep(s2, e2, "Mobility"))},
				{Choice: &db.RunChoice{ID: c1, Choice: db.Choice{Name: "Shoulders", Pick: 1, Options: []db.Step{step(e1, "Y raises")}}}},
			},
		}}},
		StartedAt: run.StartedAt,
		LocalDate: run.LocalDate,
	}.Clean()
	if _, err := db.UpdateRun(ctx, pool, ada, run.ID, f); err != nil {
		t.Fatal(err)
	}

	first, err := db.FinishRun(ctx, pool, ada, run.ID)
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	items := first.Body.Sections[0].Items
	if first.FinishedAt == nil || *items[0].Step.Status != db.StepDone || *items[1].Step.Status != db.StepSkipped {
		t.Fatalf("finished %v, statuses %v and %v, want done kept and the untouched step skipped",
			first.FinishedAt, *items[0].Step.Status, *items[1].Step.Status)
	}
	if items[2].Choice == nil {
		t.Fatal("the unpicked choice is gone")
	}

	second, err := db.FinishRun(ctx, pool, ada, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !second.FinishedAt.Equal(*first.FinishedAt) {
		t.Fatalf("finishing twice moved the time from %v to %v", first.FinishedAt, second.FinishedAt)
	}

	if _, err := db.FinishRun(ctx, pool, ada, "nope"); !errors.Is(err, db.ErrNoRun) {
		t.Fatalf("got %v, want ErrNoRun", err)
	}
}

func TestListRuns(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	day := func(d int) *time.Time { v := time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC); return &v }
	startRun(t, ctx, pool, ada, db.RunStart{Name: "Tuesday", StartedAt: time.Now(), LocalDate: day(3)})
	startRun(t, ctx, pool, ada, db.RunStart{Name: "Thursday", StartedAt: time.Now(), LocalDate: day(5)})
	startRun(t, ctx, pool, bob, db.RunStart{Name: "Bob's", StartedAt: time.Now(), LocalDate: day(4)})

	list, err := db.ListRuns(ctx, pool, ada)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range list {
		names = append(names, r.Name)
	}
	if !slices.Equal(names, []string{"Thursday", "Tuesday"}) {
		t.Fatalf("listed %q, want Ada's, newest day first", names)
	}
}

func TestDeleteRun(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	run := startRun(t, ctx, pool, ada, db.RunStart{Name: "Open", StartedAt: time.Now()})

	if err := db.DeleteRun(ctx, pool, bob, run.ID); !errors.Is(err, db.ErrNoRun) {
		t.Fatalf("got %v, want someone else's run refused", err)
	}
	if err := db.DeleteRun(ctx, pool, ada, run.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := db.GetRun(ctx, pool, ada, run.ID); !errors.Is(err, db.ErrNoRun) {
		t.Fatalf("got %v after delete, want ErrNoRun", err)
	}
}
