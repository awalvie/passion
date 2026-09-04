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

// newRunSummaryRequest builds a GET /runs/{runID}/summary request with chi params
// and auth injected. Set hx to true to simulate the HTMX fragment path.
func newRunSummaryRequest(t *testing.T, runID uint, ownerID uint, hx bool) *http.Request {
	t.Helper()
	path := fmt.Sprintf("/runs/%d/summary", runID)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("runID", fmt.Sprintf("%d", runID))
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, authUserIDKey, ownerID)
	return req.WithContext(ctx)
}

// seedGuidedRunWithClimbingExercise creates a completed guided run with a single
// climbing-kind exercise, returning the run and exercise ID.
func seedGuidedRunWithClimbingExercise(t *testing.T, store *db.Store, ownerID uint) (run *db.SessionRun, exerciseID uint) {
	t.Helper()
	tpl := &db.SessionTemplate{OwnerID: ownerID, Name: "Climbing Template"}
	if err := store.DB.Create(tpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	act := &db.Activity{
		OwnerID:           ownerID,
		SessionTemplateID: tpl.ID,
		Type:              "climbing",
		Name:              "Bouldering",
		OrderIndex:        0,
	}
	if err := store.DB.Create(act).Error; err != nil {
		t.Fatalf("create activity: %v", err)
	}
	ex := &db.Exercise{
		OwnerID:    ownerID,
		ActivityID: &act.ID,
		Name:       "Boulder Session",
		Kind:       "climbing",
		OrderIndex: 0,
	}
	if err := store.DB.Create(ex).Error; err != nil {
		t.Fatalf("create exercise: %v", err)
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
	now := time.Now()
	run = &db.SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: ss.ID,
		IsTrial:            true,
		IsOpen:             false,
		Status:             db.RunStatusCompleted,
		StartedAt:          now.Add(-30 * time.Minute),
		CompletedAt:        &now,
	}
	if err := store.DB.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
	return run, ex.ID
}

// seedOpenRunWithClimbingExercise creates a completed open run with a single
// climbing-kind exercise added directly to the run (no activity/template linkage,
// matching how open sessions store exercises).
func seedOpenRunWithClimbingExercise(t *testing.T, store *db.Store, ownerID uint) (run *db.SessionRun, exerciseID uint) {
	t.Helper()
	tpl := &db.SessionTemplate{OwnerID: ownerID, Name: "Open Climbing", IsSystem: true}
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
	now := time.Now()
	run = &db.SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: ss.ID,
		IsTrial:            true,
		IsOpen:             true,
		Status:             db.RunStatusCompleted,
		StartedAt:          now.Add(-30 * time.Minute),
		CompletedAt:        &now,
	}
	if err := store.DB.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
	ex := &db.Exercise{
		OwnerID:      ownerID,
		SessionRunID: &run.ID,
		Name:         "Boulder Session",
		Kind:         "climbing",
		OrderIndex:   0,
	}
	if err := store.DB.Create(ex).Error; err != nil {
		t.Fatalf("create exercise: %v", err)
	}
	return run, ex.ID
}

