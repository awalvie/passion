package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"passion/server/db"
)

// Session is one file of sessions/, checked, with its blocks copied in as
// sections. Each step's Exercise holds the movement's file id until Load
// swaps in the row id. AppSteps says, in the order the steps come, which of
// them name a movement the app ships.
type Session struct {
	Tree     string
	Path     string
	FileID   string
	Slug     string
	Fields   db.SessionTemplateFields
	AppSteps []bool
}

type sessionFile struct {
	ID     string   `yaml:"id"`
	Name   string   `yaml:"name"`
	Notes  *string  `yaml:"notes"`
	Source *string  `yaml:"source"`
	Color  *string  `yaml:"color"`
	Needs  *string  `yaml:"needs"`
	Tags   []string `yaml:"tags"`
	Items  []struct {
		Block string `yaml:"block"`
	} `yaml:"items"`
}

type blockFile struct {
	ID    string      `yaml:"id"`
	Name  string      `yaml:"name"`
	Notes *string     `yaml:"notes"`
	Items []blockItem `yaml:"items"`

	// Read so that a tree using them loads. A section has nowhere to put them.
	Tags   []string `yaml:"tags"`
	Source *string  `yaml:"source"`
	Role   *string  `yaml:"role"`
}

// blockItem is a movement, with numbers that replace the movement's own for
// this use, or a menu, which carries no numbers of its own.
type blockItem struct {
	Movement *string   `yaml:"movement"`
	Menu     *menuFile `yaml:"menu"`

	Sets           *int `yaml:"sets"`
	Reps           *int `yaml:"reps"`
	SetRestSeconds *int `yaml:"set_rest_seconds"`
	RepSeconds     *int `yaml:"rep_seconds"`
	RepRestSeconds *int `yaml:"rep_rest_seconds"`
	PrepSeconds    *int `yaml:"prep_seconds"`
	Seconds        *int `yaml:"seconds"`
}

type menuFile struct {
	Name  string   `yaml:"name"`
	Notes *string  `yaml:"notes"`
	Pick  *int     `yaml:"pick"`
	Of    []string `yaml:"of"`
}

// block is a blocks/ file turned into a section. Its steps' exercises are
// file ids, and appSteps marks the ones the app ships.
type block struct {
	tree     string
	path     string
	section  db.Section
	appSteps []bool
}

// scope is the names one tree, or one owner's trees, can refer to.
type scope struct {
	movements map[string]Exercise
	blocks    map[string]blockFile
	paths     map[string]string
	tree      map[string]string
}

// ReadSessions reads the blocks/ and sessions/ files of every tree one owner
// loads. exercises is what Read returned for the same trees. app is the
// catalog the app ships, which an app: name refers to, and is nil when these
// trees are that catalog. Every problem is reported, not just the first.
func ReadSessions(app *Tree, exercises []Exercise, trees ...Tree) ([]Session, []string, error) {
	var problems []error
	var warnings []string

	own, ownProblems, ownWarnings := readScope(exercises, trees)
	problems = append(problems, ownProblems...)
	warnings = append(warnings, ownWarnings...)

	var shipped *scope
	if app != nil {
		appExercises, _, err := Read(*app)
		if err != nil {
			return nil, warnings, fmt.Errorf("the shipped catalog: %w", err)
		}
		s, appProblems, _ := readScope(appExercises, []Tree{*app})
		if len(appProblems) > 0 {
			return nil, warnings, fmt.Errorf("the shipped catalog: %w", errors.Join(appProblems...))
		}
		shipped = &s
	}

	r := resolver{own: own, app: shipped, blocks: map[string]blockResult{}}
	for _, slug := range sortedKeys(own.blocks) {
		if _, err := r.block(slug, false); err != nil {
			problems = append(problems, err)
		}
	}

	var sessions []Session
	for _, tree := range trees {
		entries, err := readDir(tree, "sessions")
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			problems = append(problems, err)
			continue
		}
		for _, rel := range entries {
			s, err := r.session(tree, rel)
			if err != nil {
				problems = append(problems, fmt.Errorf("%s: %s: %w", tree.Name, rel, err))
				continue
			}
			sessions = append(sessions, s)
		}
	}

	problems = append(problems, sessionClashes(sessions)...)
	if len(problems) > 0 {
		return nil, warnings, errors.Join(problems...)
	}
	return sessions, warnings, nil
}

