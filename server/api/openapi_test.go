package api

import (
	"encoding/json"
	"net/http"
	"strings"
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

// The docs page is worth a test only because it is embedded: a renamed file or
// a changed route breaks it silently otherwise.
func TestDocsAreServed(t *testing.T) {
	h := newTestServer(t)

	page := get(t, h, "/api/docs/", "")
	if page.Code != http.StatusOK {
		t.Fatalf("status %d", page.Code)
	}
	for _, want := range []string{"scalar.js", "/api/openapi.json", "withDefaultFonts: false"} {
		if !strings.Contains(page.Body.String(), want) {
			t.Errorf("the page does not mention %q", want)
		}
	}

	if js := get(t, h, "/api/docs/scalar.js", ""); js.Code != http.StatusOK {
		t.Fatalf("the script is not served: %d", js.Code)
	}
}
