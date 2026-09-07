package db

import (
	"fmt"

	"gorm.io/gorm"
)

// A catalog written under an id with no account behind it is unreachable: every catalog
// read is owner-scoped, so nobody sees those rows, and the next import's prune treats them
// as orphaned. That is what the 4 September 2026 deploy produced — see docs/RECOVERY.md.
//
// This removes them. It takes an explicit owner id and never scans for "owners with no
// account": a scanning purge would be a loaded gun the day shared rows stop belonging to a
// real account.

// GhostPurgePlan says what a purge would remove, or did.
type GhostPurgePlan struct {
	OwnerID     uint
	RowsByTable map[string]int64
	TotalRows   int64
	// InviteCodes counts codes redeemed by the ghost id. Reported, never deleted:
	// InviteCode declares no foreign key, so foreign_key_check will not flag these, and a
	// used code is a record of a signup rather than catalog content.
	InviteCodes int64
	Applied     bool
	// Refusals is empty when the purge may proceed. Any entry stops it. Refusing rather
	// than narrowing is the same choice DeleteAllUsersExcept makes: a partial purge leaves
	// rows behind and makes the next state check unanswerable.
	Refusals []string
}

// PlanPurgeGhostCatalog counts what removing an owner's rows would take and collects every
// reason not to. It changes nothing.
func PlanPurgeGhostCatalog(gdb *gorm.DB, ownerID uint) (GhostPurgePlan, error) {
	plan := GhostPurgePlan{OwnerID: ownerID, RowsByTable: map[string]int64{}}

	if ownerID == 0 {
		plan.Refusals = append(plan.Refusals,
			"owner id 0 is the \"no owner\" sentinel across the data layer, not an account")
		return plan, nil
	}

	// The whole safety property, and deliberately Unscoped: a soft-deleted account could
	// be restored, so its rows are not unreachable in the way this command means.
	//
	// ImportYAML resolves the same id scoped, so it refuses a soft-deleted owner while
	// this refuses to purge it. That is not a contradiction — each errs towards not acting
	// on its own consequence, one declining to write and the other to delete — but an
	// operator reading "names no existing account" from the import and then "the account
	// exists" from here would reasonably think one of them is broken. So say which case
	// this is.
	var live, anyRow int64
	if err := gdb.Model(&User{}).Where("id = ?", ownerID).Count(&live).Error; err != nil {
		return plan, err
	}
	if err := gdb.Unscoped().Model(&User{}).Where("id = ?", ownerID).Count(&anyRow).Error; err != nil {
		return plan, err
	}
	switch {
	case live > 0:
		plan.Refusals = append(plan.Refusals, fmt.Sprintf(
			"user %d exists, so these are somebody's rows. This command only removes rows "+
				"owned by an id with no account behind it; use --delete-users-except to "+
				"remove an account", ownerID))
		return plan, nil
	case anyRow > 0:
		plan.Refusals = append(plan.Refusals, fmt.Sprintf(
			"user %d is soft-deleted, not gone. The import refuses this id because nobody "+
				"can log in as it, but the account row is still there and could be "+
				"restored, so its rows are not orphaned. Decide which you want: restore "+
				"the account, or remove it with --delete-users-except and run this again",
			ownerID))
		return plan, nil
	}

	for _, model := range ownerScopedTables() {
		table, err := tableNameOf(gdb, model)
		if err != nil {
			return plan, err
		}
		var n int64
		if err := gdb.Unscoped().Table(table).Where("owner_id = ?", ownerID).Count(&n).Error; err != nil {
			return plan, err
		}
		if n > 0 {
			plan.RowsByTable[table] = n
			plan.TotalRows += n
		}
	}
	if err := gdb.Unscoped().Model(&InviteCode{}).
		Where("used_by_id = ?", ownerID).Count(&plan.InviteCodes).Error; err != nil {
		return plan, err
	}

	shared, err := countSharedHeldBy(gdb, []uint{ownerID})
	if err != nil {
		return plan, err
	}
	if shared > 0 {
		plan.Refusals = append(plan.Refusals, fmt.Sprintf(
			"%d row(s) are flagged shared, which means the app's catalog is owned here. "+
				"Republish it under a real account first (--unpublish-catalog then "+
				"--publish-catalog)", shared))
	}

	// None of the six history tables declares a foreign key to exercises, so nothing in
	// the database would stop a delete stranding a completed session's records.
	var history int64
	if err := gdb.Unscoped().Model(&Exercise{}).
		Where("owner_id = ?", ownerID).
		Where(referencedByHistoryWhere).Count(&history).Error; err != nil {
		return plan, err
	}
	if history > 0 {
		plan.Refusals = append(plan.Refusals, fmt.Sprintf(
			"%d exercise(s) are still referenced by a completed session's records. These "+
				"are not unreachable catalog rows — something ran them", history))
	}

	inbound, err := countInboundReferences(gdb, ownerID)
	if err != nil {
		return plan, err
	}
	plan.Refusals = append(plan.Refusals, inbound...)

	return plan, nil
}

