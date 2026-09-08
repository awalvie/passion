package web

import (
	"encoding/json"
	"html"
	"net/http"
)

// handleHome is a placeholder for the dashboard, which phase 7 builds. It exists so phase
// 1 has somewhere to land after a login and the auth flow can be exercised end to end.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	actor, _ := actorFrom(r.Context())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8">` +
		`<title>Passion v2</title>` +
		`<p>Signed in as ` + html.EscapeString(actor.Email) + `.</p>` +
		`<form method="post" action="/logout"><button>Log out</button></form>`))
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
