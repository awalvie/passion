package store

import (
	"context"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/uuid"
)

// testID gives a file a stable id derived from its path. Fixtures then read as the format
// requires without a uuid on every one of them, and a test that cares can name the id it
// expects rather than digging it back out of the database.
func testID(p string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(p)).String()
}

// withIDs writes an id into every file in a fixture that has none. A fixture spells one out
// only when the test is about the id itself -- a copy, or two files claiming one.
func withIDs(fsys fstest.MapFS) fstest.MapFS {
	out := make(fstest.MapFS, len(fsys))
	for k, v := range fsys {
		switch d := path.Dir(k); {
		case d != "movements" && d != "blocks" && d != "sessions":
		default:
			if has, err := hasID(v.Data); err == nil && !has {
				out[k] = &fstest.MapFile{Data: insertID(v.Data, testID(k))}
				continue
			}
		}
		out[k] = v
	}
	return out
}

// A tree small enough to read in one screen and complete enough to load. Every test starts
// from this and breaks one thing, so what each test is about is the diff.
//
// Note what is not here: no slug: key anywhere, and no menus/ directory. A row's identity
// is its path, and a menu is written inside the block that holds it.
func goodTree() fstest.MapFS {
	return withIDs(fstest.MapFS{
		"catalog.yaml": &fstest.MapFile{Data: []byte("format_version: 2\n")},
		"movements/half_crimp_hang.yaml": &fstest.MapFile{Data: []byte(`
name: "Half Crimp Hang"
style: "timed_reps"
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
style: "timed_reps"
tags: ["fingers"]
sets: 3
per_set:
  - seconds: 3
  - seconds: 6
  - seconds: 9
`)},
		"movements/general_warmup.yaml": &fstest.MapFile{Data: []byte(`
name: "General Warm-up"
style: "open"
tags: ["warmup"]
`)},
		// Named "fingers" and holding a menu, which under the old format could not both
		// happen: the menu was a file and wanted the same name.
		"blocks/fingers.yaml": &fstest.MapFile{Data: []byte(`
name: "Fingers"
tags: ["fingers"]
items:
  - menu:
      name: "Pick a hang"
      pick: 1
      notes: |
        One of these per session.
      of: ["half_crimp_hang", "hangboard_ladder"]
`)},
		"blocks/warm_up.yaml": &fstest.MapFile{Data: []byte(`
name: "Warm-up"
tags: ["warmup"]
role: "warmup"
items:
  - movement: "general_warmup"
`)},
		"sessions/finger_day.yaml": &fstest.MapFile{Data: []byte(`
name: "Finger Day"
color: "#ef4444"
tags: ["fingers"]
needs: "hangboard"
items:
  - block: "warm_up"
  - block: "fingers"
`)},
	})
}

// with is a fixture edit: replace one file's body.
func with(fsys fstest.MapFS, path, body string) fstest.MapFS {
	out := fstest.MapFS{}
	for k, v := range fsys {
		out[k] = v
	}
	out[path] = &fstest.MapFile{Data: []byte(body)}
	return withIDs(out)
}

// without is a fixture edit: drop one file.
func without(fsys fstest.MapFS, path string) fstest.MapFS {
	out := fstest.MapFS{}
	for k, v := range fsys {
		if k != path {
			out[k] = v
		}
	}
	return out
}

