package db_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"passion/server/db"
	"passion/server/db/dbtest"
)

func TestScheduleSession(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")
	boulder := insertSessionTemplate(t, pool, "", "Boulder")

	one, err := db.ScheduleSession(ctx, pool, ada, hang, day("2100-01-05"))
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	if one.Cycle != nil || one.Template != hang {
		t.Fatalf("scheduled %+v, want a one-off of Hang", one)
	}
	if _, err := db.ScheduleSession(ctx, pool, ada, boulder, day("2100-01-05")); err != nil {
		t.Fatalf("a second session on the day: %v", err)
	}
	if _, err := db.ScheduleSession(ctx, pool, ada, hang, day("2100-01-05")); !errors.Is(err, db.ErrAlreadyScheduled) {
		t.Fatalf("the same session twice on one day: %v", err)
	}
	if _, err := db.ScheduleSession(ctx, pool, bob, hang, day("2100-01-05")); !errors.Is(err, db.ErrNoSessionTemplate) {
		t.Fatalf("someone else's template: %v", err)
	}

	moved, err := db.MoveScheduledSession(ctx, pool, ada, one.ID, day("2100-01-06"))
	if err != nil {
		t.Fatal(err)
	}
	if !moved.LocalDate.Equal(day("2100-01-06")) {
		t.Fatalf("moved to %v, want the 6th", moved.LocalDate)
	}
	if _, err := db.ScheduleSession(ctx, pool, ada, hang, day("2100-01-05")); err != nil {
		t.Fatalf("the day it left: %v", err)
	}
	if _, err := db.MoveScheduledSession(ctx, pool, ada, one.ID, day("2100-01-05")); !errors.Is(err, db.ErrAlreadyScheduled) {
		t.Fatalf("moved onto a day that holds it: %v", err)
	}
	if _, err := db.MoveScheduledSession(ctx, pool, bob, one.ID, day("2100-01-07")); !errors.Is(err, db.ErrNoScheduledSession) {
		t.Fatalf("someone else moved it: %v", err)
	}

	if err := db.DeleteScheduledSession(ctx, pool, ada, one.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteScheduledSession(ctx, pool, ada, one.ID); !errors.Is(err, db.ErrNoScheduledSession) {
		t.Fatalf("deleted twice: %v", err)
	}
	want := []string{"2100-01-05 " + boulder, "2100-01-05 " + hang}
	slices.Sort(want)
	if got := scheduled(t, pool, ada); !slices.Equal(got, want) {
		t.Fatalf("scheduled %v, want %v", got, want)
	}
}

func TestStartRunFromScheduledSession(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")
	planned, err := db.ScheduleSession(ctx, pool, ada, hang, day("2100-01-05"))
	if err != nil {
		t.Fatal(err)
	}

	run := startRun(t, ctx, pool, ada, db.RunStart{Scheduled: &planned.ID, StartedAt: time.Now()})
	if run.Template == nil || *run.Template != hang || run.Scheduled == nil || *run.Scheduled != planned.ID || run.Name != "Hang" {
		t.Fatalf("run %+v, want a copy of Hang that points at its day", run)
	}
	if _, err := db.StartRun(ctx, pool, bob, db.RunStart{Scheduled: &planned.ID, StartedAt: time.Now()}); !errors.Is(err, db.ErrNoScheduledSession) {
		t.Fatalf("someone else's day: %v", err)
	}
	if _, problems := (db.RunStart{Scheduled: &planned.ID, Template: &hang}).Clean(); problems["template"] == "" {
		t.Fatal("took a template beside a scheduled session")
	}

	// Taking the day out keeps the run.
	if err := db.DeleteScheduledSession(ctx, pool, ada, planned.ID); err != nil {
		t.Fatal(err)
	}
	read, err := db.GetRun(ctx, pool, ada, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Scheduled != nil {
		t.Fatalf("scheduled %v, want the link cleared", *read.Scheduled)
	}
}

// A new shape never drops a day a run was started from.
func TestPutCycleKeepsADayWithARun(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")
	if _, _, err := db.PutCycle(ctx, pool, ada, y1, cycleFields(t, 7, db.CycleDay{Day: 1, Template: hang})); err != nil {
		t.Fatal(err)
	}
	var first string
	if err := pool.QueryRow(ctx, `SELECT id FROM scheduled_session WHERE local_date = '2100-01-05'`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	startRun(t, ctx, pool, ada, db.RunStart{Scheduled: &first, StartedAt: time.Now()})

	_, leftOut, err := db.PutCycle(ctx, pool, ada, y1, cycleFields(t, 14, db.CycleDay{Day: 1, Template: hang}))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftOut) != 0 {
		t.Fatalf("left out %v, want the kept day counted as the cycle's own", leftOut)
	}
	var kept string
	if err := pool.QueryRow(ctx, `SELECT id FROM scheduled_session WHERE local_date = '2100-01-05'`).Scan(&kept); err != nil || kept != first {
		t.Fatalf("the day with a run is %q (%v), want %q kept", kept, err, first)
	}
}

func TestListScheduledSessions(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")
	boulder := insertSessionTemplate(t, pool, "", "Boulder")

	today := time.Now().UTC().Truncate(24 * time.Hour)
	yesterday, tomorrow := today.AddDate(0, 0, -1), today.AddDate(0, 0, 1)
	schedule := func(template string, d time.Time) db.ScheduledSession {
		t.Helper()
		s, err := db.ScheduleSession(ctx, pool, ada, template, d)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	missed := schedule(hang, yesterday.AddDate(0, 0, -1))
	done := schedule(hang, yesterday)
	started := schedule(boulder, today)
	planned := schedule(hang, tomorrow)
	schedule(hang, tomorrow.AddDate(0, 0, 1))
	if _, err := db.ScheduleSession(ctx, pool, bob, boulder, today); err != nil {
		t.Fatal(err)
	}

	run := startRun(t, ctx, pool, ada, db.RunStart{Scheduled: &done.ID, StartedAt: time.Now()})
	if _, err := db.FinishRun(ctx, pool, ada, run.ID); err != nil {
		t.Fatal(err)
	}
	startRun(t, ctx, pool, ada, db.RunStart{Scheduled: &started.ID, StartedAt: time.Now()})

	list, err := db.ListScheduledSessions(ctx, pool, ada, missed.LocalDate, planned.LocalDate)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range list {
		got = append(got, d.TemplateName+" "+d.Status)
	}
	// The zone is UTC, so today is the server's.
	want := []string{"Hang missed", "Hang done", "Boulder started", "Hang planned"}
	if !slices.Equal(got, want) {
		t.Fatalf("days %v, want %v", got, want)
	}
	if list[1].Run == nil || *list[1].Run != run.ID {
		t.Fatalf("run %v, want the finished one", list[1].Run)
	}
}

// A rebuild that meets a run starting from one of its days, in flight, must
// wait for it and keep the day.
func TestPutCycleWaitsForARunStarting(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")
	if _, _, err := db.PutCycle(ctx, pool, ada, y1, cycleFields(t, 7, db.CycleDay{Day: 1, Template: hang})); err != nil {
		t.Fatal(err)
	}
	var day string
	if err := pool.QueryRow(ctx, `SELECT id FROM scheduled_session WHERE local_date = '2100-01-05'`).Scan(&day); err != nil {
		t.Fatal(err)
	}

	starting, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer starting.Rollback(ctx)
	var run string
	if err := starting.QueryRow(ctx, `
		INSERT INTO run (owner, scheduled, name, body, started_at, timezone, local_date)
		VALUES ($1, $2, 'Hang', '{"sections": []}', now(), 'UTC', '2100-01-05')
		RETURNING id`, ada, day).Scan(&run); err != nil {
		t.Fatal(err)
	}

	rebuilt := make(chan error, 1)
	go func() {
		_, _, err := db.PutCycle(ctx, pool, ada, y1, cycleFields(t, 14, db.CycleDay{Day: 1, Template: hang}))
		rebuilt <- err
	}()
	for waiting := 0; waiting == 0; {
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock'`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-rebuilt:
			t.Fatalf("the rebuild finished before the run did: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err := starting.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-rebuilt; err != nil {
		t.Fatal(err)
	}

	var scheduled *string
	if err := pool.QueryRow(ctx, `SELECT scheduled::text FROM run WHERE id = $1`, run).Scan(&scheduled); err != nil {
		t.Fatal(err)
	}
	if scheduled == nil || *scheduled != day {
		t.Fatalf("the run points at %v, want the day it started from kept", scheduled)
	}
}
