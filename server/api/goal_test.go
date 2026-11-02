package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	goalID   = "0199c3a0-0000-7000-8000-000000000b01"
	customID = "0199c3a0-0000-7000-8000-000000000b02"
)

func decodeGoal(t *testing.T, rec *httptest.ResponseRecorder) goalResponse {
	t.Helper()
	var got goalResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response: %v (%s)", err, rec.Body)
	}
	return got
}

func TestGoals(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	path := "/api/v1/goals/" + goalID
	now := time.Now().UTC()
	today := now.Format(time.DateOnly)
	monday := now.AddDate(0, 0, -(int(now.Weekday())+6)%7).Format(time.DateOnly)
	nextMonday := now.AddDate(0, 0, 7-(int(now.Weekday())+6)%7).Format(time.DateOnly)

	// A retry makes one goal.
	for range 2 {
		rec := send(t, h, http.MethodPut, path, ada, `{"text": " Train fingers twice ", "period": "week"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		if got := decodeGoal(t, rec); got.ID != goalID || got.Text != "Train fingers twice" || got.Starts != today || got.Ticks == nil {
			t.Fatalf("goal %+v", got)
		}
	}

	for name, c := range map[string]struct {
		method, path, who, body string
		fields                  []string
		status                  int
	}{
		"no text and a bad period": {http.MethodPut, path, ada, `{"text": "", "period": "day"}`, []string{"text", "period"}, http.StatusUnprocessableEntity},
		"a new period":             {http.MethodPut, path, ada, `{"text": "Train", "period": "month"}`, []string{"period"}, http.StatusUnprocessableEntity},
		"someone else's id":        {http.MethodPut, path, bob, `{"text": "Mine", "period": "week"}`, nil, http.StatusNotFound},
		"a tick on no date":        {http.MethodPut, path + "/ticks/soon", ada, "", []string{"period_start"}, http.StatusUnprocessableEntity},
		"a tick next week":         {http.MethodPut, path + "/ticks/" + nextMonday, ada, "", []string{"period_start"}, http.StatusUnprocessableEntity},
		"a tick on someone else's": {http.MethodPut, path + "/ticks/" + monday, bob, "", nil, http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			rec := send(t, h, c.method, c.path, c.who, c.body)
			if rec.Code != c.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, c.status, rec.Body)
			}
			for _, field := range c.fields {
				if decodeBody(t, rec).Error.Fields[field] == "" {
					t.Fatalf("fields %s, want %s named", rec.Body, field)
				}
			}
		})
	}

	got := decodeGoal(t, send(t, h, http.MethodPut, path+"/ticks/"+monday, ada, ""))
	if len(got.Ticks) != 1 || got.Ticks[0].PeriodStart != monday || got.Ticks[0].Text != "Train fingers twice" {
		t.Fatalf("goal %+v, want this week ticked", got)
	}
	if got := decodeGoal(t, send(t, h, http.MethodDelete, path+"/ticks/"+monday, ada, "")); len(got.Ticks) != 0 {
		t.Fatalf("goal %+v, want the tick gone", got)
	}

	if rec := send(t, h, http.MethodDelete, path, bob, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want someone else's goal refused", rec.Code)
	}
	if rec := send(t, h, http.MethodDelete, path, ada, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var list goalListResponse
	if err := json.Unmarshal(send(t, h, http.MethodGet, "/api/v1/goals", ada, "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Goals) != 1 || list.Goals[0].Removed == nil || *list.Goals[0].Removed != today {
		t.Fatalf("goals %+v, want it kept, removed today", list)
	}
	if rec := send(t, h, http.MethodPut, path+"/ticks/"+monday, ada, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want a removed goal's ticks closed", rec.Code)
	}
	if err := json.Unmarshal(send(t, h, http.MethodGet, "/api/v1/goals", bob, "").Body.Bytes(), &list); err != nil || len(list.Goals) != 0 {
		t.Fatalf("goals %+v, %v, want none of ada's for bob", list, err)
	}

	custom := "/api/v1/goals/" + customID
	span := `"span": {"id": "0199c3a0-0000-7000-8000-000000000c01", "name": "Font trip", "starts": "2026-01-10", "ends": "2026-01-20"}`
	rec := send(t, h, http.MethodPut, custom, ada, `{"text": "Send 7A", "period": "custom", `+span+`}`)
	if got := decodeGoal(t, rec); rec.Code != http.StatusOK || got.Span == nil || got.Span.Ends != "2026-01-20" || got.Span.Name != "Font trip" {
		t.Fatalf("status %d, goal %+v", rec.Code, got)
	}
	if got := decodeGoal(t, send(t, h, http.MethodPut, custom+"/ticks/2026-01-10", ada, "")); len(got.Ticks) != 1 {
		t.Fatalf("goal %+v, want its one period ticked", got)
	}
	moved := strings.Replace(span, `"starts": "2026-01-10"`, `"starts": "2026-01-12"`, 1)
	if got := decodeGoal(t, send(t, h, http.MethodPut, custom, ada, `{"text": "Send 7A", "period": "custom", `+moved+`}`)); len(got.Ticks) != 1 || got.Ticks[0].PeriodStart != "2026-01-12" {
		t.Fatalf("goal %+v, want the tick moved with the first day", got)
	}
	later := "/api/v1/goals/0199c3a0-0000-7000-8000-000000000b03"
	send(t, h, http.MethodPut, later, ada, `{"text": "Later", "period": "custom", "span": {"id": "0199c3a0-0000-7000-8000-000000000c03", "starts": "2099-01-01", "ends": "2099-01-02"}}`)
	if rec := send(t, h, http.MethodPut, later+"/ticks/2099-01-01", ada, ""); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want days still to come refused", rec.Code)
	}
	for body, want := range map[string][2]string{
		`{"text": "Send 7A", "period": "custom"}`: {"span", "is required"},
		`{"text": "Send 7A", "period": "custom", "span": {"id": "", "starts": "2026-01-10", "ends": "2026-01-20"}}`:                                      {"span.id", "is required"},
		`{"text": "Send 7A", "period": "custom", "span": {"id": "0199c3a0-0000-7000-8000-000000000c01", "starts": "2026-01-10", "ends": "soon"}}`:       {"span.ends", "must be a date, written 2026-03-07"},
		`{"text": "Send 7A", "period": "custom", "span": {"id": "0199c3a0-0000-7000-8000-000000000c02", "starts": "2026-01-10", "ends": "2026-01-20"}}`: {"span.id", "cannot change once the goal is made"},
		`{"text": "Send 7A", "period": "week"}`: {"period", "cannot change once the goal is made"},
	} {
		rec := send(t, h, http.MethodPut, custom, ada, body)
		if rec.Code != http.StatusUnprocessableEntity || decodeBody(t, rec).Error.Fields[want[0]] != want[1] {
			t.Fatalf("status %d: %s, want %s: %s", rec.Code, rec.Body, want[0], want[1])
		}
	}
}
