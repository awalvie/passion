package db

import (
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"
)

// seedAccountWithData creates a user and a row in every owner-scoped table, so a deletion
// test can prove that nothing of theirs is left behind.
func seedAccountWithData(t *testing.T, store *Store, email string) uint {
	t.Helper()
	u := User{Email: email, PasswordHash: "x"}
	mustCreate(t, store, &u)
	id := u.ID

	tpl := SessionTemplate{OwnerID: id, Name: "Session " + email}
	mustCreate(t, store, &tpl)
	act := Activity{OwnerID: id, SessionTemplateID: tpl.ID, Type: "activity"}
	mustCreate(t, store, &act)
	ex := Exercise{OwnerID: id, ActivityID: &act.ID, Name: "Pull-ups"}
	mustCreate(t, store, &ex)
	mustCreate(t, store, &ExerciseMedia{OwnerID: id, ExerciseID: &ex.ID, VideoURL: "https://example.com/v"})
	mustCreate(t, store, &ExercisePlannedSet{OwnerID: id, ExerciseID: ex.ID, SetIndex: 1, Reps: 5})

	lib := LibraryExercise{OwnerID: id, Name: "Library " + email}
	mustCreate(t, store, &lib)
	at := ActivityTemplate{OwnerID: id, Name: "Block " + email}
	mustCreate(t, store, &at)

	cyc := TrainingCycle{OwnerID: id, Name: "Cycle", StartDate: time.Now(), Weeks: 4}
	mustCreate(t, store, &cyc)
	mustCreate(t, store, &TrainingCycleWeekdayMapping{OwnerID: id, TrainingCycleID: cyc.ID, Weekday: 1, SessionTemplateID: tpl.ID})
	mustCreate(t, store, &CycleGoal{OwnerID: id, TrainingCycleID: cyc.ID, Before: "a", After: "b"})
	mustCreate(t, store, &CycleExerciseOverride{OwnerID: id, TrainingCycleID: cyc.ID, ExerciseName: "Pull-ups"})
	mustCreate(t, store, &CycleExerciseWeekOverride{OwnerID: id, TrainingCycleID: cyc.ID, Week: 1, ExerciseName: "Pull-ups"})
	mustCreate(t, store, &CalendarEvent{OwnerID: id, Title: "Trip", Kind: "trip", StartDate: time.Now(), EndDate: time.Now(), TrainingCycleID: &cyc.ID})

	ss := ScheduledSession{OwnerID: id, TrainingCycleID: &cyc.ID, ScheduledDate: time.Now(), SessionTemplateID: tpl.ID}
	mustCreate(t, store, &ss)
	run := SessionRun{OwnerID: id, ScheduledSessionID: ss.ID, StartedAt: time.Now()}
	mustCreate(t, store, &run)
	mustCreate(t, store, &RunExerciseCompletion{OwnerID: id, RunID: run.ID, ExerciseID: ex.ID, CompletedAt: time.Now()})
	mustCreate(t, store, &RunExerciseChoice{OwnerID: id, RunID: run.ID, ParentExerciseID: ex.ID, ChosenExerciseID: ex.ID})
	mustCreate(t, store, &ManualExerciseSetLog{OwnerID: id, RunID: run.ID, ExerciseID: ex.ID, SetIndex: 1})
	mustCreate(t, store, &ClimbingExerciseMeta{OwnerID: id, RunID: run.ID, ExerciseID: ex.ID, Type: "board"})
	mustCreate(t, store, &ClimbingTick{OwnerID: id, RunID: run.ID, ExerciseID: ex.ID, Kind: "boulder"})
	mustCreate(t, store, &SessionJournal{OwnerID: id, RunID: &run.ID, Date: time.Now()})

	mustCreate(t, store, &ClimbingVenue{OwnerID: id, Name: "Gym", Kind: "commercial"})
	mustCreate(t, store, &ClimbingBoard{OwnerID: id, BoardType: "kilter", Name: "Home"})
	return id
}

func countOwned(t *testing.T, store *Store, ownerID uint) int64 {
	t.Helper()
	var total int64
	for _, model := range ownerScopedTables() {
		var n int64
		if err := store.DB.Unscoped().Model(model).Where("owner_id = ?", ownerID).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		total += n
	}
	return total
}

