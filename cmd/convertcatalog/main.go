// Command convertcatalog rewrites a catalog tree from the old format to the one specified
// in docs/CATALOG_FORMAT.md.
//
// It reads a tree and writes a new one somewhere else. It never edits in place, so the
// result can be read and compared before anything is replaced.
//
// The old format keeps some content inside the file that uses it. The new one gives every
// row a file of its own, so this invents a filename for each of those. A name is turned
// into a slug, and a name that collides is qualified with its parent's slug.
//
//	go run ./cmd/convertcatalog -in catalog -out /tmp/new-catalog
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Tag decisions made when the vocabulary was cleaned up. This tool runs once and applies
// them to every file.
var (
	tagMerge = map[string]string{
		"shoulders":  "shoulder",
		"stretching": "mobility",
		"lead":       "route",
		"prehab":     "antagonist",
	}
	tagDrop = map[string]bool{"squat": true, "hinge": true, "pressing": true, "rest": true}
)

// A coach subfolder becomes a source on each file inside it. Every value here is already a
// source somewhere in the tree, so nothing is newly attributed to anybody.
//
// bechtel is absent on purpose. Those files carry no source and are generic barbell and
// bodyweight lifts in our own words. The folder is named after a paid programme, so adding
// one would newly claim that provenance in a published tree.
var folderSource = map[string]string{
	"ondra":  "Adam Ondra",
	"emil":   "Emil Abrahamsson",
	"nelson": "Tyler Nelson",
}

// Names for the blocks that have none. The old format allows an unnamed group; the new one
// does not, because every row is a file and a file needs a name.
// blockNames names a group the old format left unnamed, keyed "<parent slug>:<position>".
//
// Only the public tree's groups are listed here. A private tree passes its own file with
// -names, because the names describe a licensed programme's session structure and that is
// not ours to publish in this repository.
var blockNames = map[string]string{
	"boulder_session:2":                         "Bouldering",
	"emils_sub_max_daily_fingerboard_routine:0": "Hangs",
}

// loadBlockNames merges a private tree's names over the public ones.
func loadBlockNames(path string) error {
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	extra := map[string]string{}
	if err := yaml.Unmarshal(raw, &extra); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for k, v := range extra {
		blockNames[k] = v
	}
	return nil
}

// optionalSuffix finds a display name that ends in "(optional)". The old format had no
// field for a skippable group, so the fact was written into the name. A menu's pick count
// of 0 carries it now, and the suffix comes off before the name becomes a slug.
var optionalSuffix = regexp.MustCompile(`(?i)\s*\(optional\)\s*$`)

// perSideNotes finds a movement whose numbers are per side but say so only in prose.
// The word boundaries matter: without them "upper arms" contains "per arm".
var perSideNotes = regexp.MustCompile(`(?i)\b(per|each) (side|leg|arm|hand)\b`)

// notPerSide lists movements whose notes match perSideNotes for a reason that is not the
// dose. Tailor's Pose loads both knees at once and says "per side" about the plate weight.
// Warm-up Pulls counts each hand as one of its own sets, so the sets already cover both.
var notPerSide = map[string]bool{
	"tailors_pose": true,
	"warmup_pulls": true,
}

type out struct {
	files map[string][]byte

	// slugs is "kind/slug" -> the file that defined it. Keyed by kind because a name only
	// has to be unique within its kind now.
	slugs map[string]string

	// app is "kind/slug" for everything the public tree holds. A reference that resolves
	// there rather than here is written with the app: prefix.
	app      map[string]bool
	report   []string
	perSide  []string
	invented []string
	renamed  []string
}

func main() {
	in := flag.String("in", "catalog", "the tree to read")
	dst := flag.String("out", "", "the directory to write (must not exist)")
	only := flag.String("only", "", "convert one file only, by its slug")
	app := flag.String("app", "", "a converted public tree, so a reference into it gets the app: prefix")
	names := flag.String("names", "", "a YAML map of \"<parent slug>:<position>\" to a name, for groups the old format left unnamed")
	flag.Parse()

	if *dst == "" {
		fmt.Fprintln(os.Stderr, "convertcatalog: -out is required")
		os.Exit(1)
	}
	if err := loadBlockNames(*names); err != nil {
		fmt.Fprintln(os.Stderr, "convertcatalog:", err)
		os.Exit(1)
	}
	if err := run(*in, *dst, *only, *app); err != nil {
		fmt.Fprintln(os.Stderr, "convertcatalog:", err)
		os.Exit(1)
	}
}

