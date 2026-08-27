package db

import (
	"path/filepath"
	"testing"
	"time"
)

// Ticks are written against the template exercise during a live run. When the log
// editor materialises the template into per-run exercise rows, the ticks have to
// follow — otherwise the editor renders the new row and reports "No climbs yet"
// while the data still sits under the old exercise id.
func TestMaterialiseTemplateExercises_RepointsTickData(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "materialise.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	gdb := store.DB
	const ownerID uint = 1

	tpl := SessionTemplate{OwnerID: ownerID, Name: "Route Session"}
	if err := gdb.Create(&tpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	act := Activity{OwnerID: ownerID, SessionTemplateID: tpl.ID, Type: "activity"}
	if err := gdb.Create(&act).Error; err != nil {
		t.Fatalf("create activity: %v", err)
	}
	actID := act.ID
	tplEx := Exercise{OwnerID: ownerID, ActivityID: &actID, Name: "Easy Routes", Kind: "climbing"}
	if err := gdb.Create(&tplEx).Error; err != nil {
		t.Fatalf("create exercise: %v", err)
	}

	ss := ScheduledSession{
		OwnerID:           ownerID,
		ScheduledDate:     time.Now(),
		SessionTemplateID: tpl.ID,
	}
	if err := gdb.Create(&ss).Error; err != nil {
		t.Fatalf("create scheduled session: %v", err)
	}
	run := SessionRun{OwnerID: ownerID, ScheduledSessionID: ss.ID, Status: RunStatusCompleted, StartedAt: time.Now()}
	if err := gdb.Create(&run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}

	// Data logged during the live run, keyed to the template exercise.
	tick := ClimbingTick{
		OwnerID: ownerID, RunID: run.ID, ExerciseID: tplEx.ID,
		Kind: "sport", Grade: "6b", Sent: true, Attempts: 2,
	}
	if err := gdb.Create(&tick).Error; err != nil {
		t.Fatalf("create tick: %v", err)
	}
	meta := ClimbingExerciseMeta{
		OwnerID: ownerID, RunID: run.ID, ExerciseID: tplEx.ID, Type: "gym_routes",
	}
	if err := gdb.Create(&meta).Error; err != nil {
		t.Fatalf("create meta: %v", err)
	}
	setLog := ManualExerciseSetLog{
		OwnerID: ownerID, RunID: run.ID, ExerciseID: tplEx.ID, SetIndex: 1, Reps: 5,
	}
	if err := gdb.Create(&setLog).Error; err != nil {
		t.Fatalf("create set log: %v", err)
	}

	// Reload the scheduled session with its template graph, as the handler does.
	var loaded ScheduledSession
	if err := gdb.Preload("SessionTemplate.Activities.Exercises").
		Where("id = ?", ss.ID).First(&loaded).Error; err != nil {
		t.Fatalf("reload scheduled session: %v", err)
	}

	if err := MaterialiseTemplateExercises(gdb, ownerID, run.ID, loaded); err != nil {
		t.Fatalf("materialise: %v", err)
	}

	var runEx Exercise
	if err := gdb.Where("owner_id = ? AND session_run_id = ?", ownerID, run.ID).
		First(&runEx).Error; err != nil {
		t.Fatalf("expected a materialised run exercise: %v", err)
	}
	if runEx.ID == tplEx.ID {
		t.Fatalf("materialised exercise reused the template id %d", tplEx.ID)
	}

	ticks, err := ListClimbingTicksByExercise(gdb, ownerID, run.ID, runEx.ID)
	if err != nil {
		t.Fatalf("list ticks: %v", err)
	}
	if len(ticks) != 1 {
		t.Fatalf("ticks on the materialised exercise = %d, want 1 (they were left on exercise %d)", len(ticks), tplEx.ID)
	}
	if ticks[0].Grade != "6b" {
		t.Errorf("tick grade = %q, want 6b", ticks[0].Grade)
	}

	var metaCount, setLogCount int64
	gdb.Model(&ClimbingExerciseMeta{}).
		Where("run_id = ? AND exercise_id = ?", run.ID, runEx.ID).Count(&metaCount)
	if metaCount != 1 {
		t.Errorf("climbing meta on materialised exercise = %d, want 1", metaCount)
	}
	gdb.Model(&ManualExerciseSetLog{}).
		Where("run_id = ? AND exercise_id = ?", run.ID, runEx.ID).Count(&setLogCount)
	if setLogCount != 1 {
		t.Errorf("set logs on materialised exercise = %d, want 1", setLogCount)
	}

	// Nothing should be left pointing at the template exercise for this run.
	var orphans int64
	gdb.Model(&ClimbingTick{}).
		Where("run_id = ? AND exercise_id = ?", run.ID, tplEx.ID).Count(&orphans)
	if orphans != 0 {
		t.Errorf("%d ticks still reference the template exercise", orphans)
	}
}

// RepairMaterialisedTickLinks fixes runs that were materialised before the fix
// landed. It must be idempotent, and must refuse to guess when a run has two
// exercises with the same name.
func TestRepairMaterialisedTickLinks(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "repair.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	gdb := store.DB
	const ownerID uint = 1

	newRun := func() SessionRun {
		tpl := SessionTemplate{OwnerID: ownerID, Name: "T"}
		gdb.Create(&tpl)
		ss := ScheduledSession{OwnerID: ownerID, ScheduledDate: time.Now(), SessionTemplateID: tpl.ID}
		gdb.Create(&ss)
		run := SessionRun{
			OwnerID: ownerID, ScheduledSessionID: ss.ID, Status: RunStatusCompleted,
			StartedAt: time.Now(), ExercisesMaterialised: true,
		}
		gdb.Create(&run)
		return run
	}

	// Run A: one orphaned tick, unambiguous name — should be repaired.
	runA := newRun()
	oldA := Exercise{OwnerID: ownerID, Name: "Easy Routes", Kind: "climbing"}
	gdb.Create(&oldA)
	runAID := runA.ID
	newA := Exercise{OwnerID: ownerID, SessionRunID: &runAID, Name: "Easy Routes", Kind: "climbing"}
	gdb.Create(&newA)
	gdb.Create(&ClimbingTick{OwnerID: ownerID, RunID: runA.ID, ExerciseID: oldA.ID, Kind: "sport", Grade: "6a"})

	// Run B: two run exercises share the name, so the orphan must be left alone.
	runB := newRun()
	oldB := Exercise{OwnerID: ownerID, Name: "Boulders", Kind: "climbing"}
	gdb.Create(&oldB)
	runBID := runB.ID
	gdb.Create(&Exercise{OwnerID: ownerID, SessionRunID: &runBID, Name: "Boulders", Kind: "climbing"})
	gdb.Create(&Exercise{OwnerID: ownerID, SessionRunID: &runBID, Name: "Boulders", Kind: "climbing"})
	gdb.Create(&ClimbingTick{OwnerID: ownerID, RunID: runB.ID, ExerciseID: oldB.ID, Kind: "boulder", Grade: "6c"})

	n, err := RepairMaterialisedTickLinks(gdb)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if n != 1 {
		t.Errorf("repaired rows = %d, want 1 (run B is ambiguous and must be skipped)", n)
	}

	ticks, err := ListClimbingTicksByExercise(gdb, ownerID, runA.ID, newA.ID)
	if err != nil {
		t.Fatalf("list ticks: %v", err)
	}
	if len(ticks) != 1 {
		t.Errorf("run A ticks on the materialised exercise = %d, want 1", len(ticks))
	}

	var ambiguousStillOld int64
	gdb.Model(&ClimbingTick{}).
		Where("run_id = ? AND exercise_id = ?", runB.ID, oldB.ID).Count(&ambiguousStillOld)
	if ambiguousStillOld != 1 {
		t.Errorf("ambiguous tick was moved; want it left on the original exercise")
	}

	// Idempotent: a second pass finds nothing.
	again, err := RepairMaterialisedTickLinks(gdb)
	if err != nil {
		t.Fatalf("second repair: %v", err)
	}
	if again != 0 {
		t.Errorf("second pass repaired %d rows, want 0", again)
	}
}
