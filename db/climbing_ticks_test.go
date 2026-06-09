package db

import (
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// GetLatestClimbingTickInRun
// ---------------------------------------------------------------------------

func TestGetLatestClimbingTickInRun_EmptyRun(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "latest-empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, ok := GetLatestClimbingTickInRun(store.DB, 1, 99)
	if ok {
		t.Error("expected ok=false for run with no ticks, got ok=true")
	}
}

func TestGetLatestClimbingTickInRun_MultipleExercisesInterleaved(t *testing.T) {
	// Key invariant: ordering is by created_at/id, NOT by order_index (which is
	// per-exercise and collides across drills in the same run).
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "latest-interleaved.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 10
	base := time.Now().UTC()

	// Exercise A — first tick (order_index 0 within ex A)
	tickA1 := &ClimbingTick{
		OwnerID: ownerID, RunID: runID, ExerciseID: 100,
		Kind: "boulder", Grade: "V2", Attempts: 1,
	}
	if err := CreateClimbingTick(store.DB, tickA1); err != nil {
		t.Fatal(err)
	}
	store.DB.Model(tickA1).Update("created_at", base.Add(-3*time.Minute))

	// Exercise B — second tick in wall-clock time (order_index 0 within ex B)
	tickB1 := &ClimbingTick{
		OwnerID: ownerID, RunID: runID, ExerciseID: 200,
		Kind: "boulder", Grade: "V5", Attempts: 1,
	}
	if err := CreateClimbingTick(store.DB, tickB1); err != nil {
		t.Fatal(err)
	}
	store.DB.Model(tickB1).Update("created_at", base.Add(-2*time.Minute))

	// Exercise A — third tick (order_index 1 within ex A) — chronologically last
	tickA2 := &ClimbingTick{
		OwnerID: ownerID, RunID: runID, ExerciseID: 100,
		Kind: "boulder", Grade: "V9", Attempts: 1,
	}
	if err := CreateClimbingTick(store.DB, tickA2); err != nil {
		t.Fatal(err)
	}
	store.DB.Model(tickA2).Update("created_at", base.Add(-1*time.Minute))

	got, ok := GetLatestClimbingTickInRun(store.DB, ownerID, runID)
	if !ok {
		t.Fatal("expected ok=true, got ok=false")
	}
	if got.ID != tickA2.ID {
		t.Errorf("got tick ID=%d grade=%q, want ID=%d grade=V9 (chronologically last)",
			got.ID, got.Grade, tickA2.ID)
	}
}

func TestGetLatestClimbingTickInRun_OwnerScoped(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "latest-owner.db"))
	if err != nil {
		t.Fatal(err)
	}

	const otherOwner uint = 42
	const myOwner uint = 7
	const runID uint = 5

	// Another owner's tick — should be invisible to myOwner.
	other := &ClimbingTick{
		OwnerID: otherOwner, RunID: runID, ExerciseID: 1,
		Kind: "boulder", Grade: "V9", Attempts: 1,
	}
	if err := CreateClimbingTick(store.DB, other); err != nil {
		t.Fatal(err)
	}

	_, ok := GetLatestClimbingTickInRun(store.DB, myOwner, runID)
	if ok {
		t.Error("got ok=true when myOwner has no ticks; another owner's tick leaked")
	}
}

// ---------------------------------------------------------------------------
// GetClimbingTick — owner + run scoped IDOR guard
// ---------------------------------------------------------------------------

func TestGetClimbingTick_HappyPath(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "gettick-ok.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 10

	tick := &ClimbingTick{
		OwnerID: ownerID, RunID: runID, ExerciseID: 1,
		Kind: "boulder", Grade: "V4", Attempts: 1,
	}
	if err := CreateClimbingTick(store.DB, tick); err != nil {
		t.Fatal(err)
	}

	got, err := GetClimbingTick(store.DB, ownerID, runID, tick.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != tick.ID || got.Grade != "V4" {
		t.Errorf("got {ID:%d Grade:%q}, want {ID:%d Grade:V4}", got.ID, got.Grade, tick.ID)
	}
}

func TestGetClimbingTick_WrongOwnerBlocked(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "gettick-idor-owner.db"))
	if err != nil {
		t.Fatal(err)
	}

	tick := &ClimbingTick{
		OwnerID: 1, RunID: 10, ExerciseID: 1,
		Kind: "boulder", Grade: "V4", Attempts: 1,
	}
	if err := CreateClimbingTick(store.DB, tick); err != nil {
		t.Fatal(err)
	}

	_, err = GetClimbingTick(store.DB, 2 /*attacker*/, 10, tick.ID)
	if err == nil {
		t.Error("expected error (IDOR guard) when owner_id doesn't match, got nil")
	}
}