func run(in, dst, only, appDir string) error {
	o := &out{files: map[string][]byte{}, slugs: map[string]string{}, app: map[string]bool{}}
	if appDir != "" {
		if err := o.loadApp(appDir); err != nil {
			return err
		}
	}

	o.files["catalog.yaml"] = []byte("format_version: 2\n")

	if err := o.movements(in, only); err != nil {
		return err
	}
	// Activities before sessions, so an inline menu's slug is settled before a session
	// refers to the block holding it.
	if err := o.activities(in, only); err != nil {
		return err
	}
	if err := o.sessions(in, only); err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	var paths []string
	for p := range o.files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		full := filepath.Join(dst, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, o.files[p], 0o644); err != nil {
			return err
		}
	}

	fmt.Printf("wrote %d files to %s\n", len(o.files), dst)
	if len(o.invented) > 0 {
		fmt.Printf("\ninvented %d filenames for content that had none:\n", len(o.invented))
		for _, s := range o.invented {
			fmt.Println("  " + s)
		}
	}
	if len(o.renamed) > 0 {
		fmt.Println("\ndropped \"(optional)\" from these names, because pick: 0 now says it:")
		for _, s := range o.renamed {
			fmt.Println("  " + s)
		}
	}
	if len(o.perSide) > 0 {
		fmt.Printf("\nset per_side: true on %d movements, from their notes. Check these:\n", len(o.perSide))
		for _, s := range o.perSide {
			fmt.Println("  " + s)
		}
	}
	for _, s := range o.report {
		fmt.Println(s)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Movements
// ---------------------------------------------------------------------------

func (o *out) movements(in, only string) error {
	dir := filepath.Join(in, "exercises")
	return filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || filepath.Ext(p) != ".yaml" {
			return err
		}
		var src map[string]any
		if err := readYAML(p, &src); err != nil {
			return err
		}
		slug := str(src["slug"])
		if slug == "" {
			return fmt.Errorf("%s: no slug", p)
		}
		if only != "" && slug != only {
			return nil
		}

		m := map[string]any{
			"name":  str(src["name"]),
			"style": movementStyle(str(src["kind"])),
		}
		if tags := convertTags(str(src["label"])); len(tags) > 0 {
			m["tags"] = tags
		}
		source := str(src["source"])
		if source == "" {
			if s, ok := folderSource[filepath.Base(filepath.Dir(p))]; ok {
				source = s
			}
		}
		if source != "" {
			m["source"] = source
		}

		notes := str(src["notes"])
		if perSideNotes.MatchString(notes) && !notPerSide[slug] {
			m["per_side"] = true
			o.perSide = append(o.perSide, slug)
		}

		for _, k := range []string{"sets", "reps", "weight_kg", "rep_seconds",
			"rep_rest_seconds", "set_rest_seconds", "prep_seconds"} {
			if v, ok := src[k]; ok {
				m[k] = v
			}
		}
		// The whole-step duration is renamed to match its column.
		if v, ok := src["session_duration_seconds"]; ok {
			m["seconds"] = v
		}
		// A ladder was a comma string. It becomes one entry per rung, and `reps` goes with
		// it because per_set already says how many there are.
		if rung := str(src["rung_seconds"]); rung != "" {
			var per []map[string]any
			for _, part := range strings.Split(rung, ",") {
				n, err := strconv.Atoi(strings.TrimSpace(part))
				if err != nil {
					return fmt.Errorf("%s: rung_seconds %q: %w", p, rung, err)
				}
				per = append(per, map[string]any{"seconds": n})
			}
			m["per_set"] = per
			delete(m, "reps")
		}

		if media := convertMedia(src["media"]); len(media) > 0 {
			m["media"] = media
		}
		if notes != "" {
			m["notes"] = notes
		}

		return o.add("movements/"+slug+".yaml", "movement", slug, m, p)
	})
}

// movementStyle maps the old kind to the new style. "session" meant "carries no numbers",
// which is what open says.
func movementStyle(old string) string {
	if old == "session" {
		return "open"
	}
	return old
}

// ---------------------------------------------------------------------------
// Activities become blocks, and the menus inside them become files
// ---------------------------------------------------------------------------

