package db

import (
	"time"

	"gorm.io/gorm"
)

// User is an authenticated account that owns all workout data via OwnerID.
type User struct {
	gorm.Model

	Email              string `gorm:"uniqueIndex;not null"`
	PasswordHash       string `gorm:"not null"`
	HeightCm           int
	WeightKg           float64
	ApeIndexCm         int
	MaxPullUps         int
	MaxHangKg          float64
	BoulderGrade       string `gorm:"size:32"`
	RouteGrade         string `gorm:"size:32"`
	BoulderGradeSystem string `gorm:"size:32;not null;default:'font'"`
	RouteGradeSystem   string `gorm:"size:32;not null;default:'french'"`
}

// InviteCode gates signup. Passion's catalog carries content licensed to one account, so
// an open signup page hands that content to anyone who finds it. Bounding who can create
// an account is what makes the rest of the account layer (manual deletion, support by
// hand) legitimate at this scale.
//
// A code is single-use: UsedByID is set when it is redeemed and a redeemed code is never
// accepted again.
type InviteCode struct {
	gorm.Model

	Code string `gorm:"uniqueIndex;not null"`
	// CreatedByID is the user who minted it, or nil when the binary minted it from the
	// command line before any user existed.
	CreatedByID *uint `gorm:"index"`
	// UsedByID is the account created with this code. Nil means unredeemed.
	UsedByID *uint      `gorm:"index"`
	UsedAt   *time.Time `gorm:"index"`
	// ExpiresAt is nil for a code that never expires.
	ExpiresAt *time.Time `gorm:"index"`
	// Note records who the code was meant for, so unredeemed codes can be told apart.
	Note string `gorm:"size:128;not null;default:''"`
}

// Redeemed reports whether the code has already been used to create an account.
func (c InviteCode) Redeemed() bool { return c.UsedByID != nil }

// Expired reports whether the code is past its expiry at time now.
func (c InviteCode) Expired(now time.Time) bool {
	return c.ExpiresAt != nil && now.After(*c.ExpiresAt)
}

// SessionTemplate is a reusable workout plan blueprint (activities + exercises).
type SessionTemplate struct {
	gorm.Model

	OwnerID uint   `gorm:"index;not null"`
	Name    string `gorm:"not null"`
	// Slug is the stable identity of a catalog row. The importer matches on it, so the
	// display Name is free to change without the row being deleted and recreated. It comes
	// from an explicit `slug:` in the YAML — never from the filename, because the public and
	// private trees both contain drills.yaml and warmup.yaml, and because some files hold a
	// list of items rather than one.
	Slug string `gorm:"size:128;not null;default:''"`
	// Shared marks a row as part of the catalog every account reads. The catalog is the
	// app's, not a user's: nobody edits a shared row, and saving your own version copies
	// it to you.
	//
	// A separate flag rather than a magic OwnerID. A nullable owner would mean SQLite
	// enforces no uniqueness among catalog rows (NULLs are distinct in a unique index),
	// and "WHERE owner_id <> me" would silently skip the whole catalog — a trap this
	// project already hit once, in a production cleanup script. OwnerID keeps meaning
	// exactly one thing: who made this. Forgetting to set Shared leaves a row private,
	// so the failure is closed.
	Shared bool `gorm:"index;not null;default:false"`
	// Color is an optional hex accent (#rrggbb) for dashboard cards; empty = no accent.
	Color string `gorm:"size:16;not null;default:''"`
	// Label holds comma-separated freeform tags shown as chips (e.g. "technique, indoor").
	Label string `gorm:"size:128;not null;default:''"`
	// Source is the program or coach the template comes from (e.g. "Power Company Climbing").
	Source string `gorm:"size:64;not null;default:''"`
	// Needs lists the equipment/gear this session assumes, comma-separated, shown as an
	// informational chip line (e.g. "hangboard, kilter board, 20mm edge").
	Needs string `gorm:"size:128;not null;default:''"`
	// IsSystem marks the hidden per-user anchor template used for open sessions.
	IsSystem bool `gorm:"not null;default:false"`
	// ManagedByCatalog is true for rows created by the YAML importer. Only these are
	// eligible for prune-on-import; UI-created templates stay false and are never removed.
	ManagedByCatalog bool `gorm:"not null;default:false"`
	// CatalogEditedAt is set the first time a user edits a row the importer created. Once
	// set, the importer stops overwriting the row and prune-on-import stops deleting it, so
	// the edit survives a restart. Clearing it hands the row back to the importer.
	CatalogEditedAt *time.Time

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
	// RunBlockType and RunBlockName are copied from the Activity this exercise came from,
	// and are set only on rows a run owns. A run must render from its own copy, and an
	// Activity cannot belong to a run (SessionTemplateID is not null), so the grouping
	// travels as text instead. Empty on template-owned rows.
	RunBlockType string `gorm:"size:32;not null;default:''"`
	RunBlockName string `gorm:"size:128;not null;default:''"`
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
	// Slug is the stable identity of a catalog row. The importer matches on it, so the
	// display Name is free to change without the row being deleted and recreated. It comes
	// from an explicit `slug:` in the YAML — never from the filename, because the public and
	// private trees both contain drills.yaml and warmup.yaml, and because some files hold a
	// list of items rather than one.
	Slug string `gorm:"size:128;not null;default:''"`
	// Shared marks a row as part of the catalog every account reads. The catalog is the
	// app's, not a user's: nobody edits a shared row, and saving your own version copies
	// it to you.
	//
	// A separate flag rather than a magic OwnerID. A nullable owner would mean SQLite
	// enforces no uniqueness among catalog rows (NULLs are distinct in a unique index),
	// and "WHERE owner_id <> me" would silently skip the whole catalog — a trap this
	// project already hit once, in a production cleanup script. OwnerID keeps meaning
	// exactly one thing: who made this. Forgetting to set Shared leaves a row private,
	// so the failure is closed.
	Shared bool `gorm:"index;not null;default:false"`
	// Label holds comma-separated freeform tags shown as chips (e.g. "warmup, technique").
	Label string `gorm:"size:128;not null;default:''"`
	// Source is the program or coach the template comes from (e.g. "Power Company Climbing").
	Source string `gorm:"size:64;not null;default:''"`
	// ManagedByCatalog is true for rows created by the YAML importer. Only these are
	// eligible for prune-on-import; UI-created templates stay false and are never removed.
	ManagedByCatalog bool `gorm:"not null;default:false"`
	// CatalogEditedAt is set the first time a user edits a row the importer created. Once
	// set, the importer stops overwriting the row and prune-on-import stops deleting it, so
	// the edit survives a restart. Clearing it hands the row back to the importer.
	CatalogEditedAt *time.Time

	Exercises []Exercise `gorm:"foreignKey:ActivityTemplateID;constraint:OnDelete:CASCADE;"`
}

