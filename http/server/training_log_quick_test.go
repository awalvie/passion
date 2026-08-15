package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"passion/db"
)

// postTrainingLogQuick builds and runs a POST /training-log/quick request with the
// given form fields and auth injected.
func postTrainingLogQuick(t *testing.T, srv *Server, ownerID uint, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/training-log/quick", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := context.WithValue(req.Context(), authUserIDKey, ownerID)
	rr := httptest.NewRecorder()
	srv.handleTrainingLogQuick(rr, req.WithContext(ctx))
	return rr
}

// TestHandleTrainingLogQuick_CreatesStandaloneJournal guards the core create path: a
// valid POST must persist a SessionJournal with RunID nil (standalone), the submitted
// title/notes, and redirect to the new entry's view page.
func TestHandleTrainingLogQuick_CreatesStandaloneJournal(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "quick-create.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	srv := &Server{store: store}

	form := url.Values{
		"title": {"Outdoor bouldering"},
		"date":  {"2026-01-15"},
		"notes": {"Sent the slab project."},
	}
	rr := postTrainingLogQuick(t, srv, ownerID, form)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}

	journals, err := db.ListSessionJournals(store.DB, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(journals) != 1 {
		t.Fatalf("len(journals) = %d, want 1", len(journals))
	}
	j := journals[0]

	if j.RunID != nil {
		t.Errorf("RunID = %v, want nil (standalone entry)", *j.RunID)
	}
	if j.Title != "Outdoor bouldering" {
		t.Errorf("Title = %q, want %q", j.Title, "Outdoor bouldering")
	}
	if j.WentWell != "Sent the slab project." {
		t.Errorf("WentWell = %q, want the submitted notes", j.WentWell)
	}
	if j.Date.Format("2006-01-02") != "2026-01-15" {
		t.Errorf("Date = %s, want 2026-01-15", j.Date.Format("2006-01-02"))
	}

	wantLoc := fmt.Sprintf("/training-log/%d", j.ID)
	if got := rr.Header().Get("Location"); got != wantLoc {
		t.Errorf("Location = %q, want %q", got, wantLoc)
	}
}

// TestHandleTrainingLogQuick_RejectsEmptyNotes guards the validation branch: notes
// that are empty (or whitespace-only, since the handler trims before checking) must
// re-render the form with FormErr set and must NOT create a journal row.
func TestHandleTrainingLogQuick_RejectsEmptyNotes(t *testing.T) {
	tests := []struct {
		name  string
		notes string
	}{
		{"empty string", ""},
		{"whitespace only (spaces)", "   "},
		{"whitespace only (tabs and newlines)", "\t\n  \n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withRepoRoot(t)
			store, err := db.NewSqlite(filepath.Join(t.TempDir(), "quick-empty.db"))
			if err != nil {
				t.Fatal(err)
			}
			const ownerID uint = 1
			srv, err := NewServer(store, "secret", 24, false, nil)
			if err != nil {
				t.Fatal(err)
			}

			form := url.Values{
				"title": {"Some title"},
				"date":  {"2026-01-15"},
				"notes": {tt.notes},
			}
			rr := postTrainingLogQuick(t, srv, ownerID, form)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d (re-render form); body=%q", rr.Code, http.StatusOK, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), "Write a note before saving.") {
				t.Errorf("response does not contain the validation error message: %.500q", rr.Body.String())
			}

			journals, err := db.ListSessionJournals(store.DB, ownerID)
			if err != nil {
				t.Fatal(err)
			}
			if len(journals) != 0 {
				t.Fatalf("len(journals) = %d, want 0 (empty notes must not persist): %+v", len(journals), journals)
			}
		})
	}
}

// TestHandleTrainingLogQuick_InvalidDateFallsBackToNow guards the date-parsing
// fallback: a missing or malformed date must not error out the request — it should
// fall back to the current time and still save the note.
func TestHandleTrainingLogQuick_InvalidDateFallsBackToNow(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "quick-baddate.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	srv := &Server{store: store}

	form := url.Values{
		"notes": {"Still worth logging even without a valid date."},
		"date":  {"not-a-date"},
	}
	rr := postTrainingLogQuick(t, srv, ownerID, form)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}

	journals, err := db.ListSessionJournals(store.DB, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(journals) != 1 {
		t.Fatalf("len(journals) = %d, want 1", len(journals))
	}
	if journals[0].Date.IsZero() {
		t.Errorf("Date is zero, want fallback to time.Now()")
	}
}

