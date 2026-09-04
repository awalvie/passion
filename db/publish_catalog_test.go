package db

import (
	"path/filepath"
	"testing"
	"time"
)

// Publishing hands the importer's rows to everyone. It must never hand over a row a user
// made, or one they edited — that is their own work.

func publishFixture(t *testing.T, name string) (*Store, uint) {
	t.Helper()
	store, err := NewSqlite(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	owner := User{Email: "owner@example.com", PasswordHash: "x"}
	mustCreate(t, store, &owner)
	return store, owner.ID
}

func TestPublishCatalogFlagsOnlyTheImportersRows(t *testing.T) {
	store, owner := publishFixture(t, "publish-basic.db")
	edited := time.Now()

	mustCreate(t, store, &LibraryExercise{OwnerID: owner, Name: "Max Hangs", Slug: "max_hangs", ManagedByCatalog: true})
	mustCreate(t, store, &LibraryExercise{OwnerID: owner, Name: "Pull-ups", Slug: "pull_ups", ManagedByCatalog: true})
	// The user's own work, two ways.
	mine := LibraryExercise{OwnerID: owner, Name: "My Own Thing", Slug: "my_own"}
	mustCreate(t, store, &mine)
	stamped := LibraryExercise{OwnerID: owner, Name: "I Changed This", Slug: "changed",
		ManagedByCatalog: true, CatalogEditedAt: &edited}
	mustCreate(t, store, &stamped)

	mustCreate(t, store, &ActivityTemplate{OwnerID: owner, Name: "Warm Up", Slug: "warm_up", ManagedByCatalog: true})
	mustCreate(t, store, &SessionTemplate{OwnerID: owner, Name: "Boulder", Slug: "boulder", ManagedByCatalog: true})
	// The hidden per-user anchor for open sessions. One shared copy would give every
	// account the same one.
	system := SessionTemplate{OwnerID: owner, Name: "Open Session", Slug: "open", IsSystem: true, ManagedByCatalog: true}
	mustCreate(t, store, &system)

	rep, err := PublishCatalog(store.DB, owner, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 4 {
		t.Fatalf("want 4 rows published, got %d (%v)", rep.Total, rep.ByTable)
	}
	if rep.SkippedUser["library_exercises"] != 1 {
		t.Errorf("the user's own row should be reported as skipped, got %v", rep.SkippedUser)
	}
	if rep.SkippedEdited["library_exercises"] != 1 {
		t.Errorf("the edited row should be reported as skipped, got %v", rep.SkippedEdited)
	}

	for _, tc := range []struct {
		id     uint
		shared bool
		why    string
	}{
		{mine.ID, false, "a row the user made"},
		{stamped.ID, false, "a row the user edited"},
	} {
		var row LibraryExercise
		if err := store.DB.Where("id = ?", tc.id).First(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.Shared != tc.shared {
			t.Errorf("%s was published", tc.why)
		}
	}
	var sys SessionTemplate
	if err := store.DB.Where("id = ?", system.ID).First(&sys).Error; err != nil {
		t.Fatal(err)
	}
	if sys.Shared {
		t.Error("the system open-session template was published")
	}
}

// Safe to run twice: a second pass finds nothing left to do.
func TestPublishCatalogIsIdempotent(t *testing.T) {
	store, owner := publishFixture(t, "publish-twice.db")
	mustCreate(t, store, &LibraryExercise{OwnerID: owner, Name: "Max Hangs", Slug: "max_hangs", ManagedByCatalog: true})

	if _, err := PublishCatalog(store.DB, owner, false); err != nil {
		t.Fatal(err)
	}
	rep, err := PublishCatalog(store.DB, owner, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 0 {
		t.Fatalf("a second pass published %d more rows", rep.Total)
	}
	if rep.AlreadyShared != 1 {
		t.Errorf("want 1 row reported as already shared, got %d", rep.AlreadyShared)
	}
}

func TestPublishCatalogDryRunChangesNothing(t *testing.T) {
	store, owner := publishFixture(t, "publish-dry.db")
	row := LibraryExercise{OwnerID: owner, Name: "Max Hangs", Slug: "max_hangs", ManagedByCatalog: true}
	mustCreate(t, store, &row)

	rep, err := PublishCatalog(store.DB, owner, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 1 {
		t.Fatalf("the dry run should report 1 row, reported %d", rep.Total)
	}
	var after LibraryExercise
	if err := store.DB.Where("id = ?", row.ID).First(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.Shared {
		t.Error("a dry run published a row")
	}
}

func TestPublishCatalogRefusesAnUnknownOwner(t *testing.T) {
	store, _ := publishFixture(t, "publish-badowner.db")
	for _, id := range []uint{0, 9999} {
		if _, err := PublishCatalog(store.DB, id, false); err == nil {
			t.Errorf("publish should refuse owner id %d", id)
		}
	}
}

// The way out. Publishing changes nothing else about a row, so unpublishing restores the
// state exactly.
func TestUnpublishRestoresPrivateOwnership(t *testing.T) {
	store, owner := publishFixture(t, "publish-undo.db")
	mustCreate(t, store, &LibraryExercise{OwnerID: owner, Name: "Max Hangs", Slug: "max_hangs", ManagedByCatalog: true})
	mustCreate(t, store, &SessionTemplate{OwnerID: owner, Name: "Boulder", Slug: "boulder", ManagedByCatalog: true})

	if _, err := PublishCatalog(store.DB, owner, false); err != nil {
		t.Fatal(err)
	}
	n, err := UnpublishCatalog(store.DB, owner)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("want 2 rows unpublished, got %d", n)
	}
	var shared int64
	for _, m := range catalogModels() {
		var c int64
		store.DB.Model(m).Where("shared = ?", true).Count(&c)
		shared += c
	}
	if shared != 0 {
		t.Errorf("%d rows are still shared", shared)
	}
}

// A published row is readable by an account that owns nothing, and still refuses a write.
// This is the whole point, checked end to end at the data layer.
func TestAPublishedRowIsReadableByAnyoneAndWritableByNobody(t *testing.T) {
	store, owner := publishFixture(t, "publish-effect.db")
	stranger := User{Email: "stranger@example.com", PasswordHash: "x"}
	mustCreate(t, store, &stranger)

	row := LibraryExercise{OwnerID: owner, Name: "Max Hangs", Slug: "max_hangs", ManagedByCatalog: true}
	mustCreate(t, store, &row)

	var before []LibraryExercise
	if err := store.DB.Scopes(Visible(stranger.ID)).Find(&before).Error; err != nil {
		t.Fatal(err)
	}
	if len(before) != 0 {
		t.Fatalf("the stranger should see nothing before publishing, saw %d", len(before))
	}

	if _, err := PublishCatalog(store.DB, owner, false); err != nil {
		t.Fatal(err)
	}

	var after []LibraryExercise
	if err := store.DB.Scopes(Visible(stranger.ID)).Find(&after).Error; err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 {
		t.Fatalf("the stranger should now see the catalog, saw %d rows", len(after))
	}
	if err := GuardWritable(store.DB, &LibraryExercise{}, stranger.ID, row.ID); err != ErrSharedReadOnly {
		t.Errorf("a published row must refuse a write, got %v", err)
	}
	// Not even its owner can edit it in place any more.
	if err := GuardWritable(store.DB, &LibraryExercise{}, owner, row.ID); err != ErrSharedReadOnly {
		t.Errorf("owning a published row does not make it editable, got %v", err)
	}
}
