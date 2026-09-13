package store

// Reading a catalog tree off disk and checking it. The format is docs/CATALOG_FORMAT.md.
//
// Nothing here touches the database. The structs below ARE the format: they decode with
// KnownFields(true), so a key that is not on a struct cannot be imported.

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// FormatVersion is the version this binary reads. A tree declares its own in catalog.yaml,
// and a mismatch is refused rather than guessed at.
const FormatVersion = 2

// AppPrefix marks a reference that leaves the tree it is written in and names something the
// app ships.
const AppPrefix = "app:"

// The four movement styles: how a movement is counted, and which run screen it gets.
// "open" carries no numbers at all.
const (
	StyleClimbing    = "climbing"
	StyleOpen        = "open"
	StyleRepsAndSets = "reps_and_sets"
	StyleTimedReps   = "timed_reps"
)

// idPattern is the canonical RFC 4122 spelling, lower case. Ids are written by the app.
var idPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var movementStyles = map[string]bool{
	StyleClimbing: true, StyleOpen: true,
	StyleRepsAndSets: true, StyleTimedReps: true,
}

var blockRoles = map[string]bool{"warmup": true, "main": true, "cooldown": true}

// A slug is lower case, digits and underscores. Two slugs differing only by case or
// punctuation would be two rows nobody could tell apart.
var slugPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// Ref is one reference as a file writes it. Resolution looks up the whole triple, so a bare
// name and an app: name can never stand in for each other.
type Ref struct {
	Kind string
	Slug string
	App  bool
}

func (r Ref) String() string {
	if r.App {
		return r.Kind + " " + AppPrefix + r.Slug
	}
	return r.Kind + " " + r.Slug
}

// splitRef separates a written reference into its slug and its namespace.
func splitRef(kind, written string) Ref {
	if slug, ok := strings.CutPrefix(written, AppPrefix); ok {
		return Ref{Kind: kind, Slug: slug, App: true}
	}
	return Ref{Kind: kind, Slug: written}
}

// Tree is one whole catalog tree, parsed and checked.
type Tree struct {
	// Name becomes content.source_tree on every row this tree writes. "shipped" for the
	// embedded tree; for a private tree, the name from configuration. Never a path: a path
	// differs between machines and the column has to be stable.
	Name string

	Movements []MovementFile
	Blocks    []BlockFile
	Sessions  []SessionFile
}

type catalogMeta struct {
	FormatVersion int `yaml:"format_version"`
}

// Media is one video and its thumbnail, both optional. A list, because a movement can have
// more than one.
type Media struct {
	URL      string `yaml:"url,omitempty"`
	ThumbURL string `yaml:"thumb_url,omitempty"`
}

// SetEntry is one rep inside one set: a rung of a ladder. On a movement it is that
// movement's own shape. On a reference it is what one block asks for.
type SetEntry struct {
	Reps     *int     `yaml:"reps,omitempty"`
	WeightKg *float64 `yaml:"weight_kg,omitempty"`
	Seconds  *int     `yaml:"seconds,omitempty"`
}

// Dose is every number a movement or a reference can carry. Shared so a reference can
// override any default a movement sets.
type Dose struct {
	Sets           *int     `yaml:"sets,omitempty"`
	Reps           *int     `yaml:"reps,omitempty"`
	WeightKg       *float64 `yaml:"weight_kg,omitempty"`
	RepSeconds     *int     `yaml:"rep_seconds,omitempty"`
	RepRestSeconds *int     `yaml:"rep_rest_seconds,omitempty"`
	SetRestSeconds *int     `yaml:"set_rest_seconds,omitempty"`
	PrepSeconds    *int     `yaml:"prep_seconds,omitempty"`
	Seconds        *int     `yaml:"seconds,omitempty"`

	// PerSet is a ladder: 3 seconds, then 6, then 9, inside one set.
	PerSet []SetEntry `yaml:"per_set,omitempty"`
}

