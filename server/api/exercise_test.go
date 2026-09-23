package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// send makes a request with a body and a bearer token.
func send(t *testing.T, h http.Handler, method, path, bearer, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func decodeExercise(t *testing.T, rec *httptest.ResponseRecorder) exerciseResponse {
	t.Helper()
	var got exerciseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response: %v (%s)", err, rec.Body)
	}
	return got
}

func createExercise(t *testing.T, h http.Handler, bearer, body string) exerciseResponse {
	t.Helper()
	rec := send(t, h, http.MethodPost, "/api/v1/exercises", bearer, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d: %s", rec.Code, rec.Body)
	}
	return decodeExercise(t, rec)
}

// insertShipped writes a shipped row directly, because only the catalog loader
// will write them and it does not exist yet.
func insertShipped(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO exercise (name, kind) VALUES ($1, 'open') RETURNING id`, name).Scan(&id)
	if err != nil {
		t.Fatalf("insert %s: %v", name, err)
	}
	return id
}

func TestCreateExercise(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")

	got := createExercise(t, h, ada, `{
		"name": "  Repeaters 7:3  ",
		"kind": "timed_reps",
		"notes": "   ",
		"tags": [" hangboard ", "", "fingers"],
		"sets": 4,
		"rep_seconds": 7,
		"rep_rest_seconds": 0,
		"video_url": "https://www.youtube.com/watch?v=abc"
	}`)

	if got.ID == "" || got.Shipped {
		t.Fatalf("id %q, shipped %v, want an id on one of your own", got.ID, got.Shipped)
	}
	if got.Name != "Repeaters 7:3" {
		t.Fatalf("name %q, want it trimmed", got.Name)
	}
	if got.Notes != nil {
		t.Fatalf("notes %q, want a blank one stored as not set", *got.Notes)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "hangboard" || got.Tags[1] != "fingers" {
		t.Fatalf("tags %q, want them trimmed and the empty one dropped", got.Tags)
	}
	if got.RepRestSeconds == nil || *got.RepRestSeconds != 0 {
		t.Fatalf("rep rest %v, want 0 kept apart from not set", got.RepRestSeconds)
	}
	if got.Reps != nil {
		t.Fatalf("reps %v, want not set", *got.Reps)
	}
	if got.RetiredAt != nil {
		t.Fatal("a new exercise is retired")
	}
}

func TestExerciseValidation(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")

	cases := map[string]struct {
		body  string
		field string
	}{
		"no name":           {`{"name":"  ","kind":"open"}`, "name"},
		"long name":         {`{"name":"` + strings.Repeat("a", 201) + `","kind":"open"}`, "name"},
		"no kind":           {`{"name":"Hang"}`, "kind"},
		"unknown kind":      {`{"name":"Hang","kind":"session"}`, "kind"},
		"negative sets":     {`{"name":"Hang","kind":"open","sets":-1}`, "sets"},
		"huge duration":     {`{"name":"Hang","kind":"open","duration_seconds":3000000000}`, "duration_seconds"},
		"javascript video":  {`{"name":"Hang","kind":"open","video_url":"javascript:alert(1)"}`, "video_url"},
		"ftp thumbnail":     {`{"name":"Hang","kind":"open","thumbnail_url":"ftp://example.com/a.jpg"}`, "thumbnail_url"},
		"link with no host": {`{"name":"Hang","kind":"open","video_url":"https://"}`, "video_url"},
		"unknown field":     {`{"name":"Hang","kind":"open","weight_kg":10}`, "body"},
		"not json":          {`hunter2`, "body"},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			rec := send(t, h, http.MethodPost, "/api/v1/exercises", ada, c.body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body)
			}

			var body errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response: %v", err)
			}
			if _, ok := body.Error.Fields[c.field]; !ok {
				t.Fatalf("no error on %q, got %v", c.field, body.Error.Fields)
			}
		})
	}
}

func TestListExercises(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")

	insertShipped(t, pool, "Max Hangs")
	createExercise(t, h, ada, `{"name":"Bench Press","kind":"reps_and_sets"}`)
	createExercise(t, h, bob, `{"name":"Bob's Squat","kind":"reps_and_sets"}`)
	old := createExercise(t, h, ada, `{"name":"Old Drill","kind":"open"}`)
	if rec := send(t, h, http.MethodPost, "/api/v1/exercises/"+old.ID+"/retire", ada, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("retire: status %d: %s", rec.Code, rec.Body)
	}

	rec := send(t, h, http.MethodGet, "/api/v1/exercises", ada, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	var got exerciseListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response: %v", err)
	}
	if len(got.Exercises) != 2 {
		t.Fatalf("listed %d, want 2: %+v", len(got.Exercises), got.Exercises)
	}
	if got.Exercises[0].Name != "Bench Press" || got.Exercises[0].Shipped {
		t.Fatalf("first %+v, want your own Bench Press", got.Exercises[0])
	}
	if got.Exercises[1].Name != "Max Hangs" || !got.Exercises[1].Shipped {
		t.Fatalf("second %+v, want the shipped Max Hangs", got.Exercises[1])
	}
}

// Empty lists must still be lists, or a client reading them breaks.
func TestEmptyListsAreNotNull(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")

	rec := send(t, h, http.MethodGet, "/api/v1/exercises", ada, "")
	if !strings.Contains(rec.Body.String(), `"exercises":[]`) {
		t.Fatalf("body %s, want an empty library", rec.Body)
	}

	rec = send(t, h, http.MethodPost, "/api/v1/exercises", ada, `{"name":"Hang","kind":"open"}`)
	if !strings.Contains(rec.Body.String(), `"tags":[]`) {
		t.Fatalf("body %s, want empty tags", rec.Body)
	}
}

func TestReadExercise(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")

	shipped := insertShipped(t, pool, "Max Hangs")
	own := createExercise(t, h, ada, `{"name":"Bench Press","kind":"reps_and_sets"}`)
	bobs := createExercise(t, h, bob, `{"name":"Bob's Squat","kind":"reps_and_sets"}`)

	for name, want := range map[string]struct {
		id      string
		shipped bool
	}{
		"shipped": {shipped, true},
		"own":     {own.ID, false},
	} {
		t.Run(name, func(t *testing.T) {
			rec := send(t, h, http.MethodGet, "/api/v1/exercises/"+want.id, ada, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			if got := decodeExercise(t, rec); got.ID != want.id || got.Shipped != want.shipped {
				t.Fatalf("id %s, shipped %v, want %s and %v", got.ID, got.Shipped, want.id, want.shipped)
			}
		})
	}

	t.Run("retired, still read", func(t *testing.T) {
		send(t, h, http.MethodPost, "/api/v1/exercises/"+own.ID+"/retire", ada, "")
		rec := send(t, h, http.MethodGet, "/api/v1/exercises/"+own.ID, ada, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		if decodeExercise(t, rec).RetiredAt == nil {
			t.Fatal("retired_at is not set")
		}
	})

	for name, id := range map[string]string{"someone else's": bobs.ID, "not a uuid": "nope"} {
		t.Run(name, func(t *testing.T) {
			if rec := send(t, h, http.MethodGet, "/api/v1/exercises/"+id, ada, ""); rec.Code != http.StatusNotFound {
				t.Fatalf("status %d, want 404: %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestUpdateExercise(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")

	own := createExercise(t, h, ada, `{"name":"Bench","kind":"reps_and_sets","notes":"old"}`)
	shipped := insertShipped(t, pool, "Max Hangs")
	bobs := createExercise(t, h, bob, `{"name":"Bob's Squat","kind":"reps_and_sets"}`)

	body := `{"name":"Bench Press","kind":"reps_and_sets","sets":5,"reps":5}`

	rec := send(t, h, http.MethodPut, "/api/v1/exercises/"+own.ID, ada, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	got := decodeExercise(t, rec)
	if got.Name != "Bench Press" || got.Sets == nil || *got.Sets != 5 {
		t.Fatalf("got %+v, want the new name and 5 sets", got)
	}
	if got.Notes != nil {
		t.Fatalf("notes %q, want a field left out to be cleared", *got.Notes)
	}

	for name, id := range map[string]string{"shipped": shipped, "someone else's": bobs.ID, "not a uuid": "nope"} {
		t.Run(name, func(t *testing.T) {
			if rec := send(t, h, http.MethodPut, "/api/v1/exercises/"+id, ada, body); rec.Code != http.StatusNotFound {
				t.Fatalf("status %d, want 404: %s", rec.Code, rec.Body)
			}
		})
	}

	t.Run("a bad body", func(t *testing.T) {
		rec := send(t, h, http.MethodPut, "/api/v1/exercises/"+own.ID, ada, `{"name":"","kind":"open"}`)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body)
		}
	})
}

func TestRetireExercise(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")

	own := createExercise(t, h, ada, `{"name":"Bench Press","kind":"reps_and_sets"}`)
	shipped := insertShipped(t, pool, "Max Hangs")
	bobs := createExercise(t, h, bob, `{"name":"Bob's Squat","kind":"reps_and_sets"}`)

	for i := range 2 {
		rec := send(t, h, http.MethodPost, "/api/v1/exercises/"+own.ID+"/retire", ada, "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("retire %d: status %d: %s", i+1, rec.Code, rec.Body)
		}
	}

	for name, id := range map[string]string{"shipped": shipped, "someone else's": bobs.ID, "not a uuid": "nope"} {
		t.Run(name, func(t *testing.T) {
			rec := send(t, h, http.MethodPost, "/api/v1/exercises/"+id+"/retire", ada, "")
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status %d, want 404: %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestExercisesNeedSignIn(t *testing.T) {
	h := newTestServer(t)
	id := "01a0bf77-d7e8-76ea-96cc-f09cbca175a3"

	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/exercises"},
		{http.MethodPost, "/api/v1/exercises"},
		{http.MethodGet, "/api/v1/exercises/" + id},
		{http.MethodPut, "/api/v1/exercises/" + id},
		{http.MethodPost, "/api/v1/exercises/" + id + "/retire"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			if rec := send(t, h, route.method, route.path, "", ""); rec.Code != http.StatusUnauthorized {
				t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body)
			}
		})
	}
}
