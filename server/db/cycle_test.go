package db_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/db"
	"passion/server/db/dbtest"
)

const (
	y1 = "0199c3a0-0000-7000-8000-0000000000e1"
	y2 = "0199c3a0-0000-7000-8000-0000000000e2"
	y3 = "0199c3a0-0000-7000-8000-0000000000e3"
)

func day(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}

// cycleFields is a cleaned cycle. Its days lie far ahead, so every one is
// built whatever today is.
func cycleFields(t *testing.T, blockDays int, days ...db.CycleDay) db.CycleFields {
	t.Helper()
	f, problems := db.CycleFields{
		Name:      "Spring fingers",
		Starts:    day("2100-01-05"),
		Ends:      day("2100-01-18"),
		BlockDays: blockDays,
		Body:      db.CycleBody{Days: days},
	}.Clean()
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	return f
}

// scheduled lists the person's rows as "date template".
func scheduled(t *testing.T, pool *pgxpool.Pool, owner string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT to_char(local_date, 'YYYY-MM-DD') || ' ' || template FROM scheduled_session
		WHERE owner = $1 ORDER BY local_date, template`, owner)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func TestPutCycle(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")
	boulder := insertSessionTemplate(t, pool, "", "Boulder")

	f := cycleFields(t, 7, db.CycleDay{Day: 1, Template: hang}, db.CycleDay{Day: 3, Template: boulder})
	want := []string{"2100-01-05 " + hang, "2100-01-07 " + boulder, "2100-01-12 " + hang, "2100-01-14 " + boulder}

	// A retry builds the same days.
	for range 2 {
		cycle, leftOut, err := db.PutCycle(ctx, pool, ada, y1, f)
		if err != nil {
			t.Fatalf("put: %v", err)
		}
		if cycle.ID != y1 || len(leftOut) != 0 {
			t.Fatalf("cycle %+v, left out %v, want the id the client chose and nothing left out", cycle, leftOut)
		}
		if got := scheduled(t, pool, ada); !slices.Equal(got, want) {
			t.Fatalf("scheduled %v, want %v", got, want)
		}
	}

	// A rename keeps a day the person moved.
	if _, err := pool.Exec(ctx, `UPDATE scheduled_session SET local_date = '2100-01-06' WHERE local_date = '2100-01-05'`); err != nil {
		t.Fatal(err)
	}
	f.Name = "Spring fingers, again"
	if _, _, err := db.PutCycle(ctx, pool, ada, y1, f); err != nil {
		t.Fatal(err)
	}
	if got := scheduled(t, pool, ada); got[0] != "2100-01-06 "+hang {
		t.Fatalf("scheduled %v, want the moved day kept", got)
	}

	// Goals and notes keep it too, and come back as written.
	notes := "Keep pull-ups strict"
	f.Goals = []db.Goal{{Text: "Flash 7a", Done: true, Before: "6c", After: "7a", How: "Board twice a week"}}
	f.Notes = &notes
	if _, _, err := db.PutCycle(ctx, pool, ada, y1, f); err != nil {
		t.Fatal(err)
	}
	if got := scheduled(t, pool, ada); got[0] != "2100-01-06 "+hang {
		t.Fatalf("scheduled %v, want the moved day kept", got)
	}
	got, err := db.GetCycle(ctx, pool, ada, y1)
	if err != nil || !slices.Equal(got.Goals, f.Goals) || *got.Notes != notes {
		t.Fatalf("cycle %+v, %v, want the goals and notes written", got, err)
	}

	// A new shape builds again, and leaves out a day that already holds the
	// session.
	if _, err := pool.Exec(ctx, `INSERT INTO scheduled_session (owner, template, local_date) VALUES ($1, $2, '2100-01-15')`,
		ada, hang); err != nil {
		t.Fatal(err)
	}
	f = cycleFields(t, 10, db.CycleDay{Day: 1, Template: hang})
	_, leftOut, err := db.PutCycle(ctx, pool, ada, y1, f)
	if err != nil {
		t.Fatal(err)
	}
	if len(leftOut) != 1 || !leftOut[0].LocalDate.Equal(day("2100-01-15")) {
		t.Fatalf("left out %v, want the 15th", leftOut)
	}
	if got := scheduled(t, pool, ada); !slices.Equal(got, []string{"2100-01-05 " + hang, "2100-01-15 " + hang}) {
		t.Fatalf("scheduled %v, want the new shape beside the one-off", got)
	}
}

func TestPutCycleNeverPlansThePast(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")

	f := cycleFields(t, 1, db.CycleDay{Day: 1, Template: hang})
	f.Starts, f.Ends = day("2020-01-01"), day("2020-01-10")
	if _, _, err := db.PutCycle(ctx, pool, ada, y1, f); err != nil {
		t.Fatal(err)
	}
	if got := scheduled(t, pool, ada); len(got) != 0 {
		t.Fatalf("scheduled %v, want nothing for days gone", got)
	}
}

func TestPutCycleKeepsABegunStart(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")

	// Not begun yet: the start can move.
	f := cycleFields(t, 7)
	if _, _, err := db.PutCycle(ctx, pool, ada, y1, f); err != nil {
		t.Fatal(err)
	}
	f.Starts = day("2100-01-06")
	if _, _, err := db.PutCycle(ctx, pool, ada, y1, f); err != nil {
		t.Fatalf("moved a start not begun: %v", err)
	}

	if got, _ := db.GetCycle(ctx, pool, ada, y1); !got.BlockFrom.Equal(day("2100-01-06")) {
		t.Fatalf("block from %v, want the new start", got.BlockFrom)
	}

	// An ended cycle keeps where its block counts from.
	ended := cycleFields(t, 7)
	ended.Starts, ended.Ends = day("2020-01-01"), day("2020-01-28")
	if _, _, err := db.PutCycle(ctx, pool, ada, y3, ended); err != nil {
		t.Fatal(err)
	}
	ended.BlockDays = 6
	if got, _, err := db.PutCycle(ctx, pool, ada, y3, ended); err != nil || !got.BlockFrom.Equal(ended.Starts) {
		t.Fatalf("block from %v, %v, want the start", got.BlockFrom, err)
	}

	begun := cycleFields(t, 7)
	begun.Starts, begun.Ends = day("2020-01-01"), day("2020-12-30")
	if _, _, err := db.PutCycle(ctx, pool, ada, y2, begun); err != nil {
		t.Fatal(err)
	}
	begun.Starts = day("2020-01-02")
	if _, _, err := db.PutCycle(ctx, pool, ada, y2, begun); !errors.Is(err, db.ErrStartsLocked) {
		t.Fatalf("got %v, want the start kept", err)
	}
	if got, _ := db.GetCycle(ctx, pool, ada, y2); !got.Starts.Equal(day("2020-01-01")) {
		t.Fatalf("starts %v, want it unchanged", got.Starts)
	}
}

// Once the cycle has begun, a new block length makes today day 1.
func TestPutCycleNewBlockFromToday(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")
	var today time.Time
	if err := pool.QueryRow(ctx, `SELECT (now() AT TIME ZONE timezone)::date FROM account WHERE id = $1`, ada).Scan(&today); err != nil {
		t.Fatal(err)
	}
	at := func(n int) string { return today.AddDate(0, 0, n).Format(time.DateOnly) + " " + hang }

	f := cycleFields(t, 4, db.CycleDay{Day: 1, Template: hang})
	f.Starts, f.Ends = today.AddDate(0, 0, -1), today.AddDate(0, 0, 8)
	if _, _, err := db.PutCycle(ctx, pool, ada, y1, f); err != nil {
		t.Fatal(err)
	}
	if got := scheduled(t, pool, ada); !slices.Equal(got, []string{at(3), at(7)}) {
		t.Fatalf("scheduled %v, want days 1 from the start", got)
	}

	f.BlockDays = 3
	cycle, _, err := db.PutCycle(ctx, pool, ada, y1, f)
	if err != nil {
		t.Fatal(err)
	}
	if !cycle.BlockFrom.Equal(today) {
		t.Fatalf("block from %v, want today", cycle.BlockFrom)
	}
	if got := scheduled(t, pool, ada); !slices.Equal(got, []string{at(0), at(3), at(6)}) {
		t.Fatalf("scheduled %v, want days 1 from today", got)
	}

	// The start stays locked even when the block changes with it.
	moved := f
	moved.Starts, moved.BlockDays = today, 2
	if _, _, err := db.PutCycle(ctx, pool, ada, y1, moved); !errors.Is(err, db.ErrStartsLocked) {
		t.Fatalf("got %v, want the start kept", err)
	}

	// A rename keeps the block where it is.
	f.Name = "Renamed"
	if cycle, _, err = db.PutCycle(ctx, pool, ada, y1, f); err != nil || !cycle.BlockFrom.Equal(today) {
		t.Fatalf("block from %v, %v, want it kept", cycle.BlockFrom, err)
	}
}

func TestPutCycleRefuses(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	bobs := insertSessionTemplate(t, pool, bob, "Bob's")
	if _, _, err := db.PutCycle(ctx, pool, bob, y1, cycleFields(t, 7)); err != nil {
		t.Fatal(err)
	}

	var unknown *db.UnknownTemplatesError
	for name, c := range map[string]struct {
		id   string
		f    db.CycleFields
		want func(error) bool
	}{
		"someone else's template": {y2, cycleFields(t, 7, db.CycleDay{Day: 1, Template: bobs}), func(err error) bool { return errors.As(err, &unknown) }},
		"someone else's cycle id": {y1, cycleFields(t, 7), func(err error) bool { return errors.Is(err, db.ErrNoCycle) }},
		"an id that is not uuid":  {"nope", cycleFields(t, 7), func(err error) bool { return errors.Is(err, db.ErrNoCycle) }},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := db.PutCycle(ctx, pool, ada, c.id, c.f); !c.want(err) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if got, err := db.GetCycle(ctx, pool, bob, y1); err != nil || got.Name != "Spring fingers" {
		t.Fatalf("bob's cycle %+v, %v, want it untouched", got, err)
	}
}

func TestDeleteCycle(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")
	if _, _, err := db.PutCycle(ctx, pool, ada, y1, cycleFields(t, 7, db.CycleDay{Day: 1, Template: hang})); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO scheduled_session (owner, template, local_date) VALUES ($1, $2, '2100-01-06')`,
		ada, hang); err != nil {
		t.Fatal(err)
	}

	if err := db.DeleteCycle(ctx, pool, ada, y1); err != nil {
		t.Fatal(err)
	}
	if got := scheduled(t, pool, ada); !slices.Equal(got, []string{"2100-01-06 " + hang}) {
		t.Fatalf("scheduled %v, want only the one-off", got)
	}
	if err := db.DeleteCycle(ctx, pool, ada, y1); !errors.Is(err, db.ErrNoCycle) {
		t.Fatalf("deleted twice: %v", err)
	}
	if list, err := db.ListCycles(ctx, pool, ada); err != nil || len(list) != 0 {
		t.Fatalf("list %v, %v, want none", list, err)
	}
}

