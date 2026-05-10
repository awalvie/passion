package db

import (
	"time"

	"gorm.io/gorm"
)

// User is an authenticated account that owns all workout data via OwnerID.
type User struct {
	gorm.Model

	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	HeightCm     int
	WeightKg     float64
	ApeIndexCm   int
	MaxPullUps   int
	MaxHangKg    float64
	BoulderGrade string `gorm:"size:32"`
	RouteGrade   string `gorm:"size:32"`
}

// SessionTemplate is a reusable workout plan blueprint (activities + exercises).
type SessionTemplate struct {
	gorm.Model

	OwnerID uint   `gorm:"index;not null"`
	Name    string `gorm:"not null"`
	// Color is an optional hex accent (#rrggbb) for dashboard cards; empty = no accent.
	Color string `gorm:"size:16;not null;default:''"`
	// IsSystem marks the hidden per-user anchor template used for open sessions.
	IsSystem bool `gorm:"not null;default:false"`

	Activities []Activity `gorm:"constraint:OnDelete:CASCADE;"`
}

type Activity struct {
	gorm.Model

	OwnerID           uint `gorm:"index;not null"`
	SessionTemplateID uint `gorm:"index;not null"`

	Type string `gorm:"not null"`
	Name string

	OrderIndex int `gorm:"not null;default:0;index"`

	Exercises []Exercise `gorm:"foreignKey:ActivityID;constraint:OnDelete:CASCADE;"`
}

// ExerciseMedia holds one video or image attached to an Exercise or LibraryExercise.
// Exactly one of ExerciseID / LibraryExerciseID is set (polymorphic owner).
type ExerciseMedia struct {
	gorm.Model

	OwnerID           uint  `gorm:"index;not null"`
	ExerciseID        *uint `gorm:"index"`
	LibraryExerciseID *uint `gorm:"index"`

	VideoURL     string
	ThumbnailURL string
	OrderIndex   int `gorm:"not null;default:0"`
}

// PrimaryMedia returns the first (lowest OrderIndex) media item, or nil.
func PrimaryMedia(media []ExerciseMedia) *ExerciseMedia {
	if len(media) == 0 {
		return nil
	}
	best := &media[0]
	for i := 1; i < len(media); i++ {
		if media[i].OrderIndex < best.OrderIndex {
			best = &media[i]
		}
	}
	return best
}

// Exercise belongs to exactly one of Activity (via ActivityID), ActivityTemplate
// (via ActivityTemplateID), or SessionRun (via SessionRunID) for open sessions.
type Exercise struct {
	gorm.Model

	OwnerID            uint  `gorm:"index;not null"`
	ActivityID         *uint `gorm:"index"`
	ActivityTemplateID *uint `gorm:"index"`
	// SessionRunID is set only for exercises added on-the-fly during an open session.
	SessionRunID *uint `gorm:"index"`
	// LibraryExerciseID links back to the source LibraryExercise when the exercise was added from the library.
	LibraryExerciseID *uint `gorm:"index"`

	Name string `gorm:"not null"`

	Media []ExerciseMedia `gorm:"foreignKey:ExerciseID;constraint:OnDelete:CASCADE;"`

	Notes string `gorm:"type:text"`

	Kind string `gorm:"not null;default:reps_and_sets"`

	ParentExerciseID *uint     `gorm:"index"`
	ParentExercise   *Exercise `gorm:"foreignKey:ParentExerciseID"`

	SessionDurationSeconds int

	Sets int
	Reps int
	// Timing defaults for run-player controls.
	RepSeconds     int
	RepRestSeconds int
	SetRestSeconds int
	WeightKg       float64

	OrderIndex int `gorm:"not null;default:0;index"`
}

// ActivityTemplate is a reusable activity blueprint (ordered exercises).
// It can be referenced from session template YAML via `ref:` on an activity entry,
// or copied into a session template through the editor UI.
type ActivityTemplate struct {
	gorm.Model

	OwnerID uint   `gorm:"index;not null"`
	Name    string `gorm:"not null"`

	Exercises []Exercise `gorm:"foreignKey:ActivityTemplateID;constraint:OnDelete:CASCADE;"`
}