// TestHandleRunSummary_GuidedRunShowsTicks guards that a climbing exercise inside a
// template-based (guided) run has its logged ticks loaded and rendered in the summary
// fragment. This exercises the "template branch" of handleRunSummary.
func TestHandleRunSummary_GuidedRunShowsTicks(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "summary-guided-ticks.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	run, exID := seedGuidedRunWithClimbingExercise(t, store, ownerID)

	tick := &db.ClimbingTick{
		OwnerID: ownerID, RunID: run.ID, ExerciseID: exID,
		Kind: "boulder", Grade: "V5", Sent: true, Attempts: 2,
	}
	if err := db.CreateClimbingTick(store.DB, tick); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(store, "secret", 24, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	req := newRunSummaryRequest(t, run.ID, ownerID, true)
	rr := httptest.NewRecorder()
	srv.handleRunSummary(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "V5") {
		t.Errorf("guided-run summary fragment does not contain logged tick grade 'V5': %.500q", body)
	}
}

// TestHandleRunSummary_OpenRunShowsTicks guards the open-session branch of
// handleRunSummary. Before this change, ticks were never surfaced in the summary at
// all — this pins down that the open-session code path also loads and renders them,
// not just the guided/template path.
func TestHandleRunSummary_OpenRunShowsTicks(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "summary-open-ticks.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 2
	run, exID := seedOpenRunWithClimbingExercise(t, store, ownerID)

	tick := &db.ClimbingTick{
		OwnerID: ownerID, RunID: run.ID, ExerciseID: exID,
		Kind: "boulder", Grade: "V7", Sent: true, Attempts: 1,
	}
	if err := db.CreateClimbingTick(store.DB, tick); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(store, "secret", 24, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	req := newRunSummaryRequest(t, run.ID, ownerID, true)
	rr := httptest.NewRecorder()
	srv.handleRunSummary(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "V7") {
		t.Errorf("open-run summary fragment does not contain logged tick grade 'V7': %.500q", body)
	}
}

// TestHandleRunSummary_TicksOwnerScoped guards against cross-owner tick leakage:
// another owner's tick recorded against the same run/exercise ID combination must
// never appear in this owner's summary. Reproduces the exact query path
// summaryTickViews takes (ListClimbingTicksByExercise), verified through the full
// handler + template render, not just the DB layer.
func TestHandleRunSummary_TicksOwnerScoped(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "summary-owner-scoped.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 3
	const otherOwnerID uint = 99
	run, exID := seedGuidedRunWithClimbingExercise(t, store, ownerID)

	// Another owner's tick on the same run/exercise IDs (simulating an ID collision
	// or a bug where owner scoping is dropped).
	otherTick := &db.ClimbingTick{
		OwnerID: otherOwnerID, RunID: run.ID, ExerciseID: exID,
		Kind: "boulder", Grade: "V13", Sent: true, Attempts: 1,
	}
	if err := db.CreateClimbingTick(store.DB, otherTick); err != nil {
		t.Fatal(err)
	}
	// This owner's own tick, so we can confirm real ticks still render.
	myTick := &db.ClimbingTick{
		OwnerID: ownerID, RunID: run.ID, ExerciseID: exID,
		Kind: "boulder", Grade: "V2", Sent: true, Attempts: 1,
	}
	if err := db.CreateClimbingTick(store.DB, myTick); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(store, "secret", 24, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	req := newRunSummaryRequest(t, run.ID, ownerID, true)
	rr := httptest.NewRecorder()
	srv.handleRunSummary(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, "V13") {
		t.Errorf("summary leaked another owner's tick grade 'V13' into the response: %.500q", body)
	}
	if !strings.Contains(body, "V2") {
		t.Errorf("summary is missing this owner's own tick grade 'V2': %.500q", body)
	}
}

// TestHandleRunSummary_NonClimbingExerciseHasNoTicksBlock guards the ex.Kind ==
// "climbing" gate: a non-climbing exercise must never trigger a ticks query/render,
// even if (by data corruption or a future bug) a ClimbingTick row exists for its
// exercise ID.
func TestHandleRunSummary_NonClimbingExerciseHasNoTicksBlock(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "summary-non-climbing.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 4
	run, exID1, _ := seedGuidedRunWithExercises(t, store, ownerID)
	// exID1 ("Pull Up") is Kind "reps_and_sets", not "climbing".

	// Mark the run completed so the full-page path (and fragment path) both render summary.
	now := time.Now()
	run.Status = db.RunStatusCompleted
	run.CompletedAt = &now
	if err := store.DB.Save(run).Error; err != nil {
		t.Fatal(err)
	}

	// A stray tick exists for this non-climbing exercise ID — should never surface.
	stray := &db.ClimbingTick{
		OwnerID: ownerID, RunID: run.ID, ExerciseID: exID1,
		Kind: "boulder", Grade: "V9", Sent: true, Attempts: 1,
	}
	if err := db.CreateClimbingTick(store.DB, stray); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(store, "secret", 24, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	req := newRunSummaryRequest(t, run.ID, ownerID, true)
	rr := httptest.NewRecorder()
	srv.handleRunSummary(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, "V9") {
		t.Errorf("non-climbing exercise summary rendered a stray tick grade 'V9' (Kind gate not enforced): %.500q", body)
	}
}

// TestHandleRunSummary_ClimbingExerciseNoTicksRendersCleanly guards the zero-ticks
// path: a climbing exercise with no logged ticks must render without panicking and
// without an empty run_ticks_readonly block appearing in the output.
func TestHandleRunSummary_ClimbingExerciseNoTicksRendersCleanly(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "summary-no-ticks.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 5
	run, _ := seedGuidedRunWithClimbingExercise(t, store, ownerID)
	// No ticks created for this exercise.

	srv, err := NewServer(store, "secret", 24, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	req := newRunSummaryRequest(t, run.ID, ownerID, true)
	rr := httptest.NewRecorder()
	srv.handleRunSummary(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, "run-summary-ticks") {
		t.Errorf("climbing exercise with zero ticks rendered a run-summary-ticks block, want none: %.500q", body)
	}
}

// seedGuidedRunWithCatalogMenu builds a guided run whose only top-level exercise is
// an exercise_catalog menu with two climbing children, and records a choice for one
// of them. Returns the run plus the menu and chosen child IDs.
func seedGuidedRunWithCatalogMenu(t *testing.T, store *db.Store, ownerID uint) (run *db.SessionRun, menuID, chosenID uint) {
	t.Helper()
	tpl := &db.SessionTemplate{OwnerID: ownerID, Name: "Menu Template"}
	if err := store.DB.Create(tpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	act := &db.Activity{OwnerID: ownerID, SessionTemplateID: tpl.ID, Type: "activity", Name: "Endurance Work", OrderIndex: 0}
	if err := store.DB.Create(act).Error; err != nil {
		t.Fatalf("create activity: %v", err)
	}
	menu := &db.Exercise{OwnerID: ownerID, ActivityID: &act.ID, Name: "Endurance Method", Kind: "exercise_catalog", OrderIndex: 0}
	if err := store.DB.Create(menu).Error; err != nil {
		t.Fatalf("create menu: %v", err)
	}
	chosen := &db.Exercise{OwnerID: ownerID, ActivityID: &act.ID, ParentExerciseID: &menu.ID, Name: "Traverse Circuit", Kind: "climbing", OrderIndex: 0}
	other := &db.Exercise{OwnerID: ownerID, ActivityID: &act.ID, ParentExerciseID: &menu.ID, Name: "Power Endurance Intervals", Kind: "climbing", OrderIndex: 1}
	if err := store.DB.Create(chosen).Error; err != nil {
		t.Fatalf("create chosen child: %v", err)
	}
	if err := store.DB.Create(other).Error; err != nil {
		t.Fatalf("create other child: %v", err)
	}
	ss := &db.ScheduledSession{OwnerID: ownerID, IsTrial: true, ScheduledDate: time.Now(), SessionTemplateID: tpl.ID}
	if err := store.DB.Create(ss).Error; err != nil {
		t.Fatalf("create scheduled session: %v", err)
	}
	now := time.Now()
	run = &db.SessionRun{
		OwnerID: ownerID, ScheduledSessionID: ss.ID, IsTrial: true, IsOpen: false,
		Status: db.RunStatusCompleted, StartedAt: now.Add(-time.Hour), CompletedAt: &now,
	}
	if err := store.DB.Create(run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
	choice := &db.RunExerciseChoice{OwnerID: ownerID, RunID: run.ID, ParentExerciseID: menu.ID, ChosenExerciseID: chosen.ID}
	if err := store.DB.Create(choice).Error; err != nil {
		t.Fatalf("create choice: %v", err)
	}
	return run, menu.ID, chosen.ID
}

// TestHandleRunSummary_CatalogMenuShowsChosenChildAndTicks pins down that an
// exercise_catalog resolves to whatever was actually picked. The run records the
// completion and the climbing ticks against the chosen child, not the menu, so a
// summary that only walked top-level exercises showed the menu as forever "pending"
// and hid the climbing entirely.
func TestHandleRunSummary_CatalogMenuShowsChosenChildAndTicks(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "summary-catalog.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	run, _, chosenID := seedGuidedRunWithCatalogMenu(t, store, ownerID)

	if err := store.DB.Create(&db.RunExerciseCompletion{
		OwnerID: ownerID, RunID: run.ID, ExerciseID: chosenID,
		Status: db.RunStatusCompleted, CompletedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
	tick := &db.ClimbingTick{
		OwnerID: ownerID, RunID: run.ID, ExerciseID: chosenID,
		Kind: "boulder", Grade: "Traverse", Sent: true, Attempts: 1, DurationSeconds: 252,
	}
	if err := db.CreateClimbingTick(store.DB, tick); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(store, "secret", 24, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	req := newRunSummaryRequest(t, run.ID, ownerID, true)
	rr := httptest.NewRecorder()
	srv.handleRunSummary(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Traverse Circuit") {
		t.Errorf("summary omits the chosen catalog option 'Traverse Circuit': %.700q", body)
	}
	if !strings.Contains(body, "4:12") {
		t.Errorf("summary omits the logged time on the wall '4:12': %.700q", body)
	}
	if strings.Contains(body, "Power Endurance Intervals") {
		t.Errorf("summary shows an option that was not chosen: %.700q", body)
	}
}

// The summary is keyed by exercise id against the run's completions. A materialised run
// holds its own exercises, so the summary has to read those and not today's template.
//
// This regressed in the worst possible way: before runs owned their exercises, the summary
// found 29 of the owner's 113 completions. After the backfill moved every completion onto
// run-owned rows, the summary still read the template and found none at all.
func TestRunSummaryReadsTheRunsOwnExercises(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "summary-owned.db"))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(store, "test-secret-at-least-32-characters!!", time.Hour, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1

	tpl := &db.SessionTemplate{OwnerID: ownerID, Name: "Strength Day"}
	if err := store.DB.Create(tpl).Error; err != nil {
		t.Fatal(err)
	}
	act := &db.Activity{OwnerID: ownerID, SessionTemplateID: tpl.ID, Type: "activity", Name: "Main"}
	if err := store.DB.Create(act).Error; err != nil {
		t.Fatal(err)
	}
	tplEx := &db.Exercise{OwnerID: ownerID, ActivityID: &act.ID,
		Name: "Weighted Pull-ups", Kind: "reps_and_sets", Sets: 5, Reps: 5}
	if err := store.DB.Create(tplEx).Error; err != nil {
		t.Fatal(err)
	}
	ss := &db.ScheduledSession{OwnerID: ownerID, ScheduledDate: time.Now(), SessionTemplateID: tpl.ID}
	if err := store.DB.Create(ss).Error; err != nil {
		t.Fatal(err)
	}

	// Start the run the way the app now does: it takes its own copy immediately.
	run, err := db.StartRunForScheduledSession(store.DB, ownerID, ss.ID, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var runEx db.Exercise
	if err := store.DB.Where("session_run_id = ?", run.ID).First(&runEx).Error; err != nil {
		t.Fatal(err)
	}
	done := time.Now()
	if err := store.DB.Create(&db.RunExerciseCompletion{
		OwnerID: ownerID, RunID: run.ID, ExerciseID: runEx.ID,
		Status: "completed", CompletedAt: done,
		ActualSets: 5, ActualReps: 4, ActualWeightKg: 12.5,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&db.SessionRun{}).Where("id = ?", run.ID).
		Updates(map[string]any{"status": db.RunStatusCompleted, "completed_at": done}).Error; err != nil {
		t.Fatal(err)
	}

	// Now retire the template's exercise, exactly as an import does.
	if err := store.DB.Delete(&db.Exercise{}, tplEx.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Delete(&db.Activity{}, act.ID).Error; err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	srv.handleRunSummary(rr, newRunSummaryRequest(t, run.ID, ownerID, false))
	if rr.Code != http.StatusOK {
		t.Fatalf("summary returned %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Weighted Pull-ups") {
		t.Error("the summary lost the exercise once the template row was retired")
	}
	// The completion has to be matched, or the exercise renders as "pending" with no
	// detail — the reported bug.
	if strings.Contains(body, "run-summary-row--pending") {
		t.Error("the exercise renders as pending: its completion was not matched")
	}
	if !strings.Contains(body, "run-summary-row--completed") {
		t.Error("the summary shows nothing completed, so the completion was not matched")
	}
}
