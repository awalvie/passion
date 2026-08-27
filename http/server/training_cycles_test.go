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

	"passion/db"
)

// seedGuidedTemplate creates a minimal SessionTemplate owned by ownerID.
func seedGuidedTemplate(t *testing.T, store *db.Store, ownerID uint, name string) *db.SessionTemplate {
	t.Helper()
	tpl := &db.SessionTemplate{OwnerID: ownerID, Name: name}
	if err := store.DB.Create(tpl).Error; err != nil {
		t.Fatalf("create template %q: %v", name, err)
	}
	return tpl
}

// newGuidedRequest builds a POST /training-cycles/guided request with the
// given form values and auth injected.
func newGuidedRequest(t *testing.T, ownerID uint, form url.Values) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/training-cycles/guided", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := context.WithValue(req.Context(), authUserIDKey, ownerID)
	return req.WithContext(ctx)
}

// cycleIDFromRedirect extracts the numeric cycle ID from a
// "/training-cycles/{id}" Location header set by http.Redirect.
func cycleIDFromRedirect(t *testing.T, rr *httptest.ResponseRecorder) uint {
	t.Helper()
	loc := rr.Header().Get("Location")
	const prefix = "/training-cycles/"
	if !strings.HasPrefix(loc, prefix) {
		t.Fatalf("Location = %q, want prefix %q", loc, prefix)
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(loc, prefix), 10, 64)
	if err != nil {
		t.Fatalf("Location %q did not end in a cycle id: %v", loc, err)
	}
	return uint(id)
}

// sessionDayKey builds the per-day session select field name for a weekday
// (Mon=1..Sun=7), matching the guided builder's session_day_<weekday> contract.
func sessionDayKey(weekday int) string {
	return "session_day_" + strconv.Itoa(weekday)
}

// TestHandleTrainingCyclesGuided_MapsEachDayToItsOwnSession guards the
// per-day contract: there is no round-robin in the handler anymore — each
// chosen day carries its own session_day_<weekday> select, and the handler
// must map each day to exactly the session submitted for it.
func TestHandleTrainingCyclesGuided_MapsEachDayToItsOwnSession(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-perday.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tplA := seedGuidedTemplate(t, store, ownerID, "A")
	tplB := seedGuidedTemplate(t, store, ownerID, "B")

	form := url.Values{}
	// Days submitted out of order to also exercise the sort.
	form.Add("day", "5")
	form.Add("day", "1")
	form.Add("day", "3")
	form.Add("day", "7")
	// Each day gets its own explicit session assignment (no rotation logic
	// to verify here — just that the handler respects the per-day value).
	form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tplA.ID), 10))
	form.Set(sessionDayKey(3), strconv.FormatUint(uint64(tplB.ID), 10))
	form.Set(sessionDayKey(5), strconv.FormatUint(uint64(tplA.ID), 10))
	form.Set(sessionDayKey(7), strconv.FormatUint(uint64(tplB.ID), 10))
	form.Add("weeks", "1")

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}
	cycleID := cycleIDFromRedirect(t, rr)

	var mappings []db.TrainingCycleWeekdayMapping
	if err := store.DB.Where("training_cycle_id = ?", cycleID).Order("weekday asc").Find(&mappings).Error; err != nil {
		t.Fatal(err)
	}

	want := map[int]uint{1: tplA.ID, 3: tplB.ID, 5: tplA.ID, 7: tplB.ID}
	if len(mappings) != len(want) {
		t.Fatalf("mapping count = %d, want %d (%+v)", len(mappings), len(want), mappings)
	}
	for _, m := range mappings {
		if got, wantTpl := m.SessionTemplateID, want[m.Weekday]; got != wantTpl {
			t.Errorf("weekday %d mapped to template %d, want %d (per-day mismatch)", m.Weekday, got, wantTpl)
		}
	}
}

