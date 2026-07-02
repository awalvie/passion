package pages

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/yuin/goldmark"

	"passion/db"
)

// ---------------------------------------------------------------------------
// View types (used as fields on Params structs and referenced by templates)
// ---------------------------------------------------------------------------

type DashboardSession struct {
	ID            uint
	DateLabel     string
	TemplateName  string
	ExerciseCount int
	CycleName     string
	// Color is validated hex (#rrggbb) or empty.
	Color string
	Done  bool
}

type ActiveRunView struct {
	RunID         uint
	TemplateName  string
	Color         string
	StartedLabel  string
	StartedAtUnix int64
}

type DashboardDayGroup struct {
	DayLabel string
	Sessions []DashboardSession
}

type CalendarCellSession struct {
	ID        uint
	Name      string
	Color     string
	CycleName string
	Done      bool
	Label     string
}

// CalendarEventView is a user-defined calendar event (trip, injury, etc.) for template rendering.
type CalendarEventView struct {
	ID         uint
	Title      string
	Kind       string
	Color      string
	StartKey   string // YYYY-MM-DD
	EndKey     string
	StartLabel string // "May 20"
	EndLabel   string
	Notes      string
	Blocks     bool
	SpanDays   int
}

// CalendarEventColor returns the accent hex color for a given event kind.
func CalendarEventColor(kind string) string {
	switch kind {
	case "trip":
		return "#f97316"
	case "injury":
		return "#ef4444"
	case "rest":
		return "#8b5cf6"
	case "competition":
		return "#3b82f6"
	default:
		return "#6b7280"
	}
}

// CycleConflictView describes a blocking event that overlaps a proposed cycle.
type CycleConflictView struct {
	EventTitle    string
	EventColor    string
	AffectedDates []string
	AffectedLabel string // comma-joined AffectedDates for display
	AffectedCount int
}

// ConflictWarningView is used by the dashboard banner to surface upcoming conflicts.
type ConflictWarningView struct {
	EventTitle   string
	EventColor   string
	StartLabel   string
	EndLabel     string
	SessionCount int
	CycleName    string
	CycleID      uint
}

type CalendarCell struct {
	Day               int
	InMonth           bool
	FirstSessionColor string
	DateKey           string
	Sessions          []CalendarCellSession
	CompletedCount    int
	UnscheduledCount  int
	Events            []CalendarEventView
}

type WeekColumn struct {
	DateKey   string
	DateLabel string
	Sessions  []DashboardSession
}

type PaginationView struct {
	Page       int
	TotalPages int
	PrevURL    string
	NextURL    string
	HasPrev    bool
	HasNext    bool
}

type CycleDayCellView struct {
	DateKey              string
	DateLabel            string
	DayNum               string
	DayNumber            int
	IsWeekend            bool
	SessionID            uint
	SessionTemplateName  string
	SessionTemplateColor string
	HasSession           bool
	Events               []CalendarEventView
}

type CycleWeekRowView struct {
	WeekNumber int
	Cells      []CycleDayCellView
}

// RunStepOption is one selectable option under an exercise_catalog menu step.
type RunStepOption struct {
	ExerciseID             uint
	Name                   string
	Media                  []db.ExerciseMedia
	Kind                   string
	SessionDurationSeconds int
	Sets                   int
	Reps                   int
	Notes                  string
}

func (o RunStepOption) ThumbnailURL() string {
	p := db.PrimaryMedia(o.Media)
	if p != nil {
		return p.ThumbnailURL
	}
	return ""
}

func (o RunStepOption) VideoURL() string {
	p := db.PrimaryMedia(o.Media)
	if p != nil {
		return p.VideoURL
	}
	return ""
}

type RunStep struct {
	ExerciseID             uint
	Name                   string
	Media                  []db.ExerciseMedia
	Kind                   string
	SessionDurationSeconds int
	Sets                   int
	Reps                   int
	WeightKg               float64

	RepSeconds     int
	RepRestSeconds int
	SetRestSeconds int
	PrepSeconds    int
	RungSeconds    string
	TemplateNotes  string
	Status         string

	// Set from RunExerciseCompletion for completed/skipped exercises.
	ElapsedSeconds int
	RunNotes       string

	ActivityID   uint
	ActivityName string

	// CatalogOptions is set only when Kind is exercise_catalog (menu step).
	CatalogOptions []RunStepOption

	// PlannedSets is populated for reps_and_sets exercises that have per-set targets configured.
	PlannedSets []ExercisePlannedSetView
}

func (rs RunStep) VideoURL() string {
	p := db.PrimaryMedia(rs.Media)
	if p != nil {
		return p.VideoURL
	}
	return ""
}

func (rs RunStep) ThumbnailURL() string {
	p := db.PrimaryMedia(rs.Media)
	if p != nil {
		return p.ThumbnailURL
	}
	return ""
}

func (rs RunStep) HasMedia() bool {
	for _, m := range rs.Media {
		if m.VideoURL != "" || m.ThumbnailURL != "" {
			return true
		}
	}
	return false
}

// RunActivityGroup groups run steps by their parent activity for sidebar display.
type RunActivityGroup struct {
	ActivityID uint
	Name       string
	Type       string
	Steps      []RunStep
	IsCurrent  bool
}

type HistoryRunView struct {
	ID             uint
	DateLabel      string
	RelativeDate   string
	MonthGroup     string
	TemplateName   string
	Color          string
	DurationLabel  string
	Status         string
	RunID          uint
	JournalEntryID uint // 0 if no journal exists yet
	CompletedCount int
	TotalCount     int
}

type RunSummaryExercise struct {
	Name            string
	Kind            string
	Sets            int
	Reps            int
	WeightKg        float64
	RepSeconds      int
	SessionDuration int
	Status          string // "completed" | "skipped" | "pending"
	ElapsedSeconds  int
	Notes           string
}

type RunSummaryActivity struct {
	Name      string
	Exercises []RunSummaryExercise
}

