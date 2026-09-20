package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/db"
	"passion/server/db/dbtest"
	"passion/server/token"
)

func signIn(t *testing.T, pool *pgxpool.Pool, email string) (accountID, raw string) {
	t.Helper()
	ctx := context.Background()

	account, err := db.CreateAccount(ctx, pool, email, "hash", "Ada", "Europe/Oslo")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	raw, hash, err := token.New()
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	if _, err := db.CreateAuthToken(ctx, pool, account.ID, hash, time.Now().Add(30*24*time.Hour)); err != nil {
		t.Fatalf("create auth token: %v", err)
	}
	return account.ID, raw
}

func TestAuthenticateByToken(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	accountID, raw := signIn(t, pool, "ada@example.com")

	who, err := db.AuthenticateByToken(ctx, pool, token.Hash(raw))
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if who.AccountID != accountID {
		t.Fatalf("got account %s, want %s", who.AccountID, accountID)
	}
	if who.Email != "ada@example.com" {
		t.Fatalf("got email %s", who.Email)
	}
}

// The classic auth bug, and the cheapest catastrophic one.
func TestEachTokenFindsItsOwnAccount(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)

	adaID, adaToken := signIn(t, pool, "ada@example.com")
	bobID, bobToken := signIn(t, pool, "bob@example.com")

	ada, err := db.AuthenticateByToken(ctx, pool, token.Hash(adaToken))
	if err != nil {
		t.Fatalf("ada: %v", err)
	}
	bob, err := db.AuthenticateByToken(ctx, pool, token.Hash(bobToken))
	if err != nil {
		t.Fatalf("bob: %v", err)
	}

	if ada.AccountID != adaID || bob.AccountID != bobID {
		t.Fatal("a token resolved to the wrong account")
	}
}

func TestAuthenticateRefusesAnUnknownToken(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	signIn(t, pool, "ada@example.com")

	_, err := db.AuthenticateByToken(ctx, pool, token.Hash("not a real token"))
	if !errors.Is(err, db.ErrNoAuthToken) {
		t.Fatalf("got %v, want ErrNoAuthToken", err)
	}
}

func TestAuthenticateRefusesAnExpiredToken(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)

	account, err := db.CreateAccount(ctx, pool, "ada@example.com", "hash", "Ada", "UTC")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	raw, hash, err := token.New()
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	// The CHECK forbids an expiry before creation, so this is inserted live
	// and then moved into the past.
	id, err := db.CreateAuthToken(ctx, pool, account.ID, hash, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("create auth token: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE auth_token SET created_at = now() - interval '2 days', expires_at = now() - interval '1 day' WHERE id = $1`, id); err != nil {
		t.Fatalf("expire it: %v", err)
	}

	if _, err := db.AuthenticateByToken(ctx, pool, token.Hash(raw)); !errors.Is(err, db.ErrNoAuthToken) {
		t.Fatalf("got %v, want ErrNoAuthToken", err)
	}
}

func TestSlideAuthToken(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	_, raw := signIn(t, pool, "ada@example.com")

	before, err := db.AuthenticateByToken(ctx, pool, token.Hash(raw))
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	want := before.TokenExpiresAt.Add(24 * time.Hour)
	if err := db.SlideAuthToken(ctx, pool, before.TokenID, want); err != nil {
		t.Fatalf("slide: %v", err)
	}

	after, err := db.AuthenticateByToken(ctx, pool, token.Hash(raw))
	if err != nil {
		t.Fatalf("authenticate again: %v", err)
	}
	if !after.TokenExpiresAt.After(before.TokenExpiresAt) {
		t.Fatal("the expiry did not move")
	}
}

// Signing out of one device must leave the other signed in.
func TestDeleteAuthTokenLeavesTheOtherDevice(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)

	account, err := db.CreateAccount(ctx, pool, "ada@example.com", "hash", "Ada", "UTC")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	mint := func() (string, string) {
		raw, hash, err := token.New()
		if err != nil {
			t.Fatalf("mint: %v", err)
		}
		id, err := db.CreateAuthToken(ctx, pool, account.ID, hash, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatalf("create auth token: %v", err)
		}
		return id, raw
	}

	phoneID, phone := mint()
	_, laptop := mint()

	if err := db.DeleteAuthToken(ctx, pool, phoneID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := db.AuthenticateByToken(ctx, pool, token.Hash(phone)); !errors.Is(err, db.ErrNoAuthToken) {
		t.Fatalf("the phone is still signed in: %v", err)
	}
	if _, err := db.AuthenticateByToken(ctx, pool, token.Hash(laptop)); err != nil {
		t.Fatalf("the laptop was signed out too: %v", err)
	}

	// Signing out twice is harmless.
	if err := db.DeleteAuthToken(ctx, pool, phoneID); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}

func TestDeletingAnAccountTakesItsTokens(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)

	adaID, adaToken := signIn(t, pool, "ada@example.com")
	_, bobToken := signIn(t, pool, "bob@example.com")

	if _, err := pool.Exec(ctx, `DELETE FROM account WHERE id = $1`, adaID); err != nil {
		t.Fatalf("delete account: %v", err)
	}

	if _, err := db.AuthenticateByToken(ctx, pool, token.Hash(adaToken)); !errors.Is(err, db.ErrNoAuthToken) {
		t.Fatalf("ada's token survived: %v", err)
	}
	if _, err := db.AuthenticateByToken(ctx, pool, token.Hash(bobToken)); err != nil {
		t.Fatalf("bob's token was taken too: %v", err)
	}
}
