package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"passion/db"
)

// newRunStopRequest builds a POST /runs/{runID}/stop request with chi params and auth injected.
func newRunStopRequest(t *testing.T, runID uint, ownerID uint) *http.Request {
	t.Helper()
	path := fmt.Sprintf("/runs/%d/stop", runID)
	req := httptest.NewRequest(http.MethodPost, path, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("runID", fmt.Sprintf("%d", runID))
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, authUserIDKey, ownerID)
	return req.WithContext(ctx)
}

// seedGuidedRun creates the minimal records needed to have a running guided (non-open) run:
// a SessionTemplate → ScheduledSession → SessionRun with status=running.
func seedGuidedRun(t *testing.T, store *db.Store, ownerID uint) *db.SessionRun {
	t.Helper()
	tpl := &db.SessionTemplate{OwnerID: ownerID, Name: "Test Template"}
	if err := store.DB.Create(tpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	ss := &db.ScheduledSession{
		OwnerID:           ownerID,
		IsTrial:           true,
		ScheduledDate:     time.Now(),
		SessionTemplateID: tpl.ID,
	}
	if err := store.DB.Create(ss).Error; err != nil {
		t.Fatalf("create scheduled session: %v", err)
	}
	run := &db.SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: ss.ID,
		IsTrial:            true,
		IsOpen:             false,
		Status:             db.RunStatusRunning,
		StartedAt:          time.Now(),
	}
	if err := store.DB.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
	return run
}

// seedOpenRun is like seedGuidedRun but sets IsOpen=true.
func seedOpenRun(t *testing.T, store *db.Store, ownerID uint) *db.SessionRun {
	t.Helper()
	tpl := &db.SessionTemplate{OwnerID: ownerID, Name: "Open Template", IsSystem: true}
	if err := store.DB.Create(tpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	ss := &db.ScheduledSession{
		OwnerID:           ownerID,
		IsTrial:           true,
		ScheduledDate:     time.Now(),
		SessionTemplateID: tpl.ID,
	}
	if err := store.DB.Create(ss).Error; err != nil {
		t.Fatalf("create scheduled session: %v", err)
	}
	run := &db.SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: ss.ID,
		IsTrial:            true,
		IsOpen:             true,
		Status:             db.RunStatusRunning,
		StartedAt:          time.Now(),
	}
	if err := store.DB.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
	return run
}

// ---------------------------------------------------------------------------
// handleRunStop
// ---------------------------------------------------------------------------

// TestHandleRunStop_GuidedRunRedirectsToSummary guards the new always-redirect-to-summary
// behaviour: POST /runs/{id}/stop on a running guided run must set HX-Redirect to
// /runs/{id}/summary, mark the run completed, and ensure a SessionJournal exists.
func TestHandleRunStop_GuidedRunRedirectsToSummary(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "stop-guided.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	run := seedGuidedRun(t, store, ownerID)

	srv := &Server{store: store}
	req := newRunStopRequest(t, run.ID, ownerID)
	rr := httptest.NewRecorder()

	srv.handleRunStop(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}

	wantRedirect := fmt.Sprintf("/runs/%d/summary", run.ID)
	if got := rr.Header().Get("HX-Redirect"); got != wantRedirect {
		t.Errorf("HX-Redirect = %q, want %q", got, wantRedirect)
	}

	// Run must be marked completed.
	var updated db.SessionRun
	if err := store.DB.First(&updated, run.ID).Error; err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if updated.Status != db.RunStatusCompleted {
		t.Errorf("run.Status = %q after stop, want %q", updated.Status, db.RunStatusCompleted)
	}
	if updated.CompletedAt == nil {
		t.Error("run.CompletedAt is nil after stop, want non-nil")
	}

	// A SessionJournal row must now exist for the run.
	j, err := db.GetSessionJournalByRunID(store.DB, ownerID, run.ID)
	if err != nil {
		t.Fatalf("GetSessionJournalByRunID: %v", err)
	}
	if j == nil {
		t.Error("no SessionJournal created for run after stop, want one to exist")
	}
}

// TestHandleRunStop_OpenSessionRedirectsToSummary guards that open sessions also
// land on /summary (not /dashboard) now.
func TestHandleRunStop_OpenSessionRedirectsToSummary(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "stop-open.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 2
	run := seedOpenRun(t, store, ownerID)

	srv := &Server{store: store}
	req := newRunStopRequest(t, run.ID, ownerID)
	rr := httptest.NewRecorder()

	srv.handleRunStop(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}

	wantRedirect := fmt.Sprintf("/runs/%d/summary", run.ID)
	if got := rr.Header().Get("HX-Redirect"); got != wantRedirect {
		t.Errorf("HX-Redirect = %q, want %q (open session should also go to summary)", got, wantRedirect)
	}
}

