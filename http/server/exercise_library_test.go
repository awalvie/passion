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

// TestExerciseLibraryKindFilter_Climbing guards a real regression: the kind-filter
// validation used a hand-maintained map that omitted "climbing", so
// /exercise-library?kind=climbing silently dropped the filter and showed everything.
// Validation now goes through db.NormalizeKind, so climbing must be honored.
func TestExerciseLibraryKindFilter_Climbing(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "lib-kind.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	if err := store.DB.Create(&db.User{Email: "c@example.com", PasswordHash: "h"}).Error; err != nil {
		t.Fatal(err)
	}
	// One climbing exercise and one reps exercise in the library.
	if err := store.DB.Create(&db.LibraryExercise{OwnerID: ownerID, Name: "Board Session", Kind: "climbing"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&db.LibraryExercise{OwnerID: ownerID, Name: "Weighted Pullups", Kind: "reps_and_sets"}).Error; err != nil {
		t.Fatal(err)
	}

	srv, err := NewServer(store, "secret", 24, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	get := func(query string) string {
		req := httptest.NewRequest(http.MethodGet, "/exercise-library"+query, nil)
		req = req.WithContext(context.WithValue(req.Context(), authUserIDKey, ownerID))
		rr := httptest.NewRecorder()
		srv.handleExerciseLibraryIndex(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d for %q, want 200", rr.Code, query)
		}
		return rr.Body.String()
	}

	// Filtering by climbing must show the climbing exercise and exclude the reps one.
	climbing := get("?kind=climbing")
	if !strings.Contains(climbing, "Board Session") {
		t.Errorf("?kind=climbing did not show the climbing exercise")
	}
	if strings.Contains(climbing, "Weighted Pullups") {
		t.Errorf("?kind=climbing leaked a non-climbing exercise (filter was dropped)")
	}

	// An invalid kind is ignored (filter cleared), so both show.
	all := get("?kind=bogus")
	if !strings.Contains(all, "Board Session") || !strings.Contains(all, "Weighted Pullups") {
		t.Errorf("invalid kind should clear the filter and show all exercises")
	}
}
