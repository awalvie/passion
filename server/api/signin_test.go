package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// signedIn signs an account up and returns its bearer token.
func signedIn(t *testing.T, h http.Handler, email string) string {
	t.Helper()

	body := `{"email":"` + email + `","password":"correct horse battery",` +
		`"display_name":"Ada","timezone":"Europe/Oslo"}`
	rec := post(t, h, "/api/v1/accounts", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("sign up: %d %s", rec.Code, rec.Body)
	}

	var got signUpResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("sign up response: %v", err)
	}
	return got.Token.Token
}

func TestSignIn(t *testing.T) {
	h := newTestServer(t)
	signedIn(t, h, "ada@example.com")

	rec := post(t, h, "/api/v1/tokens",
		`{"email":"ADA@Example.com","password":"correct horse battery"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	var issued tokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &issued); err != nil {
		t.Fatalf("response: %v", err)
	}
	if issued.Token == "" || issued.ExpiresAt.IsZero() {
		t.Fatal("no token was issued")
	}
}

func TestSignInRefusesBadCredentials(t *testing.T) {
	h := newTestServer(t)
	signedIn(t, h, "ada@example.com")

	cases := map[string]string{
		"wrong password": `{"email":"ada@example.com","password":"correct horse batterz"}`,
		"unknown email":  `{"email":"nobody@example.com","password":"correct horse battery"}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := post(t, h, "/api/v1/tokens", body)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body)
			}

			var got errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("response: %v", err)
			}
			if got.Error.Code != CodeInvalidCredentials {
				t.Fatalf("code %q", got.Error.Code)
			}
		})
	}
}

// Two sign-ins are two devices. Signing in on one must not reissue the other.
func TestSignInTwiceGivesTwoTokens(t *testing.T) {
	h := newTestServer(t)
	phone := signedIn(t, h, "ada@example.com")

	rec := post(t, h, "/api/v1/tokens",
		`{"email":"ada@example.com","password":"correct horse battery"}`)
	var laptop tokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &laptop); err != nil {
		t.Fatalf("response: %v", err)
	}

	if phone == laptop.Token {
		t.Fatal("both sign-ins got the same token")
	}
}
