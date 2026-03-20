package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"passion/db"
	"passion/pages"
)

func (s *Server) handleExercisesByID(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	exerciseID, err := strconv.ParseUint(chi.URLParam(r, "exerciseID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return
	}

	action := chi.URLParam(r, "action")
	switch action {
	case "update":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleUpdateExercise(w, r, uint(exerciseID), ownerID)
		return
	case "delete":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDeleteExercise(w, r, uint(exerciseID), ownerID)
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (s *Server) loadActivityExercises(activityID uint, ownerID uint) (db.Activity, error) {
	var act db.Activity
	err := s.store.DB.
		Preload("Exercises", func(tx *gorm.DB) *gorm.DB {
			return tx.Where("owner_id = ?", ownerID).Order("order_index asc")
		}).
		Preload("Exercises.Media").
		Where("id = ? AND owner_id = ?", activityID, ownerID).
		First(&act).Error
	return act, err
}

func (s *Server) nextExerciseOrder(activityID uint, ownerID uint) (int, error) {
	var maxOrder int
	if err := s.store.DB.
		Model(&db.Exercise{}).
		Where("owner_id = ? AND activity_id = ?", ownerID, activityID).
		Select("COALESCE(MAX(order_index), 0)").
		Scan(&maxOrder).Error; err != nil {
		return 0, err
	}
	return maxOrder + 1, nil
}

func (s *Server) handleReorderExercises(w http.ResponseWriter, r *http.Request, activityID uint, ownerID uint) {
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
		if seen[id] {
			continue
		}
		seen[id] = true
		orderedIDs = append(orderedIDs, id)
	}

	var existing []db.Exercise
	if err := s.store.DB.
		Where("owner_id = ? AND activity_id = ?", ownerID, activityID).
		Find(&existing).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var rootExisting []uint
	exByID := map[uint]*db.Exercise{}
	for i := range existing {
		ex := &existing[i]
		exByID[ex.ID] = ex
		if ex.ParentExerciseID == nil {
			rootExisting = append(rootExisting, ex.ID)
		}
	}
	if len(orderedIDs) != len(rootExisting) {
		http.Error(w, "ordered_ids must include all top-level exercises in the activity", http.StatusBadRequest)
		return
	}
	seenRoot := map[uint]bool{}
	for _, id := range orderedIDs {
		ex, ok := exByID[id]
		if !ok || ex.ParentExerciseID != nil {
			http.Error(w, "ordered_ids contains invalid exercise for activity", http.StatusBadRequest)
			return
		}
		seenRoot[id] = true
	}
	for _, id := range rootExisting {
		if !seenRoot[id] {
			http.Error(w, "ordered_ids must list every top-level exercise", http.StatusBadRequest)
			return
		}
	}

	tx := s.store.DB.Begin()
	for idx, id := range orderedIDs {
		if err := tx.
			Model(&db.Exercise{}).
			Where("owner_id = ? AND id = ?", ownerID, id).
			Update("order_index", idx+1).Error; err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	act, err := s.loadActivityExercises(activityID, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderExercisesWithPreview(w, act, ownerID)
}

func (s *Server) handleUpdateExercise(w http.ResponseWriter, r *http.Request, exerciseID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var ex db.Exercise
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, exerciseID).
		First(&ex).Error; err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	notes := strings.TrimSpace(r.FormValue("notes"))

	ex.Name = name
	ex.Notes = notes
	kind := db.NormalizeKind(r.FormValue("kind"))

	parentIDStr := strings.TrimSpace(r.FormValue("parent_exercise_id"))
	if parentIDStr != "" && kind == "exercise_catalog" {
		http.Error(w, "option exercises cannot be exercise catalog menus", http.StatusBadRequest)
		return
	}

	oldKind := strings.TrimSpace(ex.Kind)
	if oldKind == "" {
		oldKind = "reps_and_sets"
	}

	if oldKind == "exercise_catalog" && kind != "exercise_catalog" {
		if err := s.store.DB.
			Where("owner_id = ? AND parent_exercise_id = ?", ownerID, ex.ID).
			Delete(&db.Exercise{}).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	ex.Kind = kind
	if kind == "exercise_catalog" {
		ex.SessionDurationSeconds = 0
		ex.Sets = 0
		ex.Reps = 0
		ex.RepSeconds = 0
		ex.RepRestSeconds = 0
		ex.SetRestSeconds = 0
		ex.WeightKg = 0
		// Clear media for catalog parents.
		s.store.DB.Where("owner_id = ? AND exercise_id = ?", ownerID, ex.ID).Delete(&db.ExerciseMedia{})
	} else if kind == "session" {
		ex.SessionDurationSeconds = parseSessionDurationSeconds(r)
		ex.Sets = 0
		ex.Reps = 0
		ex.RepSeconds = 0
		ex.RepRestSeconds = 0
		ex.SetRestSeconds = 0
		ex.WeightKg = 0
	} else {
		ex.SessionDurationSeconds = 0
		ex.Sets = formInt(r, "sets")
		ex.Reps = formInt(r, "reps")
		ex.RepSeconds = formInt(r, "rep_seconds")
		ex.RepRestSeconds = formInt(r, "rep_rest_seconds")
		ex.SetRestSeconds = formInt(r, "set_rest_seconds")
		ex.WeightKg = formFloat(r, "weight_kg")
	}

	if err := s.store.DB.Save(&ex).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Sync media rows from form (non-catalog kinds only; catalogs have no media).
	if kind != "exercise_catalog" {
		if err := s.syncExerciseMediaFromForm(r, ex.ID, ownerID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := s.rerenderExerciseOwner(w, &ex, ownerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleDeleteExercise(w http.ResponseWriter, r *http.Request, exerciseID uint, ownerID uint) {
	var ex db.Exercise
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, exerciseID).
		First(&ex).Error; err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if strings.TrimSpace(ex.Kind) == "exercise_catalog" {
		if err := s.store.DB.
			Where("owner_id = ? AND parent_exercise_id = ?", ownerID, ex.ID).
			Delete(&db.Exercise{}).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, exerciseID).
		Delete(&db.Exercise{}).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.rerenderExerciseOwner(w, &ex, ownerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleAddExercise(w http.ResponseWriter, r *http.Request, activityID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "exercise name is required", http.StatusBadRequest)
		return
	}

	notes := strings.TrimSpace(r.FormValue("notes"))

	orderIndex, err := s.nextExerciseOrder(activityID, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	kind := db.NormalizeKind(r.FormValue("kind"))
	parentIDStr := strings.TrimSpace(r.FormValue("parent_exercise_id"))
	if parentIDStr != "" {
		if kind == "exercise_catalog" {
			http.Error(w, "cannot nest an exercise catalog under an option", http.StatusBadRequest)
			return
		}
		p64, err := strconv.ParseUint(parentIDStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid parent_exercise_id", http.StatusBadRequest)
			return
		}
		var parent db.Exercise
		if err := s.store.DB.
			Where("owner_id = ? AND activity_id = ? AND id = ?", ownerID, activityID, uint(p64)).
			First(&parent).Error; err != nil {
			http.Error(w, "parent exercise not found", http.StatusNotFound)
			return
		}
		if strings.TrimSpace(parent.Kind) != "exercise_catalog" {
			http.Error(w, "parent must be an exercise catalog", http.StatusBadRequest)
			return
		}
		pid := uint(p64)
		aid := activityID
		ex := &db.Exercise{
			OwnerID:          ownerID,
			ActivityID:       &aid,
			Name:             name,
			Notes:            notes,
			Kind:             kind,
			ParentExerciseID: &pid,
			OrderIndex:       orderIndex,
		}
		if kind == "session" {
			ex.SessionDurationSeconds = parseSessionDurationSeconds(r)
		} else {
			ex.Sets = formInt(r, "sets")
			ex.Reps = formInt(r, "reps")
			ex.RepSeconds = formInt(r, "rep_seconds")
			ex.RepRestSeconds = formInt(r, "rep_rest_seconds")
			ex.SetRestSeconds = formInt(r, "set_rest_seconds")
			ex.WeightKg = formFloat(r, "weight_kg")
		}
		if err := s.store.DB.Create(ex).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.syncExerciseMediaFromForm(r, ex.ID, ownerID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		act, err := s.loadActivityExercises(activityID, ownerID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.renderExercisesWithPreview(w, act, ownerID)
		return
	}

	aid := activityID
	ex := &db.Exercise{
		OwnerID:    ownerID,
		ActivityID: &aid,
		Name:       name,
		Notes:      notes,
		Kind:       kind,
		OrderIndex: orderIndex,
	}
	if kind == "exercise_catalog" {
		// Catalog parent: instructions only in UI.
	} else if kind == "session" {
		ex.SessionDurationSeconds = parseSessionDurationSeconds(r)
	} else {
		ex.Sets = formInt(r, "sets")
		ex.Reps = formInt(r, "reps")
		ex.RepSeconds = formInt(r, "rep_seconds")
		ex.RepRestSeconds = formInt(r, "rep_rest_seconds")
		ex.SetRestSeconds = formInt(r, "set_rest_seconds")
		ex.WeightKg = formFloat(r, "weight_kg")
	}

	tx := s.store.DB.Begin()
	if err := tx.Create(ex).Error; err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if kind == "exercise_catalog" {
		pid := ex.ID
		p := pid
		names := r.Form["option_name"]
		libIDs := r.Form["option_library_id"]
		n := len(names)
		if len(libIDs) > n {
			n = len(libIDs)
		}
		for i := 0; i < n; i++ {
			var optName, libStr string
			if i < len(names) {
				optName = strings.TrimSpace(names[i])
			}
			if i < len(libIDs) {
				libStr = strings.TrimSpace(libIDs[i])
			}
			if libStr == "" && optName == "" {
				continue
			}
			var maxOrder int
			if err := tx.Model(&db.Exercise{}).
				Where("owner_id = ? AND activity_id = ?", ownerID, activityID).
				Select("COALESCE(MAX(order_index), 0)").
				Scan(&maxOrder).Error; err != nil {
				tx.Rollback()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			nextOrder := maxOrder + 1
			if libStr != "" {
				libID64, err := strconv.ParseUint(libStr, 10, 64)
				if err != nil {
					tx.Rollback()
					http.Error(w, "invalid option_library_id", http.StatusBadRequest)
					return
				}
				var lib db.LibraryExercise
				if err := tx.Where("owner_id = ? AND id = ?", ownerID, uint(libID64)).First(&lib).Error; err != nil {
					tx.Rollback()
					http.Error(w, "saved exercise not found", http.StatusNotFound)
					return
				}
				child := newExerciseFromLibraryExercise(lib, ownerID, activityID, nextOrder, &p)
				if err := tx.Create(child).Error; err != nil {
					tx.Rollback()
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				continue
			}
			aaid := activityID
			child := &db.Exercise{
				OwnerID:          ownerID,
				ActivityID:       &aaid,
				Name:             optName,
				Kind:             "reps_and_sets",
				ParentExerciseID: &p,
				OrderIndex:       nextOrder,
			}
			if err := tx.Create(child).Error; err != nil {
				tx.Rollback()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	if err := tx.Commit().Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Sync media for the top-level exercise (non-catalog kinds only).
	if kind != "exercise_catalog" {
		if err := s.syncExerciseMediaFromForm(r, ex.ID, ownerID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	act, err := s.loadActivityExercises(activityID, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderExercisesWithPreview(w, act, ownerID)
}

func (s *Server) handleAddExerciseFromLibrary(w http.ResponseWriter, r *http.Request, activityID uint, ownerID uint) {
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
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, libID).
		Preload("Media").
		First(&lib).Error; err != nil {
		http.Error(w, "saved exercise not found", http.StatusNotFound)
		return
	}

	var parentPtr *uint
	parentIDStr := strings.TrimSpace(r.FormValue("parent_exercise_id"))
	if parentIDStr != "" {
		p64, err := strconv.ParseUint(parentIDStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid parent_exercise_id", http.StatusBadRequest)
			return
		}
		var parent db.Exercise
		if err := s.store.DB.
			Where("owner_id = ? AND activity_id = ? AND id = ?", ownerID, activityID, uint(p64)).
			First(&parent).Error; err != nil {
			http.Error(w, "parent exercise not found", http.StatusNotFound)
			return
		}
		if strings.TrimSpace(parent.Kind) != "exercise_catalog" {
			http.Error(w, "parent must be an exercise catalog", http.StatusBadRequest)
			return
		}
		pid := uint(p64)
		parentPtr = &pid
	}

	orderIndex, err := s.nextExerciseOrder(activityID, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ex := newExerciseFromLibraryExercise(lib, ownerID, activityID, orderIndex, parentPtr)
	if err := s.store.DB.Create(ex).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy media from library exercise to template exercise.
	if err := s.copyMediaToExercise(lib.Media, ex.ID, ownerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If the library exercise is an exercise_catalog, copy its children as template-level children.
	if lib.Kind == "exercise_catalog" && parentPtr == nil {
		var libChildren []db.LibraryExercise
		s.store.DB.Where("owner_id = ? AND parent_library_exercise_id = ?", ownerID, lib.ID).
			Preload("Media").
			Order("order_index asc").Find(&libChildren)
		for ci, lc := range libChildren {
			childEx := newExerciseFromLibraryExercise(lc, ownerID, activityID, ci, &ex.ID)
			if err := s.store.DB.Create(childEx).Error; err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Copy media for each child.
			if err := s.copyMediaToExercise(lc.Media, childEx.ID, ownerID); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	act, err := s.loadActivityExercises(activityID, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderExercisesWithPreview(w, act, ownerID)
}

// rerenderExerciseOwner re-renders the appropriate exercises fragment after an update or delete,
// routing to the activity view or the activity-template view based on which FK is set.
func (s *Server) rerenderExerciseOwner(w http.ResponseWriter, ex *db.Exercise, ownerID uint) error {
	if ex.ActivityID != nil {
		act, err := s.loadActivityExercises(*ex.ActivityID, ownerID)
		if err != nil {
			return err
		}
		s.renderExercisesWithPreview(w, act, ownerID)
		return nil
	}
	if ex.ActivityTemplateID != nil {
		tpl, err := db.GetActivityTemplateWithExercises(s.store.DB, ownerID, *ex.ActivityTemplateID)
		if err != nil {
			return err
		}
		s.renderActivityTemplateExercises(w, tpl, ownerID)
		return nil
	}
	http.Error(w, "exercise has no owner", http.StatusInternalServerError)
	return nil
}

func (s *Server) renderExercisesWithPreview(w http.ResponseWriter, act db.Activity, ownerID uint) {
	tpl, err := s.loadTemplateWithGraph(act.SessionTemplateID, ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lib, err := s.listLibraryExercises(ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	frag := pages.ExercisesFragmentData{Activity: act, LibraryExercises: lib}
	s.pages.RenderFragment(w, "fragments/exercises_container", frag)
	s.pages.RenderFragment(w, "fragments/preview_container", pages.TemplateFragmentData{Template: tpl})
}
