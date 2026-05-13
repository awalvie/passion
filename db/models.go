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
	// Label is a short freeform tag shown next to the template name (e.g. "hangboard", "strength").
	Label string `gorm:"size:64;not null;default:''"`
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
	PrepSeconds    int
	// RungSeconds is a comma-separated list of per-rep durations for ladder protocols (e.g. "3,6,9").
	// When non-empty, cycles through the values instead of using RepSeconds.
	RungSeconds string
	WeightKg    float64

	OrderIndex int `gorm:"not null;default:0;index"`
}

// ActivityTemplate is a reusable activity blueprint (ordered exercises).
// It can be referenced from session template YAML via `ref:` on an activity entry,
// or copied into a session template through the editor UI.
type ActivityTemplate struct {
	gorm.Model

	OwnerID uint   `gorm:"index;not null"`
	Name    string `gorm:"not null"`
	// Label is a short freeform tag shown next to the template name (e.g. "warmup", "technique").
	Label string `gorm:"size:64;not null;default:''"`

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
	PrepSeconds    int
	RungSeconds    string
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

	OwnerID           uint  `gorm:"index;not null"`
	TrainingCycleID   uint  `gorm:"index;not null"`
	LibraryExerciseID *uint `gorm:"index"` // preferred match key
	ExerciseName      string `gorm:"not null"` // display + fallback match key

	Sets         int
	Reps         int
	WeightKg     float64
	RepSeconds   int
	VariesByWeek bool // when true, per-week overrides take precedence
}

