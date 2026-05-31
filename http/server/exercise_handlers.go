package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"passion/db"
	"passion/pages"
)

func (s *Server) handleExercisesByID(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	exerciseID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return
	}

	action := chi.URLParam(r, "action")
	switch action {
	case "update":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w)
			return
		}
		s.handleUpdateExercise(w, r, exerciseID, ownerID)
		return
	case "delete":
		if r.Method != http.MethodPost {
			s.methodNotAllowed(w)
			return
		}
		s.handleDeleteExercise(w, r, exerciseID, ownerID)
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

	var existing []db.Exercise
	if err := s.store.DB.
		Where("owner_id = ? AND activity_id = ?", ownerID, activityID).
		Find(&existing).Error; err != nil {
		s.serverError(w, r, err)
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
			s.serverError(w, r, err)
			return
		}
	}
	if err := tx.Commit().Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	act, err := s.loadActivityExercises(activityID, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderExercisesWithPreview(w, r, act, ownerID)
}

func (s *Server) handleUpdateExercise(w http.ResponseWriter, r *http.Request, exerciseID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}

	var ex db.Exercise
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, exerciseID).
		First(&ex).Error; err != nil {
		s.notFound(w)
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

	if ex.Kind == "exercise_catalog" && kind != "exercise_catalog" {
		if err := s.store.DB.
			Where("owner_id = ? AND parent_exercise_id = ?", ownerID, ex.ID).
			Delete(&db.Exercise{}).Error; err != nil {
			s.serverError(w, r, err)
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
		ex.PrepSeconds = 0
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
		ex.PrepSeconds = 0
		ex.RungSeconds = ""
		ex.WeightKg = 0
	} else if kind == "timed_reps" {
		ex.SessionDurationSeconds = 0
		ex.Sets = formInt(r, "sets")
		ex.Reps = formInt(r, "reps")
		ex.RepSeconds = formInt(r, "rep_seconds")
		ex.RepRestSeconds = formInt(r, "rep_rest_seconds")
		ex.SetRestSeconds = formInt(r, "set_rest_seconds")
		ex.PrepSeconds = formInt(r, "prep_seconds")
		ex.RungSeconds = strings.TrimSpace(r.FormValue("rung_seconds"))
		ex.WeightKg = formFloat(r, "weight_kg")
	} else {
		// reps_and_sets: counter only, no timer fields
		ex.SessionDurationSeconds = 0
		ex.Sets = formInt(r, "sets")
		ex.Reps = formInt(r, "reps")
		ex.RepSeconds = 0
		ex.RepRestSeconds = 0
		ex.SetRestSeconds = 0
		ex.PrepSeconds = 0
		ex.RungSeconds = ""
		ex.WeightKg = formFloat(r, "weight_kg")
	}

	if err := s.store.DB.Save(&ex).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	// Sync media rows from form (non-catalog kinds only; catalogs have no media).
	if kind != "exercise_catalog" {
		if err := s.syncExerciseMediaFromForm(r, ex.ID, ownerID); err != nil {
			s.serverError(w, r, err)
			return
		}
	}

	if err := s.rerenderExerciseOwner(w, r, &ex, ownerID); err != nil {
		s.serverError(w, r, err)
	}
}

func (s *Server) handleDeleteExercise(w http.ResponseWriter, r *http.Request, exerciseID uint, ownerID uint) {
	var ex db.Exercise
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, exerciseID).
		First(&ex).Error; err != nil {
		s.notFound(w)
		return
	}

	if strings.TrimSpace(ex.Kind) == "exercise_catalog" {
		if err := s.store.DB.
			Where("owner_id = ? AND parent_exercise_id = ?", ownerID, ex.ID).
			Delete(&db.Exercise{}).Error; err != nil {
			s.serverError(w, r, err)
			return
		}
	}

	_ = db.DeleteAllExercisePlannedSets(s.store.DB, ownerID, exerciseID)

	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, exerciseID).
		Delete(&db.Exercise{}).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	if err := s.rerenderExerciseOwner(w, r, &ex, ownerID); err != nil {
		s.serverError(w, r, err)
	}
}

