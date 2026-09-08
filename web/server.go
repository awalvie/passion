package web

import (
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"passion/config"
	"passion/store"
)

// Server holds everything a handler needs and nothing it should not have. There is no
// *gorm.DB here and no way to reach one: a handler that wanted to write its own query has
// nothing to write it with.
type Server struct {
	store  *store.Store
	render *Renderer
	cfg    config.App
	log    *slog.Logger
	static fs.FS
}

func New(cfg config.App, st *store.Store, templatesFS, staticFS fs.FS, log *slog.Logger) (*Server, error) {
	r, err := NewRenderer(templatesFS, staticFS, log, pageNames())
	if err != nil {
		return nil, err
	}
	return &Server{store: st, render: r, cfg: cfg, log: log, static: staticFS}, nil
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(s.requestID)
	r.Use(s.recoverPanics)
	r.Use(s.withActor)
	r.Use(s.logRequests)
	r.Use(s.checkCSRF)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(s.static))))
	r.Get("/healthz", s.handleHealthz)

	r.Get("/login", s.handleLoginForm)
	r.Post("/login", s.handleLogin)
	r.Get("/signup", s.handleSignupForm)
	r.Post("/signup", s.handleSignup)
	r.Post("/logout", s.handleLogout)

	r.Group(func(pr chi.Router) {
		pr.Use(s.requireActor)
		pr.Get("/", s.handleHome)
		pr.Post("/profile/password", s.handleChangePassword)
	})

	return r
}
