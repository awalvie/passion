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

		pr.HandleFunc("/training-cycles", s.handleTrainingCycles)
		pr.HandleFunc("/training-cycles/new", s.handleTrainingCyclesNew)
		pr.HandleFunc("/training-cycles/{cycleID}", s.handleTrainingCyclesByID)
		pr.HandleFunc("/training-cycles/{cycleID}/{action}", s.handleTrainingCyclesByID)

		pr.HandleFunc("/exercises/{exerciseID}/history-hint", s.handleExerciseHistoryHint)
		pr.HandleFunc("/exercises/{exerciseID}/history-popup", s.handleExerciseHistoryPopup)
		pr.HandleFunc("/exercises/{exerciseID}/divergence-hint", s.handleExerciseDivergenceHint)
		pr.HandleFunc("/exercises/{exerciseID}/{action}", s.handleExercisesByID)

		pr.HandleFunc("/scheduled-sessions/add", s.handleAddScheduledSession)
		pr.HandleFunc("/scheduled-sessions/{scheduledID}/{action}", s.handleScheduledSessionsByID)
		pr.HandleFunc("/runs/open", s.handleStartOpenSession)
		pr.HandleFunc("/runs/{runID}", s.handleRunsByID)
		pr.HandleFunc("/runs/{runID}/open/add", s.handleOpenAddExercise)
		pr.HandleFunc("/runs/{runID}/stop", s.handleRunStop)
		pr.HandleFunc("/runs/{runID}/delete", s.handleRunDelete)
		pr.HandleFunc("/runs/{runID}/summary", s.handleRunSummary)
		pr.HandleFunc("/runs/{runID}/exercises/{exerciseID}/choose", s.handleRunExerciseChoose)
		pr.HandleFunc("/runs/{runID}/exercises/{exerciseID}/complete", s.handleRunsByID)
		pr.HandleFunc("/runs/{runID}/exercises/{exerciseID}/skip", s.handleRunsByID)
		pr.HandleFunc("/runs/{runID}/journal", s.handleRunJournal)

		pr.HandleFunc("/training-log", s.handleTrainingLog)
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
