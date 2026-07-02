package db

import (
	"path/filepath"
	"testing"
)

// TestUpdateSessionNotes_CreatesWhenAbsent covers the plain create path: no journal
// exists yet for the run, so UpdateSessionNotes must create one with just SessionNotes set.
func TestUpdateSessionNotes_CreatesWhenAbsent(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "notes-create.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID, runID uint = 1, 42

	if err := UpdateSessionNotes(store.DB, ownerID, runID, "first note"); err != nil {
		t.Fatalf("UpdateSessionNotes: %v", err)
	}
	j, err := GetSessionJournalByRunID(store.DB, ownerID, runID)
	if err != nil || j == nil {
		t.Fatalf("expected journal to exist: j=%v err=%v", j, err)
	}
	if j.SessionNotes != "first note" {
		t.Errorf("SessionNotes = %q, want %q", j.SessionNotes, "first note")
	}
}

// TestUpdateSessionNotes_UpdatesOnlyNotesColumn covers the update path: an existing
// journal with structured reflection fields must keep them untouched.
func TestUpdateSessionNotes_UpdatesOnlyNotesColumn(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "notes-update.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID, runID uint = 1, 42
	rid := runID
	if err := UpsertSessionJournal(store.DB, &SessionJournal{
		OwnerID: ownerID, RunID: &rid, RPE: 8, WentWell: "good session",
	}); err != nil {
		t.Fatal(err)
	}

	if err := UpdateSessionNotes(store.DB, ownerID, runID, "updated note"); err != nil {
		t.Fatalf("UpdateSessionNotes: %v", err)
	}
	j, err := GetSessionJournalByRunID(store.DB, ownerID, runID)
	if err != nil || j == nil {
		t.Fatalf("expected journal to exist: j=%v err=%v", j, err)
	}
	if j.SessionNotes != "updated note" {
		t.Errorf("SessionNotes = %q, want %q", j.SessionNotes, "updated note")
	}
	if j.RPE != 8 || j.WentWell != "good session" {
		t.Errorf("structured fields clobbered: RPE=%d WentWell=%q", j.RPE, j.WentWell)
	}
}

// TestUpdateSessionNotes_AfterJournalSoftDeleted is a regression test for a real gap:
// SessionJournal.RunID has a uniqueIndex, and DeleteSessionJournal only soft-deletes
// (gorm.Model's default Delete behavior), leaving the deleted row occupying that unique
// slot. GetSessionJournalByRunID correctly reports "no journal" after a soft-delete
// (gorm's default scope excludes deleted_at rows), so UpdateSessionNotes falls into its
// create branch — which then hits "UNIQUE constraint failed: session_journals.run_id"
// because the old soft-deleted row is still there.
//
// This currently FAILS, documenting the bug: a user who deletes a training-log entry
// and then reopens the same run and types in the session-notes box gets a 500.
func TestUpdateSessionNotes_AfterJournalSoftDeleted(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "notes-softdeleted.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID, runID uint = 1, 42
	rid := runID
	first := &SessionJournal{OwnerID: ownerID, RunID: &rid, SessionNotes: "first"}
	if err := store.DB.Create(first).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := DeleteSessionJournal(store.DB, ownerID, first.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if j, _ := GetSessionJournalByRunID(store.DB, ownerID, runID); j != nil {
		t.Fatalf("expected no visible journal after delete, got %+v", j)
	}

	if err := UpdateSessionNotes(store.DB, ownerID, runID, "second"); err != nil {
		t.Fatalf("UpdateSessionNotes after soft-delete: %v (known gap: soft-deleted row still holds the unique run_id slot)", err)
	}
	j, err := GetSessionJournalByRunID(store.DB, ownerID, runID)
	if err != nil || j == nil {
		t.Fatalf("expected a new journal to exist: j=%v err=%v", j, err)
	}
	if j.SessionNotes != "second" {
		t.Errorf("SessionNotes = %q, want %q", j.SessionNotes, "second")
	}
}