func (o *out) activities(in, only string) error {
	entries, err := os.ReadDir(filepath.Join(in, "activity_templates"))
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		p := filepath.Join(in, "activity_templates", e.Name())
		var src map[string]any
		if err := readYAML(p, &src); err != nil {
			return err
		}
		slug := str(src["slug"])
		if only != "" && slug != only {
			continue
		}

		if err := o.reserve("block", slug, "blocks/"+slug+".yaml", p); err != nil {
			return err
		}

		b := map[string]any{"name": str(src["name"])}
		if tags := convertTags(str(src["label"])); len(tags) > 0 {
			b["tags"] = tags
		}
		if s := str(src["source"]); s != "" {
			b["source"] = s
		}
		if n := str(src["notes"]); n != "" {
			b["notes"] = n
		}

		items, err := o.convertList(src["exercises"], slug, p)
		if err != nil {
			return err
		}
		b["items"] = items

		if err := o.add("blocks/"+slug+".yaml", "block", slug, b, p); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

func (o *out) sessions(in, only string) error {
	entries, err := os.ReadDir(filepath.Join(in, "session_templates"))
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		p := filepath.Join(in, "session_templates", e.Name())
		var src map[string]any
		if err := readYAML(p, &src); err != nil {
			return err
		}
		slug := str(src["slug"])
		if only != "" && slug != only {
			continue
		}

		if err := o.reserve("session", slug, "sessions/"+slug+".yaml", p); err != nil {
			return err
		}

		s := map[string]any{"name": str(src["name"])}
		if c := str(src["color"]); c != "" {
			s["color"] = c
		}
		if tags := convertTags(str(src["label"])); len(tags) > 0 {
			s["tags"] = tags
		}
		if v := str(src["source"]); v != "" {
			s["source"] = v
		}
		if v := str(src["needs"]); v != "" {
			s["needs"] = v
		}

		items, err := o.convertSessionList(src["activities"], slug, p)
		if err != nil {
			return err
		}
		s["items"] = items

		if err := o.add("sessions/"+slug+".yaml", "session", slug, s, p); err != nil {
			return err
		}
	}
	return nil
}

// convertSessionList handles a session's children. Each is either a reference to a block, or
// a group written out in place, which becomes a block file.
func (o *out) convertSessionList(raw any, parent, from string) ([]map[string]any, error) {
	list, _ := raw.([]any)
	var items []map[string]any

	for i, entry := range list {
		m, _ := entry.(map[string]any)
		if ref := str(m["ref"]); ref != "" {
			items = append(items, o.refItem(m, "block", ref))
			continue
		}

		// A group written out in place. Its role came from `type:` when that said warmup or
		// cooldown; `type: activity` said only "written out here", which needs no key.
		name := str(m["name"])
		role := ""
		switch t := str(m["type"]); t {
		case "warmup", "cooldown":
			role = t
		}
		if name == "" {
			key := fmt.Sprintf("%s:%d", parent, i)
			n, ok := blockNames[key]
			if !ok {
				return nil, fmt.Errorf("%s: the group at position %d has no name. "+
					"Add one to blockNames or to a -names file, keyed %q", from, i, key)
			}
			name = n
			o.invented = append(o.invented, fmt.Sprintf("blocks/%s.yaml  named %q, was an unnamed group in %s",
				slugify(name, parent, "block", o.slugs), name, filepath.Base(from)))
		}

		slug := slugify(name, parent, "block", o.slugs)
		b := map[string]any{"name": name}
		if role != "" {
			b["role"] = role
		}
		if n := str(m["notes"]); n != "" {
			b["notes"] = n
		}
		inner, err := o.convertList(m["exercises"], slug, from)
		if err != nil {
			return nil, err
		}
		b["items"] = inner
		if err := o.add("blocks/"+slug+".yaml", "block", slug, b, from); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"block": slug})
	}
	return items, nil
}

