package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"passion/db"
	"passion/pages"
)
// handleTrainingLogNew serves GET|POST /training-log/new.
// GET: creates a draft SessionRun so exercise/tick HTMX routes are live from page load.
// POST: finalises the draft run, creates a SessionJournal, redirects to /training-log.
func (s *Server) handleTrainingLogNew(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)

	if r.Method == http.MethodPost {
		s.finaliseManualEntry(w, r, ownerID)
		return
	}

	// GET: create a draft run anchored to the open-session system template.
	tpl, err := s.getOrCreateOpenSessionTemplate(ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	now := time.Now()
	scheduled := &db.ScheduledSession{
		OwnerID:           ownerID,
		IsTrial:           true,
		ScheduledDate:     localDate(now),
		SessionTemplateID: tpl.ID,
	}
	if err := s.store.DB.Create(scheduled).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	draft, err := db.CreateDraftSessionRun(s.store.DB, ownerID, scheduled.ID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	libExercises, err := db.ListLibraryExercises(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	activityTemplates, err := db.ListActivityTemplates(s.store.DB, ownerID, "")
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	venues, boards, err := s.loadVenuesAndBoards(ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	s.pages.TrainingLogNewPage(w, pages.TrainingLogNewParams{
		DateValue:         now.Format("2006-01-02"),
		DraftRunID:        draft.ID,
		LibraryExercises:  libExercises,
		ActivityTemplates: activityTemplates,
		Venues:            venues,
		Boards:            boards,
	})
}

// finaliseManualEntry is called on POST /training-log/new.
// It finalises the draft run and creates a journal entry linked to it.
func (s *Server) finaliseManualEntry(w http.ResponseWriter, r *http.Request, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	dateStr := strings.TrimSpace(r.FormValue("date"))
	date, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
	if err != nil {
		date = time.Now()
	}

	runIDStr := strings.TrimSpace(r.FormValue("draft_run_id"))
	runID64, parseErr := strconv.ParseUint(runIDStr, 10, 64)
	runID := uint(runID64)

	// If we have a valid draft run ID, finalise it.
	if parseErr == nil && runID > 0 {
		customName := strings.TrimSpace(r.FormValue("title"))
		if err := db.FinaliseDraftRun(s.store.DB, ownerID, runID, customName, date); err != nil {
			s.serverError(w, r, err)
			return
		}
	} else {
		// No draft run (shouldn't normally happen, but fall back gracefully).
		runID = 0
	}

	// Parse optional venue and board IDs.
	venueIDStr := strings.TrimSpace(r.FormValue("venue_id"))
	boardIDStr := strings.TrimSpace(r.FormValue("board_id"))
	var venueID, boardID *uint
	if v, err2 := strconv.ParseUint(venueIDStr, 10, 64); err2 == nil && v > 0 {
		vv := uint(v)
		venueID = &vv
	}
	if b, err2 := strconv.ParseUint(boardIDStr, 10, 64); err2 == nil && b > 0 {
		bv := uint(b)
		boardID = &bv
	}

	// Determine location from venue kind if a venue was selected.
	location := strings.TrimSpace(r.FormValue("location"))
	if venueID != nil {
		var venue db.ClimbingVenue
		if err2 := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, *venueID).First(&venue).Error; err2 == nil {
			if venue.Kind == "outdoor" {
				location = "outdoor"
			} else {
				location = "indoor"
			}
		}
	}

	j := db.SessionJournal{
		OwnerID:    ownerID,
		Title:      strings.TrimSpace(r.FormValue("title")),
		Date:       date,
		SleepScore: formInt(r, "sleep_score"),
		Energy:     formInt(r, "energy"),
		RPE:        formInt(r, "rpe"),
		Focus:      strings.TrimSpace(r.FormValue("focus")),
		Location:   location,
		VenueID:    venueID,
		BoardID:    boardID,
		WentWell:   strings.TrimSpace(r.FormValue("went_well")),
		NextFocus:  strings.TrimSpace(r.FormValue("next_focus")),
	}
	if runID > 0 {
		j.RunID = &runID
	}

	if err := db.UpsertSessionJournal(s.store.DB, &j); err != nil {
		s.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, "/training-log", http.StatusSeeOther)
}

// handleTrainingLogDraftDiscard serves POST /training-log/draft/{runID}/discard.
func (s *Server) handleTrainingLogDraftDiscard(w http.ResponseWriter, r *http.Request) {
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
	if err := db.DeleteDraftRun(s.store.DB, ownerID, runID); err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/training-log", http.StatusSeeOther)
}

// handleTrainingLogAddExercise serves POST /training-log/draft/{runID}/exercises.
func (s *Server) handleTrainingLogAddExercise(w http.ResponseWriter, r *http.Request) {
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
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	var libExerciseID *uint
	libIDStr := strings.TrimSpace(r.FormValue("library_exercise_id"))
	if libIDStr != "" {
		if lid, err2 := strconv.ParseUint(libIDStr, 10, 64); err2 == nil && lid > 0 {
			lv := uint(lid)
			libExerciseID = &lv
		}
	}

	name := strings.TrimSpace(r.FormValue("name"))
	kind := strings.TrimSpace(r.FormValue("kind"))
	if kind == "" {
		kind = "reps_and_sets"
	}

	// If a library exercise was selected, pull name and kind from it.
	if libExerciseID != nil {
		var le db.LibraryExercise
		if err2 := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, *libExerciseID).First(&le).Error; err2 == nil {
			if name == "" {
				name = le.Name
			}
			kind = le.Kind
		}
	}
	if name == "" {
		name = "Exercise"
	}

	if _, err := db.AddManualExercise(s.store.DB, ownerID, runID, name, libExerciseID, kind); err != nil {
		s.serverError(w, r, err)
		return
	}

	s.renderManualExercises(w, r, ownerID, runID)
}

// handleTrainingLogSaveExerciseCompletion serves POST /training-log/draft/{runID}/exercises/{exerciseID}/save.
func (s *Server) handleTrainingLogSaveExerciseCompletion(w http.ResponseWriter, r *http.Request) {
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
	exerciseID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise ID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}
	if err := db.UpsertManualExerciseCompletion(
		s.store.DB, ownerID, runID, exerciseID,
		formInt(r, "sets"), formInt(r, "reps"), formFloat(r, "weight_kg"),
		strings.TrimSpace(r.FormValue("notes")),
	); err != nil {
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleTrainingLogDeleteExercise serves POST /training-log/draft/{runID}/exercises/{exerciseID}/delete.
func (s *Server) handleTrainingLogDeleteExercise(w http.ResponseWriter, r *http.Request) {
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
	exerciseID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise ID", http.StatusBadRequest)
		return
	}
	if err := db.DeleteManualExercise(s.store.DB, ownerID, runID, exerciseID); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderManualExercises(w, r, ownerID, runID)
}

// renderManualExercises renders the manual exercises container fragment.
func (s *Server) renderManualExercises(w http.ResponseWriter, r *http.Request, ownerID, runID uint) {
	exs, err := db.ListExercisesForRun(s.store.DB, ownerID, runID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	libExercises, err := db.ListLibraryExercises(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	views := make([]pages.ManualExerciseView, 0, len(exs))
	for _, ex := range exs {
		mev := pages.ManualExerciseView{
			ExerciseID: ex.ID,
			RunID:      runID,
			Name:       ex.Name,
			Kind:       ex.Kind,
		}
		// Load completion if it exists.
		var comp db.RunExerciseCompletion
		if err2 := s.store.DB.Where("owner_id = ? AND run_id = ? AND exercise_id = ?",
			ownerID, runID, ex.ID).First(&comp).Error; err2 == nil {
			mev.ActualSets = comp.ActualSets
			mev.ActualReps = comp.ActualReps
			mev.ActualWeightKg = comp.ActualWeightKg
			mev.Notes = comp.RunNotes
		}
		views = append(views, mev)
	}
	activityTemplates, _ := db.ListActivityTemplates(s.store.DB, ownerID, "")
	s.pages.RenderManualExercisesContainer(w, pages.TrainingLogNewParams{
		DraftRunID:        runID,
		LibraryExercises:  libExercises,
		ActivityTemplates: activityTemplates,
		Exercises:         views,
	})
}

// handleTrainingLogAddFromTemplate serves POST /training-log/draft/{runID}/from-activity-template.
// It copies all exercises from an activity template into the draft run.
func (s *Server) handleTrainingLogAddFromTemplate(w http.ResponseWriter, r *http.Request) {
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
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	atIDStr := strings.TrimSpace(r.FormValue("activity_template_id"))
	atID, err := strconv.ParseUint(atIDStr, 10, 64)
	if err != nil || atID == 0 {
		http.Error(w, "invalid activity_template_id", http.StatusBadRequest)
		return
	}

	tpl, err := db.GetActivityTemplateWithExercises(s.store.DB, ownerID, uint(atID))
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	if tpl == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	for _, ex := range tpl.Exercises {
		if _, err := db.AddManualExercise(s.store.DB, ownerID, runID, ex.Name, nil, ex.Kind); err != nil {
			s.serverError(w, r, err)
			return
		}
	}

	s.renderManualExercises(w, r, ownerID, runID)
}
