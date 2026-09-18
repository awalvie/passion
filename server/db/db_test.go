package db_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"passion/server/db"
)

// dsn fails the run rather than skipping it. A skipped database test is a test
// nobody notices has stopped running.
func dsn(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TEST_DATABASE_URL")
	if v == "" {
		t.Fatal("TEST_DATABASE_URL is not set: run inside the nix shell, after `make db-up`")
	}
	return v
}

func TestOpen(t *testing.T) {
	pool, err := db.Open(context.Background(), dsn(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestOpenRefusesAnUnreachableServer(t *testing.T) {
	_, err := db.Open(context.Background(), "postgres://nobody@127.0.0.1:1/nothing")
	if err == nil {
		t.Fatal("expected an error, got none")
	}
	if !strings.Contains(err.Error(), "postgres") {
		t.Fatalf("error does not name postgres: %v", err)
	}
}
