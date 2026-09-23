package catalog

import (
	"bytes"
	"context"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

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
	good := "id: " + hangID + "\nname: Hang\nstyle: open\nper_side: true\nper_set:\n    - seconds: 3\n"

	result, warnings, err := Load(ctx, pool, nil, nil, mapTree("catalog", map[string]string{"movements/hang.yaml": good}))
	if err != nil {
		t.Fatal(err)
	}
	if result.Exercises.Written != 1 || len(warnings) != 1 {
		t.Fatalf("wrote %d with warnings %q", result.Exercises.Written, warnings)
	}
	var perSide bool
	if err := pool.QueryRow(ctx, `SELECT per_side FROM exercise WHERE file_id = $1`, hangID).Scan(&perSide); err != nil || !perSide {
		t.Fatalf("per side %v, %v, want true", perSide, err)
	}

	// One bad file stops the whole tree, so the good row is not retired.
	_, _, err = Load(ctx, pool, nil, nil, mapTree("catalog", map[string]string{"movements/pull.yaml": "name: Pull\nstyle: open\n"}))
	if err == nil {
		t.Fatal("a tree with a bad file loaded")
	}
	var retired bool
	if err := pool.QueryRow(ctx, `SELECT retired_at IS NOT NULL FROM exercise WHERE file_id = $1`, hangID).Scan(&retired); err != nil || retired {
		t.Fatalf("retired %v, %v", retired, err)
	}
}

func TestLoadSessions(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	files := map[string]string{
		"movements/hang.yaml": movement(hangID, "Max Hang"),
		"blocks/b.yaml":       "id: " + blockID + "\nname: Fingers\nitems:\n    - movement: hang\n",
		"sessions/s.yaml":     "id: " + sessionID + "\nname: Power\nitems:\n    - block: b\n",
	}
	load := func(want Result) {
		t.Helper()
		got, _, err := Load(ctx, pool, nil, nil, mapTree("catalog", files))
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	}
	wrote := func(exercises, sessions int) Result {
		return Result{Exercises: db.LoadResult{Written: exercises}, Sessions: db.LoadResult{Written: sessions}}
	}

	load(wrote(1, 1))
	load(wrote(0, 0))

	var stepExercise, hangRow string
	if err := pool.QueryRow(ctx, `
		SELECT body->'sections'->0->'items'->0->'step'->>'exercise', (SELECT id::text FROM exercise WHERE file_id = $1)
		FROM session_template WHERE file_id = $2`, hangID, sessionID).Scan(&stepExercise, &hangRow); err != nil {
		t.Fatal(err)
	}
	if stepExercise != hangRow {
		t.Fatalf("the step holds %s, want the hang's row %s", stepExercise, hangRow)
	}

	// A block edit reaches the session, and a movement edit reaches both.
	files["blocks/b.yaml"] = "id: " + blockID + "\nname: Fingers\nitems:\n    - movement: hang\n      sets: 3\n"
	load(wrote(0, 1))
	files["movements/hang.yaml"] = movement(hangID, "Max Hangs")
	load(wrote(1, 1))

	delete(files, "sessions/s.yaml")
	load(Result{Sessions: db.LoadResult{Retired: 1}})
}

func TestLoadAllLoadsPrivateSessions(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada, err := db.CreateAccount(ctx, pool, "ada@example.com", "hash", "Ada", "UTC")
	if err != nil {
		t.Fatal(err)
	}

	shipped := mapTree("catalog", map[string]string{
		"movements/max_hangs.yaml": movement(appHangID, "Max Hangs"),
		"blocks/fingers.yaml":      "id: " + blockID + "\nname: Fingers\nitems:\n    - movement: max_hangs\n",
	}).FS
	mine := tree(t, map[string]string{
		"movements/pull.yaml": movement(pullID, "Pull-up"),
		"blocks/pulls.yaml":   "id: " + blockID + "\nname: Pulls\nitems:\n    - movement: pull\n    - movement: app:max_hangs\n",
		"sessions/power.yaml": "id: " + sessionID + "\nname: Power\nitems:\n    - block: app:fingers\n    - block: pulls\n    - block: pulls\n",
	})
	private := []config.PrivateCatalog{{Location: mine, Owner: []string{"ada@example.com"}}}

	if err := LoadAll(ctx, pool, slog.New(slog.DiscardHandler), shipped, private); err != nil {
		t.Fatal(err)
	}

	// Each step, in order, holds the shipped row or ada's own.
	rows, err := pool.Query(ctx, `
		SELECT e.owner IS NULL
		FROM session_template s,
			jsonb_path_query(s.body, '$.sections[*].items[*].step.exercise') WITH ORDINALITY AS step(id, n)
			JOIN exercise e ON e.id = (step.id #>> '{}')::uuid
		WHERE s.owner = $1
		ORDER BY step.n`, ada.ID)
	if err != nil {
		t.Fatal(err)
	}
	shippedSteps, err := pgx.CollectRows(rows, pgx.RowTo[bool])
	if err != nil {
		t.Fatal(err)
	}
	if want := []bool{true, false, true, false, true}; !slices.Equal(shippedSteps, want) {
		t.Fatalf("shipped steps %v, want %v", shippedSteps, want)
	}

	// Dropping Ada from the config retires her session too.
	if err := LoadAll(ctx, pool, slog.New(slog.DiscardHandler), shipped, nil); err != nil {
		t.Fatal(err)
	}
	var live int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM session_template WHERE retired_at IS NULL`).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 0 {
		t.Fatalf("%d sessions still live, want none", live)
	}
}
