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

	now := time.Now()
	var draft *db.SessionRun

	// If ?resume=N is provided, load the existing draft instead of creating a new one.
	if resumeID := r.URL.Query().Get("resume"); resumeID != "" {
		id, err := strconv.ParseUint(resumeID, 10, 64)
		if err == nil {
			var existing db.SessionRun
			if err := s.store.DB.
				Where("owner_id = ? AND id = ? AND is_draft = ?", ownerID, uint(id), true).
				First(&existing).Error; err == nil {
				draft = &existing
			}
		}
	}

	if draft == nil {
		// GET: create a draft run anchored to the open-session system template.
		tpl, err := s.getOrCreateOpenSessionTemplate(ownerID)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
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
		draft, err = db.CreateDraftSessionRun(s.store.DB, ownerID, scheduled.ID)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
	}

	libExercises, err := db.ListLibraryExercises(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	activityTemplates, err := db.ListActivityTemplates(s.store.DB, ownerID, "", "")
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

	// Parse venue (free-text, find-or-create) and board.
	location := strings.TrimSpace(r.FormValue("location"))
	venueName := strings.TrimSpace(r.FormValue("venue_name"))
	var venueID *uint
	if venueName != "" {
		kind := "commercial"
		if location == "outdoor" {
			kind = "outdoor"
		}
		var existing db.ClimbingVenue
		if err2 := s.store.DB.Where("owner_id = ? AND LOWER(name) = LOWER(?)", ownerID, venueName).First(&existing).Error; err2 == nil {
			venueID = &existing.ID
		} else {
			newVenue := &db.ClimbingVenue{OwnerID: ownerID, Name: venueName, Kind: kind}
			if err2 := db.CreateClimbingVenue(s.store.DB, newVenue); err2 == nil {
				venueID = &newVenue.ID
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

	newEx, err := db.AddManualExercise(s.store.DB, ownerID, runID, name, libExerciseID, kind)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	s.renderManualExercises(w, r, ownerID, runID, newEx.ID)
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
	elapsedSeconds := formInt(r, "elapsed_minutes") * 60
	if err := db.UpsertManualExerciseCompletion(
		s.store.DB, ownerID, runID, exerciseID,
		formInt(r, "sets"), formInt(r, "reps"), formFloat(r, "weight_kg"),
		strings.TrimSpace(r.FormValue("notes")), elapsedSeconds,
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
// Pass an optional openID to auto-open that exercise (e.g. the one just added).
func (s *Server) renderManualExercises(w http.ResponseWriter, r *http.Request, ownerID, runID uint, openID ...uint) {
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
		var comp db.RunExerciseCompletion
		if err2 := s.store.DB.Where("owner_id = ? AND run_id = ? AND exercise_id = ?",
			ownerID, runID, ex.ID).First(&comp).Error; err2 == nil {
			mev.ActualSets = comp.ActualSets
			mev.ActualReps = comp.ActualReps
			mev.ActualWeightKg = comp.ActualWeightKg
			mev.Notes = comp.RunNotes
			mev.ElapsedMinutes = comp.ElapsedSeconds / 60
		}
		setLogs, _ := db.ListManualExerciseSetLogs(s.store.DB, ownerID, runID, ex.ID)
		if len(setLogs) > 0 {
			mev.PerSetMode = true
			mev.SetLogs = make([]pages.ManualExerciseSetLogView, len(setLogs))
			for i, sl := range setLogs {
				mev.SetLogs[i] = pages.ManualExerciseSetLogView{
					SetIndex: sl.SetIndex,
					Reps:     sl.Reps,
					WeightKg: sl.WeightKg,
				}
			}
		}
		if ex.Kind == "climbing" {
			if meta, _ := db.GetClimbingExerciseMeta(s.store.DB, ownerID, runID, ex.ID); meta != nil {
				mv := pages.ClimbingExerciseMetaView{
					Type:      meta.Type,
					BoardKind: meta.BoardKind,
				}
				if meta.BoardID != nil {
					mv.BoardID = *meta.BoardID
				}
				mev.ClimbingMeta = mv
			}
		}
		views = append(views, mev)
	}
	activityTemplates, _ := db.ListActivityTemplates(s.store.DB, ownerID, "", "")
	_, boards, _ := s.loadVenuesAndBoards(ownerID)
	openExerciseID := uint(0)
	if len(openID) > 0 {
		openExerciseID = openID[0]
	}
	s.pages.RenderManualExercisesContainer(w, pages.TrainingLogNewParams{
		DraftRunID:        runID,
		OpenExerciseID:    openExerciseID,
		LibraryExercises:  libExercises,
		ActivityTemplates: activityTemplates,
		Boards:            boards,
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

// handleTrainingLogSetMode serves POST /training-log/draft/{runID}/exercises/{exerciseID}/sets/mode.
// mode=on creates the first set log; mode=off deletes all set logs.
func (s *Server) handleTrainingLogSetMode(w http.ResponseWriter, r *http.Request) {
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
	if r.FormValue("mode") == "on" {
		if err := db.UpsertManualExerciseSetLog(s.store.DB, ownerID, runID, exerciseID, 1, 0, 0); err != nil {
			s.serverError(w, r, err)
			return
		}
	} else {
		if err := db.DeleteAllManualExerciseSetLogs(s.store.DB, ownerID, runID, exerciseID); err != nil {
			s.serverError(w, r, err)
			return
		}
	}
	s.renderManualExercises(w, r, ownerID, runID)
}

// handleTrainingLogAddSet serves POST /training-log/draft/{runID}/exercises/{exerciseID}/sets.
// Appends a new empty set row.
func (s *Server) handleTrainingLogAddSet(w http.ResponseWriter, r *http.Request) {
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
	logs, err := db.ListManualExerciseSetLogs(s.store.DB, ownerID, runID, exerciseID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	nextIndex := len(logs) + 1
	if err := db.UpsertManualExerciseSetLog(s.store.DB, ownerID, runID, exerciseID, nextIndex, 0, 0); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderManualExercises(w, r, ownerID, runID)
}

// handleTrainingLogSaveSet serves POST /training-log/draft/{runID}/exercises/{exerciseID}/sets/{setIndex}/save.
// Auto-saves a single set row on input change.
func (s *Server) handleTrainingLogSaveSet(w http.ResponseWriter, r *http.Request) {
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
	setIndex, err := parseUintParam(r, "setIndex")
	if err != nil {
		http.Error(w, "invalid set index", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}
	if err := db.UpsertManualExerciseSetLog(s.store.DB, ownerID, runID, exerciseID,
		int(setIndex), formInt(r, "reps"), formFloat(r, "weight_kg")); err != nil {
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleTrainingLogDeleteSet serves POST /training-log/draft/{runID}/exercises/{exerciseID}/sets/{setIndex}/delete.
func (s *Server) handleTrainingLogDeleteSet(w http.ResponseWriter, r *http.Request) {
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
	setIndex, err := parseUintParam(r, "setIndex")
	if err != nil {
		http.Error(w, "invalid set index", http.StatusBadRequest)
		return
	}
	if err := db.DeleteManualExerciseSetLog(s.store.DB, ownerID, runID, exerciseID, int(setIndex)); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderManualExercises(w, r, ownerID, runID)
}

// handleTrainingLogSaveClimbingMeta serves POST /training-log/draft/{runID}/exercises/{exerciseID}/climbing-meta.
// Auto-saves session-level climbing context (type, board kind) with hx-swap="none".
func (s *Server) handleTrainingLogSaveClimbingMeta(w http.ResponseWriter, r *http.Request) {
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
	climbType := strings.TrimSpace(r.FormValue("climb_type"))
	boardKind := strings.TrimSpace(r.FormValue("board_kind"))
	var boardID *uint
	if climbType == "board" {
		boardIDStr := strings.TrimSpace(r.FormValue("board_id"))
		if bid, err2 := strconv.ParseUint(boardIDStr, 10, 64); err2 == nil && bid > 0 {
			bv := uint(bid)
			boardID = &bv
			var board db.ClimbingBoard
			if err2 := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, bv).First(&board).Error; err2 == nil {
				boardKind = board.BoardType
			}
		}
	} else {
		boardKind = ""
	}
	if err := db.UpsertClimbingExerciseMeta(s.store.DB, ownerID, runID, exerciseID, climbType, boardKind, boardID); err != nil {
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleTrainingLogReorderExercises saves a new display order for manual exercises.
// Body: repeated ids[]=<exerciseID> in desired order.
func (s *Server) handleTrainingLogReorderExercises(w http.ResponseWriter, r *http.Request) {
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
	ids := r.Form["ids"]
	for i, idStr := range ids {
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			continue
		}
		s.store.DB.Model(&db.Exercise{}).
			Where("id = ? AND owner_id = ? AND session_run_id = ?", id, ownerID, runID).
			Update("order_index", i)
	}
	w.WriteHeader(http.StatusNoContent)
}
