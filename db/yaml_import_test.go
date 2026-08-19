package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestImportYAMLUpsertByName(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	store, err := NewSqlite(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	exercisesDir := filepath.Join(tmp, "exercises")
	templatesDir := filepath.Join(tmp, "templates")
	if err := os.MkdirAll(exercisesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(exercisesDir, "pullups.yaml"), []byte(`
name: "Weighted Pull-ups"
kind: "reps_and_sets"
sets: 5
reps: 5
rep_seconds: 5
set_rest_seconds: 120
weight_kg: 10
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "strength.yaml"), []byte(`
name: "Strength Base Session"
color: "#ef4444"
activities:
  - type: "activity"
    exercises:
      - ref: "Weighted Pull-ups"
  - type: "cooldown"
    exercises:
      - name: "Lat + Pec Stretch"
        kind: "session"
        session_duration_seconds: 300
`), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := YAMLImportOptions{
		OwnerID:             1,
		ExercisesDir:        exercisesDir,
		SessionTemplatesDir: templatesDir,
	}
	if err := store.ImportYAML(opts); err != nil {
		t.Fatal(err)
	}

	var libCount int64
	if err := store.DB.Model(&LibraryExercise{}).Where("owner_id = ?", 1).Count(&libCount).Error; err != nil {
		t.Fatal(err)
	}
	if libCount != 1 {
		t.Fatalf("expected one library exercise, got %d", libCount)
	}
	var tplCount int64
	if err := store.DB.Model(&SessionTemplate{}).Where("owner_id = ?", 1).Count(&tplCount).Error; err != nil {
		t.Fatal(err)
	}
	if tplCount != 1 {
		t.Fatalf("expected one session template, got %d", tplCount)
	}

	// Re-import changed YAML: same names should update, not duplicate.
	if err := os.WriteFile(filepath.Join(exercisesDir, "pullups.yaml"), []byte(`
name: "Weighted Pull-ups"
kind: "reps_and_sets"
sets: 5
reps: 6
rep_seconds: 5
set_rest_seconds: 150
weight_kg: 12.5
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "strength.yaml"), []byte(`
name: "Strength Base Session"
color: "#3b82f6"
activities:
  - type: "activity"
    exercises:
      - ref: "Weighted Pull-ups"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := store.ImportYAML(opts); err != nil {
		t.Fatal(err)
	}

	if err := store.DB.Model(&LibraryExercise{}).Where("owner_id = ?", 1).Count(&libCount).Error; err != nil {
		t.Fatal(err)
	}
	if libCount != 1 {
		t.Fatalf("expected one library exercise after re-import, got %d", libCount)
	}
	if err := store.DB.Model(&SessionTemplate{}).Where("owner_id = ?", 1).Count(&tplCount).Error; err != nil {
		t.Fatal(err)
	}
	if tplCount != 1 {
		t.Fatalf("expected one session template after re-import, got %d", tplCount)
	}

	var lib LibraryExercise
	if err := store.DB.Where("owner_id = ? AND name = ?", 1, "Weighted Pull-ups").First(&lib).Error; err != nil {
		t.Fatal(err)
	}
	if lib.Reps != 6 || lib.SetRestSeconds != 150 || lib.WeightKg != 12.5 {
		t.Fatalf("expected updated library fields, got %+v", lib)
	}

	var tpl SessionTemplate
	if err := store.DB.Where("owner_id = ? AND name = ?", 1, "Strength Base Session").First(&tpl).Error; err != nil {
		t.Fatal(err)
	}
	if tpl.Color != "#3b82f6" {
		t.Fatalf("expected template color update, got %q", tpl.Color)
	}

	var activityCount int64
	if err := store.DB.Model(&Activity{}).Where("owner_id = ? AND session_template_id = ?", 1, tpl.ID).Count(&activityCount).Error; err != nil {
		t.Fatal(err)
	}
	if activityCount != 1 {
		t.Fatalf("expected replaced activities, got %d", activityCount)
	}
}

func TestImportYAMLUnknownReferenceFails(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	store, err := NewSqlite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	exercisesDir := filepath.Join(tmp, "exercises")
	templatesDir := filepath.Join(tmp, "templates")
	if err := os.MkdirAll(exercisesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "broken.yaml"), []byte(`
name: "Broken Session"
activities:
  - type: "activity"
    exercises:
      - ref: "Does Not Exist"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	err = store.ImportYAML(YAMLImportOptions{
		OwnerID:             1,
		ExercisesDir:        exercisesDir,
		SessionTemplatesDir: templatesDir,
	})
	if err == nil {
		t.Fatal("expected unknown reference import error")
	}
}

// TestImportYAMLPrunesRenameOrphansButProtectsInUseAndUIRows covers pruneCatalogOrphans,
// the cleanup pass at the end of ImportYAML that removes catalog-managed rows whose names
// disappeared from the YAML (e.g. an exercise or template that was renamed). It asserts the
// four guard rails: the orphan is actually deleted, a UI-created row (managed_by_catalog =
// false) with a name absent from the YAML is never touched, a session template still
// referenced by a ScheduledSession survives even though it dropped out of the YAML, and an
// import with zero session templates (an empty directory) does not wipe existing managed
// session templates.
func TestImportYAMLPrunesRenameOrphansButProtectsInUseAndUIRows(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewSqlite(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}

	exercisesDir := filepath.Join(tmp, "exercises")
	templatesDir := filepath.Join(tmp, "templates")
	if err := os.MkdirAll(exercisesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile(filepath.Join(exercisesDir, "pullups.yaml"), `
name: "Pull Ups"
kind: "reps_and_sets"
sets: 3
reps: 10
`)
	writeFile(filepath.Join(exercisesDir, "pushups.yaml"), `
name: "Push Ups"
kind: "reps_and_sets"
sets: 3
reps: 15
`)
	writeFile(filepath.Join(templatesDir, "strength.yaml"), `
name: "Strength Session"
activities:
  - type: "activity"
    exercises:
      - ref: "Pull Ups"
`)
	writeFile(filepath.Join(templatesDir, "old.yaml"), `
name: "Old Session"
activities:
  - type: "activity"
    exercises:
      - ref: "Push Ups"
`)

	const ownerID uint = 1
	opts := YAMLImportOptions{OwnerID: ownerID, ExercisesDir: exercisesDir, SessionTemplatesDir: templatesDir}
	if err := store.ImportYAML(opts); err != nil {
		t.Fatalf("initial import: %v", err)
	}

	// A UI-created library exercise: managed_by_catalog stays false and its name is
	// never in the YAML, so it must survive every future prune.
	uiExercise := LibraryExercise{OwnerID: ownerID, Name: "Custom Stretch", Kind: "reps_and_sets"}
	if err := store.DB.Create(&uiExercise).Error; err != nil {
		t.Fatal(err)
	}

	var oldSession SessionTemplate
	if err := store.DB.Where("owner_id = ? AND name = ?", ownerID, "Old Session").First(&oldSession).Error; err != nil {
		t.Fatalf("lookup Old Session: %v", err)
	}
	// A scheduled session pins "Old Session" in place even after it drops out of the YAML.
	scheduled := ScheduledSession{OwnerID: ownerID, ScheduledDate: time.Now(), SessionTemplateID: oldSession.ID}
	if err := store.DB.Create(&scheduled).Error; err != nil {
		t.Fatal(err)
	}

	// Simulate renames: "Push Ups" and "Old Session" drop out of the YAML entirely.
	if err := os.Remove(filepath.Join(exercisesDir, "pushups.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(templatesDir, "old.yaml")); err != nil {
		t.Fatal(err)
	}

	if err := store.ImportYAML(opts); err != nil {
		t.Fatalf("re-import after rename: %v", err)
	}

	var pushUpsCount int64
	if err := store.DB.Model(&LibraryExercise{}).
		Where("owner_id = ? AND name = ?", ownerID, "Push Ups").Count(&pushUpsCount).Error; err != nil {
		t.Fatal(err)
	}
	if pushUpsCount != 0 {
		t.Errorf("expected renamed-away 'Push Ups' to be pruned, but %d row(s) remain", pushUpsCount)
	}

	var uiCount int64
	if err := store.DB.Model(&LibraryExercise{}).
		Where("owner_id = ? AND name = ? AND managed_by_catalog = ?", ownerID, "Custom Stretch", false).
		Count(&uiCount).Error; err != nil {
		t.Fatal(err)
	}
	if uiCount != 1 {
		t.Errorf("expected UI-created 'Custom Stretch' to survive the prune, got count %d", uiCount)
	}

	var oldSessionCount int64
	if err := store.DB.Model(&SessionTemplate{}).
		Where("owner_id = ? AND name = ?", ownerID, "Old Session").Count(&oldSessionCount).Error; err != nil {
		t.Fatal(err)
	}
	if oldSessionCount != 1 {
		t.Errorf("expected 'Old Session' to survive because it has a scheduled session, got count %d", oldSessionCount)
	}

	// Empty-category guard: an import whose session-templates directory has zero YAML
	// files must not wipe the managed session templates that already exist.
	emptyTemplatesDir := filepath.Join(tmp, "templates_empty")
	if err := os.MkdirAll(emptyTemplatesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.ImportYAML(YAMLImportOptions{
		OwnerID:             ownerID,
		ExercisesDir:        exercisesDir,
		SessionTemplatesDir: emptyTemplatesDir,
	}); err != nil {
		t.Fatalf("import with empty session-templates dir: %v", err)
	}

	var tplCount int64
	if err := store.DB.Model(&SessionTemplate{}).Where("owner_id = ?", ownerID).Count(&tplCount).Error; err != nil {
		t.Fatal(err)
	}
	if tplCount != 2 {
		t.Errorf("expected both session templates to survive an empty-category import, got count %d", tplCount)
	}
}

// TestImportYAMLWalksSubdirectories guards that the catalog can be organised into
// folders. An entry's identity is its name, not its path, so files must import the same
// whether they sit at the top level or nested — and moving one must not look like a
// rename (which the prune would act on).
func TestImportYAMLWalksSubdirectories(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSqlite(filepath.Join(dir, "folders.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&User{Email: "f@f.com", PasswordHash: "x"}).Error; err != nil {
		t.Fatal(err)
	}

	exDir := filepath.Join(dir, "exercises")
	nested := filepath.Join(exDir, "ondra", "mobility")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	stDir := filepath.Join(dir, "session_templates")
	if err := os.MkdirAll(filepath.Join(stDir, "programs"), 0o755); err != nil {
		t.Fatal(err)
	}
	// One exercise at the top level, one two folders deep.
	write := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(exDir, "flat.yaml"), "name: \"Flat Move\"\nkind: \"session\"\n")
	write(filepath.Join(nested, "deep.yaml"), "name: \"Deep Move\"\nkind: \"session\"\n")
	// A session template in a subfolder that refs the nested exercise.
	write(filepath.Join(stDir, "programs", "s.yaml"),
		"name: \"Folder Session\"\nactivities:\n  - type: \"activity\"\n    exercises:\n      - ref: \"Deep Move\"\n")

	if err := store.ImportYAML(YAMLImportOptions{
		OwnerID: 1, ExercisesDir: exDir, SessionTemplatesDir: stDir,
	}); err != nil {
		t.Fatalf("import with nested dirs failed: %v", err)
	}

	for _, name := range []string{"Flat Move", "Deep Move"} {
		var n int64
		store.DB.Model(&LibraryExercise{}).Where("owner_id = ? AND name = ?", 1, name).Count(&n)
		if n != 1 {
			t.Errorf("exercise %q: got %d rows, want 1 — nested files must import", name, n)
		}
	}
	var tn int64
	store.DB.Model(&SessionTemplate{}).Where("owner_id = ? AND name = ?", 1, "Folder Session").Count(&tn)
	if tn != 1 {
		t.Errorf("session template in a subfolder did not import (got %d rows)", tn)
	}
}
