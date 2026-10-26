package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

const repeatID = "0199c3a0-0000-7000-8000-000000000b01"

// osloToday is the test accounts' date, which the server builds repeats from.
func osloToday(t *testing.T) time.Time {
	t.Helper()
	oslo, err := time.LoadLocation("Europe/Oslo")
	if err != nil {
		t.Fatal(err)
	}
	return time.Now().In(oslo)
}

func TestPutRepeat(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	template := createSessionTemplate(t, h, ada, `{"name": "Power"}`)
	path := "/api/v1/repeats/" + repeatID
	today := osloToday(t).Format(time.DateOnly)
	weekly := `{"template": "` + template.ID + `", "starts": "` + today + `", "every": 1, "unit": "week", "weekdays": [1, 2, 3, 4, 5, 6, 7]}`

	// A retry makes one repeat.
	for range 2 {
		rec := send(t, h, http.MethodPut, path, ada, weekly)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var got repeatPutResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != repeatID || got.Ends != nil || len(got.Weekdays) != 7 || got.LeftOut == nil {
			t.Fatalf("repeat %+v", got)
		}
	}

	var days scheduledDayListResponse
	if err := json.Unmarshal(send(t, h, http.MethodGet, "/api/v1/scheduled-sessions?from="+today+"&to="+today, ada, "").Body.Bytes(), &days); err != nil {
		t.Fatal(err)
	}
	if len(days.Days) != 1 || days.Days[0].Repeat == nil || *days.Days[0].Repeat != repeatID {
		t.Fatalf("days %+v, want today's row marked with its repeat", days)
	}
	var list repeatListResponse
	if err := json.Unmarshal(send(t, h, http.MethodGet, "/api/v1/repeats", ada, "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Repeats) != 1 {
		t.Fatalf("repeats %+v, want one", list)
	}

	for name, c := range map[string]struct {
		who, body string
		fields    []string
		status    int
	}{
		"no start and a bad unit": {ada, `{"template": "` + template.ID + `", "every": 1, "unit": "month"}`, []string{"starts", "unit"}, http.StatusUnprocessableEntity},
		"an unknown template":     {ada, `{"template": "` + repeatID + `", "starts": "` + today + `", "every": 1, "unit": "day"}`, []string{"template"}, http.StatusUnprocessableEntity},
		"someone else's id":       {bob, `{"template": "` + createSessionTemplate(t, h, bob, `{"name": "Bob's"}`).ID + `", "starts": "` + today + `", "every": 1, "unit": "day"}`, nil, http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			rec := send(t, h, http.MethodPut, path, c.who, c.body)
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
}

func TestEndAndDeleteRepeat(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	template := createSessionTemplate(t, h, ada, `{"name": "Power"}`)
	path := "/api/v1/repeats/" + repeatID
	today := osloToday(t)
	body := `{"template": "` + template.ID + `", "starts": "` + today.Format(time.DateOnly) + `", "every": 1, "unit": "day"}`
	if rec := send(t, h, http.MethodPut, path, ada, body); rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	later := `{"from": "` + today.AddDate(0, 0, 3).Format(time.DateOnly) + `"}`
	if rec := send(t, h, http.MethodPost, path+"/end", bob, later); rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want someone else's repeat hidden", rec.Code)
	}
	if rec := send(t, h, http.MethodPost, path+"/end", ada, `{}`); rec.Code != http.StatusUnprocessableEntity || decodeBody(t, rec).Error.Fields["from"] == "" {
		t.Fatalf("status %d: %s, want from named", rec.Code, rec.Body)
	}
	if rec := send(t, h, http.MethodPost, path+"/end", ada, later); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var list repeatListResponse
	if err := json.Unmarshal(send(t, h, http.MethodGet, "/api/v1/repeats", ada, "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Repeats) != 1 || list.Repeats[0].Ends == nil || *list.Repeats[0].Ends != today.AddDate(0, 0, 2).Format(time.DateOnly) {
		t.Fatalf("repeats %+v, want it to end the day before", list)
	}

	if rec := send(t, h, http.MethodDelete, path, bob, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want someone else's repeat hidden", rec.Code)
	}
	if rec := send(t, h, http.MethodDelete, path, ada, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, http.MethodDelete, path, ada, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want the repeat gone", rec.Code)
	}
}

func TestRepeatsNeedSignIn(t *testing.T) {
	h := newTestServer(t)
	path := "/api/v1/repeats/" + repeatID
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/repeats"},
		{http.MethodPut, path},
		{http.MethodPost, path + "/end"},
		{http.MethodDelete, path},
	} {
		if rec := send(t, h, route.method, route.path, "", ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: status %d, want 401", route.method, route.path, rec.Code)
		}
	}
}
