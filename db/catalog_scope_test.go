package db

import (
	"errors"
	"path/filepath"
	"testing"
)

// Two accounts and a catalog. Each reads its own rows plus the catalog, and neither can
// see the other's. The failure mode this guards against is silent: a scope that is too
// wide leaks another user's data, one that is too narrow shows an empty catalog.
func catalogFixture(t *testing.T, name string) (*Store, uint, uint) {
	t.Helper()
	store, err := NewSqlite(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	alice := User{Email: "alice@example.com", PasswordHash: "x"}
	bob := User{Email: "bob@example.com", PasswordHash: "x"}
	mustCreate(t, store, &alice)
	mustCreate(t, store, &bob)

	// The catalog. Owned by whichever account imported it, but flagged shared.
	mustCreate(t, store, &LibraryExercise{OwnerID: alice.ID, Name: "Max Hangs", Slug: "max_hangs", Shared: true})
	mustCreate(t, store, &LibraryExercise{OwnerID: alice.ID, Name: "Pull-ups", Slug: "pull_ups", Shared: true})
	// One row each, private.
	mustCreate(t, store, &LibraryExercise{OwnerID: alice.ID, Name: "Alice's Own", Slug: "alice_own"})
	mustCreate(t, store, &LibraryExercise{OwnerID: bob.ID, Name: "Bob's Own", Slug: "bob_own"})
	return store, alice.ID, bob.ID
}

func TestVisibleShowsMineAndTheCatalog(t *testing.T) {
	store, alice, bob := catalogFixture(t, "scope-visible.db")

	for _, tc := range []struct {
		who  string
		id   uint
		want []string
	}{
		{"alice", alice, []string{"Alice's Own", "Max Hangs", "Pull-ups"}},
		{"bob", bob, []string{"Bob's Own", "Max Hangs", "Pull-ups"}},
	} {
		var rows []LibraryExercise
		if err := store.DB.Scopes(Visible(tc.id)).Order("name").Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		got := make([]string, len(rows))
		for i, r := range rows {
			got[i] = r.Name
		}
		if len(got) != len(tc.want) {
			t.Fatalf("%s sees %v, want %v", tc.who, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s sees %v, want %v", tc.who, got, tc.want)
				break
			}
		}
	}
}

// Bob must never see Alice's private row, whatever else changes.
func TestVisibleNeverLeaksAnotherUsersRows(t *testing.T) {
	store, _, bob := catalogFixture(t, "scope-leak.db")
	var rows []LibraryExercise
	if err := store.DB.Scopes(Visible(bob)).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Name == "Alice's Own" {
			t.Fatal("the scope leaked another user's private row")
		}
	}
}

// "What have I made" must exclude the catalog, or an export or a count double-counts it.
func TestMineExcludesTheCatalog(t *testing.T) {
	store, alice, _ := catalogFixture(t, "scope-mine.db")
	var rows []LibraryExercise
	if err := store.DB.Scopes(Mine(alice)).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != "Alice's Own" {
		t.Fatalf("Mine should return only Alice's own row, got %d rows", len(rows))
	}
}

// The guard has to tell three cases apart, because each deserves a different answer.
func TestGuardWritable(t *testing.T) {
	store, alice, bob := catalogFixture(t, "scope-guard.db")

	var mine, shared, theirs LibraryExercise
	mustFind := func(dst *LibraryExercise, name string) {
		if err := store.DB.Where("name = ?", name).First(dst).Error; err != nil {
			t.Fatal(err)
		}
	}
	mustFind(&mine, "Alice's Own")
	mustFind(&shared, "Max Hangs")
	mustFind(&theirs, "Bob's Own")

	if err := GuardWritable(store.DB, &LibraryExercise{}, alice, mine.ID); err != nil {
		t.Errorf("Alice must be able to change her own row: %v", err)
	}
	if err := GuardWritable(store.DB, &LibraryExercise{}, alice, shared.ID); !errors.Is(err, ErrSharedReadOnly) {
		t.Errorf("a catalog row must report ErrSharedReadOnly, got %v", err)
	}
	if err := GuardWritable(store.DB, &LibraryExercise{}, alice, theirs.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("another user's row must report ErrNotFound, not that it is shared: got %v", err)
	}
	if err := GuardWritable(store.DB, &LibraryExercise{}, alice, 99999); !errors.Is(err, ErrNotFound) {
		t.Errorf("a missing row must report ErrNotFound, got %v", err)
	}
	// Bob owns the row but it is flagged shared, so even he cannot edit it in place.
	if err := GuardWritable(store.DB, &LibraryExercise{}, bob, shared.ID); !errors.Is(err, ErrSharedReadOnly) {
		t.Errorf("owning a catalog row does not make it editable: got %v", err)
	}
}