type RunSummaryView struct {
	RunID          uint
	TemplateName   string
	Color          string
	DateLabel      string
	DurationLabel  string
	IsOpen         bool
	Completed      bool
	CompletedCount int
	SkippedCount   int
	TotalCount     int
	Activities     []RunSummaryActivity
}

// ExerciseHistoryItem is one past completion of an exercise, used in hints, popups, and history pages.
type ExerciseHistoryItem struct {
	Date           string
	TemplateName   string
	ActualSets     int
	ActualReps     int
	ActualWeightKg float64
	ElapsedSeconds int
	Notes          string
	Status         string // completed | skipped
}

func (e ExerciseHistoryItem) HasActuals() bool {
	return e.ActualSets > 0 || e.ActualReps > 0 || e.ActualWeightKg > 0
}

type ExerciseHistoryHintView struct {
	ExerciseID   uint
	ExerciseName string
	Items        []ExerciseHistoryItem // last 3
}

type ExerciseHistoryPopupView struct {
	ExerciseID        uint
	ExerciseName      string
	LibraryExerciseID uint                  // 0 if not linked
	Items             []ExerciseHistoryItem // last 10
}

type ExerciseHistoryPageParams struct {
	Base
	ExerciseName string
	LibraryID    uint
	Items        []ExerciseHistoryItem
}

type PrevRunRow struct {
	DateLabel     string
	DurationLabel string
	DoneCount     int
	TotalCount    int
	Pct           int // 0–100 for CSS bar width
}

type RunSummaryPageParams struct {
	Base
	Summary  RunSummaryView
	PrevRuns []PrevRunRow
}

type HistoryStatsView struct {
	TotalRuns         int
	TotalTimeLabel    string
	AvgDurationLabel  string
	ThisWeekCount     int
	ThisMonthCount    int
	CurrentStreak     int
	LongestStreak     int
	MostUsedTemplate  string
	MostUsedColor     string
	WeeklyChartJSON   template.JS
	WeeklyTrendJSON   template.JS
	TemplateBreakdown []TemplateBreakdownItem
	HeatmapJSON       template.JS
}

type TemplateBreakdownItem struct {
	TemplateName string
	Color        string
	Count        int
	Pct          int
}

// ExercisesFragmentData is passed to the exercises_container fragment.
type ExercisesFragmentData struct {
	Activity         db.Activity
	LibraryExercises []db.LibraryExercise
	// PlannedSets maps ExerciseID → planned set rows for per-set target display/editing.
	PlannedSets map[uint][]ExercisePlannedSetView
}

// TemplateFragmentData is passed to preview_container and scheduled_session_preview fragments.
type TemplateFragmentData struct {
	Template *db.SessionTemplate
}

// ---------------------------------------------------------------------------
// Per-page Params structs
// ---------------------------------------------------------------------------

type LoginParams struct {
	Base
	AuthFormError string
}

type SignupParams struct {
	Base
	AuthFormError string
}

// DraftLogEntryView represents an abandoned manual log draft shown on the dashboard.
type DraftLogEntryView struct {
	RunID     uint
	DateLabel string // relative label, e.g. "Today", "Yesterday", "Mon Jan 20"
}

type DashboardParams struct {
	Base
	Templates        []db.SessionTemplate
	ActiveRuns       []ActiveRunView
	DraftLogEntries  []DraftLogEntryView
	WeekSessions     []DashboardSession
	WeekDayGroups    []DashboardDayGroup
	WeekLabel        string
	WeekPrevURL      string
	WeekNextURL      string
	CalendarCells    []CalendarCell
	CalendarMonth    string
	CalendarYear     string
	CalendarWeekday  []string
	MonthPrevURL     string
	MonthNextURL     string
	ConflictWarnings []ConflictWarningView
}

type RunParams struct {
	Base
	RunID                uint
	RunTemplateName      string
	RunTotalSteps        int
	RunCompleted         bool
	RunCurrentStepNum    int
	RunSessionSeconds    int
	RunIsTrial           bool
	RunTemplateID        uint
	RunIsOpen            bool
	RunIsDraft           bool
	RunCustomName        string
	StartedAtUnix        int64
	RunLibraryExercises  []db.LibraryExercise
	RunActivityTemplates []db.ActivityTemplate
	CurrentStep          RunStep
	RunSteps             []RunStep
	RunActivityGroups    []RunActivityGroup

	// Open-session overview progress (running, non-draft).
	RunDoneCount         int
	RunCurrentExerciseID uint
	RunSessionNotes      string
}

type WeeklyColorSegment struct {
	Color     string
	Name      string
	Count     int
	HeightPct int // share of this bar's height (0-100 within the bar)
}

type WeeklyTrendItem struct {
	Label     string
	Count     int
	HeightPct int
	Color     string
	Segments  []WeeklyColorSegment
}

type HistoryParams struct {
	Base
	HistoryRuns  []HistoryRunView
	HistoryStats HistoryStatsView
	HistoryRange string
	WeeklyTrend  []WeeklyTrendItem
	Climbing     ClimbingAnalyticsView
}

// ClimbingGradeRow is one grade bar in a discipline pyramid; SentPct/AttemptPct
// are bar widths relative to the busiest grade in that discipline.
type ClimbingGradeRow struct {
	Grade      string
	Sent       int
	Total      int
	SentPct    int
	AttemptPct int
}

type ClimbingDisciplineView struct {
	Label      string
	Rows       []ClimbingGradeRow
	MoreGrades int
}

type ClimbingAnalyticsView struct {
	HasData        bool
	HasEverClimbed bool
	TotalClimbs    int
	SessionCount   int
	SendRate       int
	HardestBoulder string
	HardestRoute   string
	Disciplines    []ClimbingDisciplineView
	HasSplits      bool
	IndoorPct      int
	OutdoorPct     int
	HasBoardSplit  bool
	CommercialPct  int
	BoardPct       int
}

