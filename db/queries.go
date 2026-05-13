package db

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// ErrNotFound is returned by query helpers when a record is not found.
// Callers can distinguish it from real DB failures with errors.Is(err, ErrNotFound).
var ErrNotFound = errors.New("not found")

// isNotFound reports whether err is a GORM record-not-found error.
func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// LocalDate truncates t to midnight in its own location (strips time-of-day).
func LocalDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// GetScheduledSessionWithTemplate loads a ScheduledSession with its full template graph
// (Activities ordered by order_index → Exercises ordered by order_index → Media)
// for the given owner.
func GetScheduledSessionWithTemplate(gdb *gorm.DB, ownerID, ssID uint) (ScheduledSession, error) {
	var ss ScheduledSession
	err := gdb.
		Preload("SessionTemplate", func(tx *gorm.DB) *gorm.DB {
			return tx.
				Preload("Activities", func(tx2 *gorm.DB) *gorm.DB {
					return tx2.Where("owner_id = ?", ownerID).Order("order_index asc")
				}).
				Preload("Activities.Exercises", func(tx2 *gorm.DB) *gorm.DB {
					return tx2.Where("owner_id = ?", ownerID).Order("order_index asc")
				}).
				Preload("Activities.Exercises.Media")
		}).
		Where("owner_id = ? AND id = ?", ownerID, ssID).
		First(&ss).Error
	if isNotFound(err) {
		return ss, ErrNotFound
	}
	return ss, err
}

// GetTemplateWithGraph loads a SessionTemplate with all Activities (ordered), Exercises
// (ordered), and Media for the given owner.
func GetTemplateWithGraph(gdb *gorm.DB, ownerID, templateID uint) (*SessionTemplate, error) {
	var tpl SessionTemplate
	err := gdb.
		Preload("Activities", func(tx *gorm.DB) *gorm.DB {
			return tx.Where("owner_id = ?", ownerID).Order("order_index asc")
		}).
		Preload("Activities.Exercises", func(tx *gorm.DB) *gorm.DB {
			return tx.Where("owner_id = ?", ownerID).Order("order_index asc")
		}).
		Preload("Activities.Exercises.Media").
		Where("id = ? AND owner_id = ?", templateID, ownerID).
		First(&tpl).Error
	if isNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &tpl, nil
}

// ListTemplates returns all non-system session templates for a user, ordered by id descending.
// If labelFilter is non-empty, only templates with that label are returned.
func ListTemplates(gdb *gorm.DB, ownerID uint, labelFilter string) ([]SessionTemplate, error) {
	var templates []SessionTemplate
	q := gdb.Where("owner_id = ? AND is_system = ?", ownerID, false)
	if labelFilter != "" {
		q = q.Where("label = ?", labelFilter)
	}
	err := q.Order("id desc").Find(&templates).Error
	return templates, err
}

// DistinctTemplateLabels returns the sorted distinct non-empty labels across all session templates for a user.
func DistinctTemplateLabels(gdb *gorm.DB, ownerID uint) ([]string, error) {
	var labels []string
	err := gdb.Model(&SessionTemplate{}).
		Where("owner_id = ? AND is_system = ? AND label != ''", ownerID, false).
		Distinct("label").
		Order("label asc").
		Pluck("label", &labels).Error
	return labels, err
}

// ListLibraryExercises returns all root (no parent) library exercises for a user,
// ordered by name ascending.
func ListLibraryExercises(gdb *gorm.DB, ownerID uint) ([]LibraryExercise, error) {
	var rows []LibraryExercise
	err := gdb.
		Where("owner_id = ? AND parent_library_exercise_id IS NULL", ownerID).
		Order("name asc").
		Find(&rows).Error
	return rows, err
}