// TestHandleTrainingCyclesGuided_SkipsDaysWithoutASession guards that a day
// present in `day` but missing (or invalid) session_day_<weekday> is simply
// left unmapped rather than falling back to some other day's session.
func TestHandleTrainingCyclesGuided_SkipsDaysWithoutASession(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-partial-day.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tplA := seedGuidedTemplate(t, store, ownerID, "A")

	form := url.Values{}
	form.Add("day", "1")
	form.Add("day", "3")
	form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tplA.ID), 10))
	// day 3 deliberately has no session_day_3 value.
	form.Add("weeks", "1")

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}
	cycleID := cycleIDFromRedirect(t, rr)

	var mappings []db.TrainingCycleWeekdayMapping
	if err := store.DB.Where("training_cycle_id = ?", cycleID).Find(&mappings).Error; err != nil {
		t.Fatal(err)
	}
	if len(mappings) != 1 || mappings[0].Weekday != 1 {
		t.Errorf("mappings = %+v, want exactly one mapping for weekday 1", mappings)
	}
}

// TestHandleTrainingCyclesGuided_SkipsPastDatesInFirstWeekOnly guards that the
// guided flow mirrors the manual flow's "skip dates before start" behavior:
// only days already past in the current week are skipped, every later week is
// scheduled in full.
func TestHandleTrainingCyclesGuided_SkipsPastDatesInFirstWeekOnly(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-skip.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tpl := seedGuidedTemplate(t, store, ownerID, "Everyday")

	// An explicit Thursday start: week 1 keeps Thu-Sun, week 2 is full. The default
	// start is a Monday precisely so this truncation does NOT happen unasked — see
	// TestHandleTrainingCyclesGuided_DefaultsToNextWeekMonday.
	start := nextWeekMondayOfLocalDate(time.Now()).AddDate(0, 0, 3)
	if start.Weekday() != time.Thursday {
		t.Fatalf("test setup: start %s is %v, want Thursday", localDateKey(start), start.Weekday())
	}

	form := url.Values{}
	for wd := 1; wd <= 7; wd++ {
		form.Add("day", strconv.Itoa(wd))
		form.Set(sessionDayKey(wd), strconv.FormatUint(uint64(tpl.ID), 10))
	}
	form.Add("weeks", "2")
	form.Set("start_date", localDateKey(start))

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}
	cycleID := cycleIDFromRedirect(t, rr)

	week1Monday := mondayOfLocalDate(start)
	wantCount := 7 // week 2 is always scheduled in full
	for d := 0; d < 7; d++ {
		if !week1Monday.AddDate(0, 0, d).Before(start) {
			wantCount++
		}
	}

	var got int64
	if err := store.DB.Model(&db.ScheduledSession{}).
		Where("training_cycle_id = ?", cycleID).Count(&got).Error; err != nil {
		t.Fatal(err)
	}
	if int(got) != wantCount {
		t.Errorf("scheduled session count = %d, want %d (start=%s, week1Monday=%s)",
			got, wantCount, localDateKey(start), localDateKey(week1Monday))
	}
}

// TestHandleTrainingCyclesGuided_DeloadCreatesNonBlockingRestEvent guards that
// a requested deload week (weeks >= 2) creates a rest CalendarEvent over the
// final week with Blocks explicitly false, defeating GORM's default-true
// zero-value omission on the initial insert.
func TestHandleTrainingCyclesGuided_DeloadCreatesNonBlockingRestEvent(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-deload.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tpl := seedGuidedTemplate(t, store, ownerID, "Deload Test")

	form := url.Values{}
	form.Add("day", "1")
	form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tpl.ID), 10))
	form.Add("weeks", "3")
	form.Add("deload", "1")

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}

	var events []db.CalendarEvent
	if err := store.DB.Where("owner_id = ? AND title = ?", ownerID, "Deload week").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("deload event count = %d, want 1", len(events))
	}
	ev := events[0]
	if ev.Blocks {
		t.Errorf("deload event Blocks = true, want false (must not block the calendar)")
	}
	if ev.Kind != "rest" {
		t.Errorf("deload event Kind = %q, want %q", ev.Kind, "rest")
	}

	// Derived from the cycle's actual start rather than from today, so this stays
	// correct whatever the default start date is.
	cycleID := cycleIDFromRedirect(t, rr)
	var cycle db.TrainingCycle
	if err := store.DB.First(&cycle, cycleID).Error; err != nil {
		t.Fatal(err)
	}
	week1Monday := mondayOfLocalDate(cycle.StartDate)
	wantStart := week1Monday.AddDate(0, 0, (3-1)*7)
	wantEnd := wantStart.AddDate(0, 0, 6)
	if !ev.StartDate.Equal(wantStart) {
		t.Errorf("deload StartDate = %s, want %s (final week start)", ev.StartDate.Format("2006-01-02"), wantStart.Format("2006-01-02"))
	}
	if !ev.EndDate.Equal(wantEnd) {
		t.Errorf("deload EndDate = %s, want %s (final week end)", ev.EndDate.Format("2006-01-02"), wantEnd.Format("2006-01-02"))
	}
}

