package web

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"passion/db"
	"passion/pages"
)


// newExerciseFromLibraryExercise copies a library preset into a template Exercise (optional parent for catalog children).
func newExerciseFromLibraryExercise(lib db.LibraryExercise, ownerID, activityID uint, orderIndex int, parentID *uint) *db.Exercise {
	aid := activityID
	libID := lib.ID
	return &db.Exercise{
		OwnerID:                ownerID,
		ActivityID:             &aid,
		LibraryExerciseID:      &libID,
		Name:                   lib.Name,
		Notes:                  lib.Notes,
		Kind:                   lib.Kind,
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
		ParentExerciseID:       parentID,
	}
}

func parseSessionDurationSeconds(r *http.Request) int {
	// "session_duration_minutes" as a float (e.g. "12.5" → 750s). Empty/zero → 0 (count-up).
	v := strings.TrimSpace(r.FormValue("session_duration_minutes"))
	if v == "" {
		return 0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 0 {
		return 0
	}
	return int(math.Round(f * 60))
}

func (s *Server) syncMediaFromForm(r *http.Request, ownerID uint, exerciseID *uint, libraryExerciseID *uint) error {
	q := s.store.DB.Where("owner_id = ?", ownerID)
	if exerciseID != nil {
		q = q.Where("exercise_id = ?", *exerciseID)
	}
	if libraryExerciseID != nil {
		q = q.Where("library_exercise_id = ?", *libraryExerciseID)
	}
	q.Delete(&db.ExerciseMedia{})

	videos := r.Form["media_video_url"]
	thumbs := r.Form["media_thumbnail_url"]
	n := len(videos)
	if len(thumbs) > n {
		n = len(thumbs)
	}
	for i := 0; i < n; i++ {
		var v, t string
		if i < len(videos) {
			v = strings.TrimSpace(videos[i])
		}
		if i < len(thumbs) {
			t = strings.TrimSpace(thumbs[i])
		}
		if v == "" && t == "" {
			continue
		}
		m := db.ExerciseMedia{
			OwnerID:           ownerID,
			ExerciseID:        exerciseID,
			LibraryExerciseID: libraryExerciseID,
			VideoURL:          v,
			ThumbnailURL:      t,
			OrderIndex:        i,
		}
		if err := s.store.DB.Create(&m).Error; err != nil {
			return err
		}
	}
	return nil
}

// syncExerciseMediaFromForm deletes existing media for an exercise and re-creates
// from the submitted media_video_url[] / media_thumbnail_url[] form arrays.
func (s *Server) syncExerciseMediaFromForm(r *http.Request, exerciseID uint, ownerID uint) error {
	return s.syncMediaFromForm(r, ownerID, &exerciseID, nil)
}

// syncLibraryExerciseMediaFromForm deletes existing media for a library exercise
// and re-creates from the submitted media_video_url[] / media_thumbnail_url[] form arrays.
func (s *Server) syncLibraryExerciseMediaFromForm(r *http.Request, libraryExerciseID uint, ownerID uint) error {
	return s.syncMediaFromForm(r, ownerID, nil, &libraryExerciseID)
}

// copyMediaToExercise copies ExerciseMedia from a library exercise to a template exercise.
func (s *Server) copyMediaToExercise(srcMedia []db.ExerciseMedia, exerciseID uint, ownerID uint) error {
	for _, m := range srcMedia {
		cm := db.ExerciseMedia{
			OwnerID:      ownerID,
			ExerciseID:   &exerciseID,
			VideoURL:     m.VideoURL,
			ThumbnailURL: m.ThumbnailURL,
			OrderIndex:   m.OrderIndex,
		}
		if err := s.store.DB.Create(&cm).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) handleTemplatesIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w)
		return
	}

	ownerID := s.mustUserID(r)

	sourceFilter := strings.TrimSpace(r.URL.Query().Get("source"))
	tagFilter := strings.TrimSpace(r.URL.Query().Get("tag"))

	templates, err := db.ListTemplates(s.store.DB, ownerID, sourceFilter, tagFilter)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	distinctSources, err := db.DistinctTemplateSources(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	distinctTags, err := db.DistinctTemplateTags(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	s.pages.TemplateList(w, pages.TemplateListParams{
		Base:            pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		Templates:       templates,
		SourceFilter:    sourceFilter,
		TagFilter:       tagFilter,
		DistinctSources: distinctSources,
		DistinctTags:    distinctTags,
	})
}

func (s *Server) handleTemplatesNew(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	switch r.Method {
	case http.MethodGet:
		s.pages.NewTemplate(w, pages.NewTemplateParams{
			Base: pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		})
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			s.badRequest(w, "bad request")
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		tpl := &db.SessionTemplate{
			OwnerID: ownerID,
			Name:    name,
			Label:   strings.TrimSpace(r.FormValue("label")),
		}
		if err := s.store.DB.Create(tpl).Error; err != nil {
			s.serverError(w, r, err)
			return
		}

		// HTMX: redirect to edit page.
		w.Header().Set("HX-Redirect", "/templates/"+strconv.FormatUint(uint64(tpl.ID), 10)+"/edit")
		w.WriteHeader(http.StatusOK)
		return
	default:
		s.methodNotAllowed(w)
	}
}

func (s *Server) handleTemplatesByID(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	// Supported:
	// - GET  /templates/{id}/edit
	// - POST /templates/{id}/activities
	// - POST /templates/{id}/trial
	// - POST /templates/{id}/delete
	// - POST /templates/{id}/update  (name + color)
	templateID, err := parseUintParam(r, "templateID")
	if err != nil {
		http.Error(w, "invalid template id", http.StatusBadRequest)
		return
	}

	action := chi.URLParam(r, "action")
	subaction := chi.URLParam(r, "subaction")
	switch action {
	case "edit":
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}
		s.renderTemplateEdit(w, r, uint(templateID), ownerID)
		return
	case "activities":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w)
			return
		}
		if subaction == "reorder" {
			s.handleReorderActivities(w, r, uint(templateID), ownerID)
			return
		}
		if subaction == "from-activity-template" {
			s.handleAddActivityFromTemplate(w, r, uint(templateID), ownerID)
			return
		}
		s.handleAddActivity(w, r, uint(templateID), ownerID)
		return
	case "trial":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w)
			return
		}
		runID, err := s.startTrialRun(uint(templateID), ownerID)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		w.Header().Set("HX-Redirect", "/runs/"+strconv.FormatUint(uint64(runID), 10))
		w.WriteHeader(http.StatusOK)
		return
	case "delete":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w)
			return
		}
		if err := s.deleteTemplate(uint(templateID), ownerID); err != nil {
			s.serverError(w, r, err)
			return
		}
		w.Header().Set("HX-Redirect", "/templates")
		w.WriteHeader(http.StatusOK)
		return
	case "update":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w)
			return
		}
		s.handleUpdateSessionTemplate(w, r, uint(templateID), ownerID)
		return
	case "export":
		s.handleExportTemplate(w, r, ownerID, uint(templateID))
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (s *Server) handleReorderActivities(w http.ResponseWriter, r *http.Request, templateID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
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
		if seen[id] {
			continue
		}
		seen[id] = true
		orderedIDs = append(orderedIDs, id)
	}

	var existing []db.Activity
	if err := s.store.DB.
		Where("owner_id = ? AND session_template_id = ?", ownerID, templateID).
		Find(&existing).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	if len(existing) != len(orderedIDs) {
		http.Error(w, "ordered_ids must include all activities in the template", http.StatusBadRequest)
		return
	}
	actByID := map[uint]bool{}
	for _, act := range existing {
		actByID[act.ID] = true
	}
	for _, id := range orderedIDs {
		if !actByID[id] {
			http.Error(w, "ordered_ids contains invalid activity for template", http.StatusBadRequest)
			return
		}
	}

	tx := s.store.DB.Begin()
	for idx, id := range orderedIDs {
		if err := tx.
			Model(&db.Activity{}).
			Where("owner_id = ? AND id = ?", ownerID, id).
			Update("order_index", idx).Error; err != nil {
			tx.Rollback()
			s.serverError(w, r, err)
			return
		}
	}
	if err := tx.Commit().Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	tpl, err := s.loadTemplateWithGraph(templateID, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderActivitiesWithPreview(w, r, tpl, ownerID)
}

