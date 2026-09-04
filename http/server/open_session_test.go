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

// openChiRequest builds an httptest.Request with chi URL params injected.
func openChiRequest(t *testing.T, method, path string, params map[string]string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func withOpenOwner(t *testing.T, req *http.Request, ownerID uint) *http.Request {
	t.Helper()
	return req.WithContext(context.WithValue(req.Context(), authUserIDKey, ownerID))
}

// seedOpenRunWithCompletedExercise creates an open, running SessionRun owned by ownerID
// with one Exercise that has a full set of logged data: a RunExerciseCompletion,
// a ClimbingTick, a ManualExerciseSetLog, and an ExercisePlannedSet.
func seedOpenRunWithCompletedExercise(t *testing.T, store *db.Store, ownerID uint) (runID, exerciseID uint) {
	t.Helper()

	tpl := db.SessionTemplate{OwnerID: ownerID, Name: "Open Session", IsSystem: true}
	if err := store.DB.Create(&tpl).Error; err != nil {
		t.Fatal(err)
	}
	scheduled := db.ScheduledSession{OwnerID: ownerID, IsTrial: true, ScheduledDate: time.Now(), SessionTemplateID: tpl.ID}
	if err := store.DB.Create(&scheduled).Error; err != nil {
		t.Fatal(err)
	}
	run := db.SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: scheduled.ID,
		IsOpen:             true,
		Status:             db.RunStatusRunning,
		StartedAt:          time.Now(),
	}
	if err := store.DB.Create(&run).Error; err != nil {
		t.Fatal(err)
	}

	rid := run.ID
	ex := db.Exercise{
		OwnerID:      ownerID,
		SessionRunID: &rid,
		Name:         "Boulders",
		Kind:         "climbing",
		OrderIndex:   0,
	}
	if err := store.DB.Create(&ex).Error; err != nil {
		t.Fatal(err)
	}

	completion := db.RunExerciseCompletion{
		OwnerID:     ownerID,
		RunID:       run.ID,
		ExerciseID:  ex.ID,
		Status:      "completed",
		CompletedAt: time.Now(),
	}
	if err := store.DB.Create(&completion).Error; err != nil {
		t.Fatal(err)
	}

	tick := db.ClimbingTick{
		OwnerID:    ownerID,
		RunID:      run.ID,
		ExerciseID: ex.ID,
		Kind:       "boulder",
		Setting:    "indoor",
		Style:      "redpoint",
		Sent:       true,
	}
	if err := store.DB.Create(&tick).Error; err != nil {
		t.Fatal(err)
	}

	setLog := db.ManualExerciseSetLog{
		OwnerID:    ownerID,
		RunID:      run.ID,
		ExerciseID: ex.ID,
		SetIndex:   1,
		Reps:       5,
		WeightKg:   10,
	}
	if err := store.DB.Create(&setLog).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.UpsertExercisePlannedSet(store.DB, ownerID, ex.ID, 1, 5, 10); err != nil {
		t.Fatal(err)
	}

	return run.ID, ex.ID
}

// ---------------------------------------------------------------------------
// handleOpenDeleteExercise
// ---------------------------------------------------------------------------