// TestHandleRunStop_MethodNotAllowed guards the method guard.
func TestHandleRunStop_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	srv := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/runs/1/stop", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("runID", "1")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, authUserIDKey, uint(1))
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	srv.handleRunStop(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

// TestHandleRunStop_AlreadyCompletedRejected guards that stopping an already-completed
// run returns 400 (not a silent double-complete).
func TestHandleRunStop_AlreadyCompletedRejected(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "stop-completed.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 3
	run := seedGuidedRun(t, store, ownerID)

	// Pre-complete the run.
	now := time.Now()
	run.Status = db.RunStatusCompleted
	run.CompletedAt = &now
	if err := store.DB.Save(run).Error; err != nil {
		t.Fatal(err)
	}

	srv := &Server{store: store}
	req := newRunStopRequest(t, run.ID, ownerID)
	rr := httptest.NewRecorder()

	srv.handleRunStop(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d (stopping completed run should be rejected)", rr.Code, http.StatusBadRequest)
	}
}

// TestHandleRunStop_JournalNotDuplicatedOnRepeatCall guards idempotency: stopping a run
// that already has a journal must not create a second journal row.
func TestHandleRunStop_JournalNotDuplicatedOnRepeatCall(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "stop-no-dup-journal.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 4
	run := seedGuidedRun(t, store, ownerID)

	// Pre-create a journal (simulating a previous stop).
	runID := run.ID
	j := db.SessionJournal{OwnerID: ownerID, RunID: &runID, WentWell: "pre-existing notes"}
	if err := db.UpsertSessionJournal(store.DB, &j); err != nil {
		t.Fatal(err)
	}

	// Reset run to running so the stop handler will accept it.
	run.Status = db.RunStatusRunning
	run.CompletedAt = nil
	if err := store.DB.Save(run).Error; err != nil {
		t.Fatal(err)
	}

	srv := &Server{store: store}
	req := newRunStopRequest(t, run.ID, ownerID)
	rr := httptest.NewRecorder()

	srv.handleRunStop(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var count int64
	store.DB.Model(&db.SessionJournal{}).Where("owner_id = ? AND run_id = ?", ownerID, run.ID).Count(&count)
	if count != 1 {
		t.Errorf("journal count = %d after repeat stop, want 1 (no duplicate)", count)
	}

	// Existing content must be preserved.
	got, _ := db.GetSessionJournalByRunID(store.DB, ownerID, run.ID)
	if got == nil || got.WentWell != "pre-existing notes" {
		wentWell := ""
		if got != nil {
			wentWell = got.WentWell
		}
		t.Errorf("journal WentWell = %q, want %q (pre-existing journal should be preserved)", wentWell, "pre-existing notes")
	}
}

// ---------------------------------------------------------------------------
// completeRunExercise — redirect logic
// ---------------------------------------------------------------------------

// seedGuidedRunWithExercises creates a guided run with a template that has two exercises,
// then returns the run and the two exercise IDs.
func seedGuidedRunWithExercises(t *testing.T, store *db.Store, ownerID uint) (run *db.SessionRun, exID1, exID2 uint) {
	t.Helper()
	tpl := &db.SessionTemplate{OwnerID: ownerID, Name: "Two-exercise Template"}
	if err := store.DB.Create(tpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	act := &db.Activity{
		OwnerID:           ownerID,
		SessionTemplateID: tpl.ID,
		Type:              "strength",
		Name:              "Main",
		OrderIndex:        0,
	}
	if err := store.DB.Create(act).Error; err != nil {
		t.Fatalf("create activity: %v", err)
	}
	ex1 := &db.Exercise{
		OwnerID:    ownerID,
		ActivityID: &act.ID,
		Name:       "Pull Up",
		Kind:       "reps_and_sets",
		OrderIndex: 0,
	}
	if err := store.DB.Create(ex1).Error; err != nil {
		t.Fatalf("create ex1: %v", err)
	}
	ex2 := &db.Exercise{
		OwnerID:    ownerID,
		ActivityID: &act.ID,
		Name:       "Push Up",
		Kind:       "reps_and_sets",
		OrderIndex: 1,
	}
	if err := store.DB.Create(ex2).Error; err != nil {
		t.Fatalf("create ex2: %v", err)
	}
	ss := &db.ScheduledSession{
		OwnerID:           ownerID,
		IsTrial:           true,
		ScheduledDate:     time.Now(),
		SessionTemplateID: tpl.ID,
	}
	if err := store.DB.Create(ss).Error; err != nil {
		t.Fatalf("create scheduled session: %v", err)
	}
	run = &db.SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: ss.ID,
		IsTrial:            true,
		IsOpen:             false,
		Status:             db.RunStatusRunning,
		StartedAt:          time.Now(),
	}
	if err := store.DB.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
	return run, ex1.ID, ex2.ID
}

// newCompleteExerciseRequest builds a POST /runs/{runID}/exercises/{exerciseID}/complete
// request with chi params and auth injected.
func newCompleteExerciseRequest(t *testing.T, runID, exerciseID, ownerID uint) *http.Request {
	t.Helper()
	path := fmt.Sprintf("/runs/%d/exercises/%d/complete", runID, exerciseID)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("runID", fmt.Sprintf("%d", runID))
	rctx.URLParams.Add("exerciseID", fmt.Sprintf("%d", exerciseID))
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, authUserIDKey, ownerID)
	return req.WithContext(ctx)
}

// TestCompleteRunExercise_NonFinalRedirectsToRunPage guards that completing a non-final
// exercise still redirects to the run page (not summary).
func TestCompleteRunExercise_NonFinalRedirectsToRunPage(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "complete-nonfinal.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 10
	run, exID1, _ := seedGuidedRunWithExercises(t, store, ownerID)

	srv := &Server{store: store}
	req := newCompleteExerciseRequest(t, run.ID, exID1, ownerID)
	rr := httptest.NewRecorder()

	if err := srv.completeRunExercise(rr, req, run.ID, exID1, "complete", ownerID); err != nil {
		t.Fatalf("completeRunExercise: %v", err)
	}

	redirect := rr.Header().Get("HX-Redirect")
	wantSummary := fmt.Sprintf("/runs/%d/summary", run.ID)
	if redirect == wantSummary {
		t.Errorf("HX-Redirect = %q after non-final exercise, must NOT redirect to summary", redirect)
	}
	if !strings.HasPrefix(redirect, fmt.Sprintf("/runs/%d", run.ID)) {
		t.Errorf("HX-Redirect = %q, want prefix /runs/%d (run page)", redirect, run.ID)
	}
	// Run should still be running.
	var updated db.SessionRun
	store.DB.First(&updated, run.ID)
	if updated.Status != db.RunStatusRunning {
		t.Errorf("run.Status = %q after non-final exercise, want running", updated.Status)
	}
}

// TestCompleteRunExercise_FinalExerciseRedirectsToSummary guards the new behaviour:
// completing the LAST exercise of a guided run redirects to /runs/{id}/summary.
func TestCompleteRunExercise_FinalExerciseRedirectsToSummary(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "complete-final.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 11
	run, exID1, exID2 := seedGuidedRunWithExercises(t, store, ownerID)

	srv := &Server{store: store}

	// Complete first exercise so second becomes the last.
	req1 := newCompleteExerciseRequest(t, run.ID, exID1, ownerID)
	rr1 := httptest.NewRecorder()
	if err := srv.completeRunExercise(rr1, req1, run.ID, exID1, "complete", ownerID); err != nil {
		t.Fatalf("completeRunExercise ex1: %v", err)
	}

	// Complete second (final) exercise.
	req2 := newCompleteExerciseRequest(t, run.ID, exID2, ownerID)
	rr2 := httptest.NewRecorder()
	if err := srv.completeRunExercise(rr2, req2, run.ID, exID2, "complete", ownerID); err != nil {
		t.Fatalf("completeRunExercise ex2: %v", err)
	}

	wantRedirect := fmt.Sprintf("/runs/%d/summary", run.ID)
	if got := rr2.Header().Get("HX-Redirect"); got != wantRedirect {
		t.Errorf("HX-Redirect = %q after final exercise, want %q", got, wantRedirect)
	}

	// Run must be marked completed.
	var updated db.SessionRun
	store.DB.First(&updated, run.ID)
	if updated.Status != db.RunStatusCompleted {
		t.Errorf("run.Status = %q after completing last exercise, want completed", updated.Status)
	}
}

// TestCompleteRunExercise_OpenSessionFinalRedirectsToRunPage guards that completing all
// exercises in an open session does NOT redirect to /summary — open sessions always stay
// on the run page (IsOpen path returns the run-page URL regardless of completions).
func TestCompleteRunExercise_OpenSessionFinalRedirectsToRunPage(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "complete-open-final.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 12

	// Build an open session with a single exercise added directly to the run.
	tpl := &db.SessionTemplate{OwnerID: ownerID, Name: "Open Tpl", IsSystem: true}
	if err := store.DB.Create(tpl).Error; err != nil {
		t.Fatal(err)
	}
	act := &db.Activity{
		OwnerID:           ownerID,
		SessionTemplateID: tpl.ID,
		Type:              "strength",
		Name:              "Main",
		OrderIndex:        0,
	}
	if err := store.DB.Create(act).Error; err != nil {
		t.Fatal(err)
	}
	ex := &db.Exercise{
		OwnerID:    ownerID,
		ActivityID: &act.ID,
		Name:       "Dead Hang",
		Kind:       "reps_and_sets",
		OrderIndex: 0,
	}
	if err := store.DB.Create(ex).Error; err != nil {
		t.Fatal(err)
	}
	ss := &db.ScheduledSession{
		OwnerID:           ownerID,
		IsTrial:           true,
		ScheduledDate:     time.Now(),
		SessionTemplateID: tpl.ID,
	}
	if err := store.DB.Create(ss).Error; err != nil {
		t.Fatal(err)
	}
	run := &db.SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: ss.ID,
		IsTrial:            true,
		IsOpen:             true,
		Status:             db.RunStatusRunning,
		StartedAt:          time.Now(),
	}
	if err := store.DB.Create(run).Error; err != nil {
		t.Fatal(err)
	}

	srv := &Server{store: store}
	req := newCompleteExerciseRequest(t, run.ID, ex.ID, ownerID)
	rr := httptest.NewRecorder()
	if err := srv.completeRunExercise(rr, req, run.ID, ex.ID, "complete", ownerID); err != nil {
		t.Fatalf("completeRunExercise: %v", err)
	}

	redirect := rr.Header().Get("HX-Redirect")
	summaryURL := fmt.Sprintf("/runs/%d/summary", run.ID)
	if redirect == summaryURL {
		t.Errorf("HX-Redirect = %q for open session, must NOT redirect to summary", redirect)
	}
	if !strings.HasPrefix(redirect, fmt.Sprintf("/runs/%d", run.ID)) {
		t.Errorf("HX-Redirect = %q, want prefix /runs/%d (run page)", redirect, run.ID)
	}
}

