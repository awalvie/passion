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
	j1 = "0199c3a0-0000-7000-8000-0000000000f1"
	j2 = "0199c3a0-0000-7000-8000-0000000000f2"
	j3 = "0199c3a0-0000-7000-8000-0000000000f3"
)

func journalFields(t *testing.T, day, body string, run *string) db.JournalFields {
	t.Helper()
	date, err := time.Parse(time.DateOnly, day)
	if err != nil {
		t.Fatal(err)
	}
	f, problems := db.JournalFields{LocalDate: date, Body: body, Run: run}.Clean()
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	return f
}

func putEntry(t *testing.T, pool *pgxpool.Pool, owner, id string, f db.JournalFields) db.JournalEntry {
	t.Helper()
	entry, err := db.PutJournalEntry(context.Background(), pool, owner, id, f)
	if err != nil {
		t.Fatal(err)
	}
	return entry
}

func TestPutJournalEntry(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	run := startRun(t, ctx, pool, ada, db.RunStart{Name: "Boulder", StartedAt: time.Now()})
	other := startRun(t, ctx, pool, ada, db.RunStart{Name: "Strength", StartedAt: time.Now()})

	// A retry makes one entry.
	for range 2 {
		entry := putEntry(t, pool, ada, j1, journalFields(t, "2026-10-24", " Hips in. ", &run.ID))
		if entry.ID != j1 || entry.Body != "Hips in." || entry.Run == nil || *entry.Run != run.ID || entry.PinnedAt != nil ||
			entry.RunName == nil || *entry.RunName != "Boulder" {
			t.Fatalf("entry %+v", entry)
		}
	}

	// A replace changes the date and the text, and keeps the run, even when it
	// names another.
	entry := putEntry(t, pool, ada, j1, journalFields(t, "2026-10-23", "Hips in, then reach.", &other.ID))
	if entry.LocalDate.Format(time.DateOnly) != "2026-10-23" || entry.Body != "Hips in, then reach." || entry.Run == nil || *entry.Run != run.ID {
		t.Fatalf("entry %+v, want the new date and text with the run kept", entry)
	}
}

