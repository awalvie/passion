package web

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"passion/db"
	"passion/pages"
)

// exerciseHistoryItems returns up to `limit` past completions for an exercise,
// searching across all template exercises that share the same LibraryExerciseID
// (or the same name as fallback).
func (s *Server) exerciseHistoryItems(ex db.Exercise, ownerID uint, limit int) []pages.ExerciseHistoryItem {
	// Collect the IDs of all related template exercises.
	var exerciseIDs []uint
	if ex.LibraryExerciseID != nil && *ex.LibraryExerciseID != 0 {
		s.store.DB.Model(&db.Exercise{}).
			Where("owner_id = ? AND library_exercise_id = ?", ownerID, *ex.LibraryExerciseID).
			Pluck("id", &exerciseIDs)
	} else {
		s.store.DB.Model(&db.Exercise{}).
			Where("owner_id = ? AND name = ?", ownerID, ex.Name).
			Pluck("id", &exerciseIDs)
	}
	if len(exerciseIDs) == 0 {
		return nil
	}

	type row struct {
		ID             uint
		Status         string
		CompletedAt    string
		ElapsedSeconds int
		RunNotes       string
		ActualSets     int
		ActualReps     int
		ActualWeightKg float64
		TemplateName   string
	}
	var rows []row
	s.store.DB.
		Table("run_exercise_completions").
		Select("run_exercise_completions.id, run_exercise_completions.status, "+
			"strftime('%Y-%m-%d', run_exercise_completions.completed_at) as completed_at, "+
			"run_exercise_completions.elapsed_seconds, run_exercise_completions.run_notes, "+
			"run_exercise_completions.actual_sets, run_exercise_completions.actual_reps, "+
			"run_exercise_completions.actual_weight_kg, "+
			"COALESCE(NULLIF(session_runs.custom_name,''), session_templates.name, 'Session') as template_name").
		Joins("JOIN session_runs ON session_runs.id = run_exercise_completions.run_id").
		Joins("JOIN scheduled_sessions ON scheduled_sessions.id = session_runs.scheduled_session_id").
		Joins("LEFT JOIN session_templates ON session_templates.id = scheduled_sessions.session_template_id").
		Where("run_exercise_completions.owner_id = ? AND run_exercise_completions.exercise_id IN ? "+
			"AND run_exercise_completions.deleted_at IS NULL AND session_runs.status = 'completed'",
			ownerID, exerciseIDs).
		Order("run_exercise_completions.completed_at DESC").
		Limit(limit).
		Scan(&rows)

	items := make([]pages.ExerciseHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, pages.ExerciseHistoryItem{
			Date:           formatHistoryDate(r.CompletedAt),
			TemplateName:   r.TemplateName,
			ActualSets:     r.ActualSets,
			ActualReps:     r.ActualReps,
			ActualWeightKg: r.ActualWeightKg,
			ElapsedSeconds: r.ElapsedSeconds,
			Notes:          r.RunNotes,
			Status:         r.Status,
		})
	}
	return items
}

