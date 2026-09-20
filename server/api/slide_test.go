package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/db/dbtest"
	"passion/server/token"
)

// serverOn builds an API over a pool the test can also read directly, which is
// how the expiry is checked without an endpoint that reports it.
func serverOn(t *testing.T) (http.Handler, *pgxpool.Pool) {
	t.Helper()
	pool := dbtest.Pool(t)
	return New(pool, slog.New(slog.NewTextHandler(io.Discard, nil))).Routes(stubClient()), pool
}

func expiryOf(t *testing.T, pool *pgxpool.Pool, bearer string) time.Time {
	t.Helper()
	var expires time.Time
	err := pool.QueryRow(context.Background(),
		`SELECT expires_at FROM auth_token WHERE token_hash = $1`, token.Hash(bearer)).Scan(&expires)
	if err != nil {
		t.Fatalf("read the expiry: %v", err)
	}
	return expires
}

// A phone opened every few weeks must never be signed out, and sliding its
// token must not touch the laptop's.
func TestSlideExtendsTheTokenInUse(t *testing.T) {
	h, pool := serverOn(t)
	phone := signedIn(t, h, "ada@example.com")

	rec := post(t, h, "/api/v1/tokens",
		`{"email":"ada@example.com","password":"correct horse battery"}`)
	var laptop tokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &laptop); err != nil {
		t.Fatalf("response: %v", err)
	}
	laptopBefore := expiryOf(t, pool, laptop.Token)

	// Age the phone's token so under a day of its life is left.
	if _, err := pool.Exec(context.Background(),
		`UPDATE auth_token SET expires_at = now() + interval '2 hours' WHERE token_hash = $1`,
		token.Hash(phone)); err != nil {
		t.Fatalf("age the token: %v", err)
	}

	if rec := get(t, h, "/api/v1/accounts/me", "Bearer "+phone); rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}

	if left := time.Until(expiryOf(t, pool, phone)); left < 29*24*time.Hour {
		t.Fatalf("the token was not extended: %v left", left)
	}
	if after := expiryOf(t, pool, laptop.Token); !after.Equal(laptopBefore) {
		t.Fatalf("the other device's token moved: %v to %v", laptopBefore, after)
	}
}

// Reading must not become writing on every request.
func TestSlideLeavesAFreshTokenAlone(t *testing.T) {
	h, pool := serverOn(t)
	bearer := signedIn(t, h, "ada@example.com")
	before := expiryOf(t, pool, bearer)

	for range 3 {
		if rec := get(t, h, "/api/v1/accounts/me", "Bearer "+bearer); rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
	}

	if after := expiryOf(t, pool, bearer); !after.Equal(before) {
		t.Fatalf("a fresh token was written: %v to %v", before, after)
	}
}
