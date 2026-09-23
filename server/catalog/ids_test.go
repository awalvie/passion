package catalog

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// tree writes files, keyed by path relative to the tree, and returns its root.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// addedID checks that got is want with one id line inserted at the byte
// offset at, and returns the id.
func addedID(t *testing.T, got, want string, at int, eol string) string {
	t.Helper()
	if !strings.HasPrefix(got, want[:at]+"id: ") || !strings.HasSuffix(got, eol+want[at:]) {
		t.Fatalf("got %q, want an id line at byte %d of %q", got, at, want)
	}
	id := strings.TrimSuffix(strings.TrimPrefix(got, want[:at]+"id: "), eol+want[at:])
	if !uuidV4.MatchString(id) {
		t.Fatalf("id %q is not a version 4 uuid", id)
	}
	return id
}

func TestAddIDs(t *testing.T) {
	const hang = "name: Hang\nkind: timed_reps\n"
	const pullup = "name: Pull-up\n# a comment stays\nid: 9555c81a-ce5e-4b30-8a0a-159d5852b976\n"
	root := tree(t, map[string]string{
		"catalog.yaml":        "format_version: 2\n",
		"movements/hang.yaml": hang,
		"movements/pull.yaml": pullup,
		"movements/notes.txt": "not a catalog file\n",
		"blocks/warm_up.yaml": "name: Warm-up\nitems:\n  - movement: hang\n",
	})

	changed, err := AddIDs(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(changed, ",") != "movements/hang.yaml,blocks/warm_up.yaml" {
		t.Fatalf("changed %v", changed)
	}

	addedID(t, read(t, root, "movements/hang.yaml"), hang, 0, "\n")

	// An id further down is still an id.
	if got := read(t, root, "movements/pull.yaml"); got != pullup {
		t.Fatalf("a file with an id changed: %q", got)
	}
	if got := read(t, root, "catalog.yaml"); got != "format_version: 2\n" {
		t.Fatalf("catalog.yaml changed: %q", got)
	}

	again, err := AddIDs(root)
	if err != nil || len(again) != 0 {
		t.Fatalf("a second run changed %v, %v", again, err)
	}
}

func TestEveryFileGetsItsOwnID(t *testing.T) {
	root := tree(t, map[string]string{
		"movements/a.yaml": "name: A\n",
		"movements/b.yaml": "name: B\n",
	})
	if _, err := AddIDs(root); err != nil {
		t.Fatal(err)
	}
	a := addedID(t, read(t, root, "movements/a.yaml"), "name: A\n", 0, "\n")
	b := addedID(t, read(t, root, "movements/b.yaml"), "name: B\n", 0, "\n")
	if a == b {
		t.Fatalf("both files got %s", a)
	}
}

func TestIDPlacement(t *testing.T) {
	cases := map[string]struct {
		body string
		at   int
		eol  string
	}{
		"document marker":  {"---\nname: Hang\n", 4, "\n"},
		"byte order mark":  {"\xef\xbb\xbfname: Hang\n", 3, "\n"},
		"crlf":             {"name: Hang\r\nkind: open\r\n", 0, "\r\n"},
		"crlf with marker": {"---\r\nname: Hang\r\n", 5, "\r\n"},
		"leading comment":  {"# the hang\nname: Hang\n", 0, "\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			root := tree(t, map[string]string{"movements/hang.yaml": c.body})
			if _, err := AddIDs(root); err != nil {
				t.Fatal(err)
			}
			addedID(t, read(t, root, "movements/hang.yaml"), c.body, c.at, c.eol)
		})
	}
}

func TestAddIDsRefuses(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"empty id":               {"id:\nname: Hang\n", "holds no uuid"},
		"null id":                {"id: null\nname: Hang\n", "holds no uuid"},
		"tilde id":               {"id: ~\nname: Hang\n", "holds no uuid"},
		"mapping id":             {"id:\n  a: b\nname: Hang\n", "holds no uuid"},
		"empty file":             {"", "empty"},
		"only comments":          {"# nothing yet\n", "empty"},
		"a list at the top":      {"- name: Hang\n", "want a mapping"},
		"broken yaml":            {"name: [Hang\n", "yaml"},
		"comment above a marker": {"# the hang\n---\nname: Hang\n", "by hand"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			root := tree(t, map[string]string{
				"movements/hang.yaml": c.body,
				"movements/pull.yaml": "name: Pull-up\n",
			})

			changed, err := AddIDs(root)
			if err == nil || !strings.Contains(err.Error(), "movements/hang.yaml") || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v, want one naming the file and %q", err, c.want)
			}
			if got := read(t, root, "movements/hang.yaml"); got != c.body {
				t.Fatalf("the refused file changed: %q", got)
			}
			// One bad file must not stop the others.
			if strings.Join(changed, ",") != "movements/pull.yaml" {
				t.Fatalf("changed %v", changed)
			}
		})
	}
}

func TestCopiedIDIsRefused(t *testing.T) {
	const id = "id: 9555c81a-ce5e-4b30-8a0a-159d5852b976\n"
	root := tree(t, map[string]string{
		"movements/hang.yaml":      id + "name: Hang\n",
		"movements/hang_copy.yaml": id + "name: Hang, again\n",
	})

	_, err := AddIDs(root)
	if err == nil || !strings.Contains(err.Error(), "movements/hang_copy.yaml: its id is also in movements/hang.yaml") {
		t.Fatalf("error %v, want one naming both files", err)
	}
}

func TestAddIDsNeedsATree(t *testing.T) {
	root := tree(t, map[string]string{"exercises/hang.yaml": "name: Hang\n"})
	if _, err := AddIDs(root); err == nil || !strings.Contains(err.Error(), "movements/") {
		t.Fatalf("error %v, want one naming the directories it looks for", err)
	}
}
