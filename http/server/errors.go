package web

import (
	"log/slog"
	"net/http"
)

// serverError logs the error with request context and writes a generic 500 response.
// Use this instead of http.Error(w, err.Error(), 500) to avoid leaking internal details.
func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("internal server error",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("error", err.Error()),
	)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}
