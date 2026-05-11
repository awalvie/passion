package db

import (
	"time"

	"gorm.io/gorm"
)

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
	if err != nil {
		return nil, err
	}
	return &tpl, nil
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
			ownerID, "completed", start, end).
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