func formatHistoryDate(yyyymmdd string) string {
	if len(yyyymmdd) < 10 {
		return yyyymmdd
	}
	// Parse "2006-01-02" → "Jan 2, 2006"
	y := yyyymmdd[0:4]
	m := yyyymmdd[5:7]
	d := yyyymmdd[8:10]
	months := [...]string{"", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	mi, _ := strconv.Atoi(m)
	di, _ := strconv.Atoi(d)
	if mi < 1 || mi > 12 {
		return yyyymmdd
	}
	return fmt.Sprintf("%s %d, %s", months[mi], di, y)
}

// handleExerciseHistoryHint serves the lazy-loaded history pill shown on the run page.
func (s *Server) handleExerciseHistoryHint(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	exID, err := strconv.ParseUint(chi.URLParam(r, "exerciseID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return
	}

	var ex db.Exercise
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, uint(exID)).First(&ex).Error; err != nil {
		// Return empty fragment silently (exercise may belong to another user).
		w.WriteHeader(http.StatusOK)
		return
	}

	items := s.exerciseHistoryItems(ex, ownerID, 3)
	s.pages.RenderFragment(w, "fragments/exercise_history_hint", pages.ExerciseHistoryHintView{
		ExerciseID:   uint(exID),
		ExerciseName: ex.Name,
		Items:        items,
	})
}

// handleExerciseHistoryPopup serves the full popup content loaded into the dialog.
func (s *Server) handleExerciseHistoryPopup(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	exID, err := strconv.ParseUint(chi.URLParam(r, "exerciseID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return
	}

	var ex db.Exercise
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, uint(exID)).First(&ex).Error; err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	items := s.exerciseHistoryItems(ex, ownerID, 10)
	libID := uint(0)
	if ex.LibraryExerciseID != nil {
		libID = *ex.LibraryExerciseID
	}
	s.pages.RenderFragment(w, "fragments/exercise_history_popup", pages.ExerciseHistoryPopupView{
		ExerciseID:        uint(exID),
		ExerciseName:      ex.Name,
		LibraryExerciseID: libID,
		Items:             items,
	})
}

// handleExerciseDivergenceHint checks whether recent actuals diverge from planned values.
func (s *Server) handleExerciseDivergenceHint(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		w.WriteHeader(http.StatusOK)
		return
	}
	exID, err := strconv.ParseUint(chi.URLParam(r, "exerciseID"), 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	var ex db.Exercise
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, uint(exID)).First(&ex).Error; err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	items := s.exerciseHistoryItems(ex, ownerID, 3)

	// Need at least 2 recent completions with actuals to detect divergence.
	var diverged []pages.ExerciseHistoryItem
	for _, it := range items {
		if it.Status == "completed" && it.HasActuals() {
			diverged = append(diverged, it)
		}
	}

	type hintData struct{ Show bool; Message string }

	if len(diverged) < 2 {
		s.pages.RenderFragment(w, "fragments/exercise_divergence_hint", hintData{})
		return
	}

	// Check if the last 2 completed sessions both differ from planned on the same variable.
	planned := struct{ sets, reps int; weight float64 }{ex.Sets, ex.Reps, ex.WeightKg}

	setsChanged := diverged[0].ActualSets != planned.sets && diverged[1].ActualSets != planned.sets &&
		diverged[0].ActualSets == diverged[1].ActualSets
	repsChanged := diverged[0].ActualReps != planned.reps && diverged[1].ActualReps != planned.reps &&
		diverged[0].ActualReps == diverged[1].ActualReps
	weightChanged := planned.weight > 0 &&
		diverged[0].ActualWeightKg != planned.weight && diverged[1].ActualWeightKg != planned.weight &&
		diverged[0].ActualWeightKg == diverged[1].ActualWeightKg

	var msg string
	switch {
	case weightChanged:
		msg = fmt.Sprintf("You've been doing %.1fkg — template says %.1fkg. Update it?", diverged[0].ActualWeightKg, planned.weight)
	case setsChanged && repsChanged:
		msg = fmt.Sprintf("You've been doing %d×%d — template says %d×%d. Update it?",
			diverged[0].ActualSets, diverged[0].ActualReps, planned.sets, planned.reps)
	case setsChanged:
		msg = fmt.Sprintf("You've been doing %d sets — template says %d. Update it?", diverged[0].ActualSets, planned.sets)
	case repsChanged:
		msg = fmt.Sprintf("You've been doing %d reps — template says %d. Update it?", diverged[0].ActualReps, planned.reps)
	}

	if msg == "" {
		s.pages.RenderFragment(w, "fragments/exercise_divergence_hint", hintData{})
		return
	}
	s.pages.RenderFragment(w, "fragments/exercise_divergence_hint", hintData{Show: true, Message: msg})
}

// handleExerciseLibraryHistory shows the full history page for a library exercise.
func (s *Server) handleExerciseLibraryHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	libID, err := strconv.ParseUint(chi.URLParam(r, "libraryExerciseID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return
	}

	var lib db.LibraryExercise
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, uint(libID)).First(&lib).Error; err != nil {
		http.NotFound(w, r)
		return
	}

	// Use a stub Exercise to reuse exerciseHistoryItems with LibraryExerciseID set.
	stub := db.Exercise{Name: lib.Name, LibraryExerciseID: func() *uint { v := uint(libID); return &v }()}
	items := s.exerciseHistoryItems(stub, ownerID, 100)

	s.pages.ExerciseHistoryPage(w, pages.ExerciseHistoryPageParams{
		Base:         pages.Base{CurrentUserEmail: s.currentUserEmail(r), Title: lib.Name + " · History"},
		ExerciseName: lib.Name,
		LibraryID:    uint(libID),
		Items:        items,
	})
}
