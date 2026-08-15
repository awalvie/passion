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

// TestHandleTrainingCyclesGuided_RoundRobinAssignsSessionsAcrossDays guards the
// round-robin assignment: sorted days are paired with the chosen sessions in
// order, wrapping around when there are more days than sessions.
func TestHandleTrainingCyclesGuided_RoundRobinAssignsSessionsAcrossDays(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "guided-roundrobin.db"))
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
	form.Add("session", strconv.FormatUint(uint64(tplA.ID), 10))
	form.Add("session", strconv.FormatUint(uint64(tplB.ID), 10))
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
			t.Errorf("weekday %d mapped to template %d, want %d (round-robin mismatch)", m.Weekday, got, wantTpl)
		}
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

	form := url.Values{}
	for wd := 1; wd <= 7; wd++ {
		form.Add("day", strconv.Itoa(wd))
	}
	form.Add("session", strconv.FormatUint(uint64(tpl.ID), 10))
	form.Add("weeks", "2")

	srv := &Server{store: store}
	rr := httptest.NewRecorder()
	srv.handleTrainingCyclesGuided(rr, newGuidedRequest(t, ownerID, form))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}
	cycleID := cycleIDFromRedirect(t, rr)

	today := localDate(time.Now())
	week1Monday := mondayOfLocalDate(today)
	wantCount := 7 // week 2 is always scheduled in full
	for d := 0; d < 7; d++ {
		if !week1Monday.AddDate(0, 0, d).Before(today) {
			wantCount++
		}
	}

	var got int64
	if err := store.DB.Model(&db.ScheduledSession{}).
		Where("training_cycle_id = ?", cycleID).Count(&got).Error; err != nil {
		t.Fatal(err)
	}
	if int(got) != wantCount {
		t.Errorf("scheduled session count = %d, want %d (today=%s, week1Monday=%s)",
			got, wantCount, today.Format("2006-01-02"), week1Monday.Format("2006-01-02"))
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
	form.Add("session", strconv.FormatUint(uint64(tpl.ID), 10))
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

	today := localDate(time.Now())
	week1Monday := mondayOfLocalDate(today)
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
	form.Add("session", strconv.FormatUint(uint64(tpl.ID), 10))
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
// required-field validation: submitting with no chosen day, no chosen
// session, or neither must reject with 400 and create nothing.
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
		{"no day", url.Values{"session": {strconv.FormatUint(uint64(tpl.ID), 10)}}},
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
	form.Add("session", strconv.FormatUint(uint64(tplB.ID), 10))
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
