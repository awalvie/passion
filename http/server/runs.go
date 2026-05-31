package web

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"passion/db"
	"passion/pages"
)

// RunStep, RunStepOption, and RunActivityGroup are defined in passion/pages.

func (s *Server) handleRunsByID(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	runID, err := parseUintParam(r, "runID")
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}

	exerciseParam := chi.URLParam(r, "exerciseID")
	// GET /runs/{runID}
	if exerciseParam == "" {
		if r.Method != http.MethodGet {
			s.methodNotAllowed(w)
			return
		}
		s.renderRun(w, r, runID, ownerID)
		return
	}

	// POST /runs/{runID}/exercises/{exerciseID}/complete|skip
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	exerciseID64, err := strconv.ParseUint(exerciseParam, 10, 64)
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return
	}
	action := "complete"
	if strings.HasSuffix(r.URL.Path, "/skip") {
		action = "skip"
	}
	if err := s.completeRunExercise(w, r, runID, uint(exerciseID64), action, ownerID); err != nil {
		s.badRequest(w, "bad request")
		return
	}
	return
}

func (s *Server) renderRun(w http.ResponseWriter, r *http.Request, runID uint, ownerID uint) {
	var run db.SessionRun
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, runID).
		First(&run).Error; err != nil {
		s.notFound(w)
		return
	}

	// Open sessions get their own builder/manager view.
	if run.IsOpen && r.URL.Query().Get("exercise") == "" {
		s.renderOpenSession(w, r, run, ownerID)
		return
	}

	ss, err := db.GetScheduledSessionWithTemplate(s.store.DB, ownerID, run.ScheduledSessionID)
	if err != nil {
		s.dbError(w, r, err)
		return
	}

	steps := s.buildRunSteps(ss, runID, ownerID)
	if run.IsOpen {
		steps = append(steps, s.loadOpenSteps(runID)...)
	}

	var completions []db.RunExerciseCompletion
	if err := s.store.DB.
		Where("owner_id = ? AND run_id = ?", ownerID, runID).
		Find(&completions).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	statusByID := map[uint]string{}
	for _, c := range completions {
		statusByID[c.ExerciseID] = c.Status
	}
	for i := range steps {
		st := &steps[i]
		status := statusByID[st.ExerciseID]
		if status == "" {
			st.Status = "pending"
		} else {
			st.Status = status
		}
	}

	nextIdx := -1
	for i, st := range steps {
		if st.Status == "pending" {
			nextIdx = i
			break
		}
	}

	// Allow jumping to a specific pending exercise via ?exercise=ID.
	if jumpParam := r.URL.Query().Get("exercise"); jumpParam != "" {
		if jumpID, err := strconv.ParseUint(jumpParam, 10, 64); err == nil {
			for i, st := range steps {
				if st.ExerciseID == uint(jumpID) && st.Status == "pending" {
					nextIdx = i
					break
				}
			}
		}
	}

	runCompleted := nextIdx == -1
	currentStepNum := 0
	var currentStep pages.RunStep
	if !runCompleted {
		currentStepNum = nextIdx + 1 // 1-indexed
		currentStep = steps[nextIdx]
	}

	// Build activity groups for sidebar display.
	activityGroups := buildActivityGroups(steps, currentStep.ExerciseID)

	displayName := ss.SessionTemplate.Name
	if run.CustomName != "" {
		displayName = run.CustomName
	}

	var libExercises []db.LibraryExercise
	var activityTemplates []db.ActivityTemplate
	if run.IsOpen {
		var err error
		libExercises, err = db.ListLibraryExercises(s.store.DB, ownerID)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
		activityTemplates, err = db.ListActivityTemplatesWithExercises(s.store.DB, ownerID)
		if err != nil {
			s.serverError(w, r, err)
			return
		}
	}

	s.pages.Run(w, pages.RunParams{
		Base:                 pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		RunID:                runID,
		RunTemplateName:      displayName,
		RunTotalSteps:        len(steps),
		RunCompleted:         runCompleted,
		RunCurrentStepNum:    currentStepNum,
		RunSessionSeconds:    sumElapsedSeconds(completions),
		RunIsTrial:           run.IsTrial,
		RunTemplateID:        ss.SessionTemplate.ID,
		RunIsOpen:            run.IsOpen,
		RunCustomName:        run.CustomName,
		RunLibraryExercises:  libExercises,
		RunActivityTemplates: activityTemplates,
		CurrentStep:          currentStep,
		RunSteps:             steps,
		RunActivityGroups:    activityGroups,
	})
}

