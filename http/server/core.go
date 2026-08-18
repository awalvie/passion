package web

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"passion/db"
	"passion/pages"
)

type Server struct {
	pages         *pages.Pages
	store         *db.Store
	logger        *slog.Logger
	jwtSecret     string
	jwtTTL        time.Duration
	devAuthBypass bool
	yamlImport    *db.YAMLImportOptions // nil when YAML import is disabled
}

func NewServer(store *db.Store, jwtSecret string, jwtTTL time.Duration, devAuthBypass bool, yamlImport *db.YAMLImportOptions) (*Server, error) {
	p, err := pages.NewPages(slog.Default().With("component", "pages"))
	if err != nil {
		return nil, err
	}
	return &Server{
		pages:         p,
		store:         store,
		logger:        slog.Default().With("component", "http_server"),
		jwtSecret:     jwtSecret,
		jwtTTL:        jwtTTL,
		devAuthBypass: devAuthBypass,
		yamlImport:    yamlImport,
	}, nil
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(s.requestLogMiddleware)

	r.HandleFunc("/", s.handleIndex)
	r.HandleFunc("/login", s.handleLogin)
	r.HandleFunc("/signup", s.handleSignup)
	r.HandleFunc("/preview/markdown", s.handleMarkdownPreview)
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	r.Group(func(pr chi.Router) {
		pr.Use(s.csrfMiddleware)
		pr.Use(s.authMiddleware)
		pr.HandleFunc("/logout", s.handleLogout)
		pr.HandleFunc("/dashboard", s.handleDashboard)
		pr.HandleFunc("/dashboard/start-template", s.handleDashboardStartFromTemplate)
		pr.HandleFunc("/fragments/start-session-picker", s.handleStartSessionPicker)
		pr.HandleFunc("/profile", s.handleProfile)
		pr.HandleFunc("/history", s.handleHistory)

		pr.HandleFunc("/templates", s.handleTemplatesIndex)
		pr.HandleFunc("/templates/new", s.handleTemplatesNew)
		pr.HandleFunc("/templates/{templateID}/{action}", s.handleTemplatesByID)
		pr.HandleFunc("/templates/{templateID}/{action}/{subaction}", s.handleTemplatesByID)

		pr.HandleFunc("/activity-templates", s.handleActivityTemplatesIndex)
		pr.HandleFunc("/activity-templates/new", s.handleActivityTemplatesNew)
		pr.HandleFunc("/activity-templates/{activityTemplateID}/{action}", s.handleActivityTemplatesByID)
		pr.HandleFunc("/activity-templates/{activityTemplateID}/{action}/{subaction}", s.handleActivityTemplatesByID)

		pr.HandleFunc("/exercise-library", s.handleExerciseLibraryIndex)
		pr.HandleFunc("/exercise-library/new", s.handleExerciseLibraryNew)
		pr.HandleFunc("/exercise-library/export", s.handleExportLibraryExercisesBulk)
		pr.HandleFunc("/exercise-library/{libraryExerciseID}/history", s.handleExerciseLibraryHistory)
		pr.HandleFunc("/exercise-library/{libraryExerciseID}/{action}", s.handleExerciseLibraryByID)

		pr.HandleFunc("/activities/{activityID}/{action}", s.handleActivitiesByID)
		pr.HandleFunc("/activities/{activityID}/{action}/{subaction}", s.handleActivitiesByID)

		pr.HandleFunc("/calendar", s.handleCalendar)
		pr.HandleFunc("/calendar-events", s.handleCalendarEventCreate)
		pr.HandleFunc("/calendar-events/{eventID}/update", s.handleCalendarEventUpdate)
		pr.HandleFunc("/calendar-events/{eventID}/delete", s.handleCalendarEventDelete)

		pr.HandleFunc("/training-cycles", s.handleTrainingCycles)
		pr.HandleFunc("/training-cycles/new", s.handleTrainingCyclesNew)
		pr.HandleFunc("/training-cycles/new/guided", s.handleTrainingCyclesGuided)
		pr.HandleFunc("/training-cycles/{cycleID}", s.handleTrainingCyclesByID)
		pr.HandleFunc("/training-cycles/{cycleID}/{action}", s.handleTrainingCyclesByID)
		pr.HandleFunc("/training-cycles/{cycleID}/week-override-save", s.handleCycleWeekOverrideSave)
		pr.HandleFunc("/training-cycles/{cycleID}/week-override-toggle", s.handleCycleWeekOverrideToggle)

		pr.HandleFunc("/exercises/{exerciseID}/history-hint", s.handleExerciseHistoryHint)
		pr.HandleFunc("/exercises/{exerciseID}/history-popup", s.handleExerciseHistoryPopup)
		pr.HandleFunc("/exercises/{exerciseID}/divergence-hint", s.handleExerciseDivergenceHint)
		pr.HandleFunc("/exercises/{exerciseID}/{action}", s.handleExercisesByID)
		pr.HandleFunc("/exercises/{exerciseID}/planned-sets", s.handleExercisePlannedSets)
		pr.HandleFunc("/exercises/{exerciseID}/planned-sets/clear", s.handleExercisePlannedSetsClear)
		pr.HandleFunc("/exercises/{exerciseID}/planned-sets/{setIndex}/save", s.handleExercisePlannedSetSave)
		pr.HandleFunc("/exercises/{exerciseID}/planned-sets/{setIndex}/delete", s.handleExercisePlannedSetDelete)

		pr.HandleFunc("/scheduled-sessions/add", s.handleAddScheduledSession)
		pr.HandleFunc("/scheduled-sessions/{scheduledID}/{action}", s.handleScheduledSessionsByID)
		pr.HandleFunc("/runs/open", s.handleStartOpenSession)
		pr.HandleFunc("/runs/{runID}", s.handleRunsByID)
		pr.HandleFunc("/runs/{runID}/open/start", s.handleOpenStartSession)
		pr.HandleFunc("/runs/{runID}/open/add", s.handleOpenAddExercise)
		pr.HandleFunc("/runs/{runID}/open/add-template", s.handleOpenAddTemplate)
		pr.HandleFunc("/runs/{runID}/open/exercises/{exerciseID}/update", s.handleOpenUpdateExercise)
		pr.HandleFunc("/runs/{runID}/open/exercises/{exerciseID}/delete", s.handleOpenDeleteExercise)
		pr.HandleFunc("/runs/{runID}/open/exercises/{exerciseID}/planned-sets", s.handleOpenPlannedSets)
		pr.HandleFunc("/runs/{runID}/open/exercises/{exerciseID}/planned-sets/clear", s.handleOpenPlannedSetsClear)
		pr.HandleFunc("/runs/{runID}/open/exercises/{exerciseID}/planned-sets/{setIndex}/save", s.handleOpenPlannedSetSave)
		pr.HandleFunc("/runs/{runID}/open/exercises/{exerciseID}/planned-sets/{setIndex}/delete", s.handleOpenPlannedSetDelete)
		pr.HandleFunc("/runs/{runID}/stop", s.handleRunStop)
		pr.HandleFunc("/runs/{runID}/delete", s.handleRunDelete)
		pr.HandleFunc("/runs/{runID}/summary", s.handleRunSummary)
		pr.HandleFunc("/runs/{runID}/exercises/{exerciseID}/choose", s.handleRunExerciseChoose)
		pr.HandleFunc("/runs/{runID}/exercises/{exerciseID}/complete", s.handleRunsByID)
		pr.HandleFunc("/runs/{runID}/exercises/{exerciseID}/skip", s.handleRunsByID)
		pr.HandleFunc("/runs/{runID}/journal", s.handleRunJournal)
		pr.HandleFunc("/runs/{runID}/session-notes", s.handleRunSessionNotes)

		pr.HandleFunc("/training-log", s.handleTrainingLog)
		pr.HandleFunc("/training-log/new", s.handleTrainingLogNew)
		pr.HandleFunc("/training-log/quick", s.handleTrainingLogQuick)
		pr.HandleFunc("/training-log/for-run/{runID}", s.handleTrainingLogForRun)
		pr.HandleFunc("GET /training-log/{journalID}", s.handleTrainingLogView)
		pr.HandleFunc("/training-log/{journalID}/edit", s.handleTrainingLogEdit)
		pr.HandleFunc("/training-log/{journalID}/delete", s.handleTrainingLogDelete)
		pr.HandleFunc("/training-log/draft/{runID}/discard", s.handleTrainingLogDraftDiscard)
		pr.HandleFunc("/training-log/draft/{runID}/from-activity-template", s.handleTrainingLogAddFromTemplate)
		pr.HandleFunc("/training-log/draft/{runID}/exercises", s.handleTrainingLogAddExercise)
		pr.HandleFunc("POST /training-log/draft/{runID}/exercises/reorder", s.handleTrainingLogReorderExercises)
		pr.HandleFunc("/training-log/draft/{runID}/exercises/{exerciseID}/save", s.handleTrainingLogSaveExerciseCompletion)
		pr.HandleFunc("/training-log/draft/{runID}/exercises/{exerciseID}/delete", s.handleTrainingLogDeleteExercise)
		pr.HandleFunc("/training-log/draft/{runID}/exercises/{exerciseID}/climbing-meta", s.handleTrainingLogSaveClimbingMeta)
		pr.HandleFunc("/training-log/draft/{runID}/exercises/{exerciseID}/sets/mode", s.handleTrainingLogSetMode)
		pr.HandleFunc("/training-log/draft/{runID}/exercises/{exerciseID}/sets", s.handleTrainingLogAddSet)
		pr.HandleFunc("/training-log/draft/{runID}/exercises/{exerciseID}/sets/{setIndex}/save", s.handleTrainingLogSaveSet)
		pr.HandleFunc("/training-log/draft/{runID}/exercises/{exerciseID}/sets/{setIndex}/delete", s.handleTrainingLogDeleteSet)

		// Climbing ticks (scoped to a run+exercise)
		pr.HandleFunc("/runs/{runID}/exercises/{exerciseID}/burns", s.handleExerciseBurns)
		pr.HandleFunc("/runs/{runID}/exercises/{exerciseID}/burns/{burnID}/delete", s.handleExerciseBurnDelete)
		pr.HandleFunc("/runs/{runID}/exercises/{exerciseID}/ticks", s.handleExerciseTicks)
		pr.HandleFunc("/runs/{runID}/exercises/{exerciseID}/ticks/{tickID}/delete", s.handleExerciseTickDelete)
		pr.HandleFunc("/runs/{runID}/exercises/{exerciseID}/ticks/{tickID}/update", s.handleExerciseTickUpdate)
		pr.HandleFunc("/runs/{runID}/exercises/{exerciseID}/ticks/{tickID}/again", s.handleExerciseTickLogAgain)

		pr.HandleFunc("/profile/password", s.handleProfilePassword)

		// Venue and board management (profile sub-routes)
		pr.HandleFunc("/profile/venues", s.handleProfileVenues)
		pr.HandleFunc("/profile/venues/{venueID}/update", s.handleProfileVenueUpdate)
		pr.HandleFunc("/profile/venues/{venueID}/delete", s.handleProfileVenueDelete)
		pr.HandleFunc("/profile/boards", s.handleProfileBoards)
		pr.HandleFunc("/profile/boards/{boardID}/delete", s.handleProfileBoardDelete)
	})
	return r
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if s.devAuthBypass {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	if _, ok := s.currentUserID(r); ok {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (s *Server) requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			if rec := recover(); rec != nil {
				s.logger.Error("panic recovered", "method", r.Method, "path", r.URL.Path, "panic", rec)
				http.Error(sw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			durationMs := time.Since(start).Milliseconds()
			s.logger.Info(
				"http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration_ms", durationMs,
			)
		}()

		next.ServeHTTP(sw, r)
	})
}
