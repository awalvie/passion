package pages

import (
	"bytes"
	"cmp"
	"fmt"
	"html"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

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
	RunID        uint
	TemplateName string
	Color        string
	StartedLabel string
}

type DashboardDayGroup struct {
	DayLabel string
	Sessions []DashboardSession
}

type CalendarCellSession struct {
	Name      string
	Color     string
	CycleName string
	Done      bool
}

type CalendarCell struct {
	Day               int
	InMonth           bool
	FirstSessionColor string
	DateKey           string
	Sessions          []CalendarCellSession
	CompletedCount    int
	UnscheduledCount  int
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

	ActivityID   uint
	ActivityName string

	// CatalogOptions is set only when Kind is exercise_catalog (menu step).
	CatalogOptions []RunStepOption
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
	ExerciseID uint
	ExerciseName string
	Items      []ExerciseHistoryItem // last 3
}

type ExerciseHistoryPopupView struct {
	ExerciseID       uint
	ExerciseName     string
	LibraryExerciseID uint // 0 if not linked
	Items            []ExerciseHistoryItem // last 10
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

type DashboardParams struct {
	Base
	Templates       []db.SessionTemplate
	ActiveRuns      []ActiveRunView
	WeekSessions    []DashboardSession
	WeekDayGroups   []DashboardDayGroup
	WeekLabel       string
	WeekPrevURL     string
	WeekNextURL     string
	CalendarCells   []CalendarCell
	CalendarMonth   string
	CalendarYear    string
	CalendarWeekday []string
	MonthPrevURL    string
	MonthNextURL    string
}

type RunParams struct {
	Base
	RunID             uint
	RunTemplateName   string
	RunTotalSteps     int
	RunCompleted      bool
	RunCurrentStepNum int
	RunSessionSeconds int
	RunIsTrial        bool
	RunTemplateID     uint
	RunIsOpen         bool
	RunCustomName     string
	RunLibraryExercises []db.LibraryExercise
	CurrentStep       RunStep
	RunSteps          []RunStep
	RunActivityGroups []RunActivityGroup
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
}

type ProfileParams struct {
	Base
	UserProfile      *db.User
	ProfileFormError string
}

type TemplateListParams struct {
	Base
	Templates []db.SessionTemplate
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
	Templates []db.SessionTemplate
}

// CycleExerciseOverrideView represents one exercise row in the cycle targets panel.
type CycleExerciseOverrideView struct {
	// identity
	LibraryExerciseID uint
	ExerciseName      string
	// planned defaults (from template)
	PlannedSets     int
	PlannedReps     int
	PlannedWeightKg float64
	PlannedRepSecs  int
	// current override values (0 = not overridden)
	OverrideSets     int
	OverrideReps     int
	OverrideWeightKg float64
	OverrideRepSecs  int
	HasOverride      bool
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
}

type ActivityTemplateListParams struct {
	Base
	ActivityTemplates []db.ActivityTemplate
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
	p.RenderFragment(w, "fragments/start-session-picker", params)
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

// ---------------------------------------------------------------------------
// Template FuncMap
// ---------------------------------------------------------------------------

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
			switch ex.Kind {
			case "session":
				if ex.SessionDurationSeconds > 0 {
					h := ex.SessionDurationSeconds / 3600
					m := (ex.SessionDurationSeconds % 3600) / 60
					s := ex.SessionDurationSeconds % 60
					if h > 0 && m == 0 && s == 0 {
						return fmt.Sprintf("%dh session", h)
					}
					if h > 0 {
						return fmt.Sprintf("%dh %dm session", h, m)
					}
					if m > 0 && s == 0 {
						return fmt.Sprintf("%dm session", m)
					}
					if m > 0 {
						return fmt.Sprintf("%dm %ds session", m, s)
					}
					return fmt.Sprintf("%ds session", s)
				}
				return "Session"
			case "exercise_catalog":
				return "Exercise catalog"
			default:
				parts := []string{}
				if ex.Sets > 0 && ex.Reps > 0 {
					parts = append(parts, fmt.Sprintf("%d×%d", ex.Sets, ex.Reps))
				} else if ex.Sets > 0 {
					parts = append(parts, fmt.Sprintf("%d sets", ex.Sets))
				} else if ex.Reps > 0 {
					parts = append(parts, fmt.Sprintf("%d reps", ex.Reps))
				}
				if ex.WeightKg > 0 {
					if ex.WeightKg == float64(int(ex.WeightKg)) {
						parts = append(parts, fmt.Sprintf("%.0fkg", ex.WeightKg))
					} else {
						parts = append(parts, fmt.Sprintf("%.1fkg", ex.WeightKg))
					}
				}
				if ex.RepSeconds > 0 {
					parts = append(parts, fmt.Sprintf("%ds/rep", ex.RepSeconds))
				}
				if len(parts) == 0 {
					return ""
				}
				return strings.Join(parts, " · ")
			}
		},
		"markdownHTML": func(s string) template.HTML {
			return markdownToHTML(s)
		},
		"youtubeEmbedURL": youtubeEmbedURL,
	}
}
