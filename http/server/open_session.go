package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"passion/db"
)

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
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	customName := strings.TrimSpace(r.FormValue("name"))

	tpl, err := s.getOrCreateOpenSessionTemplate(ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	run := &db.SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: scheduled.ID,
		IsTrial:            true,
		IsOpen:             true,
		CustomName:         customName,
		Status:             "running",
		StartedAt:          now,
	}
	if err := s.store.DB.Create(run).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/runs/"+strconv.FormatUint(uint64(run.ID), 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleOpenAddExercise(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ownerID, ok := s.currentUserID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	runID64, err := strconv.ParseUint(chi.URLParam(r, "runID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}
	runID := uint(runID64)

	var run db.SessionRun
	if err := s.store.DB.
		Where("owner_id = ? AND id = ? AND is_open = ?", ownerID, runID, true).
		First(&run).Error; err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	if run.Status != "running" {
		http.Error(w, "run is not active", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var count int64
	s.store.DB.Model(&db.Exercise{}).
		Where("session_run_id = ? AND parent_exercise_id IS NULL", runID).
		Count(&count)
	orderIndex := int(count)

	libIDStr := strings.TrimSpace(r.FormValue("library_exercise_id"))
	saveToLibrary := r.FormValue("save_to_library") == "1"

	rid := runID
	var ex db.Exercise

	if libIDStr != "" {
		libID, err := strconv.ParseUint(libIDStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid library_exercise_id", http.StatusBadRequest)
			return
		}
		var lib db.LibraryExercise
		if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, uint(libID)).First(&lib).Error; err != nil {
			http.Error(w, "library exercise not found", http.StatusNotFound)
			return
		}
		ex = db.Exercise{
			OwnerID:                ownerID,
			SessionRunID:           &rid,
			Name:                   lib.Name,
			Kind:                   lib.Kind,
			Notes:                  lib.Notes,
			Sets:                   lib.Sets,
			Reps:                   lib.Reps,
			RepSeconds:             lib.RepSeconds,
			RepRestSeconds:         lib.RepRestSeconds,
			SetRestSeconds:         lib.SetRestSeconds,
			WeightKg:               lib.WeightKg,
			SessionDurationSeconds: lib.SessionDurationSeconds,
			OrderIndex:             orderIndex,
		}
	} else {
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		kind := strings.TrimSpace(r.FormValue("kind"))
		if kind == "" {
			kind = "reps_and_sets"
		}
		ex = db.Exercise{
			OwnerID:      ownerID,
			SessionRunID: &rid,
			Name:         name,
			Kind:         kind,
			OrderIndex:   orderIndex,
		}
		if saveToLibrary {
			libEx := db.LibraryExercise{
				OwnerID: ownerID,
				Name:    name,
				Kind:    kind,
			}
			s.store.DB.Create(&libEx) // best-effort; don't block on error
		}
	}

	if err := s.store.DB.Create(&ex).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/runs/"+strconv.FormatUint(uint64(runID), 10)+
		"?exercise="+strconv.FormatUint(uint64(ex.ID), 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleOpenAddTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ownerID, ok := s.currentUserID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	runID64, err := strconv.ParseUint(chi.URLParam(r, "runID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}
	runID := uint(runID64)

	var run db.SessionRun
	if err := s.store.DB.
		Where("owner_id = ? AND id = ? AND is_open = ?", ownerID, runID, true).
		First(&run).Error; err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	if run.Status != "running" {
		http.Error(w, "run is not active", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	templateIDStr := strings.TrimSpace(r.FormValue("activity_template_id"))
	if templateIDStr == "" {
		http.Error(w, "activity_template_id is required", http.StatusBadRequest)
		return
	}
	templateID64, err := strconv.ParseUint(templateIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid activity_template_id", http.StatusBadRequest)
		return
	}

	tpl, err := db.GetActivityTemplateWithExercises(s.store.DB, ownerID, uint(templateID64))
	if err != nil {
		http.Error(w, "template not found", http.StatusNotFound)
		return
	}

	var count int64
	s.store.DB.Model(&db.Exercise{}).
		Where("session_run_id = ? AND parent_exercise_id IS NULL", runID).
		Count(&count)
	orderBase := int(count)

	rid := runID
	var firstExID uint
	for i, tmplEx := range tpl.Exercises {
		ex := db.Exercise{
			OwnerID:                ownerID,
			SessionRunID:           &rid,
			Name:                   tmplEx.Name,
			Kind:                   tmplEx.Kind,
			Notes:                  tmplEx.Notes,
			Sets:                   tmplEx.Sets,
			Reps:                   tmplEx.Reps,
			RepSeconds:             tmplEx.RepSeconds,
			RepRestSeconds:         tmplEx.RepRestSeconds,
			SetRestSeconds:         tmplEx.SetRestSeconds,
			WeightKg:               tmplEx.WeightKg,
			SessionDurationSeconds: tmplEx.SessionDurationSeconds,
			OrderIndex:             orderBase + i,
		}
		if err := s.store.DB.Create(&ex).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if i == 0 {
			firstExID = ex.ID
		}
	}

	redirect := "/runs/" + strconv.FormatUint(uint64(runID), 10)
	if firstExID > 0 {
		redirect += "?exercise=" + strconv.FormatUint(uint64(firstExID), 10)
	}
	w.Header().Set("HX-Redirect", redirect)
	w.WriteHeader(http.StatusOK)
}
