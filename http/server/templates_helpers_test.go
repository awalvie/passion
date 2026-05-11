package web

import (
	"net/http/httptest"
	"strings"
	"testing"

	"passion/db"
)

func TestNormalizeExerciseKind(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"session", "session"},
		{"  Session  ", "session"},
		{"exercise_catalog", "exercise_catalog"},
		{"unknown", "reps_and_sets"},
		{"", "reps_and_sets"},
	}
	for _, tc := range cases {
		got := db.NormalizeKind(tc.in)
		if got != tc.want {
			t.Errorf("db.NormalizeKind(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseSessionDurationSeconds(t *testing.T) {
	body := strings.NewReader("session_duration_hours=1&session_duration_minutes=2&session_duration_seconds=3")
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := r.ParseForm(); err != nil {
		t.Fatal(err)
	}
	if got := parseSessionDurationSeconds(r); got != 3723 {
		t.Fatalf("parseSessionDurationSeconds = %d, want 3723", got)
	}
}

func TestParseSessionDurationSecondsNegativeAndInvalid(t *testing.T) {
	body := strings.NewReader("session_duration_hours=-1&session_duration_minutes=oops&session_duration_seconds=5")
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := r.ParseForm(); err != nil {
		t.Fatal(err)
	}
	if got := parseSessionDurationSeconds(r); got != 5 {
		t.Fatalf("parseSessionDurationSeconds invalid handling = %d, want 5", got)
	}
}

func TestNewExerciseFromLibraryExercise(t *testing.T) {
	parentID := uint(10)
	lib := db.LibraryExercise{
		Name:                   "Pull up",
		Notes:                  "strict",
		Kind:                   "reps_and_sets",
		SessionDurationSeconds: 30,
		Sets:                   5,
		Reps:                   3,
		RepSeconds:             4,
		RepRestSeconds:         7,
		SetRestSeconds:         120,
		WeightKg:               15,
	}

	got := newExerciseFromLibraryExercise(lib, 3, 4, 2, &parentID)
	if got.OwnerID != 3 || got.ActivityID == nil || *got.ActivityID != 4 || got.OrderIndex != 2 {
		t.Fatalf("id/order fields not copied: %+v", *got)
	}
	if got.Kind != lib.Kind {
		t.Fatalf("kind not copied: got %q, want %q", got.Kind, lib.Kind)
	}
	if got.ParentExerciseID == nil || *got.ParentExerciseID != parentID {
		t.Fatalf("parent id not copied: %+v", got.ParentExerciseID)
	}
	if got.Name != lib.Name || got.Notes != lib.Notes || got.Sets != lib.Sets || got.WeightKg != lib.WeightKg {
		t.Fatalf("library fields not copied: %+v", *got)
	}
}