type ClimbingTickView struct {
	ID         uint
	RunID      uint
	ExerciseID uint
	Kind       string // "Boulder" | "Sport" | "Trad"
	KindRaw    string // "boulder" | "sport" | "trad"
	Setting    string // "Indoor" | "Outdoor"
	SettingRaw string // "indoor" | "outdoor"
	Subtype    string // display: board kind name or ""
	SubtypeRaw string // "kilter"|"moon"|"tension"|"spray"|"custom"|"" (legacy: "board"|"commercial")
	IsBoard    bool   // true when SubtypeRaw indicates a board session
	Grade      string
	Focus      string // pre-climb intention
	Thoughts   string
	Style      string // display: "Onsight", "Flash", "Redpoint", "Hangdog", "Repeat"
	StyleClass string // CSS modifier
	StyleRaw   string // raw DB value — used in edit form radio
	StyleIcon  string // Lucide icon name for summary chip
	// Method is set for sport/trad ticks: how the route was climbed
	RopeStyle      string // display: "Lead", "Top Rope", "Auto-belay", "Follow"
	RopeStyleClass string // CSS modifier
	RopeStyleRaw   string // raw DB value — used in edit form radio ("lead"|"top_rope"|"auto_belay"|"follow")
	RopeStyleIcon  string // Lucide icon name
	Attempts       int
	Sent           bool
	Stars          int // 0–3
}

type ExerciseTicksParams struct {
	RunID      uint
	ExerciseID uint
	Ticks      []ClimbingTickView
	// Grade system keys: "font"|"v_scale" for boulder; "french"|"yds" for routes.
	BoulderGradeSystem string
	RouteGradeSystem   string

	// Seed values for the new-tick form, inherited from the latest tick in the
	// run (or user defaults for the first tick). "Log again" overrides these
	// from a specific tick.
	SeedKind      string
	SeedSetting   string
	SeedSubtype   string
	SeedIsBoard   bool
	SeedRopeStyle string
	SeedGrade     string

	// Live session header totals (computed over the run's ticks).
	HeaderClimbs  int
	HeaderSends   int
	HeaderHardest string
}

type ManualExerciseSetLogView struct {
	SetIndex int
	Reps     int
	WeightKg float64
}

type ExercisePlannedSetView struct {
	SetIndex int
	Reps     int
	WeightKg float64
}

type ClimbingExerciseMetaView struct {
	Type      string // "indoor_bouldering" | "board" | "gym_routes" | "outdoor_bouldering" | "sport" | "trad" | "top_rope"
	BoardKind string // "kilter" | "moon" | "tension" | "spray" | "custom"
	BoardID   uint   // 0 = unset; non-zero = configured ClimbingBoard ID
}

type ManualExerciseView struct {
	ExerciseID     uint
	RunID          uint
	Name           string
	Kind           string // "reps_and_sets" | "climbing" | "session" | "timed_reps"
	ActualSets     int
	ActualReps     int
	ActualWeightKg float64
	ElapsedMinutes int
	Notes          string
	PerSetMode     bool
	SetLogs        []ManualExerciseSetLogView
	ClimbingMeta   ClimbingExerciseMetaView
}

type ClimbingVenueView struct {
	ID       uint
	Name     string
	Kind     string // "Commercial" | "Outdoor"
	Location string // optional city / area
}

type ClimbingBoardView struct {
	ID        uint
	BoardType string // "Kilter" | "Moon" | "Tension" | "Spray" | "Custom"
	Name      string
	Label     string // e.g. "Home Kilter" if named, otherwise "Kilter Board"
}

type ProfileParams struct {
	Base
	UserProfile      *db.User
	ProfileFormError string
	PasswordError    string
	PasswordSuccess  bool
	Venues           []ClimbingVenueView
	Boards           []ClimbingBoardView
}

type TemplateListParams struct {
	Base
	Templates       []db.SessionTemplate
	SourceFilter    string
	TagFilter       string
	DistinctSources []string
	DistinctTags    []string
}

type NewTemplateParams struct {
	Base
}

type TemplateEditParams struct {
	Base
	Template          *db.SessionTemplate
	LibraryExercises  []db.LibraryExercise
	ActivityTemplates []db.ActivityTemplate
}

type TrainingCycleListParams struct {
	Base
	TrainingCycles []db.TrainingCycle
}

type NewTrainingCycleParams struct {
	Base
	Templates  []db.SessionTemplate
	Conflicts  []CycleConflictView
	FormValues map[string]string
}

// CycleWeekTargetView holds resolved targets for one week of a per-week override.
type CycleWeekTargetView struct {
	Week        int
	Sets        int
	Reps        int
	WeightKg    float64
	RepSeconds  int
	HasOverride bool // true if a CycleExerciseWeekOverride row exists for this week
}

// CycleExerciseOverrideView represents one exercise row in the cycle targets panel.
type CycleExerciseOverrideView struct {
	// identity
	LibraryExerciseID uint
	ExerciseName      string
	Kind              string // "reps_and_sets" | "timed_reps" | "exercise_catalog"
	// planned defaults (from template)
	PlannedSets     int
	PlannedReps     int
	PlannedWeightKg float64
	PlannedRepSecs  int
	// current cycle-level override values
	OverrideSets     int
	OverrideReps     int
	OverrideWeightKg float64
	OverrideRepSecs  int
	HasOverride      bool
	// per-week variation
	VariesByWeek  bool
	WeekOverrides []CycleWeekTargetView // len == CycleWeeks, indexed by week-1
}

type TrainingCycleDetailParams struct {
	Base
	CycleID            uint
	CycleName          string
	CycleWeeks         int
	CycleWeekdayLabels []string
	CycleTemplates     []db.SessionTemplate
	CycleRows          []CycleWeekRowView
	TotalScheduled     int
	ExerciseOverrides  []CycleExerciseOverrideView
	Events             []CalendarEventView
}

