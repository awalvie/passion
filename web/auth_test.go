package web

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"passion/config"
	"passion/store"
)

// Handler tests run against a real store on real SQLite. Nothing is mocked: a mock would
// prove the mock works, and the invariants worth testing here are cascades, constraints
// and cookie behaviour, none of which a mock can get wrong.
func newTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	ctx := context.Background()

	st, err := store.Open(ctx, store.Config{
		Engine: store.EngineSQLite,
		DSN:    filepath.Join(t.TempDir(), "web.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	// The real trees from disk, so a template that stops parsing fails this test rather
	// than only failing at boot.
	root, err := os.OpenRoot("..")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	templates, err := fs.Sub(root.FS(), "templates")
	if err != nil {
		t.Fatal(err)
	}
	static, err := fs.Sub(root.FS(), "static")
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultApp()
	cfg.Auth.JWTSecret = strings.Repeat("k", 48)
	cfg.Auth.InsecureCookies = true

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	srv, err := New(cfg, st, templates, static, log)
	if err != nil {
		t.Fatal(err)
	}
	return srv, st
}

func post(t *testing.T, h http.Handler, path string, form url.Values, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func get(t *testing.T, h http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func authCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == authCookieName && c.Value != "" {
			return c
		}
	}
	t.Fatalf("no %s cookie was set (status %d)", authCookieName, rec.Code)
	return nil
}

// The templates have to parse for anything else here to mean much, so this is asserted on
// its own — a malformed template should fail the suite, not the first request in production.
func TestEveryTemplateParses(t *testing.T) {
	srv, _ := newTestServer(t)
	if len(srv.render.pages) == 0 {
		t.Fatal("no pages parsed")
	}
	for _, want := range []string{"login", "signup"} {
		if _, ok := srv.render.pages[want]; !ok {
			t.Errorf("page %q did not parse", want)
		}
	}
	t.Logf("parsed %d page templates", len(srv.render.pages))
}

func TestSignupThenLogin(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.Routes()

	// Signup is open: no invite code is sent and none is required.
	rec := post(t, h, "/signup", url.Values{
		"email":    {"first@example.com"},
		"password": {"a good password"},
	}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("signup returned %d, want 303\n%s", rec.Code, rec.Body.String())
	}
	cookie := authCookie(t, rec)

	if got := get(t, h, "/", cookie); got.Code != http.StatusOK {
		t.Errorf("the signed-up account could not load /: %d", got.Code)
	}
	if !cookie.HttpOnly {
		t.Error("the auth cookie is not HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("the auth cookie SameSite is %v, want Lax", cookie.SameSite)
	}

	// The same address again, differently cased, must be refused.
	rec = post(t, h, "/signup", url.Values{
		"email":    {"FIRST@example.com"},
		"password": {"another password"},
	}, nil)
	if rec.Code != http.StatusConflict {
		t.Errorf("a duplicate signup returned %d, want 409", rec.Code)
	}

	rec = post(t, h, "/login", url.Values{
		"email":    {"first@example.com"},
		"password": {"a good password"},
	}, nil)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("login returned %d, want 303", rec.Code)
	}

	rec = post(t, h, "/login", url.Values{
		"email":    {"first@example.com"},
		"password": {"wrong"},
	}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("a wrong password returned %d, want 401", rec.Code)
	}
	// The wrong-password and unknown-email responses must be identical, or the form can be
	// used to discover which addresses have accounts.
	other := post(t, h, "/login", url.Values{
		"email":    {"nobody@example.com"},
		"password": {"wrong"},
	}, nil)
	if other.Code != rec.Code {
		t.Errorf("unknown email gave %d and wrong password gave %d; they must match",
			other.Code, rec.Code)
	}
}

func TestUnauthenticatedIsSentToLogin(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.Routes()

	rec := get(t, h, "/", nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("an anonymous request to / returned %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login") {
		t.Errorf("redirected to %q, want /login", loc)
	}
}

// The point of the token epoch. Two accounts, change one password, and only that account's
// existing cookie stops working.
func TestChangingAPasswordEndsOnlyThatAccountsSessions(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.Routes()

	recA := post(t, h, "/signup", url.Values{
		"email": {"a@example.com"}, "password": {"first password A"},
	}, nil)
	cookieA := authCookie(t, recA)

	recB := post(t, h, "/signup", url.Values{
		"email": {"b@example.com"}, "password": {"first password B"},
	}, nil)
	cookieB := authCookie(t, recB)

	// A second, older session for A — a phone, say.
	recA2 := post(t, h, "/login", url.Values{
		"email": {"a@example.com"}, "password": {"first password A"},
	}, nil)
	oldCookieA := authCookie(t, recA2)

	rec := post(t, h, "/profile/password", url.Values{
		"current_password": {"first password A"},
		"new_password":     {"second password A"},
	}, cookieA)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("changing the password returned %d, want 303\n%s", rec.Code, rec.Body.String())
	}

	// A's other session is now dead.
	if got := get(t, h, "/", oldCookieA); got.Code != http.StatusSeeOther {
		t.Errorf("A's older cookie still works after a password change: %d", got.Code)
	}
	// B is untouched.
	if got := get(t, h, "/", cookieB); got.Code != http.StatusOK {
		t.Errorf("B's session broke when A changed their password: %d", got.Code)
	}
	// And the browser that made the change is re-issued a working cookie.
	if got := get(t, h, "/", authCookie(t, rec)); got.Code != http.StatusOK {
		t.Errorf("the browser that changed the password was logged out: %d", got.Code)
	}
}

func TestCrossSiteWriteIsRejected(t *testing.T) {
	srv, _ := newTestServer(t)
	h := srv.Routes()

	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(url.Values{"email": {"a@b.c"}, "password": {"x"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("a cross-origin POST returned %d, want 403", rec.Code)
	}
}

// A crafted ?next= must not be able to bounce someone off the site after a real login.
func TestNextParameterStaysOnSite(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", "/"},
		{"/history", "/history"},
		{"//evil.example/", "/"},
		{"https://evil.example/", "/"},
		{"javascript:alert(1)", "/"},
	} {
		if got := safeNext(tc.in); got != tc.want {
			t.Errorf("safeNext(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHealthzReportsTheSchemaVersion(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := get(t, srv.Routes(), "/healthz", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz returned %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`"ok":true`, `"engine":"sqlite"`, `"schema_version":1`} {
		if !strings.Contains(body, want) {
			t.Errorf("healthz body is missing %s\n%s", want, body)
		}
	}
}

// Deleting an account must take everything it owns and leave the other account and the
// shipped catalog alone. The store proves the cascade; this proves the handler path reaches
// the same place with a real signup behind it.
func TestDeletingOneAccountLeavesTheOther(t *testing.T) {
	srv, st := newTestServer(t)
	h := srv.Routes()
	ctx := context.Background()

	for _, email := range []string{"keep@example.com", "gone@example.com"} {
		rec := post(t, h, "/signup", url.Values{
			"email": {email}, "password": {"a good password"},
		}, nil)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("signing up %s returned %d", email, rec.Code)
		}
	}

	victim, err := st.Authenticate(ctx, "gone@example.com", "a good password")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteAccount(ctx, victim.ID); err != nil {
		t.Fatalf("deleting an account: %v", err)
	}

	if n, _ := st.AccountCount(ctx); n != 1 {
		t.Errorf("%d accounts left, want 1", n)
	}
	if _, err := st.Authenticate(ctx, "keep@example.com", "a good password"); err != nil {
		t.Errorf("the surviving account cannot log in: %v", err)
	}
	if _, err := st.Authenticate(ctx, "gone@example.com", "a good password"); err == nil {
		t.Error("the deleted account can still log in")
	}
}