// TestHandleTrainingLogQuick_OwnerScoped guards against cross-owner creation: the
// journal must always be created under the authenticated request's owner ID, never
// an owner supplied via form data or any other client-controlled input.
func TestHandleTrainingLogQuick_OwnerScoped(t *testing.T) {
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "quick-scope.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerA, ownerB uint = 1, 2
	srv := &Server{store: store}

	form := url.Values{
		"notes":    {"Owner A's note"},
		"date":     {"2026-01-15"},
		"owner_id": {"999"}, // attempted spoof; handler must ignore this
	}
	rr := postTrainingLogQuick(t, srv, ownerA, form)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}

	journalsA, err := db.ListSessionJournals(store.DB, ownerA)
	if err != nil {
		t.Fatal(err)
	}
	if len(journalsA) != 1 {
		t.Fatalf("owner A: len(journals) = %d, want 1", len(journalsA))
	}

	journalsSpoofed, err := db.ListSessionJournals(store.DB, 999)
	if err != nil {
		t.Fatal(err)
	}
	if len(journalsSpoofed) != 0 {
		t.Fatalf("journal leaked to spoofed owner_id=999: %+v", journalsSpoofed)
	}

	journalsB, err := db.ListSessionJournals(store.DB, ownerB)
	if err != nil {
		t.Fatal(err)
	}
	if len(journalsB) != 0 {
		t.Fatalf("owner B should see no journals: %+v", journalsB)
	}
}

// TestHandleTrainingLogQuick_StandaloneEntryEditableViaExistingRoutes guards the
// integration between the new quick-note create path and the pre-existing
// /training-log/{id}/edit and /training-log/{id}/delete handlers, which already
// branch on RunID==nil for standalone entries. A quick note must be a genuine
// standalone journal that those routes can load, update, and delete.
func TestHandleTrainingLogQuick_StandaloneEntryEditableViaExistingRoutes(t *testing.T) {
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), "quick-edit-delete.db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	srv, err := NewServer(store, "secret", 24, false, nil)
	if err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"title": {"First cut"},
		"date":  {"2026-01-15"},
		"notes": {"Original note body."},
	}
	rr := postTrainingLogQuick(t, srv, ownerID, form)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("create: status = %d, want %d; body=%q", rr.Code, http.StatusSeeOther, rr.Body.String())
	}

	journals, err := db.ListSessionJournals(store.DB, ownerID)
	if err != nil || len(journals) != 1 {
		t.Fatalf("expected exactly one journal after create, got %d (err=%v)", len(journals), err)
	}
	journalID := journals[0].ID

	// GET the edit page for the standalone entry — must load without error via
	// GetSessionJournalByID and hit the (j.RunID == nil) branch of handleTrainingLogEdit.
	editReq := newJournalRequest(t, http.MethodGet, journalID, ownerID, nil)
	editRR := httptest.NewRecorder()
	srv.handleTrainingLogEdit(editRR, editReq)
	if editRR.Code != http.StatusOK {
		t.Fatalf("GET edit: status = %d, want %d; body=%q", editRR.Code, http.StatusOK, editRR.Body.String())
	}
	if !strings.Contains(editRR.Body.String(), "Original note body.") {
		t.Errorf("edit page does not contain the existing note body: %.500q", editRR.Body.String())
	}

	// POST an update through the same edit handler (updateJournal) — must persist
	// title/date since RunID == nil takes the standalone-fields branch.
	updateForm := url.Values{
		"title":     {"Updated title"},
		"date":      {"2026-01-16"},
		"went_well": {"Updated note body."},
	}
	updateReq := newJournalRequest(t, http.MethodPost, journalID, ownerID, updateForm)
	updateRR := httptest.NewRecorder()
	srv.handleTrainingLogEdit(updateRR, updateReq)
	if updateRR.Code != http.StatusSeeOther && updateRR.Code != http.StatusOK {
		t.Fatalf("POST edit: unexpected status %d; body=%q", updateRR.Code, updateRR.Body.String())
	}

	updated, err := db.GetSessionJournalByID(store.DB, ownerID, journalID)
	if err != nil || updated == nil {
		t.Fatalf("journal missing after update: %v, err=%v", updated, err)
	}
	if updated.Title != "Updated title" {
		t.Errorf("Title = %q, want %q", updated.Title, "Updated title")
	}
	if updated.WentWell != "Updated note body." {
		t.Errorf("WentWell = %q, want %q", updated.WentWell, "Updated note body.")
	}
	if updated.RunID != nil {
		t.Errorf("RunID = %v, want nil (must remain standalone after edit)", *updated.RunID)
	}

	// Delete through the existing delete handler.
	deleteReq := newJournalRequest(t, http.MethodPost, journalID, ownerID, nil)
	deleteRR := httptest.NewRecorder()
	srv.handleTrainingLogDelete(deleteRR, deleteReq)
	if deleteRR.Code != http.StatusSeeOther {
		t.Fatalf("delete: status = %d, want %d", deleteRR.Code, http.StatusSeeOther)
	}

	if gone, _ := db.GetSessionJournalByID(store.DB, ownerID, journalID); gone != nil {
		t.Errorf("journal still exists after delete: %+v", gone)
	}
}

// newJournalRequest builds a request against /training-log/{journalID}/edit with the
// journalID chi route param and auth injected. Pass a non-nil form for a POST body.
func newJournalRequest(t *testing.T, method string, journalID uint, ownerID uint, form url.Values) *http.Request {
	t.Helper()
	path := fmt.Sprintf("/training-log/%d/edit", journalID)
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("journalID", fmt.Sprintf("%d", journalID))
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, authUserIDKey, ownerID)
	return req.WithContext(ctx)
}
