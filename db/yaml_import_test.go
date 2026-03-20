package db

import (
	"os"
	"path/filepath"
	"testing"
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
