package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestMe(t *testing.T) {
	h := newTestServer(t)
	bearer := signedIn(t, h, "ada@example.com")

	rec := get(t, h, "/api/v1/accounts/me", "Bearer "+bearer)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}

	var got accountResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response: %v", err)
	}
	if got.Email != "ada@example.com" || got.DisplayName != "Ada" {
		t.Fatalf("wrong account: %+v", got)
	}
}

// Token-to-account mix-up is the classic auth bug.
func TestMeReturnsTheRightAccount(t *testing.T) {
	h := newTestServer(t)
	ada := signedIn(t, h, "ada@example.com")
	bob := signedIn(t, h, "bob@example.com")

	for bearer, want := range map[string]string{ada: "ada@example.com", bob: "bob@example.com"} {
		rec := get(t, h, "/api/v1/accounts/me", "Bearer "+bearer)
		var got accountResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("response: %v", err)
		}
		if got.Email != want {
			t.Fatalf("token resolved to %s, want %s", got.Email, want)
		}
	}
}

// Two devices on one account. Neither token may shut the other out.
func TestMeAcceptsEitherDeviceToken(t *testing.T) {
	h := newTestServer(t)
	phone := signedIn(t, h, "ada@example.com")

	rec := post(t, h, "/api/v1/tokens",
		`{"email":"ada@example.com","password":"correct horse battery"}`)
	var laptop tokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &laptop); err != nil {
		t.Fatalf("response: %v", err)
	}

	for _, bearer := range []string{phone, laptop.Token} {
		if rec := get(t, h, "/api/v1/accounts/me", "Bearer "+bearer); rec.Code != http.StatusOK {
			t.Fatalf("a token does not work: %d", rec.Code)
		}
	}
}

// Every one of these must answer identically.
func TestMeRefusesEveryBadToken(t *testing.T) {
	h := newTestServer(t)
	good := signedIn(t, h, "ada@example.com")

	cases := map[string]string{
		"no header":      "",
		"no scheme":      good,
		"wrong scheme":   "Basic " + good,
		"empty token":    "Bearer ",
		"unknown token":  "Bearer not-a-real-token",
		"lowercase word": "bearer " + good,
	}

	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			rec := get(t, h, "/api/v1/accounts/me", header)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body)
			}

			var got errorBody
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("response: %v", err)
			}
			if got.Error.Code != CodeUnauthenticated {
				t.Fatalf("code %q, want %q", got.Error.Code, CodeUnauthenticated)
			}
		})
	}
}
