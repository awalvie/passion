package db

import (
	"path/filepath"
	"strings"
	"testing"
)

// The 4 September 2026 incident had two halves, and the delete-users guard covers the
// first one: the account the importer writes into was deleted while yaml_import.owner_id
// still pointed at it. SharedRowsHeld did not catch it, because a catalog that has not
// been published holds no shared rows, so the deletion looked harmless.
//
// These tests fix the shape of that half. The other half — an owner id that is already
// gone by the time the import runs — is guarded in ImportYAML and covered in
// import_owner_test.go.

// seedFourAccounts mirrors the production account history: user 1 is the original, user 4
// is the one in use. Returns the ids in order.
func seedFourAccounts(t *testing.T, store *Store) []uint {
	t.Helper()
	ids := make([]uint, 0, 4)
	for _, email := range []string{"first@example.com", "second@example.com", "third@example.com", "current@example.com"} {
		ids = append(ids, seedAccountWithData(t, store, email))
	}
	if ids[0] != 1 {
		t.Fatalf("fixture assumes the first account is id 1, got %d", ids[0])
	}
	return ids
}

func countAllUsers(t *testing.T, store *Store) int64 {
	t.Helper()
	var n int64
	if err := store.DB.Unscoped().Model(&User{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// This is the test that would have caught the incident. Keep user 4, delete the rest,
// with the import owner still pointing at user 1.
func TestDeleteRefusesWhenAVictimIsTheImportOwner(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "victim.db"))
	if err != nil {
		t.Fatal(err)
	}
	ids := seedFourAccounts(t, store)
	importOwner, keepID := ids[0], ids[3]

	usersBefore := countAllUsers(t, store)
	ownedBefore := countOwned(t, store, importOwner)
	if ownedBefore == 0 {
		t.Fatal("the fixture created no rows for the import owner")
	}

	plan, err := DeleteAllUsersExcept(store.DB, keepID, importOwner)
	if err == nil {
		t.Fatal("deletion should be refused while the import owner is among the victims")
	}
	if !strings.Contains(err.Error(), "yaml import owner") {
		t.Errorf("the error should name the cause, got: %v", err)
	}
	if plan.ImportOwnerVictim != importOwner {
		t.Errorf("plan.ImportOwnerVictim = %d, want %d", plan.ImportOwnerVictim, importOwner)
	}
	if plan.Applied {
		t.Error("a refused deletion reported Applied")
	}

	// A guard that refuses after deleting is not a guard.
	if after := countAllUsers(t, store); after != usersBefore {
		t.Errorf("a refused deletion removed accounts: %d users before, %d after", usersBefore, after)
	}
	if after := countOwned(t, store, importOwner); after != ownedBefore {
		t.Errorf("a refused deletion removed the import owner's rows: %d before, %d after", ownedBefore, after)
	}
	for _, id := range ids {
		if countOwned(t, store, id) == 0 {
			t.Errorf("a refused deletion emptied account %d", id)
		}
	}
}

func TestPlanFlagsTheImportOwnerVictimWithoutChangingAnything(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "victimplan.db"))
	if err != nil {
		t.Fatal(err)
	}
	ids := seedFourAccounts(t, store)
	importOwner, keepID := ids[0], ids[3]
	before := countOwned(t, store, importOwner)

	plan, err := PlanDeleteAllUsersExcept(store.DB, keepID, importOwner)
	if err != nil {
		t.Fatalf("planning should not fail: %v", err)
	}
	if plan.ImportOwnerVictim != importOwner {
		t.Errorf("plan.ImportOwnerVictim = %d, want %d", plan.ImportOwnerVictim, importOwner)
	}
	if plan.Applied {
		t.Error("a plan reported Applied")
	}
	if after := countOwned(t, store, importOwner); after != before {
		t.Errorf("planning changed the database: %d rows before, %d after", before, after)
	}
}