// TestHandleTrainingCyclesGuided_DeloadSkippedUnderTwoWeeks guards the weeks<2
// edge: a single-week cycle has no "final week" distinct from week one, so no
// deload event should be created even when requested.
func TestHandleTrainingCyclesGuided_DeloadSkippedUnderTwoWeeks(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-deload-short.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tpl := seedGuidedTemplate(t, store, ownerID, "Short Cycle")

	form := url.Values{}
	form.Add("day", "1")
	form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tpl.ID), 10))
	form.Add("weeks", "1")
	form.Add("deload", "1")

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}

	var count int64
	store.DB.Model(&db.CalendarEvent{}).Where("owner_id = ? AND title = ?", ownerID, "Deload week").Count(&count)
	if count != 0 {
		t.Errorf("deload event count = %d for a 1-week cycle, want 0", count)
	}
}

// TestHandleTrainingCyclesGuided_RejectsMissingDaysOrSessions guards the
// required-field validation: submitting with no chosen day, no valid
// session_day_<weekday>, or neither must reject with 400 and create nothing.
func TestHandleTrainingCyclesGuided_RejectsMissingDaysOrSessions(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-missing.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tpl := seedGuidedTemplate(t, store, ownerID, "Whatever")

	cases := []struct {
		name string
		form url.Values
	}{
		{"no day", url.Values{sessionDayKey(1): {strconv.FormatUint(uint64(tpl.ID), 10)}}},
		{"no session", url.Values{"day": {"1"}}},
		{"neither", url.Values{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := &Server{store: store}
			rr := httptest.NewRecorder()
			srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, tc.form))
			if rr.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
			}
		})
	}

	var cycleCount int64
	store.DB.Model(&db.TrainingCycle{}).Where("owner_id = ?", ownerID).Count(&cycleCount)
	if cycleCount != 0 {
		t.Errorf("cycle count = %d after rejected submissions, want 0", cycleCount)
	}
}

// TestHandleTrainingCyclesGuided_EquipmentJoinedIntoNotes guards the
// equipment-to-notes persistence: multiple "equipment" form values must be
// trimmed, joined with ", ", and stored as "Equipment: a, b, c" in cycle.Notes
// — the only place this data is persisted (there is no dedicated column).
func TestHandleTrainingCyclesGuided_EquipmentJoinedIntoNotes(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-equipment.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tpl := seedGuidedTemplate(t, store, ownerID, "Equipment Test")

	form := url.Values{}
	form.Add("day", "1")
	form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tpl.ID), 10))
	form.Add("weeks", "1")
	form.Add("equipment", "  hangboard  ")
	form.Add("equipment", "rings")
	form.Add("equipment", "") // blank values must be dropped, not joined as ""

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}
	cycleID := cycleIDFromRedirect(t, rr)

	var cycle db.TrainingCycle
	if err := store.DB.First(&cycle, cycleID).Error; err != nil {
		t.Fatal(err)
	}
	want := "Equipment: hangboard, rings"
	if cycle.Notes != want {
		t.Errorf("Notes = %q, want %q", cycle.Notes, want)
	}
}

