package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"passion/db"
)

// seedCycleWithTarget builds a cycle whose Monday template holds one
// reps_and_sets exercise with concrete targets, which is what cycle targets
// resolve against.
func seedCycleWithTarget(t *testing.T, store *db.Store, ownerID uint, sets, reps int, weight float64) (*db.TrainingCycle, *db.Exercise) {
	t.Helper()

	tpl := &db.SessionTemplate{OwnerID: ownerID, Name: "Strength"}
	if err := store.DB.Create(tpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	act := &db.Activity{OwnerID: ownerID, SessionTemplateID: tpl.ID, Name: "Main", OrderIndex: 0}
	if err := store.DB.Create(act).Error; err != nil {
		t.Fatalf("create activity: %v", err)
	}
	ex := &db.Exercise{
		OwnerID: ownerID, ActivityID: &act.ID, Name: "Weighted Pull-up",
		Kind: "reps_and_sets", Sets: sets, Reps: reps, WeightKg: weight,
	}
	if err := store.DB.Create(ex).Error; err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	cycle := &db.TrainingCycle{
		OwnerID: ownerID, Name: "Block", Weeks: 3,
		StartDate: nextWeekMondayOfLocalDate(time.Now()),
	}
	if err := store.DB.Create(cycle).Error; err != nil {
		t.Fatalf("create cycle: %v", err)
	}
	mapping := &db.TrainingCycleWeekdayMapping{
		OwnerID: ownerID, TrainingCycleID: cycle.ID, Weekday: 1, SessionTemplateID: tpl.ID,
	}
	if err := store.DB.Create(mapping).Error; err != nil {
		t.Fatalf("create mapping: %v", err)
	}
	return cycle, ex
}

// newWeekToggleRequest builds the POST that flips an exercise between
// "same every week" and "varies by week".
func newWeekToggleRequest(t *testing.T, cycleID, ownerID uint, exName, mode string) *http.Request {
	t.Helper()
	form := url.Values{}
	form.Set("exercise_name", exName)
	form.Set("mode", mode)
	path := "/training-cycles/" + strconv.FormatUint(uint64(cycleID), 10) + "/week-override-toggle"
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("cycleID", strconv.FormatUint(uint64(cycleID), 10))
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, authUserIDKey, ownerID)
	return req.WithContext(ctx)
}

