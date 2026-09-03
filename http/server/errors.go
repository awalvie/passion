package web

import (
	"errors"
	"log/slog"
	"net/http"

	"passion/db"
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

// dbError writes a 404 for db.ErrNotFound and a 500 for any other error.
func (s *Server) dbError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, db.ErrNotFound) {
		s.notFound(w)
		return
	}
	s.serverError(w, r, err)
}

// catalogResetError writes a 400 when there is no catalog to reset to and a 500 otherwise.
func (s *Server) catalogResetError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errCatalogImportDisabled) {
		s.badRequest(w, "catalog import is disabled")
		return
	}
	s.serverError(w, r, err)
}