type CalendarPageParams struct {
	Base
	Cells           []CalendarCell
	AllEvents       []CalendarEventView
	CalendarMonth   string
	CalendarYear    string
	CalendarWeekday []string
	MonthPrevURL    string
	MonthNextURL    string
}

type ActivityTemplateListParams struct {
	Base
	ActivityTemplates []db.ActivityTemplate
	SourceFilter      string
	TagFilter         string
	DistinctSources   []string
	DistinctTags      []string
}

type NewActivityTemplateParams struct {
	Base
}

type ActivityTemplateEditParams struct {
	Base
	Template         *db.ActivityTemplate
	LibraryExercises []db.LibraryExercise
}

type LibraryListParams struct {
	Base
	LibraryExercises []db.LibraryExercise
	Pagination       *PaginationView
	LibrarySearch    string
	LibraryKind      string
	LibrarySort      string
	LibrarySource    string
	LibraryTag       string
	DistinctSources  []string
	DistinctTags     []string
}

type NewLibraryExerciseParams struct {
	Base
	LibraryExercises       []db.LibraryExercise
	LibraryExerciseFormErr string
}

type EditLibraryExerciseParams struct {
	Base
	LibraryExercise         *db.LibraryExercise
	LibraryExerciseChildren []db.LibraryExercise
	LibraryExercises        []db.LibraryExercise
	LibraryExerciseFormErr  string
}

// SessionExerciseSummaryView is a unified exercise row for the summary page,
// covering both template-based and manually-logged exercises.
type SessionExerciseSummaryView struct {
	Name           string
	Kind           string
	Status         string // "completed"|"skipped"|"pending"|"" (blank for manual)
	ActualSets     int
	ActualReps     int
	ActualWeightKg float64
	ElapsedMinutes int
	Notes          string
	PerSetMode     bool
	SetLogs        []ManualExerciseSetLogView
	ClimbingType   string
}

type TrainingLogSummaryParams struct {
	Base
	JournalID    uint
	Title        string
	DateLabel    string // e.g. "Mon, 13 Jan 2026"
	IsRunLinked  bool
	RunInfo      string // e.g. "Strength Base · Jan 5th"
	SleepScore   int
	Energy       int
	RPE          int
	Focus        string
	Location     string
	VenueName    string
	WentWell     string
	NextFocus    string
	SessionNotes string
	Exercises    []SessionExerciseSummaryView
}

type TrainingLogNewParams struct {
	Base
	JournalID   uint   // 0 = new entry; non-zero = editing an existing entry
	IsRunLinked bool   // true when editing a journal attached to a run (title/date read-only)
	RunInfo     string // display string shown when IsRunLinked, e.g. "Strength Base · Jan 5th"
	DateValue   string // pre-filled date in "2006-01-02" format
	Title       string
	SleepScore  int
	Energy      int
	RPE         int
	Focus       string
	Location    string
	WentWell    string
	NextFocus   string
	FormErr     string

	// Draft run fields (set when creating a new manual entry with exercises)
	DraftRunID        uint
	OpenExerciseID    uint // exercise ID to auto-open in the list (newly added)
	LibraryExercises  []db.LibraryExercise
	ActivityTemplates []db.ActivityTemplate
	Exercises         []ManualExerciseView
	Venues            []ClimbingVenueView
	Boards            []ClimbingBoardView
	VenueName         string
	// TemplateActivities holds read-only exercise data from the session template
	// for non-manual runs shown on the edit page.
	TemplateActivities []RunSummaryActivity
}

type TrainingLogEntryView struct {
	RunID          uint
	JournalEntryID uint      // ID of the SessionJournal row; 0 if no journal yet
	SortTime       time.Time // used for sorting; not rendered
	DateLabel      string
	TemplateName   string
	Color          string
	DurationLabel  string
	MonthGroup     string // e.g. "May 2026" — used for grouping in template
	WeekGroup      string // e.g. "May 12 – 18" — used for week-based grouping
	IsStandalone   bool   // true for entries created directly on /training-log/new
	IsManual       bool   // true for runs created via manual log entry
	HasJournal     bool
	SleepScore     int    // 0 = not recorded
	Energy         int    // 0 = not recorded
	RPE            int    // 0 = not recorded
	SleepPct       int    // SleepScore * 20, for CSS progress bar width
	EnergyPct      int    // Energy * 20
	RPEPct         int    // RPE * 10
	Focus          string // capitalised display value, e.g. "Strength"
	Location       string // "Indoor" | "Outdoor"
	WentWellHTML   template.HTML
	NextFocusHTML  template.HTML
	JournalTeaser  string // plain-text first line of WentWell, truncated to 120 chars
	// Tick summary (non-zero when this run has climbing ticks)
	TickSummaryLabel string // e.g. "3 boulders · 2 routes · 4 sends"
	ExerciseCount    int    // total exercises in the run
}

type AdherenceWeekView struct {
	WeekLabel string // e.g. "Apr 28 – May 4"
	Planned   int
	Completed int
	Pct       int    // 0–100 for the progress bar
	PctLabel  string // e.g. "3 / 4"
}

type TrainingLogStatsView struct {
	TotalSessions int
	ThisMonth     int
	ThisWeek      int
	CurrentStreak int
	AvgSleep      string // "3.8 / 5" or "—"
	AvgEnergy     string
	AvgRPE        string
	TopFocus      string // most common focus, or ""
	IndoorCount   int
	OutdoorCount  int
}

type TrainingLogPageParams struct {
	Base
	Entries   []TrainingLogEntryView
	Adherence []AdherenceWeekView
	CycleName string
	Stats     TrainingLogStatsView
}

type JournalFormParams struct {
	RunID        uint
	SleepScore   int
	Energy       int
	RPE          int
	Focus        string
	Location     string
	WentWell     string
	NextFocus    string
	SessionNotes string
	Saved        bool // true → render read-only view; false → render editable form
}

// ---------------------------------------------------------------------------
// Pages struct and constructor
// ---------------------------------------------------------------------------

