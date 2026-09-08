package store

// Reading a catalog tree off disk, and checking it before a single row is written.
// The format is specified in docs/CATALOG_FORMAT.md.
//
// Two rules shape this file:
//
//   - An unknown key is an error. Every struct below is decoded with KnownFields(true), so
//     the struct tags are the format: a key that is not on a struct cannot be imported. A
//     key left off by mistake is then a loud failure rather than a value silently dropped.
//   - A slug equals its filename, and both are kept. The slug is the row's identity, so it
//     cannot be derived; checking it against the filename stops the two drifting apart.
//
// Nothing here touches the database. Load returns a whole tree or an error naming the file
// and the problem, so a bad tree fails before the importer opens a transaction.

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// FormatVersion is the version this binary reads. A tree declares its own in catalog.yaml,
// and a mismatch is refused rather than guessed at.
const FormatVersion = 1

// The four movement kinds. They say how a movement is counted. "open" means it carries no
// numbers at all, so it is not called "duration": there is no duration to look for.
const (
	MovementClimbing    = "climbing"
	MovementOpen        = "open"
	MovementRepsAndSets = "reps_and_sets"
	MovementTimedReps   = "timed_reps"
)

var movementKinds = map[string]bool{
	MovementClimbing: true, MovementOpen: true,
	MovementRepsAndSets: true, MovementTimedReps: true,
}

var blockRoles = map[string]bool{"warmup": true, "main": true, "cooldown": true}

// Tree is one whole catalog tree, parsed and checked.
type Tree struct {
	// Name becomes content.source_tree on every row this tree writes. "shipped" for the
	// embedded tree; for a private tree, the name from configuration. Never a path: a path
	// differs between machines and the column has to be stable.
	Name string

	Tags      []TagDef
	Movements []MovementFile
	Menus     []MenuFile
	Blocks    []BlockFile
	Sessions  []SessionFile
}

type catalogMeta struct {
	FormatVersion int `yaml:"format_version"`
}

// TagDef is one entry in tags.yaml. The shipped tree holds the vocabulary. A private tree
// carries a tags.yaml only if it needs a tag the shipped list does not have.
type TagDef struct {
	Slug string `yaml:"slug"`
	Name string `yaml:"name"`
}

// Media is one video and its thumbnail, both optional. A list, because a movement can have
// more than one.
type Media struct {
	URL      string `yaml:"url"`
	ThumbURL string `yaml:"thumb_url"`
}

// SetEntry is one rep inside one set: a rung of a ladder. On a movement it is that
// movement's own shape. On a reference it is what one block asks for.
type SetEntry struct {
	Reps     *int     `yaml:"reps"`
	WeightKg *float64 `yaml:"weight_kg"`
	Seconds  *int     `yaml:"seconds"`
}

// Dose is every number a movement or a reference can carry. Shared so the two cannot drift:
// a reference overrides a movement's default, and an override the movement cannot express
// would be meaningless.
type Dose struct {
	Sets           *int     `yaml:"sets"`
	Reps           *int     `yaml:"reps"`
	WeightKg       *float64 `yaml:"weight_kg"`
	RepSeconds     *int     `yaml:"rep_seconds"`
	RepRestSeconds *int     `yaml:"rep_rest_seconds"`
	SetRestSeconds *int     `yaml:"set_rest_seconds"`
	PrepSeconds    *int     `yaml:"prep_seconds"`
	Seconds        *int     `yaml:"seconds"`

	// PerSet is a ladder: 3 seconds, then 6, then 9, inside one set.
	PerSet []SetEntry `yaml:"per_set"`
}

type MovementFile struct {
	Name    string   `yaml:"name"`
	Slug    string   `yaml:"slug"`
	Kind    string   `yaml:"kind"`
	Tags    []string `yaml:"tags"`
	Notes   string   `yaml:"notes"`
	Source  string   `yaml:"source"`
	PerSide bool     `yaml:"per_side"`
	Media   []Media  `yaml:"media"`
	Dose    `yaml:",inline"`
}

type MenuFile struct {
	Name  string   `yaml:"name"`
	Slug  string   `yaml:"slug"`
	Tags  []string `yaml:"tags"`
	Notes string   `yaml:"notes"`

	// Pick is the FEWEST options you must choose, not the most. 0 means the menu may be
	// skipped.
	Pick    *int   `yaml:"pick"`
	Options []Item `yaml:"options"`
}

type BlockFile struct {
	Name   string   `yaml:"name"`
	Slug   string   `yaml:"slug"`
	Tags   []string `yaml:"tags"`
	Notes  string   `yaml:"notes"`
	Source string   `yaml:"source"`

	// Role is warmup, main or cooldown. It belongs to the block, not to a session's use of
	// it, so a block cannot be a warm-up in one session and the main event in another.
	Role  string `yaml:"role"`
	Items []Item `yaml:"items"`
}

type SessionFile struct {
	Name   string   `yaml:"name"`
	Slug   string   `yaml:"slug"`
	Color  string   `yaml:"color"`
	Tags   []string `yaml:"tags"`
	Notes  string   `yaml:"notes"`
	Source string   `yaml:"source"`
	Needs  string   `yaml:"needs"`
	Items  []Item   `yaml:"items"`
}

