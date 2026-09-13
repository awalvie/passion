package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeApp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "passion.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// A private tree names its owner by EMAIL. The dead config named an id, the account it
// named was deleted, and the import went on writing rows under an id nothing pointed at.
func TestAPrivateCatalogTreeCarriesADirAndAnOwner(t *testing.T) {
	cfg, err := LoadApp(writeApp(t, `
auth:
  jwt_secret: "a-secret-long-enough-to-pass-validation-0123456789"
catalog:
  import: true
  private:
    - dir: ../passion-private-catalog
      owner: someone@example.test
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Catalog.Private) != 1 {
		t.Fatalf("%d private trees, want 1", len(cfg.Catalog.Private))
	}
	got := cfg.Catalog.Private[0]
	if got.Dir != "../passion-private-catalog" || got.Owner != "someone@example.test" {
		t.Errorf("got %+v", got)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("a complete private tree was refused: %v", err)
	}
}

// A tree with no owner would write rows nobody holds. Saying so at boot beats finding it
// in the database later.
func TestAPrivateTreeWithNoOwnerIsRefused(t *testing.T) {
	// LoadApp validates, so this never reaches Validate on its own.
	_, err := LoadApp(writeApp(t, `
auth:
  jwt_secret: "a-secret-long-enough-to-pass-validation-0123456789"
catalog:
  private:
    - dir: ../passion-private-catalog
`))
	if err == nil {
		t.Fatal("a private tree with no owner was accepted")
	}
	for _, want := range []string{"catalog.private[0].owner", "email"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}
}

func TestTheCatalogImportsByDefault(t *testing.T) {
	if !DefaultApp().Catalog.Import {
		t.Error("the catalog import is off by default")
	}
}