func (s *Server) deleteTemplate(templateID uint, ownerID uint) error {
	tx := s.store.DB.Unscoped()

	// Remove scheduled sessions that reference this template.
	if err := tx.Where("owner_id = ? AND session_template_id = ?", ownerID, templateID).
		Delete(&db.ScheduledSession{}).Error; err != nil {
		return err
	}

	// Remove weekday mappings that reference this template.
	if err := tx.Where("owner_id = ? AND session_template_id = ?", ownerID, templateID).
		Delete(&db.TrainingCycleWeekdayMapping{}).Error; err != nil {
		return err
	}

	// Remove exercises and activities belonging to this template.
	if err := tx.Where(
		"owner_id = ? AND activity_id IN (SELECT id FROM activities WHERE owner_id = ? AND session_template_id = ?)",
		ownerID, ownerID, templateID,
	).Delete(&db.Exercise{}).Error; err != nil {
		return err
	}

	if err := tx.Where("owner_id = ? AND session_template_id = ?", ownerID, templateID).
		Delete(&db.Activity{}).Error; err != nil {
		return err
	}

	// Finally remove the template itself.
	return tx.Where("owner_id = ? AND id = ?", ownerID, templateID).
		Delete(&db.SessionTemplate{}).Error
}

func (s *Server) loadTemplateWithGraph(templateID uint, ownerID uint) (*db.SessionTemplate, error) {
	return db.GetTemplateWithGraph(s.store.DB, ownerID, templateID)
}