// TestHandleTrainingCyclesGuided_NoEquipmentLeavesNotesEmpty guards the other
// side of the branch: when no equipment tags are submitted, Notes must stay
// empty rather than storing a stray "Equipment: " prefix.
func TestHandleTrainingCyclesGuided_NoEquipmentLeavesNotesEmpty(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-no-equipment.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tpl := seedGuidedTemplate(t, store, ownerID, "No Equipment Test")

	form := url.Values{}
	form.Add("day", "1")
	form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tpl.ID), 10))
	form.Add("weeks", "1")

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}
	cycleID := cycleIDFromRedirect(t, rr)

	var cycle db.TrainingCycle
	if err := store.DB.First(&cycle, cycleID).Error; err != nil {
		t.Fatal(err)
	}
	if cycle.Notes != "" {
		t.Errorf("Notes = %q, want empty when no equipment submitted", cycle.Notes)
	}
}

// TestHandleTrainingCyclesGuided_RejectsUnownedSessionTemplate guards owner
// scoping: a session template ID belonging to another user must be filtered
// out, not silently attached to the requesting owner's cycle.
func TestHandleTrainingCyclesGuided_RejectsUnownedSessionTemplate(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-unowned.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerA uint = 1
	const ownerB uint = 2
	tplB := seedGuidedTemplate(t, store, ownerB, "Belongs to B")

	form := url.Values{}
	form.Add("day", "1")
	form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tplB.ID), 10))
	form.Add("weeks", "2")

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerA, form))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (owner B's template must not be usable by owner A)", rr.Code, http.StatusBadRequest)
	}

	var cycleCount int64
	store.DB.Model(&db.TrainingCycle{}).Where("owner_id = ?", ownerA).Count(&cycleCount)
	if cycleCount != 0 {
		t.Errorf("cycle count = %d for owner A, want 0 (no cycle should be created from an unowned template)", cycleCount)
	}

	var mappingCount int64
	store.DB.Model(&db.TrainingCycleWeekdayMapping{}).Where("session_template_id = ?", tplB.ID).Count(&mappingCount)
	if mappingCount != 0 {
		t.Errorf("mapping count referencing owner B's template = %d, want 0", mappingCount)
	}
}

// TestHandleTrainingCyclesGuided_CreatesGoalRows guards first-class goal
// persistence: parallel goal_before[]/goal_after[] arrays become CycleGoal
// rows in order, a fully-empty pair is skipped (not stored as a blank row),
// and the count is capped at 5 even when more pairs are submitted.
func TestHandleTrainingCyclesGuided_CreatesGoalRows(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-goals.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tpl := seedGuidedTemplate(t, store, ownerID, "Goals Test")

	form := url.Values{}
	form.Add("day", "1")
	form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tpl.ID), 10))
	form.Add("weeks", "1")
	// 7 pairs submitted: one fully-empty (must be skipped) plus 6 real ones
	// (must be capped at 5).
	befores := []string{"V4", "", "5.10a", "20kg", "10s", "3", "2"}
	afters := []string{"V5", "", "5.10c", "25kg", "15s", "5", "4"}
	hows := []string{"one lead session a week", "", "", "", "", "", ""}
	for i := range befores {
		form.Add("goal_before", befores[i])
		form.Add("goal_after", afters[i])
		form.Add("goal_how", hows[i])
	}

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}
	cycleID := cycleIDFromRedirect(t, rr)

	var goals []db.CycleGoal
	if err := store.DB.Where("owner_id = ? AND training_cycle_id = ?", ownerID, cycleID).
		Order("order_index asc").Find(&goals).Error; err != nil {
		t.Fatal(err)
	}
	if len(goals) != 5 {
		t.Fatalf("goal count = %d, want 5 (capped, empty pair skipped); got %+v", len(goals), goals)
	}
	if goals[0].Before != "V4" || goals[0].After != "V5" {
		t.Errorf("goals[0] = %+v, want Before=V4 After=V5", goals[0])
	}
	if goals[0].How != "one lead session a week" {
		t.Errorf("goals[0].How = %q, want %q", goals[0].How, "one lead session a week")
	}
	// The empty pair ("", "") must not appear anywhere in the surviving rows.
	for _, g := range goals {
		if g.Before == "" && g.After == "" {
			t.Errorf("found a fully-empty goal row %+v, want it skipped", g)
		}
	}
}

