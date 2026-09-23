package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func decodeRun(t *testing.T, rec *httptest.ResponseRecorder) runResponse {
	t.Helper()
	var got runResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response: %v (%s)", err, rec.Body)
	}
	return got
}

func startRunFor(t *testing.T, h http.Handler, bearer, body string) runResponse {
	t.Helper()
	rec := send(t, h, http.MethodPost, "/api/v1/runs", bearer, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("start: status %d: %s", rec.Code, rec.Body)
	}
	return decodeRun(t, rec)
}

// runRequestFrom turns a run as read into the body of a PUT, after edit has
// changed it.
func runRequestFrom(t *testing.T, run runResponse, edit func(map[string]any)) string {
	t.Helper()
	raw, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "template", "plan", "timezone", "finished_at"} {
		delete(body, key)
	}
	edit(body)
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestStartRun(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")

	hang := insertShipped(t, pool, "Max Hangs")
	raise := createExercise(t, h, ada, `{"name":"Y raises","kind":"reps_and_sets"}`)
	tpl := createSessionTemplate(t, h, ada, sessionBody("Power", hang, raise.ID))

	run := startRunFor(t, h, ada, `{"template": "`+tpl.ID+`", "started_at": "2026-03-03T18:00:00Z"}`)

	if run.Name != "Power" || run.Template == nil || *run.Template != tpl.ID || run.FinishedAt != nil {
		t.Fatalf("run %+v, want an unfinished copy of Power", run)
	}
	if run.LocalDate != "2026-03-03" {
		t.Fatalf("local date %q, want the day it started in UTC", run.LocalDate)
	}
	if len(run.Plan) != 1 || len(run.Sections) != 1 {
		t.Fatalf("plan %+v, sections %+v, want one each", run.Plan, run.Sections)
	}
	items := run.Sections[0].Items
	if items[0].Step == nil || items[0].Step.ID == "" || items[0].Step.Exercise != hang {
		t.Fatalf("step %+v, want Max Hangs with an id", items[0].Step)
	}
	if items[1].Choice == nil || items[1].Choice.ID == "" {
		t.Fatalf("choice %+v, want the offer with an id", items[1].Choice)
	}
}