// TestWeekOverrideToggle_VariesKeepsTemplateTargets guards the bug where turning on
// "varies by week" silently blanked an exercise's targets for every week. The toggle
// creates a cycle-level override row purely to carry the flag, and the resolver
// treated the mere existence of that row as "override everything" — so an all-zero
// row replaced 4x6 @ 12.5kg with 0x0 @ 0kg across the whole cycle.
func TestWeekOverrideToggle_VariesKeepsTemplateTargets(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "varies-keeps.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	cycle, ex := seedCycleWithTarget(t, store, ownerID, 4, 6, 12.5)

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleCycleWeekOverrideToggle(rr, newWeekToggleRequest(t, cycle.ID, ownerID, ex.Name, "varies"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}

	views := srv.buildCycleExerciseOverrides(cycle.ID, ownerID, cycle.Weeks)
	if len(views) != 1 {
		t.Fatalf("view count = %d, want 1", len(views))
	}
	v := views[0]
	if !v.VariesByWeek {
		t.Errorf("VariesByWeek = false, want true — the toggle did not take effect")
	}
	if len(v.WeekOverrides) != 3 {
		t.Fatalf("week count = %d, want 3", len(v.WeekOverrides))
	}
	for _, wk := range v.WeekOverrides {
		if wk.Sets != 4 || wk.Reps != 6 || wk.WeightKg != 12.5 {
			t.Errorf("week %d resolved to %dx%d @ %.1fkg, want 4x6 @ 12.5kg (targets were wiped)",
				wk.Week, wk.Sets, wk.Reps, wk.WeightKg)
		}
	}

	// The stored flag row must carry the template values, not zeros.
	var ov db.CycleExerciseOverride
	if err := store.DB.Where("training_cycle_id = ? AND owner_id = ?", cycle.ID, ownerID).
		First(&ov).Error; err != nil {
		t.Fatal(err)
	}
	if ov.Sets != 4 || ov.Reps != 6 || ov.WeightKg != 12.5 {
		t.Errorf("stored override = %dx%d @ %.1fkg, want the template's 4x6 @ 12.5kg",
			ov.Sets, ov.Reps, ov.WeightKg)
	}
}

// TestWeekOverrideToggle_BodyweightZeroSurvives guards the deliberate carve-out: the
// per-field fallback uses a > 0 guard for sets/reps/rep-seconds but NOT for weight,
// because 0 kg means bodyweight. An explicit 0 kg override must not fall back to the
// template's loaded weight.
func TestWeekOverrideToggle_BodyweightZeroSurvives(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "bodyweight-zero.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	cycle, ex := seedCycleWithTarget(t, store, ownerID, 4, 6, 20)

	// An explicit cycle-level override: same sets/reps, but bodyweight.
	ov := &db.CycleExerciseOverride{
		OwnerID: ownerID, TrainingCycleID: cycle.ID, ExerciseName: ex.Name,
		Sets: 4, Reps: 6, WeightKg: 0,
	}
	if err := store.DB.Create(ov).Error; err != nil {
		t.Fatal(err)
	}

	srv := &Server{store: store}
	views := srv.buildCycleExerciseOverrides(cycle.ID, ownerID, cycle.Weeks)
	if len(views) != 1 {
		t.Fatalf("view count = %d, want 1", len(views))
	}
	for _, wk := range views[0].WeekOverrides {
		if wk.WeightKg != 0 {
			t.Errorf("week %d weight = %.1fkg, want 0 (bodyweight must not fall back to the template's 20kg)",
				wk.Week, wk.WeightKg)
		}
		if wk.Sets != 4 || wk.Reps != 6 {
			t.Errorf("week %d = %dx%d, want 4x6", wk.Week, wk.Sets, wk.Reps)
		}
	}
}

// TestWeekOverrideToggle_SameModeClearsWeekRows guards that switching back to "same
// every week" drops the per-week rows rather than leaving them to shadow the target.
func TestWeekOverrideToggle_SameModeClearsWeekRows(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "same-clears.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	cycle, ex := seedCycleWithTarget(t, store, ownerID, 3, 8, 0)

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleCycleWeekOverrideToggle(rr, newWeekToggleRequest(t, cycle.ID, ownerID, ex.Name, "varies"))
	if rr.Code != http.StatusOK {
		t.Fatalf("varies: status = %d, want %d", rr.Code, http.StatusOK)
	}

	wo := &db.CycleExerciseWeekOverride{
		OwnerID: ownerID, TrainingCycleID: cycle.ID, ExerciseName: ex.Name,
		Week: 2, Sets: 5, Reps: 5,
	}
	if err := store.DB.Create(wo).Error; err != nil {
		t.Fatal(err)
	}

	rr = httptest.NewRecorder()
	srv.handleCycleWeekOverrideToggle(rr, newWeekToggleRequest(t, cycle.ID, ownerID, ex.Name, "same"))
	if rr.Code != http.StatusOK {
		t.Fatalf("same: status = %d, want %d", rr.Code, http.StatusOK)
	}

	var n int64
	if err := store.DB.Model(&db.CycleExerciseWeekOverride{}).
		Where("training_cycle_id = ? AND owner_id = ?", cycle.ID, ownerID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("week override rows = %d, want 0 after switching back to same-every-week", n)
	}

	views := srv.buildCycleExerciseOverrides(cycle.ID, ownerID, cycle.Weeks)
	if len(views) != 1 {
		t.Fatalf("view count = %d, want 1", len(views))
	}
	for _, wk := range views[0].WeekOverrides {
		if wk.Sets != 3 || wk.Reps != 8 {
			t.Errorf("week %d = %dx%d, want the template's 3x8", wk.Week, wk.Sets, wk.Reps)
		}
	}
}
