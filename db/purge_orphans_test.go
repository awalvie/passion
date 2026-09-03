package db

import (
	"path/filepath"
	"testing"
	"time"
)

// seedOrphanFixture builds one soft-deleted activity holding two exercises. It returns
// their ids: the first is a plain orphan, the second is meant to be claimed by history.
func seedOrphanFixture(t *testing.T, store *Store) (plain, referenced uint) {
	t.Helper()

	tpl := SessionTemplate{OwnerID: 1, Name: "Old Session"}
	if err := store.DB.Create(&tpl).Error; err != nil {
		t.Fatal(err)
	}
	act := Activity{OwnerID: 1, SessionTemplateID: tpl.ID, Type: "activity"}
	if err := store.DB.Create(&act).Error; err != nil {
		t.Fatal(err)
	}
	a := Exercise{OwnerID: 1, ActivityID: &act.ID, Name: "Nobody Ran This"}
	b := Exercise{OwnerID: 1, ActivityID: &act.ID, Name: "He Actually Did This"}
	for _, e := range []*Exercise{&a, &b} {
		if err := store.DB.Create(e).Error; err != nil {
			t.Fatal(err)
		}
	}

	// Retire the activity the way the importer does: soft delete, children untouched.
	if err := store.DB.Delete(&Activity{}, act.ID).Error; err != nil {
		t.Fatal(err)
	}
	return a.ID, b.ID
}

func liveExerciseExists(t *testing.T, store *Store, id uint) bool {
	t.Helper()
	var n int64
	if err := store.DB.Model(&Exercise{}).Where("id = ?", id).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func TestPurgeOrphanedExercisesDeletesTrueOrphans(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "purge-true.db"))
	if err != nil {
		t.Fatal(err)
	}
	plain, alsoPlain := seedOrphanFixture(t, store)

	rep, err := PurgeOrphanedExercises(store.DB, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.DeletedExercises != 2 {
		t.Fatalf("want 2 orphans deleted, got %d", rep.DeletedExercises)
	}
	if liveExerciseExists(t, store, plain) || liveExerciseExists(t, store, alsoPlain) {
		t.Fatal("an unreferenced orphan survived the purge")
	}
	if rep.DeletedActivities != 1 {
		t.Fatalf("the emptied activity should go too, deleted %d", rep.DeletedActivities)
	}
}

// The one that matters. Each of these six tables carries exercise_id with no REFERENCES
// clause, so the database will not stop a delete that strands them. If this test ever
// fails, the purge is erasing a record of a session the athlete actually did.
func TestPurgeOrphanedExercisesPreservesExercisesReferencedByRunHistory(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name string
		ref  func(t *testing.T, store *Store, exerciseID uint)
	}{
		{"run completion", func(t *testing.T, s *Store, id uint) {
			mustCreate(t, s, &RunExerciseCompletion{OwnerID: 1, RunID: 1, ExerciseID: id, CompletedAt: now})
		}},
		{"climbing tick", func(t *testing.T, s *Store, id uint) {
			mustCreate(t, s, &ClimbingTick{OwnerID: 1, RunID: 1, ExerciseID: id, Kind: "boulder"})
		}},
		{"manual set log", func(t *testing.T, s *Store, id uint) {
			mustCreate(t, s, &ManualExerciseSetLog{OwnerID: 1, RunID: 1, ExerciseID: id, SetIndex: 1})
		}},
		{"planned set", func(t *testing.T, s *Store, id uint) {
			mustCreate(t, s, &ExercisePlannedSet{OwnerID: 1, ExerciseID: id, SetIndex: 1})
		}},
		{"chosen option", func(t *testing.T, s *Store, id uint) {
			mustCreate(t, s, &RunExerciseChoice{OwnerID: 1, RunID: 1, ParentExerciseID: 999, ChosenExerciseID: id})
		}},
		{"climbing meta", func(t *testing.T, s *Store, id uint) {
			mustCreate(t, s, &ClimbingExerciseMeta{OwnerID: 1, RunID: 1, ExerciseID: id, Type: "board"})
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, err := NewSqlite(filepath.Join(t.TempDir(), "purge-keep.db"))
			if err != nil {
				t.Fatal(err)
			}
			plain, referenced := seedOrphanFixture(t, store)
			tc.ref(t, store, referenced)

			rep, err := PurgeOrphanedExercises(store.DB, false)
			if err != nil {
				t.Fatal(err)
			}
			if rep.KeptForHistory != 1 {
				t.Fatalf("the referenced orphan should be counted as kept, got %d", rep.KeptForHistory)
			}
			if !liveExerciseExists(t, store, referenced) {
				t.Fatalf("the purge erased an exercise still referenced by a %s", tc.name)
			}
			if liveExerciseExists(t, store, plain) {
				t.Fatal("the unreferenced orphan should still have been removed")
			}
			// The activity still holds a kept exercise, so it must survive: activities
			// cascade into exercises for real, and removing it would take the kept row.
			if rep.DeletedActivities != 0 {
				t.Fatalf("an activity with a kept child must not be deleted, deleted %d", rep.DeletedActivities)
			}
		})
	}
}