// ListActivityTemplates returns all activity templates for a user, ordered by name ascending.
// If labelFilter is non-empty, only templates with that label are returned.
func ListActivityTemplates(gdb *gorm.DB, ownerID uint, labelFilter string) ([]ActivityTemplate, error) {
	var rows []ActivityTemplate
	q := gdb.Where("owner_id = ?", ownerID)
	if labelFilter != "" {
		q = q.Where("label = ?", labelFilter)
	}
	err := q.Order("name asc").Find(&rows).Error
	return rows, err
}

// DistinctActivityTemplateLabels returns the sorted distinct non-empty labels across all activity templates for a user.
func DistinctActivityTemplateLabels(gdb *gorm.DB, ownerID uint) ([]string, error) {
	var labels []string
	err := gdb.Model(&ActivityTemplate{}).
		Where("owner_id = ? AND label != ''", ownerID).
		Distinct("label").
		Order("label asc").
		Pluck("label", &labels).Error
	return labels, err
}

// GetActivityTemplateWithExercises loads an ActivityTemplate with its root exercises and their
// catalog children, all ordered by order_index.
func GetActivityTemplateWithExercises(gdb *gorm.DB, ownerID, templateID uint) (*ActivityTemplate, error) {
	var tpl ActivityTemplate
	err := gdb.
		Preload("Exercises", func(tx *gorm.DB) *gorm.DB {
			return tx.Where("owner_id = ?", ownerID).Order("order_index asc")
		}).
		Preload("Exercises.Media").
		Where("id = ? AND owner_id = ?", templateID, ownerID).
		First(&tpl).Error
	if isNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &tpl, nil
}

// ListActivityTemplatesWithExercises returns all activity templates for a user with their
// exercises preloaded, ordered by name ascending.
func ListActivityTemplatesWithExercises(gdb *gorm.DB, ownerID uint) ([]ActivityTemplate, error) {
	var rows []ActivityTemplate
	err := gdb.
		Preload("Exercises", func(tx *gorm.DB) *gorm.DB {
			return tx.Where("owner_id = ?", ownerID).Order("order_index asc")
		}).
		Where("owner_id = ?", ownerID).
		Order("name asc").
		Find(&rows).Error
	return rows, err
}

// ListScheduledSessionsInRange returns non-trial scheduled sessions (with their templates)
// for a user within the given date range, ordered by scheduled_date ascending.
func ListScheduledSessionsInRange(gdb *gorm.DB, ownerID uint, start, end time.Time) ([]ScheduledSession, error) {
	var rows []ScheduledSession
	err := gdb.
		Preload("SessionTemplate").
		Where("owner_id = ? AND is_trial = ? AND scheduled_date >= ? AND scheduled_date <= ?",
			ownerID, false, start, end).
		Order("scheduled_date asc").
		Find(&rows).Error
	return rows, err
}

// CompletedRunDate groups completed-run counts by date and trial status.
type CompletedRunDate struct {
	ScheduledDate      time.Time
	IsTrial            bool
	ScheduledSessionID uint
}

// ListCompletedRunDatesInRange returns one row per completed SessionRun whose
// associated ScheduledSession.ScheduledDate falls within [start, end].
func ListCompletedRunDatesInRange(gdb *gorm.DB, ownerID uint, start, end time.Time) ([]CompletedRunDate, error) {
	var rows []CompletedRunDate
	err := gdb.
		Table("session_runs").
		Select("scheduled_sessions.scheduled_date, session_runs.is_trial, session_runs.scheduled_session_id").
		Joins("JOIN scheduled_sessions ON scheduled_sessions.id = session_runs.scheduled_session_id").
		Where("session_runs.owner_id = ? AND session_runs.status = ? AND session_runs.deleted_at IS NULL AND scheduled_sessions.scheduled_date >= ? AND scheduled_sessions.scheduled_date <= ?",
			ownerID, RunStatusCompleted, start, end).
		Scan(&rows).Error
	return rows, err
}

// GetSessionJournalByRunID returns the journal for a run, or nil if none exists.
func GetSessionJournalByRunID(gdb *gorm.DB, ownerID, runID uint) (*SessionJournal, error) {
	var j SessionJournal
	err := gdb.Where("owner_id = ? AND run_id = ?", ownerID, runID).First(&j).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &j, err
}

