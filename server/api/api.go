package api

import (
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	pool *pgxpool.Pool
	log  *slog.Logger

	// tokenLife is how long a token lasts unused. It slides forward on use.
	tokenLife time.Duration

	// Each argon2id hash costs 64 MiB, so unbounded concurrency on the two
	// endpoints anyone can call is a way to exhaust the machine's memory.
	hashing chan struct{}
}

func New(pool *pgxpool.Pool, log *slog.Logger, tokenLife time.Duration) *Server {
	return &Server{
		pool:      pool,
		log:       log,
		tokenLife: tokenLife,
		hashing:   make(chan struct{}, runtime.GOMAXPROCS(0)),
	}
}

// Routes serves the API, and hands everything else to client. An unknown path
// under /api answers in the API's error shape rather than the app shell, so a
// mistyped endpoint cannot look like a working one.
func (s *Server) Routes(client http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", client)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeNotFound(w) })
	mux.HandleFunc("GET /api/openapi.json", s.openapi)
	mux.Handle("GET /api/docs/", docsHandler())
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("POST /api/v1/accounts", s.signUp)
	mux.HandleFunc("POST /api/v1/tokens", s.signIn)
	mux.HandleFunc("GET /api/v1/accounts/me", s.authenticated(s.me))
	mux.HandleFunc("PUT /api/v1/accounts/me/grades", s.authenticated(s.setGrades))
	mux.HandleFunc("DELETE /api/v1/tokens/current", s.authenticated(s.signOut))
	mux.HandleFunc("GET /api/v1/exercises", s.authenticated(s.listExercises))
	mux.HandleFunc("POST /api/v1/exercises", s.authenticated(s.createExercise))
	mux.HandleFunc("GET /api/v1/exercises/{id}", s.authenticated(s.readExercise))
	mux.HandleFunc("PUT /api/v1/exercises/{id}", s.authenticated(s.updateExercise))
	mux.HandleFunc("POST /api/v1/exercises/{id}/retire", s.authenticated(s.retireExercise))
	mux.HandleFunc("GET /api/v1/session-templates", s.authenticated(s.listSessionTemplates))
	mux.HandleFunc("POST /api/v1/session-templates", s.authenticated(s.createSessionTemplate))
	mux.HandleFunc("GET /api/v1/session-templates/{id}", s.authenticated(s.readSessionTemplate))
	mux.HandleFunc("PUT /api/v1/session-templates/{id}", s.authenticated(s.updateSessionTemplate))
	mux.HandleFunc("POST /api/v1/session-templates/{id}/retire", s.authenticated(s.retireSessionTemplate))
	mux.HandleFunc("GET /api/v1/runs", s.authenticated(s.listRuns))
	mux.HandleFunc("POST /api/v1/runs", s.authenticated(s.startRun))
	mux.HandleFunc("GET /api/v1/runs/{id}", s.authenticated(s.readRun))
	mux.HandleFunc("PUT /api/v1/runs/{id}", s.authenticated(s.updateRun))
	mux.HandleFunc("POST /api/v1/runs/{id}/finish", s.authenticated(s.finishRun))
	mux.HandleFunc("DELETE /api/v1/runs/{id}", s.authenticated(s.deleteRun))
	mux.HandleFunc("PUT /api/v1/runs/{id}/steps/{step}/sets", s.authenticated(s.replaceSets))
	mux.HandleFunc("PUT /api/v1/runs/{id}/climbs/{climb}", s.authenticated(s.putClimb))
	mux.HandleFunc("DELETE /api/v1/runs/{id}/climbs/{climb}", s.authenticated(s.deleteClimb))
	mux.HandleFunc("GET /api/v1/grades", s.authenticated(s.listGrades))
	return mux
}

// swagger:route GET /healthz health healthz
//
// # Liveness
//
// Answers 200 when the database answers, and nothing else.
//
//	Security: []
//	Responses:
//	  200: description: The database answered
//	  503: description: The database did not answer
func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// hash runs one password hash, waiting for a free slot first.
func (s *Server) hash(f func() (string, error)) (string, error) {
	s.hashing <- struct{}{}
	defer func() { <-s.hashing }()
	return f()
}