func (s *Server) listLibraryExercises(ownerID uint) ([]db.LibraryExercise, error) {
	return db.ListLibraryExercises(s.store.DB, ownerID)
}

func (s *Server) listTemplates(ownerID uint) ([]db.SessionTemplate, error) {
	return db.ListTemplates(s.store.DB, ownerID, "", "")
}

func (s *Server) renderTemplateEdit(w http.ResponseWriter, r *http.Request, templateID uint, ownerID uint) {
	tpl, err := db.GetTemplateWithGraph(s.store.DB, ownerID, uint(templateID))
	if err != nil {
		s.notFound(w)
		return
	}

	lib, err := db.ListLibraryExercises(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	ats, err := db.ListActivityTemplates(s.store.DB, ownerID, "", "")
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	s.pages.TemplateEdit(w, pages.TemplateEditParams{
		Base:              pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		Template:          tpl,
		LibraryExercises:  lib,
		ActivityTemplates: ats,
	})
}

func (s *Server) handleUpdateSessionTemplate(w http.ResponseWriter, r *http.Request, templateID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	var color string
	if strings.TrimSpace(r.FormValue("clear_color")) != "" {
		color = ""
	} else {
		color = normalizeTemplateColor(r.FormValue("color"))
	}
	label := strings.TrimSpace(r.FormValue("label"))

	var tpl db.SessionTemplate
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, templateID).First(&tpl).Error; err != nil {
		s.notFound(w)
		return
	}
	tpl.Name = name
	tpl.Color = color
	tpl.Label = label
	tpl.Source = strings.TrimSpace(r.FormValue("source"))
	if err := s.store.DB.Save(&tpl).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	w.Header().Set("HX-Redirect", "/templates/"+strconv.FormatUint(uint64(templateID), 10)+"/edit")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) startTrialRun(templateID uint, ownerID uint) (uint, error) {
	var tpl db.SessionTemplate
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, templateID).
		First(&tpl).Error; err != nil {
		return 0, err
	}

	now := time.Now()
	scheduled := &db.ScheduledSession{
		OwnerID:           ownerID,
		TrainingCycleID:   nil,
		IsTrial:           true,
		ScheduledDate:     localDate(now),
		SessionTemplateID: templateID,
	}
	if err := s.store.DB.Create(scheduled).Error; err != nil {
		return 0, err
	}

	run := &db.SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: scheduled.ID,
		IsTrial:            true,
		Status:             db.RunStatusRunning,
		StartedAt:          now,
	}
	if err := s.store.DB.Create(run).Error; err != nil {
		return 0, err
	}
	return run.ID, nil
}

func (s *Server) renderActivitiesWithPreview(w http.ResponseWriter, r *http.Request, tpl *db.SessionTemplate, ownerID uint) {
	lib, err := db.ListLibraryExercises(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	data := struct {
		Template         *db.SessionTemplate
		LibraryExercises []db.LibraryExercise
	}{Template: tpl, LibraryExercises: lib}
	s.pages.RenderFragment(w, "fragments/activities_container", data)
	s.pages.RenderFragment(w, "fragments/preview_container", pages.TemplateFragmentData{Template: tpl})
}

func (s *Server) nextActivityOrder(templateID uint, ownerID uint) (int, error) {
	var maxOrder int
	if err := s.store.DB.
		Model(&db.Activity{}).
		Where("owner_id = ? AND session_template_id = ?", ownerID, templateID).
		Select("COALESCE(MAX(order_index), 0)").
		Scan(&maxOrder).Error; err != nil {
		return 0, err
	}
	return maxOrder + 1, nil
}
