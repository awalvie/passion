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

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	return New(dbtest.Pool(t), slog.New(slog.NewTextHandler(io.Discard, nil))).Routes()
}

func post(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return rec
}

// request sends a bodyless request, with a bearer header when one is given.
func request(t *testing.T, h http.Handler, method, path, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	if bearer != "" {
		r.Header.Set("Authorization", bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func get(t *testing.T, h http.Handler, path, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	return request(t, h, http.MethodGet, path, bearer)
}

func del(t *testing.T, h http.Handler, path, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	return request(t, h, http.MethodDelete, path, bearer)
}

const goodSignUp = `{"email":"ada@example.com","password":"correct horse battery",` +
	`"display_name":"Ada","timezone":"Europe/Oslo"}`

func TestSignUp(t *testing.T) {
	rec := post(t, newTestServer(t), "/api/v1/accounts", goodSignUp)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	var got signUpResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response: %v (%s)", err, rec.Body)
	}

	if got.Account.ID == "" {
		t.Fatal("no account id")
	}
	if got.Account.Email != "ada@example.com" {
		t.Fatalf("email %q", got.Account.Email)
	}
	if got.Token.Token == "" || got.Token.ExpiresAt.IsZero() {
		t.Fatal("no token was issued")
	}
}

// The password must never come back, in any form.
func TestSignUpReturnsNothingSecret(t *testing.T) {
	rec := post(t, newTestServer(t), "/api/v1/accounts", goodSignUp)

	body := rec.Body.String()
	for _, secret := range []string{"correct horse battery", "argon2id", "password_hash"} {
		if strings.Contains(body, secret) {
			t.Fatalf("the response leaked %q: %s", secret, body)
		}
	}
}

func TestSignUpValidation(t *testing.T) {
	cases := map[string]struct {
		body  string
		field string
	}{
		"no email":       {`{"email":"","password":"correct horse","display_name":"A","timezone":"UTC"}`, "email"},
		"not an email":   {`{"email":"nope","password":"correct horse","display_name":"A","timezone":"UTC"}`, "email"},
		"short password": {`{"email":"a@b.co","password":"short","display_name":"A","timezone":"UTC"}`, "password"},
		"long password": {`{"email":"a@b.co","password":"` + strings.Repeat("a", 129) +
			`","display_name":"A","timezone":"UTC"}`, "password"},
		"no name":      {`{"email":"a@b.co","password":"correct horse","display_name":"  ","timezone":"UTC"}`, "display_name"},
		"no timezone":  {`{"email":"a@b.co","password":"correct horse","display_name":"A","timezone":""}`, "timezone"},
		"bad timezone": {`{"email":"a@b.co","password":"correct horse","display_name":"A","timezone":"Middle/Earth"}`, "timezone"},
		"not json":     {`hunter2`, "body"},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			rec := post(t, newTestServer(t), "/api/v1/accounts", c.body)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want 422: %s", rec.Code, rec.Body)
			}

			var body errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("response: %v", err)
			}
			if _, ok := body.Error.Fields[c.field]; !ok {
				t.Fatalf("no error on %q, got %v", c.field, body.Error.Fields)
			}
		})
	}
}

// Signing up with a taken address must answer exactly as a wrong password
// does, or anyone can ask the API who has an account.
func TestSignUpWithATakenAddressSaysNothing(t *testing.T) {
	h := newTestServer(t)

	if rec := post(t, h, "/api/v1/accounts", goodSignUp); rec.Code != http.StatusCreated {
		t.Fatalf("first sign up: %d %s", rec.Code, rec.Body)
	}

	rec := post(t, h, "/api/v1/accounts", goodSignUp)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body)
	}

	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response: %v", err)
	}
	if body.Error.Code != CodeInvalidCredentials {
		t.Fatalf("code %q, want %q", body.Error.Code, CodeInvalidCredentials)
	}
	if strings.Contains(strings.ToLower(body.Error.Message), "already") {
		t.Fatalf("the message gives it away: %q", body.Error.Message)
	}
}