// LibraryExercise is a reusable exercise preset (same shape as Exercise, not tied to an activity).
// Users copy these into session templates when building activities.
type LibraryExercise struct {
	gorm.Model

	OwnerID uint `gorm:"index;not null"`

	Name string `gorm:"not null"`

	Notes string `gorm:"type:text"`

	Media []ExerciseMedia `gorm:"foreignKey:LibraryExerciseID;constraint:OnDelete:CASCADE;"`

	Kind                   string `gorm:"not null;default:reps_and_sets"`
	SessionDurationSeconds int

	Sets           int
	Reps           int
	RepSeconds     int
	RepRestSeconds int
	SetRestSeconds int
	WeightKg       float64

	// ParentLibraryExerciseID links child options to a catalog parent in the library.
	ParentLibraryExerciseID *uint             `gorm:"index"`
	ParentLibraryExercise   *LibraryExercise  `gorm:"foreignKey:ParentLibraryExerciseID"`
	Children                []LibraryExercise `gorm:"foreignKey:ParentLibraryExerciseID"`
	OrderIndex              int               `gorm:"not null;default:0"`
}

// TrainingCycle defines a multi-week training plan generated from weekday->template mappings.
type TrainingCycle struct {
	gorm.Model

	OwnerID uint `gorm:"index;not null"`
	Name    string

	// StartDate is the date the user chose in the UI (used as the lower bound when generating sessions).
	StartDate time.Time `gorm:"index;not null"`
	Weeks     int       `gorm:"not null"`

	WeekdayMappings []TrainingCycleWeekdayMapping `gorm:"constraint:OnDelete:CASCADE;"`
}

// TrainingCycleWeekdayMapping maps a weekday (Mon=1..Sun=7) to a session_template.
type TrainingCycleWeekdayMapping struct {
	gorm.Model

	OwnerID uint `gorm:"index;not null"`

	TrainingCycleID uint `gorm:"index;not null"`
	Weekday         int  `gorm:"index;not null"` // 1..7 (Mon..Sun)

	SessionTemplateID uint `gorm:"index;not null"`
}

// CycleExerciseOverride stores per-cycle target values for a movement.
// The override applies to every exercise in the cycle that matches by LibraryExerciseID
// (when set) or by ExerciseName as fallback — across all templates in the cycle.
type CycleExerciseOverride struct {
	gorm.Model

	OwnerID         uint  `gorm:"index;not null"`
	TrainingCycleID uint  `gorm:"index;not null"`
	LibraryExerciseID *uint `gorm:"index"` // preferred match key
	ExerciseName    string `gorm:"not null"` // display + fallback match key

	Sets       int
	Reps       int
	WeightKg   float64
	RepSeconds int
}

// ScheduledSession is the materialized instance for a given date in a training cycle.
type ScheduledSession struct {
	gorm.Model

	OwnerID         uint  `gorm:"index;not null"`
	TrainingCycleID *uint `gorm:"index"`
	IsTrial         bool  `gorm:"index;not null;default:false"`

	// ScheduledDate is stored at local midnight for date math consistency.
	ScheduledDate time.Time `gorm:"index;not null"`

	SessionTemplateID uint            `gorm:"index;not null"`
	SessionTemplate   SessionTemplate `gorm:"foreignKey:SessionTemplateID;constraint:OnDelete:CASCADE;"`
}

// SessionRun tracks a one-shot guided run created from a scheduled session.
type SessionRun struct {
	gorm.Model

	OwnerID            uint `gorm:"index;not null"`
	ScheduledSessionID uint `gorm:"index;not null"`
	IsTrial            bool `gorm:"index;not null;default:false"`
	// IsOpen marks a freeform "open session" where exercises are added on-the-fly.
	IsOpen bool `gorm:"index;not null;default:false"`
	// CustomName is an optional user-set display name that overrides the template name.
	CustomName string `gorm:"type:text"`

	Status string `gorm:"not null;default:running"` // running/completed

	StartedAt   time.Time  `gorm:"index;not null"`
	CompletedAt *time.Time `gorm:"index"`
}

// RunExerciseCompletion stores the user's checkoff for an exercise during a run.
type RunExerciseCompletion struct {
	gorm.Model

	OwnerID uint `gorm:"index;not null"`
	RunID   uint `gorm:"index;not null"`

	ExerciseID uint   `gorm:"index;not null"`
	Status     string `gorm:"not null;default:completed"` // completed/skipped

	CompletedAt    time.Time `gorm:"index;not null"`
	ElapsedSeconds int

	// RunNotes are freeform notes/observations entered during the run.
	RunNotes string `gorm:"type:text"`

	// Actual values recorded at completion time (pre-filled from template, editable by user).
	ActualSets     int
	ActualReps     int
	ActualWeightKg float64
}

// RunExerciseChoice records which child option was selected for an exercise_catalog parent during a run.
// Multiple rows per (RunID, ParentExerciseID) are allowed so the user can pick N exercises.
type RunExerciseChoice struct {
	gorm.Model

	OwnerID uint `gorm:"index;not null"`
	RunID   uint `gorm:"index;not null"`

	ParentExerciseID uint `gorm:"index;not null"`
	ChosenExerciseID uint `gorm:"index;not null"`
}
