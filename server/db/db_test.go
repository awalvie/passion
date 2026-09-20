package db_test

import (
	"context"
	"strings"
	"testing"

	"passion/server/db"
	"passion/server/db/dbtest"
)

func TestOpen(t *testing.T) {
	pool, err := db.Open(context.Background(), dbtest.DSN(t))
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
