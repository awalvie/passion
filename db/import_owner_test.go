package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The importer writes a whole catalog under one owner id, and every read of that catalog
// is owner-scoped. So an id with no account behind it produces rows nobody can reach —
// and on the next run prune sees them as orphaned and starts deleting. That happened in
// production on 4 September 2026: a config left pointing at a deleted account wrote 1,172
// unreachable rows on deploy.

// seedImportOwner creates the account the importer writes into. ImportYAML refuses an
// owner id with no account behind it, so every import fixture needs one. Idempotent so it
// is safe to call twice on one store.
func seedImportOwner(t *testing.T, store *Store, id uint) {
	t.Helper()
	var existing User
	if err := store.DB.Where("id = ?", id).First(&existing).Error; err == nil {
		return
	}
	u := User{Email: fmt.Sprintf("owner%d@test.local", id)}
	u.ID = id
	if err := store.DB.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
}

func TestImportRefusesAnOwnerIDWithNoAccount(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewSqlite(filepath.Join(tmp, "noowner.db"))
	if err != nil {
		t.Fatal(err)
	}
	opts := writeMinimalImportFixture(t, tmp)

	// Deliberately no account: this is the production config exactly.
	err = store.ImportYAML(opts)
	if err == nil {
		t.Fatal("import ran for an owner id with no account behind it; that is the bug")
	}
	if !strings.Contains(err.Error(), "names no existing account") {
		t.Fatalf("error should name the cause, got: %v", err)
	}

	// And nothing was written. A guard that fails after writing is not a guard.
	for _, tc := range []struct {
		table string
		model any
	}{
		{"library_exercises", &LibraryExercise{}},
		{"session_templates", &SessionTemplate{}},
		{"exercises", &Exercise{}},
	} {
		var n int64
		if err := store.DB.Unscoped().Model(tc.model).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s: refused import still wrote %d row(s)", tc.table, n)
		}
	}
}

func TestImportRunsOnceTheOwnerExists(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewSqlite(filepath.Join(tmp, "hasowner.db"))
	if err != nil {
		t.Fatal(err)
	}
	opts := writeMinimalImportFixture(t, tmp)
	seedImportOwner(t, store, opts.OwnerID)

	if err := store.ImportYAML(opts); err != nil {
		t.Fatalf("import should run for a real account: %v", err)
	}
	var n int64
	if err := store.DB.Model(&LibraryExercise{}).Where("owner_id = ?", opts.OwnerID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("import reported success but wrote no library exercises")
	}
}

