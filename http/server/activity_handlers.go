package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"passion/db"
)

func (s *Server) handleAddActivity(w http.ResponseWriter, r *http.Request, templateID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	activityType := strings.TrimSpace(r.FormValue("type"))
	if activityType == "" {
		http.Error(w, "type is required", http.StatusBadRequest)
		return
	}

	orderIndex, err := s.nextActivityOrder(templateID, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	act := &db.Activity{
		OwnerID:           ownerID,
		SessionTemplateID: templateID,
		Type:              activityType,
		OrderIndex:        orderIndex,
	}
	if err := s.store.DB.Create(act).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tpl, err := s.loadTemplateWithGraph(templateID, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.renderActivitiesWithPreview(w, tpl, ownerID)
}

func (s *Server) handleActivitiesByID(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	// Supported:
	// - POST /activities/{id}/exercises
	// - POST /activities/{id}/delete
	activityID, err := strconv.ParseUint(chi.URLParam(r, "activityID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid activity id", http.StatusBadRequest)
		return
	}

	action := chi.URLParam(r, "action")
	switch action {
	case "exercises":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		subaction := chi.URLParam(r, "subaction")
		if subaction == "" {
			s.handleAddExercise(w, r, uint(activityID), ownerID)
			return
		}
		if subaction == "reorder" {
			s.handleReorderExercises(w, r, uint(activityID), ownerID)
			return
		}
		if subaction == "from-library" {
			s.handleAddExerciseFromLibrary(w, r, uint(activityID), ownerID)
			return
		}
		if subaction == "add-to-library" {
			s.handleSaveToLibraryFromActivity(w, r, uint(activityID), ownerID)
			return
		}
		http.NotFound(w, r)
		return
	case "delete":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDeleteActivity(w, uint(activityID), ownerID)
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (s *Server) handleAddActivityFromTemplate(w http.ResponseWriter, r *http.Request, templateID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	atIDStr := strings.TrimSpace(r.FormValue("activity_template_id"))
	if atIDStr == "" {
		http.Error(w, "activity_template_id is required", http.StatusBadRequest)
		return
	}
	atID64, err := strconv.ParseUint(atIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid activity_template_id", http.StatusBadRequest)
		return
	}
	atID := uint(atID64)

	at, err := db.GetActivityTemplateWithExercises(s.store.DB, ownerID, atID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	orderIndex, err := s.nextActivityOrder(templateID, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	act := &db.Activity{
		OwnerID:           ownerID,
		SessionTemplateID: templateID,
		Name:              at.Name,
		OrderIndex:        orderIndex,
	}
	if err := s.store.DB.Create(act).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy exercises: two-pass to preserve parent→child relationships.
	oldToNew := map[uint]uint{}
	for _, ex := range at.Exercises {
		if ex.ParentExerciseID != nil {
			continue
		}
		aid := act.ID
		newEx := db.Exercise{
			OwnerID:                ownerID,
			ActivityID:             &aid,
			Name:                   ex.Name,
			Notes:                  ex.Notes,
			Kind:                   ex.Kind,
			SessionDurationSeconds: ex.SessionDurationSeconds,
			Sets:                   ex.Sets,
			Reps:                   ex.Reps,
			RepSeconds:             ex.RepSeconds,
			RepRestSeconds:         ex.RepRestSeconds,
			SetRestSeconds:         ex.SetRestSeconds,
			WeightKg:               ex.WeightKg,
			OrderIndex:             ex.OrderIndex,
		}
		if err := s.store.DB.Create(&newEx).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.copyMediaToExercise(ex.Media, newEx.ID, ownerID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		oldToNew[ex.ID] = newEx.ID
	}
	for _, ex := range at.Exercises {
		if ex.ParentExerciseID == nil {
			continue
		}
		newParentID, ok := oldToNew[*ex.ParentExerciseID]
		if !ok {
			continue
		}
		aid := act.ID
		newEx := db.Exercise{
			OwnerID:                ownerID,
			ActivityID:             &aid,
			ParentExerciseID:       &newParentID,
			Name:                   ex.Name,
			Notes:                  ex.Notes,
			Kind:                   ex.Kind,
			SessionDurationSeconds: ex.SessionDurationSeconds,
			Sets:                   ex.Sets,
			Reps:                   ex.Reps,
			RepSeconds:             ex.RepSeconds,
			RepRestSeconds:         ex.RepRestSeconds,
			SetRestSeconds:         ex.SetRestSeconds,
			WeightKg:               ex.WeightKg,
			OrderIndex:             ex.OrderIndex,
		}
		if err := s.store.DB.Create(&newEx).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.copyMediaToExercise(ex.Media, newEx.ID, ownerID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	tpl, err := s.loadTemplateWithGraph(templateID, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderActivitiesWithPreview(w, tpl, ownerID)
}

func (s *Server) handleDeleteActivity(w http.ResponseWriter, activityID uint, ownerID uint) {
	var act db.Activity
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, activityID).
		First(&act).Error; err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := s.store.DB.
		Where("owner_id = ? AND activity_id = ?", ownerID, activityID).
		Delete(&db.Exercise{}).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, activityID).
		Delete(&db.Activity{}).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tpl, err := s.loadTemplateWithGraph(act.SessionTemplateID, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderActivitiesWithPreview(w, tpl, ownerID)
}