// convertList handles a block's children. Each is a reference to a movement, a menu written
// out in place, or a movement written out in place.
func (o *out) convertList(raw any, parent, from string) ([]map[string]any, error) {
	list, _ := raw.([]any)
	var items []map[string]any

	for _, entry := range list {
		m, _ := entry.(map[string]any)
		if ref := str(m["ref"]); ref != "" {
			items = append(items, o.refItem(m, "movement", ref))
			continue
		}

		name := str(m["name"])
		if name == "" {
			return nil, fmt.Errorf("%s: an entry inside %s has neither a ref nor a name", from, parent)
		}
		if trimmed := optionalSuffix.ReplaceAllString(name, ""); trimmed != name {
			o.renamed = append(o.renamed, fmt.Sprintf("%q -> %q in %s",
				name, trimmed, filepath.Base(from)))
			name = trimmed
		}

		// kind: exercise_catalog was a menu. A menu has no file now: it is written inside
		// the block that holds it, so it needs no name of its own.
		if str(m["kind"]) == "exercise_catalog" {
			menu := map[string]any{
				"name": name,
				"pick": pickFrom(str(m["notes"])),
			}
			if n := str(m["notes"]); n != "" {
				menu["notes"] = n
			}
			opts, err := o.convertList(m["children"], parent, from)
			if err != nil {
				return nil, err
			}
			menu["of"] = asOptions(opts)
			items = append(items, map[string]any{"menu": menu})
			continue
		}

		slug := slugify(name, parent, "movement", o.slugs)
		mv := map[string]any{
			"name":  name,
			"style": movementStyle(str(m["kind"])),
		}
		if n := str(m["notes"]); n != "" {
			mv["notes"] = n
		}
		for _, k := range []string{"sets", "reps", "weight_kg", "rep_seconds",
			"rep_rest_seconds", "set_rest_seconds", "prep_seconds"} {
			if v, ok := m[k]; ok {
				mv[k] = v
			}
		}
		if err := o.add("movements/"+slug+".yaml", "movement", slug, mv, from); err != nil {
			return nil, err
		}
		o.invented = append(o.invented, fmt.Sprintf("movements/%s.yaml  was written inside %s",
			slug, filepath.Base(from)))
		items = append(items, map[string]any{"movement": slug})
	}
	return items, nil
}

// asOptions turns a block-shaped item list into a menu's option list. An option carrying
// nothing but a movement becomes a bare slug, which is what keeps a five-option menu five
// readable lines instead of five mappings.
func asOptions(items []map[string]any) []any {
	out := make([]any, 0, len(items))
	for _, it := range items {
		if len(it) == 1 {
			if mv, ok := it["movement"].(string); ok {
				out = append(out, mv)
				continue
			}
		}
		out = append(out, it)
	}
	return out
}

// refItem keeps a reference's own numbers, which override the movement's defaults for that
// one use. A per-use `name:` is dropped: the name belongs to the row being referenced.
//
// The key names the kind, and the value carries the app: prefix when the reference leaves
// this tree.
func (o *out) refItem(m map[string]any, kind, ref string) map[string]any {
	it := map[string]any{kind: o.ref(kind, ref)}
	for _, k := range []string{"sets", "reps", "weight_kg", "rep_seconds",
		"rep_rest_seconds", "set_rest_seconds", "prep_seconds", "notes"} {
		if v, ok := m[k]; ok {
			it[k] = v
		}
	}
	return it
}

// pickFrom reads the arity out of a menu's prose. A menu that says nothing gets 1, and the
// report lists it so the guess can be checked.
func pickFrom(notes string) int {
	low := strings.ToLower(notes)
	switch {
	case strings.Contains(low, "one or more"):
		return 1
	case strings.Contains(low, "optional"):
		return 0
	default:
		return 1
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// reserve claims a name before its children are named, so a parent keeps the plain name and
// a child sharing it gets the qualified one.
func (o *out) reserve(kind, slug, path, from string) error {
	key := kind + "/" + slug
	if was, dup := o.slugs[key]; dup {
		return fmt.Errorf("%s: %s %q is already used by %s", from, kind, slug, was)
	}
	o.slugs[key] = path
	return nil
}

func (o *out) add(path, kind, slug string, body map[string]any, from string) error {
	key := kind + "/" + slug
	if was, dup := o.slugs[key]; dup && was != path {
		return fmt.Errorf("%s: %s %q is already used by %s", from, kind, slug, was)
	}
	o.slugs[key] = path

	b, err := marshalOrdered(body)
	if err != nil {
		return err
	}
	o.files[path] = b
	return nil
}

// ref spells a reference the way the new format needs it. A bare name means this tree; the
// app: prefix means the public catalog. There is no fallback between them, so the prefix is
// not optional where it applies.
func (o *out) ref(kind, slug string) string {
	key := kind + "/" + slug
	if _, mine := o.slugs[key]; !mine && o.app[key] {
		return "app:" + slug
	}
	return slug
}

// loadApp reads an already-converted public tree, so this run knows which references leave
// this tree. Only the filenames are read: the name is the identity.
func (o *out) loadApp(dir string) error {
	for _, d := range []string{"movements", "blocks", "sessions"} {
		entries, err := os.ReadDir(filepath.Join(dir, d))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		kind := strings.TrimSuffix(d, "s")
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
				continue
			}
			o.app[kind+"/"+strings.TrimSuffix(e.Name(), ".yaml")] = true
		}
	}
	return nil
}