// readScope indexes the movements and parses the blocks of trees.
func readScope(exercises []Exercise, trees []Tree) (scope, []error, []string) {
	s := scope{
		movements: map[string]Exercise{},
		blocks:    map[string]blockFile{},
		paths:     map[string]string{},
		tree:      map[string]string{},
	}
	for _, e := range exercises {
		s.movements[e.Slug] = e
	}

	var problems []error
	var warnings []string
	for _, tree := range trees {
		entries, err := readDir(tree, "blocks")
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			problems = append(problems, err)
			continue
		}
		for _, rel := range entries {
			slug := strings.TrimSuffix(path.Base(rel), ".yaml")
			if first, ok := s.paths[slug]; ok {
				problems = append(problems, fmt.Errorf("%s: %s and %s: %s: the same file name, so the same block. Rename one",
					s.tree[slug], first, tree.Name, rel))
				continue
			}
			var b blockFile
			if err := decodeFile(tree, rel, &b); err != nil {
				problems = append(problems, fmt.Errorf("%s: %s: %w", tree.Name, rel, err))
				continue
			}
			if b.Tags != nil || b.Source != nil || b.Role != nil {
				warnings = append(warnings, fmt.Sprintf("%s: %s: tags, source and role on a block are not stored", tree.Name, rel))
			}
			s.blocks[slug] = b
			s.paths[slug] = rel
			s.tree[slug] = tree.Name
		}
	}
	return s, problems, warnings
}

type resolver struct {
	own    scope
	app    *scope
	blocks map[string]blockResult
}

type blockResult struct {
	block block
	err   error
}

// lookup finds where a name points. A bare name means the scope of the file
// that holds it, so inApp is true inside a block the app ships. app: always
// means the catalog the app ships.
func (r resolver) lookup(ref string, inApp bool) (*scope, string, bool, error) {
	name, isApp := strings.CutPrefix(ref, "app:")
	switch {
	case isApp && r.app == nil:
		return nil, "", false, fmt.Errorf("%q: the shipped catalog names its own files without app:", ref)
	case isApp || inApp:
		return r.app, name, true, nil
	default:
		return &r.own, name, false, nil
	}
}

// hint says how to fix a bare name that this scope lacks but the app ships.
func (r resolver) hint(kind, ref string, inApp bool) string {
	if r.app == nil || inApp || strings.HasPrefix(ref, "app:") {
		return ""
	}
	_, inMovements := r.app.movements[ref]
	_, inBlocks := r.app.blocks[ref]
	if (kind == "movement" && inMovements) || (kind == "block" && inBlocks) {
		return fmt.Sprintf(". The app ships one: write %q", "app:"+ref)
	}
	return ""
}

func (r resolver) movement(ref string, inApp bool) (Exercise, bool, error) {
	s, name, isApp, err := r.lookup(ref, inApp)
	if err != nil {
		return Exercise{}, false, err
	}
	e, ok := s.movements[name]
	if !ok {
		return Exercise{}, false, fmt.Errorf("no movement %q%s", ref, r.hint("movement", ref, inApp))
	}
	return e, isApp, nil
}

// block turns a block into a section once, and reuses the result after that,
// so a broken block is reported once however many sessions use it.
func (r resolver) block(ref string, inApp bool) (block, error) {
	s, name, isApp, err := r.lookup(ref, inApp)
	if err != nil {
		return block{}, err
	}
	f, ok := s.blocks[name]
	if !ok {
		return block{}, fmt.Errorf("no block %q%s", ref, r.hint("block", ref, inApp))
	}

	key := name
	if isApp {
		key = "app:" + name
	}
	if done, ok := r.blocks[key]; ok {
		return done.block, done.err
	}
	b, err := r.section(s, name, f, isApp)
	r.blocks[key] = blockResult{b, err}
	return b, err
}