func TestPutJournalEntryRefuses(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	bobsRun := startRun(t, ctx, pool, bob, db.RunStart{Name: "Bob's", StartedAt: time.Now()})
	putEntry(t, pool, bob, j1, journalFields(t, "2026-10-24", "Bob's entry", nil))

	for name, c := range map[string]struct {
		id   string
		f    db.JournalFields
		want error
	}{
		"someone else's entry id": {j1, journalFields(t, "2026-10-24", "Mine", nil), db.ErrNoJournalEntry},
		"an id that is not uuid":  {"nope", journalFields(t, "2026-10-24", "Mine", nil), db.ErrNoJournalEntry},
		"someone else's run":      {j2, journalFields(t, "2026-10-24", "Mine", &bobsRun.ID), db.ErrNoJournalRun},
		"a run that is gone":      {j2, journalFields(t, "2026-10-24", "Mine", ptr(s1)), db.ErrNoJournalRun},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := db.PutJournalEntry(ctx, pool, ada, c.id, c.f); !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
	if entry, err := db.GetJournalEntry(ctx, pool, bob, j1); err != nil || entry.Body != "Bob's entry" {
		t.Fatalf("bob's entry %+v, %v, want it untouched", entry, err)
	}
	if _, err := db.GetJournalEntry(ctx, pool, ada, j1); !errors.Is(err, db.ErrNoJournalEntry) {
		t.Fatalf("got %v, want someone else's entry hidden", err)
	}
}

func TestPinJournalEntry(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	putEntry(t, pool, ada, j1, journalFields(t, "2026-10-24", "Hips in.", nil))

	pinned, err := db.PinJournalEntry(ctx, pool, ada, j1, true)
	if err != nil || pinned.PinnedAt == nil {
		t.Fatalf("pin: %+v, %v", pinned, err)
	}
	again, err := db.PinJournalEntry(ctx, pool, ada, j1, true)
	if err != nil || again.PinnedAt == nil || !again.PinnedAt.Equal(*pinned.PinnedAt) {
		t.Fatalf("pinned again: %+v, %v, want the first pin time kept", again, err)
	}
	// Editing the text leaves the pin alone.
	if entry := putEntry(t, pool, ada, j1, journalFields(t, "2026-10-24", "Hips in, then reach.", nil)); entry.PinnedAt == nil {
		t.Fatalf("entry %+v, want it still pinned after an edit", entry)
	}
	if entry, err := db.PinJournalEntry(ctx, pool, ada, j1, false); err != nil || entry.PinnedAt != nil {
		t.Fatalf("unpin: %+v, %v", entry, err)
	}
	if _, err := db.PinJournalEntry(ctx, pool, bob, j1, true); !errors.Is(err, db.ErrNoJournalEntry) {
		t.Fatalf("got %v, want someone else's entry refused", err)
	}
}

func TestListJournal(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	putEntry(t, pool, ada, j1, journalFields(t, "2026-10-20", "Older", nil))
	putEntry(t, pool, ada, j2, journalFields(t, "2026-10-24", "Newer day", nil))
	putEntry(t, pool, ada, j3, journalFields(t, "2026-10-24", "Same day, written later", nil))
	putEntry(t, pool, bob, s2, journalFields(t, "2026-10-24", "Bob's", nil))

	// Only a run with words in its journal is listed; scores alone are not.
	written := startRun(t, ctx, pool, ada, db.RunStart{Name: "Written", StartedAt: time.Now()})
	scored := startRun(t, ctx, pool, ada, db.RunStart{Name: "Scored", StartedAt: time.Now()})
	writeRunJournal(t, pool, ada, written, func(f *db.RunFields) { f.NextFocus = ptr("Rest the full 3 minutes") })
	writeRunJournal(t, pool, ada, scored, func(f *db.RunFields) { f.RPE = ptr(7) })
	bobs := startRun(t, ctx, pool, bob, db.RunStart{Name: "Bob's", StartedAt: time.Now()})
	writeRunJournal(t, pool, bob, bobs, func(f *db.RunFields) { f.Notes = ptr("Bob's notes") })

	entries, sessions, err := db.ListJournal(ctx, pool, ada)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[0].ID != j3 || entries[1].ID != j2 || entries[2].ID != j1 {
		t.Fatalf("entries %+v, want newest day first, then the latest written", entries)
	}
	if len(sessions) != 1 || sessions[0].Run != written.ID || *sessions[0].NextFocus != "Rest the full 3 minutes" {
		t.Fatalf("sessions %+v, want only the written run", sessions)
	}
}

func TestJournalEntryOutlivesItsRun(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	run := startRun(t, ctx, pool, ada, db.RunStart{Name: "Boulder", StartedAt: time.Now()})
	putEntry(t, pool, ada, j1, journalFields(t, "2026-10-24", "Hips in.", &run.ID))

	if err := db.DeleteRun(ctx, pool, ada, run.ID); err != nil {
		t.Fatal(err)
	}
	entry, err := db.GetJournalEntry(ctx, pool, ada, j1)
	if err != nil || entry.Run != nil || entry.Body != "Hips in." {
		t.Fatalf("entry %+v, %v, want it kept without its run", entry, err)
	}
}

func writeRunJournal(t *testing.T, pool *pgxpool.Pool, owner string, run db.Run, edit func(*db.RunFields)) {
	t.Helper()
	f := db.RunFields{Name: run.Name, StartedAt: run.StartedAt, LocalDate: run.LocalDate}
	edit(&f)
	f, problems := f.Clean()
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	if _, err := db.UpdateRun(context.Background(), pool, owner, run.ID, f); err != nil {
		t.Fatal(err)
	}
}
