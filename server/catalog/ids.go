// Package catalog reads catalog trees: directories of YAML files, one exercise,
// block or session per file.
package catalog

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

var kindDirs = []string{"movements", "blocks", "sessions"}

var bom = []byte("\xef\xbb\xbf")

// AddIDs gives every file in the tree at root that has no id: line one of its
// own, and returns the paths it changed, relative to root. It never changes an
// id that is there, and changes no other byte of a file. A file it cannot read
// is reported and left alone, and the rest still get their ids.
func AddIDs(root string) ([]string, error) {
	var files []string
	found := false
	for _, dir := range kindDirs {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		found = true
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
				files = append(files, filepath.Join(dir, e.Name()))
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("%s has none of movements/, blocks/ or sessions/", root)
	}

	var changed []string
	var problems []error
	owner := map[string]string{}
	for _, rel := range files {
		id, added, err := addID(filepath.Join(root, rel))
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", rel, err))
			continue
		}
		if added {
			changed = append(changed, rel)
		}
		// A file copied to start a new one keeps the old id, and the loader
		// would take the two for one exercise.
		if first, ok := owner[id]; ok {
			problems = append(problems, fmt.Errorf("%s: its id is also in %s. Delete the id: line in the copy and run this again", rel, first))
			continue
		}
		owner[id] = rel
	}
	return changed, errors.Join(problems...)
}

// addID returns the file's id, and whether it had to add one.
func addID(path string) (string, bool, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}

	before, err := topLevel(body)
	if err != nil {
		return "", false, err
	}
	if id, ok := before["id"]; ok {
		if id == "" {
			return "", false, errors.New("id: holds no uuid. Delete the line and run this again")
		}
		return id, false, nil
	}

	id := newID()
	updated := withID(body, id)

	// The insert is plain text, so parse the result to be sure it reads as the
	// same file with one more key. A comment above a --- line, for one, would
	// leave the id in a document of its own.
	after, err := topLevel(updated)
	if err != nil {
		return "", false, fmt.Errorf("adding the id would break the file: %w", err)
	}
	if len(after) != len(before)+1 || after["id"] != id {
		return "", false, errors.New("adding the id would change what the file says. Add the id: line by hand")
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", false, err
	}
	return id, true, os.WriteFile(path, updated, info.Mode().Perm())
}

// topLevel maps each key of the file's top mapping to its value, which is
// empty for a null or for anything but a scalar.
func topLevel(body []byte) (map[string]string, error) {
	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(body)).Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("the file is empty")
		}
		return nil, err
	}
	top := doc.Content[0]
	if top.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: want a mapping of keys at the top of the file", top.Line)
	}
	keys := make(map[string]string, len(top.Content)/2)
	for i := 0; i < len(top.Content); i += 2 {
		value := top.Content[i+1]
		if value.Kind == yaml.ScalarNode && value.Tag != "!!null" {
			keys[top.Content[i].Value] = value.Value
		} else {
			keys[top.Content[i].Value] = ""
		}
	}
	return keys, nil
}

// withID puts the id line first, after a byte order mark or a --- line if the
// file starts with one, and ends it the way the file ends its lines.
func withID(body []byte, id string) []byte {
	at := 0
	if bytes.HasPrefix(body, bom) {
		at = len(bom)
	}
	if rest := body[at:]; bytes.HasPrefix(rest, []byte("---\n")) || bytes.HasPrefix(rest, []byte("---\r\n")) {
		at += bytes.IndexByte(rest, '\n') + 1
	}

	eol := "\n"
	if bytes.Contains(body, []byte("\r\n")) {
		eol = "\r\n"
	}

	out := make([]byte, 0, len(body)+len(id)+6)
	out = append(out, body[:at]...)
	out = append(out, "id: "+id+eol...)
	return append(out, body[at:]...)
}

// newID is a random version 4 uuid.
func newID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
