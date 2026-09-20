package db_test

import (
	"context"
	"errors"
	"testing"

	"passion/server/db"
	"passion/server/db/dbtest"
)

func TestCreateAccount(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)

	account, err := db.CreateAccount(ctx, pool, "ada@example.com", "hash", "Ada", "Europe/Oslo")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if account.ID == "" {
		t.Fatal("no id was generated")
	}
	if account.Email != "ada@example.com" {
		t.Fatalf("email is %q", account.Email)
	}
	if account.CreatedAt.IsZero() || account.UpdatedAt.IsZero() {
		t.Fatal("timestamps were not set")
	}
}

func TestCreateAccountNormalisesTheEmail(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)

	account, err := db.CreateAccount(ctx, pool, "  Ada@Example.COM  ", "hash", "Ada", "UTC")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if account.Email != "ada@example.com" {
		t.Fatalf("stored %q, want it trimmed and lowercased", account.Email)
	}
}

func TestCreateAccountRefusesATakenEmail(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)

	if _, err := db.CreateAccount(ctx, pool, "ada@example.com", "hash", "Ada", "UTC"); err != nil {
		t.Fatalf("first create: %v", err)
	}

	// Different case, surrounding spaces, different name: still the same person.
	_, err := db.CreateAccount(ctx, pool, " ADA@example.com ", "hash", "Someone", "UTC")
	if !errors.Is(err, db.ErrEmailTaken) {
		t.Fatalf("got %v, want ErrEmailTaken", err)
	}
}

func TestCreateAccountRefusesAnUnknownTimezone(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)

	_, err := db.CreateAccount(ctx, pool, "ada@example.com", "hash", "Ada", "Middle/Earth")
	if !errors.Is(err, db.ErrUnknownZone) {
		t.Fatalf("got %v, want ErrUnknownZone", err)
	}
}

func TestAccountLookup(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)

	created, err := db.CreateAccount(ctx, pool, "ada@example.com", "hash", "Ada", "Europe/Oslo")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	t.Run("by email, in any case", func(t *testing.T) {
		found, err := db.AccountByEmail(ctx, pool, "ADA@Example.com")
		if err != nil {
			t.Fatalf("by email: %v", err)
		}
		if found.ID != created.ID {
			t.Fatalf("found %s, want %s", found.ID, created.ID)
		}
	})

	t.Run("an unknown email", func(t *testing.T) {
		if _, err := db.AccountByEmail(ctx, pool, "nobody@example.com"); !errors.Is(err, db.ErrNoAccount) {
			t.Fatalf("got %v, want ErrNoAccount", err)
		}
	})
}

// Each test must start from an empty database, or they pass only in order.
func TestPoolStartsEmpty(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM account").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("%d accounts left over from an earlier test", count)
	}
}
