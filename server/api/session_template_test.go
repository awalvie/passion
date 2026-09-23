package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func decodeSessionTemplate(t *testing.T, rec *httptest.ResponseRecorder) sessionTemplateResponse {
	t.Helper()
	var got sessionTemplateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response: %v (%s)", err, rec.Body)
	}
	return got
}

func createSessionTemplate(t *testing.T, h http.Handler, bearer, body string) sessionTemplateResponse {
	t.Helper()
	rec := send(t, h, http.MethodPost, "/api/v1/session-templates", bearer, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d: %s", rec.Code, rec.Body)
	}
	return decodeSessionTemplate(t, rec)
}

// insertShippedSessionTemplate writes a shipped row directly, as the catalog
// loader will.
func insertShippedSessionTemplate(t *testing.T, pool *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO session_template (name, body) VALUES ($1, '{"sections": []}') RETURNING id`, name).Scan(&id)
	if err != nil {
		t.Fatalf("insert %s: %v", name, err)
	}
	return id
}

// sessionBody is a session with one section that holds a step and a choice.
func sessionBody(name, stepExercise, optionExercise string) string {
	return fmt.Sprintf(`{
		"name": %q,
		"sections": [{
			"name": "Main",
			"items": [
				{"step": {"exercise": %q, "name": "Max Hangs", "kind": "timed_reps", "sets": 5, "notes": " Half crimp. "}},
				{"choice": {"name": "Shoulders", "pick": 1, "options": [
					{"exercise": %q, "name": "Y raises", "kind": "reps_and_sets", "reps": 10, "per_side": true}
				]}}
			]
		}]
	}`, name, stepExercise, optionExercise)
}

func TestCreateSessionTemplate(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")

	hang := insertShipped(t, pool, "Max Hangs")
	raise := createExercise(t, h, ada, `{"name":"Y raises","kind":"reps_and_sets"}`)

	got := createSessionTemplate(t, h, ada, sessionBody("  Power  ", hang, raise.ID))

	if got.ID == "" || got.Shipped || got.Name != "Power" {
		t.Fatalf("id %q, shipped %v, name %q, want your own Power", got.ID, got.Shipped, got.Name)
	}
	if len(got.Sections) != 1 || len(got.Sections[0].Items) != 2 {
		t.Fatalf("sections %+v, want one with two items", got.Sections)
	}

	step := got.Sections[0].Items[0].Step
	if step == nil || step.Exercise != hang || step.Sets == nil || *step.Sets != 5 {
		t.Fatalf("step %+v, want Max Hangs with 5 sets", step)
	}
	if step.Notes == nil || *step.Notes != "Half crimp." {
		t.Fatalf("step notes %v, want them trimmed", step.Notes)
	}

	choice := got.Sections[0].Items[1].Choice
	if choice == nil || choice.Pick != 1 || len(choice.Options) != 1 || choice.Options[0].Exercise != raise.ID {
		t.Fatalf("choice %+v, want pick 1 of Y raises", choice)
	}
	if !choice.Options[0].PerSide || step.PerSide {
		t.Fatalf("per side %v on the option and %v on the step, want true and false", choice.Options[0].PerSide, step.PerSide)
	}

	rec := send(t, h, http.MethodGet, "/api/v1/session-templates/"+got.ID, ada, "")
	if read := decodeSessionTemplate(t, rec); len(read.Sections) != 1 || read.Sections[0].Items[1].Choice == nil {
		t.Fatalf("read back %+v", read)
	}
}

func TestSessionTemplateValidation(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")

	hang := insertShipped(t, pool, "Max Hangs")
	bobs := createExercise(t, h, bob, `{"name":"Bob's Squat","kind":"reps_and_sets"}`)

	section := func(items string) string {
		return `{"name":"Power","sections":[{"name":"Main","items":[` + items + `]}]}`
	}
	step := `{"exercise":"` + hang + `","name":"Max Hangs","kind":"open"}`

	cases := map[string]struct {
		body  string
		field string
	}{
		"no name":            {`{"name":" "}`, "name"},
		"bad colour":         {`{"name":"Power","color":"blue"}`, "color"},
		"no section name":    {`{"name":"Power","sections":[{"name":""}]}`, "sections[0].name"},
		"empty item":         {section(`{}`), "sections[0].items[0]"},
		"step and choice":    {section(`{"step":` + step + `,"choice":{"name":"Pick","options":[` + step + `]}}`), "sections[0].items[0]"},
		"pick too high":      {section(`{"choice":{"name":"Pick","pick":2,"options":[` + step + `]}}`), "sections[0].items[0].choice.pick"},
		"bad step kind":      {section(`{"step":{"exercise":"` + hang + `","name":"Hang","kind":"nope"}}`), "sections[0].items[0].step.kind"},
		"step id not a uuid": {section(`{"step":{"exercise":"nope","name":"Hang","kind":"open"}}`), "sections[0].items[0].step.exercise"},
		"someone else's":     {section(`{"step":{"exercise":"` + bobs.ID + `","name":"Squat","kind":"open"}}`), "sections[0].items[0].step.exercise"},
		"unknown step field": {section(`{"step":{"exercise":"` + hang + `","name":"Hang","kind":"open","weight_kg":10}}`), "body"},
		"not json":           {`hunter2`, "body"},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			rec := send(t, h, http.MethodPost, "/api/v1/session-templates", ada, c.body)
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

// A session copies each exercise's notes into its steps, so its body can be
// far larger than any other request.
func TestSessionTemplateTakesALargeBody(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	hang := insertShipped(t, pool, "Max Hangs")

	notes := strings.Repeat("Keep the shoulders set. ", 60)
	steps := make([]string, 0, 60)
	for range 60 {
		steps = append(steps, fmt.Sprintf(`{"step":{"exercise":%q,"name":"Max Hangs","kind":"open","notes":%q}}`, hang, notes))
	}
	body := `{"name":"Long Day","sections":[{"name":"Main","items":[` + strings.Join(steps, ",") + `]}]}`
	if len(body) <= maxBodyBytes {
		t.Fatalf("body is %d bytes, want more than the usual cap", len(body))
	}

	createSessionTemplate(t, h, ada, body)
}

func TestListSessionTemplates(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")

	insertShippedSessionTemplate(t, pool, "Power")
	createSessionTemplate(t, h, ada, `{"name":"Endurance"}`)
	createSessionTemplate(t, h, bob, `{"name":"Bob's Day"}`)
	old := createSessionTemplate(t, h, ada, `{"name":"Old Day"}`)
	if rec := send(t, h, http.MethodPost, "/api/v1/session-templates/"+old.ID+"/retire", ada, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("retire: status %d: %s", rec.Code, rec.Body)
	}

	rec := send(t, h, http.MethodGet, "/api/v1/session-templates", ada, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	var got sessionTemplateListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response: %v", err)
	}
	if len(got.SessionTemplates) != 2 {
		t.Fatalf("listed %d, want 2: %+v", len(got.SessionTemplates), got.SessionTemplates)
	}
	if got.SessionTemplates[0].Name != "Endurance" || got.SessionTemplates[0].Shipped {
		t.Fatalf("first %+v, want your own Endurance", got.SessionTemplates[0])
	}
	if got.SessionTemplates[1].Name != "Power" || !got.SessionTemplates[1].Shipped {
		t.Fatalf("second %+v, want the shipped Power", got.SessionTemplates[1])
	}
}

// Empty lists must still be lists, or a client reading them breaks.
func TestSessionTemplateEmptyListsAreNotNull(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")

	rec := send(t, h, http.MethodGet, "/api/v1/session-templates", ada, "")
	if !strings.Contains(rec.Body.String(), `"session_templates":[]`) {
		t.Fatalf("body %s, want an empty list", rec.Body)
	}

	rec = send(t, h, http.MethodPost, "/api/v1/session-templates", ada, `{"name":"Power"}`)
	if !strings.Contains(rec.Body.String(), `"sections":[]`) || !strings.Contains(rec.Body.String(), `"tags":[]`) {
		t.Fatalf("body %s, want empty sections and tags", rec.Body)
	}
}

func TestReadSessionTemplate(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")

	shipped := insertShippedSessionTemplate(t, pool, "Power")
	own := createSessionTemplate(t, h, ada, `{"name":"Endurance"}`)
	bobs := createSessionTemplate(t, h, bob, `{"name":"Bob's Day"}`)

	for name, want := range map[string]struct {
		id      string
		shipped bool
	}{
		"shipped": {shipped, true},
		"own":     {own.ID, false},
	} {
		t.Run(name, func(t *testing.T) {
			rec := send(t, h, http.MethodGet, "/api/v1/session-templates/"+want.id, ada, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			if got := decodeSessionTemplate(t, rec); got.ID != want.id || got.Shipped != want.shipped {
				t.Fatalf("id %s, shipped %v, want %s and %v", got.ID, got.Shipped, want.id, want.shipped)
			}
		})
	}

	t.Run("retired, still read", func(t *testing.T) {
		send(t, h, http.MethodPost, "/api/v1/session-templates/"+own.ID+"/retire", ada, "")
		rec := send(t, h, http.MethodGet, "/api/v1/session-templates/"+own.ID, ada, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		if decodeSessionTemplate(t, rec).RetiredAt == nil {
			t.Fatal("retired_at is not set")
		}
	})

	for name, id := range map[string]string{"someone else's": bobs.ID, "not a uuid": "nope"} {
		t.Run(name, func(t *testing.T) {
			if rec := send(t, h, http.MethodGet, "/api/v1/session-templates/"+id, ada, ""); rec.Code != http.StatusNotFound {
				t.Fatalf("status %d, want 404: %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestUpdateSessionTemplate(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")

	hang := insertShipped(t, pool, "Max Hangs")
	own := createSessionTemplate(t, h, ada, `{"name":"Power","notes":"old"}`)
	shipped := insertShippedSessionTemplate(t, pool, "Endurance")
	bobs := createSessionTemplate(t, h, bob, `{"name":"Bob's Day"}`)

	body := sessionBody("Power Day", hang, hang)

	rec := send(t, h, http.MethodPut, "/api/v1/session-templates/"+own.ID, ada, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	got := decodeSessionTemplate(t, rec)
	if got.Name != "Power Day" || len(got.Sections) != 1 {
		t.Fatalf("got %+v, want the new name and one section", got)
	}
	if got.Notes != nil {
		t.Fatalf("notes %q, want a field left out to be cleared", *got.Notes)
	}

	for name, id := range map[string]string{"shipped": shipped, "someone else's": bobs.ID, "not a uuid": "nope"} {
		t.Run(name, func(t *testing.T) {
			if rec := send(t, h, http.MethodPut, "/api/v1/session-templates/"+id, ada, body); rec.Code != http.StatusNotFound {
				t.Fatalf("status %d, want 404: %s", rec.Code, rec.Body)
			}
		})
	}

	t.Run("a bad body", func(t *testing.T) {
		rec := send(t, h, http.MethodPut, "/api/v1/session-templates/"+own.ID, ada, `{"name":""}`)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body)
		}
	})
}

func TestRetireSessionTemplate(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")

	own := createSessionTemplate(t, h, ada, `{"name":"Power"}`)
	shipped := insertShippedSessionTemplate(t, pool, "Endurance")
	bobs := createSessionTemplate(t, h, bob, `{"name":"Bob's Day"}`)

	for i := range 2 {
		rec := send(t, h, http.MethodPost, "/api/v1/session-templates/"+own.ID+"/retire", ada, "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("retire %d: status %d: %s", i+1, rec.Code, rec.Body)
		}
	}

	for name, id := range map[string]string{"shipped": shipped, "someone else's": bobs.ID, "not a uuid": "nope"} {
		t.Run(name, func(t *testing.T) {
			rec := send(t, h, http.MethodPost, "/api/v1/session-templates/"+id+"/retire", ada, "")
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status %d, want 404: %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestSessionTemplatesNeedSignIn(t *testing.T) {
	h := newTestServer(t)
	id := "01a0bf77-d7e8-76ea-96cc-f09cbca175a3"

	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/session-templates"},
		{http.MethodPost, "/api/v1/session-templates"},
		{http.MethodGet, "/api/v1/session-templates/" + id},
		{http.MethodPut, "/api/v1/session-templates/" + id},
		{http.MethodPost, "/api/v1/session-templates/" + id + "/retire"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			if rec := send(t, h, route.method, route.path, "", ""); rec.Code != http.StatusUnauthorized {
				t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body)
			}
		})
	}
}