func (r resolver) section(s *scope, name string, f blockFile, isApp bool) (block, error) {
	b := block{tree: s.tree[name], path: s.paths[name]}
	var problems []string
	if len(f.Items) == 0 {
		problems = append(problems, "items: a block holds at least one movement or menu")
	}
	if err := checkID(f.ID); err != nil {
		problems = append(problems, err.Error())
	}

	var items []db.Item
	for i, item := range f.Items {
		at := fmt.Sprintf("items[%d]", i)
		switch {
		case item.Movement != nil && item.Menu != nil, item.Movement == nil && item.Menu == nil:
			problems = append(problems, at+": holds a movement or a menu, not both and not neither")
		case item.Movement != nil:
			e, app, err := r.movement(*item.Movement, isApp)
			if err != nil {
				problems = append(problems, at+": "+err.Error())
				continue
			}
			step := db.Step{Exercise: e.FileID, ExerciseFields: item.over(e.Fields)}
			items = append(items, db.Item{Step: &step})
			b.appSteps = append(b.appSteps, app)
		default:
			if item.hasNumbers() {
				problems = append(problems, at+": a menu carries no numbers. Put them on a movement")
				continue
			}
			if item.Menu.Pick == nil {
				problems = append(problems, at+".menu.pick: is required")
				continue
			}
			choice := db.Choice{Name: item.Menu.Name, Notes: item.Menu.Notes, Pick: *item.Menu.Pick}
			for j, ref := range item.Menu.Of {
				e, app, err := r.movement(ref, isApp)
				if err != nil {
					problems = append(problems, fmt.Sprintf("%s.menu.of[%d]: %s", at, j, err))
					continue
				}
				choice.Options = append(choice.Options, db.Step{Exercise: e.FileID, ExerciseFields: e.Fields})
				b.appSteps = append(b.appSteps, app)
			}
			items = append(items, db.Item{Choice: &choice})
		}
	}

	// Clean checks a whole body, so the block is checked as a body of one
	// section, and each problem is named by the block's own keys.
	cleaned, bad := db.SessionTemplateFields{
		Name: "block",
		Body: db.SessionBody{Sections: []db.Section{{Name: f.Name, Notes: f.Notes, Items: items}}},
	}.Clean()
	for key, msg := range bad {
		key = strings.TrimPrefix(key, "sections[0].")
		key = strings.ReplaceAll(key, ".choice.", ".menu.")
		key = strings.ReplaceAll(key, ".options[", ".of[")
		problems = append(problems, key+": "+msg)
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return block{}, &brokenBlock{fmt.Errorf("%s: %s: %s", b.tree, b.path, strings.Join(problems, ", "))}
	}

	b.section = cleaned.Body.Sections[0]
	return b, nil
}

// brokenBlock is a block file with problems of its own, as opposed to a name
// that points at no block.
type brokenBlock struct{ err error }

func (b *brokenBlock) Error() string { return b.err.Error() }

func (r resolver) session(tree Tree, rel string) (Session, error) {
	slug := strings.TrimSuffix(path.Base(rel), ".yaml")
	var f sessionFile
	if err := decodeFile(tree, rel, &f); err != nil {
		return Session{}, err
	}

	var problems []string
	if err := checkID(f.ID); err != nil {
		problems = append(problems, err.Error())
	}
	if len(f.Items) == 0 {
		problems = append(problems, "items: a session holds at least one block")
	}

	s := Session{Tree: tree.Name, Path: rel, FileID: strings.ToLower(f.ID), Slug: slug}
	var sections []db.Section
	for i, item := range f.Items {
		if item.Block == "" {
			problems = append(problems, fmt.Sprintf("items[%d]: names no block", i))
			continue
		}
		b, err := r.block(item.Block, false)
		var broken *brokenBlock
		switch {
		case errors.As(err, &broken) && !strings.HasPrefix(item.Block, "app:"):
			// Reported with the block's own file already.
			problems = append(problems, fmt.Sprintf("items[%d]: block %q has problems of its own", i, item.Block))
			continue
		case err != nil:
			problems = append(problems, fmt.Sprintf("items[%d]: %s", i, err))
			continue
		}
		sections = append(sections, b.section)
		s.AppSteps = append(s.AppSteps, b.appSteps...)
	}

	fields, bad := db.SessionTemplateFields{
		Name:   f.Name,
		Notes:  f.Notes,
		Source: f.Source,
		Color:  f.Color,
		Needs:  f.Needs,
		Tags:   f.Tags,
		Body:   db.SessionBody{Sections: sections},
	}.Clean()
	for key, msg := range bad {
		problems = append(problems, key+" "+msg)
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return Session{}, errors.New(strings.Join(problems, ", "))
	}
	s.Fields = fields
	return s, nil
}

