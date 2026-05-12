package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"passion/db"
	"passion/pages"
)

func (s *Server) handleTrainingCycles(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	var cycles []db.TrainingCycle
	err := s.store.DB.
		Where("owner_id = ?", ownerID).
		Order("id desc").
		Find(&cycles).Error
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.pages.TrainingCycleList(w, pages.TrainingCycleListParams{
		Base:           pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		TrainingCycles: cycles,
	})
}

func (s *Server) handleTrainingCyclesNew(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		templates, err := s.listTemplates(ownerID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.pages.NewTrainingCycle(w, pages.NewTrainingCycleParams{
			Base:      pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
			Templates: templates,
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

		startDateStr := strings.TrimSpace(r.FormValue("start_date"))
		if startDateStr == "" {
			http.Error(w, "start_date is required", http.StatusBadRequest)
			return
		}

		weeks, err := strconv.Atoi(strings.TrimSpace(r.FormValue("weeks")))
		if err != nil || weeks <= 0 {
			http.Error(w, "weeks must be a positive integer", http.StatusBadRequest)
			return
		}

		loc := time.Now().Location()
		startDate, err := time.ParseInLocation("2006-01-02", startDateStr, loc)
		if err != nil {
			http.Error(w, "invalid start_date", http.StatusBadRequest)
			return
		}
		startDate = localDate(startDate)

		templates, err := s.listTemplates(ownerID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		allowedTemplateIDs := map[uint]bool{}
		for _, t := range templates {
			allowedTemplateIDs[t.ID] = true
		}

		cycle := &db.TrainingCycle{
			OwnerID:   ownerID,
			Name:      name,
			StartDate: startDate,
			Weeks:     weeks,
		}
		if err := s.store.DB.Create(cycle).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Weekday params map to Mon=1..Sun=7
		mappings := []struct {
			weekday int
			key     string
		}{
			{1, "template_mon"},
			{2, "template_tue"},
			{3, "template_wed"},
			{4, "template_thu"},
			{5, "template_fri"},
			{6, "template_sat"},
			{7, "template_sun"},
		}

		var mappingRows []db.TrainingCycleWeekdayMapping
		for _, m := range mappings {
			raw := strings.TrimSpace(r.FormValue(m.key))
			if raw == "" {
				continue
			}
			id, err := strconv.ParseUint(raw, 10, 64)
			if err != nil || id == 0 {
				continue
			}
			if !allowedTemplateIDs[uint(id)] {
				continue
			}
			mappingRows = append(mappingRows, db.TrainingCycleWeekdayMapping{
				OwnerID:           ownerID,
				TrainingCycleID:   cycle.ID,
				Weekday:           m.weekday,
				SessionTemplateID: uint(id),
			})
		}

		if len(mappingRows) == 0 {
			http.Error(w, "select at least one template for a weekday", http.StatusBadRequest)
			return
		}

		if err := s.store.DB.Create(&mappingRows).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Generate scheduled sessions.
		week1Monday := mondayOfLocalDate(startDate)
		cycleID := cycle.ID
		for weekIdx := 0; weekIdx < weeks; weekIdx++ {
			for _, mr := range mappingRows {
				scheduled := week1Monday.AddDate(0, 0, weekIdx*7+(mr.Weekday-1))
				scheduled = localDate(scheduled)
				if scheduled.Before(startDate) {
					continue
				}
				ss := &db.ScheduledSession{
					OwnerID:           ownerID,
					TrainingCycleID:   &cycleID,
					ScheduledDate:     scheduled,
					SessionTemplateID: mr.SessionTemplateID,
				}
				if err := s.store.DB.Create(ss).Error; err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}

		http.Redirect(w, r, "/training-cycles/"+strconv.FormatUint(uint64(cycle.ID), 10), http.StatusSeeOther)
		return
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTrainingCyclesByID(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	cycleID, err := parseUintParam(r, "cycleID")
	if err != nil {
		http.Error(w, "invalid training cycle id", http.StatusBadRequest)
		return
	}

	action := chi.URLParam(r, "action")
	// Detail page: GET /training-cycles/{id}
	if action == "" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.renderTrainingCycleDetail(w, r, cycleID, ownerID)
		return
	}

	// Edit endpoints:
	// POST /training-cycles/{id}/move
	// POST /training-cycles/{id}/add
	// POST /training-cycles/{id}/remove
	if r.Method == http.MethodPost {
		switch action {
		case "move":
			s.handleTrainingCycleMove(w, r, cycleID, ownerID)
			return
		case "add":
			s.handleTrainingCycleAdd(w, r, cycleID, ownerID)
			return
		case "remove":
			s.handleTrainingCycleRemove(w, r, cycleID, ownerID)
			return
		case "override-save":
			s.handleCycleOverrideSave(w, r, cycleID, ownerID)
			return
		case "override-clear":
			s.handleCycleOverrideClear(w, r, cycleID, ownerID)
			return
		}
	}

	http.NotFound(w, r)
}

func (s *Server) renderTrainingCycleDetail(w http.ResponseWriter, r *http.Request, cycleID uint, ownerID uint) {
	var cycle db.TrainingCycle
	res := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, cycleID).
		Find(&cycle)
	if res.Error != nil {
		http.Error(w, res.Error.Error(), http.StatusInternalServerError)
		return
	}
	if res.RowsAffected == 0 {
		http.Error(w, "Training cycle not found", http.StatusNotFound)
		return
	}

	gridStart := mondayOfLocalDate(cycle.StartDate)
	gridEnd := gridStart.AddDate(0, 0, cycle.Weeks*7-1)

	// Load available templates for "Add" controls.
	templates, err := s.listTemplates(ownerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var scheduled []db.ScheduledSession
	if err := s.store.DB.
		Preload("SessionTemplate").
		Where("owner_id = ? AND training_cycle_id = ? AND scheduled_date >= ? AND scheduled_date <= ?",
			ownerID, cycleID, gridStart, gridEnd).
		Order("scheduled_date asc").
		Find(&scheduled).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessionsByDateKey := map[string]db.ScheduledSession{}
	for _, ss := range scheduled {
		key := localDateKey(ss.ScheduledDate)
		sessionsByDateKey[key] = ss
	}

	weekdayLabels := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	rows := make([]pages.CycleWeekRowView, 0, cycle.Weeks)
	for weekIdx := 0; weekIdx < cycle.Weeks; weekIdx++ {
		cells := make([]pages.CycleDayCellView, 0, 7)
		for dayIdx := 0; dayIdx < 7; dayIdx++ {
			d := gridStart.AddDate(0, 0, weekIdx*7+dayIdx)
			key := localDateKey(d)
			cell := pages.CycleDayCellView{
				DateKey:    key,
				DateLabel:  d.Format("Mon 02"),
				DayNum:     d.Format("2"),
				DayNumber:  d.Day(),
				IsWeekend:  d.Weekday() == time.Saturday || d.Weekday() == time.Sunday,
				HasSession: false,
			}
			if ss, ok := sessionsByDateKey[key]; ok {
				cell.HasSession = true
				cell.SessionID = ss.ID
				cell.SessionTemplateName = ss.SessionTemplate.Name
				cell.SessionTemplateColor = ss.SessionTemplate.Color
			}
			cells = append(cells, cell)
		}
		rows = append(rows, pages.CycleWeekRowView{
			WeekNumber: weekIdx + 1,
			Cells:      cells,
		})
	}

	// Load exercise targets for this cycle.
	exerciseOverrides := s.buildCycleExerciseOverrides(cycleID, ownerID, cycle.Weeks)

	s.pages.TrainingCycleDetail(w, pages.TrainingCycleDetailParams{
		Base:               pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		CycleID:            cycleID,
		CycleName:          cycle.Name,
		CycleWeeks:         cycle.Weeks,
		CycleWeekdayLabels: weekdayLabels,
		CycleTemplates:     templates,
		CycleRows:          rows,
		TotalScheduled:     len(scheduled),
		ExerciseOverrides:  exerciseOverrides,
	})
}

func (s *Server) handleTrainingCycleMove(w http.ResponseWriter, r *http.Request, cycleID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	scheduledSessionIDStr := strings.TrimSpace(r.FormValue("scheduled_session_id"))
	targetDateStr := strings.TrimSpace(r.FormValue("scheduled_date"))
	if scheduledSessionIDStr == "" || targetDateStr == "" {
		http.Error(w, "scheduled_session_id and scheduled_date are required", http.StatusBadRequest)
		return
	}

	scheduledSessionID, err := strconv.ParseUint(scheduledSessionIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid scheduled_session_id", http.StatusBadRequest)
		return
	}

	loc := time.Now().Location()
	parsed, err := time.ParseInLocation("2006-01-02", targetDateStr, loc)
	if err != nil {
		http.Error(w, "invalid scheduled_date", http.StatusBadRequest)
		return
	}
	parsed = localDate(parsed)

	var ss db.ScheduledSession
	if err := s.store.DB.
		Where("owner_id = ? AND id = ? AND training_cycle_id = ?", ownerID, scheduledSessionID, cycleID).
		First(&ss).Error; err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	ss.ScheduledDate = parsed
	if err := s.store.DB.Save(&ss).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/training-cycles/"+strconv.FormatUint(uint64(cycleID), 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleTrainingCycleAdd(w http.ResponseWriter, r *http.Request, cycleID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	targetDateStr := strings.TrimSpace(r.FormValue("scheduled_date"))
	templateIDStr := strings.TrimSpace(r.FormValue("session_template_id"))
	if targetDateStr == "" || templateIDStr == "" {
		http.Error(w, "scheduled_date and session_template_id are required", http.StatusBadRequest)
		return
	}

	templateID, err := strconv.ParseUint(templateIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid session_template_id", http.StatusBadRequest)
		return
	}

	var tpl db.SessionTemplate
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, templateID).
		First(&tpl).Error; err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	loc := time.Now().Location()
	parsed, err := time.ParseInLocation("2006-01-02", targetDateStr, loc)
	if err != nil {
		http.Error(w, "invalid scheduled_date", http.StatusBadRequest)
		return
	}
	parsed = localDate(parsed)

	// Replace if one already exists for that day.
	var existing db.ScheduledSession
	err = s.store.DB.
		Where("owner_id = ? AND training_cycle_id = ? AND scheduled_date = ?", ownerID, cycleID, parsed).
		First(&existing).Error
	if err == nil {
		existing.SessionTemplateID = uint(tpl.ID)
		if err := s.store.DB.Save(&existing).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("HX-Redirect", "/training-cycles/"+strconv.FormatUint(uint64(cycleID), 10))
		w.WriteHeader(http.StatusOK)
		return
	}

	ss := &db.ScheduledSession{
		OwnerID:           ownerID,
		TrainingCycleID:   &cycleID,
		ScheduledDate:     parsed,
		SessionTemplateID: uint(tpl.ID),
	}
	if err := s.store.DB.Create(ss).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/training-cycles/"+strconv.FormatUint(uint64(cycleID), 10))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleTrainingCycleRemove(w http.ResponseWriter, r *http.Request, cycleID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	scheduledSessionIDStr := strings.TrimSpace(r.FormValue("scheduled_session_id"))
	if scheduledSessionIDStr == "" {
		http.Error(w, "scheduled_session_id is required", http.StatusBadRequest)
		return
	}

	scheduledSessionID, err := strconv.ParseUint(scheduledSessionIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid scheduled_session_id", http.StatusBadRequest)
		return
	}

	// Ensure it belongs to the cycle.
	if err := s.store.DB.
		Where("owner_id = ? AND id = ? AND training_cycle_id = ?", ownerID, scheduledSessionID, cycleID).
		Delete(&db.ScheduledSession{}).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/training-cycles/"+strconv.FormatUint(uint64(cycleID), 10))
	w.WriteHeader(http.StatusOK)
}

// buildCycleExerciseOverrides collects all unique targetable exercises across
// all templates scheduled in the cycle, then merges in stored overrides and week overrides.
func (s *Server) buildCycleExerciseOverrides(cycleID uint, ownerID uint, cycleWeeks int) []pages.CycleExerciseOverrideView {
	var mappings []db.TrainingCycleWeekdayMapping
	s.store.DB.Where("training_cycle_id = ? AND owner_id = ?", cycleID, ownerID).Find(&mappings)
	templateIDs := map[uint]bool{}
	for _, m := range mappings {
		templateIDs[m.SessionTemplateID] = true
	}
	if len(templateIDs) == 0 {
		return nil
	}

	ids := make([]uint, 0, len(templateIDs))
	for id := range templateIDs {
		ids = append(ids, id)
	}

	var exercises []db.Exercise
	s.store.DB.
		Joins("JOIN activities ON activities.id = exercises.activity_id").
		Where("activities.session_template_id IN ? AND exercises.owner_id = ? "+
			"AND exercises.kind IN ('reps_and_sets','timed_reps','exercise_catalog') "+
			"AND exercises.parent_exercise_id IS NULL "+
			"AND exercises.deleted_at IS NULL AND activities.deleted_at IS NULL",
			ids, ownerID).
		Order("exercises.name asc").
		Find(&exercises)

	type exKey struct {
		libID uint
		name  string
	}
	seen := map[exKey]bool{}
	var unique []db.Exercise
	for _, ex := range exercises {
		var k exKey
		if ex.LibraryExerciseID != nil && *ex.LibraryExerciseID != 0 {
			k = exKey{libID: *ex.LibraryExerciseID}
		} else {
			k = exKey{name: ex.Name}
		}
		if !seen[k] {
			seen[k] = true
			unique = append(unique, ex)
		}
	}

	// Load cycle-level overrides.
	var overrides []db.CycleExerciseOverride
	s.store.DB.Where("training_cycle_id = ? AND owner_id = ?", cycleID, ownerID).Find(&overrides)
	overrideByLibID := map[uint]*db.CycleExerciseOverride{}
	overrideByName := map[string]*db.CycleExerciseOverride{}
	for i := range overrides {
		ov := &overrides[i]
		if ov.LibraryExerciseID != nil && *ov.LibraryExerciseID != 0 {
			overrideByLibID[*ov.LibraryExerciseID] = ov
		} else {
			overrideByName[ov.ExerciseName] = ov
		}
	}

	// Load week overrides and index them: libID/name → week → override.
	weekOverrides, _ := db.ListCycleExerciseWeekOverrides(s.store.DB, ownerID, cycleID)
	weekByLibID := map[uint]map[int]*db.CycleExerciseWeekOverride{}
	weekByName := map[string]map[int]*db.CycleExerciseWeekOverride{}
	for i := range weekOverrides {
		wo := &weekOverrides[i]
		if wo.LibraryExerciseID != nil && *wo.LibraryExerciseID != 0 {
			if weekByLibID[*wo.LibraryExerciseID] == nil {
				weekByLibID[*wo.LibraryExerciseID] = map[int]*db.CycleExerciseWeekOverride{}
			}
			weekByLibID[*wo.LibraryExerciseID][wo.Week] = wo
		} else {
			if weekByName[wo.ExerciseName] == nil {
				weekByName[wo.ExerciseName] = map[int]*db.CycleExerciseWeekOverride{}
			}
			weekByName[wo.ExerciseName][wo.Week] = wo
		}
	}

	result := make([]pages.CycleExerciseOverrideView, 0, len(unique))
	for _, ex := range unique {
		v := pages.CycleExerciseOverrideView{
			ExerciseName:    ex.Name,
			Kind:            ex.Kind,
			PlannedSets:     ex.Sets,
			PlannedReps:     ex.Reps,
			PlannedWeightKg: ex.WeightKg,
			PlannedRepSecs:  ex.RepSeconds,
		}
		if ex.LibraryExerciseID != nil && *ex.LibraryExerciseID != 0 {
			v.LibraryExerciseID = *ex.LibraryExerciseID
		}

		var ov *db.CycleExerciseOverride
		if v.LibraryExerciseID != 0 {
			ov = overrideByLibID[v.LibraryExerciseID]
		} else {
			ov = overrideByName[ex.Name]
		}
		if ov != nil {
			v.HasOverride = true
			v.OverrideSets = ov.Sets
			v.OverrideReps = ov.Reps
			v.OverrideWeightKg = ov.WeightKg
			v.OverrideRepSecs = ov.RepSeconds
			v.VariesByWeek = ov.VariesByWeek
		}

		// Resolve per-week views.
		var weekMap map[int]*db.CycleExerciseWeekOverride
		if v.LibraryExerciseID != 0 {
			weekMap = weekByLibID[v.LibraryExerciseID]
		} else {
			weekMap = weekByName[ex.Name]
		}
		// Fallback values: cycle override if set, else template default.
		fbSets, fbReps, fbWeight, fbSecs := v.PlannedSets, v.PlannedReps, v.PlannedWeightKg, v.PlannedRepSecs
		if v.HasOverride {
			fbSets, fbReps, fbWeight, fbSecs = v.OverrideSets, v.OverrideReps, v.OverrideWeightKg, v.OverrideRepSecs
		}
		v.WeekOverrides = make([]pages.CycleWeekTargetView, cycleWeeks)
		for i := 0; i < cycleWeeks; i++ {
			wv := pages.CycleWeekTargetView{
				Week:     i + 1,
				Sets:     fbSets,
				Reps:     fbReps,
				WeightKg: fbWeight,
				RepSeconds: fbSecs,
			}
			if wo := weekMap[i+1]; wo != nil {
				wv.Sets = wo.Sets
				wv.Reps = wo.Reps
				wv.WeightKg = wo.WeightKg
				wv.RepSeconds = wo.RepSeconds
				wv.HasOverride = true
			}
			v.WeekOverrides[i] = wv
		}

		result = append(result, v)
	}
	return result
}

// handleCycleOverrideSave upserts a CycleExerciseOverride for one exercise (silent auto-save).
func (s *Server) handleCycleOverrideSave(w http.ResponseWriter, r *http.Request, cycleID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	exName := strings.TrimSpace(r.FormValue("exercise_name"))
	if exName == "" {
		http.Error(w, "exercise_name required", http.StatusBadRequest)
		return
	}
	libIDRaw, _ := strconv.ParseUint(strings.TrimSpace(r.FormValue("library_exercise_id")), 10, 64)
	sets, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("sets")))
	reps, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("reps")))
	weightKg, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("weight_kg")), 64)
	repSecs, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("rep_seconds")))

	var existing db.CycleExerciseOverride
	q := s.store.DB.Where("training_cycle_id = ? AND owner_id = ?", cycleID, ownerID)
	if libIDRaw != 0 {
		q = q.Where("library_exercise_id = ?", libIDRaw)
	} else {
		q = q.Where("library_exercise_id IS NULL AND exercise_name = ?", exName)
	}
	var libIDPtr *uint
	if libIDRaw != 0 {
		v := uint(libIDRaw)
		libIDPtr = &v
	}
	if err := q.First(&existing).Error; err != nil {
		s.store.DB.Create(&db.CycleExerciseOverride{
			OwnerID: ownerID, TrainingCycleID: cycleID,
			LibraryExerciseID: libIDPtr, ExerciseName: exName,
			Sets: sets, Reps: reps, WeightKg: weightKg, RepSeconds: repSecs,
		})
	} else {
		existing.Sets = sets
		existing.Reps = reps
		existing.WeightKg = weightKg
		existing.RepSeconds = repSecs
		s.store.DB.Save(&existing)
	}
	w.WriteHeader(http.StatusOK)
}