func (s *Server) renderOpenSession(w http.ResponseWriter, r *http.Request, run db.SessionRun, ownerID uint) {
	runID := run.ID
	steps := s.loadOpenSteps(runID)

	var completions []db.RunExerciseCompletion
	if err := s.store.DB.
		Where("owner_id = ? AND run_id = ?", ownerID, runID).
		Find(&completions).Error; err != nil {
		s.serverError(w, r, err)
		return
	}

	type compInfo struct {
		status  string
		elapsed int
		notes   string
	}
	compByID := map[uint]compInfo{}
	for _, c := range completions {
		compByID[c.ExerciseID] = compInfo{status: c.Status, elapsed: c.ElapsedSeconds, notes: c.RunNotes}
	}
	for i := range steps {
		st := &steps[i]
		if info, ok := compByID[st.ExerciseID]; ok {
			st.Status = info.status
			st.ElapsedSeconds = info.elapsed
			st.RunNotes = info.notes
		} else {
			st.Status = "pending"
		}
	}

	libExercises, err := db.ListLibraryExercises(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	activityTemplates, err := db.ListActivityTemplatesWithExercises(s.store.DB, ownerID)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	displayName := run.CustomName
	if displayName == "" {
		displayName = "Open Session"
	}

	var startedAtUnix int64
	if !run.StartedAt.IsZero() {
		startedAtUnix = run.StartedAt.Unix()
	}

	s.pages.OpenSession(w, pages.RunParams{
		Base:                 pages.Base{CurrentUserEmail: s.currentUserEmail(r)},
		RunID:                runID,
		RunTemplateName:      displayName,
		RunTotalSteps:        len(steps),
		RunIsOpen:            true,
		RunIsDraft:           run.Status == db.RunStatusDraft,
		RunCustomName:        run.CustomName,
		StartedAtUnix:        startedAtUnix,
		RunLibraryExercises:  libExercises,
		RunActivityTemplates: activityTemplates,
		RunSteps:             steps,
	})
}

func (s *Server) completeRunExercise(w http.ResponseWriter, r *http.Request, runID uint, exerciseID uint, action string, ownerID uint) error {
	// Load run and ensure ownership.
	var run db.SessionRun
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, runID).
		First(&run).Error; err != nil {
		return err
	}

	// Determine the next expected exercise.
	ss, err := db.GetScheduledSessionWithTemplate(s.store.DB, ownerID, run.ScheduledSessionID)
	if err != nil {
		return err
	}

	steps := s.buildRunSteps(ss, runID, ownerID)
	if run.IsOpen {
		steps = append(steps, s.loadOpenSteps(runID)...)
	}

	var completions []db.RunExerciseCompletion
	if err := s.store.DB.
		Where("owner_id = ? AND run_id = ?", ownerID, runID).
		Find(&completions).Error; err != nil {
		return err
	}

	completedByID := map[uint]bool{}
	for _, c := range completions {
		completedByID[c.ExerciseID] = true
	}

	// Allow completing/skipping any pending exercise, not just the strict next one.
	if completedByID[exerciseID] {
		return errors.New("exercise already completed")
	}
	validExercise := false
	for _, st := range steps {
		if st.ExerciseID == exerciseID {
			validExercise = true
			break
		}
	}
	if !validExercise {
		return errors.New("exercise not found in this run")
	}

	if err := r.ParseForm(); err != nil {
		return err
	}

	runNotes := strings.TrimSpace(r.FormValue("run_notes"))
	elapsedSecondsStr := strings.TrimSpace(r.FormValue("elapsed_seconds"))
	elapsedSeconds, _ := strconv.Atoi(elapsedSecondsStr)
	if elapsedSeconds < 0 {
		elapsedSeconds = 0
	}

	status := db.RunStatusCompleted
	if action == "skip" {
		status = db.RunStatusSkipped
	}

	// Parse per-set actuals (set_reps_1, set_weight_kg_1, …) when present.
	// These are submitted when the exercise has planned sets configured.
	actualSets := formInt(r, "actual_sets")
	actualReps := formInt(r, "actual_reps")
	actualWeightKg := formFloat(r, "actual_weight_kg")

	if status == db.RunStatusCompleted {
		var perSetReps []int
		var perSetWeights []float64
		for i := 1; ; i++ {
			repsKey := "set_reps_" + strconv.Itoa(i)
			weightKey := "set_weight_kg_" + strconv.Itoa(i)
			repsVal := strings.TrimSpace(r.FormValue(repsKey))
			weightVal := strings.TrimSpace(r.FormValue(weightKey))
			if repsVal == "" && weightVal == "" {
				break
			}
			reps, _ := strconv.Atoi(repsVal)
			weight, _ := strconv.ParseFloat(weightVal, 64)
			perSetReps = append(perSetReps, reps)
			perSetWeights = append(perSetWeights, weight)
		}
		if len(perSetReps) > 0 {
			for i, reps := range perSetReps {
				_ = db.UpsertManualExerciseSetLog(s.store.DB, ownerID, runID, exerciseID, i+1, reps, perSetWeights[i])
			}
			// Aggregate: total sets, avg reps, max weight.
			actualSets = len(perSetReps)
			totalReps := 0
			maxWeight := 0.0
			for i, reps := range perSetReps {
				totalReps += reps
				if perSetWeights[i] > maxWeight {
					maxWeight = perSetWeights[i]
				}
			}
			if actualSets > 0 {
				actualReps = totalReps / actualSets
			}
			actualWeightKg = maxWeight
		}
	}

	comp := &db.RunExerciseCompletion{
		OwnerID:        ownerID,
		RunID:          runID,
		ExerciseID:     exerciseID,
		Status:         status,
		CompletedAt:    time.Now(),
		ElapsedSeconds: elapsedSeconds,
		RunNotes:       runNotes,
		ActualSets:     actualSets,
		ActualReps:     actualReps,
		ActualWeightKg: actualWeightKg,
	}
	if err := s.store.DB.Create(comp).Error; err != nil {
		return err
	}

	// If there are no more incomplete steps, mark the run as completed.
	// We keep it simple: re-check after inserting.
	var after []db.RunExerciseCompletion
	if err := s.store.DB.
		Where("owner_id = ? AND run_id = ?", ownerID, runID).
		Find(&after).Error; err != nil {
		return err
	}
	done := map[uint]bool{}
	for _, c := range after {
		done[c.ExerciseID] = true
	}
	anyIncomplete := false
	for _, st := range steps {
		if !done[st.ExerciseID] {
			anyIncomplete = true
			break
		}
	}
	if !anyIncomplete && !run.IsOpen {
		now := time.Now()
		run.Status = db.RunStatusCompleted
		run.CompletedAt = &now
		if err := s.store.DB.Save(&run).Error; err != nil {
			return err
		}
	}

	runIDStr := strconv.FormatUint(uint64(runID), 10)
	redirect := "/runs/" + runIDStr + "?t=" + strconv.FormatInt(time.Now().UnixNano(), 10) + "#run-current-step"
	if run.IsOpen {
		redirect = "/runs/" + runIDStr
	}
	w.Header().Set("HX-Redirect", redirect)
	w.WriteHeader(http.StatusOK)
	return nil
}