func TestStartOpenRun(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")

	rec := send(t, h, http.MethodPost, "/api/v1/runs", ada, `{"name": " Saturday ", "local_date": "2026-03-07"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if raw["plan"] != nil || raw["name"] != "Saturday" || raw["local_date"] != "2026-03-07" {
		t.Fatalf("got %s, want an open run on the day named", rec.Body)
	}
	if sections, ok := raw["sections"].([]any); !ok || len(sections) != 0 {
		t.Fatalf("sections %v, want an empty list", raw["sections"])
	}
}

func TestStartRunValidation(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	bobs := createSessionTemplate(t, h, bob, sessionBody("Bob's", insertShipped(t, pool, "Hang"), insertShipped(t, pool, "Pull")))

	for name, c := range map[string]struct{ body, field string }{
		"no name":                 {`{}`, "name"},
		"a bad template":          {`{"template": "nope"}`, "template"},
		"someone else's template": {`{"template": "` + bobs.ID + `"}`, "template"},
		"a bad day":               {`{"name": "Open", "local_date": "7 March"}`, "local_date"},
	} {
		t.Run(name, func(t *testing.T) {
			rec := send(t, h, http.MethodPost, "/api/v1/runs", ada, c.body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body)
			}
			if got := decodeBody(t, rec).Error.Fields; got[c.field] == "" {
				t.Fatalf("fields %v, want %s named", got, c.field)
			}
		})
	}
}

func TestUpdateRun(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")

	hang := insertShipped(t, pool, "Max Hangs")
	raise := insertShipped(t, pool, "Y raises")
	tpl := createSessionTemplate(t, h, ada, sessionBody("Power", hang, raise))
	run := startRunFor(t, h, ada, `{"template": "`+tpl.ID+`"}`)

	// Mark the hang done, and pick from the choice: its item becomes a step
	// that keeps the offer.
	body := runRequestFrom(t, run, func(b map[string]any) {
		items := b["sections"].([]any)[0].(map[string]any)["items"].([]any)
		first := items[0].(map[string]any)["step"].(map[string]any)
		first["status"] = "done"
		first["run_notes"] = "right shoulder tight"
		offer := items[1].(map[string]any)["choice"]
		kept, _ := json.Marshal(offer)
		picked := offer.(map[string]any)["options"].([]any)[0].(map[string]any)
		picked["id"] = "0199c3a0-0000-7000-8000-0000000000aa"
		picked["from_choice"] = json.RawMessage(kept)
		items[1] = map[string]any{"step": picked}
		b["place"] = "The Castle"
		b["sleep"] = 3
	})

	rec := send(t, h, http.MethodPut, "/api/v1/runs/"+run.ID, ada, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	got := decodeRun(t, rec)
	items := got.Sections[0].Items
	if *items[0].Step.Status != "done" || *items[0].Step.RunNotes != "right shoulder tight" {
		t.Fatalf("first step %+v", items[0].Step)
	}
	if items[1].Step == nil || items[1].Step.FromChoice == nil || items[1].Step.FromChoice.Name != "Shoulders" {
		t.Fatalf("second item %+v, want the pick with its offer", items[1])
	}
	if *got.Place != "The Castle" || *got.Sleep != 3 || len(got.Plan) != 1 {
		t.Fatalf("run %+v, want the place, the journal and the plan untouched", got)
	}

	noStart := runRequestFrom(t, run, func(b map[string]any) { delete(b, "started_at") })
	if rec := send(t, h, http.MethodPut, "/api/v1/runs/"+run.ID, ada, noStart); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d without started_at, want 422: %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, http.MethodPut, "/api/v1/runs/"+run.ID, bob, body); rec.Code != http.StatusNotFound {
		t.Fatalf("status %d for someone else's run, want 404: %s", rec.Code, rec.Body)
	}
}

func TestFinishRun(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	tpl := createSessionTemplate(t, h, ada, sessionBody("Power", insertShipped(t, pool, "Hang"), insertShipped(t, pool, "Pull")))
	run := startRunFor(t, h, ada, `{"template": "`+tpl.ID+`"}`)

	rec := send(t, h, http.MethodPost, "/api/v1/runs/"+run.ID+"/finish", ada, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	got := decodeRun(t, rec)
	if got.FinishedAt == nil || *got.Sections[0].Items[0].Step.Status != "skipped" {
		t.Fatalf("finished %v, status %v, want the untouched step skipped", got.FinishedAt, got.Sections[0].Items[0].Step.Status)
	}
}

func TestListAndDeleteRuns(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")

	tuesday := startRunFor(t, h, ada, `{"name": "Tuesday", "local_date": "2026-03-03"}`)
	startRunFor(t, h, ada, `{"name": "Thursday", "local_date": "2026-03-05"}`)

	rec := send(t, h, http.MethodGet, "/api/v1/runs", ada, "")
	var list runListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range list.Runs {
		names = append(names, r.Name)
	}
	if !slices.Equal(names, []string{"Thursday", "Tuesday"}) {
		t.Fatalf("listed %q, want newest day first", names)
	}

	if rec := send(t, h, http.MethodDelete, "/api/v1/runs/"+tuesday.ID, ada, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d: %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, http.MethodGet, "/api/v1/runs/"+tuesday.ID, ada, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("read after delete: status %d, want 404", rec.Code)
	}
}

func TestRunsNeedSignIn(t *testing.T) {
	h := newTestServer(t)
	id := "01a0bf77-d7e8-76ea-96cc-f09cbca175a3"

	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/runs"},
		{http.MethodPost, "/api/v1/runs"},
		{http.MethodGet, "/api/v1/runs/" + id},
		{http.MethodPut, "/api/v1/runs/" + id},
		{http.MethodPost, "/api/v1/runs/" + id + "/finish"},
		{http.MethodDelete, "/api/v1/runs/" + id},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			if rec := send(t, h, route.method, route.path, "", ""); rec.Code != http.StatusUnauthorized {
				t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body)
			}
		})
	}
}