// The import owner being the account you are keeping is the fixed state, not a problem.
// Without this the guard would be indistinguishable from refusing every deletion.
func TestDeleteProceedsWhenTheImportOwnerIsTheAccountKept(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "victimkeep.db"))
	if err != nil {
		t.Fatal(err)
	}
	keepID := seedAccountWithData(t, store, "keep@example.com")
	goneID := seedAccountWithData(t, store, "gone@example.com")

	plan, err := DeleteAllUsersExcept(store.DB, keepID, keepID)
	if err != nil {
		t.Fatalf("deletion should proceed when the import owner is the kept account: %v", err)
	}
	if plan.ImportOwnerVictim != 0 {
		t.Errorf("plan.ImportOwnerVictim = %d, want 0: the import owner is not a victim", plan.ImportOwnerVictim)
	}
	if !plan.Applied {
		t.Error("the deletion did not apply")
	}
	if left := countOwned(t, store, goneID); left != 0 {
		t.Errorf("the other account still owns %d row(s)", left)
	}
}

// Every pre-existing test passes importOwnerID 0, meaning "no import owner configured".
// User ids start at 1, so 0 can never name a real account — but the guard is an equality
// test against every victim id, so this pins that 0 is genuinely inert rather than
// accidentally matching.
func TestImportOwnerZeroNeverMatchesAVictim(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "victimzero.db"))
	if err != nil {
		t.Fatal(err)
	}
	firstID := seedAccountWithData(t, store, "first@example.com")
	keepID := seedAccountWithData(t, store, "keep@example.com")
	if firstID != 1 {
		t.Fatalf("fixture assumes the first account is id 1, got %d", firstID)
	}

	plan, err := DeleteAllUsersExcept(store.DB, keepID, 0)
	if err != nil {
		t.Fatalf("importOwnerID 0 should not block a deletion: %v", err)
	}
	if plan.ImportOwnerVictim != 0 {
		t.Errorf("plan.ImportOwnerVictim = %d, want 0 for an unconfigured import owner", plan.ImportOwnerVictim)
	}
	if !plan.Applied {
		t.Error("the deletion did not apply")
	}
	if left := countOwned(t, store, firstID); left != 0 {
		t.Errorf("user 1 should have been deleted, still owns %d row(s)", left)
	}
}

// Both refusals can be true at once. The import-owner one is reported first, and both
// counts are on the plan so the operator sees the whole picture in one dry run.
func TestTheImportOwnerGuardIsReportedBeforeSharedRows(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "victimorder.db"))
	if err != nil {
		t.Fatal(err)
	}
	importOwner := seedAccountWithData(t, store, "owner@example.com")
	keepID := seedAccountWithData(t, store, "keep@example.com")
	mustCreate(t, store, &LibraryExercise{
		OwnerID: importOwner, Name: "Shared Max Hangs", Slug: "max_hangs", Shared: true,
	})

	plan, err := PlanDeleteAllUsersExcept(store.DB, keepID, importOwner)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ImportOwnerVictim != importOwner {
		t.Errorf("plan.ImportOwnerVictim = %d, want %d", plan.ImportOwnerVictim, importOwner)
	}
	if plan.SharedRowsHeld != 1 {
		t.Errorf("plan.SharedRowsHeld = %d, want 1: both problems should be reported", plan.SharedRowsHeld)
	}

	_, err = DeleteAllUsersExcept(store.DB, keepID, importOwner)
	if err == nil {
		t.Fatal("the deletion should be refused")
	}
	if !strings.Contains(err.Error(), "yaml import owner") {
		t.Errorf("the import-owner refusal should come first, got: %v", err)
	}
	var still int64
	if err := store.DB.Model(&LibraryExercise{}).Where("shared = ?", true).Count(&still).Error; err != nil {
		t.Fatal(err)
	}
	if still != 1 {
		t.Errorf("the catalog row is gone: %d shared rows left", still)
	}
}