// TestHandleRunStop_MarksRemainingExercisesSkipped guards that finishing a session
// early accounts for every step. Previously the handler only flipped the run status,
// leaving untouched exercises with no completion row at all — they rendered as
// "pending" in the summary and were counted as neither done nor skipped.
func TestHandleRunStop_MarksRemainingExercisesSkipped(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "run-stop.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1

	tpl := &db.SessionTemplate{OwnerID: ownerID, Name: "Stop Template"}
	if err := store.DB.Create(tpl).Error; err != nil {
		t.Fatal(err)
	}
	act := &db.Activity{OwnerID: ownerID, SessionTemplateID: tpl.ID, Type: "activity", Name: "Work", OrderIndex: 0}
	if err := store.DB.Create(act).Error; err != nil {
		t.Fatal(err)
	}
	var exIDs []uint
	for i, name := range []string{"First", "Second", "Third"} {
		ex := &db.Exercise{OwnerID: ownerID, ActivityID: &act.ID, Name: name, Kind: "session", OrderIndex: i}
		if err := store.DB.Create(ex).Error; err != nil {
			t.Fatal(err)
		}
		exIDs = append(exIDs, ex.ID)
	}
	ss := &db.ScheduledSession{OwnerID: ownerID, IsTrial: true, ScheduledDate: time.Now(), SessionTemplateID: tpl.ID}
	if err := store.DB.Create(ss).Error; err != nil {
		t.Fatal(err)
	}
	run := &db.SessionRun{
		OwnerID: ownerID, ScheduledSessionID: ss.ID, IsTrial: true,
		Status: db.RunStatusRunning, StartedAt: time.Now().Add(-20 * time.Minute),
	}
	if err := store.DB.Create(run).Error; err != nil {
		t.Fatal(err)
	}
	// Only the first exercise was actually done.
	if err := store.DB.Create(&db.RunExerciseCompletion{
		OwnerID: ownerID, RunID: run.ID, ExerciseID: exIDs[0],
		Status: db.RunStatusCompleted, CompletedAt: time.Now(), ElapsedSeconds: 90,
	}).Error; err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(store, "secret", 24, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	srv.handleRunStop(rr, newRunStopRequest(t, run.ID, ownerID))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", rr.Code, rr.Body.String())
	}

	var comps []db.RunExerciseCompletion
	if err := store.DB.Where("owner_id = ? AND run_id = ?", ownerID, run.ID).Find(&comps).Error; err != nil {
		t.Fatal(err)
	}
	if len(comps) != len(exIDs) {
		t.Fatalf("completion rows = %d, want %d (every step accounted for)", len(comps), len(exIDs))
	}
	byID := map[uint]db.RunExerciseCompletion{}
	for _, c := range comps {
		byID[c.ExerciseID] = c
	}
	// The real completion must survive untouched.
	if got := byID[exIDs[0]]; got.Status != db.RunStatusCompleted || got.ElapsedSeconds != 90 {
		t.Errorf("first exercise = %+v, want completed with 90s elapsed", got)
	}
	for _, id := range exIDs[1:] {
		if got := byID[id].Status; got != db.RunStatusSkipped {
			t.Errorf("exercise %d status = %q, want %q", id, got, db.RunStatusSkipped)
		}
	}
}
