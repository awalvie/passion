package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

const centreID = "0199c3a0-0000-7000-8000-0000000000c1"

func TestPutCentre(t *testing.T) {
	h, pool := newTestServerWithPool(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	power := insertShippedSessionTemplate(t, pool, "Power")
	path := "/api/v1/centres/" + centreID

	// A retry makes one centre.
	for range 2 {
		rec := send(t, h, http.MethodPut, path, ada, `{"name": " The Arch ", "sessions": ["`+power+`", "`+power+`"]}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var got centreResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != centreID || got.Name != "The Arch" || len(got.Sessions) != 1 || got.Sessions[0] != power {
			t.Fatalf("centre %+v, want The Arch with Power once", got)
		}
	}

	var list centreListResponse
	if err := json.Unmarshal(send(t, h, http.MethodGet, "/api/v1/centres", ada, "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Centres) != 1 {
		t.Fatalf("%d centres, want one after a retry", len(list.Centres))
	}

	for name, c := range map[string]struct {
		who, body, field string
		status           int
	}{
		"no name":                  {ada, `{"name": " "}`, "name", http.StatusUnprocessableEntity},
		"a template you lack":      {ada, `{"name": "C", "sessions": ["0199c3a0-0000-7000-8000-0000000000ff"]}`, "sessions[0]", http.StatusUnprocessableEntity},
		"someone else's centre id": {bob, `{"name": "C"}`, "", http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			rec := send(t, h, http.MethodPut, path, c.who, c.body)
			if rec.Code != c.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, c.status, rec.Body)
			}
			if c.field != "" && decodeBody(t, rec).Error.Fields[c.field] == "" {
				t.Fatalf("fields %s, want %s named", rec.Body, c.field)
			}
		})
	}

	if rec := send(t, h, http.MethodDelete, path, bob, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("someone else deleted it: status %d", rec.Code)
	}
	if rec := send(t, h, http.MethodDelete, path, ada, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d: %s", rec.Code, rec.Body)
	}
}

func TestCentresNeedSignIn(t *testing.T) {
	h := newTestServer(t)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/centres"},
		{http.MethodPut, "/api/v1/centres/" + centreID},
		{http.MethodDelete, "/api/v1/centres/" + centreID},
	} {
		if rec := send(t, h, route.method, route.path, "", ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: status %d, want 401", route.method, route.path, rec.Code)
		}
	}
}
