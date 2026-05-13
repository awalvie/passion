package web

import (
	"log/slog"
	"net/http"
)

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("internal server error",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("error", err.Error()),
	)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func (s *Server) badRequest(w http.ResponseWriter, msg string) {
	http.Error(w, msg, http.StatusBadRequest)
}

func (s *Server) methodNotAllowed(w http.ResponseWriter) {
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) notFound(w http.ResponseWriter) {
	http.Error(w, "not found", http.StatusNotFound)
}
