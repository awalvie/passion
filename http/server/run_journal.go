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
		params.SessionNotes = j.SessionNotes
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
		// Preserve fields this structured form doesn't submit so a save here
		// doesn't clobber them (SessionNotes is edited via its own endpoint).
		j.SessionNotes = existing.SessionNotes
		j.Title = existing.Title
		j.Date = existing.Date
		j.VenueID = existing.VenueID
		j.BoardID = existing.BoardID
	}

	if err := db.UpsertSessionJournal(s.store.DB, &j); err != nil {
		s.serverError(w, r, err)
		return
	}

	// Return the saved (read-only) fragment so HTMX can swap it in.
	s.serveJournalForm(w, r, ownerID, runID, true)
}

// handleRunSessionNotes serves POST /runs/{runID}/session-notes — a live autosave
// of the free-form session notes from the open-session overview. It writes only the
// SessionNotes column (creating the journal row if needed) so it never disturbs the
// structured reflection fields.
func (s *Server) handleRunSessionNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)

	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	var run db.SessionRun
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, runID).First(&run).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		s.serverError(w, r, err)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	notes := strings.TrimSpace(r.FormValue("session_notes"))
	if err := db.UpdateSessionNotes(s.store.DB, ownerID, runID, notes); err != nil {
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
