package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"passion/db"
)

// postSessionNotes builds and runs a POST /runs/{runID}/session-notes request.
func postSessionNotes(t *testing.T, srv *Server, runID, ownerID uint, notes string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"session_notes": {notes}}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/runs/%d/session-notes", runID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("runID", fmt.Sprintf("%d", runID))
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, authUserIDKey, ownerID)
	rr := httptest.NewRecorder()
	srv.handleRunSessionNotes(rr, req.WithContext(ctx))
	return rr
}

// TestHandleRunSessionNotes_PreservesJournalFields guards the partial-overwrite trap:
// saving session notes must NOT clobber the structured reflection fields (WentWell, RPE…).
func TestHandleRunSessionNotes_PreservesJournalFields(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "session-notes.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	run := seedOpenRun(t, store, ownerID)
	srv := &Server{store: store}

	// Seed a full journal with reflection fields populated.
	rid := run.ID
	if err := db.UpsertSessionJournal(store.DB, &db.SessionJournal{
		OwnerID:  ownerID,
		RunID:    &rid,
		RPE:      7,
		WentWell: "sent the project",
	}); err != nil {
		t.Fatal(err)
	}

	rr := postSessionNotes(t, srv, run.ID, ownerID, "felt strong overall")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}

	j, err := db.GetSessionJournalByRunID(store.DB, ownerID, run.ID)
	if err != nil || j == nil {
		t.Fatalf("journal lookup failed: j=%v err=%v", j, err)
	}
	if j.SessionNotes != "felt strong overall" {
		t.Errorf("SessionNotes = %q, want %q", j.SessionNotes, "felt strong overall")
	}
	if j.RPE != 7 {
		t.Errorf("RPE = %d, want 7 (must be preserved)", j.RPE)
	}
	if j.WentWell != "sent the project" {
		t.Errorf("WentWell = %q, want preserved", j.WentWell)
	}
}

// TestHandleRunSessionNotes_CreatesJournalIfAbsent verifies notes autosave works on a
// running open session that has no journal row yet (journals are only created on Finish).
func TestHandleRunSessionNotes_CreatesJournalIfAbsent(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "session-notes-create.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	run := seedOpenRun(t, store, ownerID)
	srv := &Server{store: store}

	if j, _ := db.GetSessionJournalByRunID(store.DB, ownerID, run.ID); j != nil {
		t.Fatal("expected no journal before autosave")
	}

	rr := postSessionNotes(t, srv, run.ID, ownerID, "first note")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	j, err := db.GetSessionJournalByRunID(store.DB, ownerID, run.ID)
	if err != nil || j == nil {
		t.Fatalf("journal not created: j=%v err=%v", j, err)
	}
	if j.SessionNotes != "first note" {
		t.Errorf("SessionNotes = %q, want %q", j.SessionNotes, "first note")
	}
}

// TestHandleRunSessionNotes_RejectsOtherOwnersRun guards owner scoping: a user posting
// notes to a run that belongs to a different owner must get 404, not write into it.
func TestHandleRunSessionNotes_RejectsOtherOwnersRun(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "session-notes-scope.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerA, ownerB uint = 1, 2
	run := seedOpenRun(t, store, ownerA)
	srv := &Server{store: store}

	rr := postSessionNotes(t, srv, run.ID, ownerB, "sneaky note")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d for cross-owner run", rr.Code, http.StatusNotFound)
	}

	if j, _ := db.GetSessionJournalByRunID(store.DB, ownerA, run.ID); j != nil {
		t.Fatalf("owner A's run should not have gained a journal from owner B's request: %+v", j)
	}
	if j, _ := db.GetSessionJournalByRunID(store.DB, ownerB, run.ID); j != nil {
		t.Fatalf("owner B should not be able to create a journal against another owner's run: %+v", j)
	}
}

// TestRenderRun_OpenSessionInProgressDoesNotRedirect guards against the new open-session
// "all done" redirect (runs.go) over-firing: jumping to a specific pending exercise via
// ?exercise=ID on an open run that still has incomplete exercises must render the player
// normally, not redirect to the overview.
func TestRenderRun_OpenSessionInProgressDoesNotRedirect(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "open-inprogress.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	run := seedOpenRun(t, store, ownerID)

	rid := run.ID
	done := &db.Exercise{OwnerID: ownerID, SessionRunID: &rid, Name: "Hangs", Kind: "climbing", OrderIndex: 0}
	pending := &db.Exercise{OwnerID: ownerID, SessionRunID: &rid, Name: "Pull Ups", Kind: "reps_and_sets", OrderIndex: 1}
	if err := store.DB.Create(done).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(pending).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&db.RunExerciseCompletion{
		OwnerID: ownerID, RunID: run.ID, ExerciseID: done.ID,
		Status: db.RunStatusCompleted, CompletedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(store, "secret", 24, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/runs/%d?exercise=%d", run.ID, pending.ID), nil)
	req = req.WithContext(context.WithValue(req.Context(), authUserIDKey, ownerID))

	srv.renderRun(rr, req, run.ID, ownerID)

	if rr.Code == http.StatusSeeOther {
		t.Fatalf("unexpected redirect for an open session still in progress; Location=%q", rr.Header().Get("Location"))
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (render player)", rr.Code, http.StatusOK)
	}
}

// TestRenderRun_OpenSessionAllDoneRedirectsToOverview guards #4: reloading the player
// URL (?exercise=X) on an open run whose every exercise is logged must redirect back to
// the open-session overview, not render the guided "Workout complete" screen.
func TestRenderRun_OpenSessionAllDoneRedirectsToOverview(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "open-alldone.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	run := seedOpenRun(t, store, ownerID)

	// One open-session exercise, already completed.
	rid := run.ID
	ex := &db.Exercise{OwnerID: ownerID, SessionRunID: &rid, Name: "Hangs", Kind: "climbing", OrderIndex: 0}
	if err := store.DB.Create(ex).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&db.RunExerciseCompletion{
		OwnerID:     ownerID,
		RunID:       run.ID,
		ExerciseID:  ex.ID,
		Status:      db.RunStatusCompleted,
		CompletedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	// Simulate GET /runs/{id}?exercise={exID} (the player URL after a reload).
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/runs/%d?exercise=%d", run.ID, ex.ID), nil)
	req = req.WithContext(context.WithValue(req.Context(), authUserIDKey, ownerID))

	srv.renderRun(rr, req, run.ID, ownerID)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d (redirect); body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}
	want := fmt.Sprintf("/runs/%d", run.ID)
	if got := rr.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}