// countInboundReferences finds rows owned by somebody else that point at this owner's
// rows. Every one of these would become a dangling id, and the ones through exercises and
// library exercises are the shapes that actually occurred in production.
func countInboundReferences(gdb *gorm.DB, ownerID uint) ([]string, error) {
	checks := []struct {
		what string
		sql  string
	}{
		{"exercises pointing at its library exercises", `
			SELECT COUNT(*) FROM exercises e JOIN library_exercises l
			  ON l.id = e.library_exercise_id
			WHERE e.owner_id <> ? AND l.owner_id = ?`},
		{"cycle overrides pointing at its library exercises", `
			SELECT COUNT(*) FROM cycle_exercise_overrides o JOIN library_exercises l
			  ON l.id = o.library_exercise_id
			WHERE o.owner_id <> ? AND l.owner_id = ?`},
		{"cycle week overrides pointing at its library exercises", `
			SELECT COUNT(*) FROM cycle_exercise_week_overrides o JOIN library_exercises l
			  ON l.id = o.library_exercise_id
			WHERE o.owner_id <> ? AND l.owner_id = ?`},
		{"library exercises whose parent it owns", `
			SELECT COUNT(*) FROM library_exercises c JOIN library_exercises p
			  ON p.id = c.parent_library_exercise_id
			WHERE c.owner_id <> ? AND p.owner_id = ?`},
		{"scheduled sessions pointing at its session templates", `
			SELECT COUNT(*) FROM scheduled_sessions s JOIN session_templates t
			  ON t.id = s.session_template_id
			WHERE s.owner_id <> ? AND t.owner_id = ?`},
		{"cycle weekday mappings pointing at its session templates", `
			SELECT COUNT(*) FROM training_cycle_weekday_mappings m JOIN session_templates t
			  ON t.id = m.session_template_id
			WHERE m.owner_id <> ? AND t.owner_id = ?`},
		{"exercises pointing at its activity templates", `
			SELECT COUNT(*) FROM exercises e JOIN activity_templates a
			  ON a.id = e.activity_template_id
			WHERE e.owner_id <> ? AND a.owner_id = ?`},
	}
	var out []string
	for _, c := range checks {
		var n int64
		if err := gdb.Raw(c.sql, ownerID, ownerID).Scan(&n).Error; err != nil {
			return nil, err
		}
		if n > 0 {
			out = append(out, fmt.Sprintf("%d %s", n, c.what))
		}
	}
	return out, nil
}

// PurgeGhostCatalog permanently removes every row owned by ownerID, after checking that
// nothing is left pointing at them. One transaction: all of it or none.
//
// Hard deletes, not soft. A soft delete of a parent does not cascade, so soft-deleting the
// activity templates and activities here would leave their exercises and media live and
// unreachable — creating the orphan bug rather than clearing it. A soft-deleted row also
// keeps occupying its (owner_id, slug) identity.
//
// The way back is the backup file. There is no finer undo, and the caller says so.
func PurgeGhostCatalog(gdb *gorm.DB, ownerID uint) (GhostPurgePlan, error) {
	plan, err := PlanPurgeGhostCatalog(gdb, ownerID)
	if err != nil {
		return plan, err
	}
	if len(plan.Refusals) > 0 {
		return plan, fmt.Errorf("refusing to purge owner %d: %s", ownerID, plan.Refusals[0])
	}
	if plan.TotalRows == 0 {
		return plan, nil
	}

	err = gdb.Transaction(func(tx *gorm.DB) error {
		// Foreign keys checked once at commit rather than after each statement, so the
		// deletes need no hand-sorted child-before-parent order across twenty-odd tables.
		// Constraints still hold: a genuinely inconsistent result still fails.
		if err := tx.Exec("PRAGMA defer_foreign_keys = ON").Error; err != nil {
			return err
		}
		for _, model := range ownerScopedTables() {
			if err := tx.Unscoped().Where("owner_id = ?", ownerID).Delete(model).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return plan, err
	}
	plan.Applied = true
	return plan, nil
}

// tableNameOf resolves a model's table name through GORM's schema parser, so the purge
// reports the same names the migrations use.
func tableNameOf(gdb *gorm.DB, model any) (string, error) {
	stmt := gorm.Statement{DB: gdb}
	if err := stmt.Parse(model); err != nil {
		return "", err
	}
	return stmt.Schema.Table, nil
}
