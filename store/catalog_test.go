package store

import (
	"strings"
	"testing"
	"testing/fstest"
)

// A tree small enough to read in one screen and complete enough to load. Every test starts
// from this and breaks one thing, so what each test is about is the diff.
func goodTree() fstest.MapFS {
	return fstest.MapFS{
		"catalog.yaml": &fstest.MapFile{Data: []byte("format_version: 1\n")},
		"tags.yaml": &fstest.MapFile{Data: []byte(`
- slug: fingers
  name: Fingers
- slug: strength
  name: Strength
- slug: warmup
  name: Warm-up
`)},
		"movements/half_crimp_hang.yaml": &fstest.MapFile{Data: []byte(`
name: "Half Crimp Hang"
slug: "half_crimp_hang"
kind: "timed_reps"
tags: ["fingers", "strength"]
sets: 4
reps: 6
rep_seconds: 7
media:
  - url: "https://example.test/v"
    thumb_url: "https://example.test/t"
`)},
		"movements/hangboard_ladder.yaml": &fstest.MapFile{Data: []byte(`
name: "Hangboard Ladder"
slug: "hangboard_ladder"
kind: "timed_reps"
tags: ["fingers"]
sets: 3
per_set:
  - seconds: 3
  - seconds: 6
  - seconds: 9
`)},
		"movements/general_warmup.yaml": &fstest.MapFile{Data: []byte(`
name: "General Warm-up"
slug: "general_warmup"
kind: "open"
tags: ["warmup"]
`)},
		"menus/hang_choice.yaml": &fstest.MapFile{Data: []byte(`
name: "Pick a hang"
slug: "hang_choice"
pick: 1
notes: |
  One of these per session.
options:
  - ref: "half_crimp_hang"
  - ref: "hangboard_ladder"
`)},
		"blocks/fingers_block.yaml": &fstest.MapFile{Data: []byte(`
name: "Fingers"
slug: "fingers_block"
tags: ["fingers"]
items:
  - ref: "hang_choice"
`)},
		"blocks/warm_up.yaml": &fstest.MapFile{Data: []byte(`
name: "Warm-up"
slug: "warm_up"
tags: ["warmup"]
role: "warmup"
items:
  - ref: "general_warmup"
`)},
		"sessions/finger_day.yaml": &fstest.MapFile{Data: []byte(`
name: "Finger Day"
slug: "finger_day"
color: "#ef4444"
tags: ["fingers"]
needs: "hangboard"
items:
  - ref: "warm_up"
  - ref: "fingers_block"
`)},
	}
}

// rename is a fixture edit: replace one file's body.
func with(fsys fstest.MapFS, path, body string) fstest.MapFS {
	out := fstest.MapFS{}
	for k, v := range fsys {
		out[k] = v
	}
	out[path] = &fstest.MapFile{Data: []byte(body)}
	return out
}

func TestAGoodTreeLoads(t *testing.T) {
	tree, err := Load(goodTree(), "shipped")
	if err != nil {
		t.Fatalf("a valid tree failed to load: %v", err)
	}
	if n := len(tree.Movements); n != 3 {
		t.Errorf("%d movements, want 3", n)
	}
	if n := len(tree.Menus); n != 1 {
		t.Errorf("%d menus, want 1", n)
	}
	if n := len(tree.Blocks); n != 2 {
		t.Errorf("%d blocks, want 2", n)
	}
	if n := len(tree.Sessions); n != 1 {
		t.Errorf("%d sessions, want 1", n)
	}
	if n := len(tree.Tags); n != 3 {
		t.Errorf("%d tags, want 3", n)
	}
	if tree.Name != "shipped" {
		t.Errorf("tree name is %q, want shipped", tree.Name)
	}

	// Files load in filename order, so an import is repeatable.
	if tree.Movements[0].Slug != "general_warmup" {
		t.Errorf("movements are not in filename order: first is %q", tree.Movements[0].Slug)
	}

	// The ladder survives as three rungs, in order.
	var ladder *MovementFile
	for i := range tree.Movements {
		if tree.Movements[i].Slug == "hangboard_ladder" {
			ladder = &tree.Movements[i]
		}
	}
	if ladder == nil {
		t.Fatal("the ladder movement is missing")
	}
	if n := len(ladder.PerSet); n != 3 {
		t.Fatalf("the ladder has %d rungs, want 3", n)
	}
	for i, want := range []int{3, 6, 9} {
		if got := ladder.PerSet[i].Seconds; got == nil || *got != want {
			t.Errorf("rung %d is %v, want %d", i, got, want)
		}
	}

	// Optional keys stay absent rather than becoming zero. 0 kg is bodyweight and 0 reps is
	// not the same as no target, so this distinction has to survive parsing.
	warm := tree.Movements[0]
	if warm.Sets != nil || warm.Reps != nil {
		t.Errorf("an absent number parsed as a value: sets=%v reps=%v", warm.Sets, warm.Reps)
	}
}

