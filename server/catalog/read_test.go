package catalog

import (
	"strings"
	"testing"
	"testing/fstest"
)

const hangID = "9555c81a-ce5e-4b30-8a0a-159d5852b976"

func mapTree(name string, files map[string]string) Tree {
	fsys := fstest.MapFS{}
	for rel, body := range files {
		fsys[rel] = &fstest.MapFile{Data: []byte(body)}
	}
	return Tree{Name: name, FS: fsys}
}

func readOne(t *testing.T, body string) Exercise {
	t.Helper()
	got, _, err := Read(mapTree("mine", map[string]string{"movements/hang.yaml": body}))
	if err != nil {
		t.Fatal(err)
	}
	return got[0]
}

func TestReadMapsTheFileKeys(t *testing.T) {
	e := readOne(t, `id: 9555C81A-CE5E-4B30-8A0A-159D5852B976
name: '  Max Hang '
style: timed_reps
tags: [fingers, " ", hangboard]
notes: |-
    Ten seconds on.
source: Eva López
media:
    - url: https://www.youtube.com/watch?v=abc
      thumb_url: https://i.ytimg.com/vi/abc/hqdefault.jpg
    - thumb_url: https://example.com/grip.jpg
sets: 5
rep_seconds: 10
seconds: 300
`)

	f := e.Fields
	switch {
	case e.Slug != "hang" || e.FileID != hangID || e.Path != "movements/hang.yaml":
		t.Fatalf("slug %q, id %q, path %q", e.Slug, e.FileID, e.Path)
	case f.Name != "Max Hang" || f.Kind != "timed_reps":
		t.Fatalf("name %q, kind %q", f.Name, f.Kind)
	case strings.Join(f.Tags, ",") != "fingers,hangboard":
		t.Fatalf("tags %v", f.Tags)
	case f.Notes == nil || *f.Notes != "Ten seconds on." || f.Source == nil || *f.Source != "Eva López":
		t.Fatalf("notes %v, source %v", f.Notes, f.Source)
	case len(f.Media) != 2 || *f.Media[0].URL != "https://www.youtube.com/watch?v=abc" || *f.Media[0].ThumbURL != "https://i.ytimg.com/vi/abc/hqdefault.jpg":
		t.Fatalf("media %+v", f.Media)
	case f.Media[1].URL != nil || *f.Media[1].ThumbURL != "https://example.com/grip.jpg":
		t.Fatalf("an image with no video: %+v", f.Media[1])
	case f.Sets == nil || *f.Sets != 5 || f.RepSeconds == nil || *f.RepSeconds != 10:
		t.Fatalf("sets %v, rep seconds %v", f.Sets, f.RepSeconds)
	case f.DurationSeconds == nil || *f.DurationSeconds != 300 || f.Reps != nil:
		t.Fatalf("duration %v, reps %v", f.DurationSeconds, f.Reps)
	}
}

// The hash must follow the content, not how the file happens to be written.
func TestHashFollowsContent(t *testing.T) {
	base := readOne(t, "id: "+hangID+"\nname: Hang\nstyle: open\n")

	same := []string{
		"id: " + hangID + "\nname: Hang\nstyle: open\ntags: []\n",
		"# a comment\nstyle: open\nname: '  Hang'\nid: " + hangID + "\nnotes: '  '\n",
	}
	for _, body := range same {
		if got := readOne(t, body).Hash; got != base.Hash {
			t.Errorf("%q hashed differently from the same content", body)
		}
	}

	changed := []string{
		"id: " + hangID + "\nname: Hang!\nstyle: open\n",
		"id: " + hangID + "\nname: Hang\nstyle: climbing\n",
		"id: " + hangID + "\nname: Hang\nstyle: open\nsets: 0\n",
	}
	for _, body := range changed {
		if got := readOne(t, body).Hash; got == base.Hash {
			t.Errorf("%q hashed the same as different content", body)
		}
	}

	renamed, _, err := Read(mapTree("mine", map[string]string{"movements/long_hang.yaml": "id: " + hangID + "\nname: Hang\nstyle: open\n"}))
	if err != nil {
		t.Fatal(err)
	}
	if renamed[0].Hash == base.Hash {
		t.Error("a renamed file hashed the same, so the loader would keep the old slug")
	}
}

