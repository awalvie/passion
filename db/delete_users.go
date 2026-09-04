package db

import (
	"fmt"

	"gorm.io/gorm"
)

// Deleting an account is irreversible and touches nearly every table, so this lives in
// one place with one ordering. It exists because signup was open before invite codes
// landed, and an account that should not exist can hold catalog content licensed to
// someone else.

// ownerScopedTables lists every table carrying owner_id, with the model used to delete
// from it. Order does not matter here: the delete runs with deferred foreign keys, so
// consistency is checked once at commit rather than after each statement. That is the
// only sane way to remove a graph this tangled without hand-sorting twenty tables.
//
// A new model with an OwnerID must be added here, or deleting an account will leave its
// rows behind. There is a test that fails when this list falls behind the migrations.
func ownerScopedTables() []any {
	return []any{
		&ExerciseMedia{},
		&RunExerciseCompletion{},
		&RunExerciseChoice{},
		&ClimbingTick{},
		&ManualExerciseSetLog{},
		&ExercisePlannedSet{},
		&ClimbingExerciseMeta{},
		&SessionJournal{},
		&SessionRun{},
		&ScheduledSession{},
		&Exercise{},
		&Activity{},
		&ActivityTemplate{},
		&LibraryExercise{},
		&CycleExerciseOverride{},
		&CycleExerciseWeekOverride{},
		&TrainingCycleWeekdayMapping{},
		&CycleGoal{},
		&CalendarEvent{},
		&TrainingCycle{},
		&SessionTemplate{},
		&ClimbingVenue{},
		&ClimbingBoard{},
	}
}

// UserDeletionPlan describes what removing a set of accounts would do.
type UserDeletionPlan struct {
	KeepUserID   uint
	KeepEmail    string
	DeleteUsers  []User
	RowsByTable  map[string]int64
	TotalRows    int64
	InviteCodes  int64
	Applied      bool
	DeletedUsers int64
	// SharedRowsHeld counts catalog rows owned by an account marked for deletion. Any at
	// all stops the deletion: those rows are the app's catalog, read by everyone, and
	// removing an account must never take them.
	SharedRowsHeld int64
}

// catalogModels are the three that can carry Shared. A shared row belongs to the app's
// catalog, not to the account that happens to own the row, so account deletion must
// leave it alone.
func catalogModels() []any {
	return []any{&LibraryExercise{}, &ActivityTemplate{}, &SessionTemplate{}}
}

// countSharedHeldBy reports how many catalog rows the given accounts own.
func countSharedHeldBy(gdb *gorm.DB, ownerIDs []uint) (int64, error) {
	var total int64
	for _, model := range catalogModels() {
		var n int64
		if err := gdb.Unscoped().Model(model).
			Where("owner_id IN ? AND shared = ?", ownerIDs, true).Count(&n).Error; err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

// PlanDeleteAllUsersExcept reports which accounts would go and how many rows each table
// would lose. It changes nothing.
func PlanDeleteAllUsersExcept(gdb *gorm.DB, keepUserID uint) (UserDeletionPlan, error) {
	plan := UserDeletionPlan{KeepUserID: keepUserID, RowsByTable: map[string]int64{}}

	if keepUserID == 0 {
		return plan, fmt.Errorf("refusing to run with keep-user-id 0: that would delete every account")
	}
	var keep User
	if err := gdb.Where("id = ?", keepUserID).First(&keep).Error; err != nil {
		return plan, fmt.Errorf("the account to keep (id %d) does not exist, so nothing was touched: %w", keepUserID, err)
	}
	plan.KeepEmail = keep.Email

	if err := gdb.Where("id <> ?", keepUserID).Order("id").Find(&plan.DeleteUsers).Error; err != nil {
		return plan, err
	}
	if len(plan.DeleteUsers) == 0 {
		return plan, nil
	}

	victimIDs := make([]uint, 0, len(plan.DeleteUsers))
	for _, u := range plan.DeleteUsers {
		victimIDs = append(victimIDs, u.ID)
	}

	for _, model := range ownerScopedTables() {
		stmt := gorm.Statement{DB: gdb}
		if err := stmt.Parse(model); err != nil {
			return plan, err
		}
		table := stmt.Schema.Table

		var n int64
		if err := gdb.Unscoped().Table(table).Where("owner_id IN ?", victimIDs).Count(&n).Error; err != nil {
			return plan, err
		}
		if n > 0 {
			plan.RowsByTable[table] = n
			plan.TotalRows += n
		}
	}
	if err := gdb.Unscoped().Model(&InviteCode{}).
		Where("used_by_id IN ?", victimIDs).Count(&plan.InviteCodes).Error; err != nil {
		return plan, err
	}
	sharedHeld, err := countSharedHeldBy(gdb, victimIDs)
	if err != nil {
		return plan, err
	}
	plan.SharedRowsHeld = sharedHeld
	return plan, nil
}

// DeleteAllUsersExcept permanently removes every account except keepUserID, along with
// everything those accounts own. It is one transaction: it either all happens or none of
// it does. There is no undo, so the caller must have taken a backup.
func DeleteAllUsersExcept(gdb *gorm.DB, keepUserID uint) (UserDeletionPlan, error) {
	plan, err := PlanDeleteAllUsersExcept(gdb, keepUserID)
	if err != nil || len(plan.DeleteUsers) == 0 {
		return plan, err
	}

	// A deletion that would take catalog rows with it is refused rather than narrowed.
	// The alternative — skip the shared rows and delete the account anyway — leaves the
	// catalog owned by an account that no longer exists, which is worse and silent.
	if plan.SharedRowsHeld > 0 {
		return plan, fmt.Errorf(
			"refusing to delete: %d catalog row(s) are owned by an account marked for deletion. "+
				"Move them to the account you are keeping first, or keep that account instead",
			plan.SharedRowsHeld)
	}

	victimIDs := make([]uint, 0, len(plan.DeleteUsers))
	for _, u := range plan.DeleteUsers {
		victimIDs = append(victimIDs, u.ID)
	}

	err = gdb.Transaction(func(tx *gorm.DB) error {
		// Check foreign keys once, at commit, instead of after every statement. Without
		// this the deletes would have to be hand-sorted into child-before-parent order
		// across twenty-odd tables, and any future model would silently break that order.
		// Constraints are still enforced — a genuinely inconsistent result still fails.
		if err := tx.Exec("PRAGMA defer_foreign_keys = ON").Error; err != nil {
			return err
		}

		for _, model := range ownerScopedTables() {
			if err := tx.Unscoped().Where("owner_id IN ?", victimIDs).Delete(model).Error; err != nil {
				return err
			}
		}
		// A code redeemed by a deleted account goes with it. Leaving it would keep a row
		// pointing at a user that no longer exists, and clearing the claim instead would
		// quietly make a spent code usable again.
		if err := tx.Unscoped().Where("used_by_id IN ?", victimIDs).Delete(&InviteCode{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("created_by_id IN ?", victimIDs).
			Model(&InviteCode{}).Update("created_by_id", nil).Error; err != nil {
			return err
		}

		res := tx.Unscoped().Where("id IN ?", victimIDs).Delete(&User{})
		if res.Error != nil {
			return res.Error
		}
		plan.DeletedUsers = res.RowsAffected
		return nil
	})
	if err != nil {
		return plan, err
	}
	plan.Applied = true
	return plan, nil
}