// TestHandleTrainingCyclesGuided_SkipsEmptyGoalPair guards the other side of
// the goals branch: when both before and after are blank for every
// submitted row, no CycleGoal rows should be created at all.
func TestHandleTrainingCyclesGuided_SkipsEmptyGoalPair(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-goals-empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tpl := seedGuidedTemplate(t, store, ownerID, "Empty Goals Test")

	form := url.Values{}
	form.Add("day", "1")
	form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tpl.ID), 10))
	form.Add("weeks", "1")
	form.Add("goal_before", "  ")
	form.Add("goal_after", "")

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}
	cycleID := cycleIDFromRedirect(t, rr)

	var count int64
	store.DB.Model(&db.CycleGoal{}).Where("training_cycle_id = ?", cycleID).Count(&count)
	if count != 0 {
		t.Errorf("goal count = %d, want 0 when only a blank pair is submitted", count)
	}
}

// TestHandleTrainingCyclesGuided_EnergyFoldedIntoNotes guards the per-day
// energy annotation: non-moderate days must appear in Notes as
// "Energy: Mon hard · Fri easy" (day-order, using the 3-letter weekday
// label), while "moderate" (the neutral baseline) is omitted entirely. It
// must also coexist with an equipment line, newline-separated.
func TestHandleTrainingCyclesGuided_EnergyFoldedIntoNotes(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-energy.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tpl := seedGuidedTemplate(t, store, ownerID, "Energy Test")

	form := url.Values{}
	for _, wd := range []int{1, 3, 5} {
		form.Add("day", strconv.Itoa(wd))
		form.Set(sessionDayKey(wd), strconv.FormatUint(uint64(tpl.ID), 10))
	}
	form.Set("energy_day_1", "hard")
	form.Set("energy_day_3", "moderate") // neutral baseline, must be omitted
	form.Set("energy_day_5", "easy")
	form.Add("equipment", "hangboard")
	form.Add("weeks", "1")

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}
	cycleID := cycleIDFromRedirect(t, rr)

	var cycle db.TrainingCycle
	if err := store.DB.First(&cycle, cycleID).Error; err != nil {
		t.Fatal(err)
	}
	want := "Equipment: hangboard\nEnergy: Mon hard · Fri easy"
	if cycle.Notes != want {
		t.Errorf("Notes = %q, want %q", cycle.Notes, want)
	}
}

// TestHandleTrainingCyclesGuided_AllModerateEnergyOmitsEnergyLine guards that
// when every chosen day is left at the "moderate" baseline (or unset), no
// "Energy:" line is added to Notes at all.
func TestHandleTrainingCyclesGuided_AllModerateEnergyOmitsEnergyLine(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-energy-moderate.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tpl := seedGuidedTemplate(t, store, ownerID, "Moderate Test")

	form := url.Values{}
	form.Add("day", "1")
	form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tpl.ID), 10))
	form.Set("energy_day_1", "moderate")
	form.Add("weeks", "1")

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}
	cycleID := cycleIDFromRedirect(t, rr)

	var cycle db.TrainingCycle
	if err := store.DB.First(&cycle, cycleID).Error; err != nil {
		t.Fatal(err)
	}
	if cycle.Notes != "" {
		t.Errorf("Notes = %q, want empty when all days are moderate", cycle.Notes)
	}
}

