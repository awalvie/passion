package web

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"passion/db"
)

// sliceStr returns the trimmed string at index i of sl, or def if out of range or blank.
func sliceStr(sl []string, i int, def string) string {
	if i < len(sl) && strings.TrimSpace(sl[i]) != "" {
		return strings.TrimSpace(sl[i])
	}
	return def
}

func sliceInt(sl []string, i int) int {
	n, _ := strconv.Atoi(sliceStr(sl, i, "0"))
	return n
}

func sliceFloat(sl []string, i int) float64 {
	f, _ := strconv.ParseFloat(sliceStr(sl, i, "0"), 64)
	return f
}

// sliceDurSecs parses a decimal minutes string and returns whole seconds (0 on error).
func sliceDurSecs(sl []string, i int) int {
	f, err := strconv.ParseFloat(sliceStr(sl, i, ""), 64)
	if err != nil || f <= 0 {
		return 0
	}
	return int(math.Round(f * 60))
}

// getOrCreateOpenSessionTemplate returns the per-user hidden system template used
// as the ScheduledSession anchor for all open sessions, creating it on first use.
func (s *Server) getOrCreateOpenSessionTemplate(ownerID uint) (db.SessionTemplate, error) {
	var tpl db.SessionTemplate
	res := s.store.DB.
		Where("owner_id = ? AND is_system = ?", ownerID, true).
		Limit(1).Find(&tpl)
	if res.Error != nil {
		return tpl, res.Error
	}
	if res.RowsAffected > 0 {
		return tpl, nil
	}
	tpl = db.SessionTemplate{
		OwnerID:  ownerID,
		Name:     "Open Session",
		IsSystem: true,
	}
	return tpl, s.store.DB.Create(&tpl).Error
}

