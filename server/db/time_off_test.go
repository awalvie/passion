package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/db"
	"passion/server/db/dbtest"
)

const (
	off1 = "0199c3a0-0000-7000-8000-000000000a01"
	off2 = "0199c3a0-0000-7000-8000-000000000a02"
	off3 = "0199c3a0-0000-7000-8000-000000000a03"
	off4 = "0199c3a0-0000-7000-8000-000000000a04"
)

func timeOffFields(t *testing.T, name, starts string) db.TimeOffFields {
	t.Helper()
	f, problems := db.TimeOffFields{Kind: "trip", Name: name, Starts: day(starts), Pauses: true}.Clean()
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	return f
}

func putTimeOff(t *testing.T, pool *pgxpool.Pool, owner, id string, f db.TimeOffFields) {
	t.Helper()
	if _, err := db.PutTimeOff(context.Background(), pool, owner, id, f); err != nil {
		t.Fatal(err)
	}
}

func TestTimeOffClean(t *testing.T) {
	for name, c := range map[string]struct {
		f     db.TimeOffFields
		field string
	}{
		"an unknown kind":     {db.TimeOffFields{Kind: "holiday", Name: "Away", Starts: day("2026-11-07")}, "kind"},
		"no name":             {db.TimeOffFields{Kind: "trip", Name: " ", Starts: day("2026-11-07")}, "name"},
		"an end before start": {db.TimeOffFields{Kind: "trip", Name: "Away", Starts: day("2026-11-07"), Ends: ptr(day("2026-11-06"))}, "ends"},
		"one day":             {db.TimeOffFields{Kind: "trip", Name: "Away", Starts: day("2026-11-07"), Ends: ptr(day("2026-11-07"))}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			_, problems := c.f.Clean()
			if c.field == "" && len(problems) != 0 {
				t.Fatalf("problems %v, want none", problems)
			}
			if c.field != "" && problems[c.field] == "" {
				t.Fatalf("problems %v, want %s named", problems, c.field)
			}
		})
	}
}

func TestListTimeOff(t *testing.T) {
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	putTimeOff(t, pool, ada, off1, timeOffFields(t, "Older", "2026-11-01"))
	putTimeOff(t, pool, ada, off3, timeOffFields(t, "Newer", "2026-11-07"))
	putTimeOff(t, pool, ada, off2, timeOffFields(t, "Same start", "2026-11-07"))
	putTimeOff(t, pool, bob, off4, timeOffFields(t, "Bob's", "2026-11-07"))

	list, err := db.ListTimeOff(context.Background(), pool, ada)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].ID != off2 || list[1].ID != off3 || list[2].ID != off1 {
		t.Fatalf("time off %+v, want the latest start first, then by id", list)
	}
}

func TestPutTimeOffKeepsOthersApart(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	putTimeOff(t, pool, bob, off1, timeOffFields(t, "Bob's", "2026-11-07"))

	if _, err := db.PutTimeOff(ctx, pool, ada, off1, timeOffFields(t, "Mine", "2026-11-07")); !errors.Is(err, db.ErrNoTimeOff) {
		t.Fatalf("put: got %v, want someone else's id refused", err)
	}
	if err := db.DeleteTimeOff(ctx, pool, ada, off1); !errors.Is(err, db.ErrNoTimeOff) {
		t.Fatalf("delete: got %v, want someone else's time off refused", err)
	}
	if got, err := db.GetTimeOff(ctx, pool, bob, off1); err != nil || got.Name != "Bob's" {
		t.Fatalf("bob's %+v, %v, want it untouched", got, err)
	}
}
