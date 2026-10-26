package db_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/db"
	"passion/server/db/dbtest"
)

const (
	rep1 = "0199c3a0-0000-7000-8000-000000000b01"
	rep2 = "0199c3a0-0000-7000-8000-000000000b02"
)

func repeatRows(t *testing.T, pool *pgxpool.Pool, id string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT local_date::text FROM scheduled_session WHERE repeat = $1 ORDER BY local_date`, id)
	if err != nil {
		t.Fatal(err)
	}
	var days []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			t.Fatal(err)
		}
		days = append(days, d)
	}
	return days
}

func cleaned(t *testing.T, f db.RepeatFields) db.RepeatFields {
	t.Helper()
	f, problems := f.Clean()
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	return f
}

func dates(today time.Time, offsets ...int) []string {
	var out []string
	for _, o := range offsets {
		out = append(out, today.AddDate(0, 0, o).Format(time.DateOnly))
	}
	return out
}

func TestRepeatClean(t *testing.T) {
	starts := day("2026-10-27")
	for name, c := range map[string]struct {
		f     db.RepeatFields
		field string
	}{
		"an unknown unit":       {db.RepeatFields{Template: rep1, Starts: starts, Every: 1, Unit: "month"}, "unit"},
		"every 0":               {db.RepeatFields{Template: rep1, Starts: starts, Every: 0, Unit: "day"}, "every"},
		"a week with no days":   {db.RepeatFields{Template: rep1, Starts: starts, Every: 1, Unit: "week"}, "weekdays"},
		"a day after Sunday":    {db.RepeatFields{Template: rep1, Starts: starts, Every: 1, Unit: "week", Weekdays: []int{8}}, "weekdays"},
		"an end before start":   {db.RepeatFields{Template: rep1, Starts: starts, Ends: ptr(day("2026-10-26")), Every: 1, Unit: "day"}, "ends"},
		"a template not a uuid": {db.RepeatFields{Template: "nope", Starts: starts, Every: 1, Unit: "day"}, "template"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, problems := c.f.Clean(); problems[c.field] == "" {
				t.Fatalf("problems %v, want %s named", problems, c.field)
			}
		})
	}

	f, problems := db.RepeatFields{Template: rep1, Starts: starts, Every: 1, Unit: "week", Weekdays: []int{4, 2, 4}}.Clean()
	if len(problems) != 0 || !slices.Equal(f.Weekdays, []int{2, 4}) {
		t.Fatalf("weekdays %v, problems %v, want 2 and 4 once each", f.Weekdays, problems)
	}
	f, _ = db.RepeatFields{Template: rep1, Starts: starts, Every: 1, Unit: "day", Weekdays: []int{2}}.Clean()
	if len(f.Weekdays) != 0 {
		t.Fatalf("weekdays %v, want none on a daily repeat", f.Weekdays)
	}
}

func TestPutRepeat(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if _, err := db.ScheduleSession(ctx, pool, ada, hang, today.AddDate(0, 0, 2)); err != nil {
		t.Fatal(err)
	}
	daily := db.RepeatFields{Template: hang, Starts: today, Ends: ptr(today.AddDate(0, 0, 6)), Every: 1, Unit: "day"}

	// It leaves out the day a one-off holds, and a retry makes one repeat.
	r, leftOut, err := db.PutRepeat(ctx, pool, ada, rep1, cleaned(t, daily))
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != rep1 || len(leftOut) != 1 || !leftOut[0].LocalDate.Equal(today.AddDate(0, 0, 2)) {
		t.Fatalf("repeat %+v, left out %v, want the one-off's day left out", r, leftOut)
	}
	if r, _, err := db.PutRepeat(ctx, pool, ada, rep1, cleaned(t, daily)); err != nil || r.ID != rep1 {
		t.Fatalf("retry %+v, %v", r, err)
	}
	if got, want := repeatRows(t, pool, rep1), dates(today, 0, 1, 3, 4, 5, 6); !slices.Equal(got, want) {
		t.Fatalf("days %v, want %v", got, want)
	}

	// The same rule again leaves a removed day removed.
	if _, err := pool.Exec(ctx, `DELETE FROM scheduled_session WHERE repeat = $1 AND local_date = $2`, rep1, today.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.PutRepeat(ctx, pool, ada, rep1, cleaned(t, daily)); err != nil {
		t.Fatal(err)
	}
	if got, want := repeatRows(t, pool, rep1), dates(today, 0, 3, 4, 5, 6); !slices.Equal(got, want) {
		t.Fatalf("days %v, want %v", got, want)
	}

	// An edit places the days again.
	daily.Every = 2
	if _, _, err := db.PutRepeat(ctx, pool, ada, rep1, cleaned(t, daily)); err != nil {
		t.Fatal(err)
	}
	if got, want := repeatRows(t, pool, rep1), dates(today, 0, 4, 6); !slices.Equal(got, want) {
		t.Fatalf("days %v, want %v", got, want)
	}

	if _, _, err := db.PutRepeat(ctx, pool, bob, rep1, cleaned(t, db.RepeatFields{Template: hang, Starts: today, Every: 1, Unit: "day"})); !errors.Is(err, db.ErrNoSessionTemplate) {
		t.Fatalf("got %v, want someone else's template refused", err)
	}
	bobs := insertSessionTemplate(t, pool, bob, "Bob's")
	if _, _, err := db.PutRepeat(ctx, pool, bob, rep1, cleaned(t, db.RepeatFields{Template: bobs, Starts: today, Every: 1, Unit: "day"})); !errors.Is(err, db.ErrNoRepeat) {
		t.Fatalf("got %v, want someone else's repeat refused", err)
	}
}

// A repeat with no end is built a year ahead, and topped up as time passes.
func TestTopUpRepeats(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")
	today := time.Now().UTC().Truncate(24 * time.Hour)
	weekly := db.RepeatFields{Template: hang, Starts: today, Every: 1, Unit: "week", Weekdays: []int{1, 2, 3, 4, 5, 6, 7}}
	if _, _, err := db.PutRepeat(ctx, pool, ada, rep1, cleaned(t, weekly)); err != nil {
		t.Fatal(err)
	}
	if n := len(repeatRows(t, pool, rep1)); n != 367 {
		t.Fatalf("%d days, want today and a year ahead", n)
	}

	// As if ten months went by: only ten days are left built.
	if _, err := pool.Exec(ctx, `UPDATE repeat SET built_until = $1`, today.AddDate(0, 0, 10)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM scheduled_session WHERE local_date > $1`, today.AddDate(0, 0, 10)); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := db.TopUpRepeats(ctx, pool, ada); err != nil {
			t.Fatal(err)
		}
		if n := len(repeatRows(t, pool, rep1)); n != 367 {
			t.Fatalf("%d days, want a year ahead again", n)
		}
	}
	list, err := db.ListRepeats(ctx, pool, ada)
	if err != nil || len(list) != 1 || !list[0].BuiltUntil.Equal(today.AddDate(0, 0, 366)) {
		t.Fatalf("repeats %+v, %v, want it built a year ahead", list, err)
	}
}

