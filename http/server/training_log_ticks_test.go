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

// Most of the writing in a climbing session lives on the climbs, not the exercises: the
// owner's database held 28 ticks with thoughts against 15 exercise notes. The run summary
// rendered them from the start. The training log had no Ticks field at all, so a session
// full of notes read as empty and looked like data loss.
func TestTrainingLogShowsTheNotesWrittenOnEachClimb(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "log-ticks.db"))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(store, "test-secret-at-least-32-characters!!", time.Hour, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1

	tpl := &db.SessionTemplate{OwnerID: ownerID, Name: "Lead Session"}
	if err := store.DB.Create(tpl).Error; err != nil {
		t.Fatal(err)
	}
	act := &db.Activity{OwnerID: ownerID, SessionTemplateID: tpl.ID, Type: "activity", Name: "Routes"}
	if err := store.DB.Create(act).Error; err != nil {
		t.Fatal(err)
	}
	ex := &db.Exercise{OwnerID: ownerID, ActivityID: &act.ID, Name: "Lead Routes", Kind: "climbing"}
	if err := store.DB.Create(ex).Error; err != nil {
		t.Fatal(err)
	}
	ss := &db.ScheduledSession{OwnerID: ownerID, ScheduledDate: time.Now(), SessionTemplateID: tpl.ID}
	if err := store.DB.Create(ss).Error; err != nil {
		t.Fatal(err)
	}
	run, err := db.StartRunForScheduledSession(store.DB, ownerID, ss.ID, false, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var runEx db.Exercise
	if err := store.DB.Where("session_run_id = ?", run.ID).First(&runEx).Error; err != nil {
		t.Fatal(err)
	}

	const thoughts = "Accidentally Z clipped on the third hold"
	const focus = "stay relaxed and breathe"
	if err := store.DB.Create(&db.ClimbingTick{
		OwnerID: ownerID, RunID: run.ID, ExerciseID: runEx.ID,
		Kind: "sport", Setting: "indoor", Grade: "6a",
		Focus: focus, Thoughts: thoughts, Attempts: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	journal := &db.SessionJournal{OwnerID: ownerID, RunID: &run.ID, Date: time.Now()}
	if err := store.DB.Create(journal).Error; err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/training-log/%d", journal.ID), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("journalID", fmt.Sprintf("%d", journal.ID))
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, authUserIDKey, ownerID)
	rr := httptest.NewRecorder()
	srv.handleTrainingLogView(rr, req.WithContext(ctx))

	if rr.Code != http.StatusOK {
		t.Fatalf("the training log returned %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, thoughts) {
		t.Error("the log does not show what the athlete wrote about the climb")
	}
	if !strings.Contains(body, focus) {
		t.Error("the log does not show the focus set before the climb")
	}
	if !strings.Contains(body, "6a") {
		t.Error("the log does not show the climb itself")
	}
}