// TestHandleTrainingCyclesGuided_RestPeriodCreatesNonBlockingEvent guards
// the optional rest-period range: a valid rest_start/rest_end produces
// exactly one non-blocking CalendarEvent titled "Rest period" spanning that
// range, in addition to any deload event.
func TestHandleTrainingCyclesGuided_RestPeriodCreatesNonBlockingEvent(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-rest.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tpl := seedGuidedTemplate(t, store, ownerID, "Rest Test")

	form := url.Values{}
	form.Add("day", "1")
	form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tpl.ID), 10))
	form.Add("weeks", "2")
	form.Set("rest_enabled", "1")
	form.Set("rest_start", "2026-09-01")
	form.Set("rest_end", "2026-09-05")

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}

	var events []db.CalendarEvent
	if err := store.DB.Where("owner_id = ? AND title = ?", ownerID, "Rest period").Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("rest period event count = %d, want 1", len(events))
	}
	ev := events[0]
	if ev.Blocks {
		t.Errorf("rest period event Blocks = true, want false (must not block the calendar)")
	}
	if ev.Kind != "rest" {
		t.Errorf("rest period event Kind = %q, want %q", ev.Kind, "rest")
	}
	loc := time.Now().Location()
	wantStart, _ := time.ParseInLocation("2006-01-02", "2026-09-01", loc)
	wantEnd, _ := time.ParseInLocation("2006-01-02", "2026-09-05", loc)
	if !ev.StartDate.Equal(localDate(wantStart)) {
		t.Errorf("rest period StartDate = %s, want %s", ev.StartDate.Format("2006-01-02"), wantStart.Format("2006-01-02"))
	}
	if !ev.EndDate.Equal(localDate(wantEnd)) {
		t.Errorf("rest period EndDate = %s, want %s", ev.EndDate.Format("2006-01-02"), wantEnd.Format("2006-01-02"))
	}
}

// TestHandleTrainingCyclesGuided_InvalidOrMissingRestRangeSkipsEvent guards
// that a missing or invalid rest range is silently skipped — it's an
// optional field, not a hard validation error — and the request still
// succeeds with a redirect.
func TestHandleTrainingCyclesGuided_InvalidOrMissingRestRangeSkipsEvent(t *testing.T) {
	cases := []struct {
		name  string
		start string
		end   string
	}{
		{"missing both", "", ""},
		{"invalid dates", "not-a-date", "also-not-a-date"},
		{"end before start", "2026-09-05", "2026-09-01"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-rest-invalid.db"))
			if err != nil {
				t.Fatal(err)
			}
			const ownerID uint = 1
			tpl := seedGuidedTemplate(t, store, ownerID, "Invalid Rest Test")

			form := url.Values{}
			form.Add("day", "1")
			form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tpl.ID), 10))
			form.Add("weeks", "1")
			form.Set("rest_enabled", "1")
			form.Set("rest_start", tc.start)
			form.Set("rest_end", tc.end)

			srv := &Server{store: store}
			rr := httptest.NewRecorder()
			srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
			if rr.Code != http.StatusSeeOther {
				t.Fatalf("status = %d, want %d (invalid rest range must not hard-fail); body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
			}

			var count int64
			store.DB.Model(&db.CalendarEvent{}).Where("owner_id = ? AND title = ?", ownerID, "Rest period").Count(&count)
			if count != 0 {
				t.Errorf("rest period event count = %d, want 0 for %s", count, tc.name)
			}
		})
	}
}