// A soft-deleted history row is still a record of something the athlete did.
func TestPurgeOrphanedExercisesKeepsRowsHeldBySoftDeletedHistory(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "purge-softref.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, referenced := seedOrphanFixture(t, store)

	comp := RunExerciseCompletion{OwnerID: 1, RunID: 1, ExerciseID: referenced, CompletedAt: time.Now()}
	mustCreate(t, store, &comp)
	if err := store.DB.Delete(&comp).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := PurgeOrphanedExercises(store.DB, false); err != nil {
		t.Fatal(err)
	}
	if !liveExerciseExists(t, store, referenced) {
		t.Fatal("a soft-deleted completion still holds its exercise")
	}
}

// An exercise still reachable from a live template is not an orphan and must not be
// touched, however many orphans sit beside it.
func TestPurgeOrphanedExercisesLeavesLiveTemplatesAlone(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "purge-live.db"))
	if err != nil {
		t.Fatal(err)
	}
	seedOrphanFixture(t, store)

	tpl := SessionTemplate{OwnerID: 1, Name: "Current Session"}
	mustCreate(t, store, &tpl)
	act := Activity{OwnerID: 1, SessionTemplateID: tpl.ID, Type: "activity"}
	mustCreate(t, store, &act)
	live := Exercise{OwnerID: 1, ActivityID: &act.ID, Name: "In Use"}
	mustCreate(t, store, &live)

	if _, err := PurgeOrphanedExercises(store.DB, false); err != nil {
		t.Fatal(err)
	}
	if !liveExerciseExists(t, store, live.ID) {
		t.Fatal("the purge removed an exercise under a live activity")
	}
}

// Exercise.ParentExerciseID is a real foreign key back to exercises, and it is NO ACTION
// rather than a cascade. SQLite checks foreign keys at the end of each statement, so a
// parent and its children only survive removal together. An earlier version of this purge
// deleted in batches of 5,000 and died with "FOREIGN KEY constraint failed" against the
// real database, where 44,513 of the orphans are somebody's parent.
func TestPurgeOrphanedExercisesRemovesParentsAndChildrenTogether(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "purge-parents.db"))
	if err != nil {
		t.Fatal(err)
	}

	tpl := SessionTemplate{OwnerID: 1, Name: "Old Session"}
	mustCreate(t, store, &tpl)
	act := Activity{OwnerID: 1, SessionTemplateID: tpl.ID, Type: "activity"}
	mustCreate(t, store, &act)

	// A catalog parent with several options hanging off it, repeated enough times that a
	// naive batching implementation would split at least one pair.
	const families = 40
	var everyID []uint
	for i := 0; i < families; i++ {
		parent := Exercise{OwnerID: 1, ActivityID: &act.ID, Name: "Pick One", Kind: "exercise_catalog"}
		mustCreate(t, store, &parent)
		everyID = append(everyID, parent.ID)
		for c := 0; c < 3; c++ {
			child := Exercise{OwnerID: 1, ActivityID: &act.ID, Name: "Option", ParentExerciseID: &parent.ID}
			mustCreate(t, store, &child)
			everyID = append(everyID, child.ID)
		}
	}
	if err := store.DB.Delete(&Activity{}, act.ID).Error; err != nil {
		t.Fatal(err)
	}

	rep, err := PurgeOrphanedExercises(store.DB, false)
	if err != nil {
		t.Fatalf("purge failed on a parent/child family: %v", err)
	}
	if rep.DeletedExercises != int64(len(everyID)) {
		t.Fatalf("want %d exercises removed, got %d", len(everyID), rep.DeletedExercises)
	}
	for _, id := range everyID {
		if liveExerciseExists(t, store, id) {
			t.Fatalf("exercise %d survived the purge", id)
		}
	}
}

func TestPurgeOrphanedExercisesDryRunChangesNothing(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "purge-dry.db"))
	if err != nil {
		t.Fatal(err)
	}
	plain, other := seedOrphanFixture(t, store)

	rep, err := PurgeOrphanedExercises(store.DB, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.SafeToPurge != 2 {
		t.Fatalf("dry run should find 2 purgeable rows, found %d", rep.SafeToPurge)
	}
	if rep.DeletedExercises != 0 || rep.DeletedActivities != 0 {
		t.Fatal("a dry run must delete nothing")
	}
	if !liveExerciseExists(t, store, plain) || !liveExerciseExists(t, store, other) {
		t.Fatal("a dry run removed rows")
	}
}

func mustCreate(t *testing.T, store *Store, v any) {
	t.Helper()
	if err := store.DB.Create(v).Error; err != nil {
		t.Fatal(err)
	}
}
