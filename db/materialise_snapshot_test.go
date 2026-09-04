package db

import (
	"path/filepath"
	"testing"
	"time"
)

// A run has to own what it says the athlete did. The importer deletes and recreates a
// template's exercises on every restart, so a run that reads the template renders whatever
// the catalog says today. These tests pin the snapshot.
//
// Three real defects sat here. The copy carried only Name, Kind and OrderIndex, so every
// prescription became zero. It created a second completion without moving the first, so the
// same exercise held two. And it skipped option rows entirely, so a catalog parent's
// choices stayed under ids the run did not own.

type snapshotFixture struct {
	store  *Store
	ss     ScheduledSession
	run    SessionRun
	parent Exercise
	child  Exercise
	plain  Exercise
}

func newSnapshotFixture(t *testing.T, name string) snapshotFixture {
	t.Helper()
	store, err := NewSqlite(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	const owner uint = 1
	gdb := store.DB

	lib := LibraryExercise{OwnerID: owner, Name: "Weighted Pull-ups"}
	mustCreate(t, store, &lib)

	tpl := SessionTemplate{OwnerID: owner, Name: "Strength Day"}
	mustCreate(t, store, &tpl)
	act := Activity{OwnerID: owner, SessionTemplateID: tpl.ID, Type: "activity"}
	mustCreate(t, store, &act)

	// A fully specified exercise. Every field here must survive the copy.
	plain := Exercise{
		OwnerID: owner, ActivityID: &act.ID, LibraryExerciseID: &lib.ID,
		Name: "Weighted Pull-ups", Notes: "controlled tempo", Kind: "timed_reps",
		Sets: 5, Reps: 5, RepSeconds: 5, RepRestSeconds: 10, SetRestSeconds: 120,
		PrepSeconds: 15, RungSeconds: "3,6,9", WeightKg: 12.5,
		SessionDurationSeconds: 600, OrderIndex: 0,
	}
	mustCreate(t, store, &plain)

	// A catalog parent with one option hanging off it.
	parent := Exercise{OwnerID: owner, ActivityID: &act.ID, Name: "Pick a hang", Kind: "exercise_catalog", OrderIndex: 1}
	mustCreate(t, store, &parent)
	child := Exercise{OwnerID: owner, ActivityID: &act.ID, ParentExerciseID: &parent.ID,
		Name: "20mm half crimp", Kind: "reps_and_sets", Sets: 4, Reps: 1, WeightKg: 30}
	mustCreate(t, store, &child)

	ss := ScheduledSession{OwnerID: owner, ScheduledDate: time.Now(), SessionTemplateID: tpl.ID}
	mustCreate(t, store, &ss)
	run := SessionRun{OwnerID: owner, ScheduledSessionID: ss.ID, StartedAt: time.Now()}
	mustCreate(t, store, &run)

	if err := gdb.Preload("SessionTemplate.Activities.Exercises").
		Where("id = ?", ss.ID).First(&ss).Error; err != nil {
		t.Fatal(err)
	}
	return snapshotFixture{store, ss, run, parent, child, plain}
}

func runExercises(t *testing.T, store *Store, runID uint) []Exercise {
	t.Helper()
	var out []Exercise
	if err := store.DB.Where("session_run_id = ?", runID).Order("order_index").Find(&out).Error; err != nil {
		t.Fatal(err)
	}
	return out
}

// The prescription is the point. A run that renders zeros tells the athlete nothing.
func TestMaterialiseCopiesTheWholePrescription(t *testing.T) {
	f := newSnapshotFixture(t, "snap-fields.db")

	if err := MaterialiseTemplateExercises(f.store.DB, 1, f.run.ID, f.ss); err != nil {
		t.Fatal(err)
	}

	var got Exercise
	if err := f.store.DB.Where("session_run_id = ? AND name = ?", f.run.ID, "Weighted Pull-ups").
		First(&got).Error; err != nil {
		t.Fatalf("the run has no copy of the exercise: %v", err)
	}

	checks := []struct {
		field string
		want  any
		got   any
	}{
		{"Notes", f.plain.Notes, got.Notes},
		{"Kind", f.plain.Kind, got.Kind},
		{"Sets", f.plain.Sets, got.Sets},
		{"Reps", f.plain.Reps, got.Reps},
		{"RepSeconds", f.plain.RepSeconds, got.RepSeconds},
		{"RepRestSeconds", f.plain.RepRestSeconds, got.RepRestSeconds},
		{"SetRestSeconds", f.plain.SetRestSeconds, got.SetRestSeconds},
		{"PrepSeconds", f.plain.PrepSeconds, got.PrepSeconds},
		{"RungSeconds", f.plain.RungSeconds, got.RungSeconds},
		{"WeightKg", f.plain.WeightKg, got.WeightKg},
		{"SessionDurationSeconds", f.plain.SessionDurationSeconds, got.SessionDurationSeconds},
	}
	for _, c := range checks {
		if c.want != c.got {
			t.Errorf("%s was dropped by the copy: want %v, got %v", c.field, c.want, c.got)
		}
	}
	if got.LibraryExerciseID == nil || *got.LibraryExerciseID != *f.plain.LibraryExerciseID {
		t.Error("LibraryExerciseID must come across, or metrics cannot group the movement across runs")
	}
}

// A completion is a record of one thing the athlete did. Two of them is a lie.
func TestMaterialiseMovesTheCompletionInsteadOfCopyingIt(t *testing.T) {
	f := newSnapshotFixture(t, "snap-comp.db")

	comp := RunExerciseCompletion{
		OwnerID: 1, RunID: f.run.ID, ExerciseID: f.plain.ID,
		Status: "completed", CompletedAt: time.Now(),
		ActualSets: 5, ActualReps: 4, ActualWeightKg: 12.5,
		RunNotes: "last set was ugly",
	}
	mustCreate(t, f.store, &comp)

	if err := MaterialiseTemplateExercises(f.store.DB, 1, f.run.ID, f.ss); err != nil {
		t.Fatal(err)
	}

	var comps []RunExerciseCompletion
	if err := f.store.DB.Where("run_id = ?", f.run.ID).Find(&comps).Error; err != nil {
		t.Fatal(err)
	}
	if len(comps) != 1 {
		t.Fatalf("want exactly 1 completion after materialising, got %d", len(comps))
	}
	if comps[0].ID != comp.ID {
		t.Error("the completion should be moved, keeping its id, not recreated")
	}
	if comps[0].ExerciseID == f.plain.ID {
		t.Error("the completion still points at the template exercise")
	}
	if comps[0].ActualReps != 4 || comps[0].RunNotes != "last set was ugly" {
		t.Errorf("the logged values changed: %+v", comps[0])
	}

	// And it must point at a row the run owns.
	var owned int64
	if err := f.store.DB.Model(&Exercise{}).
		Where("id = ? AND session_run_id = ?", comps[0].ExerciseID, f.run.ID).
		Count(&owned).Error; err != nil {
		t.Fatal(err)
	}
	if owned != 1 {
		t.Error("the completion points at an exercise the run does not own")
	}
}

// A catalog parent is useless in a run without its options.
func TestMaterialiseBringsOptionRowsAcross(t *testing.T) {
	f := newSnapshotFixture(t, "snap-children.db")

	choice := RunExerciseChoice{
		OwnerID: 1, RunID: f.run.ID,
		ParentExerciseID: f.parent.ID, ChosenExerciseID: f.child.ID,
	}
	mustCreate(t, f.store, &choice)

	if err := MaterialiseTemplateExercises(f.store.DB, 1, f.run.ID, f.ss); err != nil {
		t.Fatal(err)
	}

	rows := runExercises(t, f.store, f.run.ID)
	byName := map[string]Exercise{}
	for _, r := range rows {
		byName[r.Name] = r
	}
	newParent, okP := byName["Pick a hang"]
	newChild, okC := byName["20mm half crimp"]
	if !okP || !okC {
		t.Fatalf("the run is missing the parent or its option: %v", byName)
	}
	if newChild.ParentExerciseID == nil || *newChild.ParentExerciseID != newParent.ID {
		t.Error("the option must hang off the run's own parent, not the template's")
	}
	if newChild.Sets != 4 || newChild.WeightKg != 30 {
		t.Errorf("the option lost its prescription: sets=%d weight=%v", newChild.Sets, newChild.WeightKg)
	}

	var got RunExerciseChoice
	if err := f.store.DB.Where("id = ?", choice.ID).First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.ParentExerciseID != newParent.ID {
		t.Error("the choice still names the template's parent")
	}
	if got.ChosenExerciseID != newChild.ID {
		t.Error("the choice still names the template's option — this one was never repointed")
	}
}

// The whole point: after materialising, the run must not depend on the template at all.
func TestMaterialisedRunSurvivesTheTemplateBeingRewritten(t *testing.T) {
	f := newSnapshotFixture(t, "snap-detach.db")

	comp := RunExerciseCompletion{
		OwnerID: 1, RunID: f.run.ID, ExerciseID: f.plain.ID,
		Status: "completed", CompletedAt: time.Now(), ActualReps: 5,
	}
	mustCreate(t, f.store, &comp)

	if err := MaterialiseTemplateExercises(f.store.DB, 1, f.run.ID, f.ss); err != nil {
		t.Fatal(err)
	}

	// What the importer does on every restart: retire this generation of children.
	if err := f.store.DB.Where("activity_id IS NOT NULL").Delete(&Exercise{}).Error; err != nil {
		t.Fatal(err)
	}

	rows := runExercises(t, f.store, f.run.ID)
	if len(rows) != 3 {
		t.Fatalf("the run should still hold its 3 exercises, got %d", len(rows))
	}
	var got RunExerciseCompletion
	if err := f.store.DB.Where("id = ?", comp.ID).First(&got).Error; err != nil {
		t.Fatalf("the completion vanished with the template: %v", err)
	}
	var live int64
	if err := f.store.DB.Model(&Exercise{}).Where("id = ?", got.ExerciseID).Count(&live).Error; err != nil {
		t.Fatal(err)
	}
	if live != 1 {
		t.Error("the completion points at an exercise that no longer exists")
	}
}