// LibraryExercise is a reusable exercise preset (same shape as Exercise, not tied to an activity).
// Users copy these into session templates when building activities.
type LibraryExercise struct {
	gorm.Model

	OwnerID uint `gorm:"index;not null"`

	Name string `gorm:"not null"`
	// Slug is the stable identity of a catalog row. The importer matches on it, so the
	// display Name is free to change without the row being deleted and recreated. It comes
	// from an explicit `slug:` in the YAML — never from the filename, because the public and
	// private trees both contain drills.yaml and warmup.yaml, and because some files hold a
	// list of items rather than one.
	Slug string `gorm:"size:128;not null;default:''"`
	// Shared marks a row as part of the catalog every account reads. The catalog is the
	// app's, not a user's: nobody edits a shared row, and saving your own version copies
	// it to you.
	//
	// A separate flag rather than a magic OwnerID. A nullable owner would mean SQLite
	// enforces no uniqueness among catalog rows (NULLs are distinct in a unique index),
	// and "WHERE owner_id <> me" would silently skip the whole catalog — a trap this
	// project already hit once, in a production cleanup script. OwnerID keeps meaning
	// exactly one thing: who made this. Forgetting to set Shared leaves a row private,
	// so the failure is closed.
	Shared bool `gorm:"index;not null;default:false"`
	// Label holds comma-separated freeform tags shown as chips (e.g. "technique, fingers").
	Label string `gorm:"size:128;not null;default:''"`
	// Source is the program or coach the exercise comes from (e.g. "Power Company Climbing").
	Source string `gorm:"size:64;not null;default:''"`
	// ManagedByCatalog is true for rows created by the YAML importer. Only these are
	// eligible for prune-on-import; UI-created exercises stay false and are never removed.
	ManagedByCatalog bool `gorm:"not null;default:false"`
	// CatalogEditedAt is set the first time a user edits a row the importer created. Once
	// set, the importer stops overwriting the row and prune-on-import stops deleting it, so
	// the edit survives a restart. Clearing it hands the row back to the importer.
	CatalogEditedAt *time.Time

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

	// Optional cycle metadata (all default-empty; edited on the detail page).
	// Notes: free-form markdown. Focus: mirrors SessionJournal.Focus
	// (strength|endurance|technique|projects|general). Label: comma-separated tag
	// chips (same pattern as templates/exercises). Goal: one-line aspiration.
	Notes string `gorm:"type:text"`
	Focus string `gorm:"size:32;not null;default:''"`
	Label string `gorm:"size:128;not null;default:''"`
	Goal  string `gorm:"size:255;not null;default:''"`

	WeekdayMappings []TrainingCycleWeekdayMapping `gorm:"constraint:OnDelete:CASCADE;"`
	CycleGoals      []CycleGoal                   `gorm:"constraint:OnDelete:CASCADE;"`
}

