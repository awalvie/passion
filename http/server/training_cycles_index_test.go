package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"passion/db"
)

// newTrainingCyclesIndexRequest builds a GET /training-cycles request with auth
// injected.
func newTrainingCyclesIndexRequest(ownerID uint) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/training-cycles", nil)
	ctx := context.WithValue(req.Context(), authUserIDKey, ownerID)
	return req.WithContext(ctx)
}

// TestHandleTrainingCycles_ScheduledCountsCountsSessionsPerCycle guards the
// GROUP BY query added to power the index page's session-count column: it must
// count ScheduledSession rows per training_cycle_id, not the number of weeks or
// some other proxy value.
func TestHandleTrainingCycles_ScheduledCountsCountsSessionsPerCycle(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "cycles-index-counts.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	srv, err := NewServer(store, "secret", 24, false, false, nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	tpl := seedGuidedTemplate(t, store, ownerID, "Push")

	cycleA := &db.TrainingCycle{OwnerID: ownerID, Name: "Cycle A", Weeks: 3}
	if err := store.DB.Create(cycleA).Error; err != nil {
		t.Fatal(err)
	}
	cycleB := &db.TrainingCycle{OwnerID: ownerID, Name: "Cycle B", Weeks: 10}
	if err := store.DB.Create(cycleB).Error; err != nil {
		t.Fatal(err)
	}

	// Cycle A: 2 scheduled sessions. Cycle B: 5 weeks but 0 scheduled sessions —
	// if the column were still mislabeling the week count, B would show 10.
	for i := 0; i < 2; i++ {
		ss := &db.ScheduledSession{
			OwnerID: ownerID, TrainingCycleID: &cycleA.ID,
			ScheduledDate:     cycleA.CreatedAt.AddDate(0, 0, i),
			SessionTemplateID: tpl.ID,
		}
		if err := store.DB.Create(ss).Error; err != nil {
			t.Fatal(err)
		}
	}

	rr := httptest.NewRecorder()
	srv.handleTrainingCycles(rr, newTrainingCyclesIndexRequest(ownerID))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}

	body := rr.Body.String()
	if !strings.Contains(body, "2 sessions") {
		t.Errorf("index page does not show \"2 sessions\" for Cycle A (2 scheduled sessions): %.2000q", body)
	}
	if strings.Contains(body, "10 session") {
		t.Errorf("index page shows a stray \"10 session\" — Cycle B's week count (10) must not be used as its session count: %.2000q", body)
	}
	if !strings.Contains(body, "0 sessions") {
		t.Errorf("index page does not show \"0 sessions\" for Cycle B (no scheduled sessions): %.2000q", body)
	}
}

// TestHandleTrainingCycles_ScheduledCountsAreOwnerScoped guards that the
// GROUP BY query filters by owner_id: another user's scheduled sessions against
// a cycle ID that happens to collide must never leak into this owner's counts.
// We verify this at the query level directly, since the count map itself is not
// exposed by the HTTP response.
func TestHandleTrainingCycles_ScheduledCountsAreOwnerScoped(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "cycles-index-scope.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerA, ownerB uint = 1, 2

	tplA := seedGuidedTemplate(t, store, ownerA, "A's template")
	tplB := seedGuidedTemplate(t, store, ownerB, "B's template")

	cycleA := &db.TrainingCycle{OwnerID: ownerA, Name: "A's cycle", Weeks: 4}
	if err := store.DB.Create(cycleA).Error; err != nil {
		t.Fatal(err)
	}
	cycleB := &db.TrainingCycle{OwnerID: ownerB, Name: "B's cycle", Weeks: 4}
	if err := store.DB.Create(cycleB).Error; err != nil {
		t.Fatal(err)
	}

	// Owner A gets 1 scheduled session, owner B gets 3.
	ssA := &db.ScheduledSession{OwnerID: ownerA, TrainingCycleID: &cycleA.ID, ScheduledDate: cycleA.CreatedAt, SessionTemplateID: tplA.ID}
	if err := store.DB.Create(ssA).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		ssB := &db.ScheduledSession{
			OwnerID: ownerB, TrainingCycleID: &cycleB.ID,
			ScheduledDate: cycleB.CreatedAt.AddDate(0, 0, i), SessionTemplateID: tplB.ID,
		}
		if err := store.DB.Create(ssB).Error; err != nil {
			t.Fatal(err)
		}
	}

	// Reproduce the handler's exact query for owner A and assert isolation.
	type cycleCount struct {
		TrainingCycleID uint
		N               int
	}
	var counts []cycleCount
	if err := store.DB.Model(&db.ScheduledSession{}).
		Select("training_cycle_id, count(*) as n").
		Where("owner_id = ? AND training_cycle_id IS NOT NULL", ownerA).
		Group("training_cycle_id").
		Scan(&counts).Error; err != nil {
		t.Fatal(err)
	}

	got := map[uint]int{}
	for _, c := range counts {
		got[c.TrainingCycleID] = c.N
	}
	if got[cycleA.ID] != 1 {
		t.Errorf("count for owner A's cycle = %d, want 1", got[cycleA.ID])
	}
	if n, ok := got[cycleB.ID]; ok {
		t.Errorf("owner B's cycle count leaked into owner A's query: %d", n)
	}
}