// FileID is the identity every file carries.
//
// ID is written for you, by the import or by `passion catalog lint --fix`. The importer
// matches on it, so renaming a file changes nothing.
//
// Family holds the id of the row a copy came from, and keeps one progression chart whole
// across the copy. A file with no family line is its own family.
type FileID struct {
	ID     string `yaml:"id"`
	Family string `yaml:"family,omitempty"`
}

// MovementFile is one file under movements/.
//
// Slug carries no YAML tag: it comes from the filename and is never written into the file.
type MovementFile struct {
	Slug string `yaml:"-"`

	FileID `yaml:",inline"`

	Name string `yaml:"name"`

	// Style is how the movement is performed. Named apart from the row's kind, which the
	// directory already gives.
	Style string `yaml:"style"`

	Tags    []string `yaml:"tags,omitempty"`
	Notes   string   `yaml:"notes,omitempty"`
	Source  string   `yaml:"source,omitempty"`
	PerSide bool     `yaml:"per_side,omitempty"`
	Media   []Media  `yaml:"media,omitempty"`
	Dose    `yaml:",inline"`
}

// BlockFile is one file under blocks/.
type BlockFile struct {
	Slug string `yaml:"-"`

	FileID `yaml:",inline"`

	Name   string   `yaml:"name"`
	Tags   []string `yaml:"tags,omitempty"`
	Notes  string   `yaml:"notes,omitempty"`
	Source string   `yaml:"source,omitempty"`

	// warmup, main or cooldown. A property of the block, not of a session's use of it.
	Role  string `yaml:"role,omitempty"`
	Items []Item `yaml:"items"`
}

// SessionFile is one file under sessions/.
type SessionFile struct {
	Slug string `yaml:"-"`

	FileID `yaml:",inline"`

	Name   string   `yaml:"name"`
	Color  string   `yaml:"color,omitempty"`
	Tags   []string `yaml:"tags,omitempty"`
	Notes  string   `yaml:"notes,omitempty"`
	Source string   `yaml:"source,omitempty"`
	Needs  string   `yaml:"needs,omitempty"`
	Items  []Item   `yaml:"items"`
}

// Item is one entry in an items list. Exactly one of Movement, Block or Menu is set.
// A menu has no file of its own, so it is written in place.
type Item struct {
	Movement string    `yaml:"movement,omitempty"`
	Block    string    `yaml:"block,omitempty"`
	Menu     *MenuBody `yaml:"menu,omitempty"`

	Notes string `yaml:"notes,omitempty"`
	Dose  `yaml:",inline"`
}

// MenuBody is a menu written inside its block.
type MenuBody struct {
	Name  string   `yaml:"name,omitempty"`
	Notes string   `yaml:"notes,omitempty"`
	Tags  []string `yaml:"tags,omitempty"`

	// Pick is the fewest options you must choose, not the most. 0 means skippable.
	Pick *int     `yaml:"pick"`
	Of   []Option `yaml:"of"`
}

// Option is one choice in a menu. Written as a bare slug, or as a mapping when it carries
// numbers of its own.
type Option struct {
	Movement string `yaml:"movement"`
	Notes    string `yaml:"notes,omitempty"`
	Dose     `yaml:",inline"`
}

// yaml.Node.Decode ignores the decoder's KnownFields setting, so a type with its own
// UnmarshalYAML has to refuse unknown keys itself.
var optionKeys = []string{
	"movement", "notes",
	"sets", "reps", "weight_kg", "rep_seconds", "rep_rest_seconds",
	"set_rest_seconds", "prep_seconds", "seconds", "per_set",
}

func (o *Option) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		o.Movement = n.Value
		return nil
	}
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: an option is either a slug or a mapping with movement:", n.Line)
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if k := n.Content[i].Value; !contains(optionKeys, k) {
			return fmt.Errorf("line %d: unknown key %q in an option", n.Content[i].Line, k)
		}
	}
	// A distinct type, so decoding does not call this method again.
	type plain Option
	var v plain
	if err := n.Decode(&v); err != nil {
		return err
	}
	*o = Option(v)
	return nil
}

