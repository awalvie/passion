package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const journalEntryID = "0199c3a0-0000-7000-8000-0000000000d1"

func decodeJournalEntry(t *testing.T, rec *httptest.ResponseRecorder) journalEntryResponse {
	t.Helper()
	var got journalEntryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response: %v (%s)", err, rec.Body)
	}
	return got
}

func TestPutJournalEntry(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	run := startRunFor(t, h, ada, `{"name": "Boulder"}`)
	bobsRun := startRunFor(t, h, bob, `{"name": "Bob's"}`)
	path := "/api/v1/journal/" + journalEntryID

	// A retry makes one entry.
	for range 2 {
		rec := send(t, h, http.MethodPut, path, ada, `{"local_date": "2026-10-24", "body": " Hips in. ", "run": "`+run.ID+`"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		got := decodeJournalEntry(t, rec)
		if got.ID != journalEntryID || got.Body != "Hips in." || got.LocalDate != "2026-10-24" || got.Run == nil || *got.Run != run.ID {
			t.Fatalf("entry %+v", got)
		}
	}

	var list journalResponse
	if err := json.Unmarshal(send(t, h, http.MethodGet, "/api/v1/journal", ada, "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Entries) != 1 || list.Sessions == nil {
		t.Fatalf("journal %+v, want one entry and a sessions list", list)
	}

	newPath := "/api/v1/journal/0199c3a0-0000-7000-8000-0000000000d2"
	for name, c := range map[string]struct {
		path, who, body, field string
		status                 int
	}{
		"no text":                 {path, ada, `{"local_date": "2026-10-24", "body": " "}`, "body", http.StatusUnprocessableEntity},
		"no date":                 {path, ada, `{"body": "Hips in."}`, "local_date", http.StatusUnprocessableEntity},
		"a NUL in the text":       {path, ada, `{"local_date": "2026-10-24", "body": "a\u0000b"}`, "body", http.StatusUnprocessableEntity},
		"a run that is not uuid":  {newPath, ada, `{"local_date": "2026-10-24", "body": "Hips in.", "run": "nope"}`, "run", http.StatusUnprocessableEntity},
		"someone else's run":      {newPath, ada, `{"local_date": "2026-10-24", "body": "Hips in.", "run": "` + bobsRun.ID + `"}`, "run", http.StatusUnprocessableEntity},
		"someone else's entry id": {path, bob, `{"local_date": "2026-10-24", "body": "Mine"}`, "", http.StatusNotFound},
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

	if got := decodeJournalEntry(t, send(t, h, http.MethodGet, path, ada, "")); got.Body != "Hips in." {
		t.Fatalf("entry %+v, want it untouched by the refused writes", got)
	}
}

func TestJournalReadsAreOwnerScoped(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	path := "/api/v1/journal/" + journalEntryID
	send(t, h, http.MethodPut, path, ada, `{"local_date": "2026-10-24", "body": "Hips in."}`)

	if rec := send(t, h, http.MethodGet, path, bob, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("someone else read it: status %d", rec.Code)
	}
	var list journalResponse
	if err := json.Unmarshal(send(t, h, http.MethodGet, "/api/v1/journal", bob, "").Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Entries) != 0 {
		t.Fatalf("journal %+v, want none of ada's entries", list)
	}
}

func TestPinJournalEntry(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	path := "/api/v1/journal/" + journalEntryID
	send(t, h, http.MethodPut, path, ada, `{"local_date": "2026-10-24", "body": "Hips in."}`)

	if got := decodeJournalEntry(t, send(t, h, http.MethodPut, path+"/pin", ada, "")); got.PinnedAt == nil {
		t.Fatalf("entry %+v, want it pinned", got)
	}
	if got := decodeJournalEntry(t, send(t, h, http.MethodDelete, path+"/pin", ada, "")); got.PinnedAt != nil {
		t.Fatalf("entry %+v, want it unpinned", got)
	}
	if rec := send(t, h, http.MethodPut, path+"/pin", bob, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("someone else pinned it: status %d", rec.Code)
	}
}

func TestDeleteJournalEntry(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")
	path := "/api/v1/journal/" + journalEntryID
	send(t, h, http.MethodPut, path, ada, `{"local_date": "2026-10-24", "body": "Hips in."}`)

	if rec := send(t, h, http.MethodDelete, path, bob, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("someone else deleted it: status %d", rec.Code)
	}
	if rec := send(t, h, http.MethodDelete, path, ada, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d: %s", rec.Code, rec.Body)
	}
	if rec := send(t, h, http.MethodGet, path, ada, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("read after delete: status %d", rec.Code)
	}
}

func TestJournalNeedsSignIn(t *testing.T) {
	h := newTestServer(t)
	path := "/api/v1/journal/" + journalEntryID
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/journal"},
		{http.MethodGet, path},
		{http.MethodPut, path},
		{http.MethodDelete, path},
		{http.MethodPut, path + "/pin"},
		{http.MethodDelete, path + "/pin"},
	} {
		if rec := send(t, h, route.method, route.path, "", ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: status %d, want 401", route.method, route.path, rec.Code)
		}
	}
}