// An import owner whose account is already gone is not a victim, so it is not a refusal —
// there is nothing left to protect. It is reported as a NOTE instead, because this plan is
// what an operator reads while recovering from exactly that misconfiguration.
func TestPlanNotesAnImportOwnerThatNoAccountHas(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "victimgone.db"))
	if err != nil {
		t.Fatal(err)
	}
	keepID := seedAccountWithData(t, store, "keep@example.com")
	seedAccountWithData(t, store, "other@example.com")

	const neverExisted uint = 4242
	plan, err := PlanDeleteAllUsersExcept(store.DB, keepID, neverExisted)
	if err != nil {
		t.Fatal(err)
	}
	if plan.ImportOwnerMissing != neverExisted {
		t.Errorf("plan.ImportOwnerMissing = %d, want %d", plan.ImportOwnerMissing, neverExisted)
	}
	if plan.ImportOwnerVictim != 0 {
		t.Errorf("plan.ImportOwnerVictim = %d, want 0: id %d owns no account", plan.ImportOwnerVictim, neverExisted)
	}
	if _, err := DeleteAllUsersExcept(store.DB, keepID, neverExisted); err != nil {
		t.Errorf("a missing import owner is a note, not a refusal: %v", err)
	}
}

// The note has to survive the early return for "no other accounts exist", which is the
// state the recovery runbook leaves the database in: one account, and a config still
// pointing at the id that was deleted.
func TestPlanNotesAMissingImportOwnerWithNoVictims(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "victimonly.db"))
	if err != nil {
		t.Fatal(err)
	}
	keepID := seedAccountWithData(t, store, "only@example.com")

	const neverExisted uint = 4242
	if keepID == neverExisted {
		t.Fatalf("fixture needs the kept account to differ from %d", neverExisted)
	}
	plan, err := PlanDeleteAllUsersExcept(store.DB, keepID, neverExisted)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.DeleteUsers) != 0 {
		t.Fatalf("fixture should leave nothing to delete, got %d victims", len(plan.DeleteUsers))
	}
	if plan.ImportOwnerMissing != neverExisted {
		t.Errorf("plan.ImportOwnerMissing = %d, want %d: the note must survive the "+
			"no-victims early return", plan.ImportOwnerMissing, neverExisted)
	}
}

// The two guards disagree on what "exists" means, and this pins the gap.
//
// PlanDeleteAllUsersExcept resolves the import owner with Unscoped(), so a soft-deleted
// account counts as present and the plan says nothing. ImportYAML resolves it default-scoped,
// so the same account is "no existing account" and boot refuses. An operator reading a clean
// dry run therefore gets no warning about a config that will not boot.
//
// Nothing in production soft-deletes a user today, so this is a latent mismatch rather than
// a live bug — but the two checks should answer the same question the same way.
func TestPlanAndImportDisagreeOnASoftDeletedImportOwner(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewSqlite(filepath.Join(tmp, "victimsoft.db"))
	if err != nil {
		t.Fatal(err)
	}
	softID := seedAccountWithData(t, store, "soft@example.com")
	keepID := seedAccountWithData(t, store, "keep@example.com")
	if err := store.DB.Delete(&User{}, softID).Error; err != nil {
		t.Fatal(err)
	}

	plan, err := PlanDeleteAllUsersExcept(store.DB, keepID, softID)
	if err != nil {
		t.Fatal(err)
	}

	opts := writeMinimalImportFixture(t, tmp)
	opts.OwnerID = softID
	importErr := store.ImportYAML(opts)

	if importErr != nil && plan.ImportOwnerMissing == 0 && plan.ImportOwnerVictim == 0 {
		t.Errorf("the import refuses this owner (%v) but the plan reports nothing: "+
			"ImportOwnerMissing=%d ImportOwnerVictim=%d. The plan resolves the owner "+
			"Unscoped, ImportYAML resolves it default-scoped, so a soft-deleted owner "+
			"passes the dry run and then fails at boot",
			importErr, plan.ImportOwnerMissing, plan.ImportOwnerVictim)
	}
}