func TestAGoodTreeLoads(t *testing.T) {
	tree, err := Load(goodTree(), "shipped", nil)
	if err != nil {
		t.Fatalf("a valid tree failed to load: %v", err)
	}
	if n := len(tree.Movements); n != 3 {
		t.Errorf("%d movements, want 3", n)
	}
	if n := len(tree.Blocks); n != 2 {
		t.Errorf("%d blocks, want 2", n)
	}
	if n := len(tree.Sessions); n != 1 {
		t.Errorf("%d sessions, want 1", n)
	}
	if tree.Name != "shipped" {
		t.Errorf("tree name is %q, want shipped", tree.Name)
	}

	// A menu is found through the block that holds it, and its slug is derived.
	menus := tree.Menus()
	if n := len(menus); n != 1 {
		t.Fatalf("%d menus, want 1", n)
	}
	if menus[0].Slug != "fingers_0" {
		t.Errorf("the menu's slug is %q, want fingers_0", menus[0].Slug)
	}
	if menus[0].Block != "fingers" {
		t.Errorf("the menu says its block is %q, want fingers", menus[0].Block)
	}
	if n := len(menus[0].Body.Of); n != 2 {
		t.Errorf("the menu has %d options, want 2", n)
	}

	// The slug comes from the filename, so it is set even though no file says it.
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

// A key the format does not list must fail, not be ignored. Otherwise a misspelling
// silently drops the value it was meant to set.
func TestAnUnknownKeyIsRefused(t *testing.T) {
	fsys := with(goodTree(), "movements/half_crimp_hang.yaml", `
name: "Half Crimp Hang"
style: "timed_reps"
tags: ["fingers"]
set_rest_second: 45
`)
	_, err := Load(fsys, "shipped", nil)
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

// A slug: key is the old format. Refusing it by the general rule is enough, and it names
// the key, which is what tells a person the format changed.
func TestASlugKeyIsRefused(t *testing.T) {
	fsys := with(goodTree(), "movements/general_warmup.yaml", `
name: "General Warm-up"
slug: "general_warmup"
style: "open"
`)
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("a slug: key was accepted")
	}
	if !strings.Contains(err.Error(), "slug") {
		t.Errorf("the error does not name the key: %v", err)
	}
}

// An option has its own UnmarshalYAML, and yaml.Node.Decode ignores the decoder's
// KnownFields setting. Without an explicit check the format would grow a hole exactly here.
func TestAnUnknownKeyInAMenuOptionIsRefused(t *testing.T) {
	fsys := with(goodTree(), "blocks/fingers.yaml", `
name: "Fingers"
items:
  - menu:
      name: "Pick a hang"
      pick: 1
      of:
        - movement: "half_crimp_hang"
          rep_second: 7
`)
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("a misspelled key inside a menu option was accepted")
	}
	if !strings.Contains(err.Error(), "rep_second") {
		t.Errorf("the error does not name the bad key: %v", err)
	}
}

// A tag is whatever you write, so there is no list to be missing from. It still has to look
// like a slug: "Warm Up" and "warm_up" would otherwise be two tags for one idea.
func TestATagMustBeASlug(t *testing.T) {
	fsys := with(goodTree(), "movements/general_warmup.yaml", `
name: "General Warm-up"
style: "open"
tags: ["warmup", "Warm Up"]
`)
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("a tag that is not a slug was accepted")
	}
	if !strings.Contains(err.Error(), "Warm Up") {
		t.Errorf("the error does not name the bad tag: %v", err)
	}
}

// A tag nothing has used before is created on first use. Without this a person cannot tag
// their own file without editing the app and rebuilding it.
func TestANewTagIsCreatedOnFirstUse(t *testing.T) {
	fsys := with(goodTree(), "movements/general_warmup.yaml", `
name: "General Warm-up"
style: "open"
tags: ["warmup", "hip_mobility"]
`)
	if _, err := Load(fsys, "shipped", nil); err != nil {
		t.Fatalf("a tag no list mentions was refused: %v", err)
	}
}

// A filename is an identity now, so it has to look like one. Two names differing only by
// case or punctuation would be two rows nobody could tell apart.
func TestAFilenameMustBeASlug(t *testing.T) {
	fsys := goodTree()
	fsys["movements/General Warmup.yaml"] = &fstest.MapFile{Data: []byte(`
name: "General Warm-up"
style: "open"
`)}
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("a filename that is not a slug was accepted")
	}
	if !strings.Contains(err.Error(), "General Warmup") {
		t.Errorf("the error does not name the file: %v", err)
	}
}

// The reverse of the old rule. A name only has to be unique within its kind, because every
// reference now names the kind it expects. The database already worked this way: there are
// two partial unique indexes and both include the kind.
func TestTwoKindsCanShareAName(t *testing.T) {
	fsys := with(goodTree(), "sessions/warm_up.yaml", `
name: "Warm-up Only"
items:
  - block: "warm_up"
`)
	tree, err := Load(fsys, "shipped", nil)
	if err != nil {
		t.Fatalf("a block and a session with one name were refused: %v", err)
	}
	if n := len(tree.Sessions); n != 2 {
		t.Errorf("%d sessions, want 2", n)
	}
}

func TestADanglingRefIsRefused(t *testing.T) {
	fsys := with(goodTree(), "blocks/warm_up.yaml", `
name: "Warm-up"
items:
  - movement: "no_such_movement"
`)
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("a reference to nothing was accepted")
	}
	if !strings.Contains(err.Error(), "no_such_movement") {
		t.Errorf("the error does not name the missing reference: %v", err)
	}
}

