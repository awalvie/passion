package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/db"
	"passion/server/db/dbtest"
)

const (
	goal1 = "0199c3a0-0000-7000-8000-000000000b01"
	goal2 = "0199c3a0-0000-7000-8000-000000000b02"
	goal3 = "0199c3a0-0000-7000-8000-000000000b03"
	span1 = "0199c3a0-0000-7000-8000-000000000c01"
	span2 = "0199c3a0-0000-7000-8000-000000000c02"
)

func goalFields(t *testing.T, text, period string) db.PeriodGoalFields {
	t.Helper()
	f, problems := db.PeriodGoalFields{Text: text, Period: period}.Clean()
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	return f
}

func spanFields(t *testing.T, text, id, name, starts, ends string) db.PeriodGoalFields {
	t.Helper()
	f, problems := db.PeriodGoalFields{Text: text, Period: "custom",
		Span: &db.GoalSpanFields{ID: id, Name: name, Starts: day(starts), Ends: day(ends)}}.Clean()
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	return f
}

// backdate sets the day a goal was set, as PutGoal always sets today.
func backdate(t *testing.T, pool *pgxpool.Pool, id string, d time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE goal SET starts = $2 WHERE id = $1`, id, d); err != nil {
		t.Fatal(err)
	}
}

func TestPeriodGoalClean(t *testing.T) {
	nov1, oct1 := day("2026-11-01"), day("2026-10-01")
	span := func(id, name string, from, to time.Time) *db.GoalSpanFields {
		return &db.GoalSpanFields{ID: id, Name: name, Starts: from, Ends: to}
	}
	for name, c := range map[string]struct {
		f           db.PeriodGoalFields
		field, want string
	}{
		"no text":                    {db.PeriodGoalFields{Text: " ", Period: "week"}, "text", "is required"},
		"an unknown period":          {db.PeriodGoalFields{Text: "Climb", Period: "day"}, "period", "must be week, month, year or custom"},
		"a custom goal with no span": {db.PeriodGoalFields{Text: "Climb", Period: "custom"}, "span", "is required"},
		"a week with a span":         {db.PeriodGoalFields{Text: "Climb", Period: "week", Span: span(span1, "", oct1, nov1)}, "span", "is only for a custom goal"},
		"a span with no id":          {db.PeriodGoalFields{Text: "Climb", Period: "custom", Span: span("", "", oct1, nov1)}, "span.id", "is required"},
		"an end before the start":    {db.PeriodGoalFields{Text: "Climb", Period: "custom", Span: span(span1, "", nov1, oct1)}, "span.ends", "must be on or after the first day"},
		"a custom goal of one day":   {db.PeriodGoalFields{Text: "Climb", Period: "custom", Span: span(span1, " ", nov1, nov1)}, "", ""},
		// The handler names a day that did not parse; Clean must not replace it.
		"an end that did not parse": {db.PeriodGoalFields{Text: "Climb", Period: "custom", Span: span(span1, "", nov1, time.Time{})}, "", ""},
	} {
		t.Run(name, func(t *testing.T) {
			_, problems := c.f.Clean()
			if c.field == "" && len(problems) != 0 {
				t.Fatalf("problems %v, want none", problems)
			}
			if c.field != "" && problems[c.field] != c.want {
				t.Fatalf("problems %v, want %s: %s", problems, c.field, c.want)
			}
		})
	}
}

// A goal is set today. A replace changes the text and refuses a new period.
func TestPutGoal(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	today := time.Now().UTC().Truncate(24 * time.Hour)

	if _, err := db.PutGoal(ctx, pool, ada, goal1, goalFields(t, "Train fingers", "week")); err != nil {
		t.Fatal(err)
	}
	g, err := db.PutGoal(ctx, pool, ada, goal1, goalFields(t, "Train fingers twice", "week"))
	if err != nil {
		t.Fatal(err)
	}
	if g.Text != "Train fingers twice" || !g.Starts.Equal(today) || g.Removed != nil || len(g.Ticks) != 0 {
		t.Fatalf("goal %+v, want the new text, set today", g)
	}
	if _, err := db.PutGoal(ctx, pool, ada, goal1, goalFields(t, "Train fingers", "month")); !errors.Is(err, db.ErrPeriodLocked) {
		t.Fatalf("got %v, want the period locked", err)
	}
	if _, err := db.PutGoal(ctx, pool, bob, goal1, goalFields(t, "Mine", "week")); !errors.Is(err, db.ErrNoGoal) {
		t.Fatalf("got %v, want someone else's id refused", err)
	}
}

// A tick names the first day of a period, from the one the goal was set in up
// to today. It keeps the text it was ticked with.
func TestTickGoal(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	today := time.Now().UTC().Truncate(24 * time.Hour)
	thisMonday := today.AddDate(0, 0, -(int(today.Weekday())+6)%7)
	lastMonday := thisMonday.AddDate(0, 0, -7)

	if _, err := db.PutGoal(ctx, pool, ada, goal1, goalFields(t, "Train fingers", "week")); err != nil {
		t.Fatal(err)
	}
	// Set on last week's Wednesday.
	backdate(t, pool, goal1, lastMonday.AddDate(0, 0, 2))

	for name, d := range map[string]time.Time{
		"not a Monday":          lastMonday.AddDate(0, 0, 2),
		"before the first week": lastMonday.AddDate(0, 0, -7),
		"next week":             thisMonday.AddDate(0, 0, 7),
	} {
		if _, err := db.TickGoal(ctx, pool, ada, goal1, d, true); !errors.Is(err, db.ErrNotAPeriod) {
			t.Fatalf("%s: got %v, want it refused", name, err)
		}
	}
	if _, err := db.TickGoal(ctx, pool, bob, goal1, lastMonday, true); !errors.Is(err, db.ErrNoGoal) {
		t.Fatalf("got %v, want someone else's goal refused", err)
	}

	// The week holding the first day counts, though it began before it.
	if _, err := db.TickGoal(ctx, pool, ada, goal1, lastMonday, true); err != nil {
		t.Fatal(err)
	}
	if _, err := db.PutGoal(ctx, pool, ada, goal1, goalFields(t, "Train fingers twice", "week")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.TickGoal(ctx, pool, ada, goal1, thisMonday, true); err != nil {
		t.Fatal(err)
	}
	g, err := db.TickGoal(ctx, pool, ada, goal1, thisMonday, true)
	if err != nil {
		t.Fatal(err)
	}
	last, this := lastMonday.Format(time.DateOnly), thisMonday.Format(time.DateOnly)
	want := []db.GoalTick{{PeriodStart: last, Text: "Train fingers"}, {PeriodStart: this, Text: "Train fingers twice"}}
	if len(g.Ticks) != 2 || g.Ticks[0] != want[0] || g.Ticks[1] != want[1] {
		t.Fatalf("ticks %+v, want %+v", g.Ticks, want)
	}

	g, err = db.TickGoal(ctx, pool, ada, goal1, lastMonday, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Ticks) != 1 || g.Ticks[0] != want[1] {
		t.Fatalf("ticks %+v, want only %+v", g.Ticks, want[1])
	}
}

// Each period starts on its own first day, and the one holding the day the
// goal was set is its first.
func TestTickGoalPeriods(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	for id, c := range map[string]struct{ period, set string }{
		goal1: {"week", "2025-12-28"}, // a Sunday
		goal2: {"month", "2025-01-15"},
		goal3: {"year", "2025-06-01"},
	} {
		if _, err := db.PutGoal(ctx, pool, ada, id, goalFields(t, "Climb", c.period)); err != nil {
			t.Fatal(err)
		}
		backdate(t, pool, id, day(c.set))
	}
	for _, c := range []struct {
		id, day string
		ok      bool
	}{
		{goal1, "2025-12-22", true},  // the Monday before the Sunday it was set
		{goal1, "2025-12-29", true},  // the next week crosses into 2026
		{goal1, "2025-12-28", false}, // a Sunday
		{goal1, "2025-12-15", false}, // the week before
		{goal2, "2025-01-01", true},
		{goal2, "2025-03-01", true},
		{goal2, "2025-01-31", false},
		{goal2, "2024-12-01", false},
		{goal3, "2025-01-01", true},
		{goal3, "2026-01-01", true},
		{goal3, "2025-12-31", false},
		{goal3, "2099-01-01", false}, // still to come
	} {
		_, err := db.TickGoal(ctx, pool, ada, c.id, day(c.day), true)
		if c.ok && err != nil || !c.ok && !errors.Is(err, db.ErrNotAPeriod) {
			t.Errorf("tick %s on %s: got %v, want ok %v", c.id, c.day, err, c.ok)
		}
	}
}

// A removed goal stays in the list with the day it went, keeps its ticks, and
// takes no more.
func TestRemoveGoal(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if _, err := db.PutGoal(ctx, pool, ada, goal1, goalFields(t, "Train fingers", "year")); err != nil {
		t.Fatal(err)
	}
	jan1 := time.Date(today.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := db.TickGoal(ctx, pool, ada, goal1, jan1, true); err != nil {
		t.Fatal(err)
	}

	if err := db.RemoveGoal(ctx, pool, ada, goal1); err != nil {
		t.Fatal(err)
	}
	list, err := db.ListGoals(ctx, pool, ada)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Removed == nil || !list[0].Removed.Equal(today) || len(list[0].Ticks) != 1 {
		t.Fatalf("goals %+v, want it removed today with its tick", list)
	}
	if _, err := db.TickGoal(ctx, pool, ada, goal1, jan1, false); !errors.Is(err, db.ErrNoGoal) {
		t.Fatalf("got %v, want a removed goal refused", err)
	}
	if _, err := db.PutGoal(ctx, pool, ada, goal1, goalFields(t, "Train fingers", "year")); !errors.Is(err, db.ErrNoGoal) {
		t.Fatalf("got %v, want a removed goal kept removed", err)
	}
	if err := db.RemoveGoal(ctx, pool, ada, goal1); !errors.Is(err, db.ErrNoGoal) {
		t.Fatalf("got %v, want a second remove refused", err)
	}
}

// A custom goal has one period, its span's. Renaming or moving the span
// changes it for every goal in it, and a tick moves with its first day.
func TestCustomGoal(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	if _, err := db.PutGoal(ctx, pool, ada, goal1, spanFields(t, "Send 7A", span1, "Font trip", "2026-01-10", "2026-01-20")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.TickGoal(ctx, pool, ada, goal1, day("2026-01-12"), true); !errors.Is(err, db.ErrNotAPeriod) {
		t.Fatalf("got %v, want a Monday inside it refused", err)
	}
	if _, err := db.TickGoal(ctx, pool, ada, goal1, day("2026-01-10"), true); err != nil {
		t.Fatal(err)
	}

	if _, err := db.PutGoal(ctx, pool, ada, goal2, spanFields(t, "Rest", span1, "", "2026-01-12", "2026-01-22")); err != nil {
		t.Fatal(err)
	}
	list, err := db.ListGoals(ctx, pool, ada)
	if err != nil {
		t.Fatal(err)
	}
	want := db.GoalSpan{ID: span1, Name: "", Starts: "2026-01-12", Ends: "2026-01-22"}
	if len(list) != 2 || *list[0].Span != want || *list[1].Span != want {
		t.Fatalf("goals %+v, want both on %+v", list, want)
	}
	if len(list[0].Ticks) != 1 || list[0].Ticks[0].PeriodStart != "2026-01-12" {
		t.Fatalf("ticks %+v, want the tick moved to the new first day", list[0].Ticks)
	}

	if _, err := db.PutGoal(ctx, pool, ada, goal1, goalFields(t, "Send 7A", "week")); !errors.Is(err, db.ErrPeriodLocked) {
		t.Fatalf("got %v, want a custom goal kept custom", err)
	}
	if _, err := db.PutGoal(ctx, pool, ada, goal1, spanFields(t, "Send 7A", span2, "", "2026-02-01", "2026-02-02")); !errors.Is(err, db.ErrSpanLocked) {
		t.Fatalf("got %v, want the goal kept on its span", err)
	}
	if _, err := db.PutGoal(ctx, pool, bob, goal3, spanFields(t, "Mine", span1, "", "2026-01-12", "2026-01-22")); !errors.Is(err, db.ErrNoGoal) {
		t.Fatalf("got %v, want someone else's span refused", err)
	}

	if _, err := db.PutGoal(ctx, pool, ada, goal3, spanFields(t, "Later", span2, "", "2099-01-01", "2099-01-02")); err != nil {
		t.Fatal(err)
	}
	if _, err := db.TickGoal(ctx, pool, ada, goal3, day("2099-01-01"), true); !errors.Is(err, db.ErrNotAPeriod) {
		t.Fatalf("got %v, want a span still to come refused", err)
	}
}

// A span moved to start later takes its tick to a day still to come, and
// the tick can still be taken off there.
func TestUntickMovedSpan(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	today := time.Now().UTC().Truncate(24 * time.Hour)
	from, to := today.Format(time.DateOnly), today.AddDate(0, 0, 3).Format(time.DateOnly)
	later, end := today.AddDate(0, 0, 7).Format(time.DateOnly), today.AddDate(0, 0, 9).Format(time.DateOnly)

	if _, err := db.PutGoal(ctx, pool, ada, goal1, spanFields(t, "Send 7A", span1, "", from, to)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.TickGoal(ctx, pool, ada, goal1, today, true); err != nil {
		t.Fatal(err)
	}
	if _, err := db.PutGoal(ctx, pool, ada, goal1, spanFields(t, "Send 7A", span1, "", later, end)); err != nil {
		t.Fatal(err)
	}
	g, err := db.TickGoal(ctx, pool, ada, goal1, day(later), false)
	if err != nil || len(g.Ticks) != 0 {
		t.Fatalf("goal %+v, %v, want the moved tick taken off", g, err)
	}
}

// Today is the person's own: the day a goal is set and removed, and the last
// day it takes a tick.
func TestGoalTimeZone(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	const zone = "Pacific/Kiritimati" // UTC+14, so its day is often a UTC tomorrow
	if _, err := pool.Exec(ctx, `UPDATE account SET timezone = $2 WHERE id = $1`, ada, zone); err != nil {
		t.Fatal(err)
	}
	var local time.Time
	if err := pool.QueryRow(ctx, `SELECT (now() AT TIME ZONE $1)::date`, zone).Scan(&local); err != nil {
		t.Fatal(err)
	}

	g, err := db.PutGoal(ctx, pool, ada, goal1, goalFields(t, "Climb", "year"))
	if err != nil {
		t.Fatal(err)
	}
	if !g.Starts.Equal(local) {
		t.Fatalf("set %s, want the local %s", g.Starts, local)
	}
	if _, err := db.PutGoal(ctx, pool, ada, goal2, spanFields(t, "Rest", span1, "", local.Format(time.DateOnly), local.Format(time.DateOnly))); err != nil {
		t.Fatal(err)
	}
	if _, err := db.TickGoal(ctx, pool, ada, goal2, local, true); err != nil {
		t.Fatalf("got %v, want the local today ticked", err)
	}
	if err := db.RemoveGoal(ctx, pool, ada, goal1); err != nil {
		t.Fatal(err)
	}
	list, err := db.ListGoals(ctx, pool, ada)
	if err != nil {
		t.Fatal(err)
	}
	if list[0].Removed == nil || !list[0].Removed.Equal(local) {
		t.Fatalf("removed %v, want the local %s", list[0].Removed, local)
	}
}