func TestReadWarnsOnKeysItDoesNotStore(t *testing.T) {
	_, warnings, err := Read(mapTree("mine", map[string]string{
		"movements/hang.yaml": "id: " + hangID + "\nname: Hang\nstyle: timed_reps\nper_side: true\nsets: 3\nper_set:\n    - seconds: 3\n    - seconds: 6\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(warnings, "\n") != "mine: movements/hang.yaml: per_side is not stored yet\nmine: movements/hang.yaml: per_set is not stored yet" {
		t.Fatalf("warnings %q", warnings)
	}
}

func TestReadRefuses(t *testing.T) {
	good := "id: " + hangID + "\nname: Hang\nstyle: open\n"
	cases := map[string]struct {
		files map[string]string
		want  string
	}{
		"unknown key":     {map[string]string{"movements/hang.yaml": good + "sets_rest: 3\n"}, "line 4: field sets_rest not found"},
		"no id":           {map[string]string{"movements/hang.yaml": "name: Hang\nstyle: open\n"}, "no id:. Run make catalog-ids"},
		"bad id":          {map[string]string{"movements/hang.yaml": "id: 42\nname: Hang\nstyle: open\n"}, `id: "42" is not a uuid`},
		"bad file name":   {map[string]string{"movements/Hang.yaml": good}, "lower case letters"},
		"bad kind":        {map[string]string{"movements/hang.yaml": "id: " + hangID + "\nname: Hang\nstyle: session\n"}, "style must be"},
		"no name":         {map[string]string{"movements/hang.yaml": "id: " + hangID + "\nstyle: open\n"}, "name is required"},
		"negative number": {map[string]string{"movements/hang.yaml": good + "seconds: -1\n"}, "seconds cannot be negative"},
		"script link":     {map[string]string{"movements/hang.yaml": good + "media:\n  - url: javascript:alert(1)\n"}, "media[0] must hold only http or https links"},
		"two documents":   {map[string]string{"movements/hang.yaml": good + "---\nname: Other\n"}, "more than one document"},
		"empty file":      {map[string]string{"movements/hang.yaml": ""}, "the file is empty"},
		"a subdirectory":  {map[string]string{"movements/hang.yaml": good, "movements/old/pull.yaml": good}, "movements/old: a directory"},
		"no movements":    {map[string]string{"blocks/warm_up.yaml": "name: Warm-up\n"}, "movements"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := Read(mapTree("mine", c.files))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v, want one mentioning %q", err, c.want)
			}
		})
	}
}

// One owner's trees share one set of rows, so a clash across two trees is a
// clash.
func TestReadFindsClashesAcrossTrees(t *testing.T) {
	a := mapTree("paradigm", map[string]string{"movements/hang.yaml": "id: " + hangID + "\nname: Hang\nstyle: open\n"})

	t.Run("same id", func(t *testing.T) {
		b := mapTree("kettle", map[string]string{"movements/long_hang.yaml": "id: " + hangID + "\nname: Long Hang\nstyle: open\n"})
		_, _, err := Read(a, b)
		if err == nil || !strings.Contains(err.Error(), "paradigm: movements/hang.yaml and kettle: movements/long_hang.yaml: the same id") {
			t.Fatalf("error %v", err)
		}
	})

	t.Run("same name", func(t *testing.T) {
		b := mapTree("kettle", map[string]string{"movements/hang.yaml": "id: 11111111-1111-4111-8111-111111111111\nname: Hang\nstyle: open\n"})
		_, _, err := Read(a, b)
		if err == nil || !strings.Contains(err.Error(), "paradigm: movements/hang.yaml and kettle: movements/hang.yaml: the same file name") {
			t.Fatalf("error %v", err)
		}
	})
}

func TestReadReportsEveryBadFile(t *testing.T) {
	_, _, err := Read(mapTree("mine", map[string]string{
		"movements/a.yaml": "name: A\nstyle: open\n",
		"movements/b.yaml": "name: B\nstyle: open\n",
	}))
	if err == nil || !strings.Contains(err.Error(), "movements/a.yaml") || !strings.Contains(err.Error(), "movements/b.yaml") {
		t.Fatalf("error %v, want both files named", err)
	}
}
