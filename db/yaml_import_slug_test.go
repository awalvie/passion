package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The importer matches catalog rows by slug, not by display name. A rename used to read as
// delete-then-create: the old row was pruned and a new one built, taking its id with it.

func slugImportFixture(t *testing.T, exerciseYAML, templateYAML string) (*Store, YAMLImportOptions) {
	t.Helper()
	tmp := t.TempDir()
	store, err := NewSqlite(filepath.Join(tmp, "slugimport.db"))
	if err != nil {
		t.Fatal(err)
	}
	exDir := filepath.Join(tmp, "exercises")
	tplDir := filepath.Join(tmp, "templates")
	for _, d := range []string{exDir, tplDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(exDir, "e.yaml"), []byte(exerciseYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tplDir, "t.yaml"), []byte(templateYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	return store, YAMLImportOptions{
		OwnerID:             1,
		ExercisesDir:        []string{exDir},
		SessionTemplatesDir: []string{tplDir},
	}
}

// The ref names its target by slug. That is what lets a rename survive: a ref by display
// name breaks the moment the name changes, which is the whole problem slugs fix.
const oneTemplate = `
name: "Strength Day"
slug: "strength_day"
activities:
  - type: "activity"
    exercises:
      - ref: "weighted_pull_ups"
`

// The whole point. Rename an entry and its row survives, keeping its id, so everything
// pointing at it still resolves.
func TestRenamingAnEntryUpdatesTheRowInsteadOfReplacingIt(t *testing.T) {
	store, opts := slugImportFixture(t, `
name: "Weighted Pull-ups"
slug: "weighted_pull_ups"
kind: "reps_and_sets"
sets: 5
reps: 5
`, oneTemplate)

	if err := store.ImportYAML(opts); err != nil {
		t.Fatal(err)
	}
	var before LibraryExercise
	if err := store.DB.Where("slug = ?", "weighted_pull_ups").First(&before).Error; err != nil {
		t.Fatal(err)
	}

	// Rename it in the YAML, keeping the slug.
	dir := filepath.Dir(opts.ExercisesDir[0] + "/x")
	if err := os.WriteFile(filepath.Join(dir, "e.yaml"), []byte(`
name: "Weighted Pull-Ups (Renamed)"
slug: "weighted_pull_ups"
kind: "reps_and_sets"
sets: 6
reps: 4
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.ImportYAML(opts); err != nil {
		t.Fatal(err)
	}

	var after LibraryExercise
	if err := store.DB.Where("slug = ?", "weighted_pull_ups").First(&after).Error; err != nil {
		t.Fatalf("the row was deleted by the rename: %v", err)
	}
	if after.ID != before.ID {
		t.Errorf("the row was replaced: id %d became %d", before.ID, after.ID)
	}
	if after.Name != "Weighted Pull-Ups (Renamed)" {
		t.Errorf("the name did not update: %q", after.Name)
	}
	if after.Sets != 6 || after.Reps != 4 {
		t.Errorf("the numbers did not update: %dx%d", after.Sets, after.Reps)
	}
	var count int64
	store.DB.Model(&LibraryExercise{}).Where("owner_id = ?", 1).Count(&count)
	if count != 1 {
		t.Errorf("the rename left %d rows behind", count)
	}
}

// A missing slug is fatal. Deriving one from the name would reinstate the identity this
// replaces, and the first rename would then silently delete the row.
func TestImportRefusesAnEntryWithNoSlug(t *testing.T) {
	store, opts := slugImportFixture(t, `
name: "Weighted Pull-ups"
kind: "reps_and_sets"
`, oneTemplate)

	err := store.ImportYAML(opts)
	if err == nil {
		t.Fatal("the import should refuse an entry with no slug")
	}
	if !strings.Contains(err.Error(), "slug") {
		t.Errorf("the error should name the problem, got %q", err)
	}
	var n int64
	store.DB.Model(&LibraryExercise{}).Count(&n)
	if n != 0 {
		t.Errorf("a refused import wrote %d rows", n)
	}
}

// Two entries claiming one identity would overwrite each other on every boot, alternately.
func TestImportRefusesTwoEntriesSharingASlug(t *testing.T) {
	store, opts := slugImportFixture(t, `
exercises:
  - name: "Max Hangs"
    slug: "hangs"
    kind: "timed_reps"
  - name: "Density Hangs"
    slug: "hangs"
    kind: "timed_reps"
`, `
name: "Strength Day"
slug: "strength_day"
activities:
  - type: "activity"
    exercises:
      - name: "Something"
        sets: 1
`)

	err := store.ImportYAML(opts)
	if err == nil {
		t.Fatal("the import should refuse a duplicate slug")
	}
	if !strings.Contains(err.Error(), "hangs") {
		t.Errorf("the error should name the slug, got %q", err)
	}
}

// An exercise placed by ref: is a copy of a library entry and must say so, or metrics and
// cycle overrides fall back to matching on the display name.
func TestRefPlacedExercisesLinkBackToTheLibrary(t *testing.T) {
	store, opts := slugImportFixture(t, `
name: "Weighted Pull-ups"
slug: "weighted_pull_ups"
kind: "reps_and_sets"
sets: 5
reps: 5
`, oneTemplate)

	if err := store.ImportYAML(opts); err != nil {
		t.Fatal(err)
	}
	var lib LibraryExercise
	if err := store.DB.Where("slug = ?", "weighted_pull_ups").First(&lib).Error; err != nil {
		t.Fatal(err)
	}
	var placed Exercise
	if err := store.DB.Where("name = ? AND activity_id IS NOT NULL", "Weighted Pull-ups").
		First(&placed).Error; err != nil {
		t.Fatal(err)
	}
	if placed.LibraryExerciseID == nil {
		t.Fatal("a ref-placed exercise does not say which library entry it came from")
	}
	if *placed.LibraryExerciseID != lib.ID {
		t.Errorf("it points at %d, want %d", *placed.LibraryExerciseID, lib.ID)
	}
}

// Prune keeps by slug now. An entry that leaves the YAML still goes; one that is only
// renamed does not.
func TestPruneRemovesOnlyWhatLeftTheYAML(t *testing.T) {
	store, opts := slugImportFixture(t, `
exercises:
  - name: "Max Hangs"
    slug: "max_hangs"
    kind: "timed_reps"
  - name: "Going Away"
    slug: "going_away"
    kind: "timed_reps"
`, `
name: "Strength Day"
slug: "strength_day"
activities:
  - type: "activity"
    exercises:
      - ref: "max_hangs"
`)
	if err := store.ImportYAML(opts); err != nil {
		t.Fatal(err)
	}
	var n int64
	store.DB.Model(&LibraryExercise{}).Count(&n)
	if n != 2 {
		t.Fatalf("expected 2 exercises imported, got %d", n)
	}

	// Drop one entry and rename the other.
	if err := os.WriteFile(filepath.Join(opts.ExercisesDir[0], "e.yaml"), []byte(`
name: "Max Hangs (Renamed)"
slug: "max_hangs"
kind: "timed_reps"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.ImportYAML(opts); err != nil {
		t.Fatal(err)
	}

	var left []LibraryExercise
	if err := store.DB.Order("slug").Find(&left).Error; err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 {
		t.Fatalf("want 1 exercise left, got %d", len(left))
	}
	if left[0].Slug != "max_hangs" || left[0].Name != "Max Hangs (Renamed)" {
		t.Errorf("the wrong row survived: %+v", left[0])
	}
}