type Pages struct {
	pageTemplates     map[string]*template.Template
	fragmentTemplates *template.Template
	logger            *slog.Logger
}

func NewPages(logger *slog.Logger) (*Pages, error) {
	funcMap := buildFuncMap()

	baseFiles := []string{
		filepath.Join("templates", "layouts", "base.html"),
		filepath.Join("templates", "layouts", "fragments", "topbar.html"),
		filepath.Join("templates", "layouts", "fragments", "footer.html"),
		filepath.Join("templates", "fragments", "activities_container.html"),
		filepath.Join("templates", "fragments", "exercises_container.html"),
		filepath.Join("templates", "fragments", "preview_container.html"),
		filepath.Join("templates", "fragments", "scheduled_session_preview.html"),
		filepath.Join("templates", "fragments", "activity_template_exercises_container.html"),
		filepath.Join("templates", "fragments", "venues_list.html"),
		filepath.Join("templates", "fragments", "boards_list.html"),
		filepath.Join("templates", "fragments", "manual_exercises.html"),
		filepath.Join("templates", "fragments", "open_exercise_panel.html"),
		filepath.Join("templates", "fragments", "open_template_panel.html"),
	}

	fragmentFiles := []string{
		filepath.Join("templates", "fragments", "activities_container.html"),
		filepath.Join("templates", "fragments", "exercises_container.html"),
		filepath.Join("templates", "fragments", "preview_container.html"),
		filepath.Join("templates", "fragments", "scheduled_session_preview.html"),
		filepath.Join("templates", "fragments", "activity_template_exercises_container.html"),
		filepath.Join("templates", "fragments", "start_session_picker.html"),
		filepath.Join("templates", "fragments", "run_summary.html"),
		filepath.Join("templates", "fragments", "exercise_history_hint.html"),
		filepath.Join("templates", "fragments", "exercise_history_popup.html"),
		filepath.Join("templates", "fragments", "exercise_divergence_hint.html"),
		filepath.Join("templates", "fragments", "journal_form.html"),
		filepath.Join("templates", "fragments", "run_ticks.html"),
		filepath.Join("templates", "fragments", "manual_exercises.html"),
		filepath.Join("templates", "fragments", "planned_sets.html"),
		filepath.Join("templates", "fragments", "venues_list.html"),
		filepath.Join("templates", "fragments", "boards_list.html"),
	}

	fragmentTpl := template.New("fragments").Funcs(funcMap)
	var err error
	fragmentTpl, err = fragmentTpl.ParseFiles(fragmentFiles...)
	if err != nil {
		return nil, err
	}

	parsePage := func(pageFile string) (*template.Template, error) {
		files := append([]string{}, baseFiles...)
		files = append(files, pageFile)
		tpl := template.New("root").Funcs(funcMap)
		return tpl.ParseFiles(files...)
	}

	pageTemplates := make(map[string]*template.Template)
	pages := []struct {
		key  string
		file string
	}{
		{"pages/dashboard_content", "dashboard.html"},
		{"pages/templates_content", "templates.html"},
		{"pages/new_template_content", "new_template.html"},
		{"pages/template_edit_content", "template_edit.html"},
		{"pages/training_cycles_content", "training_cycles.html"},
		{"pages/new_cycle_content", "new_cycle.html"},
		{"pages/training_cycle_detail_content", "training_cycle_detail.html"},
		{"pages/run_content", "run.html"},
		{"pages/open_session_content", "open_session.html"},
		{"pages/activity_templates_content", "activity_templates.html"},
		{"pages/new_activity_template_content", "new_activity_template.html"},
		{"pages/activity_template_edit_content", "activity_template_edit.html"},
		{"pages/exercise_library_content", "exercise_library.html"},
		{"pages/new_exercise_library_content", "new_exercise_library.html"},
		{"pages/edit_exercise_library_content", "edit_exercise_library.html"},
		{"pages/login_content", "login.html"},
		{"pages/signup_content", "signup.html"},
		{"pages/profile_content", "profile.html"},
		{"pages/history_content", "history.html"},
		{"pages/run_summary_content", "run_summary.html"},
		{"pages/exercise_history_content", "exercise_history.html"},
		{"pages/training_log_content", "training_log.html"},
		{"pages/training_log_new_content", "training_log_new.html"},
		{"pages/training_log_summary_content", "training_log_summary.html"},
		{"pages/calendar_content", "calendar.html"},
	}
	for _, p := range pages {
		pageTemplates[p.key], err = parsePage(filepath.Join("templates", p.file))
		if err != nil {
			return nil, err
		}
	}

	return &Pages{
		pageTemplates:     pageTemplates,
		fragmentTemplates: fragmentTpl,
		logger:            logger,
	}, nil
}

// ---------------------------------------------------------------------------
// Base — embedded in every Params struct so templates can access shared fields
// directly (e.g. {{.Title}}, {{.Authenticated}}, {{.CurrentUserEmail}}).
// ---------------------------------------------------------------------------

// Base holds fields read by the base layout and topbar templates.
// Render methods set these fields; callers only supply page-specific data.
type Base struct {
	Title            string
	Authenticated    bool
	IsAuthPage       bool
	WideLayout       bool
	CurrentUserEmail string
}

// ---------------------------------------------------------------------------
// Internal render helper
// ---------------------------------------------------------------------------

func (p *Pages) renderPage(w http.ResponseWriter, key string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tpl := p.pageTemplates[key]
	if tpl == nil {
		p.logger.Error("unknown page template", "template", key)
		http.Error(w, "unknown page template: "+key, http.StatusInternalServerError)
		return
	}
	if err := tpl.ExecuteTemplate(w, "layouts/base", data); err != nil {
		p.logger.Error("failed to render page template", "template", key, "error", err)
	}
}

