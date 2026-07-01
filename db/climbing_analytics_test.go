package db

import (
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// ClimbingAnalytics
// ---------------------------------------------------------------------------

// TestClimbingAnalytics_EmptyRunIDsHasNoData guards the early-return when no run IDs
// are supplied — HasData must be false and counts must be zero.
func TestClimbingAnalytics_EmptyRunIDsHasNoData(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "analytics-empty-runs.db"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := ClimbingAnalytics(store.DB, 1, nil)
	if err != nil {
		t.Fatalf("ClimbingAnalytics: %v", err)
	}
	if result.HasData {
		t.Error("HasData = true for empty runIDs, want false")
	}
	if result.TotalClimbs != 0 {
		t.Errorf("TotalClimbs = %d, want 0", result.TotalClimbs)
	}
}

// TestClimbingAnalytics_NoTicksHasNoData guards that supplying runIDs that have zero
// ticks also yields HasData==false (the zero-tick early-return path).
func TestClimbingAnalytics_NoTicksHasNoData(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "analytics-no-ticks.db"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := ClimbingAnalytics(store.DB, 1, []uint{99, 100})
	if err != nil {
		t.Fatalf("ClimbingAnalytics: %v", err)
	}
	if result.HasData {
		t.Error("HasData = true for run IDs with no ticks, want false")
	}
}

// TestClimbingAnalytics_PyramidCountsAndSendRate is the core correctness test.
// It inserts a known set of ticks and asserts pyramid totals, sent counts, and send rate.
func TestClimbingAnalytics_PyramidCountsAndSendRate(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "analytics-pyramid.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 10

	// 3 boulder ticks: V3 sent, V3 not-sent, V5 sent.
	ticks := []ClimbingTick{
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Sent: true, Setting: "indoor", Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Sent: false, Setting: "indoor", Attempts: 2},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V5", Sent: true, Setting: "indoor", Attempts: 1},
	}
	for i := range ticks {
		if err := CreateClimbingTick(store.DB, &ticks[i]); err != nil {
			t.Fatalf("create tick[%d]: %v", i, err)
		}
	}

	result, err := ClimbingAnalytics(store.DB, ownerID, []uint{runID})
	if err != nil {
		t.Fatalf("ClimbingAnalytics: %v", err)
	}

	if !result.HasData {
		t.Fatal("HasData = false, want true")
	}
	if result.TotalClimbs != 3 {
		t.Errorf("TotalClimbs = %d, want 3", result.TotalClimbs)
	}
	if result.TotalSends != 2 {
		t.Errorf("TotalSends = %d, want 2", result.TotalSends)
	}
	// send rate = 2/3 = 66%
	if result.SendRate != 66 {
		t.Errorf("SendRate = %d, want 66", result.SendRate)
	}

	// Boulder discipline
	if result.Boulder.Ticks != 3 {
		t.Errorf("Boulder.Ticks = %d, want 3", result.Boulder.Ticks)
	}
	if result.Boulder.Sends != 2 {
		t.Errorf("Boulder.Sends = %d, want 2", result.Boulder.Sends)
	}
	// 2/3 = 66
	if result.Boulder.SendRate != 66 {
		t.Errorf("Boulder.SendRate = %d, want 66", result.Boulder.SendRate)
	}

	// Pyramid must contain V3 and V5.
	pyramidByGrade := make(map[string]ClimbingGradeTally, len(result.Boulder.Pyramid))
	for _, row := range result.Boulder.Pyramid {
		pyramidByGrade[row.Grade] = row
	}

	v3, ok := pyramidByGrade["V3"]
	if !ok {
		t.Fatal("V3 missing from boulder pyramid")
	}
	if v3.Total != 2 {
		t.Errorf("V3 Total = %d, want 2", v3.Total)
	}
	if v3.Sent != 1 {
		t.Errorf("V3 Sent = %d, want 1", v3.Sent)
	}

	v5, ok := pyramidByGrade["V5"]
	if !ok {
		t.Fatal("V5 missing from boulder pyramid")
	}
	if v5.Total != 1 {
		t.Errorf("V5 Total = %d, want 1", v5.Total)
	}
	if v5.Sent != 1 {
		t.Errorf("V5 Sent = %d, want 1", v5.Sent)
	}

	// Route discipline should be empty.
	if result.Route.Ticks != 0 {
		t.Errorf("Route.Ticks = %d, want 0 (no route ticks inserted)", result.Route.Ticks)
	}
}

