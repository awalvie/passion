package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
)

func TestListGrades(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")

	rec := send(t, h, http.MethodGet, "/api/v1/grades", ada, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var got gradeListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	var systems []string
	for _, s := range got.Scales {
		systems = append(systems, s.System)
	}
	if !slices.Equal(systems, []string{"font", "v", "french", "yds"}) || !got.Scales[0].Boulder || got.Scales[2].Boulder {
		t.Fatalf("scales %v, want the two boulder scales, then the two route scales", systems)
	}
	if len(got.Ungraded) == 0 {
		t.Fatal("no ungraded labels")
	}

	if rec := send(t, h, http.MethodGet, "/api/v1/grades", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d signed out, want 401", rec.Code)
	}
}
