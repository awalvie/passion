package db

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ghostFixture builds the state the 4 September 2026 deploy produced: a full catalog owned
// by an id with no account behind it, beside a real account with its own catalog.
//
// It imports as owner 1 while user 1 still exists, then removes the account — which is the
// order things actually happened in, and the only order still possible now that ImportYAML
// refuses an owner with no account.
func ghostFixture(t *testing.T) (*Store, YAMLImportOptions) {
	t.Helper()
	tmp := t.TempDir()
	store, err := NewSqlite(filepath.Join(tmp, "ghost.db"))
	if err != nil {
		t.Fatal(err)
	}
	seedImportOwner(t, store, 1)
	seedImportOwner(t, store, 4)

	opts := writeMinimalImportFixture(t, tmp)
	if err := store.ImportYAML(opts); err != nil {
		t.Fatalf("import as owner 1: %v", err)
	}
	realOpts := opts
	realOpts.OwnerID = 4
	if err := store.ImportYAML(realOpts); err != nil {
		t.Fatalf("import as owner 4: %v", err)
	}

	// The account goes, the rows stay. Hard delete, as --delete-users-except does.
	if err := store.DB.Unscoped().Delete(&User{}, uint(1)).Error; err != nil {
		t.Fatal(err)
	}
	return store, realOpts
}

func countOwnedRows(t *testing.T, store *Store, ownerID uint) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	for _, model := range ownerScopedTables() {
		table, err := tableNameOf(store.DB, model)
		if err != nil {
			t.Fatal(err)
		}
		var n int64
		if err := store.DB.Unscoped().Table(table).Where("owner_id = ?", ownerID).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			out[table] = n
		}
	}
	return out
}

// The test that would have caught the incident. The purge finds exactly what the importer
// wrote under the dead id, and nothing of the real account's.
func TestPurgeGhostCatalogFindsWhatTheStaleImportOwnerLeft(t *testing.T) {
	store, _ := ghostFixture(t)

	plan, err := PlanPurgeGhostCatalog(store.DB, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Refusals) > 0 {
		t.Fatalf("plan refused a genuine ghost catalog: %v", plan.Refusals)
	}
	if plan.TotalRows == 0 {
		t.Fatal("fixture is decorative: no ghost rows to remove")
	}

	ghostBefore := countOwnedRows(t, store, 1)
	realBefore := countOwnedRows(t, store, 4)
	if len(realBefore) == 0 {
		t.Fatal("fixture is decorative: the real account owns nothing to protect")
	}
	for table, n := range ghostBefore {
		if plan.RowsByTable[table] != n {
			t.Errorf("%s: plan says %d, database has %d", table, plan.RowsByTable[table], n)
		}
	}

	applied, err := PurgeGhostCatalog(store.DB, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied {
		t.Error("purge reported nothing applied")
	}
	if left := countOwnedRows(t, store, 1); len(left) != 0 {
		t.Errorf("ghost rows survived: %v", left)
	}
	realAfter := countOwnedRows(t, store, 4)
	for table, n := range realBefore {
		if realAfter[table] != n {
			t.Errorf("%s: the real account went from %d to %d rows", table, n, realAfter[table])
		}
	}

	var violations []struct{ Table string }
	if err := store.DB.Raw("PRAGMA foreign_key_check").Scan(&violations).Error; err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Errorf("purge left %d foreign key violation(s)", len(violations))
	}
}

// The command's entire safety property: it only ever removes rows nobody can reach.
func TestPurgeGhostCatalogRefusesWhenTheAccountExists(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, store *Store)
	}{
		{"live account", func(t *testing.T, store *Store) {}},
		{"soft-deleted account", func(t *testing.T, store *Store) {
			// Still an account: its rows are scoped to it and it could be restored.
			if err := store.DB.Delete(&User{}, uint(1)).Error; err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			store, err := NewSqlite(filepath.Join(tmp, "exists.db"))
			if err != nil {
				t.Fatal(err)
			}
			seedImportOwner(t, store, 1)
			opts := writeMinimalImportFixture(t, tmp)
			if err := store.ImportYAML(opts); err != nil {
				t.Fatal(err)
			}
			before := countOwnedRows(t, store, 1)
			tc.setup(t, store)

			plan, err := PlanPurgeGhostCatalog(store.DB, 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Refusals) == 0 {
				t.Fatal("planned to purge rows belonging to a real account")
			}
			if _, err := PurgeGhostCatalog(store.DB, 1); err == nil {
				t.Fatal("purge ran against a real account's rows")
			}
			if after := countOwnedRows(t, store, 1); len(after) != len(before) {
				t.Error("a refused purge still deleted rows")
			}
		})
	}
}

