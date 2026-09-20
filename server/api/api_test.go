package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"passion/server/db/dbtest"
)

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestHealthzReportsALostDatabase(t *testing.T) {
	pool := dbtest.Pool(t)
	pool.Close()

	rec := httptest.NewRecorder()
	routes := New(pool, slog.New(slog.NewTextHandler(io.Discard, nil))).Routes(stubClient())
	routes.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", rec.Code)
	}
}

// Everything outside /api is the client's, because its routes exist only in
// the browser.
func TestUnknownPathReachesTheClient(t *testing.T) {
	h := newTestServer(t)

	for _, path := range []string{"/nope", "/login", "/sessions/42"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if !strings.Contains(rec.Body.String(), "the client") {
				t.Fatalf("the api answered: %s", rec.Body)
			}
		})
	}
}

// A mistyped endpoint must not come back as the app shell with a 200. The
// client would read HTML as JSON and report something unrelated.
func TestUnknownAPIPathIsJSON(t *testing.T) {
	h := newTestServer(t)

	for _, path := range []string{"/api/v1/typo", "/api/v1/accounts/nobody", "/api/"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status %d, want 404", rec.Code)
			}

			var got errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("not json: %v", err)
			}
			if got.Error.Code != CodeNotFound {
				t.Fatalf("code %q, want %q", got.Error.Code, CodeNotFound)
			}
		})
	}
}

// Serving the client at / means every path matches a pattern, so the mux never
// reaches its 405 case. A wrong method is a 404 in the API's shape, which is
// the part that matters: it is not the app shell.
func TestWrongMethodIsStillTheAPI(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}

	var got errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("not json: %v", err)
	}
	if got.Error.Code != CodeNotFound {
		t.Fatalf("code %q", got.Error.Code)
	}
}