// The whole point of two namespaces: a bare name means this tree and nothing else. An
// earlier version fell back to the app's catalog, which meant adding a row could silently
// change what an existing reference meant.
func TestABareNameNeverReachesAppContent(t *testing.T) {
	app := map[Ref]bool{{Kind: KindMovement, Slug: "bench_press", App: true}: true}
	fsys := with(goodTree(), "blocks/warm_up.yaml", `
name: "Warm-up"
items:
  - movement: "bench_press"
`)
	_, err := Load(fsys, "private", app)
	if err == nil {
		t.Fatal("a bare name resolved to the app's catalog")
	}
	// And the message says exactly what to write instead, because the two namespaces make
	// that knowable.
	if !strings.Contains(err.Error(), "app:bench_press") {
		t.Errorf("the error does not say to write the prefix: %v", err)
	}
}

func TestAnAppRefResolvesToTheAppsCatalog(t *testing.T) {
	app := map[Ref]bool{{Kind: KindMovement, Slug: "bench_press", App: true}: true}
	fsys := with(goodTree(), "blocks/warm_up.yaml", `
name: "Warm-up"
items:
  - movement: "app:bench_press"
  - movement: "general_warmup"
`)
	if _, err := Load(fsys, "private", app); err != nil {
		t.Fatalf("an app: reference to something the app ships was refused: %v", err)
	}
}

// The other direction of the same message. A prefix on something only this tree has is
// just as wrong, and just as fixable.
func TestAnAppRefToYourOwnThingSaysToDropThePrefix(t *testing.T) {
	fsys := with(goodTree(), "blocks/warm_up.yaml", `
name: "Warm-up"
items:
  - movement: "app:general_warmup"
`)
	_, err := Load(fsys, "private", nil)
	if err == nil {
		t.Fatal("an app: reference to a local movement was accepted")
	}
	if !strings.Contains(err.Error(), "drop") {
		t.Errorf("the error does not say to drop the prefix: %v", err)
	}
}

// The same rule as ck_item_pair in the schema. Catching it here names the file. Letting the
// database catch it gives a driver's constraint violation instead.
func TestTheParentChildRulesAreEnforced(t *testing.T) {
	for _, tc := range []struct{ name, path, body, wantIn string }{
		{
			name: "a session cannot hold a movement",
			path: "sessions/finger_day.yaml",
			body: `
name: "Finger Day"
items:
  - movement: "general_warmup"
`,
			wantIn: "may hold only block",
		},
		{
			name: "a session cannot hold a menu",
			path: "sessions/finger_day.yaml",
			body: `
name: "Finger Day"
items:
  - menu:
      pick: 1
      of: ["general_warmup"]
`,
			wantIn: "may hold only block",
		},
		{
			name: "a block cannot hold a block",
			path: "blocks/warm_up.yaml",
			body: `
name: "Warm-up"
items:
  - block: "fingers"
`,
			wantIn: "may hold only movement or menu",
		},
		{
			name: "a block cannot hold a session",
			path: "blocks/warm_up.yaml",
			body: `
name: "Warm-up"
items:
  - session: "finger_day"
`,
			wantIn: "session",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(with(goodTree(), tc.path, tc.body), "shipped", nil)
			if err == nil {
				t.Fatal("an illegal parent and child pair was accepted")
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("the error does not say %q: %v", tc.wantIn, err)
			}
		})
	}
}

// Every item names exactly one thing. None is a file that forgot to say what it holds; two
// is a file that means two different trees.
func TestAnItemMustNameExactlyOneKind(t *testing.T) {
	for _, tc := range []struct{ name, body, wantIn string }{
		{
			name:   "none",
			body:   "name: \"Warm-up\"\nitems:\n  - notes: \"nothing here\"\n",
			wantIn: "names the kind",
		},
		{
			name:   "two",
			body:   "name: \"Warm-up\"\nitems:\n  - movement: \"general_warmup\"\n    block: \"fingers\"\n",
			wantIn: "exactly one",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(with(goodTree(), "blocks/warm_up.yaml", tc.body), "shipped", nil)
			if err == nil {
				t.Fatal("an item naming the wrong number of kinds was accepted")
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("the error does not say %q: %v", tc.wantIn, err)
			}
		})
	}
}

// ux_item_edge is UNIQUE (parent_id, child_id), so one parent cannot hold one child twice.
// It is also why a ladder is one reference with per_set rather than three references.
func TestOneListCannotHoldTheSameNameTwice(t *testing.T) {
	fsys := with(goodTree(), "blocks/warm_up.yaml", `
name: "Warm-up"
items:
  - movement: "general_warmup"
  - movement: "general_warmup"
    sets: 2
`)
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("one list held the same movement twice")
	}
	if !strings.Contains(err.Error(), "twice") {
		t.Errorf("the error does not say what is wrong: %v", err)
	}
}