func TestCleanCycleFields(t *testing.T) {
	for name, c := range map[string]struct {
		f    db.CycleFields
		want []string
	}{
		"ends before it starts": {
			db.CycleFields{Name: "C", Starts: day("2026-03-10"), Ends: day("2026-03-09"), BlockDays: 1},
			[]string{"ends"},
		},
		"longer than a year": {
			db.CycleFields{Name: "C", Starts: day("2026-01-01"), Ends: day("2027-01-02"), BlockDays: 7},
			[]string{"ends"},
		},
		"a block longer than the cycle": {
			db.CycleFields{Name: "C", Starts: day("2026-03-01"), Ends: day("2026-03-05"), BlockDays: 7},
			[]string{"block_days"},
		},
		"a goal too long": {
			db.CycleFields{Name: "C", Starts: day("2026-03-01"), Ends: day("2026-03-07"), BlockDays: 7, Goals: []db.Goal{
				{Text: " "}, {Text: strings.Repeat("a", 201)},
			}},
			[]string{"goals[1]"},
		},
		"a goal's how too long": {
			db.CycleFields{Name: "C", Starts: day("2026-03-01"), Ends: day("2026-03-07"), BlockDays: 7, Goals: []db.Goal{
				{Text: "Flash 7a", How: strings.Repeat("a", 501)},
			}},
			[]string{"goals[0].how"},
		},
		"bad days": {
			db.CycleFields{Name: "", Starts: day("2026-03-01"), Ends: day("2026-03-28"), BlockDays: 7, Body: db.CycleBody{Days: []db.CycleDay{
				{Day: 8, Template: e1}, {Day: 1, Template: "nope"}, {Day: 2, Template: e1}, {Day: 2, Template: e1},
			}}},
			[]string{"days[0].day", "days[1].template", "days[3]", "name"},
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

func TestCleanCycleDropsBlankLines(t *testing.T) {
	f, problems := db.CycleFields{
		Name: "C", Starts: day("2026-03-01"), Ends: day("2026-03-07"), BlockDays: 7,
		Goals:  []db.Goal{{Text: "  "}, {Text: " Flash 7a "}},
	}.Clean()
	if len(problems) != 0 || !slices.Equal(f.Goals, []db.Goal{{Text: "Flash 7a"}}) {
		t.Fatalf("fields %+v, problems %v", f, problems)
	}
}

// A second create of one id, sent while the first is in flight, waits for it
// and keeps the days it wrote.
func TestPutCycleTwiceAtOnce(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")
	f := cycleFields(t, 7, db.CycleDay{Day: 1, Template: hang})

	first, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Rollback(ctx)
	if _, err := first.Exec(ctx, `
		INSERT INTO cycle (id, owner, name, starts, ends, block_days, block_from, body)
		VALUES ($1, $2, $3, $4, $5, $6, $4, $7)`, y1, ada, f.Name, f.Starts, f.Ends, f.BlockDays, f.Body); err != nil {
		t.Fatal(err)
	}
	var day string
	if err := first.QueryRow(ctx, `
		INSERT INTO scheduled_session (owner, cycle, template, local_date) VALUES ($1, $2, $3, '2100-01-05')
		RETURNING id`, ada, y1, hang).Scan(&day); err != nil {
		t.Fatal(err)
	}

	second := make(chan error, 1)
	go func() {
		_, _, err := db.PutCycle(ctx, pool, ada, y1, f)
		second <- err
	}()
	for waiting := 0; waiting == 0; {
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock'`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-second:
			t.Fatalf("the second create finished before the first: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err := first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}

	var kept bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM scheduled_session WHERE id = $1)`, day).Scan(&kept); err != nil || !kept {
		t.Fatalf("the first create's day is gone (%v), want it kept", err)
	}
}