func (s *Server) handleRunExerciseChoose(w http.ResponseWriter, r *http.Request) {
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
	parentID, err := parseUintParam(r, "exerciseID")
	if err != nil {
		http.Error(w, "invalid exercise id", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	rawIDs := r.Form["child_exercise_ids[]"]
	if len(rawIDs) == 0 {
		http.Error(w, "no exercises selected", http.StatusBadRequest)
		return
	}
	childIDs := make([]uint, 0, len(rawIDs))
	for _, raw := range rawIDs {
		v, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
		if err != nil || v == 0 {
			http.Error(w, "invalid child_exercise_ids[]", http.StatusBadRequest)
			return
		}
		childIDs = append(childIDs, uint(v))
	}

	var run db.SessionRun
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, runID).
		First(&run).Error; err != nil {
		s.notFound(w)
		return
	}

	ss, err := db.GetScheduledSessionWithTemplate(s.store.DB, ownerID, run.ScheduledSessionID)
	if err != nil {
		s.dbError(w, r, err)
		return
	}

	// Build an index of all exercises for fast lookup.
	exerciseByID := map[uint]*db.Exercise{}
	for i := range ss.SessionTemplate.Activities {
		act := &ss.SessionTemplate.Activities[i]
		for j := range act.Exercises {
			ex := &act.Exercises[j]
			exerciseByID[ex.ID] = ex
		}
	}
	parent := exerciseByID[parentID]
	if parent == nil || strings.TrimSpace(parent.Kind) != "exercise_catalog" {
		http.Error(w, "invalid exercise catalog parent", http.StatusBadRequest)
		return
	}
	// Validate every selected child belongs to this parent.
	for _, cid := range childIDs {
		child := exerciseByID[cid]
		if child == nil || child.ParentExerciseID == nil || *child.ParentExerciseID != parentID {
			http.Error(w, "invalid child option", http.StatusBadRequest)
			return
		}
	}

	steps := s.buildRunSteps(ss, runID, ownerID)
	var completions []db.RunExerciseCompletion
	if err := s.store.DB.
		Where("owner_id = ? AND run_id = ?", ownerID, runID).
		Find(&completions).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	completedByID := map[uint]bool{}
	for _, c := range completions {
		completedByID[c.ExerciseID] = true
	}
	// Allow choosing for any pending catalog exercise, not just the strict next one.
	foundCatalog := false
	for _, st := range steps {
		if st.ExerciseID == parentID && st.Kind == "exercise_catalog" && !completedByID[st.ExerciseID] {
			foundCatalog = true
			break
		}
	}
	if !foundCatalog {
		http.Error(w, "exercise is not a pending catalog step", http.StatusBadRequest)
		return
	}

	// Clear previous selections and insert new ones.
	if err := s.store.DB.
		Where("owner_id = ? AND run_id = ? AND parent_exercise_id = ?", ownerID, runID, parentID).
		Delete(&db.RunExerciseChoice{}).Error; err != nil {
		s.serverError(w, r, err)
		return
	}
	for _, cid := range childIDs {
		row := &db.RunExerciseChoice{
			OwnerID:          ownerID,
			RunID:            runID,
			ParentExerciseID: parentID,
			ChosenExerciseID: cid,
		}
		if err := s.store.DB.Create(row).Error; err != nil {
			s.serverError(w, r, err)
			return
		}
	}

	w.Header().Set("HX-Redirect", "/runs/"+strconv.FormatUint(uint64(runID), 10)+"?t="+strconv.FormatInt(time.Now().UnixNano(), 10)+"#run-current-step")
	w.WriteHeader(http.StatusOK)
}

// loadOpenSteps returns run-scoped exercises for an open session, ordered by order_index.
func (s *Server) loadOpenSteps(runID uint) []pages.RunStep {
	var exercises []db.Exercise
	s.store.DB.
		Where("session_run_id = ? AND parent_exercise_id IS NULL", runID).
		Order("order_index asc").
		Find(&exercises)
	steps := make([]pages.RunStep, 0, len(exercises))
	for _, ex := range exercises {
		st := exerciseToRunStep(ex)
		st.ActivityName = "Exercises"
		st.PlannedSets = loadPlannedSetViews(s.store.DB, ex.ID)
		steps = append(steps, st)
	}
	return steps
}

func exerciseToRunStep(ex db.Exercise) pages.RunStep {
	return pages.RunStep{
		ExerciseID:             ex.ID,
		Name:                   ex.Name,
		Media:                  ex.Media,
		Kind:                   ex.Kind,
		SessionDurationSeconds: ex.SessionDurationSeconds,
		Sets:                   ex.Sets,
		Reps:                   ex.Reps,
		WeightKg:               ex.WeightKg,
		RepSeconds:             ex.RepSeconds,
		RepRestSeconds:         ex.RepRestSeconds,
		SetRestSeconds:         ex.SetRestSeconds,
		PrepSeconds:            ex.PrepSeconds,
		RungSeconds:            ex.RungSeconds,
		TemplateNotes:          ex.Notes,
		Status:                 "pending",
	}
}

func loadPlannedSetViews(gdb *gorm.DB, exerciseID uint) []pages.ExercisePlannedSetView {
	rows, _ := db.ListExercisePlannedSets(gdb, exerciseID)
	if len(rows) == 0 {
		return nil
	}
	views := make([]pages.ExercisePlannedSetView, len(rows))
	for i, r := range rows {
		views[i] = pages.ExercisePlannedSetView{SetIndex: r.SetIndex, Reps: r.Reps, WeightKg: r.WeightKg}
	}
	return views
}

func catalogChildren(all []db.Exercise, parentID uint) []db.Exercise {
	var out []db.Exercise
	for i := range all {
		ex := all[i]
		if ex.ParentExerciseID != nil && *ex.ParentExerciseID == parentID {
			out = append(out, ex)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].OrderIndex < out[j].OrderIndex
	})
	return out
}

