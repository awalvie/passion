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
	for _, key := range []string{"id", "template", "scheduled", "plan", "timezone", "finished_at", "sets", "climbs"} {
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
		"a day that is not yours": {`{"scheduled": "0199c3a0-0000-7000-8000-0000000000ff"}`, "scheduled"},
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

func TestSkipChoice(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	tpl := createSessionTemplate(t, h, ada, sessionBody("Power", insertShipped(t, pool, "Hang"), insertShipped(t, pool, "Pull")))
	run := startRunFor(t, h, ada, `{"template": "`+tpl.ID+`"}`)
	if run.Sections[0].Items[1].Choice.Skipped {
		t.Fatal("a new run's choice is skipped, want it open")
	}

	body := runRequestFrom(t, run, func(b map[string]any) {
		items := b["sections"].([]any)[0].(map[string]any)["items"].([]any)
		items[1].(map[string]any)["choice"].(map[string]any)["skipped"] = true
	})
	rec := send(t, h, http.MethodPut, "/api/v1/runs/"+run.ID, ada, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if !decodeRun(t, rec).Sections[0].Items[1].Choice.Skipped {
		t.Fatal("the choice is not skipped after the update")
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

// runWithSteps starts an open run holding one hang, typed in with an
// exercise id of its own, and one climbing step.
func runWithSteps(t *testing.T, h http.Handler, bearer string) (runResponse, string, string) {
	t.Helper()
	const hang, climb = "0199c3a0-0000-7000-8000-0000000000a1", "0199c3a0-0000-7000-8000-0000000000a2"
	run := startRunFor(t, h, bearer, `{"name": "Open"}`)
	body := runRequestFrom(t, run, func(b map[string]any) {
		b["sections"] = []any{map[string]any{"name": "Main", "items": []any{
			map[string]any{"step": map[string]any{"id": hang, "exercise": "0199c3a0-0000-7000-8000-0000000000b1", "name": "Hang", "kind": "timed_reps"}},
			map[string]any{"step": map[string]any{"id": climb, "exercise": "0199c3a0-0000-7000-8000-0000000000b2", "name": "Bouldering", "kind": "climbing"}},
		}}}
	})
	rec := send(t, h, http.MethodPut, "/api/v1/runs/"+run.ID, bearer, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("put body: status %d: %s", rec.Code, rec.Body)
	}
	return decodeRun(t, rec), hang, climb
}

func TestReplaceSets(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	run, hang, climb := runWithSteps(t, h, ada)
	path := "/api/v1/runs/" + run.ID + "/steps/" + hang + "/sets"

	rec := send(t, h, http.MethodPut, path, ada, `{"sets": [{"seconds": 10, "weight_kg": -5}, {"reps": 2, "rep_seconds": [3, 6]}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var written setListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &written); err != nil {
		t.Fatal(err)
	}
	if len(written.Sets) != 2 || written.Sets[1].Number != 2 || *written.Sets[0].WeightKG != -5 {
		t.Fatalf("sets %+v", written.Sets)
	}
	if written.Sets[0].RepSeconds != nil || !slices.Equal(written.Sets[1].RepSeconds, []int{3, 6}) {
		t.Fatalf("rep seconds %v and %v, want none, then a ladder's", written.Sets[0].RepSeconds, written.Sets[1].RepSeconds)
	}

	read := decodeRun(t, send(t, h, http.MethodGet, "/api/v1/runs/"+run.ID, ada, ""))
	if len(read.Sets) != 2 || *read.Sections[0].Items[0].Step.Status != "done" {
		t.Fatalf("run reads %d sets and status %v, want 2 and done", len(read.Sets), read.Sections[0].Items[0].Step.Status)
	}

	for name, c := range map[string]struct {
		path, body string
		status     int
	}{
		"a bad number":              {path, `{"sets": [{"reps": -1}]}`, http.StatusUnprocessableEntity},
		"two kinds of time":         {path, `{"sets": [{"seconds": 9, "rep_seconds": [3, 6, 9]}]}`, http.StatusUnprocessableEntity},
		"a climbing step":           {"/api/v1/runs/" + run.ID + "/steps/" + climb + "/sets", `{"sets": []}`, http.StatusUnprocessableEntity},
		"a step the run lacks":      {"/api/v1/runs/" + run.ID + "/steps/0199c3a0-0000-7000-8000-0000000000ff/sets", `{"sets": []}`, http.StatusNotFound},
		"a run that does not exist": {"/api/v1/runs/0199c3a0-0000-7000-8000-0000000000ff/steps/" + hang + "/sets", `{"sets": []}`, http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			if rec := send(t, h, http.MethodPut, c.path, ada, c.body); rec.Code != c.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, c.status, rec.Body)
			}
		})
	}

	// Skipping the step drops its sets.
	skip := runRequestFrom(t, read, func(b map[string]any) {
		step := b["sections"].([]any)[0].(map[string]any)["items"].([]any)[0].(map[string]any)["step"].(map[string]any)
		step["status"] = "skipped"
	})
	if got := decodeRun(t, send(t, h, http.MethodPut, "/api/v1/runs/"+run.ID, ada, skip)); len(got.Sets) != 0 {
		t.Fatalf("%d sets after the step was skipped, want none", len(got.Sets))
	}
}

func TestClimbs(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	run, hang, climb := runWithSteps(t, h, ada)
	path := "/api/v1/runs/" + run.ID + "/climbs/0199c3a0-0000-7000-8000-0000000000c1"
	flash := `{"step": "` + climb + `", "discipline": "boulder", "setting": "indoor", "grade": "6c", "grade_system": "font", "goes": ["attempt", "send", "send"]}`

	for range 2 {
		rec := send(t, h, http.MethodPut, path, ada, flash)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var got climbBody
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if !got.Sent || got.Grade == nil || *got.Grade != "6c" || len(got.Goes) != 3 {
			t.Fatalf("climb %+v, want a sent 6c with its three goes", got)
		}
	}
	read := decodeRun(t, send(t, h, http.MethodGet, "/api/v1/runs/"+run.ID, ada, ""))
	if len(read.Climbs) != 1 || *read.Sections[0].Items[1].Step.Status != "done" {
		t.Fatalf("run reads %d climbs and status %v, want one after a retry, and done", len(read.Climbs), read.Sections[0].Items[1].Step.Status)
	}

	for name, c := range map[string]struct {
		who, path, body, field string
		status                 int
	}{
		"a grade off its scale": {ada, path, `{"step": "` + climb + `", "discipline": "boulder", "setting": "indoor", "grade": "V4", "grade_system": "font"}`, "grade", http.StatusUnprocessableEntity},
		"a step that logs sets": {ada, path, `{"step": "` + hang + `", "discipline": "boulder", "setting": "indoor"}`, "step", http.StatusUnprocessableEntity},
		"someone else's run":    {bob, path, flash, "", http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			rec := send(t, h, http.MethodPut, c.path, c.who, c.body)
			if rec.Code != c.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, c.status, rec.Body)
			}
			if c.field != "" && decodeBody(t, rec).Error.Fields[c.field] == "" {
				t.Fatalf("fields %s, want %s named", rec.Body, c.field)
			}
		})
	}

	if rec := send(t, h, http.MethodDelete, path, ada, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d: %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, http.MethodDelete, path, ada, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("delete twice: status %d, want 404", rec.Code)
	}
}

func TestListClimbs(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	const step = "0199c3a0-0000-7000-8000-0000000000a2"
	climbOn := func(who, day, climb string, finish bool) string {
		t.Helper()
		run := startRunFor(t, h, who, `{"name": "Open", "local_date": "`+day+`"}`)
		body := runRequestFrom(t, run, func(b map[string]any) {
			b["sections"] = []any{map[string]any{"name": "Main", "items": []any{
				map[string]any{"step": map[string]any{"id": step, "exercise": "0199c3a0-0000-7000-8000-0000000000b2", "name": "Bouldering", "kind": "climbing"}},
			}}}
		})
		if rec := send(t, h, http.MethodPut, "/api/v1/runs/"+run.ID, who, body); rec.Code != http.StatusOK {
			t.Fatalf("put body: status %d: %s", rec.Code, rec.Body)
		}
		path := "/api/v1/runs/" + run.ID + "/climbs/" + climb
		if rec := send(t, h, http.MethodPut, path, who, `{"step": "`+step+`", "discipline": "boulder", "setting": "indoor"}`); rec.Code != http.StatusOK {
			t.Fatalf("put climb: status %d: %s", rec.Code, rec.Body)
		}
		if finish {
			send(t, h, http.MethodPost, "/api/v1/runs/"+run.ID+"/finish", who, "")
		}
		return run.ID
	}
	climbOn(ada, "2026-03-06", "0199c3a0-0000-7000-8000-0000000000c1", true)
	first := climbOn(ada, "2026-03-07", "0199c3a0-0000-7000-8000-0000000000c2", true)
	climbOn(ada, "2026-03-08", "0199c3a0-0000-7000-8000-0000000000c3", true)
	climbOn(ada, "2026-03-08", "0199c3a0-0000-7000-8000-0000000000c4", false)
	climbOn(ada, "2026-03-09", "0199c3a0-0000-7000-8000-0000000000c5", true)
	climbOn(bob, "2026-03-07", "0199c3a0-0000-7000-8000-0000000000c6", true)

	rec := send(t, h, http.MethodGet, "/api/v1/climbs?from=2026-03-07&to=2026-03-08", ada, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: status %d: %s", rec.Code, rec.Body)
	}
	var got climbListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	var days []string
	for _, c := range got.Climbs {
		days = append(days, c.LocalDate)
	}
	if !slices.Equal(days, []string{"2026-03-07", "2026-03-08"}) || got.Climbs[0].Run != first {
		t.Fatalf("climbs %+v, want ada's finished ones on the 7th and the 8th", got.Climbs)
	}

	for name, query := range map[string]string{
		"no range":         "",
		"a bad day":        "?from=soon&to=2026-03-08",
		"a range reversed": "?from=2026-03-08&to=2026-03-07",
	} {
		t.Run(name, func(t *testing.T) {
			if rec := send(t, h, http.MethodGet, "/api/v1/climbs"+query, ada, ""); rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want 422", rec.Code)
			}
		})
	}
}

// A started_at that reads back past year 9999, or before year 0, cannot be
// written as JSON. Taking one answers an empty body, and empties the run list
// for good, so it must be refused.
func TestRunsRefuseAStartedAtJSONCannotHold(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	run := startRunFor(t, h, ada, `{"name": "Open"}`)

	refused := func(t *testing.T, rec *httptest.ResponseRecorder) {
		t.Helper()
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("status %d with body %q, want 422", rec.Code, rec.Body)
			return
		}
		if decodeBody(t, rec).Error.Fields["started_at"] == "" {
			t.Errorf("fields %s, want started_at named", rec.Body)
		}
	}
	for name, at := range map[string]string{
		"past year 9999": "9999-12-31T23:59:59-10:00",
		"before year 0":  "0000-01-01T00:00:00+05:00",
	} {
		t.Run(name, func(t *testing.T) {
			refused(t, send(t, h, http.MethodPost, "/api/v1/runs", ada, `{"name": "Far", "started_at": "`+at+`"}`))
			body := runRequestFrom(t, run, func(b map[string]any) { b["started_at"] = at })
			refused(t, send(t, h, http.MethodPut, "/api/v1/runs/"+run.ID, ada, body))
		})
	}

	rec := send(t, h, http.MethodGet, "/api/v1/runs", ada, "")
	var list runListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("the run list answers %d %q (%v), want it readable", rec.Code, rec.Body, err)
	}
}

// A finished run can still be changed, and the change does not unfinish it.
func TestEditingAFinishedRunKeepsItFinished(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	run, hang, _ := runWithSteps(t, h, ada)

	finished := decodeRun(t, send(t, h, http.MethodPost, "/api/v1/runs/"+run.ID+"/finish", ada, ""))
	if finished.FinishedAt == nil {
		t.Fatal("the run did not finish")
	}

	body := runRequestFrom(t, finished, func(b map[string]any) { b["went_well"] = "Heel hooks" })
	rec := send(t, h, http.MethodPut, "/api/v1/runs/"+run.ID, ada, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("edit: status %d: %s", rec.Code, rec.Body)
	}
	got := decodeRun(t, rec)
	if got.FinishedAt == nil || !got.FinishedAt.Equal(*finished.FinishedAt) || got.WentWell == nil {
		t.Fatalf("finished %v, went well %v, want the first finish kept beside the edit", got.FinishedAt, got.WentWell)
	}

	// The finish skipped the hang, so it takes no sets until it is reopened.
	rec = send(t, h, http.MethodPut, "/api/v1/runs/"+run.ID+"/steps/"+hang+"/sets", ada, `{"sets": [{"reps": 1}]}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("sets on a step the finish skipped: status %d, want 422: %s", rec.Code, rec.Body)
	}
}

// Another account can neither read nor change a run by its id, nor anything
// nested in it by a step or climb id.
func TestSomeoneElsesRunIsOutOfReach(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	run, hang, climb := runWithSteps(t, h, ada)
	const climbID = "0199c3a0-0000-7000-8000-0000000000c1"
	climbPath := "/api/v1/runs/" + run.ID + "/climbs/" + climbID
	setsPath := "/api/v1/runs/" + run.ID + "/steps/" + hang + "/sets"
	if rec := send(t, h, http.MethodPut, setsPath, ada, `{"sets": [{"reps": 5}]}`); rec.Code != http.StatusOK {
		t.Fatalf("sets: status %d: %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, http.MethodPut, climbPath, ada, `{"step": "`+climb+`", "discipline": "boulder", "setting": "indoor"}`); rec.Code != http.StatusOK {
		t.Fatalf("climb: status %d: %s", rec.Code, rec.Body)
	}

	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/runs/" + run.ID, ""},
		{http.MethodPut, "/api/v1/runs/" + run.ID, runRequestFrom(t, run, func(b map[string]any) { b["name"] = "Bob's now" })},
		{http.MethodPost, "/api/v1/runs/" + run.ID + "/finish", ""},
		{http.MethodPut, setsPath, `{"sets": []}`},
		{http.MethodPut, climbPath, `{"step": "` + climb + `", "discipline": "sport", "setting": "outdoor"}`},
		{http.MethodDelete, climbPath, ""},
		{http.MethodDelete, "/api/v1/runs/" + run.ID, ""},
	} {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			if rec := send(t, h, c.method, c.path, bob, c.body); rec.Code != http.StatusNotFound {
				t.Errorf("status %d, want 404: %s", rec.Code, rec.Body)
			}
		})
	}

	got := decodeRun(t, send(t, h, http.MethodGet, "/api/v1/runs/"+run.ID, ada, ""))
	if got.Name != "Open" || got.FinishedAt != nil || len(got.Sets) != 1 || len(got.Climbs) != 1 || got.Climbs[0].Discipline != "boulder" {
		t.Fatalf("ada's run reads %+v, want it untouched", got)
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
		{http.MethodPut, "/api/v1/runs/" + id + "/steps/" + id + "/sets"},
		{http.MethodPut, "/api/v1/runs/" + id + "/climbs/" + id},
		{http.MethodDelete, "/api/v1/runs/" + id + "/climbs/" + id},
		{http.MethodGet, "/api/v1/climbs"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			if rec := send(t, h, route.method, route.path, "", ""); rec.Code != http.StatusUnauthorized {
				t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestListRunsSaysEachRunsZoneAndWhetherItHasAJournal(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")

	startRunFor(t, h, ada, `{"name": "Plain", "local_date": "2026-03-03"}`)
	noted := startRunFor(t, h, ada, `{"name": "Noted", "local_date": "2026-03-05"}`)
	body := runRequestFrom(t, noted, func(b map[string]any) { b["went_well"] = "Heel hooks" })
	if rec := send(t, h, http.MethodPut, "/api/v1/runs/"+noted.ID, ada, body); rec.Code != http.StatusOK {
		t.Fatalf("put: status %d: %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, http.MethodPost, "/api/v1/runs/"+noted.ID+"/finish", ada, ""); rec.Code != http.StatusOK {
		t.Fatalf("finish: status %d: %s", rec.Code, rec.Body)
	}

	list := func(bearer string) []map[string]any {
		t.Helper()
		rec := send(t, h, http.MethodGet, "/api/v1/runs", bearer, "")
		var got struct {
			Runs []map[string]any `json:"runs"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("list: %v (%s)", err, rec.Body)
		}
		return got.Runs
	}

	runs := list(ada)
	if len(runs) != 2 {
		t.Fatalf("listed %d runs, want 2: %v", len(runs), runs)
	}
	// Newest day first: Noted, then Plain.
	for i, want := range []struct {
		name     string
		journal  bool
		finished bool
	}{{"Noted", true, true}, {"Plain", false, false}} {
		r := runs[i]
		if r["name"] != want.name || r["has_journal"] != want.journal || r["timezone"] != "Europe/Oslo" || (r["finished_at"] != nil) != want.finished {
			t.Fatalf("run %d is %v, want %s in Europe/Oslo, has_journal %v, finished %v", i, r, want.name, want.journal, want.finished)
		}
	}
	if got := list(bob); len(got) != 0 {
		t.Fatalf("Bob sees %v, want none of Ada's runs", got)
	}
}
