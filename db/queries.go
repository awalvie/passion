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

// ListTemplates returns all session templates for a user, ordered by id descending.
func ListTemplates(gdb *gorm.DB, ownerID uint) ([]SessionTemplate, error) {
	var templates []SessionTemplate
	err := gdb.
		Where("owner_id = ?", ownerID).
		Order("id desc").
		Find(&templates).Error
	return templates, err
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
func ListActivityTemplates(gdb *gorm.DB, ownerID uint) ([]ActivityTemplate, error) {
	var rows []ActivityTemplate
	err := gdb.
		Where("owner_id = ?", ownerID).
		Order("name asc").
		Find(&rows).Error
	return rows, err
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