// Load reads and checks a whole tree, returning the first problem it finds.
//
// known is every reference that resolves outside this tree: app: for rows the app ships,
// bare for rows the importing account already holds. Pass nil for the shipped tree.
func Load(fsys fs.FS, name string, known map[Ref]bool) (*Tree, error) {
	t := &Tree{Name: name}

	var meta catalogMeta
	if err := decodeFile(fsys, "catalog.yaml", &meta); err != nil {
		return nil, err
	}
	if meta.FormatVersion != FormatVersion {
		return nil, fmt.Errorf("catalog %q: catalog.yaml says format_version %d, this build reads %d",
			name, meta.FormatVersion, FormatVersion)
	}

	if err := loadDir(fsys, "movements", &t.Movements,
		func(v *MovementFile, slug string) { v.Slug = slug }); err != nil {
		return nil, err
	}
	if err := loadDir(fsys, "blocks", &t.Blocks,
		func(v *BlockFile, slug string) { v.Slug = slug }); err != nil {
		return nil, err
	}
	if err := loadDir(fsys, "sessions", &t.Sessions,
		func(v *SessionFile, slug string) { v.Slug = slug }); err != nil {
		return nil, err
	}

	// A menus/ directory means a tree written for the old format.
	if _, err := fs.Stat(fsys, "menus"); err == nil {
		return nil, fmt.Errorf("catalog %q: this tree has a menus/ directory. A menu is now "+
			"written inside the block that holds it, under `menu:`", name)
	}

	if err := t.validate(known); err != nil {
		return nil, fmt.Errorf("catalog %q: %w", name, err)
	}
	return t, nil
}

// Index is every reference this tree defines, spelled as a later tree would have to write
// it. app is true for the shipped tree, whose rows other trees name with the prefix.
func (t *Tree) Index(app bool) map[Ref]bool {
	out := map[Ref]bool{}
	for _, v := range t.Movements {
		out[Ref{Kind: KindMovement, Slug: v.Slug, App: app}] = true
	}
	for _, v := range t.Blocks {
		out[Ref{Kind: KindBlock, Slug: v.Slug, App: app}] = true
	}
	for _, v := range t.Sessions {
		out[Ref{Kind: KindSession, Slug: v.Slug, App: app}] = true
	}
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

// loadDir reads every .yaml in one directory, in filename order so an import is repeatable.
// A missing directory is not an error: a private tree may hold sessions only. setSlug is
// given the filename without its extension.
func loadDir[T any](fsys fs.FS, dir string, out *[]T, setSlug func(*T, string)) error {
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
		p := path.Join(dir, n)
		stem := strings.TrimSuffix(n, ".yaml")
		if !slugPattern.MatchString(stem) {
			return fmt.Errorf("%s: a filename must be lower case letters, digits and "+
				"underscores, because it is the row's name", p)
		}
		var v T
		if err := decodeFile(fsys, p, &v); err != nil {
			return err
		}
		setSlug(&v, stem)
		*out = append(*out, v)
	}
	return nil
}