func TestMenuPickIsChecked(t *testing.T) {
	for _, tc := range []struct{ name, body, wantIn string }{
		{
			name: "missing",
			body: `
name: "Fingers"
items:
  - menu:
      name: "Pick a hang"
      of: ["half_crimp_hang"]
`,
			wantIn: "needs pick",
		},
		{
			name: "negative",
			body: `
name: "Fingers"
items:
  - menu:
      name: "Pick a hang"
      pick: -1
      of: ["half_crimp_hang"]
`,
			wantIn: "cannot be negative",
		},
		{
			name: "more than there are options",
			body: `
name: "Fingers"
items:
  - menu:
      name: "Pick a hang"
      pick: 3
      of: ["half_crimp_hang", "hangboard_ladder"]
`,
			wantIn: "only 2 options",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(with(goodTree(), "blocks/fingers.yaml", tc.body), "shipped", nil)
			if err == nil {
				t.Fatal("a bad pick was accepted")
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("the error does not say %q: %v", tc.wantIn, err)
			}
		})
	}
}

// pick: 0 is the only place optionality can live. The alternative was the word "optional"
// inside a display name, which nothing could act on. It has to survive the database too,
// so this imports rather than only loading.
func TestPickZeroMeansSkippable(t *testing.T) {
	fsys := with(goodTree(), "blocks/fingers.yaml", `
name: "Fingers"
items:
  - menu:
      name: "Hangs"
      pick: 0
      of: ["half_crimp_hang", "hangboard_ladder"]
`)
	tree, err := Load(fsys, "shipped", nil)
	if err != nil {
		t.Fatalf("pick: 0 was refused: %v", err)
	}
	if got := tree.Menus()[0].Body.Pick; got == nil || *got != 0 {
		t.Fatalf("pick is %v, want 0", got)
	}

	eachEngine(t, func(t *testing.T, s *Store) {
		if _, err := s.ImportShipped(context.Background(), tree); err != nil {
			t.Fatalf("the database refused pick: 0: %v", err)
		}
	})
}

// A menu is a choice, not a thing you do. Numbers on the menu itself would have nothing to
// apply to, so they belong on the option.
func TestNumbersOnAMenuItselfAreRefused(t *testing.T) {
	fsys := with(goodTree(), "blocks/fingers.yaml", `
name: "Fingers"
items:
  - menu:
      name: "Hangs"
      pick: 1
      of: ["half_crimp_hang"]
    sets: 3
`)
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("numbers on a menu itself were accepted")
	}
	if !strings.Contains(err.Error(), "option") {
		t.Errorf("the error does not say where the numbers belong: %v", err)
	}
}

// An option is a bare slug when it carries nothing else, and a mapping when it carries
// numbers. Both spellings in one list, because that is what stops a whole menu reflowing
// when one option gains a number.
func TestAMenuOptionCanBeBareOrCarryNumbers(t *testing.T) {
	fsys := with(goodTree(), "blocks/fingers.yaml", `
name: "Fingers"
items:
  - menu:
      name: "Hangs"
      pick: 1
      of:
        - "half_crimp_hang"
        - movement: "hangboard_ladder"
          sets: 2
`)
	tree, err := Load(fsys, "shipped", nil)
	if err != nil {
		t.Fatalf("a mixed option list was refused: %v", err)
	}
	of := tree.Menus()[0].Body.Of
	if of[0].Movement != "half_crimp_hang" || of[0].Sets != nil {
		t.Errorf("the bare option parsed wrong: %+v", of[0])
	}
	if of[1].Movement != "hangboard_ladder" || of[1].Sets == nil || *of[1].Sets != 2 {
		t.Errorf("the mapping option parsed wrong: %+v", of[1])
	}
}

func TestALadderNeedsSetsAndNotReps(t *testing.T) {
	for _, tc := range []struct{ name, body, wantIn string }{
		{
			name: "reps as well",
			body: `
name: "Hangboard Ladder"
style: "timed_reps"
sets: 3
reps: 3
per_set:
  - seconds: 3
`,
			wantIn: "same thing twice",
		},
		{
			name: "no sets",
			body: `
name: "Hangboard Ladder"
style: "timed_reps"
per_set:
  - seconds: 3
`,
			wantIn: "needs sets",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(with(goodTree(), "movements/hangboard_ladder.yaml", tc.body), "shipped", nil)
			if err == nil {
				t.Fatal("a bad ladder was accepted")
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("the error does not say %q: %v", tc.wantIn, err)
			}
		})
	}
}

