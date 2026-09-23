package catalog_test

import (
	"testing"

	shipped "passion/catalog"
	"passion/server/catalog"
)

// A bad shipped file would stop every server at startup, so it has to fail
// here first.
func TestShippedCatalogReads(t *testing.T) {
	got, _, err := catalog.Read(catalog.Tree{Name: "catalog", FS: shipped.Files})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("the shipped catalog is empty")
	}
}