// validate runs every rule that needs more than one file to check.
func (t *Tree) validate(known map[Ref]bool) error {
	// A bare reference and an app: reference are separate entries, so one never stands in
	// for the other.
	refs := t.Index(false)
	for r := range known {
		refs[r] = true
	}

	// A tag is whatever you write; the importer creates it on first use.
	checkTags := func(where string, list []string) error {
		for _, tg := range list {
			if !slugPattern.MatchString(tg) {
				return fmt.Errorf("%s: tag %q is not a slug: lower case, digits and "+
					"underscores only", where, tg)
			}
		}
		return nil
	}

	// Two files holding one id is the copy-and-forget mistake. Name both and never renumber:
	// only a person can say which file was meant to keep the id.
	ids := map[string]string{}
	claimID := func(where string, f FileID) error {
		if f.ID == "" {
			return fmt.Errorf("%s: no id. Run `passion catalog lint --fix` to write one", where)
		}
		if !idPattern.MatchString(f.ID) {
			return fmt.Errorf("%s: id %q is not a uuid", where, f.ID)
		}
		if f.Family != "" && !idPattern.MatchString(f.Family) {
			return fmt.Errorf("%s: family %q is not a uuid", where, f.Family)
		}
		if f.Family == f.ID {
			return fmt.Errorf("%s: family is the same as id. Leave family out; a file with no "+
				"family is its own family", where)
		}
		if was, dup := ids[f.ID]; dup {
			return fmt.Errorf("%s: id %s is also in %s. Copying a file means changing its id, "+
				"and setting family: to the old one", where, f.ID, was)
		}
		ids[f.ID] = where
		return nil
	}

	for _, v := range t.Movements {
		where := "movements/" + v.Slug + ".yaml"
		if v.Name == "" {
			return fmt.Errorf("%s: no name", where)
		}
		if !movementStyles[v.Style] {
			return fmt.Errorf("%s: style is %q, want one of climbing, open, reps_and_sets, timed_reps",
				where, v.Style)
		}
		if err := claimID(where, v.FileID); err != nil {
			return err
		}
		if err := checkTags(where, v.Tags); err != nil {
			return err
		}
		if err := v.Dose.validate(where); err != nil {
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
		if err := claimID(where, v.FileID); err != nil {
			return err
		}
		if err := checkTags(where, v.Tags); err != nil {
			return err
		}
		if err := checkItems(where, v.Items, refs, checkTags,
			KindMovement, KindMenu); err != nil {
			return err
		}
	}

	for _, v := range t.Sessions {
		where := "sessions/" + v.Slug + ".yaml"
		if v.Name == "" {
			return fmt.Errorf("%s: no name", where)
		}
		if err := claimID(where, v.FileID); err != nil {
			return err
		}
		if err := checkTags(where, v.Tags); err != nil {
			return err
		}
		if err := checkItems(where, v.Items, refs, checkTags, KindBlock); err != nil {
			return err
		}
	}
	return nil
}

// checkItems enforces the chain: a session holds blocks, a block holds movements or menus,
// a menu holds movements. ck_item_pair enforces the same rule; checking here names the file
// instead of failing with a constraint violation.
func checkItems(where string, items []Item, refs map[Ref]bool,
	checkTags func(string, []string) error, want ...string) error {

	if len(items) == 0 {
		return fmt.Errorf("%s: items is empty", where)
	}
	seen := map[Ref]bool{}
	for i, it := range items {
		at := fmt.Sprintf("%s: items[%d]", where, i)

		kind, written, err := it.what()
		if err != nil {
			return fmt.Errorf("%s: %w", at, err)
		}
		if !contains(want, kind) {
			return fmt.Errorf("%s: a %s, and this list may hold only %s",
				at, kind, strings.Join(want, " or "))
		}

		if kind == KindMenu {
			if err := checkMenu(at, *it.Menu, refs, checkTags); err != nil {
				return err
			}
			if err := it.Dose.empty(at); err != nil {
				return err
			}
			continue
		}

		r := splitRef(kind, written)
		if !slugPattern.MatchString(r.Slug) {
			return fmt.Errorf("%s: %q is not a slug", at, written)
		}
		if seen[r] {
			return fmt.Errorf("%s: %s appears twice in one list, which the schema refuses (ux_item_edge)",
				at, r)
		}
		seen[r] = true

		if !refs[r] {
			return fmt.Errorf("%s: %s", at, missing(r, refs))
		}
		if err := it.Dose.validate(at); err != nil {
			return err
		}
	}
	return nil
}

// what reports which of the three keys an item set, and refuses none or more than one.
func (it Item) what() (kind, written string, err error) {
	var set []string
	if it.Movement != "" {
		set = append(set, KindMovement)
		kind, written = KindMovement, it.Movement
	}
	if it.Block != "" {
		set = append(set, KindBlock)
		kind, written = KindBlock, it.Block
	}
	if it.Menu != nil {
		set = append(set, KindMenu)
		kind, written = KindMenu, ""
	}
	switch len(set) {
	case 1:
		return kind, written, nil
	case 0:
		return "", "", errors.New("no movement:, block: or menu:. Every entry names the kind it holds")
	default:
		return "", "", fmt.Errorf("both %s. An entry holds exactly one thing",
			strings.Join(set, " and "))
	}
}

// missing explains a reference that resolves to nothing. If the slug exists in the other
// namespace, the only thing wrong is the prefix, and the message says so.
func missing(r Ref, refs map[Ref]bool) string {
	other := Ref{Kind: r.Kind, Slug: r.Slug, App: !r.App}
	switch {
	case refs[other] && r.App:
		return fmt.Sprintf("no %s %q in the app's catalog. You have one — drop the %q prefix",
			r.Kind, r.Slug, AppPrefix)
	case refs[other]:
		return fmt.Sprintf("no %s %q in this tree. The app's catalog has one — write %q",
			r.Kind, r.Slug, AppPrefix+r.Slug)
	case r.App:
		return fmt.Sprintf("no %s %q in the app's catalog", r.Kind, r.Slug)
	default:
		return fmt.Sprintf("no %s %q in this tree", r.Kind, r.Slug)
	}
}

func checkMenu(at string, m MenuBody, refs map[Ref]bool,
	checkTags func(string, []string) error) error {

	if m.Pick == nil {
		return fmt.Errorf("%s: a menu needs pick. Say how many options must be chosen, "+
			"and 0 means the menu may be skipped", at)
	}
	if *m.Pick < 0 {
		return fmt.Errorf("%s: pick is %d, and it cannot be negative", at, *m.Pick)
	}
	if *m.Pick > len(m.Of) {
		return fmt.Errorf("%s: pick is %d but there are only %d options", at, *m.Pick, len(m.Of))
	}
	if len(m.Of) == 0 {
		return fmt.Errorf("%s: a menu with no options", at)
	}
	if err := checkTags(at, m.Tags); err != nil {
		return err
	}

	seen := map[Ref]bool{}
	for i, o := range m.Of {
		oat := fmt.Sprintf("%s: of[%d]", at, i)
		if o.Movement == "" {
			return fmt.Errorf("%s: no movement", oat)
		}
		r := splitRef(KindMovement, o.Movement)
		if !slugPattern.MatchString(r.Slug) {
			return fmt.Errorf("%s: %q is not a slug", oat, o.Movement)
		}
		if seen[r] {
			return fmt.Errorf("%s: %s appears twice in one menu, which the schema refuses (ux_item_edge)",
				oat, r)
		}
		seen[r] = true
		if !refs[r] {
			return fmt.Errorf("%s: %s", oat, missing(r, refs))
		}
		if err := o.Dose.validate(oat); err != nil {
			return err
		}
	}
	return nil
}

// A ladder's entries are reps inside one set, so `sets` counts sets and cannot also be the
// rung count.
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

// empty refuses a dose on a menu. A menu is a choice, so the numbers belong on its options.
func (d Dose) empty(where string) error {
	if d.Sets != nil || d.Reps != nil || d.WeightKg != nil || d.RepSeconds != nil ||
		d.RepRestSeconds != nil || d.SetRestSeconds != nil || d.PrepSeconds != nil ||
		d.Seconds != nil || len(d.PerSet) > 0 {
		return fmt.Errorf("%s: numbers on a menu itself. Put them on the option they apply to", where)
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