func (s *Server) handleAddExercise(w http.ResponseWriter, r *http.Request, activityID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
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
		s.serverError(w, r, err)
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
		} else if kind == "timed_reps" {
			ex.Sets = formInt(r, "sets")
			ex.Reps = formInt(r, "reps")
			ex.RepSeconds = formInt(r, "rep_seconds")
			ex.RepRestSeconds = formInt(r, "rep_rest_seconds")
			ex.SetRestSeconds = formInt(r, "set_rest_seconds")
			ex.PrepSeconds = formInt(r, "prep_seconds")
			ex.RungSeconds = strings.TrimSpace(r.FormValue("rung_seconds"))
			ex.WeightKg = formFloat(r, "weight_kg")
		} else {
			ex.Sets = formInt(r, "sets")
			ex.Reps = formInt(r, "reps")
			ex.WeightKg = formFloat(r, "weight_kg")
		}
		if err := s.store.DB.Create(ex).Error; err != nil {
			s.serverError(w, r, err)
			return
		}
		if err := s.syncExerciseMediaFromForm(r, ex.ID, ownerID); err != nil {
			s.serverError(w, r, err)
			return
		}
		act, err := s.loadActivityExercises(activityID, ownerID)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		s.renderExercisesWithPreview(w, r, act, ownerID)
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
	} else if kind == "timed_reps" {
		ex.Sets = formInt(r, "sets")
		ex.Reps = formInt(r, "reps")
		ex.RepSeconds = formInt(r, "rep_seconds")
		ex.RepRestSeconds = formInt(r, "rep_rest_seconds")
		ex.SetRestSeconds = formInt(r, "set_rest_seconds")
		ex.PrepSeconds = formInt(r, "prep_seconds")
		ex.RungSeconds = strings.TrimSpace(r.FormValue("rung_seconds"))
		ex.WeightKg = formFloat(r, "weight_kg")
	} else {
		// reps_and_sets: counter only
		ex.Sets = formInt(r, "sets")
		ex.Reps = formInt(r, "reps")
		ex.WeightKg = formFloat(r, "weight_kg")
	}

	tx := s.store.DB.Begin()
	if err := tx.Create(ex).Error; err != nil {
		tx.Rollback()
		s.serverError(w, r, err)
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
				s.serverError(w, r, err)
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
					s.serverError(w, r, err)
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
				s.serverError(w, r, err)
				return
			}
		}
	}
	if err := tx.Commit().Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	// Sync media for the top-level exercise (non-catalog kinds only).
	if kind != "exercise_catalog" {
		if err := s.syncExerciseMediaFromForm(r, ex.ID, ownerID); err != nil {
			s.serverError(w, r, err)
			return
		}
	}

	act, err := s.loadActivityExercises(activityID, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderExercisesWithPreview(w, r, act, ownerID)
}

func (s *Server) handleAddExerciseFromLibrary(w http.ResponseWriter, r *http.Request, activityID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
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
		s.serverError(w, r, err)
		return
	}

	ex := newExerciseFromLibraryExercise(lib, ownerID, activityID, orderIndex, parentPtr)
	if err := s.store.DB.Create(ex).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	// Copy media from library exercise to template exercise.
	if err := s.copyMediaToExercise(lib.Media, ex.ID, ownerID); err != nil {
		s.serverError(w, r, err)
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
				s.serverError(w, r, err)
				return
			}
			// Copy media for each child.
			if err := s.copyMediaToExercise(lc.Media, childEx.ID, ownerID); err != nil {
				s.serverError(w, r, err)
				return
			}
		}
	}

	act, err := s.loadActivityExercises(activityID, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderExercisesWithPreview(w, r, act, ownerID)
}

// rerenderExerciseOwner re-renders the appropriate exercises fragment after an update or delete,
// routing to the activity view or the activity-template view based on which FK is set.
func (s *Server) rerenderExerciseOwner(w http.ResponseWriter, r *http.Request, ex *db.Exercise, ownerID uint) error {
	if ex.ActivityID != nil {
		act, err := s.loadActivityExercises(*ex.ActivityID, ownerID)
		if err != nil {
			return err
		}
		s.renderExercisesWithPreview(w, r, act, ownerID)
		return nil
	}
	if ex.ActivityTemplateID != nil {
		tpl, err := db.GetActivityTemplateWithExercises(s.store.DB, ownerID, *ex.ActivityTemplateID)
		if err != nil {
			return err
		}
		s.renderActivityTemplateExercises(w, r, tpl, ownerID)
		return nil
	}
	s.serverError(w, r, fmt.Errorf("exercise %d has no owner", ex.ID))
	return nil
}

