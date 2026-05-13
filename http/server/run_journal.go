package web

import (
	"net/http"
	"strings"

	"gorm.io/gorm"

	"passion/db"
	"passion/pages"
)
// handleRunJournal serves GET|POST /runs/{runID}/journal.
func (s *Server) handleRunJournal(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)

	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	// Validate ownership of the run.
	var run db.SessionRun
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, runID).First(&run).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		s.serverError(w, r, err)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.serveJournalForm(w, r, ownerID, runID, false)
	case http.MethodPost:
		s.saveJournal(w, r, ownerID, runID)
	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) serveJournalForm(w http.ResponseWriter, r *http.Request, ownerID, runID uint, saved bool) {
	j, err := db.GetSessionJournalByRunID(s.store.DB, ownerID, runID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	params := pages.JournalFormParams{
		RunID: runID,
		Saved: saved,
	}
	if j != nil {
		params.SleepScore = j.SleepScore
		params.Energy = j.Energy
		params.RPE = j.RPE
		params.Focus = j.Focus
		params.Location = j.Location
		params.WentWell = j.WentWell
		params.NextFocus = j.NextFocus
	}

	s.pages.RenderJournalForm(w, params)
}

func (s *Server) saveJournal(w http.ResponseWriter, r *http.Request, ownerID, runID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	// Look up existing journal (upsert).
	existing, err := db.GetSessionJournalByRunID(s.store.DB, ownerID, runID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	j := db.SessionJournal{
		OwnerID:    ownerID,
		RunID:      &runID,
		SleepScore: formInt(r, "sleep_score"),
		Energy:     formInt(r, "energy"),
		RPE:        formInt(r, "rpe"),
		Focus:      strings.TrimSpace(r.FormValue("focus")),
		Location:   strings.TrimSpace(r.FormValue("location")),
		WentWell:   strings.TrimSpace(r.FormValue("went_well")),
		NextFocus:  strings.TrimSpace(r.FormValue("next_focus")),
	}
	if existing != nil {
		j.Model = existing.Model // preserve ID + timestamps for update
	}

	if err := db.UpsertSessionJournal(s.store.DB, &j); err != nil {
		s.serverError(w, r, err)
		return
	}

	// Return the saved (read-only) fragment so HTMX can swap it in.
	s.serveJournalForm(w, r, ownerID, runID, true)
}
