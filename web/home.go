package web

import (
	"encoding/json"
	"net/http"
)

type homeParams struct {
	BaseParams
	Email string
}

// handleHome is a placeholder for the dashboard, which phase 7 builds.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	actor, _ := actorFrom(r.Context())
	s.render.page(w, http.StatusOK, "home", homeParams{
		BaseParams: BaseParams{Title: "Passion", Authenticated: true},
		Email:      actor.Email,
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	version, err := s.store.SchemaVersion(r.Context())
	if err != nil {
		s.log.Error("healthz", "error", err)
		http.Error(w, "unhealthy", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":             true,
		"engine":         s.store.Engine(),
		"schema_version": version,
	})
}
