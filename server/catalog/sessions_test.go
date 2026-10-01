package catalog

import (
	"slices"
	"strings"
	"testing"
)

const (
	pullID    = "11111111-1111-4111-8111-111111111111"
	blockID   = "33333333-3333-4333-8333-333333333333"
	sessionID = "22222222-2222-4222-8222-222222222222"
	appHangID = "44444444-4444-4444-8444-444444444444"
)

func movement(id, name string) string {
	return "id: " + id + "\nname: " + name + "\nstyle: timed_reps\nsets: 5\nnotes: Ten seconds on.\n"
}

// readSessions reads trees as one owner's, with app as the shipped catalog.
func readSessions(t *testing.T, app *Tree, trees ...Tree) ([]Session, []string, error) {
	t.Helper()
	exercises, _, err := Read(trees...)
	if err != nil {
		t.Fatalf("read movements: %v", err)
	}
	return ReadSessions(app, exercises, trees...)
}

func TestReadSessionsCopiesBlocksIn(t *testing.T) {
	tree := mapTree("mine", map[string]string{
		"movements/hang.yaml": movement(hangID, "Max Hang"),
		"movements/pull.yaml": movement(pullID, "Pull-up"),
		"blocks/warm_up.yaml": `id: ` + blockID + `
name: Warm-up
tags: [warmup]
items:
    - movement: hang
      sets: 3
    - menu:
        name: Pulls
        notes: One is enough.
        pick: 1
        of: [hang, pull]
`,
		"sessions/power.yaml": `id: ` + sessionID + `
name: Power
color: '#5D86C9'
icon: hand
items:
    - block: warm_up
    - block: warm_up
`,
	})

	sessions, warnings, err := readSessions(t, nil, tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "blocks/warm_up.yaml") {
		t.Fatalf("warnings %q, want one for the block's tags", warnings)
	}

	s := sessions[0]
	if s.Slug != "power" || s.FileID != sessionID || *s.Fields.Color != "#5d86c9" || *s.Fields.Icon != "hand" {
		t.Fatalf("slug %q, id %q, color %q", s.Slug, s.FileID, *s.Fields.Color)
	}
	if len(s.Fields.Body.Sections) != 2 || s.Fields.Body.Sections[1].Name != "Warm-up" {
		t.Fatalf("sections %+v, want the block twice", s.Fields.Body.Sections)
	}

	items := s.Fields.Body.Sections[0].Items
	step := items[0].Step
	if step.Exercise != hangID || *step.Sets != 3 || step.Notes == nil || *step.Notes != "Ten seconds on." {
		t.Fatalf("step %+v, want the hang with its notes and the block's 3 sets", step)
	}
	choice := items[1].Choice
	if choice.Name != "Pulls" || choice.Pick != 1 || len(choice.Options) != 2 || choice.Options[1].Exercise != pullID {
		t.Fatalf("choice %+v", choice)
	}
	if *choice.Options[0].Sets != 5 {
		t.Fatalf("option sets %d, want the movement's own 5", *choice.Options[0].Sets)
	}
	if want := []bool{false, false, false, false, false, false}; !slices.Equal(s.AppSteps, want) {
		t.Fatalf("app steps %v, want %v", s.AppSteps, want)
	}
}

func TestReadSessionsResolvesAppNames(t *testing.T) {
	app := mapTree("catalog", map[string]string{
		"movements/max_hangs.yaml": movement(appHangID, "Max Hangs"),
		"blocks/fingers.yaml":      "id: " + blockID + "\nname: Fingers\nitems:\n    - movement: max_hangs\n",
	})
	mine := mapTree("mine", map[string]string{
		// A copy of a shipped file keeps its id, and is still this owner's own.
		"movements/max_hangs.yaml": movement(appHangID, "My Max Hangs"),
		"blocks/mix.yaml":          "id: " + blockID + "\nname: Mix\nitems:\n    - movement: app:max_hangs\n    - movement: max_hangs\n",
		"sessions/power.yaml":      "id: " + sessionID + "\nname: Power\nitems:\n    - block: app:fingers\n    - block: mix\n",
	})

	sessions, _, err := readSessions(t, &app, mine)
	if err != nil {
		t.Fatal(err)
	}
	s := sessions[0]
	if want := []bool{true, true, false}; !slices.Equal(s.AppSteps, want) {
		t.Fatalf("app steps %v, want %v", s.AppSteps, want)
	}
	shippedStep := s.Fields.Body.Sections[0].Items[0].Step
	ownStep := s.Fields.Body.Sections[1].Items[1].Step
	if shippedStep.Name != "Max Hangs" || ownStep.Name != "My Max Hangs" {
		t.Fatalf("steps %q and %q, want the shipped one and the copy", shippedStep.Name, ownStep.Name)
	}
}