// TestClimbingAnalytics_HardestSentIsWithinDiscipline is the CRITICAL regression guard
// for the cross-scale bug. A boulder V16 and a route 8a must be ranked within their
// own scales. V16 is the hardest boulder; 8a is the hardest route. Neither should
// "win" the other discipline.
func TestClimbingAnalytics_HardestSentIsWithinDiscipline(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "analytics-discipline-rank.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 1

	ticks := []ClimbingTick{
		// Boulder: V16 send (highest in V-scale)
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V16", Sent: true, Attempts: 1},
		// Route: 8a send (a hard French grade)
		{OwnerID: ownerID, RunID: runID, ExerciseID: 2, Kind: "route", Grade: "8a", Sent: true, Attempts: 1},
	}
	for i := range ticks {
		if err := CreateClimbingTick(store.DB, &ticks[i]); err != nil {
			t.Fatalf("create tick[%d]: %v", i, err)
		}
	}

	result, err := ClimbingAnalytics(store.DB, ownerID, []uint{runID})
	if err != nil {
		t.Fatalf("ClimbingAnalytics: %v", err)
	}

	// Boulder hardest must be V16, not influenced by the route 8a.
	if result.Boulder.HardestSent != "V16" {
		t.Errorf("Boulder.HardestSent = %q, want V16 (cross-scale bug guard)", result.Boulder.HardestSent)
	}
	// Route hardest must be 8a, not influenced by the boulder V16.
	if result.Route.HardestSent != "8a" {
		t.Errorf("Route.HardestSent = %q, want 8a (cross-scale bug guard)", result.Route.HardestSent)
	}

	// V16 must NOT appear in the route pyramid and 8a must NOT appear in the boulder pyramid.
	for _, row := range result.Route.Pyramid {
		if row.Grade == "V16" {
			t.Error("V16 appeared in Route pyramid — cross-scale contamination")
		}
	}
	for _, row := range result.Boulder.Pyramid {
		if row.Grade == "8a" {
			t.Error("8a appeared in Boulder pyramid — cross-scale contamination")
		}
	}
}

// TestClimbingAnalytics_UngradedExcludedFromPyramidIncludedInVolume guards that ungraded
// ticks (Grade=="", Rainbow, Traverse) are excluded from the pyramid but still counted
// toward TotalClimbs and SendRate.
func TestClimbingAnalytics_UngradedExcludedFromPyramidIncludedInVolume(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "analytics-ungraded.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 5

	ticks := []ClimbingTick{
		// Graded send — appears in pyramid.
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V4", Sent: true, Attempts: 1},
		// Ungraded (empty grade) — counts toward volume, excluded from pyramid.
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "", Sent: false, Attempts: 1},
		// Rainbow — counts toward volume, excluded from pyramid.
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "Rainbow", Sent: false, Attempts: 1},
	}
	for i := range ticks {
		if err := CreateClimbingTick(store.DB, &ticks[i]); err != nil {
			t.Fatalf("create tick[%d]: %v", i, err)
		}
	}

	result, err := ClimbingAnalytics(store.DB, ownerID, []uint{runID})
	if err != nil {
		t.Fatalf("ClimbingAnalytics: %v", err)
	}

	if result.TotalClimbs != 3 {
		t.Errorf("TotalClimbs = %d, want 3 (ungraded ticks count toward volume)", result.TotalClimbs)
	}
	if result.TotalSends != 1 {
		t.Errorf("TotalSends = %d, want 1", result.TotalSends)
	}

	// Pyramid must contain only V4 — ungraded ticks must not appear.
	if len(result.Boulder.Pyramid) != 1 {
		t.Errorf("Boulder.Pyramid length = %d, want 1 (only V4)", len(result.Boulder.Pyramid))
	} else if result.Boulder.Pyramid[0].Grade != "V4" {
		t.Errorf("Boulder.Pyramid[0].Grade = %q, want V4", result.Boulder.Pyramid[0].Grade)
	}

	// HardestSent must be V4 (ungraded never wins).
	if result.Boulder.HardestSent != "V4" {
		t.Errorf("Boulder.HardestSent = %q, want V4", result.Boulder.HardestSent)
	}
}

// TestClimbingAnalytics_SessionCountIsDistinctRuns asserts that SessionCount reflects
// the number of distinct run IDs that have ticks, not the total tick count.
func TestClimbingAnalytics_SessionCountIsDistinctRuns(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "analytics-session-count.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1

	// Three ticks across two runs.
	ticks := []ClimbingTick{
		{OwnerID: ownerID, RunID: 1, ExerciseID: 1, Kind: "boulder", Grade: "V3", Sent: true, Attempts: 1},
		{OwnerID: ownerID, RunID: 1, ExerciseID: 1, Kind: "boulder", Grade: "V4", Sent: true, Attempts: 1},
		{OwnerID: ownerID, RunID: 2, ExerciseID: 1, Kind: "boulder", Grade: "V2", Sent: false, Attempts: 1},
	}
	for i := range ticks {
		if err := CreateClimbingTick(store.DB, &ticks[i]); err != nil {
			t.Fatalf("create tick[%d]: %v", i, err)
		}
	}

	result, err := ClimbingAnalytics(store.DB, ownerID, []uint{1, 2})
	if err != nil {
		t.Fatalf("ClimbingAnalytics: %v", err)
	}

	if result.SessionCount != 2 {
		t.Errorf("SessionCount = %d, want 2 (distinct run IDs)", result.SessionCount)
	}
	if result.TotalClimbs != 3 {
		t.Errorf("TotalClimbs = %d, want 3", result.TotalClimbs)
	}
}

