package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"

	"passion/server/db"
)

// Tree is one catalog tree. Name is how errors refer to it: the location
// from the config, or "catalog" for the one the app ships.
type Tree struct {
	Name string
	FS   fs.FS
}

// Exercise is one file of movements/, checked and ready to load.
type Exercise struct {
	file
	Fields db.ExerciseFields
	Hash   string
}

// file names where an entry came from, so two can be told apart.
type file struct {
	Tree   string
	Path   string
	FileID string
	Slug   string
}

// movementFile is every key a movements/ file may hold. Decoding refuses any
// other, so a misspelt key fails the load and is not dropped.
type movementFile struct {
	ID     string   `yaml:"id"`
	Name   string   `yaml:"name"`
	Style  string   `yaml:"style"`
	Tags   []string `yaml:"tags"`
	Notes  *string  `yaml:"notes"`
	Source *string  `yaml:"source"`
	Media  []struct {
		URL      *string `yaml:"url"`
		ThumbURL *string `yaml:"thumb_url"`
	} `yaml:"media"`

	Sets           *int  `yaml:"sets"`
	Reps           *int  `yaml:"reps"`
	SetRestSeconds *int  `yaml:"set_rest_seconds"`
	RepSeconds     *int  `yaml:"rep_seconds"`
	RepRestSeconds *int  `yaml:"rep_rest_seconds"`
	PrepSeconds    *int  `yaml:"prep_seconds"`
	Seconds        *int  `yaml:"seconds"`
	PerSide        bool  `yaml:"per_side"`
	Ladder         []int `yaml:"ladder"`
}

// fileKeys names a Clean problem by the key the file spells it with.
var fileKeys = map[string]string{
	"kind":             "style",
	"duration_seconds": "seconds",
}

var slugPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// Read reads the movements/ files of every tree one owner loads. They are read
// together because the owner's rows share one set of slugs and file ids, so a
// clash between two trees is as fatal as one inside a tree. Every problem is
// reported, not just the first.
func Read(trees ...Tree) ([]Exercise, error) {
	var exercises []Exercise
	var files []file
	var problems []error

	for _, tree := range trees {
		entries, err := readDir(tree, "movements")
		if err != nil {
			problems = append(problems, err)
			continue
		}
		for _, rel := range entries {
			e, err := readFile(tree, rel)
			if err != nil {
				problems = append(problems, fmt.Errorf("%s: %s: %w", tree.Name, rel, err))
				continue
			}
			exercises = append(exercises, e)
			files = append(files, e.file)
		}
	}

	problems = append(problems, clashes(files)...)
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return exercises, nil
}

func readFile(tree Tree, rel string) (Exercise, error) {
	slug := slugOf(rel)

	var f movementFile
	if err := decodeFile(tree, rel, &f); err != nil {
		return Exercise{}, err
	}
	if err := checkID(f.ID); err != nil {
		return Exercise{}, err
	}
	id := strings.ToLower(f.ID)

	fields := db.ExerciseFields{
		Name:            f.Name,
		Kind:            f.Style,
		Notes:           f.Notes,
		Source:          f.Source,
		Tags:            f.Tags,
		Sets:            f.Sets,
		Reps:            f.Reps,
		SetRestSeconds:  f.SetRestSeconds,
		RepSeconds:      f.RepSeconds,
		RepRestSeconds:  f.RepRestSeconds,
		PrepSeconds:     f.PrepSeconds,
		DurationSeconds: f.Seconds,
		PerSide:         f.PerSide,
		Ladder:          f.Ladder,
	}
	for _, m := range f.Media {
		fields.Media = append(fields.Media, db.Media{URL: m.URL, ThumbURL: m.ThumbURL})
	}

	fields, bad := fields.Clean()
	if len(bad) > 0 {
		var msgs []string
		for column, msg := range bad {
			key := column
			if k, ok := fileKeys[column]; ok {
				key = k
			}
			msgs = append(msgs, key+" "+msg)
		}
		sort.Strings(msgs)
		return Exercise{}, errors.New(strings.Join(msgs, ", "))
	}

	return Exercise{
		file:   file{Tree: tree.Name, Path: rel, FileID: id, Slug: slug},
		Fields: fields,
		Hash:   Hash(slug, fields),
	}, nil
}

// clashes finds two files that would claim one row or one name.
func clashes(files []file) []error {
	var problems []error
	byID := map[string]file{}
	bySlug := map[string]file{}
	for _, f := range files {
		if first, ok := byID[f.FileID]; ok {
			problems = append(problems, fmt.Errorf("%s: %s and %s: %s: the same id. Delete the id: line in the copy and run make catalog-ids",
				first.Tree, first.Path, f.Tree, f.Path))
		} else {
			byID[f.FileID] = f
		}
		if first, ok := bySlug[f.Slug]; ok {
			problems = append(problems, fmt.Errorf("%s: %s and %s: %s: the same file name, so the same slug. Rename one",
				first.Tree, first.Path, f.Tree, f.Path))
		} else {
			bySlug[f.Slug] = f
		}
	}
	return problems
}

// Hash is what the loader stores in loaded_hash. It takes cleaned fields, so a
// row read back from the database hashes the same as the file that wrote it.
func Hash(slug string, fields any) string {
	body, _ := json.Marshal(struct {
		Slug   string
		Fields any
	}{slug, fields})
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
