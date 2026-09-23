package catalog_test

import (
	"testing"

	shipped "passion/catalog"
	"passion/server/catalog"
)

// A bad shipped file would stop every server at startup, so it has to fail
// here first.
func TestShippedCatalogReads(t *testing.T) {
	tree := catalog.Tree{Name: "catalog", FS: shipped.Files}
	exercises, _, err := catalog.Read(tree)
	if err != nil {
		t.Fatal(err)
	}
	sessions, _, err := catalog.ReadSessions(nil, exercises, tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(exercises) == 0 || len(sessions) == 0 {
		t.Fatalf("%d exercises and %d sessions, want some of each", len(exercises), len(sessions))
	}
}