// TestClimbingAnalytics_SettingSplitsPercentages asserts indoor/outdoor percentages
// are computed against TotalClimbs, not just the ticks with a known setting.
func TestClimbingAnalytics_SettingSplitsPercentages(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "analytics-splits.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 1

	// 4 ticks: 3 indoor, 1 outdoor.
	ticks := []ClimbingTick{
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Setting: "indoor", Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Setting: "indoor", Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Setting: "indoor", Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Setting: "outdoor", Attempts: 1},
	}
	for i := range ticks {
		if err := CreateClimbingTick(store.DB, &ticks[i]); err != nil {
			t.Fatalf("create tick[%d]: %v", i, err)
		}
	}

	result, err := ClimbingAnalytics(store.DB, ownerID, []uint{runID})
	if err != nil {
		t.Fatalf("ClimbingAnalytics: %v", err)
	}

	if !result.HasIndoorOutdoor {
		t.Fatal("HasIndoorOutdoor = false, want true")
	}
	if result.IndoorPct != 75 {
		t.Errorf("IndoorPct = %d, want 75", result.IndoorPct)
	}
	if result.OutdoorPct != 25 {
		t.Errorf("OutdoorPct = %d, want 25", result.OutdoorPct)
	}
}

// TestClimbingAnalytics_BoardCommercialSplit asserts the board/commercial split is
// computed correctly and only includes boulder+indoor subtypes.
func TestClimbingAnalytics_BoardCommercialSplit(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "analytics-board-split.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 1

	ticks := []ClimbingTick{
		// 2 commercial boulders (indoor)
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Setting: "indoor", Subtype: "commercial", Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Setting: "indoor", Subtype: "commercial", Attempts: 1},
		// 1 board boulder (indoor)
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V4", Setting: "indoor", Subtype: "board", Attempts: 1},
		// 1 route — should not affect board/commercial split
		{OwnerID: ownerID, RunID: runID, ExerciseID: 2, Kind: "route", Grade: "6a", Setting: "indoor", Subtype: "commercial", Attempts: 1},
	}
	for i := range ticks {
		if err := CreateClimbingTick(store.DB, &ticks[i]); err != nil {
			t.Fatalf("create tick[%d]: %v", i, err)
		}
	}

	result, err := ClimbingAnalytics(store.DB, ownerID, []uint{runID})
	if err != nil {
		t.Fatalf("ClimbingAnalytics: %v", err)
	}

	if !result.HasBoardSplit {
		t.Fatal("HasBoardSplit = false, want true")
	}
	// commercial=2, board=1 out of total=4 → 50% and 25%
	if result.CommercialPct != 50 {
		t.Errorf("CommercialPct = %d, want 50", result.CommercialPct)
	}
	if result.BoardPct != 25 {
		t.Errorf("BoardPct = %d, want 25", result.BoardPct)
	}
}

// TestClimbingAnalytics_PyramidHardestFirst asserts the pyramid is sorted hardest-first
// within a discipline, using rank ordering (not lexicographic).
// V10 > V1 by rank but "V10" < "V1" lexicographically — a regression guard for lex sort.
func TestClimbingAnalytics_PyramidHardestFirst(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "analytics-pyramid-order.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 1

	ticks := []ClimbingTick{
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V1", Sent: true, Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V10", Sent: true, Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V5", Sent: false, Attempts: 1},
	}
	for i := range ticks {
		if err := CreateClimbingTick(store.DB, &ticks[i]); err != nil {
			t.Fatalf("create tick[%d]: %v", i, err)
		}
	}

	result, err := ClimbingAnalytics(store.DB, ownerID, []uint{runID})
	if err != nil {
		t.Fatalf("ClimbingAnalytics: %v", err)
	}

	if len(result.Boulder.Pyramid) < 2 {
		t.Fatalf("Boulder.Pyramid length = %d, want >= 2", len(result.Boulder.Pyramid))
	}
	// First entry must be the hardest grade (V10), not V1 (would be first lexicographically).
	if result.Boulder.Pyramid[0].Grade != "V10" {
		t.Errorf("Boulder.Pyramid[0].Grade = %q, want V10 (hardest-first by rank, not lex)", result.Boulder.Pyramid[0].Grade)
	}
}