func TestHandleOpenDeleteExercise_CompletedExerciseFullyCleanedUp(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "open-delete-completed.db"))
	if err != nil {
		t.Fatal(err)
	}
	user := &db.User{Email: "a@a.com", PasswordHash: "x"}
	if err := store.DB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	ownerID := user.ID

	runID, exID := seedOpenRunWithCompletedExercise(t, store, ownerID)

	srv, err := NewServer(store, "secret", 24, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	req := openChiRequest(t, http.MethodPost, fmt.Sprintf("/runs/%d/open/exercises/%d/delete", runID, exID),
		map[string]string{"runID": fmt.Sprintf("%d", runID), "exerciseID": fmt.Sprintf("%d", exID)})
	req = withOpenOwner(t, req, ownerID)
	rr := httptest.NewRecorder()

	srv.handleOpenDeleteExercise(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for deleting a completed exercise, got %d; body: %q", rr.Code, rr.Body.String())
	}
	wantRedirect := fmt.Sprintf("/runs/%d", runID)
	if got := rr.Header().Get("HX-Redirect"); got != wantRedirect {
		t.Errorf("HX-Redirect = %q, want %q", got, wantRedirect)
	}

	// Exercise itself should no longer be visible via normal (soft-delete-scoped) query.
	var exCount int64
	store.DB.Model(&db.Exercise{}).Where("id = ?", exID).Count(&exCount)
	if exCount != 0 {
		t.Errorf("expected exercise to be soft-deleted (invisible to scoped query), still visible")
	}

	var completions []db.RunExerciseCompletion
	store.DB.Where("exercise_id = ?", exID).Find(&completions)
	if len(completions) != 0 {
		t.Errorf("expected RunExerciseCompletion rows to be cleaned up, found %d", len(completions))
	}

	var ticks []db.ClimbingTick
	store.DB.Where("exercise_id = ?", exID).Find(&ticks)
	if len(ticks) != 0 {
		t.Errorf("expected ClimbingTick rows to be cleaned up, found %d", len(ticks))
	}

	var setLogs []db.ManualExerciseSetLog
	store.DB.Where("exercise_id = ?", exID).Find(&setLogs)
	if len(setLogs) != 0 {
		t.Errorf("expected ManualExerciseSetLog rows to be cleaned up, found %d", len(setLogs))
	}

	plannedSets, err := db.ListExercisePlannedSets(store.DB, exID)
	if err != nil {
		t.Fatal(err)
	}
	if len(plannedSets) != 0 {
		t.Errorf("expected ExercisePlannedSet rows to be cleaned up, found %d", len(plannedSets))
	}
}

func TestHandleOpenDeleteExercise_DeletedExerciseExcludedFromOrderIndexCount(t *testing.T) {
	// Regression guard: DeleteManualExercise soft-deletes the Exercise (gorm.Model
	// DeletedAt) rather than hard-deleting it. handleOpenAddExercise computes the
	// new exercise's OrderIndex from a Count() of exercises on the run. If that
	// count were not soft-delete-scoped, a deleted exercise would still occupy an
	// order-index slot and/or reappear in the run's exercise list.
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "open-delete-orderindex.db"))
	if err != nil {
		t.Fatal(err)
	}
	user := &db.User{Email: "b@b.com", PasswordHash: "x"}
	if err := store.DB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	ownerID := user.ID

	runID, exID := seedOpenRunWithCompletedExercise(t, store, ownerID)

	srv, err := NewServer(store, "secret", 24, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	delReq := openChiRequest(t, http.MethodPost, fmt.Sprintf("/runs/%d/open/exercises/%d/delete", runID, exID),
		map[string]string{"runID": fmt.Sprintf("%d", runID), "exerciseID": fmt.Sprintf("%d", exID)})
	delReq = withOpenOwner(t, delReq, ownerID)
	delRR := httptest.NewRecorder()
	srv.handleOpenDeleteExercise(delRR, delReq)
	if delRR.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %q", delRR.Code, delRR.Body.String())
	}

	// The run still has zero live exercises. A fresh count-based OrderIndex query
	// (the same shape handleOpenAddExercise uses) must return 0, not 1.
	var count int64
	store.DB.Model(&db.Exercise{}).
		Where("session_run_id = ? AND parent_exercise_id IS NULL", runID).
		Count(&count)
	if count != 0 {
		t.Errorf("expected 0 live exercises on run after delete, got %d (soft-deleted exercise leaking into count)", count)
	}

	var exercises []db.Exercise
	store.DB.Where("session_run_id = ?", runID).Find(&exercises)
	if len(exercises) != 0 {
		t.Errorf("expected deleted exercise to be excluded from scoped list query, found %d", len(exercises))
	}
}

