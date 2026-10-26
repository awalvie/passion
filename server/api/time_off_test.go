package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const timeOffID = "0199c3a0-0000-7000-8000-000000000a01"

func decodeTimeOff(t *testing.T, rec *httptest.ResponseRecorder) timeOffResponse {
	t.Helper()
	var got timeOffResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response: %v (%s)", err, rec.Body)
	}
	return got
}

func TestPutTimeOff(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	path := "/api/v1/time-off/" + timeOffID

	// A retry makes one time off, and pauses when left out.
	for range 2 {
		rec := send(t, h, http.MethodPut, path, ada, `{"kind": "trip", "name": " Fontainebleau ", "starts": "2026-11-07", "ends": "2026-11-15"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		got := decodeTimeOff(t, rec)
		if got.ID != timeOffID || got.Name != "Fontainebleau" || got.Ends == nil || *got.Ends != "2026-11-15" || !got.Pauses {
			t.Fatalf("time off %+v", got)
		}
	}

	var list timeOffListResponse
	if err := json.Unmarshal(send(t, h, http.MethodGet, "/api/v1/time-off", ada, "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.TimeOff) != 1 {
		t.Fatalf("time off %+v, want one", list)
	}

	for name, c := range map[string]struct {
		path, who, body string
		fields          []string
		status          int
	}{
		"no start":                 {path, ada, `{"kind": "trip", "name": "Away"}`, []string{"starts"}, http.StatusUnprocessableEntity},
		"a bad kind and a bad end": {path, ada, `{"kind": "holiday", "name": "Away", "starts": "2026-11-07", "ends": "soon"}`, []string{"kind", "ends"}, http.StatusUnprocessableEntity},
		"someone else's id":        {path, bob, `{"kind": "trip", "name": "Mine", "starts": "2026-11-07"}`, nil, http.StatusNotFound},
		"an id that is not a uuid": {"/api/v1/time-off/nope", ada, `{"kind": "trip", "name": "Mine", "starts": "2026-11-07"}`, nil, http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			rec := send(t, h, http.MethodPut, c.path, c.who, c.body)
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

	if got := decodeTimeOff(t, send(t, h, http.MethodGet, path, ada, "")); got.Name != "Fontainebleau" || got.Kind != "trip" {
		t.Fatalf("time off %+v, want it untouched by the refused writes", got)
	}

	// A replace can leave the end open and stop pausing.
	got := decodeTimeOff(t, send(t, h, http.MethodPut, path, ada, `{"kind": "injury", "name": "Finger pulley", "starts": "2026-11-07", "pauses": false}`))
	if got.Kind != "injury" || got.Ends != nil || got.Pauses {
		t.Fatalf("time off %+v, want the injury with no end", got)
	}
}

func TestTimeOffIsOwnerScoped(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	path := "/api/v1/time-off/" + timeOffID
	if rec := send(t, h, http.MethodPut, path, ada, `{"kind": "trip", "name": "Fontainebleau", "starts": "2026-11-07"}`); rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		if rec := send(t, h, method, path, bob, ""); rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status %d, want someone else's time off hidden", method, rec.Code)
		}
		for _, missing := range []string{"/api/v1/time-off/nope", "/api/v1/time-off/0199c3a0-0000-7000-8000-000000000a09"} {
			if rec := send(t, h, method, missing, ada, ""); rec.Code != http.StatusNotFound {
				t.Fatalf("%s %s: status %d, want 404", method, missing, rec.Code)
			}
		}
	}

	if rec := send(t, h, http.MethodDelete, path, ada, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, http.MethodGet, path, ada, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want the time off gone", rec.Code)
	}
}

func TestTimeOffNeedsSignIn(t *testing.T) {
	h := newTestServer(t)
	path := "/api/v1/time-off/" + timeOffID
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/time-off"},
		{http.MethodGet, path},
		{http.MethodPut, path},
		{http.MethodDelete, path},
	} {
		if rec := send(t, h, route.method, route.path, "", ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: status %d, want 401", route.method, route.path, rec.Code)
		}
	}
}
