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

// guardCatalogWrite refuses a write that targets a shared catalog row, and says why.
// Returns false when it has already written a response.
//
// The distinction matters. A catalog row deserves "save your own copy to change this";
// a row belonging to someone else is simply not found, and saying anything else would
// confirm it exists. Before this, the write was scoped to owner_id and matched nothing:
// the user pressed Delete on a catalog row, got a success redirect, and found the row
// still there on reload. Silent, and identical in shape to the empty-children bug on the
// read side.
func (s *Server) guardCatalogWrite(w http.ResponseWriter, r *http.Request, model any, ownerID, id uint) bool {
	err := db.GuardWritable(s.store.DB, model, ownerID, id)
	switch {
	case err == nil:
		return true
	case errors.Is(err, db.ErrSharedReadOnly):
		s.badRequest(w, "That is part of the shared catalog. Save your own copy to change it.")
		return false
	case errors.Is(err, db.ErrNotFound):
		s.notFound(w)
		return false
	default:
		s.serverError(w, r, err)
		return false
	}
}