// UpsertSessionJournal creates or updates the journal for a run.
// Pass j.ID = 0 to create, non-zero to update.
func UpsertSessionJournal(gdb *gorm.DB, j *SessionJournal) error {
	return gdb.Save(j).Error
}

// ListSessionJournals returns all journals for a user, newest first.
func ListSessionJournals(gdb *gorm.DB, ownerID uint) ([]SessionJournal, error) {
	var journals []SessionJournal
	err := gdb.Where("owner_id = ?", ownerID).Order("id desc").Find(&journals).Error
	return journals, err
}

// GetSessionJournalByID returns a single journal by primary key, scoped to the owner.
func GetSessionJournalByID(gdb *gorm.DB, ownerID, id uint) (*SessionJournal, error) {
	var j SessionJournal
	err := gdb.Where("owner_id = ? AND id = ?", ownerID, id).First(&j).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &j, err
}

// DeleteSessionJournal hard-deletes a journal entry owned by the given user.
func DeleteSessionJournal(gdb *gorm.DB, ownerID, id uint) error {
	return gdb.Where("owner_id = ? AND id = ?", ownerID, id).Delete(&SessionJournal{}).Error
}

// ListCycleExerciseWeekOverrides returns all week overrides for a cycle, ordered by week asc.
func ListCycleExerciseWeekOverrides(gdb *gorm.DB, ownerID, cycleID uint) ([]CycleExerciseWeekOverride, error) {
	var rows []CycleExerciseWeekOverride
	err := gdb.
		Where("owner_id = ? AND training_cycle_id = ?", ownerID, cycleID).
		Order("week asc").
		Find(&rows).Error
	return rows, err
}

// UpsertCycleExerciseWeekOverride creates or updates a week override.
func UpsertCycleExerciseWeekOverride(gdb *gorm.DB, o *CycleExerciseWeekOverride) error {
	var existing CycleExerciseWeekOverride
	q := gdb.Where("owner_id = ? AND training_cycle_id = ? AND week = ?", o.OwnerID, o.TrainingCycleID, o.Week)
	if o.LibraryExerciseID != nil && *o.LibraryExerciseID != 0 {
		q = q.Where("library_exercise_id = ?", *o.LibraryExerciseID)
	} else {
		q = q.Where("library_exercise_id IS NULL AND exercise_name = ?", o.ExerciseName)
	}
	if err := q.First(&existing).Error; err != nil {
		return gdb.Create(o).Error
	}
	existing.Sets = o.Sets
	existing.Reps = o.Reps
	existing.WeightKg = o.WeightKg
	existing.RepSeconds = o.RepSeconds
	return gdb.Save(&existing).Error
}

// DeleteCycleExerciseWeekOverridesForExercise deletes all week overrides for one exercise in a cycle.
func DeleteCycleExerciseWeekOverridesForExercise(gdb *gorm.DB, ownerID, cycleID uint, libID *uint, exerciseName string) error {
	q := gdb.Where("owner_id = ? AND training_cycle_id = ?", ownerID, cycleID)
	if libID != nil && *libID != 0 {
		q = q.Where("library_exercise_id = ?", *libID)
	} else {
		q = q.Where("library_exercise_id IS NULL AND exercise_name = ?", exerciseName)
	}
	return q.Delete(&CycleExerciseWeekOverride{}).Error
}

// ---------------------------------------------------------------------------
// Climbing ticks
// ---------------------------------------------------------------------------

// ListClimbingTicksByExercise returns all ticks for a specific exercise step, ordered by order_index asc.
func ListClimbingTicksByExercise(gdb *gorm.DB, ownerID, runID, exerciseID uint) ([]ClimbingTick, error) {
	var rows []ClimbingTick
	err := gdb.
		Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exerciseID).
		Order("order_index asc, id asc").
		Find(&rows).Error
	return rows, err
}