// CycleGoal is one free-text "before → after" goal for a training cycle. A cycle
// can have several (the plan's engine). Hard-deleted with the cycle in
// handleTrainingCycleDelete — the CASCADE tag alone isn't relied on (soft-deletes
// don't trigger FK cascade in this codebase).
type CycleGoal struct {
	gorm.Model

	OwnerID         uint   `gorm:"index;not null"`
	TrainingCycleID uint   `gorm:"index;not null"`
	Before          string `gorm:"size:255;not null;default:''"`
	After           string `gorm:"size:255;not null;default:''"`
	// How is the strategy to close the before→after gap (e.g. "one lead session a week").
	How        string `gorm:"size:255;not null;default:''"`
	OrderIndex int    `gorm:"not null;default:0"`
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

	OwnerID           uint   `gorm:"index;not null"`
	TrainingCycleID   uint   `gorm:"index;not null"`
	LibraryExerciseID *uint  `gorm:"index"`    // preferred match key
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
	// ExercisesMaterialised is set true once template exercises have been copied to RunExercise rows for log editing.
	ExercisesMaterialised bool `gorm:"not null;default:false"`

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

// ExercisePlannedSet records a per-set target (reps + weight) on an exercise.
// Used for pre-planned progressive loading (e.g. set 1: 5×60kg, set 2: 3×80kg).
type ExercisePlannedSet struct {
	gorm.Model
	OwnerID    uint `gorm:"index;not null"`
	ExerciseID uint `gorm:"index:idx_exercise_planned_set,unique,not null"`
	SetIndex   int  `gorm:"index:idx_exercise_planned_set,unique,not null"` // 1-based
	Reps       int
	WeightKg   float64
}

// ManualExerciseSetLog records per-set reps and weight for a manual exercise.
type ManualExerciseSetLog struct {
	gorm.Model
	OwnerID    uint `gorm:"index;not null"`
	RunID      uint `gorm:"index;not null"`
	ExerciseID uint `gorm:"index;not null"`
	SetIndex   int  `gorm:"not null"` // 1-based
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

	// SessionNotes holds free-form, general notes for the whole session (markdown),
	// captured live from the open-session overview — distinct from the structured
	// WentWell/NextFocus reflection prompts.
	SessionNotes string `gorm:"type:text"`
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

	Kind     string `gorm:"not null"`                    // "boulder" | "sport" | "trad"
	Setting  string `gorm:"size:32;not null;default:''"` // "indoor" | "outdoor"
	Subtype  string `gorm:"size:32;not null;default:''"` // boulder+indoor: "commercial"|"board"; others: ""
	Grade    string `gorm:"size:32"`
	Focus    string `gorm:"type:text"` // pre-climb intention
	Thoughts string `gorm:"type:text"` // post-climb reflection
	Style    string `gorm:"size:32"`   // "onsight"|"flash"|"redpoint"|"hangdog"|"repeat"|"attempt"
	// RopeStyle is set for sport/trad ticks: "lead"|"top_rope"|"auto_belay"|"second"
	RopeStyle string `gorm:"size:32;not null;default:''"`
	Attempts  int
	Sent      bool
	// DurationSeconds is optional time on the wall for this climb — mainly for
	// ungraded traverse / ARC laps, where how long you stayed on is the point.
	DurationSeconds int `gorm:"not null;default:0"`
	Stars           int `gorm:"default:0"` // 0 = unrated; 1–3 = quality rating

	OrderIndex int `gorm:"default:0"`
}

// CalendarEvent marks a period on the user's calendar (trip, injury, rest, etc.).
// When Blocks is true, the cycle planner warns about or skips sessions on those dates.
type CalendarEvent struct {
	gorm.Model
	OwnerID   uint      `gorm:"index;not null"`
	Title     string    `gorm:"size:128;not null"`
	Kind      string    `gorm:"size:32;not null"` // "trip"|"injury"|"rest"|"competition"|"other"
	StartDate time.Time `gorm:"not null"`         // local midnight, inclusive
	EndDate   time.Time `gorm:"not null"`         // local midnight, inclusive
	Notes     string    `gorm:"type:text"`
	Blocks    bool      `gorm:"default:true"`
	// TrainingCycleID marks the deload and rest events a cycle builder created, so
	// deleting the cycle can take them with it instead of leaving them stranded on the
	// calendar. Deliberately a bare pointer with no association tag: foreign keys are
	// enforced (_foreign_keys=on), and a declared association would add a REFERENCES
	// clause that makes the cycle's hard delete fail. Events created by hand leave it
	// nil and are never touched by a cycle delete.
	TrainingCycleID *uint `gorm:"index"`
}