// handleCycleOverrideClear deletes the CycleExerciseOverride and all week overrides for one exercise.
func (s *Server) handleCycleOverrideClear(w http.ResponseWriter, r *http.Request, cycleID uint, ownerID uint) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	exName := strings.TrimSpace(r.FormValue("exercise_name"))
	libIDRaw, _ := strconv.ParseUint(strings.TrimSpace(r.FormValue("library_exercise_id")), 10, 64)

	var libIDPtr *uint
	if libIDRaw != 0 {
		v := uint(libIDRaw)
		libIDPtr = &v
	}

	q := s.store.DB.Where("training_cycle_id = ? AND owner_id = ?", cycleID, ownerID)
	if libIDRaw != 0 {
		q = q.Where("library_exercise_id = ?", libIDRaw)
	} else {
		q = q.Where("library_exercise_id IS NULL AND exercise_name = ?", exName)
	}
	q.Delete(&db.CycleExerciseOverride{})
	db.DeleteCycleExerciseWeekOverridesForExercise(s.store.DB, ownerID, cycleID, libIDPtr, exName)

	w.Header().Set("HX-Redirect", "/training-cycles/"+strconv.FormatUint(uint64(cycleID), 10))
	w.WriteHeader(http.StatusOK)
}

// handleCycleWeekOverrideSave upserts a CycleExerciseWeekOverride for one exercise+week (silent auto-save).
// POST /training-cycles/{cycleID}/week-override-save
func (s *Server) handleCycleWeekOverrideSave(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cycleID, err := parseUintParam(r, "cycleID")
	if err != nil {
		http.Error(w, "invalid cycleID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	exName := strings.TrimSpace(r.FormValue("exercise_name"))
	week, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("week")))
	libIDRaw, _ := strconv.ParseUint(strings.TrimSpace(r.FormValue("library_exercise_id")), 10, 64)
	sets, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("sets")))
	reps, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("reps")))
	weightKg, _ := strconv.ParseFloat(strings.TrimSpace(r.FormValue("weight_kg")), 64)
	repSecs, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("rep_seconds")))

	if exName == "" || week < 1 {
		http.Error(w, "exercise_name and week required", http.StatusBadRequest)
		return
	}
	var libIDPtr *uint
	if libIDRaw != 0 {
		v := uint(libIDRaw)
		libIDPtr = &v
	}
	db.UpsertCycleExerciseWeekOverride(s.store.DB, &db.CycleExerciseWeekOverride{
		OwnerID: ownerID, TrainingCycleID: cycleID, Week: week,
		ExerciseName: exName, LibraryExerciseID: libIDPtr,
		Sets: sets, Reps: reps, WeightKg: weightKg, RepSeconds: repSecs,
	})
	w.WriteHeader(http.StatusOK)
}

