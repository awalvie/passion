package catalog

import (
	"context"
	"testing"

	"passion/server/db/dbtest"
)

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
