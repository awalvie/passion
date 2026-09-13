package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// writeTree puts a fixture on a real disk, because minting writes files and fstest.MapFS
// cannot be written to.
func writeTree(t *testing.T, fsys fstest.MapFS) string {
	t.Helper()
	dir := t.TempDir()
	for name, f := range fsys {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, f.Data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// A tree is hand-edited and hand-reviewed, so minting an id must not disturb anything else
// in the file. One line is added, and a header comment keeps its place at the top.
func TestMintingAnIDLeavesTheRestOfTheFileAlone(t *testing.T) {
	body := `# how this one is done, in our words
name: "Easy Start"
style: "open"
notes: |
  Keep it slow.
tags: ["warmup"]
`
	dir := writeTree(t, fstest.MapFS{
		"catalog.yaml":                &fstest.MapFile{Data: []byte("format_version: 2\n")},
		"movements/easy_start.yaml":   &fstest.MapFile{Data: []byte(body)},
		"blocks/warm_up.yaml":         &fstest.MapFile{Data: []byte("name: \"Warm-up\"\nitems:\n  - movement: \"easy_start\"\n")},
		"sessions/easy_session.yaml":  &fstest.MapFile{Data: []byte("name: \"Easy\"\nitems:\n  - block: \"warm_up\"\n")},
		"movements/.not_yaml.txt.bak": &fstest.MapFile{Data: []byte("ignore me")},
	})

	wrote, err := MintIDs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(wrote) != 3 {
		t.Errorf("wrote %d files, want 3: %v", len(wrote), wrote)
	}

	got, err := os.ReadFile(filepath.Join(dir, "movements", "easy_start.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(got), "\n")
	if lines[0] != "# how this one is done, in our words" {
		t.Errorf("the header comment moved: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "id: ") {
		t.Errorf("the id is not on line two: %q", lines[1])
	}
	if !idPattern.MatchString(strings.TrimPrefix(lines[1], "id: ")) {
		t.Errorf("the minted id is not a uuid: %q", lines[1])
	}
	// Everything else, byte for byte.
	if rest := strings.Join(lines[2:], "\n"); rest != strings.TrimPrefix(body, "# how this one is done, in our words\n") {
		t.Errorf("the rest of the file changed:\n%s", rest)
	}
}

// Minting twice writes nothing the second time, so booting the app does not churn a tree
// kept in git.
func TestMintingTwiceWritesNothingTheSecondTime(t *testing.T) {
	dir := writeTree(t, fstest.MapFS{
		"catalog.yaml":              &fstest.MapFile{Data: []byte("format_version: 2\n")},
		"movements/easy_start.yaml": &fstest.MapFile{Data: []byte("name: \"Easy Start\"\nstyle: \"open\"\n")},
	})

	if _, err := MintIDs(dir); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, "movements", "easy_start.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	wrote, err := MintIDs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(wrote) != 0 {
		t.Errorf("a second mint rewrote %v", wrote)
	}
	second, err := os.ReadFile(filepath.Join(dir, "movements", "easy_start.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Error("a second mint changed the file")
	}
}

// MissingIDs is what a read-only tree and a CI check both need: the same answer, without
// writing.
func TestMissingIDsNamesTheFilesAndWritesNothing(t *testing.T) {
	dir := writeTree(t, fstest.MapFS{
		"catalog.yaml":              &fstest.MapFile{Data: []byte("format_version: 2\n")},
		"movements/has_one.yaml":    &fstest.MapFile{Data: []byte("id: 11111111-2222-4333-8444-555555555555\nname: \"A\"\nstyle: \"open\"\n")},
		"movements/easy_start.yaml": &fstest.MapFile{Data: []byte("name: \"Easy Start\"\nstyle: \"open\"\n")},
	})

	missing, err := MissingIDs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || !strings.HasSuffix(missing[0], "easy_start.yaml") {
		t.Errorf("missing = %v, want only easy_start.yaml", missing)
	}
	if _, err := os.ReadFile(filepath.Join(dir, "movements", "easy_start.yaml")); err != nil {
		t.Fatal(err)
	}
	again, err := MissingIDs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != len(missing) {
		t.Error("MissingIDs changed the tree")
	}
}

// A family typed by hand cannot be checked at import time, because nothing ever resolves a
// family to a row. This is the only place the mistake is visible.
func TestUnknownFamiliesNamesAFamilyThatMatchesNothing(t *testing.T) {
	tree := with(goodTree(), "movements/general_warmup.yaml", `
id: 11111111-2222-4333-8444-555555555555
family: 99999999-9999-4999-8999-999999999999
name: "General Warm-up"
style: "open"
tags: ["warmup"]
`)
	t1, err := Load(tree, "private", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := UnknownFamilies(t1)
	if len(got) != 1 {
		t.Fatalf("%d reported, want 1: %v", len(got), got)
	}
	if !strings.Contains(got[0], "general_warmup.yaml") {
		t.Errorf("the report does not name the file: %q", got[0])
	}

	// The same family is fine once the row it names is in one of the trees given.
	ok := with(tree, "movements/hangboard_ladder.yaml", `
id: 99999999-9999-4999-8999-999999999999
name: "Hangboard Ladder"
style: "timed_reps"
tags: ["fingers"]
`)
	t2, err := Load(ok, "private", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := UnknownFamilies(t2); len(got) != 0 {
		t.Errorf("a family naming a file in the same tree was reported: %v", got)
	}
}

// The reason the id lives in the file at all. Export a tree, restore it into a database
// that has never seen it, and every row keeps the identity history points at. An id minted
// at import time would be new on the far side, and every old log row would name nothing.
func TestIdentitySurvivesAnExportAndRestore(t *testing.T) {
	eachEngine(t, func(t *testing.T, first *Store) {
		ctx := context.Background()
		if _, err := first.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}
		before := identities(t, first)

		files, err := first.ExportTree(ctx, ShippedTree, nil)
		if err != nil {
			t.Fatal(err)
		}
		second := freshSQLite(t)
		tree, err := Load(asFS(files), ShippedTree, nil)
		if err != nil {
			t.Fatalf("the export does not load: %v", err)
		}
		if _, err := second.ImportShipped(ctx, tree); err != nil {
			t.Fatal(err)
		}

		after := identities(t, second)
		for slug, want := range before {
			// A menu has no file, so its id is minted on each side and is expected to
			// differ. Nothing logs a menu, so nothing reads it.
			if strings.HasSuffix(slug, "_0") || strings.HasSuffix(slug, "_1") {
				continue
			}
			if after[slug] != want {
				t.Errorf("%s: identity %q became %q across the round trip",
					slug, want, after[slug])
			}
		}
	})
}

func identities(t *testing.T, s *Store) map[string]string {
	t.Helper()
	var rows []Content
	if err := s.read(context.Background()).Order("kind, slug").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, r := range rows {
		out[r.Kind+"/"+r.Slug] = r.UUID + " " + r.Family
	}
	return out
}