// TestHandleCycleDetailsSave_ReplacesGoalsAndClearsLegacyColumn guards the
// autosave replace semantics: saving details deletes the cycle's existing
// CycleGoal rows and recreates them from the submitted goal_before[]/
// goal_after[] arrays (2 existing → 1 submitted must leave exactly 1), and
// clears the legacy TrainingCycle.Goal column so it can't shadow the new list.
func TestHandleCycleDetailsSave_ReplacesGoalsAndClearsLegacyColumn(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "details-save-goals.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 7

	cycle := &db.TrainingCycle{
		OwnerID: ownerID, Name: "Cycle", StartDate: localDate(time.Now()), Weeks: 4,
		Goal: "legacy goal text",
	}
	if err := store.DB.Create(cycle).Error; err != nil {
		t.Fatal(err)
	}
	existingGoals := []db.CycleGoal{
		{OwnerID: ownerID, TrainingCycleID: cycle.ID, Before: "V3", After: "V4", OrderIndex: 0},
		{OwnerID: ownerID, TrainingCycleID: cycle.ID, Before: "5.9", After: "5.10a", OrderIndex: 1},
	}
	if err := store.DB.Create(&existingGoals).Error; err != nil {
		t.Fatal(err)
	}

	form := url.Values{}
	form.Set("name", "Cycle")
	form.Set("notes", "")
	form.Add("goal_before", "V5")
	form.Add("goal_after", "V6")

	req := httptest.NewRequest(http.MethodPost, "/training-cycles/1/details-save", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := context.WithValue(req.Context(), authUserIDKey, ownerID)
	req = req.WithContext(ctx)

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleCycleDetailsSave(rr, req, cycle.ID, ownerID)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}

	var goals []db.CycleGoal
	if err := store.DB.Where("owner_id = ? AND training_cycle_id = ?", ownerID, cycle.ID).Find(&goals).Error; err != nil {
		t.Fatal(err)
	}
	if len(goals) != 1 {
		t.Fatalf("goal count = %d, want 1 after replace; got %+v", len(goals), goals)
	}
	if goals[0].Before != "V5" || goals[0].After != "V6" {
		t.Errorf("goals[0] = %+v, want Before=V5 After=V6", goals[0])
	}

	var saved db.TrainingCycle
	if err := store.DB.First(&saved, cycle.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Goal != "" {
		t.Errorf("legacy Goal column = %q, want cleared to empty", saved.Goal)
	}
}

// TestHandleTrainingCycleDelete_RemovesCycleGoalRows guards that deleting a
// cycle hard-deletes its CycleGoal rows — they're plan metadata with no
// history to preserve, unlike scheduled sessions with runs.
func TestHandleTrainingCycleDelete_RemovesCycleGoalRows(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "delete-goals.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 7

	cycle := &db.TrainingCycle{OwnerID: ownerID, Name: "Cycle", StartDate: localDate(time.Now()), Weeks: 4}
	if err := store.DB.Create(cycle).Error; err != nil {
		t.Fatal(err)
	}
	goal := &db.CycleGoal{OwnerID: ownerID, TrainingCycleID: cycle.ID, Before: "V3", After: "V4"}
	if err := store.DB.Create(goal).Error; err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/training-cycles/1/delete", nil)
	ctx := context.WithValue(req.Context(), authUserIDKey, ownerID)
	req = req.WithContext(ctx)

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCycleDelete(rr, req, cycle.ID, ownerID)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got := rr.Header().Get("HX-Redirect"); got != "/training-cycles" {
		t.Errorf("HX-Redirect = %q, want %q", got, "/training-cycles")
	}

	var count int64
	store.DB.Unscoped().Model(&db.CycleGoal{}).Where("training_cycle_id = ?", cycle.ID).Count(&count)
	if count != 0 {
		t.Errorf("CycleGoal count = %d after delete, want 0 (hard-deleted with the cycle)", count)
	}
}

// TestHandleTrainingCyclesGuided_DefaultsToNextWeekMonday guards the fix for the
// bug where the guided builder hardcoded "starts today": every mapped weekday
// earlier in the week than today was silently dropped from week 1, because
// scheduling anchors on the Monday of the start week and then discards dates
// before the start date. Anchoring the default start on next week's Monday means
// week 1 holds every mapped day whatever weekday the cycle is created on.
func TestHandleTrainingCyclesGuided_DefaultsToNextWeekMonday(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-default-start.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tpl := seedGuidedTemplate(t, store, ownerID, "A")

	// Monday and Sunday: the extremes of the week. Under the old behaviour, one of
	// these was always dropped unless the cycle happened to be created on a Monday.
	form := url.Values{}
	form.Add("day", "1")
	form.Add("day", "7")
	form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tpl.ID), 10))
	form.Set(sessionDayKey(7), strconv.FormatUint(uint64(tpl.ID), 10))
	form.Add("weeks", "1")
	// start_date deliberately absent — this exercises the default.

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}
	cycleID := cycleIDFromRedirect(t, rr)

	var cycle db.TrainingCycle
	if err := store.DB.First(&cycle, cycleID).Error; err != nil {
		t.Fatal(err)
	}
	wantStart := nextWeekMondayOfLocalDate(time.Now())
	if got := localDateKey(cycle.StartDate); got != localDateKey(wantStart) {
		t.Errorf("StartDate = %s, want next week's Monday %s", got, localDateKey(wantStart))
	}
	if wd := cycle.StartDate.Weekday(); wd != time.Monday {
		t.Errorf("StartDate weekday = %v, want Monday", wd)
	}
	if !localDate(cycle.StartDate).After(localDate(time.Now())) {
		t.Errorf("StartDate %s is not in the future", localDateKey(cycle.StartDate))
	}

	// Both mapped weekdays must survive into week 1.
	var sessions []db.ScheduledSession
	if err := store.DB.Where("training_cycle_id = ?", cycleID).
		Order("scheduled_date asc").Find(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("session count = %d, want 2 (a dropped day is the bug this guards)", len(sessions))
	}
	gotDays := map[time.Weekday]bool{}
	for _, ss := range sessions {
		gotDays[ss.ScheduledDate.Weekday()] = true
	}
	for _, want := range []time.Weekday{time.Monday, time.Sunday} {
		if !gotDays[want] {
			t.Errorf("no session scheduled on %v; got %+v", want, gotDays)
		}
	}
}

