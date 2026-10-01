package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const cycleID = "0199c3a0-0000-7000-8000-0000000000e1"

func cycleBody(template string) string {
	return `{"name": "Spring fingers", "starts": "2100-01-05", "ends": "2100-01-18", "block_days": 7,
		"days": [{"day": 1, "template": "` + template + `"}, {"day": 3, "template": "` + template + `"}]}`
}

func TestPutCycle(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	power := insertShippedSessionTemplate(t, pool, "Power")
	path := "/api/v1/cycles/" + cycleID

	// A retry makes one cycle.
	for range 2 {
		rec := send(t, h, http.MethodPut, path, ada, cycleBody(power))
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var got cyclePutResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != cycleID || got.Starts != "2100-01-05" || len(got.Days) != 2 || got.LeftOut == nil {
			t.Fatalf("cycle %+v, want Spring fingers under the id chosen, with an empty left_out", got)
		}
	}
	// Goals, entries and notes come back as written, blank lines dropped.
	rec := send(t, h, http.MethodPut, path, ada, `{"name": "Spring fingers", "starts": "2100-01-05", "ends": "2100-01-18", "block_days": 7,
		"goals": [{"text": "Flash 7a", "done": true}, {"text": " "}], "before": ["Max hang +18 kg"], "notes": "Strict pull-ups"}`)
	var got cycleResponse
	if err := json.Unmarshal(send(t, h, http.MethodGet, path, ada, "").Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("status %d, %v", rec.Code, err)
	}
	if len(got.Goals) != 1 || !got.Goals[0].Done || len(got.Before) != 1 || got.After == nil || *got.Notes != "Strict pull-ups" {
		t.Fatalf("cycle %+v, want the goal, the entry and the notes", got)
	}

	var list cycleListResponse
	if err := json.Unmarshal(send(t, h, http.MethodGet, "/api/v1/cycles", ada, "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Cycles) != 1 {
		t.Fatalf("%d cycles, want one after a retry", len(list.Cycles))
	}

	for name, c := range map[string]struct {
		who, path, body, field string
		status                 int
	}{
		"a bad date":              {ada, path, `{"name": "C", "starts": "soon", "ends": "2100-01-18", "block_days": 7}`, "starts", http.StatusUnprocessableEntity},
		"a day past the block":    {ada, path, `{"name": "C", "starts": "2100-01-05", "ends": "2100-01-18", "block_days": 7, "days": [{"day": 8, "template": "` + power + `"}]}`, "days[0].day", http.StatusUnprocessableEntity},
		"a goal too long":         {ada, path, `{"name": "C", "starts": "2100-01-05", "ends": "2100-01-18", "block_days": 7, "goals": [{"text": "` + strings.Repeat("a", 201) + `"}]}`, "goals[0]", http.StatusUnprocessableEntity},
		"a template you lack":     {ada, path, `{"name": "C", "starts": "2100-01-05", "ends": "2100-01-18", "block_days": 7, "days": [{"day": 1, "template": "0199c3a0-0000-7000-8000-0000000000ff"}]}`, "days[0].template", http.StatusUnprocessableEntity},
		"someone else's cycle id": {bob, path, cycleBody(power), "", http.StatusNotFound},
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

	if rec := send(t, h, http.MethodGet, path, bob, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("someone else read it: status %d", rec.Code)
	}
	if rec := send(t, h, http.MethodDelete, path, ada, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d: %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, http.MethodGet, path, ada, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("read after delete: status %d, want 404", rec.Code)
	}
}

func TestCyclesNeedSignIn(t *testing.T) {
	h := newTestServer(t)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/cycles"},
		{http.MethodGet, "/api/v1/cycles/" + cycleID},
		{http.MethodPut, "/api/v1/cycles/" + cycleID},
		{http.MethodDelete, "/api/v1/cycles/" + cycleID},
	} {
		if rec := send(t, h, route.method, route.path, "", ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: status %d, want 401", route.method, route.path, rec.Code)
		}
	}
}