func TestEndRepeat(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if _, _, err := db.PutRepeat(ctx, pool, ada, rep1, cleaned(t, db.RepeatFields{Template: hang, Starts: today, Ends: ptr(today.AddDate(0, 0, 9)), Every: 1, Unit: "day"})); err != nil {
		t.Fatal(err)
	}
	var started string
	if err := pool.QueryRow(ctx, `SELECT id FROM scheduled_session WHERE local_date = $1`, today.AddDate(0, 0, 5)).Scan(&started); err != nil {
		t.Fatal(err)
	}
	run := startRun(t, ctx, pool, ada, db.RunStart{Scheduled: &started, StartedAt: time.Now()})

	if err := db.EndRepeat(ctx, pool, ada, "nope", today); !errors.Is(err, db.ErrNoRepeat) {
		t.Fatalf("got %v, want an id that is not a uuid refused", err)
	}
	if err := db.EndRepeat(ctx, pool, bob, rep1, today); !errors.Is(err, db.ErrNoRepeat) {
		t.Fatalf("got %v, want someone else's repeat refused", err)
	}

	// A day after it ends changes nothing.
	if err := db.EndRepeat(ctx, pool, ada, rep1, today.AddDate(0, 0, 20)); err != nil {
		t.Fatal(err)
	}
	if list, err := db.ListRepeats(ctx, pool, ada); err != nil || !list[0].Ends.Equal(today.AddDate(0, 0, 9)) {
		t.Fatalf("repeats %+v, %v, want it to end as before", list, err)
	}

	// This and later ones: the day with a run stays.
	if err := db.EndRepeat(ctx, pool, ada, rep1, today.AddDate(0, 0, 3)); err != nil {
		t.Fatal(err)
	}
	if got, want := repeatRows(t, pool, rep1), dates(today, 0, 1, 2, 5); !slices.Equal(got, want) {
		t.Fatalf("days %v, want %v", got, want)
	}
	list, err := db.ListRepeats(ctx, pool, ada)
	if err != nil || len(list) != 1 || !list[0].Ends.Equal(today.AddDate(0, 0, 2)) {
		t.Fatalf("repeats %+v, %v, want it to end the day before", list, err)
	}

	// From its first day, the repeat goes, and the run stays.
	if err := db.EndRepeat(ctx, pool, ada, rep1, today); err != nil {
		t.Fatal(err)
	}
	if list, err := db.ListRepeats(ctx, pool, ada); err != nil || len(list) != 0 {
		t.Fatalf("repeats %+v, %v, want none", list, err)
	}
	if got, err := db.GetRun(ctx, pool, ada, run.ID); err != nil || got.Scheduled != nil {
		t.Fatalf("run %+v, %v, want it kept with no day", got, err)
	}
}

func TestDeleteRepeat(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if _, _, err := db.PutRepeat(ctx, pool, ada, rep1, cleaned(t, db.RepeatFields{Template: hang, Starts: today, Ends: ptr(today.AddDate(0, 0, 3)), Every: 1, Unit: "day"})); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteRepeat(ctx, pool, bob, rep1); !errors.Is(err, db.ErrNoRepeat) {
		t.Fatalf("got %v, want someone else's repeat refused", err)
	}
	if err := db.DeleteRepeat(ctx, pool, ada, rep2); !errors.Is(err, db.ErrNoRepeat) {
		t.Fatalf("got %v, want a missing repeat refused", err)
	}
	if err := db.DeleteRepeat(ctx, pool, ada, rep1); err != nil {
		t.Fatal(err)
	}
	if days := repeatRows(t, pool, rep1); len(days) != 0 {
		t.Fatalf("days %v, want them gone with the repeat", days)
	}
}