// TestHandleTrainingCyclesGuided_HonoursExplicitStartDate guards that an explicit
// date always wins over the Monday default, including a mid-week date — where
// dropping the earlier mapped days is the correct, requested behaviour.
func TestHandleTrainingCyclesGuided_HonoursExplicitStartDate(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-explicit-start.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tpl := seedGuidedTemplate(t, store, ownerID, "A")

	// A Wednesday well clear of today, with Monday and Friday mapped: Monday falls
	// before the start and must be dropped, Friday must survive.
	start := nextWeekMondayOfLocalDate(time.Now()).AddDate(0, 0, 2)
	if start.Weekday() != time.Wednesday {
		t.Fatalf("test setup: start %s is %v, want Wednesday", localDateKey(start), start.Weekday())
	}

	form := url.Values{}
	form.Add("day", "1")
	form.Add("day", "5")
	form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tpl.ID), 10))
	form.Set(sessionDayKey(5), strconv.FormatUint(uint64(tpl.ID), 10))
	form.Add("weeks", "1")
	form.Set("start_date", localDateKey(start))

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}
	cycleID := cycleIDFromRedirect(t, rr)

	var cycle db.TrainingCycle
	if err := store.DB.First(&cycle, cycleID).Error; err != nil {
		t.Fatal(err)
	}
	if got := localDateKey(cycle.StartDate); got != localDateKey(start) {
		t.Errorf("StartDate = %s, want the explicit %s", got, localDateKey(start))
	}

	var sessions []db.ScheduledSession
	if err := store.DB.Where("training_cycle_id = ?", cycleID).Find(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("session count = %d, want 1 (Friday only; Monday precedes the start)", len(sessions))
	}
	if wd := sessions[0].ScheduledDate.Weekday(); wd != time.Friday {
		t.Errorf("surviving session is on %v, want Friday", wd)
	}
}

// TestHandleTrainingCyclesGuided_RejectsUnparseableStartDate guards that a
// malformed date is refused rather than silently falling back to the default.
func TestHandleTrainingCyclesGuided_RejectsUnparseableStartDate(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-bad-start.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	tpl := seedGuidedTemplate(t, store, ownerID, "A")

	form := url.Values{}
	form.Add("day", "1")
	form.Set(sessionDayKey(1), strconv.FormatUint(uint64(tpl.ID), 10))
	form.Add("weeks", "1")
	form.Set("start_date", "not-a-date")

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	var n int64
	store.DB.Model(&db.TrainingCycle{}).Count(&n)
	if n != 0 {
		t.Errorf("cycle count = %d, want 0 — nothing should be created on a bad date", n)
	}
}
