package db

import (
	"os"
	"path/filepath"
	"testing"
)

// The importer runs for every user on every server restart. Re-importing YAML that has
// not changed is therefore the commonest path through this code by a wide margin, and it
// was the one path no test exercised — every other re-import test either renames
// something (taking the prune branch) or marks a row edited (taking the skip branch).
//
// A soft delete only removes rows from the table it names. Soft-deleting an Activity is
// an UPDATE, so the ON DELETE CASCADE on activities→exercises never fires, and the child
// Exercise rows were left live and unreachable. Around 3,400 of them accumulated per day,
// and a production database reached 140MB before anyone counted rows.

func writeReimportFixture(t *testing.T) YAMLImportOptions {
	t.Helper()
	tmp := t.TempDir()
	exercisesDir := filepath.Join(tmp, "exercises")
	templatesDir := filepath.Join(tmp, "templates")
	for _, d := range []string{exercisesDir, templatesDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(exercisesDir, "pullups.yaml"), []byte(`
name: "Weighted Pull-ups"
kind: "reps_and_sets"
sets: 5
reps: 5
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "strength.yaml"), []byte(`
name: "Strength Base Session"
activities:
  - type: "warmup"
    exercises:
      - name: "Band Prep"
        sets: 2
        reps: 12
  - type: "activity"
    exercises:
      - ref: "Weighted Pull-ups"
      - name: "Ring Rows"
        sets: 4
        reps: 10
`), 0o644); err != nil {
		t.Fatal(err)
	}
	return YAMLImportOptions{
		OwnerID:             1,
		ExercisesDir:        []string{exercisesDir},
		SessionTemplatesDir: []string{templatesDir},
	}
}

func countRows(t *testing.T, store *Store, model any, cond ...any) int64 {
	t.Helper()
	var n int64
	q := store.DB.Model(model)
	if len(cond) > 0 {
		q = q.Where(cond[0], cond[1:]...)
	}
	if err := q.Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// Importing the same unchanged YAML repeatedly must not grow the database.
func TestReimportUnchangedYAMLDoesNotGrowExercises(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "reimport.db"))
	if err != nil {
		t.Fatal(err)
	}
	opts := writeReimportFixture(t)

	if err := store.ImportYAML(opts); err != nil {
		t.Fatal(err)
	}
	liveAfterFirst := countRows(t, store, &Exercise{})
	if liveAfterFirst == 0 {
		t.Fatal("the fixture imported no exercises at all")
	}

	for i := 0; i < 3; i++ {
		if err := store.ImportYAML(opts); err != nil {
			t.Fatalf("re-import %d failed: %v", i+1, err)
		}
	}

	if got := countRows(t, store, &Exercise{}); got != liveAfterFirst {
		t.Fatalf("live exercises grew from %d to %d over 3 unchanged re-imports", liveAfterFirst, got)
	}
}

// The direct check. A live exercise whose activity is gone is unreachable from every
// template view, but still occupies a row and still counts against the file size.
func TestReimportLeavesNoLiveExerciseUnderADeadActivity(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "reimport-orphan.db"))
	if err != nil {
		t.Fatal(err)
	}
	opts := writeReimportFixture(t)

	for i := 0; i < 3; i++ {
		if err := store.ImportYAML(opts); err != nil {
			t.Fatal(err)
		}
	}

	orphans := countRows(t, store, &Exercise{},
		"activity_id IS NOT NULL AND activity_id NOT IN (SELECT id FROM activities WHERE deleted_at IS NULL)")
	if orphans != 0 {
		t.Fatalf("%d live exercises hang off an activity that no longer exists", orphans)
	}

	mediaOrphans := countRows(t, store, &ExerciseMedia{},
		"exercise_id IS NOT NULL AND exercise_id NOT IN (SELECT id FROM exercises WHERE deleted_at IS NULL)")
	if mediaOrphans != 0 {
		t.Fatalf("%d live media rows hang off an exercise that no longer exists", mediaOrphans)
	}
}

// Retiring the old generation must not hard-delete it. RunExerciseCompletion,
// ClimbingTick, ManualExerciseSetLog, ExercisePlannedSet, RunExerciseChoice and
// ClimbingExerciseMeta all carry exercise_id with no REFERENCES clause, so nothing at the
// database level would stop a hard delete from stranding a completed run for good.
func TestReimportKeepsRetiredExercisesRecoverable(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "reimport-soft.db"))
	if err != nil {
		t.Fatal(err)
	}
	opts := writeReimportFixture(t)

	if err := store.ImportYAML(opts); err != nil {
		t.Fatal(err)
	}
	var firstGeneration []uint
	if err := store.DB.Model(&Exercise{}).Pluck("id", &firstGeneration).Error; err != nil {
		t.Fatal(err)
	}

	if err := store.ImportYAML(opts); err != nil {
		t.Fatal(err)
	}

	var stillOnDisk int64
	if err := store.DB.Unscoped().Model(&Exercise{}).
		Where("id IN ?", firstGeneration).Count(&stillOnDisk).Error; err != nil {
		t.Fatal(err)
	}
	if stillOnDisk != int64(len(firstGeneration)) {
		t.Fatalf("re-import hard-deleted %d of %d retired exercises — run history referencing them "+
			"would be stranded with no row behind it",
			int64(len(firstGeneration))-stillOnDisk, len(firstGeneration))
	}
}
