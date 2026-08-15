package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"passion/db"
)

// newScheduledStartRequest builds a POST /scheduled-sessions/{scheduledID}/start
// request with chi params and auth injected.
func newScheduledStartRequest(t *testing.T, scheduledID, ownerID uint) *http.Request {
	t.Helper()
	path := fmt.Sprintf("/scheduled-sessions/%d/start", scheduledID)
	req := httptest.NewRequest(http.MethodPost, path, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("scheduledID", fmt.Sprintf("%d", scheduledID))
	rctx.URLParams.Add("action", "start")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, authUserIDKey, ownerID)
	return req.WithContext(ctx)
}

// seedScheduledSession creates a SessionTemplate → ScheduledSession with no run yet.
func seedScheduledSession(t *testing.T, store *db.Store, ownerID uint) *db.ScheduledSession {
	t.Helper()
	tpl := &db.SessionTemplate{OwnerID: ownerID, Name: "Start Template"}
	if err := store.DB.Create(tpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	ss := &db.ScheduledSession{
		OwnerID:           ownerID,
		ScheduledDate:     time.Now(),
		SessionTemplateID: tpl.ID,
	}
	if err := store.DB.Create(ss).Error; err != nil {
		t.Fatalf("create scheduled session: %v", err)
	}
	return ss
}

// TestHandleScheduledStart_CreatesRunWhenNone guards that starting a fresh
// scheduled session creates exactly one run and redirects to it.
func TestHandleScheduledStart_CreatesRunWhenNone(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "start-fresh.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	ss := seedScheduledSession(t, store, ownerID)

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleScheduledSessionsByID(rr, newScheduledStartRequest(t, ss.ID, ownerID))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}

	var count int64
	store.DB.Model(&db.SessionRun{}).Where("scheduled_session_id = ?", ss.ID).Count(&count)
	if count != 1 {
		t.Fatalf("run count = %d after first start, want 1", count)
	}

	var run db.SessionRun
	store.DB.Where("scheduled_session_id = ?", ss.ID).First(&run)
	wantRedirect := fmt.Sprintf("/runs/%d", run.ID)
	if got := rr.Header().Get("HX-Redirect"); got != wantRedirect {
		t.Errorf("HX-Redirect = %q, want %q", got, wantRedirect)
	}
}

// TestHandleScheduledStart_DoesNotDuplicateRun guards the duplicate-run fix:
// a second Start on the same scheduled session must NOT create a second run,
// and must redirect to the existing one.
func TestHandleScheduledStart_DoesNotDuplicateRun(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "start-nodup.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 2
	ss := seedScheduledSession(t, store, ownerID)

	srv := &Server{store: store}

	// First start.
	rr1 := httptest.NewRecorder()
	srv.handleScheduledSessionsByID(rr1, newScheduledStartRequest(t, ss.ID, ownerID))
	if rr1.Code != http.StatusOK {
		t.Fatalf("first start status = %d, want %d", rr1.Code, http.StatusOK)
	}
	firstRedirect := rr1.Header().Get("HX-Redirect")

	// Second start (the double-click / re-start scenario).
	rr2 := httptest.NewRecorder()
	srv.handleScheduledSessionsByID(rr2, newScheduledStartRequest(t, ss.ID, ownerID))
	if rr2.Code != http.StatusOK {
		t.Fatalf("second start status = %d, want %d", rr2.Code, http.StatusOK)
	}

	var count int64
	store.DB.Model(&db.SessionRun{}).Where("scheduled_session_id = ?", ss.ID).Count(&count)
	if count != 1 {
		t.Errorf("run count = %d after two starts, want 1 (no duplicate)", count)
	}

	if got := rr2.Header().Get("HX-Redirect"); got != firstRedirect {
		t.Errorf("second start HX-Redirect = %q, want %q (existing run)", got, firstRedirect)
	}
}

// TestHandleScheduledStart_CompletedRunNotRerun guards that starting an
// already-completed scheduled session redirects to the completed run rather
// than spawning a fresh one.
func TestHandleScheduledStart_CompletedRunNotRerun(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "start-completed.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 3
	ss := seedScheduledSession(t, store, ownerID)

	now := time.Now()
	done := &db.SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: ss.ID,
		Status:             db.RunStatusCompleted,
		StartedAt:          now.Add(-time.Hour),
		CompletedAt:        &now,
	}
	if err := store.DB.Create(done).Error; err != nil {
		t.Fatal(err)
	}

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleScheduledSessionsByID(rr, newScheduledStartRequest(t, ss.ID, ownerID))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var count int64
	store.DB.Model(&db.SessionRun{}).Where("scheduled_session_id = ?", ss.ID).Count(&count)
	if count != 1 {
		t.Errorf("run count = %d after starting a completed session, want 1 (no new run)", count)
	}

	wantRedirect := fmt.Sprintf("/runs/%d/summary", done.ID)
	if got := rr.Header().Get("HX-Redirect"); got != wantRedirect {
		t.Errorf("HX-Redirect = %q, want %q (completed run → summary)", got, wantRedirect)
	}
}

// TestHandleScheduledStart_EarlyFinishedRunGoesToSummary guards the corner the
// QA review surfaced: a run can be marked Status=completed via /runs/{id}/stop
// ("Finish this session?") before every exercise is checked off. renderRun's
// completed screen is derived from step completion (buildRunSteps +
// RunExerciseCompletion), not run.Status, so GET /runs/{id} for such a run would
// render the live in-progress player rather than a done state. The start guard
// therefore must NOT redirect a completed run to /runs/{id}; it routes completed
// runs to /runs/{id}/summary, which reflects run.Status directly and never sends
// the user back into a session they already finished.
func TestHandleScheduledStart_EarlyFinishedRunGoesToSummary(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "start-early-finish.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 4
	tpl := &db.SessionTemplate{OwnerID: ownerID, Name: "Early Finish Template"}
	if err := store.DB.Create(tpl).Error; err != nil {
		t.Fatal(err)
	}
	act := &db.Activity{OwnerID: ownerID, SessionTemplateID: tpl.ID, Type: "exercise", OrderIndex: 0}
	if err := store.DB.Create(act).Error; err != nil {
		t.Fatal(err)
	}
	ex := &db.Exercise{OwnerID: ownerID, ActivityID: &act.ID, Name: "Pull-ups", Kind: "reps_and_sets", OrderIndex: 0}
	if err := store.DB.Create(ex).Error; err != nil {
		t.Fatal(err)
	}
	ss := &db.ScheduledSession{OwnerID: ownerID, ScheduledDate: time.Now(), SessionTemplateID: tpl.ID}
	if err := store.DB.Create(ss).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	// Finished early via /runs/{id}/stop: Status=completed but the exercise was
	// never checked off (no RunExerciseCompletion row for it).
	run := &db.SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: ss.ID,
		Status:             db.RunStatusCompleted,
		StartedAt:          now.Add(-time.Hour),
		CompletedAt:        &now,
	}
	if err := store.DB.Create(run).Error; err != nil {
		t.Fatal(err)
	}

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleScheduledSessionsByID(rr, newScheduledStartRequest(t, ss.ID, ownerID))
	if rr.Code != http.StatusOK {
		t.Fatalf("start status = %d, want %d", rr.Code, http.StatusOK)
	}

	wantRedirect := fmt.Sprintf("/runs/%d/summary", run.ID)
	if got := rr.Header().Get("HX-Redirect"); got != wantRedirect {
		t.Errorf("HX-Redirect = %q, want %q; a completed run (even one finished early) must go to its "+
			"summary, not back into the live player at /runs/%d", got, wantRedirect, run.ID)
	}
}
