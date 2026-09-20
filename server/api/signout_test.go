package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// The point of a token per device: signing out the phone must not sign out the
// laptop.
func TestSignOutLeavesTheOtherDevice(t *testing.T) {
	h := newTestServer(t)
	phone := signedIn(t, h, "ada@example.com")

	rec := post(t, h, "/api/v1/tokens",
		`{"email":"ada@example.com","password":"correct horse battery"}`)
	var laptop tokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &laptop); err != nil {
		t.Fatalf("response: %v", err)
	}

	if rec := del(t, h, "/api/v1/tokens/current", "Bearer "+phone); rec.Code != http.StatusNoContent {
		t.Fatalf("sign out: %d %s", rec.Code, rec.Body)
	}

	if rec := get(t, h, "/api/v1/accounts/me", "Bearer "+phone); rec.Code != http.StatusUnauthorized {
		t.Fatalf("the signed-out token still works: %d", rec.Code)
	}
	if rec := get(t, h, "/api/v1/accounts/me", "Bearer "+laptop.Token); rec.Code != http.StatusOK {
		t.Fatalf("the other device was signed out too: %d", rec.Code)
	}
}

// The route authenticates before it deletes, so a token cannot be used twice.
func TestSignOutTwiceIsUnauthenticated(t *testing.T) {
	h := newTestServer(t)
	bearer := signedIn(t, h, "ada@example.com")

	if rec := del(t, h, "/api/v1/tokens/current", "Bearer "+bearer); rec.Code != http.StatusNoContent {
		t.Fatalf("sign out: %d %s", rec.Code, rec.Body)
	}

	rec := del(t, h, "/api/v1/tokens/current", "Bearer "+bearer)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body)
	}
}

func TestSignOutNeedsAToken(t *testing.T) {
	h := newTestServer(t)

	rec := del(t, h, "/api/v1/tokens/current", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401: %s", rec.Code, rec.Body)
	}
}
