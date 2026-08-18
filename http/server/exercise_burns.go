package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"passion/db"
	"passion/pages"
)

// burnLabel formats a burn duration as m:ss (or h:mm:ss for very long efforts).
func burnLabel(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// buildExerciseBurnsView loads an exercise's burns and the two summary numbers.
func (s *Server) buildExerciseBurnsView(ownerID, runID, exerciseID uint, readOnly bool) pages.ExerciseBurnsView {
	rows, _ := db.ListExerciseBurns(s.store.DB, ownerID, runID, exerciseID)
	view := pages.ExerciseBurnsView{RunID: runID, ExerciseID: exerciseID, ReadOnly: readOnly}
	total, best := 0, 0
	for _, b := range rows {
		total += b.DurationSeconds
		if b.DurationSeconds > best {
			best = b.DurationSeconds
		}
		view.Burns = append(view.Burns, pages.ExerciseBurnView{
			ID:    b.ID,
			Index: b.OrderIndex,
			Label: burnLabel(b.DurationSeconds),
		})
	}
	view.TotalLabel = burnLabel(total)
	view.BestLabel = burnLabel(best)
	return view
}

// attachBurnSummary fills the time-on-the-wall figures on a summary exercise, leaving
// them zero when no burns were logged so the summary stays quiet for normal climbing.
func (s *Server) attachBurnSummary(se *pages.RunSummaryExercise, ownerID, runID, exerciseID uint) {
	rows, _ := db.ListExerciseBurns(s.store.DB, ownerID, runID, exerciseID)
	if len(rows) == 0 {
		return
	}
	total, best := 0, 0
	for _, b := range rows {
		total += b.DurationSeconds
		if b.DurationSeconds > best {
			best = b.DurationSeconds
		}
	}
	se.BurnCount = len(rows)
	se.BurnTotalLabel = burnLabel(total)
	se.BurnBestLabel = burnLabel(best)
}

// resolveRunExercise validates that the exercise belongs to the owner's run.
func (s *Server) resolveRunExercise(w http.ResponseWriter, r *http.Request, ownerID uint) (uint, uint, bool) {
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return 0, 0, false
	}
	exerciseID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return 0, 0, false
	}
	var run db.SessionRun
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, runID).First(&run).Error; err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return 0, 0, false
	}
	var ex db.Exercise
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, exerciseID).First(&ex).Error; err != nil {
		http.Error(w, "exercise not found", http.StatusNotFound)
		return 0, 0, false
	}
	return runID, exerciseID, true
}

// handleExerciseBurns serves the burn list (GET) and logs a new burn (POST).
func (s *Server) handleExerciseBurns(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	runID, exerciseID, ok := s.resolveRunExercise(w, r, ownerID)
	if !ok {
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			s.badRequest(w, "bad request")
			return
		}
		// The stopwatch posts duration_seconds; the manual form (used when filling in a
		// past session) posts minutes + seconds instead.
		secs, err := strconv.Atoi(strings.TrimSpace(r.FormValue("duration_seconds")))
		if err != nil || secs <= 0 {
			mins, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("minutes")))
			rem, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("seconds")))
			if mins < 0 {
				mins = 0
			}
			if rem < 0 {
				rem = 0
			}
			secs = mins*60 + rem
		}
		if secs <= 0 {
			http.Error(w, "a burn needs a duration", http.StatusBadRequest)
			return
		}
		// A burn is one continuous effort; cap it so a forgotten running timer can't
		// record an absurd value.
		if secs > 6*60*60 {
			secs = 6 * 60 * 60
		}
		if err := db.AddExerciseBurn(s.store.DB, ownerID, runID, exerciseID, secs); err != nil {
			s.serverError(w, r, err)
			return
		}
	} else if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	s.pages.RenderFragment(w, "fragments/exercise_burns", s.buildExerciseBurnsView(ownerID, runID, exerciseID, false))
}

// handleExerciseBurnDelete removes a logged burn and re-renders the list.
func (s *Server) handleExerciseBurnDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)
	runID, exerciseID, ok := s.resolveRunExercise(w, r, ownerID)
	if !ok {
		return
	}
	burnID, err := parseUintParam(r, "burnID")
	if err != nil {
		http.Error(w, "invalid burn id", http.StatusBadRequest)
		return
	}
	if err := db.DeleteExerciseBurn(s.store.DB, ownerID, runID, exerciseID, burnID); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.pages.RenderFragment(w, "fragments/exercise_burns", s.buildExerciseBurnsView(ownerID, runID, exerciseID, false))
}
