package store

// Writing an id into a file that has none. An id minted at import time would be lost when a
// tree is exported and restored into a fresh database, so it has to be in the file before
// the importer runs.
//
// The write is textual, one line inserted, never a re-marshal: a round trip through
// yaml.Marshal would lose the comments, the key order and the quoting.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// catalogDirs are the three directories a tree holds files in. A menu is written inside its
// block, so there is no menus/.
var catalogDirs = []string{"movements", "blocks", "sessions"}

// MintIDs writes an id into every file under dir that has none, and returns the paths it
// wrote. A file that already carries one is left alone. It does not validate the tree, and
// a file it cannot parse is reported rather than written to.
func MintIDs(dir string) ([]string, error) {
	var wrote []string
	for _, sub := range catalogDirs {
		names, err := yamlFilesIn(filepath.Join(dir, sub))
		if err != nil {
			return wrote, err
		}
		for _, name := range names {
			did, err := mintOne(name)
			if err != nil {
				return wrote, err
			}
			if did {
				wrote = append(wrote, name)
			}
		}
	}
	return wrote, nil
}

// MissingIDs names every file under dir that carries no id, without writing anything, for a
// CI check and for a read-only tree.
func MissingIDs(dir string) ([]string, error) {
	var missing []string
	for _, sub := range catalogDirs {
		names, err := yamlFilesIn(filepath.Join(dir, sub))
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			raw, err := os.ReadFile(name)
			if err != nil {
				return nil, err
			}
			has, err := hasID(raw)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			if !has {
				missing = append(missing, name)
			}
		}
	}
	return missing, nil
}

func yamlFilesIn(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// A tree need not hold all three directories.
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	return out, nil
}

func mintOne(name string) (bool, error) {
	raw, err := os.ReadFile(name)
	if err != nil {
		return false, err
	}
	has, err := hasID(raw)
	if err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}
	if has {
		return false, nil
	}

	info, err := os.Stat(name)
	if err != nil {
		return false, err
	}
	out := insertID(raw, uuid.NewString())
	if err := os.WriteFile(name, out, info.Mode().Perm()); err != nil {
		return false, err
	}
	return true, nil
}

// hasID decodes into a struct holding only id, so a file with a key this build does not
// know still gets one. The loader is what then refuses that key by name.
func hasID(raw []byte) (bool, error) {
	var probe struct {
		ID string `yaml:"id"`
	}
	if err := yaml.Unmarshal(raw, &probe); err != nil {
		return false, err
	}
	return probe.ID != "", nil
}

// insertID puts the id line above the first line that is not a comment or blank, so a
// file's header comment stays at the top where a person put it.
func insertID(raw []byte, id string) []byte {
	line := []byte("id: " + id + "\n")

	lines := bytes.SplitAfter(raw, []byte("\n"))
	at := 0
	for ; at < len(lines); at++ {
		t := strings.TrimSpace(string(lines[at]))
		if t != "" && !strings.HasPrefix(t, "#") {
			break
		}
	}

	var out bytes.Buffer
	for i, l := range lines {
		if i == at {
			out.Write(line)
		}
		out.Write(l)
	}
	// A file holding nothing but comments, or nothing at all, still gets its id.
	if at >= len(lines) {
		out.Write(line)
	}
	return out.Bytes()
}

// UnknownFamilies names every file whose family: line matches no id in the trees given.
// Nothing resolves a family to a row at import time, so this is the only place a hand-typed
// family is checked. Pass every tree that can hold the other end.
func UnknownFamilies(trees ...*Tree) []string {
	// Ids only. A family always names an id, so counting declared families as known would
	// let a file vouch for its own typo.
	known := map[string]bool{}
	for _, t := range trees {
		for _, f := range allFileIDs(t) {
			known[f.ID] = true
		}
	}

	var out []string
	for _, t := range trees {
		for where, f := range allFileIDsByPath(t) {
			if f.Family != "" && !known[f.Family] {
				out = append(out, fmt.Sprintf("%s: family %s matches no id in the trees given",
					where, f.Family))
			}
		}
	}
	sort.Strings(out)
	return out
}

func allFileIDs(t *Tree) []FileID {
	out := make([]FileID, 0, len(t.Movements)+len(t.Blocks)+len(t.Sessions))
	for _, v := range t.Movements {
		out = append(out, v.FileID)
	}
	for _, v := range t.Blocks {
		out = append(out, v.FileID)
	}
	for _, v := range t.Sessions {
		out = append(out, v.FileID)
	}
	return out
}

func allFileIDsByPath(t *Tree) map[string]FileID {
	out := map[string]FileID{}
	for _, v := range t.Movements {
		out["movements/"+v.Slug+".yaml"] = v.FileID
	}
	for _, v := range t.Blocks {
		out["blocks/"+v.Slug+".yaml"] = v.FileID
	}
	for _, v := range t.Sessions {
		out["sessions/"+v.Slug+".yaml"] = v.FileID
	}
	return out
}