// A soft-deleted owner is refused, and that is the correct answer rather than an accident
// of GORM's default scope.
//
// Nothing can reach a catalog owned by a soft-deleted account. Both auth paths load the
// user with a default-scoped First — the login lookup by email and the session check by id
// — so that account cannot sign in or hold a session. A catalog written under it is exactly
// as unreachable as one written under an id with no row at all, which is the incident this
// guard exists for. PublishCatalog resolves its owner the same default-scoped way, so
// refusing here also keeps the two entry points to the catalog in agreement.
//
// No production path soft-deletes a user: DeleteAllUsersExcept is Unscoped and hard-deletes.
// So this asserts a deliberate choice, not observed behaviour. If a restore-account feature
// ever lands, revisit it here — the change would be Unscoped() in the guard.
func TestImportRefusesASoftDeletedOwner(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewSqlite(filepath.Join(tmp, "softowner.db"))
	if err != nil {
		t.Fatal(err)
	}
	opts := writeMinimalImportFixture(t, tmp)
	seedImportOwner(t, store, opts.OwnerID)
	if err := store.DB.Delete(&User{}, opts.OwnerID).Error; err != nil {
		t.Fatal(err)
	}
	var soft User
	if err := store.DB.Unscoped().Where("id = ?", opts.OwnerID).First(&soft).Error; err != nil {
		t.Fatal(err)
	}
	if !soft.DeletedAt.Valid {
		t.Fatal("fixture is decorative: the account was not soft-deleted")
	}

	err = store.ImportYAML(opts)
	if err == nil {
		t.Fatal("the import ran for a soft-deleted owner, whose catalog no session can reach")
	}
	if !strings.Contains(err.Error(), "names no existing account") {
		t.Fatalf("error should name the cause, got: %v", err)
	}
	var n int64
	if err := store.DB.Unscoped().Model(&LibraryExercise{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("a refused import still wrote %d library exercise(s)", n)
	}
}

// writeMinimalImportFixture lays down the smallest valid catalog on disk and returns the
// options pointing at it. Owner 1 by default, matching the other import fixtures.
func writeMinimalImportFixture(t *testing.T, tmp string) YAMLImportOptions {
	t.Helper()
	exDir := filepath.Join(tmp, "fx-exercises")
	tplDir := filepath.Join(tmp, "fx-templates")
	for _, d := range []string{exDir, tplDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(exDir, "e.yaml"), []byte(`
name: "Weighted Pull-ups"
slug: "weighted_pull_ups"
kind: "reps_and_sets"
sets: 5
reps: 5
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tplDir, "t.yaml"), []byte(`
name: "Strength Day"
slug: "strength_day"
activities:
  - type: "activity"
    exercises:
      - ref: "weighted_pull_ups"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	return YAMLImportOptions{
		OwnerID:             1,
		ExercisesDir:        []string{exDir},
		SessionTemplatesDir: []string{tplDir},
	}
}

// A guard whose named remedy does not clear it is worse than no guard: this one is on the
// boot path, and the systemd unit restarts on failure, so a permanent refusal is a
// crash-loop. The guard counts soft-deleted rows and NULL slugs; BackfillSlugs used to
// see neither, so one soft-deleted managed row with no slug bricked the import and
// --backfill-slugs reported nothing to do.
func TestTheSlugGuardIsAlwaysClearableByTheBackfillItNames(t *testing.T) {
	for _, tc := range []struct {
		name      string
		makeStuck func(t *testing.T, store *Store, id uint)
	}{
		{"empty slug", func(t *testing.T, store *Store, id uint) {
			if err := store.DB.Exec("UPDATE library_exercises SET slug = '' WHERE id = ?", id).Error; err != nil {
				t.Fatal(err)
			}
		}},
		{"soft-deleted with empty slug", func(t *testing.T, store *Store, id uint) {
			if err := store.DB.Exec("UPDATE library_exercises SET slug = '' WHERE id = ?", id).Error; err != nil {
				t.Fatal(err)
			}
			if err := store.DB.Delete(&LibraryExercise{}, id).Error; err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, err := NewSqlite(filepath.Join(t.TempDir(), "clearable.db"))
			if err != nil {
				t.Fatal(err)
			}
			seedImportOwner(t, store, 1)
			row := LibraryExercise{OwnerID: 1, Name: "Weighted Pull-ups", ManagedByCatalog: true}
			if err := store.DB.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			tc.makeStuck(t, store, row.ID)

			if err := refuseUnsluggedCatalog(store.DB, 1); err == nil {
				t.Fatal("the guard did not fire, so this case proves nothing")
			}
			rep, err := BackfillSlugs(store.DB, false)
			if err != nil {
				t.Fatal(err)
			}
			if rep.Total == 0 {
				t.Fatal("the backfill the error message names found nothing to do; the import is stuck for good")
			}
			if err := refuseUnsluggedCatalog(store.DB, 1); err != nil {
				t.Fatalf("still refused after the backfill ran: %v", err)
			}
		})
	}
}

// A user-created row never carries a slug, because no UI path sets one. It is also never
// matched or pruned by the importer, so it must not block the import.
func TestTheSlugGuardIgnoresUserCreatedRows(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "userrows.db"))
	if err != nil {
		t.Fatal(err)
	}
	seedImportOwner(t, store, 1)
	row := LibraryExercise{OwnerID: 1, Name: "My Own Thing", ManagedByCatalog: false}
	if err := store.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Exec("UPDATE library_exercises SET slug = '' WHERE id = ?", row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := refuseUnsluggedCatalog(store.DB, 1); err != nil {
		t.Fatalf("a row the user made blocked the import: %v", err)
	}
}