func TestPurgeGhostCatalogRefusesOwnerZero(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "zero.db"))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanPurgeGhostCatalog(store.DB, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Refusals) == 0 {
		t.Fatal("owner 0 is the no-owner sentinel and must be refused")
	}
	if _, err := PurgeGhostCatalog(store.DB, 0); err == nil {
		t.Fatal("purge accepted owner 0")
	}
}

// A dry run is the default, so this is what an operator sees first. It must change nothing.
func TestPurgeGhostCatalogPlanChangesNothing(t *testing.T) {
	store, _ := ghostFixture(t)
	before := countOwnedRows(t, store, 1)

	if _, err := PlanPurgeGhostCatalog(store.DB, 1); err != nil {
		t.Fatal(err)
	}
	after := countOwnedRows(t, store, 1)
	if len(after) != len(before) {
		t.Fatal("planning changed the row counts")
	}
	for table, n := range before {
		if after[table] != n {
			t.Errorf("%s: %d became %d", table, n, after[table])
		}
	}
}

// Run history is the thing this must never destroy, and none of these six tables declares a
// foreign key to exercises — so nothing but this check would stop it.
func TestPurgeGhostCatalogRefusesWhenHistoryReferencesItsExercises(t *testing.T) {
	for _, tc := range []struct {
		name string
		make func(exerciseID uint) any
	}{
		{"completion", func(id uint) any {
			return &RunExerciseCompletion{OwnerID: 4, RunID: 1, ExerciseID: id, CompletedAt: time.Now()}
		}},
		{"climbing tick", func(id uint) any {
			return &ClimbingTick{OwnerID: 4, RunID: 1, ExerciseID: id, Kind: "boulder"}
		}},
		{"manual set log", func(id uint) any {
			return &ManualExerciseSetLog{OwnerID: 4, RunID: 1, ExerciseID: id, SetIndex: 1}
		}},
		{"planned set", func(id uint) any {
			return &ExercisePlannedSet{OwnerID: 4, ExerciseID: id, SetIndex: 1}
		}},
		{"climbing meta", func(id uint) any {
			return &ClimbingExerciseMeta{OwnerID: 4, RunID: 1, ExerciseID: id, Type: "board"}
		}},
		{"choice as parent", func(id uint) any {
			return &RunExerciseChoice{OwnerID: 4, RunID: 1, ParentExerciseID: id, ChosenExerciseID: 999}
		}},
		{"choice as chosen", func(id uint) any {
			return &RunExerciseChoice{OwnerID: 4, RunID: 1, ParentExerciseID: 999, ChosenExerciseID: id}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, _ := ghostFixture(t)

			var ghostEx Exercise
			if err := store.DB.Where("owner_id = ?", 1).First(&ghostEx).Error; err != nil {
				t.Fatal(err)
			}
			if err := store.DB.Create(tc.make(ghostEx.ID)).Error; err != nil {
				t.Fatal(err)
			}

			plan, err := PlanPurgeGhostCatalog(store.DB, 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Refusals) == 0 {
				t.Fatalf("planned to delete an exercise a %s still points at", tc.name)
			}
			before := countOwnedRows(t, store, 1)
			if _, err := PurgeGhostCatalog(store.DB, 1); err == nil {
				t.Fatal("purge ran anyway")
			}
			if after := countOwnedRows(t, store, 1); len(after) != len(before) {
				t.Error("a refused purge still deleted rows")
			}
		})
	}
}

// Anything pointing in from another account would become a dangling id.
func TestPurgeGhostCatalogRefusesOnInboundReferences(t *testing.T) {
	store, _ := ghostFixture(t)

	var ghostLib LibraryExercise
	if err := store.DB.Where("owner_id = ?", 1).First(&ghostLib).Error; err != nil {
		t.Fatal(err)
	}
	// The real account's exercise now points at a ghost library row — the shape production
	// was checked for by hand.
	var realEx Exercise
	if err := store.DB.Where("owner_id = ?", 4).First(&realEx).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&realEx).Update("library_exercise_id", ghostLib.ID).Error; err != nil {
		t.Fatal(err)
	}

	plan, err := PlanPurgeGhostCatalog(store.DB, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Refusals) == 0 {
		t.Fatal("planned to delete a library row another account's exercise points at")
	}
	if !strings.Contains(strings.Join(plan.Refusals, " "), "library exercises") {
		t.Errorf("refusal should name what points in, got: %v", plan.Refusals)
	}
}

