package db

import (
	"gorm.io/gorm"
)

// The importer used to soft-delete a session template's activities without touching their
// exercises. A soft delete is an UPDATE, so the ON DELETE CASCADE never fired and the
// child rows stayed live and unreachable. That ran for every user on every restart. The
// bug is fixed in upsertSessionTemplate, but the rows it already made are still here.
//
// This purge removes them. It is a command the owner runs and watches. It is never a boot
// step, because it hard-deletes and the database holds his real training history.

// orphanExerciseWhere matches a live exercise whose activity is soft-deleted or gone.
// Both cases are the same defect and the same clause covers them.
const orphanExerciseWhere = `
	exercises.deleted_at IS NULL
	AND exercises.activity_id IS NOT NULL
	AND exercises.activity_id NOT IN (SELECT id FROM activities WHERE deleted_at IS NULL)`

// referencedByHistoryWhere matches an exercise that some record of a completed session
// still points at. None of these six tables declares a foreign key to exercises, so the
// database will not stop a delete that strands them. This clause is the only guard.
//
// Their own deleted_at is deliberately not filtered. A soft-deleted completion is still a
// record of something the athlete did, and this purge must not be the thing that decides
// otherwise.
const referencedByHistoryWhere = `(
	   EXISTS (SELECT 1 FROM run_exercise_completions c WHERE c.exercise_id = exercises.id)
	OR EXISTS (SELECT 1 FROM climbing_ticks t WHERE t.exercise_id = exercises.id)
	OR EXISTS (SELECT 1 FROM manual_exercise_set_logs l WHERE l.exercise_id = exercises.id)
	OR EXISTS (SELECT 1 FROM exercise_planned_sets p WHERE p.exercise_id = exercises.id)
	OR EXISTS (SELECT 1 FROM run_exercise_choices ch
	           WHERE ch.parent_exercise_id = exercises.id OR ch.chosen_exercise_id = exercises.id)
	OR EXISTS (SELECT 1 FROM climbing_exercise_meta m WHERE m.exercise_id = exercises.id)
)`

// PurgeReport says what a purge found and what it removed. On a dry run the Deleted
// counts are zero and the rest still describe the database as it stands.
type PurgeReport struct {
	LiveExercisesBefore int64
	OrphanCandidates    int64
	KeptForHistory      int64
	SafeToPurge         int64

	DeletedExercises  int64
	DeletedActivities int64
	DryRun            bool
}

// CountOrphanedExercises measures the damage without changing anything.
func CountOrphanedExercises(gdb *gorm.DB) (PurgeReport, error) {
	var rep PurgeReport
	rep.DryRun = true

	counts := []struct {
		into *int64
		cond string
	}{
		{&rep.LiveExercisesBefore, "exercises.deleted_at IS NULL"},
		{&rep.OrphanCandidates, orphanExerciseWhere},
		{&rep.KeptForHistory, orphanExerciseWhere + " AND " + referencedByHistoryWhere},
		{&rep.SafeToPurge, orphanExerciseWhere + " AND NOT " + referencedByHistoryWhere},
	}
	for _, c := range counts {
		if err := gdb.Model(&Exercise{}).Where(c.cond).Count(c.into).Error; err != nil {
			return rep, err
		}
	}
	return rep, nil
}

// PurgeOrphanedExercises removes orphaned exercises that no run history references, then
// removes the soft-deleted activities left with no children at all. Pass dryRun to get
// the counts without touching anything.
func PurgeOrphanedExercises(gdb *gorm.DB, dryRun bool) (PurgeReport, error) {
	rep, err := CountOrphanedExercises(gdb)
	if err != nil {
		return rep, err
	}
	rep.DryRun = dryRun
	if dryRun || rep.SafeToPurge == 0 {
		return rep, nil
	}

	err = gdb.Transaction(func(tx *gorm.DB) error {
		// Exercise.ParentExerciseID is a real foreign key back to exercises, and it is
		// NO ACTION, not a cascade. SQLite checks foreign keys at the end of each
		// statement, so a parent and its children survive removal only when they go in
		// the *same* statement. Splitting this into batches fails with "FOREIGN KEY
		// constraint failed" the moment a batch boundary lands between the two — of the
		// 118,059 orphans in the owner's database, 44,513 are somebody's parent.
		//
		// So: one statement, driven by the predicate rather than by a list of ids. That
		// also sidesteps the bound-parameter limit an id list of this size would hit.
		// ExerciseMedia follows through its own real cascade.
		res := tx.Unscoped().
			Where("id IN (?)",
				tx.Session(&gorm.Session{NewDB: true}).Model(&Exercise{}).
					Select("exercises.id").
					Where(orphanExerciseWhere+" AND NOT "+referencedByHistoryWhere)).
			Delete(&Exercise{})
		if res.Error != nil {
			return res.Error
		}
		rep.DeletedExercises = res.RowsAffected
		if rep.DeletedExercises == 0 {
			return nil
		}

		// Now the activities those exercises hung off. Only ones with nothing left: an
		// activity that still holds a kept exercise would take it down with it, because
		// activities → exercises is a real cascade.
		res = tx.Unscoped().Where(`
			deleted_at IS NOT NULL
			AND id NOT IN (SELECT activity_id FROM exercises WHERE activity_id IS NOT NULL)`).
			Delete(&Activity{})
		if res.Error != nil {
			return res.Error
		}
		rep.DeletedActivities = res.RowsAffected
		return nil
	})
	return rep, err
}
