package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// The spec is embedded, so a rename or a bad generate breaks the build rather
// than the page. This checks it arrives intact and describes every route.
func TestOpenAPIIsServed(t *testing.T) {
	rec := get(t, newTestServer(t), "/api/openapi.json", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type %q", ct)
	}

	var spec struct {
		Swagger string                     `json:"swagger"`
		Paths   map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("not json: %v", err)
	}
	if spec.Swagger == "" {
		t.Fatal("no swagger version")
	}

	for _, path := range []string{
		"/api/v1/accounts",
		"/api/v1/accounts/me",
		"/api/v1/tokens",
		"/api/v1/tokens/current",
		"/healthz",
	} {
		if _, ok := spec.Paths[path]; !ok {
			t.Errorf("%s is missing from the spec", path)
		}
	}
}
