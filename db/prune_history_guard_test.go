package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// pruneCatalogOrphans hard-deletes a block's exercises with tx.Unscoped(), and none of the
// six tables that carry exercise_id declares a foreign key to exercises. So
// referencedByHistoryWhere is the only thing standing between a block leaving the YAML and
// a completed session losing its record. countExercisesReferencedByHistory is the guard;
// these tests cover the parts of it the incident tests do not reach.

// blockPruneFixture imports a catalog holding one library exercise, one session template
// and one activity template ("finger_block") whose single exercise refs the library entry.
// Returns the store, the options to re-import with, the block directory, and the block's
// one exercise.
func blockPruneFixture(t *testing.T, name string) (*Store, YAMLImportOptions, string, Exercise) {
	t.Helper()
	tmp := t.TempDir()
	store, err := NewSqlite(filepath.Join(tmp, name+".db"))
	if err != nil {
		t.Fatal(err)
	}
	const ownerID uint = 1
	seedImportOwner(t, store, ownerID)

	exDir := filepath.Join(tmp, "exercises")
	tplDir := filepath.Join(tmp, "templates")
	atDir := filepath.Join(tmp, "blocks")
	mustWrite(t, exDir, "e.yaml", `
name: "Weighted Pull-ups"
slug: "weighted_pull_ups"
kind: "reps_and_sets"
sets: 5
reps: 5
`)
	mustWrite(t, tplDir, "t.yaml", `
name: "Strength Day"
slug: "strength_day"
activities:
  - type: "activity"
    exercises:
      - ref: "weighted_pull_ups"
`)
	mustWrite(t, atDir, "b.yaml", `
name: "Finger Block"
slug: "finger_block"
type: "activity"
exercises:
  - ref: "weighted_pull_ups"
`)

	opts := YAMLImportOptions{
		OwnerID:              ownerID,
		ExercisesDir:         []string{exDir},
		SessionTemplatesDir:  []string{tplDir},
		ActivityTemplatesDir: []string{atDir},
	}
	if err := store.ImportYAML(opts); err != nil {
		t.Fatalf("first import: %v", err)
	}

	var block ActivityTemplate
	if err := store.DB.Where("owner_id = ? AND slug = ?", ownerID, "finger_block").
		First(&block).Error; err != nil {
		t.Fatal(err)
	}
	var exs []Exercise
	if err := store.DB.Where("activity_template_id = ?", block.ID).Find(&exs).Error; err != nil {
		t.Fatal(err)
	}
	if len(exs) != 1 {
		t.Fatalf("fixture is decorative: the block has %d exercises, want 1", len(exs))
	}
	return store, opts, atDir, exs[0]
}

// dropBlockFromYAML is what a rename or a deletion looks like to the importer: the block's
// file is gone, but the category still has entries so the prune actually runs.
func dropBlockFromYAML(t *testing.T, atDir string) {
	t.Helper()
	if err := os.Remove(filepath.Join(atDir, "b.yaml")); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, atDir, "other.yaml", `
name: "Some Other Block"
slug: "some_other_block"
type: "activity"
exercises:
  - ref: "weighted_pull_ups"
`)
}

// referencedByHistoryWhere names six tables and seven columns. The incident test only
// exercises run_exercise_completions, so a typo in any of the other clauses — or a column
// dropped during a refactor — would hard-delete history with every test still green.
func TestPruningABlockKeepsItForEveryKindOfHistory(t *testing.T) {
	const ownerID uint = 1

	cases := []struct {
		name string
		// ref attaches one history row pointing at the block's exercise. other is an
		// exercise outside the block, used where a row needs two exercise columns so each
		// column can be tested on its own.
		ref func(t *testing.T, store *Store, blockEx Exercise, other Exercise)
	}{
		{"run_exercise_completions", func(t *testing.T, store *Store, ex, _ Exercise) {
			mustCreate(t, store, &RunExerciseCompletion{
				OwnerID: ownerID, RunID: 1, ExerciseID: ex.ID, CompletedAt: time.Now(),
			})
		}},
		{"climbing_ticks", func(t *testing.T, store *Store, ex, _ Exercise) {
			mustCreate(t, store, &ClimbingTick{
				OwnerID: ownerID, RunID: 1, ExerciseID: ex.ID, Kind: "boulder",
			})
		}},
		{"manual_exercise_set_logs", func(t *testing.T, store *Store, ex, _ Exercise) {
			mustCreate(t, store, &ManualExerciseSetLog{
				OwnerID: ownerID, RunID: 1, ExerciseID: ex.ID, SetIndex: 1, Reps: 5,
			})
		}},
		{"exercise_planned_sets", func(t *testing.T, store *Store, ex, _ Exercise) {
			mustCreate(t, store, &ExercisePlannedSet{
				OwnerID: ownerID, ExerciseID: ex.ID, SetIndex: 1, Reps: 5,
			})
		}},
		{"climbing_exercise_meta", func(t *testing.T, store *Store, ex, _ Exercise) {
			mustCreate(t, store, &ClimbingExerciseMeta{
				OwnerID: ownerID, RunID: 1, ExerciseID: ex.ID, Type: "board",
			})
		}},
		{"run_exercise_choices.parent_exercise_id", func(t *testing.T, store *Store, ex, other Exercise) {
			mustCreate(t, store, &RunExerciseChoice{
				OwnerID: ownerID, RunID: 1, ParentExerciseID: ex.ID, ChosenExerciseID: other.ID,
			})
		}},
		{"run_exercise_choices.chosen_exercise_id", func(t *testing.T, store *Store, ex, other Exercise) {
			mustCreate(t, store, &RunExerciseChoice{
				OwnerID: ownerID, RunID: 1, ParentExerciseID: other.ID, ChosenExerciseID: ex.ID,
			})
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, opts, atDir, blockEx := blockPruneFixture(t, "hist")

			// Lives outside the block, so it is never matched by activity_template_id and
			// cannot be the reason the block survives.
			other := Exercise{OwnerID: ownerID, Name: "Unrelated"}
			mustCreate(t, store, &other)

			tc.ref(t, store, blockEx, other)

			dropBlockFromYAML(t, atDir)
			if err := store.ImportYAML(opts); err != nil {
				t.Fatalf("second import: %v", err)
			}

			var exLeft int64
			if err := store.DB.Unscoped().Model(&Exercise{}).
				Where("id = ?", blockEx.ID).Count(&exLeft).Error; err != nil {
				t.Fatal(err)
			}
			if exLeft != 1 {
				t.Errorf("prune hard-deleted the exercise that %s points at", tc.name)
			}
			var blockLeft int64
			if err := store.DB.Unscoped().Model(&ActivityTemplate{}).
				Where("slug = ?", "finger_block").Count(&blockLeft).Error; err != nil {
				t.Fatal(err)
			}
			if blockLeft != 1 {
				t.Errorf("prune removed the block despite a reference from %s", tc.name)
			}
		})
	}
}