// A published catalog owned here is the app's, not a ghost's, whatever the account state.
func TestPurgeGhostCatalogRefusesOnSharedRows(t *testing.T) {
	store, _ := ghostFixture(t)
	if err := store.DB.Model(&LibraryExercise{}).Where("owner_id = ?", 1).
		Update("shared", true).Error; err != nil {
		t.Fatal(err)
	}

	plan, err := PlanPurgeGhostCatalog(store.DB, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Refusals) == 0 {
		t.Fatal("planned to delete the app's published catalog")
	}
	if _, err := PurgeGhostCatalog(store.DB, 1); err == nil {
		t.Fatal("purge removed shared rows")
	}
}

// Ad-hoc SQL run during the incident may have left a mixed live/soft-deleted state, and a
// soft-deleted row still occupies its (owner_id, slug) identity.
func TestPurgeGhostCatalogRemovesSoftDeletedGhostRows(t *testing.T) {
	store, _ := ghostFixture(t)
	if err := store.DB.Where("owner_id = ?", 1).Delete(&LibraryExercise{}).Error; err != nil {
		t.Fatal(err)
	}
	var soft int64
	if err := store.DB.Unscoped().Model(&LibraryExercise{}).
		Where("owner_id = ? AND deleted_at IS NOT NULL", 1).Count(&soft).Error; err != nil {
		t.Fatal(err)
	}
	if soft == 0 {
		t.Fatal("fixture is decorative: nothing was soft-deleted")
	}

	if _, err := PurgeGhostCatalog(store.DB, 1); err != nil {
		t.Fatal(err)
	}
	var left int64
	if err := store.DB.Unscoped().Model(&LibraryExercise{}).
		Where("owner_id = ?", 1).Count(&left).Error; err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d soft-deleted ghost row(s) survived", left)
	}
}

func TestPurgeGhostCatalogIsIdempotent(t *testing.T) {
	store, _ := ghostFixture(t)
	if _, err := PurgeGhostCatalog(store.DB, 1); err != nil {
		t.Fatal(err)
	}
	again, err := PurgeGhostCatalog(store.DB, 1)
	if err != nil {
		t.Fatalf("a second purge errored instead of finding nothing to do: %v", err)
	}
	if again.TotalRows != 0 {
		t.Errorf("second run reported %d row(s) to remove", again.TotalRows)
	}
}

// ImportYAML refuses a soft-deleted owner ("names no existing account") while this refuses
// to purge it ("the account is still there"). Both are right for their own consequence,
// but an operator reading the two in sequence would reasonably think one is broken — so the
// refusal has to say which case it is rather than flatly claiming the account exists.
func TestPurgeGhostCatalogExplainsASoftDeletedAccountRatherThanClaimingItExists(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewSqlite(filepath.Join(tmp, "softacct.db"))
	if err != nil {
		t.Fatal(err)
	}
	seedImportOwner(t, store, 1)
	opts := writeMinimalImportFixture(t, tmp)
	if err := store.ImportYAML(opts); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Delete(&User{}, uint(1)).Error; err != nil {
		t.Fatal(err)
	}

	// The pairing that makes this confusing in the first place.
	if err := store.ImportYAML(opts); err == nil {
		t.Fatal("import accepted a soft-deleted owner; this test's premise is gone")
	} else if !strings.Contains(err.Error(), "names no existing account") {
		t.Fatalf("unexpected import error: %v", err)
	}

	plan, err := PlanPurgeGhostCatalog(store.DB, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Refusals) != 1 {
		t.Fatalf("want exactly one refusal, got %v", plan.Refusals)
	}
	if !strings.Contains(plan.Refusals[0], "soft-deleted") {
		t.Errorf("refusal should name the soft-delete, got: %q", plan.Refusals[0])
	}
	if !strings.Contains(plan.Refusals[0], "--delete-users-except") {
		t.Errorf("refusal should say how to proceed, got: %q", plan.Refusals[0])
	}
}