// CreateClimbingTick inserts a new tick, assigning the next order_index within the exercise.
func CreateClimbingTick(gdb *gorm.DB, t *ClimbingTick) error {
	var maxIdx int
	if err := gdb.Model(&ClimbingTick{}).
		Where("owner_id = ? AND run_id = ? AND exercise_id = ?", t.OwnerID, t.RunID, t.ExerciseID).
		Select("COALESCE(MAX(order_index), -1)").
		Scan(&maxIdx).Error; err != nil {
		return err
	}
	t.OrderIndex = maxIdx + 1
	return gdb.Create(t).Error
}

// DeleteClimbingTick hard-deletes a tick (validates ownerID).
func DeleteClimbingTick(gdb *gorm.DB, ownerID, id uint) error {
	return gdb.Where("owner_id = ? AND id = ?", ownerID, id).Delete(&ClimbingTick{}).Error
}

// ClimbingTickSummary is a compact summary of ticks for a run, used in the training log.
type ClimbingTickSummary struct {
	TotalBoulders int
	TotalRoutes   int
	TotalSends    int
	MinGrade      string
	MaxGrade      string
}

// GetClimbingTickSummaryForRun builds a summary of all ticks in a run.
func GetClimbingTickSummaryForRun(gdb *gorm.DB, ownerID, runID uint) (ClimbingTickSummary, error) {
	var ticks []ClimbingTick
	if err := gdb.Where("owner_id = ? AND run_id = ?", ownerID, runID).Find(&ticks).Error; err != nil {
		return ClimbingTickSummary{}, err
	}
	var s ClimbingTickSummary
	grades := make([]string, 0)
	for _, t := range ticks {
		if t.Kind == "boulder" {
			s.TotalBoulders++
		} else {
			s.TotalRoutes++
		}
		if t.Sent {
			s.TotalSends++
		}
		if t.Grade != "" {
			grades = append(grades, t.Grade)
		}
	}
	if len(grades) > 0 {
		s.MinGrade = grades[0]
		s.MaxGrade = grades[len(grades)-1]
		for _, g := range grades {
			if g < s.MinGrade {
				s.MinGrade = g
			}
			if g > s.MaxGrade {
				s.MaxGrade = g
			}
		}
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// Climbing venues and boards
// ---------------------------------------------------------------------------

// ListClimbingVenues returns all venues for a user, ordered by name.
func ListClimbingVenues(gdb *gorm.DB, ownerID uint) ([]ClimbingVenue, error) {
	var rows []ClimbingVenue
	err := gdb.Where("owner_id = ?", ownerID).Order("name asc").Find(&rows).Error
	return rows, err
}

// CreateClimbingVenue inserts a new venue.
func CreateClimbingVenue(gdb *gorm.DB, v *ClimbingVenue) error {
	return gdb.Create(v).Error
}

// DeleteClimbingVenue hard-deletes a venue and nulls SessionJournal.VenueID for affected entries.
func DeleteClimbingVenue(gdb *gorm.DB, ownerID, id uint) error {
	if err := gdb.Model(&SessionJournal{}).
		Where("owner_id = ? AND venue_id = ?", ownerID, id).
		Updates(map[string]interface{}{"venue_id": nil, "board_id": nil}).Error; err != nil {
		return err
	}
	return gdb.Where("owner_id = ? AND id = ?", ownerID, id).Delete(&ClimbingVenue{}).Error
}

// ListClimbingBoards returns all standalone boards for a user, ordered by name.
func ListClimbingBoards(gdb *gorm.DB, ownerID uint) ([]ClimbingBoard, error) {
	var rows []ClimbingBoard
	err := gdb.Where("owner_id = ?", ownerID).Order("name asc, board_type asc").Find(&rows).Error
	return rows, err
}

// CreateClimbingBoard inserts a standalone board.
func CreateClimbingBoard(gdb *gorm.DB, b *ClimbingBoard) error {
	return gdb.Create(b).Error
}

// DeleteClimbingBoard hard-deletes a board and nulls SessionJournal.BoardID for affected entries.
func DeleteClimbingBoard(gdb *gorm.DB, ownerID, id uint) error {
	if err := gdb.Model(&SessionJournal{}).
		Where("owner_id = ? AND board_id = ?", ownerID, id).
		Update("board_id", nil).Error; err != nil {
		return err
	}
	return gdb.Where("owner_id = ? AND id = ?", ownerID, id).Delete(&ClimbingBoard{}).Error
}

// ---------------------------------------------------------------------------
// Draft runs (for manual log entries)
// ---------------------------------------------------------------------------

// CreateDraftSessionRun creates a draft manual run using the given scheduled session anchor.
// The caller must create the ScheduledSession anchor first (same pattern as open sessions).
func CreateDraftSessionRun(gdb *gorm.DB, ownerID, scheduledSessionID uint) (*SessionRun, error) {
	run := &SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: scheduledSessionID,
		IsTrial:            true,
		IsManual:           true,
		IsDraft:            true,
		Status:             RunStatusRunning,
		StartedAt:          time.Now(),
	}
	return run, gdb.Create(run).Error
}

// FinaliseDraftRun promotes a draft run to a completed manual entry.
func FinaliseDraftRun(gdb *gorm.DB, ownerID, runID uint, customName string, date time.Time) error {
	return gdb.Model(&SessionRun{}).
		Where("owner_id = ? AND id = ? AND is_draft = ?", ownerID, runID, true).
		Updates(map[string]interface{}{
			"is_draft":     false,
			"status":       RunStatusCompleted,
			"custom_name":  customName,
			"started_at":   date,
			"completed_at": date,
		}).Error
}

// DeleteDraftRun hard-deletes a draft run and all its exercises, completions, and ticks.
func DeleteDraftRun(gdb *gorm.DB, ownerID, runID uint) error {
	return gdb.Transaction(func(tx *gorm.DB) error {
		var exerciseIDs []uint
		if err := tx.Model(&Exercise{}).
			Where("owner_id = ? AND session_run_id = ?", ownerID, runID).
			Pluck("id", &exerciseIDs).Error; err != nil {
			return err
		}
		if len(exerciseIDs) > 0 {
			if err := tx.Where("owner_id = ? AND exercise_id IN ?", ownerID, exerciseIDs).Delete(&ClimbingTick{}).Error; err != nil {
				return err
			}
			if err := tx.Where("owner_id = ? AND exercise_id IN ?", ownerID, exerciseIDs).Delete(&RunExerciseCompletion{}).Error; err != nil {
				return err
			}
			if err := tx.Where("owner_id = ? AND id IN ?", ownerID, exerciseIDs).Delete(&Exercise{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("owner_id = ? AND run_id = ?", ownerID, runID).Delete(&ClimbingTick{}).Error; err != nil {
			return err
		}
		return tx.Where("owner_id = ? AND id = ? AND is_draft = ?", ownerID, runID, true).
			Delete(&SessionRun{}).Error
	})
}

// AddManualExercise creates an Exercise attached directly to a SessionRun (no ActivityID).
func AddManualExercise(gdb *gorm.DB, ownerID, runID uint, name string, libraryExerciseID *uint, kind string) (*Exercise, error) {
	var orderIndex int
	if err := gdb.Model(&Exercise{}).
		Where("owner_id = ? AND session_run_id = ?", ownerID, runID).
		Select("COALESCE(MAX(order_index), -1)").
		Scan(&orderIndex).Error; err != nil {
		return nil, err
	}
	ex := &Exercise{
		OwnerID:           ownerID,
		SessionRunID:      &runID,
		LibraryExerciseID: libraryExerciseID,
		Name:              name,
		Kind:              kind,
		OrderIndex:        orderIndex + 1,
	}
	return ex, gdb.Create(ex).Error
}

// DeleteManualExercise removes an exercise from a draft run, along with any ticks.
func DeleteManualExercise(gdb *gorm.DB, ownerID, runID, exerciseID uint) error {
	if err := gdb.Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exerciseID).Delete(&ClimbingTick{}).Error; err != nil {
		return err
	}
	if err := gdb.Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exerciseID).Delete(&RunExerciseCompletion{}).Error; err != nil {
		return err
	}
	return gdb.Where("owner_id = ? AND session_run_id = ? AND id = ?", ownerID, runID, exerciseID).
		Delete(&Exercise{}).Error
}

// ListExercisesForRun returns exercises attached directly to a session run, ordered by order_index.
func ListExercisesForRun(gdb *gorm.DB, ownerID, runID uint) ([]Exercise, error) {
	var rows []Exercise
	err := gdb.
		Where("owner_id = ? AND session_run_id = ?", ownerID, runID).
		Order("order_index asc").
		Find(&rows).Error
	return rows, err
}

// ListExerciseCountsByRun returns a map of runID → exercise count for the given run IDs.
// Replaces per-run ListExercisesForRun calls in list views.
func ListExerciseCountsByRun(gdb *gorm.DB, ownerID uint, runIDs []uint) (map[uint]int, error) {
	if len(runIDs) == 0 {
		return map[uint]int{}, nil
	}
	type row struct {
		SessionRunID uint
		Count        int
	}
	var rows []row
	err := gdb.Model(&Exercise{}).
		Select("session_run_id, COUNT(*) as count").
		Where("owner_id = ? AND session_run_id IN ?", ownerID, runIDs).
		Group("session_run_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[uint]int, len(rows))
	for _, r := range rows {
		m[r.SessionRunID] = r.Count
	}
	return m, nil
}

// ListTickSummariesByRun returns a map of runID → ClimbingTickSummary for the given run IDs.
// Replaces per-run GetClimbingTickSummaryForRun calls in list views.
func ListTickSummariesByRun(gdb *gorm.DB, ownerID uint, runIDs []uint) (map[uint]ClimbingTickSummary, error) {
	if len(runIDs) == 0 {
		return map[uint]ClimbingTickSummary{}, nil
	}
	var ticks []ClimbingTick
	if err := gdb.Where("owner_id = ? AND run_id IN ?", ownerID, runIDs).Find(&ticks).Error; err != nil {
		return nil, err
	}
	m := make(map[uint]ClimbingTickSummary)
	for _, t := range ticks {
		s := m[t.RunID]
		if t.Kind == "boulder" {
			s.TotalBoulders++
		} else {
			s.TotalRoutes++
		}
		if t.Sent {
			s.TotalSends++
		}
		if t.Grade != "" {
			if s.MinGrade == "" || t.Grade < s.MinGrade {
				s.MinGrade = t.Grade
			}
			if t.Grade > s.MaxGrade {
				s.MaxGrade = t.Grade
			}
		}
		m[t.RunID] = s
	}
	return m, nil
}

// UpsertManualExerciseCompletion creates or updates the completion record for a manual exercise.
func UpsertManualExerciseCompletion(gdb *gorm.DB, ownerID, runID, exerciseID uint, sets, reps int, weightKg float64, notes string) error {
	var existing RunExerciseCompletion
	err := gdb.Where("owner_id = ? AND run_id = ? AND exercise_id = ?", ownerID, runID, exerciseID).
		First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return gdb.Create(&RunExerciseCompletion{
			OwnerID:        ownerID,
			RunID:          runID,
			ExerciseID:     exerciseID,
			Status:         RunStatusCompleted,
			CompletedAt:    time.Now(),
			ActualSets:     sets,
			ActualReps:     reps,
			ActualWeightKg: weightKg,
			RunNotes:       notes,
		}).Error
	}
	if err != nil {
		return err
	}
	existing.ActualSets = sets
	existing.ActualReps = reps
	existing.ActualWeightKg = weightKg
	existing.RunNotes = notes
	return gdb.Save(&existing).Error
}