// The rule that makes the spec normative. Six keys existed in the real trees and in no
// draft of the spec, and `color` alone would have dropped off every session in silence.
func TestAnUnknownKeyIsRefused(t *testing.T) {
	fsys := with(goodTree(), "movements/half_crimp_hang.yaml", `
name: "Half Crimp Hang"
slug: "half_crimp_hang"
kind: "timed_reps"
tags: ["fingers"]
set_rest_second: 45
`)
	_, err := Load(fsys, "shipped")
	if err == nil {
		t.Fatal("a misspelled key was accepted")
	}
	if !strings.Contains(err.Error(), "set_rest_second") {
		t.Errorf("the error does not name the bad key: %v", err)
	}
	if !strings.Contains(err.Error(), "half_crimp_hang.yaml") {
		t.Errorf("the error does not name the file: %v", err)
	}
}

func TestAnUnknownTagIsRefused(t *testing.T) {
	fsys := with(goodTree(), "movements/general_warmup.yaml", `
name: "General Warm-up"
slug: "general_warmup"
kind: "open"
tags: ["warmup", "shouldres"]
`)
	_, err := Load(fsys, "shipped")
	if err == nil {
		t.Fatal("an unknown tag was accepted")
	}
	if !strings.Contains(err.Error(), "shouldres") {
		t.Errorf("the error does not name the bad tag: %v", err)
	}
}

// 46 of the 225 real files have a slug that differs from their filename. This is the check
// that turns those into deliberate renames instead of silent reinterpretation.
func TestASlugMustMatchItsFilename(t *testing.T) {
	fsys := with(goodTree(), "movements/general_warmup.yaml", `
name: "General Warm-up"
slug: "warmup_general"
kind: "open"
tags: ["warmup"]
`)
	_, err := Load(fsys, "shipped")
	if err == nil {
		t.Fatal("a slug that disagreed with its filename was accepted")
	}
	for _, want := range []string{"warmup_general", "general_warmup"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}
}

// A bare `ref:` names a target without naming its kind, which only works while a slug
// means one thing.
func TestASlugCannotBeUsedByTwoKinds(t *testing.T) {
	fsys := with(goodTree(), "menus/general_warmup.yaml", `
name: "Clash"
slug: "general_warmup"
pick: 1
options:
  - ref: "half_crimp_hang"
`)
	_, err := Load(fsys, "shipped")
	if err == nil {
		t.Fatal("one slug was accepted for both a movement and a menu")
	}
	if !strings.Contains(err.Error(), "general_warmup") {
		t.Errorf("the error does not name the slug: %v", err)
	}
}

func TestADanglingRefIsRefused(t *testing.T) {
	fsys := with(goodTree(), "blocks/warm_up.yaml", `
name: "Warm-up"
slug: "warm_up"
tags: ["warmup"]
items:
  - ref: "no_such_movement"
`)
	_, err := Load(fsys, "shipped")
	if err == nil {
		t.Fatal("a ref to nothing was accepted")
	}
	if !strings.Contains(err.Error(), "no_such_movement") {
		t.Errorf("the error does not name the missing ref: %v", err)
	}
}

// The same rule as ck_item_pair in the schema. Catching it here names the file; letting the
// database catch it gives a constraint violation instead.
func TestTheParentChildRulesAreEnforced(t *testing.T) {
	for _, tc := range []struct{ name, path, body, wantIn string }{
		{
			name: "a session cannot hold a movement",
			path: "sessions/finger_day.yaml",
			body: `
name: "Finger Day"
slug: "finger_day"
color: "#ef4444"
tags: ["fingers"]
items:
  - ref: "half_crimp_hang"
`,
			wantIn: "may hold only block",
		},
		{
			name: "a session cannot hold a menu",
			path: "sessions/finger_day.yaml",
			body: `
name: "Finger Day"
slug: "finger_day"
color: "#ef4444"
tags: ["fingers"]
items:
  - ref: "hang_choice"
`,
			wantIn: "may hold only block",
		},
		{
			name: "a block cannot hold a block",
			path: "blocks/warm_up.yaml",
			body: `
name: "Warm-up"
slug: "warm_up"
tags: ["warmup"]
items:
  - ref: "fingers_block"
`,
			wantIn: "may hold only movement or menu",
		},
		{
			name: "a menu cannot hold a block",
			path: "menus/hang_choice.yaml",
			body: `
name: "Pick a hang"
slug: "hang_choice"
pick: 1
options:
  - ref: "fingers_block"
`,
			wantIn: "may hold only movement",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(with(goodTree(), tc.path, tc.body), "shipped")
			if err == nil {
				t.Fatalf("%s: accepted", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error was %q, want it to contain %q", err, tc.wantIn)
			}
		})
	}
}

