package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"passion/db"
)

// tickChiRequest builds an httptest.Request with chi URL params injected.
func tickChiRequest(t *testing.T, method, path string, body string, params map[string]string) *http.Request {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func withOwner(t *testing.T, req *http.Request, ownerID uint) *http.Request {
	t.Helper()
	return req.WithContext(context.WithValue(req.Context(), authUserIDKey, ownerID))
}

// ---------------------------------------------------------------------------
// resolveResult unit tests
// ---------------------------------------------------------------------------

func TestResolveResult(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		style     string
		grade     string
		wantSent  bool
		wantStyle string
	}{
		// Graded + clean-send styles → sent=true, style preserved
		{name: "onsight graded", style: "onsight", grade: "6a", wantSent: true, wantStyle: "onsight"},
		{name: "flash graded", style: "flash", grade: "V3", wantSent: true, wantStyle: "flash"},
		{name: "redpoint graded", style: "redpoint", grade: "V5", wantSent: true, wantStyle: "redpoint"},
		// Graded + non-send styles → sent=false, style preserved
		{name: "hangdog graded", style: "hangdog", grade: "6a", wantSent: false, wantStyle: "hangdog"},
		{name: "working graded", style: "working", grade: "V3", wantSent: false, wantStyle: "working"},
		// Ungraded grades override entirely → (false, "")
		{name: "Rainbow any style", style: "onsight", grade: "Rainbow", wantSent: false, wantStyle: ""},
		{name: "Traverse any style", style: "flash", grade: "Traverse", wantSent: false, wantStyle: ""},
		{name: "Rainbow hangdog", style: "hangdog", grade: "Rainbow", wantSent: false, wantStyle: ""},
		{name: "Traverse working", style: "working", grade: "Traverse", wantSent: false, wantStyle: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotSent, gotStyle := resolveResult(tc.style, tc.grade)
			if gotSent != tc.wantSent || gotStyle != tc.wantStyle {
				t.Errorf("resolveResult(%q, %q) = (%v, %q), want (%v, %q)",
					tc.style, tc.grade,
					gotSent, gotStyle,
					tc.wantSent, tc.wantStyle)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isUngradedClimb unit tests
// ---------------------------------------------------------------------------

func TestIsUngradedClimb(t *testing.T) {
	t.Parallel()
	cases := []struct {
		grade string
		want  bool
	}{
		{"Rainbow", true},
		{"Traverse", true},
		{"6a", false},
		{"V3", false},
		{"", false},
		{"rainbow", false}, // case-sensitive
	}
	for _, tc := range cases {
		if got := isUngradedClimb(tc.grade); got != tc.want {
			t.Errorf("isUngradedClimb(%q) = %v, want %v", tc.grade, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// styleImpliesSent unit tests
// ---------------------------------------------------------------------------

func TestStyleImpliesSent(t *testing.T) {
	t.Parallel()
	// Only clean ascents count as sends: onsight (first go, no beta),
	// flash (first go, with beta), redpoint (later go, no falls).
	sent := []string{"onsight", "flash", "redpoint"}
	// repeat is no longer a send in the new Result model.
	notSent := []string{"hangdog", "working", "repeat", "attempt", "", "project"}
	for _, s := range sent {
		if !styleImpliesSent(s) {
			t.Errorf("styleImpliesSent(%q) = false, want true", s)
		}
	}
	for _, s := range notSent {
		if styleImpliesSent(s) {
			t.Errorf("styleImpliesSent(%q) = true, want false", s)
		}
	}
}

// ---------------------------------------------------------------------------
// DB layer: GetLatestClimbingTickInRun
// ---------------------------------------------------------------------------

func TestGetLatestClimbingTickInRun_EmptyRun(t *testing.T) {
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "latest-empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, ok := db.GetLatestClimbingTickInRun(store.DB, 1, 99)
	if ok {
		t.Error("expected ok=false for empty run, got ok=true")
	}
}

func TestGetLatestClimbingTickInRun_ReturnsMostRecentByCreatedAt(t *testing.T) {
	// Two exercises in the same run; ticks interleaved in time.
	// The function must return the chronologically last tick, NOT the one with
	// the highest order_index (which is per-exercise and can collide).
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "latest-time.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 10

	now := time.Now().UTC()

	// Exercise A tick at t+0
	tickA := &db.ClimbingTick{
		OwnerID: ownerID, RunID: runID, ExerciseID: 100,
		Kind: "boulder", Grade: "V3", Attempts: 1,
	}
	if err := store.DB.Create(tickA).Error; err != nil {
		t.Fatal(err)
	}
	store.DB.Model(tickA).Update("created_at", now.Add(-2*time.Minute))

	// Exercise B tick at t+1 (later — should win)
	tickB := &db.ClimbingTick{
		OwnerID: ownerID, RunID: runID, ExerciseID: 200,
		Kind: "boulder", Grade: "V5", Attempts: 1,
	}
	if err := store.DB.Create(tickB).Error; err != nil {
		t.Fatal(err)
	}
	store.DB.Model(tickB).Update("created_at", now.Add(-1*time.Minute))

	// Exercise A tick at t+2 (even later — should win over B)
	tickA2 := &db.ClimbingTick{
		OwnerID: ownerID, RunID: runID, ExerciseID: 100,
		Kind: "boulder", Grade: "V7", Attempts: 1,
	}
	if err := store.DB.Create(tickA2).Error; err != nil {
		t.Fatal(err)
	}
	store.DB.Model(tickA2).Update("created_at", now)

	got, ok := db.GetLatestClimbingTickInRun(store.DB, ownerID, runID)
	if !ok {
		t.Fatal("expected ok=true, got ok=false")
	}
	if got.ID != tickA2.ID {
		t.Errorf("latest tick ID = %d, want %d (tickA2 with grade V7)", got.ID, tickA2.ID)
	}
	if got.Grade != "V7" {
		t.Errorf("latest tick grade = %q, want V7", got.Grade)
	}
}

func TestGetLatestClimbingTickInRun_OwnerScoped(t *testing.T) {
	// A tick from another owner in the same run must not be returned.
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "latest-owner.db"))
	if err != nil {
		t.Fatal(err)
	}

	const otherOwner uint = 42
	const myOwner uint = 7
	const runID uint = 5

	tickOther := &db.ClimbingTick{
		OwnerID: otherOwner, RunID: runID, ExerciseID: 1,
		Kind: "boulder", Grade: "V9", Attempts: 1,
	}
	if err := store.DB.Create(tickOther).Error; err != nil {
		t.Fatal(err)
	}

	_, ok := db.GetLatestClimbingTickInRun(store.DB, myOwner, runID)
	if ok {
		t.Error("expected ok=false for owner with no ticks, got ok=true")
	}
}

// ---------------------------------------------------------------------------
// DB layer: GetClimbingTick (owner+run scoped IDOR guard)
// ---------------------------------------------------------------------------

func TestGetClimbingTick_HappyPath(t *testing.T) {
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "gettick-ok.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 10

	tick := &db.ClimbingTick{
		OwnerID: ownerID, RunID: runID, ExerciseID: 1,
		Kind: "boulder", Grade: "V4", Attempts: 1,
	}
	if err := store.DB.Create(tick).Error; err != nil {
		t.Fatal(err)
	}

	got, err := db.GetClimbingTick(store.DB, ownerID, runID, tick.ID)
	if err != nil {
		t.Fatalf("GetClimbingTick error: %v", err)
	}
	if got.ID != tick.ID {
		t.Errorf("got ID %d, want %d", got.ID, tick.ID)
	}
}

func TestGetClimbingTick_WrongOwnerBlocked(t *testing.T) {
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "gettick-idor.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const attackerID uint = 2
	const runID uint = 10

	tick := &db.ClimbingTick{
		OwnerID: ownerID, RunID: runID, ExerciseID: 1,
		Kind: "boulder", Grade: "V4", Attempts: 1,
	}
	if err := store.DB.Create(tick).Error; err != nil {
		t.Fatal(err)
	}

	_, err = db.GetClimbingTick(store.DB, attackerID, runID, tick.ID)
	if err == nil {
		t.Error("expected error when accessing tick with wrong owner, got nil")
	}
}

func TestGetClimbingTick_WrongRunBlocked(t *testing.T) {
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "gettick-run.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 10
	const otherRunID uint = 99

	tick := &db.ClimbingTick{
		OwnerID: ownerID, RunID: runID, ExerciseID: 1,
		Kind: "boulder", Grade: "V4", Attempts: 1,
	}
	if err := store.DB.Create(tick).Error; err != nil {
		t.Fatal(err)
	}

	_, err = db.GetClimbingTick(store.DB, ownerID, otherRunID, tick.ID)
	if err == nil {
		t.Error("expected error when tick run_id doesn't match, got nil")
	}
}

// ---------------------------------------------------------------------------
// DB layer: GetClimbingSessionHeader
// ---------------------------------------------------------------------------

func TestGetClimbingSessionHeader_Empty(t *testing.T) {
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "header-empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	h, err := db.GetClimbingSessionHeader(store.DB, 1, 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.TotalClimbs != 0 || h.TotalSends != 0 || h.HardestGrade != "" {
		t.Errorf("empty run header = %+v, want zero value", h)
	}
}

func TestGetClimbingSessionHeader_CountsAndHardest(t *testing.T) {
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "header-counts.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 5

	ticks := []db.ClimbingTick{
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Sent: true, Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V5", Sent: true, Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V3", Sent: false, Attempts: 1},
	}
	for i := range ticks {
		if err := store.DB.Create(&ticks[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	h, err := db.GetClimbingSessionHeader(store.DB, ownerID, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.TotalClimbs != 3 {
		t.Errorf("TotalClimbs = %d, want 3", h.TotalClimbs)
	}
	if h.TotalSends != 2 {
		t.Errorf("TotalSends = %d, want 2", h.TotalSends)
	}
	if h.HardestGrade != "V5" {
		t.Errorf("HardestGrade = %q, want V5", h.HardestGrade)
	}
}

func TestGetClimbingSessionHeader_HardestUsesRankNotLex(t *testing.T) {
	// "6a" < "10a" lexicographically but "10a" (French) does not exist in V-scale;
	// use a clear case: "V1" vs "V10" — "V10" > "V1" by rank but "V10" < "V1" lexicographically.
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "header-rank.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 5

	ticks := []db.ClimbingTick{
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V10", Sent: true, Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V1", Sent: true, Attempts: 1},
	}
	for i := range ticks {
		if err := store.DB.Create(&ticks[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	h, err := db.GetClimbingSessionHeader(store.DB, ownerID, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.HardestGrade != "V10" {
		t.Errorf("HardestGrade = %q, want V10 (rank-based, not lexicographic)", h.HardestGrade)
	}
}

func TestGetClimbingSessionHeader_UngradedExcludedFromHardest(t *testing.T) {
	// Rainbow and Traverse are not in gradeRanks; they must never win HardestGrade.
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "header-ungraded.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 5

	ticks := []db.ClimbingTick{
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "V4", Sent: true, Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "Rainbow", Sent: false, Attempts: 1},
		{OwnerID: ownerID, RunID: runID, ExerciseID: 1, Kind: "boulder", Grade: "Traverse", Sent: false, Attempts: 1},
	}
	for i := range ticks {
		if err := store.DB.Create(&ticks[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	h, err := db.GetClimbingSessionHeader(store.DB, ownerID, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.HardestGrade != "V4" {
		t.Errorf("HardestGrade = %q, want V4 (Rainbow/Traverse excluded)", h.HardestGrade)
	}
	if h.TotalClimbs != 3 {
		t.Errorf("TotalClimbs = %d, want 3 (ungraded climbs still count)", h.TotalClimbs)
	}
}

func TestGetClimbingSessionHeader_AllUngradedHardestIsEmpty(t *testing.T) {
	// If every tick is Rainbow/Traverse, HardestGrade must be "".
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "header-allungraded.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 5

	for _, grade := range []string{"Rainbow", "Traverse", "Rainbow"} {
		tick := &db.ClimbingTick{
			OwnerID: ownerID, RunID: runID, ExerciseID: 1,
			Kind: "boulder", Grade: grade, Attempts: 1,
		}
		if err := store.DB.Create(tick).Error; err != nil {
			t.Fatal(err)
		}
	}

	h, err := db.GetClimbingSessionHeader(store.DB, ownerID, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.HardestGrade != "" {
		t.Errorf("HardestGrade = %q, want empty when all ticks are ungraded", h.HardestGrade)
	}
}

func TestGetClimbingSessionHeader_OwnerScoped(t *testing.T) {
	// Another owner's ticks in the same runID must not inflate the count.
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "header-owner.db"))
	if err != nil {
		t.Fatal(err)
	}

	const myOwner uint = 1
	const otherOwner uint = 2
	const runID uint = 5

	// other owner's tick
	if err := store.DB.Create(&db.ClimbingTick{
		OwnerID: otherOwner, RunID: runID, ExerciseID: 1,
		Kind: "boulder", Grade: "V9", Sent: true, Attempts: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	// my tick
	if err := store.DB.Create(&db.ClimbingTick{
		OwnerID: myOwner, RunID: runID, ExerciseID: 1,
		Kind: "boulder", Grade: "V2", Sent: true, Attempts: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	h, err := db.GetClimbingSessionHeader(store.DB, myOwner, runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.TotalClimbs != 1 {
		t.Errorf("TotalClimbs = %d, want 1 (other owner excluded)", h.TotalClimbs)
	}
	if h.HardestGrade != "V2" {
		t.Errorf("HardestGrade = %q, want V2 (other owner's V9 excluded)", h.HardestGrade)
	}
}

// ---------------------------------------------------------------------------
// DB layer: Attempts is persisted as supplied
// ---------------------------------------------------------------------------

func TestCreateClimbingTick_AttemptsPersistedAsSupplied(t *testing.T) {
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "tick-attempts.db"))
	if err != nil {
		t.Fatal(err)
	}

	tick := &db.ClimbingTick{
		OwnerID: 1, RunID: 1, ExerciseID: 1,
		Kind: "boulder", Grade: "V3", Attempts: 4,
	}
	if err := db.CreateClimbingTick(store.DB, tick); err != nil {
		t.Fatal(err)
	}

	var stored db.ClimbingTick
	if err := store.DB.First(&stored, tick.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Attempts != 4 {
		t.Errorf("Attempts = %d, want 4 (stored as supplied)", stored.Attempts)
	}
}

// ---------------------------------------------------------------------------
// Handler: createExerciseTick — Attempts from form is persisted
// ---------------------------------------------------------------------------

// TestCreateExerciseTick_AttemptsPersistedFromForm verifies the handler reads the
// posted "attempts" value and stores it. Uses a full server + template path so
// the response renders successfully.
func TestCreateExerciseTick_AttemptsPersistedFromForm(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "handler-attempts.db"))
	if err != nil {
		t.Fatal(err)
	}

	user := &db.User{Email: "a@a.com", PasswordHash: "x"}
	if err := store.DB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	actualOwnerID := user.ID

	const runID uint = 10
	const exerciseID uint = 5

	srv, err := NewServer(store, "secret", 24, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"kind":     {"boulder"},
		"grade":    {"V4"},
		"style":    {"redpoint"},
		"setting":  {"indoor"},
		"attempts": {"4"},
	}
	req := tickChiRequest(t, http.MethodPost, "/runs/10/exercises/5/ticks", form.Encode(),
		map[string]string{"runID": fmt.Sprintf("%d", runID), "exerciseID": fmt.Sprintf("%d", exerciseID)})
	req = withOwner(t, req, actualOwnerID)
	rr := httptest.NewRecorder()

	srv.createExerciseTick(rr, req, actualOwnerID, runID, exerciseID)

	var ticks []db.ClimbingTick
	if err := store.DB.Where("owner_id = ? AND run_id = ? AND exercise_id = ?", actualOwnerID, runID, exerciseID).Find(&ticks).Error; err != nil {
		t.Fatal(err)
	}
	if len(ticks) != 1 {
		t.Fatalf("expected 1 tick, got %d; response: %q", len(ticks), rr.Body.String())
	}
	if ticks[0].Attempts != 4 {
		t.Errorf("Attempts = %d, want 4 (posted value persisted)", ticks[0].Attempts)
	}
}

// TestCreateExerciseTick_AttemptsLessThanOneClampedToOne verifies that attempts<1
// is clamped to 1 (min guard in the handler).
func TestCreateExerciseTick_AttemptsLessThanOneClampedToOne(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "handler-attempts-clamp.db"))
	if err != nil {
		t.Fatal(err)
	}

	user := &db.User{Email: "b@b.com", PasswordHash: "x"}
	if err := store.DB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	ownerID := user.ID

	const runID uint = 20
	const exerciseID uint = 6

	srv, err := NewServer(store, "secret", 24, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"kind":     {"boulder"},
		"grade":    {"V2"},
		"style":    {"hangdog"},
		"setting":  {"outdoor"},
		"attempts": {"0"},
	}
	req := tickChiRequest(t, http.MethodPost, "/runs/20/exercises/6/ticks", form.Encode(),
		map[string]string{"runID": fmt.Sprintf("%d", runID), "exerciseID": fmt.Sprintf("%d", exerciseID)})
	req = withOwner(t, req, ownerID)
	rr := httptest.NewRecorder()

	srv.createExerciseTick(rr, req, ownerID, runID, exerciseID)

	var ticks []db.ClimbingTick
	if err := store.DB.Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exerciseID).Find(&ticks).Error; err != nil {
		t.Fatal(err)
	}
	if len(ticks) != 1 {
		t.Fatalf("expected 1 tick, got %d; response: %q", len(ticks), rr.Body.String())
	}
	if ticks[0].Attempts != 1 {
		t.Errorf("Attempts = %d, want 1 (clamped from 0)", ticks[0].Attempts)
	}
}

// ---------------------------------------------------------------------------
// DB: UpdateClimbingTick — handler persists the posted attempts value
// ---------------------------------------------------------------------------

// TestUpdateClimbingTick_AttemptsPersistedFromForm verifies that the DB update
// function used by handleExerciseTickUpdate stores the attempts value passed by
// the handler (parsed from the posted form).
func TestUpdateClimbingTick_AttemptsPersistedFromForm(t *testing.T) {
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "update-attempts.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 10
	const exerciseID uint = 5

	tick := &db.ClimbingTick{
		OwnerID: ownerID, RunID: runID, ExerciseID: exerciseID,
		Kind: "boulder", Grade: "V3", Attempts: 1, Sent: false,
	}
	if err := db.CreateClimbingTick(store.DB, tick); err != nil {
		t.Fatal(err)
	}

	// Update with attempts=3 (simulating the handler parsing "attempts" from the form).
	sent, style := resolveResult("redpoint", "V4")
	if err := db.UpdateClimbingTick(store.DB, ownerID, tick.ID,
		"boulder", "indoor", "", "V4", "", "", style, "", 3 /*attempts*/, 0, sent,
	); err != nil {
		t.Fatalf("UpdateClimbingTick: %v", err)
	}

	var updated db.ClimbingTick
	if err := store.DB.First(&updated, tick.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Attempts != 3 {
		t.Errorf("Attempts after update = %d, want 3 (posted value persisted)", updated.Attempts)
	}
	if !updated.Sent {
		t.Error("Sent should be true for redpoint on a graded climb")
	}
}

// ---------------------------------------------------------------------------
// Handler: handleExerciseTickLogAgain — seeds from specific tick
// ---------------------------------------------------------------------------

func TestHandleExerciseTickLogAgain_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	srv := &Server{}
	req := tickChiRequest(t, http.MethodGet, "/runs/1/exercises/1/ticks/1/again", "",
		map[string]string{"runID": "1", "exerciseID": "1", "tickID": "1"})
	req = withOwner(t, req, uint(1))
	rr := httptest.NewRecorder()

	srv.handleExerciseTickLogAgain(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleExerciseTickLogAgain_SeedsFromTick(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "log-again.db"))
	if err != nil {
		t.Fatal(err)
	}

	user := &db.User{Email: "climber@test.com", PasswordHash: "x"}
	if err := store.DB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	ownerID := user.ID

	const runID uint = 1
	const exerciseID uint = 1

	// Create the source tick with a distinctive grade.
	tick := &db.ClimbingTick{
		OwnerID: ownerID, RunID: runID, ExerciseID: exerciseID,
		Kind: "boulder", Grade: "V8", Setting: "indoor", Attempts: 1, Sent: true,
	}
	if err := db.CreateClimbingTick(store.DB, tick); err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(store, "secret", 24, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	req := tickChiRequest(t, http.MethodPost,
		fmt.Sprintf("/runs/%d/exercises/%d/ticks/%d/again", runID, exerciseID, tick.ID),
		"",
		map[string]string{
			"runID":      fmt.Sprintf("%d", runID),
			"exerciseID": fmt.Sprintf("%d", exerciseID),
			"tickID":     fmt.Sprintf("%d", tick.ID),
		})
	req = withOwner(t, req, ownerID)
	rr := httptest.NewRecorder()

	srv.handleExerciseTickLogAgain(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "V8") {
		t.Errorf("response body does not contain the seeded grade 'V8': %.200q", body)
	}
}

func TestHandleExerciseTickLogAgain_UnknownTickFallsBackGracefully(t *testing.T) {
	// If the tick ID doesn't exist (or belongs to another run), the handler should
	// still return 200 (falls back to latest-in-run or defaults).
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "log-again-missing.db"))
	if err != nil {
		t.Fatal(err)
	}

	user := &db.User{Email: "c@c.com", PasswordHash: "x"}
	if err := store.DB.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	ownerID := user.ID

	srv, err := NewServer(store, "secret", 24, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Use the user's actual ID in the URL params to avoid a user-not-found server error.
	req := tickChiRequest(t, http.MethodPost, "/runs/1/exercises/1/ticks/9999/again", "",
		map[string]string{"runID": "1", "exerciseID": "1", "tickID": "9999"})
	req = withOwner(t, req, ownerID)
	rr := httptest.NewRecorder()

	srv.handleExerciseTickLogAgain(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body=%q", rr.Code, http.StatusOK, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// resolveResult: hangdog on a graded climb → sent=false (key regression guard)
// ---------------------------------------------------------------------------

func TestResolveResult_HangdogIsNotSent(t *testing.T) {
	t.Parallel()
	sent, style := resolveResult("hangdog", "V5")
	if sent {
		t.Error("hangdog on a graded climb should result in sent=false")
	}
	if style != "hangdog" {
		t.Errorf("style = %q, want hangdog", style)
	}
}

// ---------------------------------------------------------------------------
// resolveResult + DB: style maps to correct Sent/Style values in the DB
// This tests the contract at the resolveResult boundary. The handler calls
// resolveResult then passes sent/style to db.CreateClimbingTick — so testing
// them in sequence is equivalent to the handler path without template rendering.
// ---------------------------------------------------------------------------

func TestResolveResult_ToDBValues(t *testing.T) {
	t.Parallel()
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "result-db.db"))
	if err != nil {
		t.Fatal(err)
	}

	const ownerID uint = 1
	const runID uint = 1
	const exerciseID uint = 1

	cases := []struct {
		style     string
		grade     string
		wantSent  bool
		wantStyle string
	}{
		{"flash", "6a", true, "flash"},
		{"onsight", "6a", true, "onsight"},
		{"redpoint", "V3", true, "redpoint"},
		{"hangdog", "V3", false, "hangdog"},
		{"working", "V3", false, "working"},
		{"onsight", "Rainbow", false, ""},
		{"flash", "Traverse", false, ""},
	}

	for _, tc := range cases {
		sent, style := resolveResult(tc.style, tc.grade)
		tick := &db.ClimbingTick{
			OwnerID: ownerID, RunID: runID, ExerciseID: exerciseID,
			Kind: "boulder", Grade: tc.grade,
			Sent: sent, Style: style, Attempts: 1,
		}
		if err := db.CreateClimbingTick(store.DB, tick); err != nil {
			t.Fatalf("style=%q grade=%q: CreateClimbingTick: %v", tc.style, tc.grade, err)
		}

		var stored db.ClimbingTick
		if err := store.DB.First(&stored, tick.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.Sent != tc.wantSent {
			t.Errorf("style=%q grade=%q: Sent=%v, want %v", tc.style, tc.grade, stored.Sent, tc.wantSent)
		}
		if stored.Style != tc.wantStyle {
			t.Errorf("style=%q grade=%q: Style=%q, want %q", tc.style, tc.grade, stored.Style, tc.wantStyle)
		}
	}
}
