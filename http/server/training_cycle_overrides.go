package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"passion/db"
	"passion/pages"
)

// cycleRedirectTarget picks where an override edit should send the browser. These
// handlers answer with HX-Redirect to reload the page, which is fine on the cycle page
// but would bounce the owner off the standalone targets page mid-edit. HTMX sends the
// current page in HX-Current-URL, so stay put when the edit came from there.
func cycleRedirectTarget(r *http.Request, cycleID uint) string {
	base := "/training-cycles/" + strconv.FormatUint(uint64(cycleID), 10)
	if cur := r.Header.Get("HX-Current-URL"); cur != "" {
		if u, err := url.Parse(cur); err == nil && strings.HasSuffix(u.Path, "/targets") {
			return base + "/targets"
		}
	}
	return base
}

// cycleTemplateExercise returns the template exercise a cycle target is derived from
// — the same row buildCycleExerciseOverrides reads planned sets/reps/weight from. Used
// to seed a newly created override row so it never starts life all-zero.
func (s *Server) cycleTemplateExercise(cycleID, ownerID uint, libID *uint, name string) (db.Exercise, bool) {
	var mappings []db.TrainingCycleWeekdayMapping
	s.store.DB.Where("training_cycle_id = ? AND owner_id = ?", cycleID, ownerID).Find(&mappings)
	if len(mappings) == 0 {
		return db.Exercise{}, false
	}
	ids := make([]uint, 0, len(mappings))
	for _, m := range mappings {
		ids = append(ids, m.SessionTemplateID)
	}

	q := s.store.DB.
		Joins("JOIN activities ON activities.id = exercises.activity_id").
		Where("activities.session_template_id IN ? AND exercises.owner_id = ? "+
			"AND exercises.kind IN ('reps_and_sets','timed_reps') "+
			"AND exercises.parent_exercise_id IS NULL "+
			"AND exercises.deleted_at IS NULL AND activities.deleted_at IS NULL",
			ids, ownerID)
	if libID != nil && *libID != 0 {
		q = q.Where("exercises.library_exercise_id = ?", *libID)
	} else {
		q = q.Where("exercises.name = ?", name)
	}

	var ex db.Exercise
	if err := q.Order("exercises.id asc").First(&ex).Error; err != nil {
		return db.Exercise{}, false
	}
	return ex, true
}

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
			"AND exercises.kind IN ('reps_and_sets','timed_reps') "+
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
		// Fallback values: cycle override if set, else template default. Resolved per
		// field rather than wholesale — a zero sets/reps/rep-seconds means "not
		// overridden", so the template value must survive. Wholesale replacement is
		// what made the varies-by-week toggle blank out every target: it creates a
		// bare flag row, and HasOverride is true whenever a row exists at all.
		//
		// Weight is deliberately excluded from the > 0 guard: 0 kg legitimately means
		// bodyweight, so guarding it would make bodyweight impossible to set. Seeding
		// the flag row from the template (see cycleTemplateExercise) covers weight.
		fbSets, fbReps, fbWeight, fbSecs := v.PlannedSets, v.PlannedReps, v.PlannedWeightKg, v.PlannedRepSecs
		if v.HasOverride {
			if v.OverrideSets > 0 {
				fbSets = v.OverrideSets
			}
			if v.OverrideReps > 0 {
				fbReps = v.OverrideReps
			}
			if v.OverrideRepSecs > 0 {
				fbSecs = v.OverrideRepSecs
			}
			fbWeight = v.OverrideWeightKg
		}
		v.WeekOverrides = make([]pages.CycleWeekTargetView, cycleWeeks)
		for i := 0; i < cycleWeeks; i++ {
			wv := pages.CycleWeekTargetView{
				Week:       i + 1,
				Sets:       fbSets,
				Reps:       fbReps,
				WeightKg:   fbWeight,
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
		s.badRequest(w, "bad request")
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
		s.badRequest(w, "bad request")
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

	w.Header().Set("HX-Redirect", cycleRedirectTarget(r, cycleID))
	w.WriteHeader(http.StatusOK)
}

// handleCycleWeekOverrideSave upserts a CycleExerciseWeekOverride for one exercise+week (silent auto-save).
// POST /training-cycles/{cycleID}/week-override-save
func (s *Server) handleCycleWeekOverrideSave(w http.ResponseWriter, r *http.Request) {
	ownerID := s.mustUserID(r)
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	cycleID, err := parseUintParam(r, "cycleID")
	if err != nil {
		http.Error(w, "invalid cycleID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
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
	ownerID := s.mustUserID(r)
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w)
		return
	}
	cycleID, err := parseUintParam(r, "cycleID")
	if err != nil {
		http.Error(w, "invalid cycleID", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.badRequest(w, "bad request")
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
		// Seed from the template so the row is never all-zero. An all-zero row reads as
		// "override everything to nothing", which is how flipping this toggle used to
		// blank out the exercise's targets for every week of the cycle.
		if tex, ok := s.cycleTemplateExercise(cycleID, ownerID, libIDPtr, exName); ok {
			existing.Sets = tex.Sets
			existing.Reps = tex.Reps
			existing.WeightKg = tex.WeightKg
			existing.RepSeconds = tex.RepSeconds
		}
		s.store.DB.Create(&existing)
	}
	existing.VariesByWeek = mode == "varies"
	s.store.DB.Save(&existing)

	if mode == "same" {
		db.DeleteCycleExerciseWeekOverridesForExercise(s.store.DB, ownerID, cycleID, libIDPtr, exName)
	}

	w.Header().Set("HX-Redirect", cycleRedirectTarget(r, cycleID))
	w.WriteHeader(http.StatusOK)
}