// Item is one entry in an items or options list. Always a reference — the format has no
// inline children, so every row in the database comes from a file of its own.
type Item struct {
	Ref   string `yaml:"ref"`
	Notes string `yaml:"notes"`
	Dose  `yaml:",inline"`
}

// Load reads and checks a whole tree. It returns the first problem it finds, naming the
// file, because a tree that is half right is not worth importing.
//
// known is slug to kind for content that already exists outside this tree, so a file here
// can point at one there. A private tree does this a great deal: it leans on the shipped
// movements rather than carrying its own copies. Pass nil when loading the shipped tree,
// which by definition has nothing before it.
func Load(fsys fs.FS, name string, known map[string]string) (*Tree, error) {
	t := &Tree{Name: name}

	var meta catalogMeta
	if err := decodeFile(fsys, "catalog.yaml", &meta); err != nil {
		return nil, err
	}
	if meta.FormatVersion != FormatVersion {
		return nil, fmt.Errorf("catalog %q: catalog.yaml says format_version %d, this build reads %d",
			name, meta.FormatVersion, FormatVersion)
	}

	// tags.yaml is optional. Only the shipped tree carries one; a private tree validates
	// against the shipped vocabulary unless it adds a tag of its own.
	if err := decodeFile(fsys, "tags.yaml", &t.Tags); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	if err := loadDir(fsys, "movements", &t.Movements, func(v MovementFile) string { return v.Slug }); err != nil {
		return nil, err
	}
	if err := loadDir(fsys, "menus", &t.Menus, func(v MenuFile) string { return v.Slug }); err != nil {
		return nil, err
	}
	if err := loadDir(fsys, "blocks", &t.Blocks, func(v BlockFile) string { return v.Slug }); err != nil {
		return nil, err
	}
	if err := loadDir(fsys, "sessions", &t.Sessions, func(v SessionFile) string { return v.Slug }); err != nil {
		return nil, err
	}

	if err := t.validate(known); err != nil {
		return nil, fmt.Errorf("catalog %q: %w", name, err)
	}
	return t, nil
}

// Index is every slug this tree defines and its kind. Pass it to Load as the known set when
// loading a tree that comes after this one.
func (t *Tree) Index() map[string]string {
	out, _ := t.kindOf()
	return out
}

