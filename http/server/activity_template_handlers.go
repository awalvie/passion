package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"passion/db"
	"passion/pages"
)

func (s *Server) handleActivityTemplatesIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	templates, err := db.ListActivityTemplates(s.store.DB, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.pages.ActivityTemplateList(w, pages.ActivityTemplateListParams{
		Base:              pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		ActivityTemplates: templates,
	})
}

func (s *Server) handleActivityTemplatesNew(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.pages.NewActivityTemplate(w, pages.NewActivityTemplateParams{
			Base: pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		})
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		tpl := &db.ActivityTemplate{
			OwnerID: ownerID,
			Name:    name,
			Label:   strings.TrimSpace(r.FormValue("label")),
		}
		if err := s.store.DB.Create(tpl).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("HX-Redirect", "/activity-templates/"+strconv.FormatUint(uint64(tpl.ID), 10)+"/edit")
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleActivityTemplatesByID(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	tplID, err := strconv.ParseUint(chi.URLParam(r, "activityTemplateID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid activity template id", http.StatusBadRequest)
		return
	}

	action := chi.URLParam(r, "action")
	subaction := chi.URLParam(r, "subaction")

	switch action {
	case "edit":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.renderActivityTemplateEdit(w, r, uint(tplID), ownerID)
	case "update":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleUpdateActivityTemplate(w, r, uint(tplID), ownerID)
	case "delete":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := s.deleteActivityTemplate(uint(tplID), ownerID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("HX-Redirect", "/activity-templates")
		w.WriteHeader(http.StatusOK)
	case "export":
		s.handleExportActivityTemplate(w, r, ownerID, uint(tplID))
	case "exercises":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		switch subaction {
		case "":
			s.handleAddActivityTemplateExercise(w, r, uint(tplID), ownerID)
		case "reorder":
			s.handleReorderActivityTemplateExercises(w, r, uint(tplID), ownerID)
		case "from-library":
			s.handleAddATExerciseFromLibrary(w, r, uint(tplID), ownerID)
		default:
			http.NotFound(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) renderActivityTemplateEdit(w http.ResponseWriter, r *http.Request, tplID, ownerID uint) {
	tpl, err := db.GetActivityTemplateWithExercises(s.store.DB, ownerID, tplID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	lib, err := db.ListLibraryExercises(s.store.DB, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.pages.ActivityTemplateEdit(w, pages.ActivityTemplateEditParams{
		Base:             pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		Template:         tpl,
		LibraryExercises: lib,
	})
}

func (s *Server) handleUpdateActivityTemplate(w http.ResponseWriter, r *http.Request, tplID, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	label := strings.TrimSpace(r.FormValue("label"))
	var tpl db.ActivityTemplate
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, tplID).First(&tpl).Error; err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	tpl.Name = name
	tpl.Label = label
	if err := s.store.DB.Save(&tpl).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("HX-Redirect", "/activity-templates/"+strconv.FormatUint(uint64(tplID), 10)+"/edit")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) deleteActivityTemplate(tplID, ownerID uint) error {
	tx := s.store.DB.Unscoped()
	if err := tx.Where("owner_id = ? AND activity_template_id = ?", ownerID, tplID).
		Delete(&db.Exercise{}).Error; err != nil {
		return err
	}
	return tx.Where("owner_id = ? AND id = ?", ownerID, tplID).
		Delete(&db.ActivityTemplate{}).Error
}

func (s *Server) nextATExerciseOrder(tplID, ownerID uint) (int, error) {
	var maxOrder int
	if err := s.store.DB.
		Model(&db.Exercise{}).
		Where("owner_id = ? AND activity_template_id = ? AND parent_exercise_id IS NULL", ownerID, tplID).
		Select("COALESCE(MAX(order_index), 0)").
		Scan(&maxOrder).Error; err != nil {
		return 0, err
	}
	return maxOrder + 1, nil
}

func (s *Server) renderActivityTemplateExercises(w http.ResponseWriter, tpl *db.ActivityTemplate, ownerID uint) {
	lib, err := db.ListLibraryExercises(s.store.DB, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := struct {
		Template         *db.ActivityTemplate
		LibraryExercises []db.LibraryExercise
	}{Template: tpl, LibraryExercises: lib}
	s.pages.RenderFragment(w, "fragments/activity_template_exercises_container", data)
}

func (s *Server) handleAddActivityTemplateExercise(w http.ResponseWriter, r *http.Request, tplID, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "exercise name is required", http.StatusBadRequest)
		return
	}
	orderIndex, err := s.nextATExerciseOrder(tplID, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	kind := db.NormalizeKind(r.FormValue("kind"))
	atID := tplID
	ex := &db.Exercise{
		OwnerID:            ownerID,
		ActivityTemplateID: &atID,
		Name:               name,
		Notes:              strings.TrimSpace(r.FormValue("notes")),
		Kind:               kind,
		OrderIndex:         orderIndex,
	}
	if kind == "session" {
		ex.SessionDurationSeconds = parseSessionDurationSeconds(r)
	} else if kind == "timed_reps" {
		ex.Sets = formInt(r, "sets")
		ex.Reps = formInt(r, "reps")
		ex.RepSeconds = formInt(r, "rep_seconds")
		ex.RepRestSeconds = formInt(r, "rep_rest_seconds")
		ex.SetRestSeconds = formInt(r, "set_rest_seconds")
		ex.PrepSeconds = formInt(r, "prep_seconds")
		ex.RungSeconds = strings.TrimSpace(r.FormValue("rung_seconds"))
		ex.WeightKg = formFloat(r, "weight_kg")
	} else if kind != "exercise_catalog" {
		// reps_and_sets: counter only
		ex.Sets = formInt(r, "sets")
		ex.Reps = formInt(r, "reps")
		ex.WeightKg = formFloat(r, "weight_kg")
	}
	if err := s.store.DB.Create(ex).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tpl, err := db.GetActivityTemplateWithExercises(s.store.DB, ownerID, tplID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderActivityTemplateExercises(w, tpl, ownerID)
}

func (s *Server) handleAddATExerciseFromLibrary(w http.ResponseWriter, r *http.Request, tplID, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	libIDStr := strings.TrimSpace(r.FormValue("library_exercise_id"))
	if libIDStr == "" {
		http.Error(w, "saved exercise is required", http.StatusBadRequest)
		return
	}
	libID, err := strconv.ParseUint(libIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid saved exercise", http.StatusBadRequest)
		return
	}
	var lib db.LibraryExercise
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, libID).
		Preload("Children").First(&lib).Error; err != nil {
		http.Error(w, "saved exercise not found", http.StatusNotFound)
		return
	}
	orderIndex, err := s.nextATExerciseOrder(tplID, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	atID := tplID
	kind := lib.Kind
	ex := &db.Exercise{
		OwnerID:                ownerID,
		ActivityTemplateID:     &atID,
		Name:                   lib.Name,
		Notes:                  lib.Notes,
		Kind:                   kind,
		SessionDurationSeconds: lib.SessionDurationSeconds,
		Sets:                   lib.Sets,
		Reps:                   lib.Reps,
		RepSeconds:             lib.RepSeconds,
		RepRestSeconds:         lib.RepRestSeconds,
		SetRestSeconds:         lib.SetRestSeconds,
		PrepSeconds:            lib.PrepSeconds,
		RungSeconds:            lib.RungSeconds,
		WeightKg:               lib.WeightKg,
		OrderIndex:             orderIndex,
	}
	if err := s.store.DB.Create(ex).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if kind == "exercise_catalog" {
		for ci, lc := range lib.Children {
			childKind := lc.Kind
			pid := ex.ID
			child := &db.Exercise{
				OwnerID:                ownerID,
				ActivityTemplateID:     &atID,
				Name:                   lc.Name,
				Notes:                  lc.Notes,
				Kind:                   childKind,
				SessionDurationSeconds: lc.SessionDurationSeconds,
				Sets:                   lc.Sets,
				Reps:                   lc.Reps,
				RepSeconds:             lc.RepSeconds,
				RepRestSeconds:         lc.RepRestSeconds,
				SetRestSeconds:         lc.SetRestSeconds,
				PrepSeconds:            lc.PrepSeconds,
				RungSeconds:            lc.RungSeconds,
				WeightKg:               lc.WeightKg,
				OrderIndex:             ci,
				ParentExerciseID:       &pid,
			}
			if err := s.store.DB.Create(child).Error; err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	tpl, err := db.GetActivityTemplateWithExercises(s.store.DB, ownerID, tplID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderActivityTemplateExercises(w, tpl, ownerID)
}

func (s *Server) handleReorderActivityTemplateExercises(w http.ResponseWriter, r *http.Request, tplID, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	orderedStr := strings.TrimSpace(r.FormValue("ordered_ids"))
	if orderedStr == "" {
		http.Error(w, "ordered_ids is required", http.StatusBadRequest)
		return
	}
	rawIDs := strings.Split(orderedStr, ",")
	orderedIDs := make([]uint, 0, len(rawIDs))
	seen := map[uint]bool{}
	for _, raw := range rawIDs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		id64, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			http.Error(w, "invalid ordered_ids", http.StatusBadRequest)
			return
		}
		id := uint(id64)
		if !seen[id] {
			seen[id] = true
			orderedIDs = append(orderedIDs, id)
		}
	}
	tx := s.store.DB.Begin()
	for idx, id := range orderedIDs {
		if err := tx.Model(&db.Exercise{}).
			Where("owner_id = ? AND activity_template_id = ? AND id = ?", ownerID, tplID, id).
			Update("order_index", idx).Error; err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tpl, err := db.GetActivityTemplateWithExercises(s.store.DB, ownerID, tplID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderActivityTemplateExercises(w, tpl, ownerID)
}