// RenderFragment renders a named fragment template.
func (p *Pages) RenderFragment(w http.ResponseWriter, templateName string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := p.fragmentTemplates.ExecuteTemplate(w, templateName, data); err != nil {
		p.logger.Error("failed to render fragment template", "template", templateName, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type StartSessionPickerParams struct {
	Templates []db.SessionTemplate
}

func (p *Pages) StartSessionPicker(w http.ResponseWriter, params StartSessionPickerParams) {
	p.RenderFragment(w, "start_session_picker.html", params)
}

// ---------------------------------------------------------------------------
// Per-page render methods
// ---------------------------------------------------------------------------

func (p *Pages) Login(w http.ResponseWriter, params LoginParams) {
	params.Title = "Log in"
	params.IsAuthPage = true
	p.renderPage(w, "pages/login_content", params)
}

func (p *Pages) Signup(w http.ResponseWriter, params SignupParams) {
	params.Title = "Sign up"
	params.IsAuthPage = true
	p.renderPage(w, "pages/signup_content", params)
}

func (p *Pages) Dashboard(w http.ResponseWriter, params DashboardParams) {
	params.Title = "Dashboard"
	params.Authenticated = true
	p.renderPage(w, "pages/dashboard_content", params)
}

func (p *Pages) Run(w http.ResponseWriter, params RunParams) {
	params.Title = "Session Run"
	params.Authenticated = true
	params.WideLayout = true
	p.renderPage(w, "pages/run_content", params)
}

func (p *Pages) OpenSession(w http.ResponseWriter, params RunParams) {
	params.Title = "Open Session"
	params.Authenticated = true
	p.renderPage(w, "pages/open_session_content", params)
}

func (p *Pages) History(w http.ResponseWriter, params HistoryParams) {
	params.Title = "History"
	params.Authenticated = true
	p.renderPage(w, "pages/history_content", params)
}

func (p *Pages) RunSummaryPage(w http.ResponseWriter, params RunSummaryPageParams) {
	params.Authenticated = true
	p.renderPage(w, "pages/run_summary_content", params)
}

func (p *Pages) ExerciseHistoryPage(w http.ResponseWriter, params ExerciseHistoryPageParams) {
	params.Authenticated = true
	p.renderPage(w, "pages/exercise_history_content", params)
}

func (p *Pages) Profile(w http.ResponseWriter, params ProfileParams) {
	params.Title = "Profile"
	params.Authenticated = true
	p.renderPage(w, "pages/profile_content", params)
}

func (p *Pages) TemplateList(w http.ResponseWriter, params TemplateListParams) {
	params.Title = "Session Templates"
	params.Authenticated = true
	p.renderPage(w, "pages/templates_content", params)
}

func (p *Pages) NewTemplate(w http.ResponseWriter, params NewTemplateParams) {
	params.Title = "New Session Template"
	params.Authenticated = true
	p.renderPage(w, "pages/new_template_content", params)
}

func (p *Pages) TemplateEdit(w http.ResponseWriter, params TemplateEditParams) {
	params.Title = "Edit Template"
	params.Authenticated = true
	p.renderPage(w, "pages/template_edit_content", params)
}

func (p *Pages) TrainingCycleList(w http.ResponseWriter, params TrainingCycleListParams) {
	params.Title = "Training Cycles"
	params.Authenticated = true
	p.renderPage(w, "pages/training_cycles_content", params)
}

func (p *Pages) NewTrainingCycle(w http.ResponseWriter, params NewTrainingCycleParams) {
	params.Title = "New Training Cycle"
	params.Authenticated = true
	p.renderPage(w, "pages/new_cycle_content", params)
}

func (p *Pages) TrainingCycleDetail(w http.ResponseWriter, params TrainingCycleDetailParams) {
	params.Title = "Training Cycle: " + params.CycleName
	params.Authenticated = true
	p.renderPage(w, "pages/training_cycle_detail_content", params)
}

func (p *Pages) NewActivityTemplate(w http.ResponseWriter, params NewActivityTemplateParams) {
	params.Title = "New Activity Template"
	params.Authenticated = true
	p.renderPage(w, "pages/new_activity_template_content", params)
}

func (p *Pages) ActivityTemplateList(w http.ResponseWriter, params ActivityTemplateListParams) {
	params.Title = "Activity Templates"
	params.Authenticated = true
	p.renderPage(w, "pages/activity_templates_content", params)
}

func (p *Pages) ActivityTemplateEdit(w http.ResponseWriter, params ActivityTemplateEditParams) {
	params.Title = "Edit Activity Template"
	params.Authenticated = true
	p.renderPage(w, "pages/activity_template_edit_content", params)
}

func (p *Pages) LibraryList(w http.ResponseWriter, params LibraryListParams) {
	params.Title = "Exercise library"
	params.Authenticated = true
	p.renderPage(w, "pages/exercise_library_content", params)
}

func (p *Pages) NewLibraryExercise(w http.ResponseWriter, params NewLibraryExerciseParams) {
	params.Title = "New saved exercise"
	params.Authenticated = true
	p.renderPage(w, "pages/new_exercise_library_content", params)
}

func (p *Pages) EditLibraryExercise(w http.ResponseWriter, params EditLibraryExerciseParams) {
	params.Title = "Edit saved exercise"
	params.Authenticated = true
	p.renderPage(w, "pages/edit_exercise_library_content", params)
}

func (p *Pages) TrainingLogSummaryPage(w http.ResponseWriter, params TrainingLogSummaryParams) {
	params.Authenticated = true
	if params.Title == "" {
		params.Title = "Session"
	}
	p.renderPage(w, "pages/training_log_summary_content", params)
}

func (p *Pages) TrainingLogNewPage(w http.ResponseWriter, params TrainingLogNewParams) {
	if params.JournalID > 0 {
		params.Title = "Edit Log Entry"
	} else {
		params.Title = "New Log Entry"
	}
	params.Authenticated = true
	p.renderPage(w, "pages/training_log_new_content", params)
}

func (p *Pages) CalendarPage(w http.ResponseWriter, params CalendarPageParams) {
	params.Title = "Calendar"
	params.Authenticated = true
	params.WideLayout = true
	p.renderPage(w, "pages/calendar_content", params)
}

func (p *Pages) TrainingLogPage(w http.ResponseWriter, params TrainingLogPageParams) {
	params.Title = "Training Log"
	params.Authenticated = true
	p.renderPage(w, "pages/training_log_content", params)
}

func (p *Pages) RenderJournalForm(w http.ResponseWriter, params JournalFormParams) {
	p.RenderFragment(w, "journal_form.html", params)
}

func (p *Pages) RenderExerciseTicks(w http.ResponseWriter, params ExerciseTicksParams) {
	p.RenderFragment(w, "run_ticks.html", params)
}

func (p *Pages) RenderManualExercisesContainer(w http.ResponseWriter, params TrainingLogNewParams) {
	p.RenderFragment(w, "manual_exercises.html", params)
}

func (p *Pages) RenderVenuesList(w http.ResponseWriter, params ProfileParams) {
	p.RenderFragment(w, "venues_list.html", params)
}

func (p *Pages) RenderBoardsList(w http.ResponseWriter, params ProfileParams) {
	p.RenderFragment(w, "boards_list.html", params)
}

// ---------------------------------------------------------------------------
// Template FuncMap
// ---------------------------------------------------------------------------

// formatExerciseSummary formats an exercise's parameters into a compact human-readable string.
// sessionSuffix appends " session" to duration strings (used for session-template exercises).
// catalogLabel is returned for exercise_catalog kind (e.g. "Exercise catalog" or "menu").
func formatExerciseSummary(kind string, sessionDurSec, sets, reps int, weightKg float64, repSec int, sessionSuffix bool, catalogLabel string) string {
	switch kind {
	case "session", "climbing":
		if sessionDurSec > 0 {
			h := sessionDurSec / 3600
			m := (sessionDurSec % 3600) / 60
			s := sessionDurSec % 60
			sfx := ""
			if sessionSuffix {
				sfx = " session"
			}
			if h > 0 && m == 0 && s == 0 {
				return fmt.Sprintf("%dh%s", h, sfx)
			}
			if h > 0 {
				return fmt.Sprintf("%dh %dm%s", h, m, sfx)
			}
			if m > 0 && s == 0 {
				return fmt.Sprintf("%dm%s", m, sfx)
			}
			if m > 0 {
				return fmt.Sprintf("%dm %ds%s", m, s, sfx)
			}
			return fmt.Sprintf("%ds%s", s, sfx)
		}
		if sessionSuffix {
			return "Session"
		}
		return ""
	case "exercise_catalog":
		return catalogLabel
	default:
		var parts []string
		if sets > 0 && reps > 0 {
			parts = append(parts, fmt.Sprintf("%d×%d", sets, reps))
		} else if sets > 0 {
			parts = append(parts, fmt.Sprintf("%d sets", sets))
		} else if reps > 0 {
			parts = append(parts, fmt.Sprintf("%d reps", reps))
		}
		if weightKg > 0 {
			if weightKg == float64(int(weightKg)) {
				parts = append(parts, fmt.Sprintf("%.0fkg", weightKg))
			} else {
				parts = append(parts, fmt.Sprintf("%.1fkg", weightKg))
			}
		}
		if repSec > 0 {
			parts = append(parts, fmt.Sprintf("%ds/rep", repSec))
		}
		return strings.Join(parts, " · ")
	}
}

var markdownEngine = goldmark.New()

func markdownToHTML(s string) template.HTML {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := markdownEngine.Convert([]byte(s), &buf); err != nil {
		return template.HTML(html.EscapeString(s))
	}
	return template.HTML(buf.String())
}

func youtubeEmbedURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	if host != "youtube.com" && host != "youtube-nocookie.com" && host != "m.youtube.com" && host != "youtu.be" {
		return ""
	}
	if host == "youtu.be" {
		id := strings.Trim(u.Path, "/")
		id = strings.Split(id, "/")[0]
		if id == "" || strings.Contains(id, ".") {
			return ""
		}
		return "https://www.youtube.com/embed/" + id + "?rel=0"
	}
	path := u.Path
	if strings.HasPrefix(path, "/embed/") {
		id := strings.TrimPrefix(path, "/embed/")
		id = strings.Split(id, "/")[0]
		if id == "" {
			return ""
		}
		return "https://www.youtube.com/embed/" + id + "?rel=0"
	}
	if path == "/watch" || strings.HasPrefix(path, "/watch/") {
		v := u.Query().Get("v")
		if v != "" {
			return "https://www.youtube.com/embed/" + v + "?rel=0"
		}
	}
	if strings.HasPrefix(path, "/shorts/") {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 2 && parts[0] == "shorts" && parts[1] != "" {
			return "https://www.youtube.com/embed/" + parts[1] + "?rel=0"
		}
	}
	if strings.HasPrefix(path, "/live/") {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 2 && parts[0] == "live" && parts[1] != "" {
			return "https://www.youtube.com/embed/" + parts[1] + "?rel=0"
		}
	}
	return ""
}

func buildFuncMap() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"pct": func(part, total int) int {
			if total == 0 {
				return 0
			}
			return part * 100 / total
		},
		"seq": func(start, end int) []int {
			if end < start {
				return []int{}
			}
			out := make([]int, 0, end-start+1)
			for i := start; i <= end; i++ {
				out = append(out, i)
			}
			return out
		},
		"durationHMS": func(sec int) map[string]int {
			if sec < 0 {
				sec = 0
			}
			return map[string]int{
				"h": sec / 3600,
				"m": (sec % 3600) / 60,
				"s": sec % 60,
			}
		},
		"durationMinutes": func(sec int) string {
			if sec <= 0 {
				return ""
			}
			mins := float64(sec) / 60.0
			if mins == float64(int(mins)) {
				return fmt.Sprintf("%d", int(mins))
			}
			return fmt.Sprintf("%.4g", mins)
		},
		"formatElapsed": func(sec int) string {
			if sec <= 0 {
				return ""
			}
			m := sec / 60
			s := sec % 60
			if m == 0 {
				return fmt.Sprintf("%ds", s)
			}
			if s == 0 {
				return fmt.Sprintf("%dm", m)
			}
			return fmt.Sprintf("%dm %ds", m, s)
		},
		"formatSessionDuration": func(sec int) string {
			if sec < 0 {
				sec = 0
			}
			h := sec / 3600
			m := (sec % 3600) / 60
			s := sec % 60
			if h > 0 && m == 0 && s == 0 {
				return fmt.Sprintf("%dh", h)
			}
			if h > 0 && s == 0 {
				return fmt.Sprintf("%dh %dm", h, m)
			}
			if h > 0 {
				return fmt.Sprintf("%dh %dm %ds", h, m, s)
			}
			if m > 0 && s == 0 {
				return fmt.Sprintf("%dm", m)
			}
			if m > 0 {
				return fmt.Sprintf("%dm %ds", m, s)
			}
			return fmt.Sprintf("%ds", s)
		},
		"runHeroAccent": func(exerciseID uint) string {
			palette := []string{
				"#2563eb", "#7c3aed", "#db2777", "#d97706", "#059669",
				"#0891b2", "#4f46e5", "#ca8a04", "#be123c",
			}
			if exerciseID == 0 {
				return palette[0]
			}
			return palette[int(exerciseID)%len(palette)]
		},
		"exercisesFragment": func(act db.Activity, lib []db.LibraryExercise) ExercisesFragmentData {
			return ExercisesFragmentData{Activity: act, LibraryExercises: lib}
		},
		"exerciseChildren": func(all []db.Exercise, parentID uint) []db.Exercise {
			var out []db.Exercise
			for _, e := range all {
				if e.ParentExerciseID != nil && *e.ParentExerciseID == parentID {
					out = append(out, e)
				}
			}
			slices.SortFunc(out, func(a, b db.Exercise) int {
				return cmp.Compare(a.OrderIndex, b.OrderIndex)
			})
			return out
		},
		"countRootExercises": func(all []db.Exercise) int {
			n := 0
			for _, e := range all {
				if e.ParentExerciseID == nil {
					n++
				}
			}
			return n
		},
		"exerciseSummary": func(ex db.Exercise) string {
			return formatExerciseSummary(ex.Kind, ex.SessionDurationSeconds, ex.Sets, ex.Reps, ex.WeightKg, ex.RepSeconds, true, "Exercise catalog")
		},
		"runStepSummary": func(rs RunStep) string {
			return formatExerciseSummary(rs.Kind, rs.SessionDurationSeconds, rs.Sets, rs.Reps, rs.WeightKg, rs.RepSeconds, true, "Exercise catalog")
		},
		"markdownHTML": func(s string) template.HTML {
			return markdownToHTML(s)
		},
		"splitTags": func(s string) []string {
			var out []string
			for _, part := range strings.Split(s, ",") {
				if t := strings.TrimSpace(part); t != "" {
					out = append(out, t)
				}
			}
			return out
		},
		"youtubeEmbedURL": youtubeEmbedURL,
		"libExJSON": func(ex db.LibraryExercise) string {
			b, _ := json.Marshal(struct {
				Name  string  `json:"name"`
				Kind  string  `json:"kind"`
				Sets  int     `json:"sets"`
				Reps  int     `json:"reps"`
				Wkg   float64 `json:"wkg"`
				Rs    int     `json:"rs"`
				Rrs   int     `json:"rrs"`
				Srs   int     `json:"srs"`
				Ps    int     `json:"ps"`
				Sds   int     `json:"sds"`
				Notes string  `json:"notes"`
			}{ex.Name, ex.Kind, ex.Sets, ex.Reps, ex.WeightKg,
				ex.RepSeconds, ex.RepRestSeconds, ex.SetRestSeconds,
				ex.PrepSeconds, ex.SessionDurationSeconds, ex.Notes})
			return string(b)
		},
		"activityTemplateExJSON": func(tpl db.ActivityTemplate) string {
			type row struct {
				Name  string  `json:"name"`
				Kind  string  `json:"kind"`
				Sets  int     `json:"sets"`
				Reps  int     `json:"reps"`
				Wkg   float64 `json:"wkg"`
				Rs    int     `json:"rs"`
				Rrs   int     `json:"rrs"`
				Srs   int     `json:"srs"`
				Ps    int     `json:"ps"`
				Sds   int     `json:"sds"`
				Notes string  `json:"notes"`
			}
			var out []row
			for _, e := range tpl.Exercises {
				if e.ParentExerciseID != nil {
					continue
				}
				out = append(out, row{e.Name, e.Kind, e.Sets, e.Reps, e.WeightKg,
					e.RepSeconds, e.RepRestSeconds, e.SetRestSeconds,
					e.PrepSeconds, e.SessionDurationSeconds, e.Notes})
			}
			if out == nil {
				out = []row{}
			}
			b, _ := json.Marshal(out)
			return string(b)
		},
		"libExerciseSummary": func(ex db.LibraryExercise) string {
			return formatExerciseSummary(ex.Kind, ex.SessionDurationSeconds, ex.Sets, ex.Reps, ex.WeightKg, ex.RepSeconds, false, "menu")
		},
		"libExerciseNotesSnippet": func(s string) string {
			// Replace newlines with spaces and truncate for use in preview.
			out := strings.ReplaceAll(s, "\n", " ")
			out = strings.Join(strings.Fields(out), " ")
			if len([]rune(out)) > 180 {
				runes := []rune(out)
				return string(runes[:180]) + "…"
			}
			return out
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict: odd number of args")
			}
			m := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict: key %d is not a string", i)
				}
				m[key] = values[i+1]
			}
			return m, nil
		},
	}
}
