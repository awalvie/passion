package catalog

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"passion/server/config"
	"passion/server/db"
	"passion/server/db/dbtest"
)

func TestLoadAll(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada, err := db.CreateAccount(ctx, pool, "ada@example.com", "hash", "Ada", "UTC")
	if err != nil {
		t.Fatal(err)
	}

	shipped := mapTree("catalog", map[string]string{"movements/hang.yaml": "id: " + hangID + "\nname: Hang\nstyle: open\n"}).FS
	paradigm := tree(t, map[string]string{"movements/pull.yaml": "id: 11111111-1111-4111-8111-111111111111\nname: Pull\nstyle: open\n"})
	kettle := tree(t, map[string]string{"movements/row.yaml": "id: 22222222-2222-4222-8222-222222222222\nname: Row\nstyle: open\n"})

	var logs bytes.Buffer
	err = LoadAll(ctx, pool, slog.New(slog.NewTextHandler(&logs, nil)), shipped, []config.PrivateCatalog{
		{Location: paradigm, Owner: []string{"Ada@Example.com", "bob@example.com"}},
		{Location: kettle, Owner: []string{"ada@example.com"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	var shippedRows, adaRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE owner IS NULL), count(*) FILTER (WHERE owner = $1) FROM exercise`, ada.ID).
		Scan(&shippedRows, &adaRows); err != nil {
		t.Fatal(err)
	}
	if shippedRows != 1 || adaRows != 2 {
		t.Fatalf("shipped %d, ada %d, want 1 and both of her trees", shippedRows, adaRows)
	}
	if !strings.Contains(logs.String(), "skipped a private catalog: no account yet") || !strings.Contains(logs.String(), "bob@example.com") {
		t.Fatalf("the log does not say bob was skipped:\n%s", logs.String())
	}

	// Dropping Ada from the config retires her rows, and leaves the shipped ones.
	if err := LoadAll(ctx, pool, slog.New(slog.DiscardHandler), shipped, nil); err != nil {
		t.Fatal(err)
	}
	var live int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM exercise WHERE retired_at IS NULL`).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 1 {
		t.Fatalf("%d rows still in the library, want only the shipped one", live)
	}
}

// The same tree named twice for one owner is read once, not refused as a clash.
func TestLoadAllReadsARepeatedTreeOnce(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	if _, err := db.CreateAccount(ctx, pool, "ada@example.com", "hash", "Ada", "UTC"); err != nil {
		t.Fatal(err)
	}

	shipped := mapTree("catalog", map[string]string{"movements/hang.yaml": "id: " + hangID + "\nname: Hang\nstyle: open\n"}).FS
	paradigm := tree(t, map[string]string{"movements/pull.yaml": "id: 11111111-1111-4111-8111-111111111111\nname: Pull\nstyle: open\n"})
	err := LoadAll(ctx, pool, slog.New(slog.DiscardHandler), shipped, []config.PrivateCatalog{
		{Location: paradigm, Owner: []string{"ada@example.com"}},
		{Location: paradigm, Owner: []string{"ada@example.com"}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadAllStopsOnAMissingTree(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	if _, err := db.CreateAccount(ctx, pool, "ada@example.com", "hash", "Ada", "UTC"); err != nil {
		t.Fatal(err)
	}

	shipped := mapTree("catalog", map[string]string{"movements/hang.yaml": "id: " + hangID + "\nname: Hang\nstyle: open\n"}).FS
	err := LoadAll(ctx, pool, slog.New(slog.DiscardHandler), shipped, []config.PrivateCatalog{
		{Location: "/nowhere/at/all", Owner: []string{"ada@example.com"}},
	})
	if err == nil || !strings.Contains(err.Error(), "/nowhere/at/all") {
		t.Fatalf("error %v, want one naming the location", err)
	}
}

func TestLoad(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	good := "id: " + hangID + "\nname: Hang\nstyle: open\nper_side: true\n"

	result, warnings, err := Load(ctx, pool, nil, mapTree("catalog", map[string]string{"movements/hang.yaml": good}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Written != 1 || len(warnings) != 1 {
		t.Fatalf("wrote %d with warnings %q", result.Written, warnings)
	}

	// One bad file stops the whole tree, so the good row is not retired.
	_, _, err = Load(ctx, pool, nil, mapTree("catalog", map[string]string{"movements/pull.yaml": "name: Pull\nstyle: open\n"}))
	if err == nil {
		t.Fatal("a tree with a bad file loaded")
	}
	var retired bool
	if err := pool.QueryRow(ctx, `SELECT retired_at IS NOT NULL FROM exercise WHERE file_id = $1`, hangID).Scan(&retired); err != nil || retired {
		t.Fatalf("retired %v, %v", retired, err)
	}
}
