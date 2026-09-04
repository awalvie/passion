package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"passion/db"
)

// The catalog carries content licensed to one account, so an open signup page hands that
// content to anyone who finds it. These tests pin the gate shut.

func inviteTestServer(t *testing.T, name string) (*Server, *db.Store) {
	t.Helper()
	withRepoRoot(t)
	store, err := db.NewSqlite(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer(store, "test-secret-at-least-32-characters!!", time.Hour, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	return srv, store
}

func postSignup(t *testing.T, srv *Server, email, password, code string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{
		"email":            {email},
		"password":         {password},
		"password_confirm": {password},
	}
	if code != "" {
		form.Set("invite_code", code)
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	srv.handleSignup(rr, req)
	return rr
}

func userCount(t *testing.T, store *db.Store) int64 {
	t.Helper()
	var n int64
	if err := store.DB.Model(&db.User{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// mintCode creates a usable invite and returns the code as a person would type it.
func mintCode(t *testing.T, store *db.Store) string {
	t.Helper()
	row, err := db.CreateInviteCode(store.DB, nil, "test", nil)
	if err != nil {
		t.Fatal(err)
	}
	return row.Code
}

// seedFirstUser fills the bootstrap slot, so later signups need a code.
func seedFirstUser(t *testing.T, store *db.Store) {
	t.Helper()
	if err := store.DB.Create(&db.User{Email: "first@example.com", PasswordHash: "x"}).Error; err != nil {
		t.Fatal(err)
	}
}

// A fresh self-hosted install has to be able to create its first account.
func TestSignupOpenForTheVeryFirstAccount(t *testing.T) {
	srv, store := inviteTestServer(t, "invite-bootstrap.db")

	rr := postSignup(t, srv, "first@example.com", "password123", "")
	if got := rr.Header().Get("HX-Redirect"); got != "/dashboard" {
		t.Fatalf("first signup should succeed with no code, got status %d and HX-Redirect %q", rr.Code, got)
	}
	if n := userCount(t, store); n != 1 {
		t.Fatalf("want 1 user after bootstrap signup, got %d", n)
	}

	// The slot closes behind it.
	rr = postSignup(t, srv, "second@example.com", "password123", "")
	if rr.Header().Get("HX-Redirect") != "" {
		t.Fatal("the second signup must require an invite code")
	}
	if n := userCount(t, store); n != 1 {
		t.Fatalf("a rejected signup must not create a user, got %d", n)
	}
}

// A rejected signup must leave no account behind. Creating the user first and checking
// the code afterwards is the easy mistake, and it would let anyone in.
func TestSignupRejectsBadInviteCodesWithoutCreatingAUser(t *testing.T) {
	srv, store := inviteTestServer(t, "invite-bad.db")
	seedFirstUser(t, store)

	usedCode := mintCode(t, store)
	if err := db.RedeemInviteCode(store.DB, usedCode, 1, time.Now()); err != nil {
		t.Fatal(err)
	}

	past := time.Now().Add(-time.Hour)
	expired, err := db.CreateInviteCode(store.DB, nil, "expired", &past)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		code string
	}{
		{"no code at all", ""},
		{"unknown code", "ZZZZ-ZZZZ-ZZZZ"},
		{"already used", usedCode},
		{"expired", expired.Code},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := userCount(t, store)
			rr := postSignup(t, srv, "someone@example.com", "password123", tc.code)
			if rr.Header().Get("HX-Redirect") != "" {
				t.Fatalf("signup succeeded with %s", tc.name)
			}
			if rr.Code >= 500 {
				t.Fatalf("a bad code must not be a server error, got %d", rr.Code)
			}
			if after := userCount(t, store); after != before {
				t.Fatalf("a rejected signup created %d user(s)", after-before)
			}
		})
	}
}

// The code must be marked used, not merely checked. A handler that validates and forgets
// to claim would let one code create unlimited accounts.
func TestSignupWithValidCodeCreatesUserAndClaimsTheCode(t *testing.T) {
	srv, store := inviteTestServer(t, "invite-good.db")
	seedFirstUser(t, store)
	code := mintCode(t, store)

	rr := postSignup(t, srv, "invited@example.com", "password123", code)
	if got := rr.Header().Get("HX-Redirect"); got != "/dashboard" {
		t.Fatalf("valid code should sign up, got status %d and HX-Redirect %q", rr.Code, got)
	}

	var user db.User
	if err := store.DB.Where("email = ?", "invited@example.com").First(&user).Error; err != nil {
		t.Fatalf("account was not created: %v", err)
	}

	var row db.InviteCode
	if err := store.DB.Where("code = ?", db.NormaliseInviteCode(code)).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.UsedByID == nil || *row.UsedByID != user.ID {
		t.Fatalf("code should be claimed by user %d, got %v", user.ID, row.UsedByID)
	}
	if row.UsedAt == nil {
		t.Fatal("UsedAt should be set when a code is claimed")
	}
}

// The security property, tested through the handler rather than the model.
func TestSignupCodeCannotBeReusedBySomeoneElse(t *testing.T) {
	srv, store := inviteTestServer(t, "invite-reuse.db")
	seedFirstUser(t, store)
	code := mintCode(t, store)

	if rr := postSignup(t, srv, "one@example.com", "password123", code); rr.Header().Get("HX-Redirect") == "" {
		t.Fatal("the first signup should have succeeded")
	}
	before := userCount(t, store)

	if rr := postSignup(t, srv, "two@example.com", "password123", code); rr.Header().Get("HX-Redirect") != "" {
		t.Fatal("a used code let a second account through")
	}
	if after := userCount(t, store); after != before {
		t.Fatalf("a reused code created %d extra account(s)", after-before)
	}
}

// Two people pasting the same code at the same moment. "Read, then update" is not atomic,
// so the claim has to be a single conditional UPDATE that only one caller can win.
func TestSignupConcurrentRedemptionLetsExactlyOneThrough(t *testing.T) {
	srv, store := inviteTestServer(t, "invite-race.db")
	seedFirstUser(t, store)
	code := mintCode(t, store)

	const racers = 8
	var wg sync.WaitGroup
	results := make([]bool, racers)
	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			rr := postSignup(t, srv, "racer"+string(rune('a'+i))+"@example.com", "password123", code)
			results[i] = rr.Header().Get("HX-Redirect") == "/dashboard"
		}(i)
	}
	close(start)
	wg.Wait()

	won := 0
	for _, ok := range results {
		if ok {
			won++
		}
	}
	if won != 1 {
		t.Fatalf("exactly one racer should win, %d did", won)
	}
	// One seeded user plus exactly one signup.
	if n := userCount(t, store); n != 2 {
		t.Fatalf("want 2 users after the race, got %d", n)
	}
}

// A code is read off a screen and typed by hand, so case and the grouping dashes must not
// matter. This is a usability property, but a wrong normalisation is a lockout.
func TestSignupAcceptsACodeTypedLoosely(t *testing.T) {
	srv, store := inviteTestServer(t, "invite-loose.db")
	seedFirstUser(t, store)
	code := mintCode(t, store)

	loose := "  " + strings.ToLower(strings.ReplaceAll(code, "-", "")) + "  "
	if rr := postSignup(t, srv, "loose@example.com", "password123", loose); rr.Header().Get("HX-Redirect") == "" {
		t.Fatalf("a code typed as %q should be accepted", loose)
	}
}

// A browser will not store a Secure cookie over plain http, so running the app locally on
// http:// used to mean the login succeeded and the session vanished — you landed back on
// the login page with no explanation. Safari refuses it even on localhost.
func TestAuthCookieSecureFlagFollowsTheConfig(t *testing.T) {
	for _, tc := range []struct {
		name       string
		insecure   bool
		wantSecure bool
	}{
		{"default is secure", false, true},
		{"insecure cookies for local http", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withRepoRoot(t)
			store, err := db.NewSqlite(filepath.Join(t.TempDir(), "cookie.db"))
			if err != nil {
				t.Fatal(err)
			}
			srv, err := NewServer(store, "test-secret-at-least-32-characters!!", time.Hour, false, tc.insecure, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			if err := srv.setAuthCookie(rr, 1); err != nil {
				t.Fatal(err)
			}
			cookies := rr.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("want 1 cookie, got %d", len(cookies))
			}
			if cookies[0].Secure != tc.wantSecure {
				t.Errorf("Secure=%v, want %v", cookies[0].Secure, tc.wantSecure)
			}
			// Never negotiable, whatever the transport.
			if !cookies[0].HttpOnly {
				t.Error("the session cookie must always be HttpOnly")
			}
			if cookies[0].SameSite != http.SameSiteLaxMode {
				t.Error("the session cookie must always be SameSite=Lax")
			}
		})
	}
}