// ux_item_edge is UNIQUE (parent_id, child_id), so one parent cannot hold one child twice.
// It is also why a ladder is one reference with per_set and not three references.
func TestOneListCannotHoldTheSameRefTwice(t *testing.T) {
	fsys := with(goodTree(), "menus/hang_choice.yaml", `
name: "Pick a hang"
slug: "hang_choice"
pick: 1
options:
  - ref: "half_crimp_hang"
  - ref: "half_crimp_hang"
`)
	_, err := Load(fsys, "shipped")
	if err == nil {
		t.Fatal("the same ref twice in one list was accepted")
	}
	if !strings.Contains(err.Error(), "twice") {
		t.Errorf("unhelpful error: %v", err)
	}
}

func TestMenuPickIsChecked(t *testing.T) {
	for _, tc := range []struct{ name, body, wantIn string }{
		{"missing", `
name: "Pick a hang"
slug: "hang_choice"
options:
  - ref: "half_crimp_hang"
`, "no pick"},
		{"negative", `
name: "Pick a hang"
slug: "hang_choice"
pick: -1
options:
  - ref: "half_crimp_hang"
`, "cannot be negative"},
		{"more than there are options", `
name: "Pick a hang"
slug: "hang_choice"
pick: 5
options:
  - ref: "half_crimp_hang"
`, "only 1 options"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(with(goodTree(), "menus/hang_choice.yaml", tc.body), "shipped")
			if err == nil {
				t.Fatalf("pick %s was accepted", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error was %q, want it to contain %q", err, tc.wantIn)
			}
		})
	}
}

// pick: 0 is legal and means the menu may be skipped. Four real menus put that in their
// display name as "(optional)", which the app cannot act on.
func TestPickZeroMeansSkippable(t *testing.T) {
	fsys := with(goodTree(), "menus/hang_choice.yaml", `
name: "Pick a hang"
slug: "hang_choice"
pick: 0
options:
  - ref: "half_crimp_hang"
`)
	if _, err := Load(fsys, "shipped"); err != nil {
		t.Errorf("pick: 0 was refused: %v", err)
	}
}

func TestALadderNeedsSetsAndNotReps(t *testing.T) {
	for _, tc := range []struct{ name, body, wantIn string }{
		{"reps alongside per_set", `
name: "Hangboard Ladder"
slug: "hangboard_ladder"
kind: "timed_reps"
tags: ["fingers"]
sets: 3
reps: 3
per_set:
  - seconds: 3
  - seconds: 6
`, "twice"},
		{"per_set with no sets", `
name: "Hangboard Ladder"
slug: "hangboard_ladder"
kind: "timed_reps"
tags: ["fingers"]
per_set:
  - seconds: 3
  - seconds: 6
`, "needs sets"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(with(goodTree(), "movements/hangboard_ladder.yaml", tc.body), "shipped")
			if err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error was %q, want it to contain %q", err, tc.wantIn)
			}
		})
	}
}

func TestMovementKindIsChecked(t *testing.T) {
	// "duration" was the earlier name for "open" and must not quietly work.
	for _, kind := range []string{"duration", "session", ""} {
		fsys := with(goodTree(), "movements/general_warmup.yaml", `
name: "General Warm-up"
slug: "general_warmup"
kind: "`+kind+`"
tags: ["warmup"]
`)
		if _, err := Load(fsys, "shipped"); err == nil {
			t.Errorf("kind %q was accepted", kind)
		}
	}
}

func TestABlockRoleIsChecked(t *testing.T) {
	fsys := with(goodTree(), "blocks/warm_up.yaml", `
name: "Warm-up"
slug: "warm_up"
tags: ["warmup"]
role: "cooldwn"
items:
  - ref: "general_warmup"
`)
	_, err := Load(fsys, "shipped")
	if err == nil {
		t.Fatal("a misspelled role was accepted")
	}
	if !strings.Contains(err.Error(), "cooldwn") {
		t.Errorf("the error does not name the bad role: %v", err)
	}
}

// A stale tree must fail loudly rather than import wrong. The version lives in one file per
// tree, not in all 274, so there is one thing to bump.
func TestAWrongFormatVersionIsRefused(t *testing.T) {
	fsys := with(goodTree(), "catalog.yaml", "format_version: 99\n")
	_, err := Load(fsys, "shipped")
	if err == nil {
		t.Fatal("a tree from the future was accepted")
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("the error does not name the version it found: %v", err)
	}
}

// A private tree carries no tags.yaml of its own: after the merges, every tag either real
// tree uses is in the shipped list. So a tree without one still has to parse.
func TestATreeWithoutItsOwnTagsStillLoads(t *testing.T) {
	fsys := goodTree()
	delete(fsys, "tags.yaml")
	tree, err := Load(fsys, "private")
	if err != nil {
		t.Fatalf("a tree with no tags.yaml failed: %v", err)
	}
	if len(tree.Tags) != 0 {
		t.Errorf("%d tags appeared from nowhere", len(tree.Tags))
	}
}

func TestAnEmptyListIsRefused(t *testing.T) {
	fsys := with(goodTree(), "blocks/warm_up.yaml", `
name: "Warm-up"
slug: "warm_up"
tags: ["warmup"]
items: []
`)
	if _, err := Load(fsys, "shipped"); err == nil {
		t.Fatal("a block with no items was accepted")
	}
}