func (s *Server) handleStartOpenSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}

	customName := strings.TrimSpace(r.FormValue("name"))

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

	run := &db.SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: scheduled.ID,
		IsTrial:            false,
		IsOpen:             true,
		CustomName:         customName,
		Status:             db.RunStatusDraft,
	}
	if err := s.store.DB.Create(run).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	w.Header().Set("HX-Redirect", "/runs/"+strconv.FormatUint(uint64(run.ID), 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleOpenAddExercise(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}

	var run db.SessionRun
	if err := s.store.DB.
		Where("owner_id = ? AND id = ? AND is_open = ?", ownerID, runID, true).
		First(&run).Error; err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	if run.Status != db.RunStatusDraft && run.Status != db.RunStatusRunning {
		http.Error(w, "run is not active", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}

	var count int64
	s.store.DB.Model(&db.Exercise{}).
		Where("session_run_id = ? AND parent_exercise_id IS NULL", runID).
		Count(&count)
	orderIndex := int(count)

	saveToLibrary := r.FormValue("save_to_library") == "1"

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	kind := strings.TrimSpace(r.FormValue("kind"))
	if kind == "" {
		kind = "reps_and_sets"
	}

	rid := runID
	ex := db.Exercise{
		OwnerID:                ownerID,
		SessionRunID:           &rid,
		Name:                   name,
		Kind:                   kind,
		Sets:                   formInt(r, "sets"),
		Reps:                   formInt(r, "reps"),
		WeightKg:               formFloat(r, "weight_kg"),
		RepSeconds:             formInt(r, "rep_seconds"),
		RepRestSeconds:         formInt(r, "rep_rest_seconds"),
		SetRestSeconds:         formInt(r, "set_rest_seconds"),
		PrepSeconds:            formInt(r, "prep_seconds"),
		SessionDurationSeconds: parseSessionDurationSeconds(r),
		Notes:                  strings.TrimSpace(r.FormValue("notes")),
		OrderIndex:             orderIndex,
	}

	if saveToLibrary {
		libEx := db.LibraryExercise{
			OwnerID: ownerID,
			Name:    name,
			Kind:    kind,
		}
		s.store.DB.Create(&libEx) // best-effort
	}

	if err := s.store.DB.Create(&ex).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	w.Header().Set("HX-Redirect", "/runs/"+strconv.FormatUint(uint64(runID), 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleOpenAddTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}

	var run db.SessionRun
	if err := s.store.DB.
		Where("owner_id = ? AND id = ? AND is_open = ?", ownerID, runID, true).
		First(&run).Error; err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	if run.Status != db.RunStatusDraft && run.Status != db.RunStatusRunning {
		http.Error(w, "run is not active", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}

	names := r.Form["ex_name"]
	if len(names) == 0 {
		http.Error(w, "no exercises provided", http.StatusBadRequest)
		return
	}

	kinds       := r.Form["ex_kind"]
	setsList    := r.Form["ex_sets"]
	repsList    := r.Form["ex_reps"]
	weights     := r.Form["ex_weight_kg"]
	repSecs     := r.Form["ex_rep_seconds"]
	repRestSecs := r.Form["ex_rep_rest_seconds"]
	setRestSecs := r.Form["ex_set_rest_seconds"]
	prepSecs    := r.Form["ex_prep_seconds"]
	durMins     := r.Form["ex_session_duration_minutes"]
	notesList   := r.Form["ex_notes"]

	var count int64
	s.store.DB.Model(&db.Exercise{}).
		Where("session_run_id = ? AND parent_exercise_id IS NULL", runID).
		Count(&count)

	rid := runID
	for i, rawName := range names {
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		}
		ex := db.Exercise{
			OwnerID:                ownerID,
			SessionRunID:           &rid,
			Name:                   name,
			Kind:                   sliceStr(kinds, i, "reps_and_sets"),
			Sets:                   sliceInt(setsList, i),
			Reps:                   sliceInt(repsList, i),
			WeightKg:               sliceFloat(weights, i),
			RepSeconds:             sliceInt(repSecs, i),
			RepRestSeconds:         sliceInt(repRestSecs, i),
			SetRestSeconds:         sliceInt(setRestSecs, i),
			PrepSeconds:            sliceInt(prepSecs, i),
			SessionDurationSeconds: sliceDurSecs(durMins, i),
			Notes:                  sliceStr(notesList, i, ""),
			OrderIndex:             int(count) + i,
		}
		if err := s.store.DB.Create(&ex).Error; err != nil {
			s.serverError(w, r, err)
			return
		}
	}

	w.Header().Set("HX-Redirect", "/runs/"+strconv.FormatUint(uint64(runID), 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleOpenStartSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}

	var run db.SessionRun
	if err := s.store.DB.
		Where("owner_id = ? AND id = ? AND is_open = ?", ownerID, runID, true).
		First(&run).Error; err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	if run.Status != db.RunStatusDraft {
		http.Error(w, "run is not in draft state", http.StatusBadRequest)
		return
	}

	now := time.Now()
	run.Status = db.RunStatusRunning
	run.StartedAt = now
	if err := s.store.DB.Save(&run).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	// Redirect to the first exercise in the guided playlist view.
	var firstEx db.Exercise
	dest := "/runs/" + strconv.FormatUint(uint64(runID), 10)
	if err := s.store.DB.
		Where("session_run_id = ? AND parent_exercise_id IS NULL", runID).
		Order("order_index ASC").
		First(&firstEx).Error; err == nil {
		dest += "?exercise=" + strconv.FormatUint(uint64(firstEx.ID), 10)
	}

	w.Header().Set("HX-Redirect", dest)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleOpenUpdateExercise(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}
	exID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return
	}

	var run db.SessionRun
	if err := s.store.DB.
		Where("owner_id = ? AND id = ? AND is_open = ?", ownerID, runID, true).
		First(&run).Error; err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	if run.Status != db.RunStatusDraft && run.Status != db.RunStatusRunning {
		http.Error(w, "run is not active", http.StatusBadRequest)
		return
	}

	var ex db.Exercise
	if err := s.store.DB.
		Where("owner_id = ? AND id = ? AND session_run_id = ?", ownerID, exID, runID).
		First(&ex).Error; err != nil {
		http.Error(w, "exercise not found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = ex.Name
	}
	kind := strings.TrimSpace(r.FormValue("kind"))
	if kind == "" {
		kind = ex.Kind
	}

	ex.Name = name
	ex.Kind = kind
	ex.Sets = formInt(r, "sets")
	ex.Reps = formInt(r, "reps")
	ex.WeightKg = formFloat(r, "weight_kg")
	ex.RepSeconds = formInt(r, "rep_seconds")
	ex.RepRestSeconds = formInt(r, "rep_rest_seconds")
	ex.SetRestSeconds = formInt(r, "set_rest_seconds")
	ex.PrepSeconds = formInt(r, "prep_seconds")
	ex.SessionDurationSeconds = parseSessionDurationSeconds(r)
	ex.Notes = strings.TrimSpace(r.FormValue("notes"))

	if err := s.store.DB.Save(&ex).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	w.Header().Set("HX-Redirect", "/runs/"+strconv.FormatUint(uint64(runID), 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleOpenDeleteExercise(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}
	exID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return
	}

	var run db.SessionRun
	if err := s.store.DB.
		Where("owner_id = ? AND id = ? AND is_open = ?", ownerID, runID, true).
		First(&run).Error; err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	if run.Status != db.RunStatusDraft && run.Status != db.RunStatusRunning {
		http.Error(w, "run is not active", http.StatusBadRequest)
		return
	}

	var ex db.Exercise
	if err := s.store.DB.
		Where("owner_id = ? AND id = ? AND session_run_id = ?", ownerID, exID, runID).
		First(&ex).Error; err != nil {
		http.Error(w, "exercise not found", http.StatusNotFound)
		return
	}

	// Only allow deleting exercises with no completion record.
	var compCount int64
	s.store.DB.Model(&db.RunExerciseCompletion{}).
		Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exID).
		Count(&compCount)
	if compCount > 0 {
		http.Error(w, "cannot delete a completed or skipped exercise", http.StatusBadRequest)
		return
	}

	if err := s.store.DB.Unscoped().Delete(&ex).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	w.Header().Set("HX-Redirect", "/runs/"+strconv.FormatUint(uint64(runID), 10))
	w.WriteHeader(http.StatusOK)
}
