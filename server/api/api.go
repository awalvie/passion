package api

import (
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// tokenLife is thirty days, which is the longest a reauthentication timeout
// should be at this assurance level. It slides forward on use.
const tokenLife = 30 * 24 * time.Hour

type Server struct {
	pool *pgxpool.Pool
	log  *slog.Logger

	// Each argon2id hash costs 64 MiB, so unbounded concurrency on the two
	// endpoints anyone can call is a way to exhaust the machine's memory.
	hashing chan struct{}
}

func New(pool *pgxpool.Pool, log *slog.Logger) *Server {
	return &Server{
		pool:    pool,
		log:     log,
		hashing: make(chan struct{}, runtime.GOMAXPROCS(0)),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("POST /api/v1/accounts", s.signUp)
	mux.HandleFunc("POST /api/v1/tokens", s.signIn)
	mux.HandleFunc("GET /api/v1/accounts/me", s.authenticated(s.me))
	mux.HandleFunc("DELETE /api/v1/tokens/current", s.authenticated(s.signOut))
	return mux
}

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