// TestClimbingAnalytics_OwnerScoped asserts that ticks from a different owner in the
// same run IDs are excluded.
func TestClimbingAnalytics_OwnerScoped(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "analytics-owner.db"))
	if err != nil {
		t.Fatal(err)
	}

	const myOwner uint = 1
	const otherOwner uint = 2
	const runID uint = 10

	// Other owner has a hard V16 send.
	if err := CreateClimbingTick(store.DB, &ClimbingTick{
		OwnerID: otherOwner, RunID: runID, ExerciseID: 1,
		Kind: "boulder", Grade: "V16", Sent: true, Attempts: 1,
	}); err != nil {
		t.Fatal(err)
	}
	// My tick is easier.
	if err := CreateClimbingTick(store.DB, &ClimbingTick{
		OwnerID: myOwner, RunID: runID, ExerciseID: 1,
		Kind: "boulder", Grade: "V2", Sent: true, Attempts: 1,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := ClimbingAnalytics(store.DB, myOwner, []uint{runID})
	if err != nil {
		t.Fatalf("ClimbingAnalytics: %v", err)
	}

	if result.TotalClimbs != 1 {
		t.Errorf("TotalClimbs = %d, want 1 (other owner excluded)", result.TotalClimbs)
	}
	if result.Boulder.HardestSent != "V2" {
		t.Errorf("Boulder.HardestSent = %q, want V2 (other owner's V16 excluded)", result.Boulder.HardestSent)
	}
}

// TestClimbingAnalytics_MaxTotalForBarScaling asserts MaxTotal equals the busiest
// grade's total count, used for proportional bar width scaling in the UI.
func TestClimbingAnalytics_MaxTotalForBarScaling(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "analytics-maxtotal.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 1

	// V3: 3 ticks, V4: 1 tick → MaxTotal should be 3.
	ticks := []ClimbingTick{
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Sent: true, Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Sent: false, Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Sent: true, Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V4", Sent: true, Attempts: 1},
	}
	for i := range ticks {
		if err := CreateClimbingTick(store.DB, &ticks[i]); err != nil {
			t.Fatalf("create tick[%d]: %v", i, err)
		}
	}

	result, err := ClimbingAnalytics(store.DB, ownerID, []uint{runID})
	if err != nil {
		t.Fatalf("ClimbingAnalytics: %v", err)
	}

	if result.Boulder.MaxTotal != 3 {
		t.Errorf("Boulder.MaxTotal = %d, want 3 (V3 is busiest)", result.Boulder.MaxTotal)
	}
}

// ---------------------------------------------------------------------------
// UserHasClimbingTicks
// ---------------------------------------------------------------------------

// TestUserHasClimbingTicks_NoTicks asserts false is returned for a user with no ticks.
func TestUserHasClimbingTicks_NoTicks(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "has-ticks-none.db"))
	if err != nil {
		t.Fatal(err)
	}
	has, err := UserHasClimbingTicks(store.DB, 99)
	if err != nil {
		t.Fatalf("UserHasClimbingTicks: %v", err)
	}
	if has {
		t.Error("UserHasClimbingTicks = true for user with no ticks, want false")
	}
}

// TestUserHasClimbingTicks_HasTicks asserts true is returned after inserting a tick.
func TestUserHasClimbingTicks_HasTicks(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "has-ticks-one.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	if err := CreateClimbingTick(store.DB, &ClimbingTick{
		OwnerID: ownerID, RunID: 1, ExerciseID: 1,
		Kind: "boulder", Grade: "V3", Attempts: 1,
	}); err != nil {
		t.Fatal(err)
	}
	has, err := UserHasClimbingTicks(store.DB, ownerID)
	if err != nil {
		t.Fatalf("UserHasClimbingTicks: %v", err)
	}
	if !has {
		t.Error("UserHasClimbingTicks = false after inserting a tick, want true")
	}
}

// TestUserHasClimbingTicks_OwnerScoped asserts another owner's ticks don't affect the result.
func TestUserHasClimbingTicks_OwnerScoped(t *testing.T) {
	t.Parallel()
	store, err := NewSqlite(filepath.Join(t.TempDir(), "has-ticks-owner.db"))
	if err != nil {
		t.Fatal(err)
	}
	const otherOwner uint = 42
	const myOwner uint = 7
	if err := CreateClimbingTick(store.DB, &ClimbingTick{
		OwnerID: otherOwner, RunID: 1, ExerciseID: 1,
		Kind: "boulder", Grade: "V9", Attempts: 1,
	}); err != nil {
		t.Fatal(err)
	}
	has, err := UserHasClimbingTicks(store.DB, myOwner)
	if err != nil {
		t.Fatalf("UserHasClimbingTicks: %v", err)
	}
	if has {
		t.Error("UserHasClimbingTicks = true for user with no ticks (other owner has ticks), want false")
	}
}
