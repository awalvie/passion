package db_test

import (
	"context"
	"errors"
	"slices"
	"testing"

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