// style says how a movement is performed. It is named apart from kind, which the directory
// already gives, because two meanings under one name is a trap.
func TestMovementStyleIsChecked(t *testing.T) {
	fsys := with(goodTree(), "movements/general_warmup.yaml", `
name: "General Warm-up"
style: "duration"
`)
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("an unknown style was accepted")
	}
	if !strings.Contains(err.Error(), "duration") {
		t.Errorf("the error does not name the bad style: %v", err)
	}
}

func TestABlockRoleIsChecked(t *testing.T) {
	fsys := with(goodTree(), "blocks/warm_up.yaml", `
name: "Warm-up"
role: "prep"
items:
  - movement: "general_warmup"
`)
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("an unknown role was accepted")
	}
	if !strings.Contains(err.Error(), "prep") {
		t.Errorf("the error does not name the bad role: %v", err)
	}
}

func TestAWrongFormatVersionIsRefused(t *testing.T) {
	fsys := with(goodTree(), "catalog.yaml", "format_version: 1\n")
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("a tree written for another format version was accepted")
	}
	if !strings.Contains(err.Error(), "format_version") {
		t.Errorf("the error does not name the problem: %v", err)
	}
}

// A tree written for the old format has its menus in files. Saying so beats importing three
// quarters of it and leaving the rest on disk unread.
func TestAMenusDirectoryIsRefused(t *testing.T) {
	fsys := goodTree()
	fsys["menus/hang_choice.yaml"] = &fstest.MapFile{Data: []byte("name: \"Pick a hang\"\npick: 1\n")}
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("a menus/ directory was accepted")
	}
	if !strings.Contains(err.Error(), "menu:") {
		t.Errorf("the error does not say where a menu goes now: %v", err)
	}
}

func TestAnEmptyListIsRefused(t *testing.T) {
	fsys := with(goodTree(), "blocks/warm_up.yaml", "name: \"Warm-up\"\nitems: []\n")
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("a block with no items was accepted")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("the error does not say what is wrong: %v", err)
	}
}

// A file with no id cannot be imported, and the error says how to fix it. The id is the
// identity, so guessing one at import time would mint a different value on every machine.
func TestAFileWithNoIDIsRefused(t *testing.T) {
	fsys := goodTree()
	fsys["movements/half_crimp_hang.yaml"] = &fstest.MapFile{Data: []byte(`
name: "Half Crimp Hang"
style: "timed_reps"
`)}
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("a file with no id was accepted")
	}
	for _, want := range []string{"half_crimp_hang.yaml", "no id", "--fix"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}
}

// An id that is not a uuid is a hand-typed one, and hand-typing is the thing the format
// asks people not to do.
func TestAnIDThatIsNotAUUIDIsRefused(t *testing.T) {
	fsys := with(goodTree(), "movements/half_crimp_hang.yaml", `
id: half-crimp-1
name: "Half Crimp Hang"
style: "timed_reps"
`)
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("an id that is not a uuid was accepted")
	}
	if !strings.Contains(err.Error(), "not a uuid") {
		t.Errorf("the error does not say what is wrong: %v", err)
	}
}

// A family equal to the file's own id says nothing: a file with no family line already
// heads its own series. Writing it means somebody misunderstood, so it is refused rather
// than ignored.
func TestAFamilyEqualToItsOwnIDIsRefused(t *testing.T) {
	id := "11111111-2222-4333-8444-555555555555"
	fsys := with(goodTree(), "movements/half_crimp_hang.yaml", `
id: `+id+`
family: `+id+`
name: "Half Crimp Hang"
style: "timed_reps"
`)
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("a family equal to its own id was accepted")
	}
	if !strings.Contains(err.Error(), "family") {
		t.Errorf("the error does not name the key: %v", err)
	}
}

// aliases: was how a rename used to be declared. The id does that job now, and a file
// carrying the old key is refused rather than silently ignored.
func TestTheAliasesKeyIsGone(t *testing.T) {
	fsys := with(goodTree(), "movements/half_crimp_hang.yaml", `
name: "Half Crimp Hang"
style: "timed_reps"
aliases: ["old_hang"]
`)
	_, err := Load(fsys, "shipped", nil)
	if err == nil {
		t.Fatal("the aliases key was still accepted")
	}
	if !strings.Contains(err.Error(), "aliases") {
		t.Errorf("the error does not name the key: %v", err)
	}
}
