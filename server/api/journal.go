package api

import (
	"errors"
	"net/http"
	"time"

	"passion/server/db"
)

// swagger:model journalEntryRequest
type journalEntryRequest struct {
	// The day it is about, written YYYY-MM-DD.
	//
	// required: true
	// example: 2026-10-24
	LocalDate string `json:"local_date"`

	// Free text. Blank text is refused.
	//
	// required: true
	// example: I pull with my arms when I should push with my feet.
	Body string `json:"body"`

	// The session it was written during. Only a new entry takes it; a
	// replace keeps the one it has.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	Run *string `json:"run"`
}

// swagger:model journalEntryResponse
type journalEntryResponse struct {
	// The id the client chose.
	//
	// example: 01a0bf77-d7e8-76ea-96cc-f09cbca175a3
	ID string `json:"id"`

	LocalDate string `json:"local_date"`
	Body      string `json:"body"`

	// The session it was written during. Null once that session is deleted.
	Run *string `json:"run"`

	// That session's name and length, for its link.
	RunName           *string `json:"run_name"`
	RunElapsedSeconds *int    `json:"run_elapsed_seconds"`

	// When it was pinned, or null.
	PinnedAt *time.Time `json:"pinned_at"`
}

// swagger:model sessionJournalResponse
type sessionJournalResponse struct {
	Run       string  `json:"run"`
	Name      string  `json:"name"`
	LocalDate string  `json:"local_date"`
	RPE       *int    `json:"rpe"`
	WentWell  *string `json:"went_well"`
	NextFocus *string `json:"next_focus"`
	Notes     *string `json:"notes"`
}

// swagger:model journalResponse
type journalResponse struct {
	// Newest day first.
	Entries []journalEntryResponse `json:"entries"`

	// Sessions with something written in their journal, newest day first.
	Sessions []sessionJournalResponse `json:"sessions"`
}

// swagger:route GET /api/v1/journal journal listJournal
//
// # List your journal
//
// Your entries, and the sessions you wrote something about.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: journalResponse
//	  401: unauthenticated
func (s *Server) listJournal(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	entries, sessions, err := db.ListJournal(r.Context(), s.pool, who.AccountID)
	if s.failed(w, err) {
		return
	}
	out := journalResponse{
		Entries:  make([]journalEntryResponse, 0, len(entries)),
		Sessions: make([]sessionJournalResponse, 0, len(sessions)),
	}
	for _, e := range entries {
		out.Entries = append(out.Entries, toJournalEntryResponse(e))
	}
	for _, j := range sessions {
		out.Sessions = append(out.Sessions, sessionJournalResponse{
			Run: j.Run, Name: j.Name, LocalDate: j.LocalDate.Format(time.DateOnly),
			RPE: j.RPE, WentWell: j.WentWell, NextFocus: j.NextFocus, Notes: j.Notes,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// swagger:route GET /api/v1/journal/{id} journal readJournalEntry
//
// # Read a journal entry
//
//	Security:
//	  bearer:
//	Responses:
//	  200: journalEntryResponse
//	  401: unauthenticated
//	  404: notFound
func (s *Server) readJournalEntry(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	entry, err := db.GetJournalEntry(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	if s.failed(w, err, db.ErrNoJournalEntry) {
		return
	}
	writeJSON(w, http.StatusOK, toJournalEntryResponse(entry))
}

// swagger:route PUT /api/v1/journal/{id} journal putJournalEntry
//
// # Create or replace a journal entry
//
// Creates the entry under the id you chose, or replaces its day and text, so a
// retry makes one entry. A replace leaves the pin alone.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: journalEntryResponse
//	  401: unauthenticated
//	  404: notFound
//	  422: validationFailed
func (s *Server) putJournalEntry(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	var req journalEntryRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	problems := map[string]string{}
	day, _ := parseDay(req.LocalDate, "local_date", problems)
	f, more := db.JournalFields{LocalDate: day, Body: req.Body, Run: req.Run}.Clean()
	for k, v := range more {
		problems[k] = v
	}
	if len(problems) > 0 {
		writeFieldErrors(w, problems)
		return
	}

	entry, err := db.PutJournalEntry(r.Context(), s.pool, who.AccountID, r.PathValue("id"), f)
	if errors.Is(err, db.ErrNoJournalRun) {
		writeFieldErrors(w, map[string]string{"run": "is not a session of yours"})
		return
	}
	if s.failed(w, err, db.ErrNoJournalEntry) {
		return
	}
	writeJSON(w, http.StatusOK, toJournalEntryResponse(entry))
}

// swagger:route PUT /api/v1/journal/{id}/pin journal pinJournalEntry
//
// # Pin a journal entry
//
// A pinned entry shows on Today and at the start of each session. Pinning it
// again keeps the time it was first pinned.
//
//	Security:
//	  bearer:
//	Responses:
//	  200: journalEntryResponse
//	  401: unauthenticated
//	  404: notFound
func (s *Server) pinJournalEntry(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	s.setPin(w, r, who, true)
}

// swagger:route DELETE /api/v1/journal/{id}/pin journal unpinJournalEntry
//
// # Unpin a journal entry
//
//	Security:
//	  bearer:
//	Responses:
//	  200: journalEntryResponse
//	  401: unauthenticated
//	  404: notFound
func (s *Server) unpinJournalEntry(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	s.setPin(w, r, who, false)
}

func (s *Server) setPin(w http.ResponseWriter, r *http.Request, who db.Authenticated, pinned bool) {
	entry, err := db.PinJournalEntry(r.Context(), s.pool, who.AccountID, r.PathValue("id"), pinned)
	if s.failed(w, err, db.ErrNoJournalEntry) {
		return
	}
	writeJSON(w, http.StatusOK, toJournalEntryResponse(entry))
}

// swagger:route DELETE /api/v1/journal/{id} journal deleteJournalEntry
//
// # Delete a journal entry
//
//	Security:
//	  bearer:
//	Responses:
//	  204: description: Deleted
//	  401: unauthenticated
//	  404: notFound
func (s *Server) deleteJournalEntry(w http.ResponseWriter, r *http.Request, who db.Authenticated) {
	err := db.DeleteJournalEntry(r.Context(), s.pool, who.AccountID, r.PathValue("id"))
	if s.failed(w, err, db.ErrNoJournalEntry) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func toJournalEntryResponse(e db.JournalEntry) journalEntryResponse {
	return journalEntryResponse{
		ID: e.ID, LocalDate: e.LocalDate.Format(time.DateOnly), Body: e.Body,
		Run: e.Run, RunName: e.RunName, RunElapsedSeconds: e.RunElapsedSeconds,
		PinnedAt: e.PinnedAt,
	}
}