func TestReadSessionsProblems(t *testing.T) {
	app := mapTree("catalog", map[string]string{
		"movements/max_hangs.yaml": movement(appHangID, "Max Hangs"),
	})
	hang := movement(hangID, "Max Hang")
	withBlock := func(block string) map[string]string {
		return map[string]string{
			"movements/hang.yaml": hang,
			"blocks/b.yaml":       "id: " + blockID + "\nname: B\n" + block,
			"sessions/s.yaml":     "id: " + sessionID + "\nname: S\nitems:\n    - block: b\n",
		}
	}

	cases := map[string]struct {
		files map[string]string
		want  string
	}{
		"a missing movement": {withBlock("items:\n    - movement: nope\n"), `items[0]: no movement "nope"`},
		"a shipped name without app:": {withBlock("items:\n    - movement: max_hangs\n"),
			`write "app:max_hangs"`},
		"a block with no items":     {withBlock("items: []\n"), "a block holds at least one"},
		"a menu with numbers":       {withBlock("items:\n    - menu: {name: M, pick: 1, of: [hang]}\n      sets: 3\n"), "a menu carries no numbers"},
		"a menu with no pick":       {withBlock("items:\n    - menu: {name: M, of: [hang]}\n"), "items[0].menu.pick: is required"},
		"a pick above the options":  {withBlock("items:\n    - menu: {name: M, pick: 2, of: [hang]}\n"), "items[0].menu.pick: cannot be more than the options"},
		"a menu with no name":       {withBlock("items:\n    - menu: {pick: 1, of: [hang]}\n"), "items[0].menu.name: is required"},
		"a negative override":       {withBlock("items:\n    - movement: hang\n      sets: -1\n"), "items[0].step.sets: cannot be negative"},
		"an item that is both":      {withBlock("items:\n    - movement: hang\n      menu: {name: M, pick: 1, of: [hang]}\n"), "not both"},
		"an unknown key on an item": {withBlock("items:\n    - movement: hang\n      weight_kg: 10\n"), "weight_kg"},
		"a block with no id":        {map[string]string{"movements/hang.yaml": hang, "blocks/b.yaml": "name: B\nitems:\n    - movement: hang\n"}, "no id:"},
		"a missing block": {map[string]string{
			"movements/hang.yaml": hang,
			"sessions/s.yaml":     "id: " + sessionID + "\nname: S\nitems:\n    - block: nope\n",
		}, `items[0]: no block "nope"`},
		"a session with no items": {map[string]string{
			"movements/hang.yaml": hang,
			"sessions/s.yaml":     "id: " + sessionID + "\nname: S\nitems: []\n",
		}, "a session holds at least one block"},
		"a session with a movement": {map[string]string{
			"movements/hang.yaml": hang,
			"sessions/s.yaml":     "id: " + sessionID + "\nname: S\nitems:\n    - movement: hang\n",
		}, "movement"},
		"a bad colour": {map[string]string{
			"movements/hang.yaml": hang,
			"blocks/b.yaml":       "id: " + blockID + "\nname: B\nitems:\n    - movement: hang\n",
			"sessions/s.yaml":     "id: " + sessionID + "\nname: S\ncolor: blue\nitems:\n    - block: b\n",
		}, "color must look like"},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, _, err := readSessions(t, &app, mapTree("mine", c.files))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error with %q", err, c.want)
			}
		})
	}
}

func TestReadSessionsReportsABrokenBlockOnce(t *testing.T) {
	tree := mapTree("mine", map[string]string{
		"movements/hang.yaml": movement(hangID, "Max Hang"),
		"blocks/b.yaml":       "id: " + blockID + "\nname: B\nitems:\n    - movement: nope\n",
		"sessions/s1.yaml":    "id: " + sessionID + "\nname: S1\nitems:\n    - block: b\n",
		"sessions/s2.yaml":    "id: " + pullID + "\nname: S2\nitems:\n    - block: b\n",
	})

	_, _, err := readSessions(t, nil, tree)
	if err == nil {
		t.Fatal("read a broken block")
	}
	if n := strings.Count(err.Error(), `no movement "nope"`); n != 1 {
		t.Fatalf("the block's problem appears %d times:\n%v", n, err)
	}
	if !strings.Contains(err.Error(), `sessions/s2.yaml: items[0]: block "b" has problems of its own`) {
		t.Fatalf("got %v, want each session to name the block", err)
	}
}

func TestReadSessionsInTheShippedCatalog(t *testing.T) {
	tree := mapTree("catalog", map[string]string{
		"movements/hang.yaml": movement(hangID, "Max Hang"),
		"blocks/b.yaml":       "id: " + blockID + "\nname: B\nitems:\n    - movement: app:hang\n",
	})
	_, _, err := readSessions(t, nil, tree)
	if err == nil || !strings.Contains(err.Error(), "names its own files without app:") {
		t.Fatalf("got %v, want app: refused inside the shipped catalog", err)
	}
}

func TestReadSessionsClashAcrossTrees(t *testing.T) {
	block := "id: " + blockID + "\nname: B\nitems:\n    - movement: hang\n"
	a := mapTree("a", map[string]string{
		"movements/hang.yaml": movement(hangID, "Max Hang"),
		"blocks/b.yaml":       block,
		"sessions/s.yaml":     "id: " + sessionID + "\nname: S\nitems:\n    - block: b\n",
	})
	b := mapTree("b", map[string]string{
		"movements/pull.yaml": movement(pullID, "Pull-up"),
		"blocks/b.yaml":       block,
		"sessions/s.yaml":     "id: " + pullID + "\nname: S\nitems:\n    - block: b\n",
	})

	_, _, err := readSessions(t, nil, a, b)
	if err == nil {
		t.Fatal("two trees shared a block and a session name")
	}
	for _, want := range []string{"the same block", "the same slug"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("got %v, want %q", err, want)
		}
	}
}
