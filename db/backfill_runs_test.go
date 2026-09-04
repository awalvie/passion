package db

import (
	"path/filepath"
	"testing"
	"time"
)

// brokenRun builds the state real runs were left in: completions pointing at template
// rows, and the template's activity retired the way the importer used to retire it.
func brokenRun(t *testing.T, store *Store) (SessionRun, RunExerciseCompletion, Exercise) {
	t.Helper()
	const owner uint = 1

	tpl := SessionTemplate{OwnerID: owner, Name: "Old Session"}
	mustCreate(t, store, &tpl)
	act := Activity{OwnerID: owner, SessionTemplateID: tpl.ID, Type: "activity", Name: "Main"}
	mustCreate(t, store, &act)
	ex := Exercise{OwnerID: owner, ActivityID: &act.ID, Name: "Pull-ups",
		Kind: "reps_and_sets", Sets: 5, Reps: 5, WeightKg: 10}
	mustCreate(t, store, &ex)

	ss := ScheduledSession{OwnerID: owner, ScheduledDate: time.Now(), SessionTemplateID: tpl.ID}
	mustCreate(t, store, &ss)
	run := SessionRun{OwnerID: owner, ScheduledSessionID: ss.ID, StartedAt: time.Now(),
		Status: RunStatusCompleted}
	mustCreate(t, store, &run)

	comp := RunExerciseCompletion{OwnerID: owner, RunID: run.ID, ExerciseID: ex.ID,
		Status: "completed", CompletedAt: time.Now(), ActualSets: 5, ActualReps: 4,
		RunNotes: "felt strong"}
	mustCreate(t, store, &comp)

	// What the importer did on every restart: retire the activity, leave the exercise.
	if err := store.DB.Delete(&Activity{}, act.ID).Error; err != nil {
		t.Fatal(err)
	}
	return run, comp, ex
}

func TestBackfillGivesAPastRunItsOwnExercises(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "backfill.db"))
	if err != nil {
		t.Fatal(err)
	}
	run, comp, srcEx := brokenRun(t, store)

	rep, err := BackfillRunExercises(store.DB, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Copied != 1 || rep.RunsChanged != 1 {
		t.Fatalf("want 1 exercise copied into 1 run, got copied=%d runs=%d", rep.Copied, rep.RunsChanged)
	}

	var got RunExerciseCompletion
	if err := store.DB.Where("id = ?", comp.ID).First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.ExerciseID == srcEx.ID {
		t.Fatal("the completion still points at the template row")
	}
	var owned Exercise
	if err := store.DB.Where("id = ? AND session_run_id = ?", got.ExerciseID, run.ID).
		First(&owned).Error; err != nil {
		t.Fatalf("the completion does not point at a row the run owns: %v", err)
	}
	if owned.Sets != 5 || owned.Reps != 5 || owned.WeightKg != 10 {
		t.Errorf("the copy lost the prescription: %+v", owned)
	}
	if owned.RunBlockName != "Main" || owned.RunBlockType != "activity" {
		t.Errorf("the copy lost its block: %q / %q", owned.RunBlockType, owned.RunBlockName)
	}
	if got.ActualReps != 4 || got.RunNotes != "felt strong" {
		t.Errorf("the logged values changed: %+v", got)
	}
}

// Run 40's shape: materialised once by the old code, leaving rows of zeros that nothing
// points at, while the real completions stayed on the template rows.
func TestBackfillClearsRowsNothingPointsAtAndKeepsTheRest(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "backfill-empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	run, comp, _ := brokenRun(t, store)

	// Two leftovers: one attached to nothing, one holding a real tick.
	junk := Exercise{OwnerID: 1, SessionRunID: &run.ID, Name: "Pull-ups", Sets: 0, Reps: 0}
	mustCreate(t, store, &junk)
	kept := Exercise{OwnerID: 1, SessionRunID: &run.ID, Name: "Boulder", Kind: "climbing"}
	mustCreate(t, store, &kept)
	mustCreate(t, store, &ClimbingTick{OwnerID: 1, RunID: run.ID, ExerciseID: kept.ID, Kind: "boulder"})

	if _, err := BackfillRunExercises(store.DB, false); err != nil {
		t.Fatal(err)
	}

	if liveExerciseExists(t, store, junk.ID) {
		t.Error("an unattached leftover row should have been removed")
	}
	if !liveExerciseExists(t, store, kept.ID) {
		t.Error("a row holding a real tick must never be removed")
	}
	var got RunExerciseCompletion
	if err := store.DB.Where("id = ?", comp.ID).First(&got).Error; err != nil {
		t.Fatal(err)
	}
	var owned int64
	if err := store.DB.Model(&Exercise{}).
		Where("id = ? AND session_run_id = ?", got.ExerciseID, run.ID).Count(&owned).Error; err != nil {
		t.Fatal(err)
	}
	if owned != 1 {
		t.Error("the completion was not repointed at a row the run owns")
	}
}

func TestBackfillDryRunChangesNothing(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "backfill-dry.db"))
	if err != nil {
		t.Fatal(err)
	}
	_, comp, srcEx := brokenRun(t, store)

	rep, err := BackfillRunExercises(store.DB, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Copied != 1 {
		t.Fatalf("dry run should report 1 copy, reported %d", rep.Copied)
	}
	var got RunExerciseCompletion
	if err := store.DB.Where("id = ?", comp.ID).First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.ExerciseID != srcEx.ID {
		t.Error("a dry run moved a completion")
	}
}

// Open sessions already own their exercises, and a user-added one legitimately has no
// prescription. The backfill must not touch them.
func TestBackfillSkipsOpenSessionsAndManualEntries(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "backfill-open.db"))
	if err != nil {
		t.Fatal(err)
	}
	const owner uint = 1
	for _, spec := range []struct{ open, manual bool }{{true, false}, {false, true}} {
		run := SessionRun{OwnerID: owner, ScheduledSessionID: 999, StartedAt: time.Now(),
			IsOpen: spec.open, IsManual: spec.manual}
		mustCreate(t, store, &run)
		ex := Exercise{OwnerID: owner, SessionRunID: &run.ID, Name: "Ad hoc", Sets: 0, Reps: 0}
		mustCreate(t, store, &ex)

		if _, err := BackfillRunExercises(store.DB, false); err != nil {
			t.Fatal(err)
		}
		if !liveExerciseExists(t, store, ex.ID) {
			t.Fatalf("the backfill removed an exercise from an open=%v manual=%v run",
				spec.open, spec.manual)
		}
	}
}

// Running it twice must not copy anything a second time.
func TestBackfillIsIdempotent(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "backfill-twice.db"))
	if err != nil {
		t.Fatal(err)
	}
	brokenRun(t, store)

	if _, err := BackfillRunExercises(store.DB, false); err != nil {
		t.Fatal(err)
	}
	rep, err := BackfillRunExercises(store.DB, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Copied != 0 || rep.EmptyRemoved != 0 {
		t.Fatalf("a second pass changed things: copied=%d removed=%d", rep.Copied, rep.EmptyRemoved)
	}
}