// CycleExerciseWeekOverride stores per-week targets for a single movement within a cycle.
// Resolution order at runtime: week override → cycle override → template default.
type CycleExerciseWeekOverride struct {
	gorm.Model

	OwnerID           uint   `gorm:"index;not null"`
	TrainingCycleID   uint   `gorm:"index;not null"`
	Week              int    `gorm:"not null"` // 1-based week number within the cycle
	ExerciseName      string `gorm:"not null"`
	LibraryExerciseID *uint  `gorm:"index"`

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

const (
	RunStatusDraft     = "draft"
	RunStatusRunning   = "running"
	RunStatusCompleted = "completed"
	RunStatusSkipped   = "skipped"
)

// SessionRun tracks a one-shot guided run created from a scheduled session.
type SessionRun struct {
	gorm.Model

	OwnerID            uint `gorm:"index;not null"`
	ScheduledSessionID uint `gorm:"index;not null"`
	IsTrial            bool `gorm:"index;not null;default:false"`
	// IsOpen marks a freeform "open session" where exercises are added on-the-fly.
	IsOpen bool `gorm:"index;not null;default:false"`
	// IsManual marks a run created via manual log entry (not from a live scheduled session).
	IsManual bool `gorm:"index;not null;default:false"`
	// IsDraft is true until the user saves the manual entry; draft runs are hidden from history/log.
	IsDraft bool `gorm:"index;not null;default:false"`
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

// ClimbingExerciseMeta stores session-level context for a climbing exercise in a run.
// Type values: "indoor_bouldering" | "board" | "gym_routes" | "outdoor_bouldering" | "sport" | "trad" | "top_rope"
// BoardKind values: "kilter" | "moon" | "tension" | "spray" | "custom"
type ClimbingExerciseMeta struct {
	gorm.Model
	OwnerID    uint   `gorm:"index;not null"`
	RunID      uint   `gorm:"index;not null"`
	ExerciseID uint   `gorm:"index;not null"`
	Type       string `gorm:"size:32"`
	BoardKind  string `gorm:"size:32"`
	BoardID    *uint  `gorm:"index"` // optional reference to a configured ClimbingBoard
}

// ManualExerciseSetLog records per-set reps and weight for a manual exercise.
type ManualExerciseSetLog struct {
	gorm.Model
	OwnerID    uint    `gorm:"index;not null"`
	RunID      uint    `gorm:"index;not null"`
	ExerciseID uint    `gorm:"index;not null"`
	SetIndex   int     `gorm:"not null"` // 1-based
	Reps       int
	WeightKg   float64
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

// SessionJournal holds the athlete's post-session reflection.
// RunID is nil for standalone entries created directly from the Training Log page.
// Numeric fields use 0 as "not set" sentinel to avoid nullable columns.
type SessionJournal struct {
	gorm.Model

	OwnerID uint  `gorm:"index;not null"`
	RunID   *uint `gorm:"uniqueIndex"` // nil for standalone entries; SQLite unique index allows multiple NULLs

	// Standalone entry fields (only used when RunID is nil)
	Title string    // optional display name for standalone entries
	Date  time.Time // date of the entry; for run-linked entries this mirrors the run's date

	// Quick-fill fields
	SleepScore int    // 1–5 night-before sleep quality; 0 = not recorded
	Energy     int    // 1–5 pre-session energy level; 0 = not recorded
	RPE        int    // 1–10 post-session perceived exertion; 0 = not recorded
	Focus      string // "strength" | "endurance" | "technique" | "projects" | "general"
	Location   string // "indoor" | "outdoor"

	// Venue and board (optional — set when user has configured climbing venues/boards)
	VenueID *uint `gorm:"index"` // optional climbing venue
	BoardID *uint `gorm:"index"` // optional standalone training board

	// Reflection text (markdown)
	WentWell  string
	NextFocus string
}

// ClimbingVenue is a named climbing location (gym or outdoor crag) belonging to a user.
type ClimbingVenue struct {
	gorm.Model

	OwnerID  uint   `gorm:"index;not null"`
	Name     string `gorm:"size:128;not null"`
	Kind     string `gorm:"size:32;not null"` // "commercial" | "outdoor"
	Location string `gorm:"size:128"`         // optional city / area, e.g. "London"
}

// ClimbingBoard is a standalone training board (Kilter, Moon, Tension, etc.) belonging to a user.
// Boards are independent of venues — a user can log "trained on Home Kilter" without specifying a venue.
type ClimbingBoard struct {
	gorm.Model

	OwnerID   uint   `gorm:"index;not null"`
	BoardType string `gorm:"size:32;not null"` // "kilter" | "moon" | "tension" | "spray" | "custom"
	Name      string `gorm:"size:128"`         // optional display name, e.g. "Home Kilter"
}

// ClimbingTick records a single boulder or route attempt within a climbing exercise step.
// ExerciseID is always set — ticks only exist within exercises where Kind == "climbing".
type ClimbingTick struct {
	gorm.Model

	OwnerID    uint `gorm:"index;not null"`
	RunID      uint `gorm:"index;not null"`
	ExerciseID uint `gorm:"index;not null"`

	Kind     string `gorm:"not null"` // "boulder" | "route"
	Grade    string `gorm:"size:32"`
	Focus    string `gorm:"type:text"` // pre-climb intention: "silent feet", "rhythm with breath"
	Thoughts string `gorm:"type:text"` // post-climb reflection
	Style    string `gorm:"size:32"`   // optional: "onsight"|"flash"|"redpoint"|"project"|"repeat"|"top_rope"
	Attempts int
	Sent     bool
	Stars    int `gorm:"default:0"` // 0 = unrated; 1–3 = quality rating

	OrderIndex int `gorm:"default:0"`
}

// CalendarEvent marks a period on the user's calendar (trip, injury, rest, etc.).
// When Blocks is true, the cycle planner warns about or skips sessions on those dates.
type CalendarEvent struct {
	gorm.Model
	OwnerID   uint      `gorm:"index;not null"`
	Title     string    `gorm:"size:128;not null"`
	Kind      string    `gorm:"size:32;not null"` // "trip"|"injury"|"rest"|"competition"|"other"
	StartDate time.Time `gorm:"not null"`          // local midnight, inclusive
	EndDate   time.Time `gorm:"not null"`          // local midnight, inclusive
	Notes     string    `gorm:"type:text"`
	Blocks    bool      `gorm:"default:true"`
}
