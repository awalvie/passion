package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"passion/server/web"
)

func client() http.Handler {
	return web.HandlerFS(fstest.MapFS{
		"index.html":               {Data: []byte("<!doctype html>the app shell")},
		"robots.txt":               {Data: []byte("User-agent: *")},
		"_app/immutable/app.js":    {Data: []byte("console.log('app')")},
		"_app/immutable/style.css": {Data: []byte("body{}")},
	})
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestServesRealFiles(t *testing.T) {
	h := client()

	cases := map[string]string{
		"/":                         "<!doctype html>the app shell",
		"/robots.txt":               "User-agent: *",
		"/_app/immutable/app.js":    "console.log('app')",
		"/_app/immutable/style.css": "body{}",
	}

	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			rec := get(t, h, path)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d", rec.Code)
			}
			if rec.Body.String() != want {
				t.Fatalf("body %q, want %q", rec.Body.String(), want)
			}
		})
	}
}

// http.FileServer sends /index.html to / so one page has one address.
func TestIndexHTMLRedirects(t *testing.T) {
	rec := get(t, client(), "/index.html")
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status %d, want 301", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "./" {
		t.Fatalf("location %q", got)
	}
}

// A client route exists only in the browser, so the server answers the shell
// and lets the client's router read the URL.
func TestUnknownPathServesTheShell(t *testing.T) {
	h := client()

	for _, path := range []string{"/login", "/sessions/42", "/a/b/c", "/nope.html"} {
		t.Run(path, func(t *testing.T) {
			rec := get(t, h, path)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d", rec.Code)
			}
			if rec.Body.String() != "<!doctype html>the app shell" {
				t.Fatalf("body %q", rec.Body.String())
			}
		})
	}
}

// http.FileServer lists a directory that has no index.html. That would publish
// the shape of the build.
func TestDirectoryIsNotListed(t *testing.T) {
	rec := get(t, client(), "/_app/immutable/")
	if rec.Body.String() != "<!doctype html>the app shell" {
		t.Fatalf("a directory was listed: %q", rec.Body.String())
	}
}

// Serving the shell must not let a request walk out of the build directory.
func TestDoesNotEscapeTheRoot(t *testing.T) {
	rec := get(t, client(), "/../../etc/passwd")
	if rec.Code != http.StatusOK || rec.Body.String() != "<!doctype html>the app shell" {
		t.Fatalf("status %d, body %q", rec.Code, rec.Body.String())
	}
}
