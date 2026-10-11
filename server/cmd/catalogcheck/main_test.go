package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tree(t *testing.T, block string) string {
	t.Helper()
	dir := t.TempDir()
	for _, d := range []string{"movements", "sessions"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	session := "id: 22222222-2222-4222-8222-222222222222\nname: Power\nitems:\n    - block: " + block + "\n"
	if err := os.WriteFile(filepath.Join(dir, "sessions", "power.yaml"), []byte(session), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckResolvesShippedNames(t *testing.T) {
	if _, err := check(tree(t, "app:warm_up")); err != nil {
		t.Fatal(err)
	}
}

func TestCheckRefusesUnknownNames(t *testing.T) {
	_, err := check(tree(t, "app:no_such_block"))
	if err == nil || !strings.Contains(err.Error(), "no_such_block") {
		t.Fatalf("err %v, want one naming no_such_block", err)
	}
}