func catalogMenuRunStep(parent db.Exercise, children []db.Exercise) pages.RunStep {
	st := exerciseToRunStep(parent)
	st.Kind = "exercise_catalog"
	st.CatalogOptions = make([]pages.RunStepOption, 0, len(children))
	for _, c := range children {
		st.CatalogOptions = append(st.CatalogOptions, pages.RunStepOption{
			ExerciseID:             c.ID,
			Name:                   c.Name,
			Media:                  c.Media,
			Kind:                   c.Kind,
			SessionDurationSeconds: c.SessionDurationSeconds,
			Sets:                   c.Sets,
			Reps:                   c.Reps,
			Notes:                  c.Notes,
		})
	}
	return st
}

func (s *Server) buildRunSteps(ss db.ScheduledSession, runID uint, ownerID uint) []pages.RunStep {
	var choices []db.RunExerciseChoice
	if err := s.store.DB.
		Where("owner_id = ? AND run_id = ?", ownerID, runID).
		Find(&choices).Error; err != nil {
		s.logger.Error("failed to load run exercise choices", "run_id", runID, "error", err)
	}
	// Map parent → list of chosen child IDs (preserving insertion order).
	choicesByParent := map[uint][]uint{}
	for _, c := range choices {
		choicesByParent[c.ParentExerciseID] = append(choicesByParent[c.ParentExerciseID], c.ChosenExerciseID)
	}

	// Load cycle overrides when this run belongs to a cycle.
	type overrideKey struct {
		libID uint
		name  string
	}
	overrideMap := map[overrideKey]*db.CycleExerciseOverride{}
	if ss.TrainingCycleID != nil && *ss.TrainingCycleID != 0 {
		var overrides []db.CycleExerciseOverride
		s.store.DB.Where("training_cycle_id = ? AND owner_id = ?", *ss.TrainingCycleID, ownerID).Find(&overrides)
		for i := range overrides {
			ov := &overrides[i]
			if ov.LibraryExerciseID != nil && *ov.LibraryExerciseID != 0 {
				overrideMap[overrideKey{libID: *ov.LibraryExerciseID}] = ov
			} else {
				overrideMap[overrideKey{name: ov.ExerciseName}] = ov
			}
		}
	}
	applyOverride := func(st pages.RunStep, ex db.Exercise) pages.RunStep {
		var ov *db.CycleExerciseOverride
		if ex.LibraryExerciseID != nil && *ex.LibraryExerciseID != 0 {
			ov = overrideMap[overrideKey{libID: *ex.LibraryExerciseID}]
		}
		if ov == nil {
			ov = overrideMap[overrideKey{name: ex.Name}]
		}
		if ov == nil {
			return st
		}
		if ov.Sets > 0 {
			st.Sets = ov.Sets
		}
		if ov.Reps > 0 {
			st.Reps = ov.Reps
		}
		if ov.WeightKg > 0 {
			st.WeightKg = ov.WeightKg
		}
		if ov.RepSeconds > 0 {
			st.RepSeconds = ov.RepSeconds
		}
		return st
	}

	var completions []db.RunExerciseCompletion
	if err := s.store.DB.
		Where("owner_id = ? AND run_id = ?", ownerID, runID).
		Find(&completions).Error; err != nil {
		s.logger.Error("failed to load run exercise completions", "run_id", runID, "error", err)
	}
	completionByID := map[uint]string{}
	for _, c := range completions {
		completionByID[c.ExerciseID] = c.Status
	}

	steps := make([]pages.RunStep, 0)
	for _, act := range ss.SessionTemplate.Activities {
		actName := act.Name
		if actName == "" {
			if act.Type != "" {
				actName = strings.ToUpper(act.Type[:1]) + act.Type[1:]
			} else {
				actName = "Activity"
			}
		}
		for _, ex := range act.Exercises {
			if ex.ParentExerciseID != nil && *ex.ParentExerciseID != 0 {
				continue
			}
			if ex.Kind == "exercise_catalog" {
				children := catalogChildren(act.Exercises, ex.ID)
				if childIDs, ok := choicesByParent[ex.ID]; ok && len(childIDs) > 0 {
					// Expand chosen children into individual steps.
					for _, childID := range childIDs {
						var chosen *db.Exercise
						for i := range act.Exercises {
							if act.Exercises[i].ID == childID &&
								act.Exercises[i].ParentExerciseID != nil &&
								*act.Exercises[i].ParentExerciseID == ex.ID {
								chosen = &act.Exercises[i]
								break
							}
						}
						if chosen != nil {
							st := applyOverride(exerciseToRunStep(*chosen), *chosen)
							st.ActivityID = act.ID
							st.ActivityName = actName
							st.PlannedSets = loadPlannedSetViews(s.store.DB, chosen.ID)
							steps = append(steps, st)
						}
					}
					continue
				}
				if st := completionByID[ex.ID]; st == db.RunStatusSkipped || st == db.RunStatusCompleted {
					continue
				}
				catStep := catalogMenuRunStep(ex, children)
				catStep.ActivityID = act.ID
				catStep.ActivityName = actName
				steps = append(steps, catStep)
				continue
			}
			st := applyOverride(exerciseToRunStep(ex), ex)
			st.ActivityID = act.ID
			st.ActivityName = actName
			st.PlannedSets = loadPlannedSetViews(s.store.DB, ex.ID)
			steps = append(steps, st)
		}
	}
	return steps
}

// buildActivityGroups groups steps by ActivityID for sidebar display.
// The group containing currentExerciseID is marked IsCurrent.
func buildActivityGroups(steps []pages.RunStep, currentExerciseID uint) []pages.RunActivityGroup {
	var groups []pages.RunActivityGroup
	for _, st := range steps {
		// Append to current group if same activity, otherwise start a new one.
		if len(groups) == 0 || groups[len(groups)-1].ActivityID != st.ActivityID {
			groups = append(groups, pages.RunActivityGroup{
				ActivityID: st.ActivityID,
				Name:       st.ActivityName,
			})
		}
		groups[len(groups)-1].Steps = append(groups[len(groups)-1].Steps, st)
	}
	for i := range groups {
		for _, st := range groups[i].Steps {
			if st.ExerciseID == currentExerciseID {
				groups[i].IsCurrent = true
				break
			}
		}
	}
	return groups
}

func sumElapsedSeconds(completions []db.RunExerciseCompletion) int {
	total := 0
	for _, c := range completions {
		if c.ElapsedSeconds > 0 {
			total += c.ElapsedSeconds
		}
	}
	return total
}