func TestDeleteAllUsersExceptRemovesEverythingTheOthersOwn(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "deluser.db"))
	if err != nil {
		t.Fatal(err)
	}
	keepID := seedAccountWithData(t, store, "keep@example.com")
	goneID := seedAccountWithData(t, store, "gone@example.com")

	keepBefore := countOwned(t, store, keepID)
	if keepBefore == 0 || countOwned(t, store, goneID) == 0 {
		t.Fatal("the fixture created no owned rows")
	}

	plan, err := DeleteAllUsersExcept(store.DB, keepID)
	if err != nil {
		t.Fatalf("deletion failed: %v", err)
	}
	if plan.DeletedUsers != 1 {
		t.Fatalf("want 1 account deleted, got %d", plan.DeletedUsers)
	}

	if left := countOwned(t, store, goneID); left != 0 {
		t.Fatalf("%d rows of the deleted account survived", left)
	}
	if after := countOwned(t, store, keepID); after != keepBefore {
		t.Fatalf("the kept account lost rows: %d before, %d after", keepBefore, after)
	}
	var users int64
	if err := store.DB.Unscoped().Model(&User{}).Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if users != 1 {
		t.Fatalf("want 1 account left, got %d", users)
	}
}

// Keeping an account that does not exist would delete everything. Refuse instead.
func TestDeleteAllUsersExceptRefusesAnUnknownOrZeroKeepID(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "deluser-guard.db"))
	if err != nil {
		t.Fatal(err)
	}
	keepID := seedAccountWithData(t, store, "keep@example.com")

	for _, badID := range []uint{0, 9999} {
		if _, err := DeleteAllUsersExcept(store.DB, badID); err == nil {
			t.Fatalf("deletion should refuse keep-id %d", badID)
		}
	}
	if countOwned(t, store, keepID) == 0 {
		t.Fatal("a refused deletion still removed data")
	}
}

func TestPlanDeleteAllUsersExceptChangesNothing(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "deluser-plan.db"))
	if err != nil {
		t.Fatal(err)
	}
	keepID := seedAccountWithData(t, store, "keep@example.com")
	goneID := seedAccountWithData(t, store, "gone@example.com")
	before := countOwned(t, store, goneID)

	plan, err := PlanDeleteAllUsersExcept(store.DB, keepID)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.DeleteUsers) != 1 || plan.DeleteUsers[0].ID != goneID {
		t.Fatalf("plan should name exactly the other account, got %+v", plan.DeleteUsers)
	}
	if plan.TotalRows != before {
		t.Fatalf("plan counted %d rows, the account owns %d", plan.TotalRows, before)
	}
	if plan.Applied {
		t.Fatal("planning must not apply anything")
	}
	if countOwned(t, store, goneID) != before {
		t.Fatal("planning removed rows")
	}
}

// An invite code redeemed by a deleted account must go with it. Leaving the row would
// point at a user that no longer exists, and clearing the claim would quietly make a
// spent code work again.
func TestDeleteAllUsersExceptRemovesTheirInviteCodes(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "deluser-invite.db"))
	if err != nil {
		t.Fatal(err)
	}
	keepID := seedAccountWithData(t, store, "keep@example.com")
	goneID := seedAccountWithData(t, store, "gone@example.com")

	theirs, err := CreateInviteCode(store.DB, nil, "theirs", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := RedeemInviteCode(store.DB, theirs.Code, goneID, time.Now()); err != nil {
		t.Fatal(err)
	}
	unused, err := CreateInviteCode(store.DB, nil, "still open", nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := DeleteAllUsersExcept(store.DB, keepID); err != nil {
		t.Fatal(err)
	}

	var n int64
	if err := store.DB.Unscoped().Model(&InviteCode{}).
		Where("code = ?", NormaliseInviteCode(theirs.Code)).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("the deleted account's invite code is still there")
	}
	if err := store.DB.Model(&InviteCode{}).
		Where("code = ?", NormaliseInviteCode(unused.Code)).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("an unrelated unused code was removed")
	}
}

// ownerScopedTables is written by hand, so it can fall behind the models. This compares it
// against every table the migrations actually created with an owner_id column. A new model
// with an OwnerID fails here instead of silently leaving rows behind on every deletion.
func TestOwnerScopedTablesCoversEveryTableWithAnOwner(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "deluser-cover.db"))
	if err != nil {
		t.Fatal(err)
	}

	listed := map[string]bool{}
	for _, model := range ownerScopedTables() {
		stmt := gorm.Statement{DB: store.DB}
		if err := stmt.Parse(model); err != nil {
			t.Fatal(err)
		}
		listed[stmt.Schema.Table] = true
	}

	var tables []string
	if err := store.DB.Raw(`
		SELECT m.name FROM sqlite_master m
		WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'
		  AND EXISTS (SELECT 1 FROM pragma_table_info(m.name) c WHERE c.name = 'owner_id')
		ORDER BY m.name`).Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	if len(tables) == 0 {
		t.Fatal("found no owner-scoped tables at all — the query is wrong")
	}

	for _, table := range tables {
		if !listed[table] {
			t.Errorf("table %q has an owner_id but is missing from ownerScopedTables(); "+
				"deleting an account would leave its rows behind", table)
		}
	}
}