func TestHandleOpenDeleteExercise_WrongOwnerBlocked(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "open-delete-idor.db"))
	if err != nil {
		t.Fatal(err)
	}
	owner := &db.User{Email: "owner@a.com", PasswordHash: "x"}
	attacker := &db.User{Email: "attacker@a.com", PasswordHash: "x"}
	if err := store.DB.Create(owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(attacker).Error; err != nil {
		t.Fatal(err)
	}

	runID, exID := seedOpenRunWithCompletedExercise(t, store, owner.ID)

	srv, err := NewServer(store, "secret", 24, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	req := openChiRequest(t, http.MethodPost, fmt.Sprintf("/runs/%d/open/exercises/%d/delete", runID, exID),
		map[string]string{"runID": fmt.Sprintf("%d", runID), "exerciseID": fmt.Sprintf("%d", exID)})
	req = withOpenOwner(t, req, attacker.ID)
	rr := httptest.NewRecorder()

	srv.handleOpenDeleteExercise(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when a different owner attempts delete, got %d", rr.Code)
	}

	// Exercise and its logged data must remain untouched.
	var exCount int64
	store.DB.Model(&db.Exercise{}).Where("id = ?", exID).Count(&exCount)
	if exCount != 1 {
		t.Errorf("expected exercise to survive a cross-owner delete attempt, count = %d", exCount)
	}
	var completions []db.RunExerciseCompletion
	store.DB.Where("exercise_id = ?", exID).Find(&completions)
	if len(completions) != 1 {
		t.Errorf("expected completion record to survive cross-owner delete attempt, found %d", len(completions))
	}
}

func TestHandleOpenDeleteExercise_ClosedRunBlocked(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "open-delete-closed.db"))
	if err != nil {
		t.Fatal(err)
	}
	user := &db.User{Email: "c@c.com", PasswordHash: "x"}
	if err := store.DB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	ownerID := user.ID

	runID, exID := seedOpenRunWithCompletedExercise(t, store, ownerID)

	// Mark the run completed — deletion should no longer be permitted.
	if err := store.DB.Model(&db.SessionRun{}).Where("id = ?", runID).Update("status", db.RunStatusCompleted).Error; err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(store, "secret", 24, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	req := openChiRequest(t, http.MethodPost, fmt.Sprintf("/runs/%d/open/exercises/%d/delete", runID, exID),
		map[string]string{"runID": fmt.Sprintf("%d", runID), "exerciseID": fmt.Sprintf("%d", exID)})
	req = withOpenOwner(t, req, ownerID)
	rr := httptest.NewRecorder()

	srv.handleOpenDeleteExercise(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when run is not active (completed), got %d", rr.Code)
	}

	var exCount int64
	store.DB.Model(&db.Exercise{}).Where("id = ?", exID).Count(&exCount)
	if exCount != 1 {
		t.Errorf("expected exercise to survive delete attempt on a completed run, count = %d", exCount)
	}
}

func TestHandleOpenDeleteExercise_UnknownExerciseNotFound(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "open-delete-missing.db"))
	if err != nil {
		t.Fatal(err)
	}
	user := &db.User{Email: "d@d.com", PasswordHash: "x"}
	if err := store.DB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	ownerID := user.ID

	runID, _ := seedOpenRunWithCompletedExercise(t, store, ownerID)

	srv, err := NewServer(store, "secret", 24, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	const bogusExerciseID = 999999
	req := openChiRequest(t, http.MethodPost, fmt.Sprintf("/runs/%d/open/exercises/%d/delete", runID, bogusExerciseID),
		map[string]string{"runID": fmt.Sprintf("%d", runID), "exerciseID": fmt.Sprintf("%d", bogusExerciseID)})
	req = withOpenOwner(t, req, ownerID)
	rr := httptest.NewRecorder()

	srv.handleOpenDeleteExercise(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown exercise id, got %d", rr.Code)
	}
}

func TestHandleOpenDeleteExercise_MethodNotAllowed(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "open-delete-method.db"))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(store, "secret", 24, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	req := openChiRequest(t, http.MethodGet, "/runs/1/open/exercises/1/delete",
		map[string]string{"runID": "1", "exerciseID": "1"})
	rr := httptest.NewRecorder()

	srv.handleOpenDeleteExercise(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for GET, got %d", rr.Code)
	}
}