func TestGetClimbingTick_WrongRunBlocked(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "gettick-idor-run.db"))
	if err != nil {
		t.Fatal(err)
	}

	tick := &ClimbingTick{
		OwnerID: 1, RunID: 10, ExerciseID: 1,
		Kind: "boulder", Grade: "V4", Attempts: 1,
	}
	if err := CreateClimbingTick(store.DB, tick); err != nil {
		t.Fatal(err)
	}

	_, err = GetClimbingTick(store.DB, 1, 99 /*wrong run*/, tick.ID)
	if err == nil {
		t.Error("expected error when run_id doesn't match, got nil")
	}
}

// ---------------------------------------------------------------------------
// GetClimbingSessionHeader
// ---------------------------------------------------------------------------

func TestGetClimbingSessionHeader_Empty(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "header-empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	h, err := GetClimbingSessionHeader(store.DB, 1, 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.TotalClimbs != 0 || h.TotalSends != 0 || h.HardestGrade != "" {
		t.Errorf("empty header = %+v, want all-zero", h)
	}
}

func TestGetClimbingSessionHeader_TotalClimbsAndSends(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "header-counts.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 5

	ticks := []ClimbingTick{
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Sent: true, Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V5", Sent: true, Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Sent: false, Attempts: 1},
	}
	for i := range ticks {
		if err := CreateClimbingTick(store.DB, &ticks[i]); err != nil {
			t.Fatal(err)
		}
	}

	h, err := GetClimbingSessionHeader(store.DB, ownerID, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.TotalClimbs != 3 {
		t.Errorf("TotalClimbs = %d, want 3", h.TotalClimbs)
	}
	if h.TotalSends != 2 {
		t.Errorf("TotalSends = %d, want 2", h.TotalSends)
	}
}

func TestGetClimbingSessionHeader_HardestByRankNotLex(t *testing.T) {
	// "V10" > "V1" by rank. Lexicographically "V10" < "V1" (10 < 1 as strings).
	// The function must use gradeRanks, not string comparison.
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "header-rank.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 5

	for _, grade := range []string{"V10", "V1", "V3"} {
		if err := CreateClimbingTick(store.DB, &ClimbingTick{
			OwnerID: ownerID, RunID: runID, ExerciseID: 1,
			Kind: "boulder", Grade: grade, Sent: true, Attempts: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}

	h, err := GetClimbingSessionHeader(store.DB, ownerID, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.HardestGrade != "V10" {
		t.Errorf("HardestGrade = %q, want V10 (rank-based not lexicographic)", h.HardestGrade)
	}
}

func TestGetClimbingSessionHeader_UngradedExcludedFromHardest(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "header-ungraded.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 5

	for _, grade := range []string{"V4", "Rainbow", "Traverse"} {
		if err := CreateClimbingTick(store.DB, &ClimbingTick{
			OwnerID: ownerID, RunID: runID, ExerciseID: 1,
			Kind: "boulder", Grade: grade, Attempts: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}

	h, err := GetClimbingSessionHeader(store.DB, ownerID, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.HardestGrade != "V4" {
		t.Errorf("HardestGrade = %q, want V4 (Rainbow/Traverse not ranked)", h.HardestGrade)
	}
	if h.TotalClimbs != 3 {
		t.Errorf("TotalClimbs = %d, want 3 (ungraded climbs counted)", h.TotalClimbs)
	}
}

func TestGetClimbingSessionHeader_AllUngradedHardestEmpty(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "header-allungraded.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 5

	for _, grade := range []string{"Rainbow", "Traverse"} {
		if err := CreateClimbingTick(store.DB, &ClimbingTick{
			OwnerID: ownerID, RunID: runID, ExerciseID: 1,
			Kind: "boulder", Grade: grade, Attempts: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}

	h, err := GetClimbingSessionHeader(store.DB, ownerID, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.HardestGrade != "" {
		t.Errorf("HardestGrade = %q, want empty when only ungraded climbs", h.HardestGrade)
	}
}

func TestGetClimbingSessionHeader_OwnerScoped(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "header-owner.db"))
	if err != nil {
		t.Fatal(err)
	}

	const myOwner uint = 1
	const otherOwner uint = 2
	const runID uint = 5

	// Other owner's very hard tick — must not leak.
	if err := CreateClimbingTick(store.DB, &ClimbingTick{
		OwnerID: otherOwner, RunID: runID, ExerciseID: 1,
		Kind: "boulder", Grade: "V15", Sent: true, Attempts: 1,
	}); err != nil {
		t.Fatal(err)
	}
	// My tick.
	if err := CreateClimbingTick(store.DB, &ClimbingTick{
		OwnerID: myOwner, RunID: runID, ExerciseID: 1,
		Kind: "boulder", Grade: "V2", Sent: true, Attempts: 1,
	}); err != nil {
		t.Fatal(err)
	}

	h, err := GetClimbingSessionHeader(store.DB, myOwner, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.TotalClimbs != 1 {
		t.Errorf("TotalClimbs = %d, want 1 (other owner excluded)", h.TotalClimbs)
	}
	if h.HardestGrade != "V2" {
		t.Errorf("HardestGrade = %q, want V2 (other owner's V15 excluded)", h.HardestGrade)
	}
}