// The scenario the Unscoped() in countExercisesReferencedByHistory exists for, and the one
// production actually produces.
//
// upsertActivityTemplate soft-deletes a block's exercises and rebuilds them on every
// import. So an athlete who did the block in January has a completion pointing at a row
// that later imports have soft-deleted. When the block finally leaves the YAML, the live
// generation has no history at all — only the soft-deleted one does. deleteExercisesAndMedia
// then hard-deletes by activity_template_id with Unscoped(), which takes every generation.
//
// Drop the Unscoped() from the count and this is silent, permanent data loss.
func TestPruningABlockCountsHistoryOnASoftDeletedExercise(t *testing.T) {
	const ownerID uint = 1
	store, opts, atDir, firstGen := blockPruneFixture(t, "softgen")

	mustCreate(t, store, &RunExerciseCompletion{
		OwnerID: ownerID, RunID: 1, ExerciseID: firstGen.ID, CompletedAt: time.Now(),
	})

	// A later deploy re-imports the unchanged block, retiring the generation the athlete's
	// completion points at.
	if err := store.ImportYAML(opts); err != nil {
		t.Fatalf("second import: %v", err)
	}

	var retired Exercise
	if err := store.DB.Unscoped().Where("id = ?", firstGen.ID).First(&retired).Error; err != nil {
		t.Fatal(err)
	}
	if !retired.DeletedAt.Valid {
		t.Fatal("fixture is decorative: the re-import did not soft-delete the first generation, " +
			"so this test no longer covers the Unscoped() path")
	}
	var live int64
	if err := store.DB.Model(&Exercise{}).
		Where("activity_template_id IS NOT NULL AND id <> ?", firstGen.ID).
		Count(&live).Error; err != nil {
		t.Fatal(err)
	}
	if live == 0 {
		t.Fatal("fixture is decorative: the re-import created no new generation")
	}

	// Now the block leaves the YAML. Only the soft-deleted row carries history.
	dropBlockFromYAML(t, atDir)
	if err := store.ImportYAML(opts); err != nil {
		t.Fatalf("third import: %v", err)
	}

	var survived int64
	if err := store.DB.Unscoped().Model(&Exercise{}).
		Where("id = ?", firstGen.ID).Count(&survived).Error; err != nil {
		t.Fatal(err)
	}
	if survived != 1 {
		t.Error("prune hard-deleted a soft-deleted exercise that a completion still points at")
	}
	var stranded int64
	if err := store.DB.Model(&RunExerciseCompletion{}).
		Where("exercise_id NOT IN (SELECT id FROM exercises)").Count(&stranded).Error; err != nil {
		t.Fatal(err)
	}
	if stranded != 0 {
		t.Errorf("%d completion(s) now point at an exercise that no longer exists", stranded)
	}
}

// The negative control. Without this the guard above is indistinguishable from having
// switched the prune off: a block that leaves the YAML with nothing pointing at it must
// still go.
func TestPruningABlockStillRemovesItWhenNothingReferencesIt(t *testing.T) {
	store, opts, atDir, ex := blockPruneFixture(t, "blockprune-clean")
	blockID := *ex.ActivityTemplateID

	dropBlockFromYAML(t, atDir)
	if err := store.ImportYAML(opts); err != nil {
		t.Fatalf("second import: %v", err)
	}

	var left int64
	if err := store.DB.Unscoped().Model(&ActivityTemplate{}).
		Where("id = ?", blockID).Count(&left).Error; err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Error("a block with nothing pointing at it should still be pruned when it leaves the YAML")
	}
}