// handleCycleWeekOverrideToggle switches an exercise between "same every week" and "varies by week".
// POST /training-cycles/{cycleID}/week-override-toggle
func (s *Server) handleCycleWeekOverrideToggle(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := s.currentUserID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cycleID, err := parseUintParam(r, "cycleID")
	if err != nil {
		http.Error(w, "invalid cycleID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	exName := strings.TrimSpace(r.FormValue("exercise_name"))
	libIDRaw, _ := strconv.ParseUint(strings.TrimSpace(r.FormValue("library_exercise_id")), 10, 64)
	mode := strings.TrimSpace(r.FormValue("mode")) // "varies" or "same"

	var libIDPtr *uint
	if libIDRaw != 0 {
		v := uint(libIDRaw)
		libIDPtr = &v
	}

	// Find or create the cycle-level override to set VariesByWeek flag.
	var existing db.CycleExerciseOverride
	q := s.store.DB.Where("training_cycle_id = ? AND owner_id = ?", cycleID, ownerID)
	if libIDRaw != 0 {
		q = q.Where("library_exercise_id = ?", libIDRaw)
	} else {
		q = q.Where("library_exercise_id IS NULL AND exercise_name = ?", exName)
	}
	if err := q.First(&existing).Error; err != nil {
		existing = db.CycleExerciseOverride{
			OwnerID: ownerID, TrainingCycleID: cycleID,
			LibraryExerciseID: libIDPtr, ExerciseName: exName,
		}
		s.store.DB.Create(&existing)
	}
	existing.VariesByWeek = mode == "varies"
	s.store.DB.Save(&existing)

	if mode == "same" {
		db.DeleteCycleExerciseWeekOverridesForExercise(s.store.DB, ownerID, cycleID, libIDPtr, exName)
	}

	w.Header().Set("HX-Redirect", "/training-cycles/"+strconv.FormatUint(uint64(cycleID), 10))
	w.WriteHeader(http.StatusOK)
}