func (item blockItem) hasNumbers() bool {
	return item.Sets != nil || item.Reps != nil || item.SetRestSeconds != nil || item.RepSeconds != nil ||
		item.RepRestSeconds != nil || item.PrepSeconds != nil || item.Seconds != nil
}

// over lays the item's numbers over a movement's own.
func (item blockItem) over(f db.ExerciseFields) db.ExerciseFields {
	for _, n := range []struct{ from, to **int }{
		{&item.Sets, &f.Sets},
		{&item.Reps, &f.Reps},
		{&item.SetRestSeconds, &f.SetRestSeconds},
		{&item.RepSeconds, &f.RepSeconds},
		{&item.RepRestSeconds, &f.RepRestSeconds},
		{&item.PrepSeconds, &f.PrepSeconds},
		{&item.Seconds, &f.DurationSeconds},
	} {
		if *n.from != nil {
			*n.to = *n.from
		}
	}
	return f
}

func checkID(id string) error {
	switch {
	case id == "":
		return errors.New("no id:. Run make catalog-ids")
	case !uuidPattern.MatchString(strings.ToLower(id)):
		return fmt.Errorf("id: %q is not a uuid", id)
	}
	return nil
}

// sessionClashes finds two session files that would claim one row or one name.
func sessionClashes(sessions []Session) []error {
	var problems []error
	byID := map[string]Session{}
	bySlug := map[string]Session{}
	for _, s := range sessions {
		if first, ok := byID[s.FileID]; ok {
			problems = append(problems, fmt.Errorf("%s: %s and %s: %s: the same id. Delete the id: line in the copy and run make catalog-ids",
				first.Tree, first.Path, s.Tree, s.Path))
		} else {
			byID[s.FileID] = s
		}
		if first, ok := bySlug[s.Slug]; ok {
			problems = append(problems, fmt.Errorf("%s: %s and %s: %s: the same file name, so the same slug. Rename one",
				first.Tree, first.Path, s.Tree, s.Path))
		} else {
			bySlug[s.Slug] = s
		}
	}
	return problems
}

// readDir lists the .yaml files of one directory of a tree, and refuses a
// subdirectory or a file name that is not a slug.
func readDir(tree Tree, dir string) ([]string, error) {
	entries, err := fs.ReadDir(tree.FS, dir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tree.Name, err)
	}
	var files []string
	for _, entry := range entries {
		rel := path.Join(dir, entry.Name())
		if entry.IsDir() {
			return nil, fmt.Errorf("%s: %s: a directory. Every file sits in %s/ itself", tree.Name, rel, dir)
		}
		if !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		if !slugPattern.MatchString(strings.TrimSuffix(entry.Name(), ".yaml")) {
			return nil, fmt.Errorf("%s: %s: a file name holds only lower case letters, digits and underscores", tree.Name, rel)
		}
		files = append(files, rel)
	}
	return files, nil
}

// decodeFile reads one YAML document into v, and refuses any key v lacks.
func decodeFile(tree Tree, rel string, v any) error {
	body, err := fs.ReadFile(tree.FS, rel)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(body))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("the file is empty")
		}
		return err
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("after the first document: %w", err)
		}
		return errors.New("more than one document. Delete the second --- and everything after it")
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