// decodeFile reads one YAML file with unknown keys refused.
func decodeFile(fsys fs.FS, name string, out any) error {
	f, err := fsys.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// loadDir reads every .yaml in one directory into a slice, in filename order so an import
// is repeatable. A missing directory is not an error: a private tree may hold sessions only.
//
// slugOf checks the slug against the filename here, where both are in hand. A file whose
// slug disagrees is refused, so renaming one becomes a deliberate act rather than a silent
// change of identity.
func loadDir[T any](fsys fs.FS, dir string, out *[]T, slugOf func(T) string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, n := range names {
		var v T
		p := path.Join(dir, n)
		if err := decodeFile(fsys, p, &v); err != nil {
			return err
		}
		stem := strings.TrimSuffix(n, ".yaml")
		switch slug := slugOf(v); slug {
		case "":
			return fmt.Errorf("%s: no slug. Add `slug: %q`", p, stem)
		case stem:
		default:
			return fmt.Errorf("%s: slug is %q but the filename says %q. Rename the file or fix the slug",
				p, slug, stem)
		}
		*out = append(*out, v)
	}
	return nil
}

// kindOf is every slug in the tree and what kind it is. It is what lets a bare `ref:` name
// a target without naming its kind, and what makes the parent/child rules checkable.
func (t *Tree) kindOf() (map[string]string, error) {
	out := make(map[string]string, len(t.Movements)+len(t.Menus)+len(t.Blocks)+len(t.Sessions))
	add := func(slug, kind string) error {
		if was, dup := out[slug]; dup {
			return fmt.Errorf("slug %q is used by both a %s and a %s. A slug is unique across all four kinds",
				slug, was, kind)
		}
		out[slug] = kind
		return nil
	}
	for _, v := range t.Movements {
		if err := add(v.Slug, KindMovement); err != nil {
			return nil, err
		}
	}
	for _, v := range t.Menus {
		if err := add(v.Slug, KindMenu); err != nil {
			return nil, err
		}
	}
	for _, v := range t.Blocks {
		if err := add(v.Slug, KindBlock); err != nil {
			return nil, err
		}
	}
	for _, v := range t.Sessions {
		if err := add(v.Slug, KindSession); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// validate runs every rule that needs more than one file to check.
func (t *Tree) validate(known map[string]string) error {
	kinds, err := t.kindOf()
	if err != nil {
		return err
	}
	// A slug in this tree shadows the same slug outside it, which is the same order the
	// importer resolves in: this tree first, then what came before.
	for slug, kind := range known {
		if _, ours := kinds[slug]; !ours {
			kinds[slug] = kind
		}
	}

	tags := make(map[string]bool, len(t.Tags))
	for _, tg := range t.Tags {
		if tg.Slug == "" || tg.Name == "" {
			return fmt.Errorf("tags.yaml: an entry is missing its slug or its name")
		}
		if tags[tg.Slug] {
			return fmt.Errorf("tags.yaml: %q is listed twice", tg.Slug)
		}
		tags[tg.Slug] = true
	}
	// A tree with no tags.yaml of its own validates against the shipped vocabulary, which
	// the importer supplies. With neither, tag checking is skipped rather than failing every
	// file, so a tree can be parsed on its own in a test.
	checkTags := func(where string, list []string) error {
		if len(tags) == 0 {
			return nil
		}
		for _, tg := range list {
			if !tags[tg] {
				return fmt.Errorf("%s: unknown tag %q. Add it to tags.yaml or fix the spelling", where, tg)
			}
		}
		return nil
	}

	for _, v := range t.Movements {
		where := "movements/" + v.Slug + ".yaml"
		if v.Name == "" {
			return fmt.Errorf("%s: no name", where)
		}
		if !movementKinds[v.Kind] {
			return fmt.Errorf("%s: kind is %q, want one of climbing, open, reps_and_sets, timed_reps",
				where, v.Kind)
		}
		if err := checkTags(where, v.Tags); err != nil {
			return err
		}
		if err := v.Dose.validate(where); err != nil {
			return err
		}
	}

	for _, v := range t.Menus {
		where := "menus/" + v.Slug + ".yaml"
		if v.Name == "" {
			return fmt.Errorf("%s: no name", where)
		}
		if v.Pick == nil {
			return fmt.Errorf("%s: no pick. Say how many options must be chosen, and 0 means the menu may be skipped", where)
		}
		if *v.Pick < 0 {
			return fmt.Errorf("%s: pick is %d, and it cannot be negative", where, *v.Pick)
		}
		if *v.Pick > len(v.Options) {
			return fmt.Errorf("%s: pick is %d but there are only %d options", where, *v.Pick, len(v.Options))
		}
		if err := checkTags(where, v.Tags); err != nil {
			return err
		}
		if err := checkItems(where, "options", v.Options, kinds, KindMovement); err != nil {
			return err
		}
	}

	for _, v := range t.Blocks {
		where := "blocks/" + v.Slug + ".yaml"
		if v.Name == "" {
			return fmt.Errorf("%s: no name", where)
		}
		if v.Role != "" && !blockRoles[v.Role] {
			return fmt.Errorf("%s: role is %q, want warmup, main or cooldown", where, v.Role)
		}
		if err := checkTags(where, v.Tags); err != nil {
			return err
		}
		if err := checkItems(where, "items", v.Items, kinds, KindMovement, KindMenu); err != nil {
			return err
		}
	}

	for _, v := range t.Sessions {
		where := "sessions/" + v.Slug + ".yaml"
		if v.Name == "" {
			return fmt.Errorf("%s: no name", where)
		}
		if err := checkTags(where, v.Tags); err != nil {
			return err
		}
		if err := checkItems(where, "items", v.Items, kinds, KindBlock); err != nil {
			return err
		}
	}
	return nil
}

// checkItems enforces the structural rule: a session holds blocks, a block holds movements
// or menus, a menu holds movements. Nothing else.
//
// ck_item_pair in the schema enforces the same rule, and the fixed chain is what makes a
// loop in the tree impossible. Checking here as well gives a message naming the file,
// instead of a constraint violation from the driver.
func checkItems(where, listKey string, items []Item, kinds map[string]string, want ...string) error {
	if len(items) == 0 {
		return fmt.Errorf("%s: %s is empty", where, listKey)
	}
	seen := make(map[string]bool, len(items))
	for i, it := range items {
		at := fmt.Sprintf("%s: %s[%d]", where, listKey, i)
		if it.Ref == "" {
			return fmt.Errorf("%s: no ref. Every entry names another file; the format has no inline children", at)
		}
		if seen[it.Ref] {
			return fmt.Errorf("%s: %q appears twice in one list, which the schema refuses (ux_item_edge)", at, it.Ref)
		}
		seen[it.Ref] = true

		got, ok := kinds[it.Ref]
		if !ok {
			return fmt.Errorf("%s: ref %q names nothing in this tree", at, it.Ref)
		}
		if !contains(want, got) {
			return fmt.Errorf("%s: ref %q is a %s, and this list may hold only %s",
				at, it.Ref, got, strings.Join(want, " or "))
		}
		if err := it.Dose.validate(at); err != nil {
			return err
		}
	}
	return nil
}

// validate covers the rule that ties the numbers together. A ladder's entries are reps
// inside one set, so `sets` counts sets and cannot also be the rung count.
func (d Dose) validate(where string) error {
	if len(d.PerSet) == 0 {
		return nil
	}
	if d.Reps != nil {
		return fmt.Errorf("%s: per_set and reps say the same thing twice. per_set already gives each rep", where)
	}
	if d.Sets == nil {
		return fmt.Errorf("%s: per_set needs sets, to say how many times the ladder is repeated", where)
	}
	return nil
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