func (s *Server) renderExercisesWithPreview(w http.ResponseWriter, r *http.Request, act db.Activity, ownerID uint) {
	tpl, err := s.loadTemplateWithGraph(act.SessionTemplateID, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	lib, err := s.listLibraryExercises(ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	plannedSets := make(map[uint][]pages.ExercisePlannedSetView)
	for _, ex := range act.Exercises {
		rows, _ := db.ListExercisePlannedSets(s.store.DB, ex.ID)
		if len(rows) > 0 {
			views := make([]pages.ExercisePlannedSetView, len(rows))
			for i, r := range rows {
				views[i] = pages.ExercisePlannedSetView{SetIndex: r.SetIndex, Reps: r.Reps, WeightKg: r.WeightKg}
			}
			plannedSets[ex.ID] = views
		}
	}

	frag := pages.ExercisesFragmentData{Activity: act, LibraryExercises: lib, PlannedSets: plannedSets}
	s.pages.RenderFragment(w, "fragments/exercises_container", frag)
	s.pages.RenderFragment(w, "fragments/preview_container", pages.TemplateFragmentData{Template: tpl})
}

// renderPlannedSetsFragment returns the planned-sets sub-fragment for a single exercise.
func (s *Server) renderPlannedSetsFragment(w http.ResponseWriter, exerciseID uint) {
	rows, _ := db.ListExercisePlannedSets(s.store.DB, exerciseID)
	views := make([]pages.ExercisePlannedSetView, len(rows))
	for i, r := range rows {
		views[i] = pages.ExercisePlannedSetView{SetIndex: r.SetIndex, Reps: r.Reps, WeightKg: r.WeightKg}
	}
	s.pages.RenderFragment(w, "fragments/planned_sets", struct {
		ExerciseID  uint
		PlannedSets []pages.ExercisePlannedSetView
		RoutePrefix string // e.g. "/exercises/42" or "/runs/7/open/exercises/42"
	}{ExerciseID: exerciseID, PlannedSets: views, RoutePrefix: fmt.Sprintf("/exercises/%d", exerciseID)})
}

// handleExercisePlannedSets handles POST /exercises/{exerciseID}/planned-sets (add row).
func (s *Server) handleExercisePlannedSets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)
	exerciseID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return
	}
	if err := s.verifyExerciseOwner(exerciseID, ownerID); err != nil {
		http.Error(w, "exercise not found", http.StatusNotFound)
		return
	}
	rows, _ := db.ListExercisePlannedSets(s.store.DB, exerciseID)
	nextIndex := len(rows) + 1
	if err := db.UpsertExercisePlannedSet(s.store.DB, ownerID, exerciseID, nextIndex, 0, 0); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderPlannedSetsFragment(w, exerciseID)
}

// handleExercisePlannedSetSave handles POST /exercises/{exerciseID}/planned-sets/{setIndex}/save.
func (s *Server) handleExercisePlannedSetSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)
	exerciseID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return
	}
	setIndex, err := parseUintParam(r, "setIndex")
	if err != nil {
		http.Error(w, "invalid set index", http.StatusBadRequest)
		return
	}
	if err := s.verifyExerciseOwner(exerciseID, ownerID); err != nil {
		http.Error(w, "exercise not found", http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
		return
	}
	if err := db.UpsertExercisePlannedSet(s.store.DB, ownerID, exerciseID, int(setIndex), formInt(r, "reps"), formFloat(r, "weight_kg")); err != nil {
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleExercisePlannedSetDelete handles POST /exercises/{exerciseID}/planned-sets/{setIndex}/delete.
func (s *Server) handleExercisePlannedSetDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)
	exerciseID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return
	}
	setIndex, err := parseUintParam(r, "setIndex")
	if err != nil {
		http.Error(w, "invalid set index", http.StatusBadRequest)
		return
	}
	if err := s.verifyExerciseOwner(exerciseID, ownerID); err != nil {
		http.Error(w, "exercise not found", http.StatusNotFound)
		return
	}
	if err := db.DeleteExercisePlannedSet(s.store.DB, ownerID, exerciseID, int(setIndex)); err != nil {
		s.serverError(w, r, err)
		return
	}
	// Re-index remaining sets so SetIndex stays contiguous.
	if err := s.reindexPlannedSets(exerciseID, ownerID); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderPlannedSetsFragment(w, exerciseID)
}

// handleExercisePlannedSetsClear handles POST /exercises/{exerciseID}/planned-sets/clear.
func (s *Server) handleExercisePlannedSetsClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	ownerID := s.mustUserID(r)
	exerciseID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return
	}
	if err := s.verifyExerciseOwner(exerciseID, ownerID); err != nil {
		http.Error(w, "exercise not found", http.StatusNotFound)
		return
	}
	if err := db.DeleteAllExercisePlannedSets(s.store.DB, ownerID, exerciseID); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.renderPlannedSetsFragment(w, exerciseID)
}

// verifyExerciseOwner checks that an exercise belongs to ownerID.
func (s *Server) verifyExerciseOwner(exerciseID, ownerID uint) error {
	var ex db.Exercise
	return s.store.DB.Where("owner_id = ? AND id = ?", ownerID, exerciseID).First(&ex).Error
}

// reindexPlannedSets renumbers set_index values to 1..N after a delete.
func (s *Server) reindexPlannedSets(exerciseID, ownerID uint) error {
	rows, err := db.ListExercisePlannedSets(s.store.DB, exerciseID)
	if err != nil {
		return err
	}
	for i, row := range rows {
		if row.SetIndex != i+1 {
			row.SetIndex = i + 1
			if err := s.store.DB.Save(&row).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
