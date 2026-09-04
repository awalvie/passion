package db

import (
	"sort"

	"gorm.io/gorm"
)

// Runs logged before the copy moved to run start still point at template rows. The
// importer rewrites those rows on every restart, so the log editor and the history page
// render a past session from whatever the catalog says today — or from nothing, once the
// rows were retired.
//
// This walks each past run and gives it its own copy of exactly the exercises its own
// records point at. It reads the source row by id, not through the template graph: the
// graph is what changed, and following it is how run 40 ended up with fifteen rows of
// zeros and none of its thirteen completions attached.

// referencesExerciseWhere matches an exercise that some record still points at. None of
// these tables declares a foreign key to exercises, so nothing at the database level
// stops a delete from stranding them.
const referencesExerciseWhere = `(
	   EXISTS (SELECT 1 FROM run_exercise_completions c WHERE c.exercise_id = exercises.id)
	OR EXISTS (SELECT 1 FROM climbing_ticks t           WHERE t.exercise_id = exercises.id)
	OR EXISTS (SELECT 1 FROM manual_exercise_set_logs l WHERE l.exercise_id = exercises.id)
	OR EXISTS (SELECT 1 FROM exercise_planned_sets p    WHERE p.exercise_id = exercises.id)
	OR EXISTS (SELECT 1 FROM run_exercise_choices ch
	           WHERE ch.parent_exercise_id = exercises.id OR ch.chosen_exercise_id = exercises.id)
	OR EXISTS (SELECT 1 FROM climbing_exercise_meta m   WHERE m.exercise_id = exercises.id)
	OR EXISTS (SELECT 1 FROM exercises k WHERE k.parent_exercise_id = exercises.id)
)`

// BackfillReport says what the backfill found and what it changed.
type BackfillReport struct {
	RunsExamined int
	RunsChanged  int
	Copied       int
	EmptyRemoved int
	DryRun       bool
}

// referencedExerciseIDs returns every exercise id the run's own records point at.
func referencedExerciseIDs(tx *gorm.DB, ownerID, runID uint) ([]uint, error) {
	seen := map[uint]struct{}{}
	add := func(ids []uint) {
		for _, id := range ids {
			if id != 0 {
				seen[id] = struct{}{}
			}
		}
	}
	for _, spec := range []struct {
		table string
		col   string
	}{
		{"run_exercise_completions", "exercise_id"},
		{"climbing_ticks", "exercise_id"},
		{"manual_exercise_set_logs", "exercise_id"},
		{"climbing_exercise_meta", "exercise_id"},
		{"run_exercise_choices", "parent_exercise_id"},
		{"run_exercise_choices", "chosen_exercise_id"},
	} {
		var ids []uint
		if err := tx.Table(spec.table).
			Where("owner_id = ? AND run_id = ?", ownerID, runID).
			Pluck(spec.col, &ids).Error; err != nil {
			return nil, err
		}
		add(ids)
	}
	out := make([]uint, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// blockFor finds the activity a template exercise came from, so the run's copy keeps its
// grouping. Unscoped, because the activity is usually soft-deleted by now — that is the
// whole reason this backfill exists.
//
// Many are gone outright: prune hard-deleted the activities left childless by the old
// importer, so about half the rows this backfill copies have no block to recover. Those
// fall back to a plain main block rather than an unnamed one, so the run still renders as
// a session instead of a single nameless group.
func blockFor(tx *gorm.DB, ex Exercise) Activity {
	fallback := Activity{Type: "activity"}
	if ex.ActivityID == nil {
		return fallback
	}
	var act Activity
	if err := tx.Unscoped().Where("id = ?", *ex.ActivityID).First(&act).Error; err != nil {
		return fallback
	}
	if act.Type == "" {
		act.Type = fallback.Type
	}
	return act
}

// BackfillRunExercises gives every past run its own copy of the exercises its records
// point at. Open sessions and manual entries are skipped: their exercises already belong
// to the run. Pass dryRun to see the counts without changing anything.
func BackfillRunExercises(gdb *gorm.DB, dryRun bool) (BackfillReport, error) {
	rep := BackfillReport{DryRun: dryRun}

	var runs []SessionRun
	if err := gdb.Where("is_open = ? AND is_manual = ?", false, false).
		Order("id").Find(&runs).Error; err != nil {
		return rep, err
	}
	rep.RunsExamined = len(runs)

	for _, run := range runs {
		changed := false
		err := gdb.Transaction(func(tx *gorm.DB) error {
			// Rows the old lazy copy created and then left unattached. Only ever the ones
			// nothing points at, so a real record can never be removed.
			var empties []uint
			if err := tx.Model(&Exercise{}).
				Where("owner_id = ? AND session_run_id = ? AND NOT "+referencesExerciseWhere, run.OwnerID, run.ID).
				Pluck("exercises.id", &empties).Error; err != nil {
				return err
			}
			if len(empties) > 0 {
				rep.EmptyRemoved += len(empties)
				changed = true
				if !dryRun {
					if err := tx.Unscoped().Where("id IN ?", empties).Delete(&Exercise{}).Error; err != nil {
						return err
					}
				}
			}

			ids, err := referencedExerciseIDs(tx, run.OwnerID, run.ID)
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				return nil
			}

			// Unscoped, and by id: the source is often a retired row, and the template
			// graph no longer reaches it.
			var sources []Exercise
			if err := tx.Unscoped().Where("id IN ?", ids).Find(&sources).Error; err != nil {
				return err
			}
			// Parents before their options, so a child can point at the new parent.
			sort.SliceStable(sources, func(i, j int) bool {
				pi := sources[i].ParentExerciseID != nil
				pj := sources[j].ParentExerciseID != nil
				if pi != pj {
					return !pi
				}
				return sources[i].OrderIndex < sources[j].OrderIndex
			})

			newIDFor := map[uint]uint{}
			order := 0
			for _, src := range sources {
				// Already the run's own row: nothing to copy.
				if src.SessionRunID != nil && *src.SessionRunID == run.ID {
					newIDFor[src.ID] = src.ID
					continue
				}
				var parent *uint
				if src.ParentExerciseID != nil {
					if mapped, ok := newIDFor[*src.ParentExerciseID]; ok {
						parent = &mapped
					}
				}
				rep.Copied++
				changed = true
				if dryRun {
					continue
				}
				copyRow := snapshotExercise(src, blockFor(tx, src), run.OwnerID, run.ID, order, parent)
				if err := tx.Create(&copyRow).Error; err != nil {
					return err
				}
				newIDFor[src.ID] = copyRow.ID
				if err := repointRunRows(tx, run.OwnerID, run.ID, src.ID, copyRow.ID); err != nil {
					return err
				}
				order++
			}

			if dryRun {
				return nil
			}
			return tx.Model(&SessionRun{}).
				Where("id = ?", run.ID).
				Update("exercises_materialised", true).Error
		})
		if err != nil {
			return rep, err
		}
		if changed {
			rep.RunsChanged++
		}
	}
	return rep, nil
}