// marshalOrdered writes the keys in a settled order, so a file reads the same way every
// time and a diff is about content rather than about key order.
var keyOrder = []string{
	"name", "style", "role", "color", "tags", "source", "needs", "aliases",
	"movement", "block", "menu", "pick",
	"per_side", "sets", "reps", "weight_kg", "rep_seconds", "rep_rest_seconds",
	"set_rest_seconds", "prep_seconds", "seconds", "per_set", "media", "notes",
	"items", "of",
	// Inside a media entry.
	"url", "thumb_url",
}

func marshalOrdered(m map[string]any) ([]byte, error) {
	node, err := orderedNode(m)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(node)
}

// orderedNode builds a YAML node with the keys in a settled order, so a file reads the same
// way every time and a diff is about content rather than key order.
//
// It recurses, because a menu is written inside its block and its keys need the same order.
func orderedNode(v any) (*yaml.Node, error) {
	switch t := v.(type) {
	case map[string]any:
		for k := range t {
			if !contains(keyOrder, k) {
				return nil, fmt.Errorf("key %q is not in keyOrder, so its position would be random", k)
			}
		}
		node := &yaml.Node{Kind: yaml.MappingNode}
		for _, k := range keyOrder {
			val, ok := t[k]
			if !ok {
				continue
			}
			vn, err := orderedNode(val)
			if err != nil {
				return nil, err
			}
			// Prose keeps its line breaks rather than becoming one long quoted string.
			if k == "notes" {
				vn.Style = yaml.LiteralStyle
			}
			node.Content = append(node.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: k}, vn)
		}
		return node, nil

	case []map[string]any:
		node := &yaml.Node{Kind: yaml.SequenceNode}
		for _, e := range t {
			vn, err := orderedNode(e)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, vn)
		}
		return node, nil

	case []any:
		node := &yaml.Node{Kind: yaml.SequenceNode}
		for _, e := range t {
			vn, err := orderedNode(e)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, vn)
		}
		return node, nil

	default:
		node := &yaml.Node{}
		if err := node.Encode(v); err != nil {
			return nil, err
		}
		return node, nil
	}
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}

func readYAML(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(b, out); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func str(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

// convertTags turns the comma string into a list, applies the merges and drops, and sorts
// so the same input always gives the same file.
func convertTags(label string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range strings.Split(label, ",") {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		if m, ok := tagMerge[t]; ok {
			t = m
		}
		if tagDrop[t] || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

func convertMedia(raw any) []map[string]any {
	list, _ := raw.([]any)
	var out []map[string]any
	for _, e := range list {
		m, _ := e.(map[string]any)
		entry := map[string]any{}
		if v := str(m["video_url"]); v != "" {
			entry["url"] = v
		}
		if v := str(m["thumbnail_url"]); v != "" {
			entry["thumb_url"] = v
		}
		if len(entry) > 0 {
			out = append(out, entry)
		}
	}
	return out
}

var notSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slugify turns a display name into a filename.
//
// A name that is already taken is qualified, and the order matters for readability. The kind
// comes first, because the usual collision is a block and the one menu inside it sharing a
// name: blocks/drills.yaml holding menus/drills_menu.yaml reads plainly. The parent's name
// comes next, which is what the tree already does by hand for its own duplicates.
func slugify(name, parent, kind string, taken map[string]string) string {
	base := strings.Trim(notSlug.ReplaceAllString(strings.ToLower(name), "_"), "_")
	if base == "" {
		base = parent
	}
	for _, try := range []string{base, base + "_" + kind, base + "_" + parent} {
		if _, dup := taken[try]; !dup {
			return try
		}
	}
	for i := 2; ; i++ {
		try := fmt.Sprintf("%s_%s_%d", base, parent, i)
		if _, dup := taken[try]; !dup {
			return try
		}
	}
}
