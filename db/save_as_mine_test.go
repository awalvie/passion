package db

import (
	"errors"
	"path/filepath"
	"testing"
)

// Nobody edits a catalog row. Saving your own version copies it to you, and from that
// moment it is an ordinary row — which is what keeps this cheap, because every existing
// owner-scoped path then applies unchanged.

func saveAsFixture(t *testing.T, name string) (*Store, uint, uint) {
	t.Helper()
	store, err := NewSqlite(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	return store, 1, 2 // app, user
}

func TestSaveLibraryExerciseAsMine(t *testing.T) {
	store, app, user := saveAsFixture(t, "saveas-lib.db")

	src := LibraryExercise{OwnerID: app, Name: "Max Hangs", Slug: "max_hangs", Shared: true,
		Notes: "20mm edge", Kind: "timed_reps", Sets: 4, Reps: 1, WeightKg: 30,
		ManagedByCatalog: true}
	mustCreate(t, store, &src)
	mustCreate(t, store, &ExerciseMedia{OwnerID: app, LibraryExerciseID: &src.ID, VideoURL: "https://example.com/v"})
	child := LibraryExercise{OwnerID: app, Name: "Half crimp", Slug: "half_crimp", Shared: true,
		ParentLibraryExerciseID: &src.ID, Sets: 4}
	mustCreate(t, store, &child)

	got, err := SaveLibraryExerciseAsMine(store.DB, user, src.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.OwnerID != user {
		t.Errorf("the copy should belong to the user, owner is %d", got.OwnerID)
	}
	if got.Shared {
		t.Error("a copy must not itself be part of the catalog")
	}
	if got.ManagedByCatalog {
		t.Error("a copy is the user's own work; the importer must not manage it")
	}
	if got.CopiedFromID == nil || *got.CopiedFromID != src.ID {
		t.Error("the copy should record what it came from")
	}
	if got.Slug == src.Slug {
		t.Error("slug is identity, so a copy cannot share one with its original")
	}
	if got.WeightKg != 30 || got.Notes != "20mm edge" || got.Sets != 4 {
		t.Errorf("the copy lost its content: %+v", got)
	}

	// Media and options come across, owned by the user.
	var media int64
	store.DB.Model(&ExerciseMedia{}).Where("library_exercise_id = ? AND owner_id = ?", got.ID, user).Count(&media)
	if media != 1 {
		t.Errorf("want the media copied, got %d rows", media)
	}
	var kids []LibraryExercise
	if err := store.DB.Where("parent_library_exercise_id = ?", got.ID).Find(&kids).Error; err != nil {
		t.Fatal(err)
	}
	if len(kids) != 1 || kids[0].OwnerID != user || kids[0].Shared {
		t.Errorf("the option row did not come across as the user's: %+v", kids)
	}

	// The original is untouched.
	var after LibraryExercise
	if err := store.DB.Where("id = ?", src.ID).First(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.Name != "Max Hangs" || !after.Shared || after.OwnerID != app {
		t.Errorf("the catalog row changed: %+v", after)
	}
}

// The hard case. An Activity belongs to one SessionTemplate and keeps no reference to the
// block it was expanded from, so a session's children are its own rows and the copy has to
// be deep. There is nothing to point at.
func TestSaveSessionTemplateAsMineCopiesTheWholeGraph(t *testing.T) {
	store, app, user := saveAsFixture(t, "saveas-session.db")

	src := SessionTemplate{OwnerID: app, Name: "Boulder Session", Slug: "boulder",
		Shared: true, Color: "#ef4444", ManagedByCatalog: true}
	mustCreate(t, store, &src)
	warm := Activity{OwnerID: app, SessionTemplateID: src.ID, Type: "warmup", Name: "Warm Up", OrderIndex: 0}
	mustCreate(t, store, &warm)
	main := Activity{OwnerID: app, SessionTemplateID: src.ID, Type: "activity", Name: "Main", OrderIndex: 1}
	mustCreate(t, store, &main)

	row := Exercise{OwnerID: app, ActivityID: &warm.ID, Name: "Row", Sets: 2, Reps: 12}
	mustCreate(t, store, &row)
	mustCreate(t, store, &ExerciseMedia{OwnerID: app, ExerciseID: &row.ID, VideoURL: "https://example.com/row"})
	parent := Exercise{OwnerID: app, ActivityID: &main.ID, Name: "Pick a hang", Kind: "exercise_catalog"}
	mustCreate(t, store, &parent)
	mustCreate(t, store, &Exercise{OwnerID: app, ActivityID: &main.ID, ParentExerciseID: &parent.ID,
		Name: "20mm", Sets: 4, WeightKg: 30})

	got, err := SaveSessionTemplateAsMine(store.DB, user, src.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.OwnerID != user || got.Shared || got.Color != "#ef4444" {
		t.Errorf("the copy is wrong: %+v", got)
	}

	full, err := GetTemplateWithGraph(store.DB, user, got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Activities) != 2 {
		t.Fatalf("want 2 blocks copied, got %d", len(full.Activities))
	}
	if full.Activities[0].Name != "Warm Up" || full.Activities[1].Name != "Main" {
		t.Errorf("block order lost: %q then %q", full.Activities[0].Name, full.Activities[1].Name)
	}

	var exCount, mediaCount, childCount int64
	store.DB.Model(&Exercise{}).
		Where("activity_id IN (SELECT id FROM activities WHERE session_template_id = ?)", got.ID).
		Count(&exCount)
	if exCount != 3 {
		t.Errorf("want 3 exercises copied, got %d", exCount)
	}
	store.DB.Model(&Exercise{}).
		Where("activity_id IN (SELECT id FROM activities WHERE session_template_id = ?) AND parent_exercise_id IS NOT NULL", got.ID).
		Count(&childCount)
	if childCount != 1 {
		t.Errorf("the option row did not come across: %d", childCount)
	}
	store.DB.Model(&ExerciseMedia{}).
		Where("exercise_id IN (SELECT id FROM exercises WHERE activity_id IN (SELECT id FROM activities WHERE session_template_id = ?))", got.ID).
		Count(&mediaCount)
	if mediaCount != 1 {
		t.Errorf("want the media copied, got %d", mediaCount)
	}

	// The option must hang off the copy's own parent, not the catalog's.
	var kid Exercise
	if err := store.DB.Where("name = ? AND owner_id = ?", "20mm", user).First(&kid).Error; err != nil {
		t.Fatal(err)
	}
	if kid.ParentExerciseID == nil || *kid.ParentExerciseID == parent.ID {
		t.Error("the option still points at the catalog's parent exercise")
	}

	// The original is untouched.
	var acts int64
	store.DB.Model(&Activity{}).Where("session_template_id = ?", src.ID).Count(&acts)
	if acts != 2 {
		t.Errorf("the catalog session lost blocks: %d", acts)
	}
}

func TestSaveActivityTemplateAsMine(t *testing.T) {
	store, app, user := saveAsFixture(t, "saveas-block.db")
	src := ActivityTemplate{OwnerID: app, Name: "Finger Prep", Slug: "finger_prep", Shared: true}
	mustCreate(t, store, &src)
	for i, n := range []string{"Warm-up Pulls", "Density Hangs"} {
		mustCreate(t, store, &Exercise{OwnerID: app, ActivityTemplateID: &src.ID, Name: n, OrderIndex: i})
	}

	got, err := SaveActivityTemplateAsMine(store.DB, user, src.ID)
	if err != nil {
		t.Fatal(err)
	}
	full, err := GetActivityTemplateWithExercises(store.DB, user, got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(full.Exercises) != 2 {
		t.Fatalf("want 2 exercises copied, got %d", len(full.Exercises))
	}
	if full.Exercises[0].Name != "Warm-up Pulls" {
		t.Errorf("exercise order lost: %q first", full.Exercises[0].Name)
	}
}

// Saving the same row twice must not collide on the slug.
func TestSaveAsMineTwiceGetsDistinctSlugs(t *testing.T) {
	store, app, user := saveAsFixture(t, "saveas-twice.db")
	src := LibraryExercise{OwnerID: app, Name: "Max Hangs", Slug: "max_hangs", Shared: true}
	mustCreate(t, store, &src)

	first, err := SaveLibraryExerciseAsMine(store.DB, user, src.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SaveLibraryExerciseAsMine(store.DB, user, src.ID)
	if err != nil {
		t.Fatalf("a second copy should be allowed: %v", err)
	}
	if first.Slug == second.Slug {
		t.Errorf("both copies share the slug %q", first.Slug)
	}
}

// You cannot copy what you cannot see.
func TestSaveAsMineRefusesAnotherUsersPrivateRow(t *testing.T) {
	store, app, user := saveAsFixture(t, "saveas-private.db")
	private := LibraryExercise{OwnerID: app, Name: "Their Own", Slug: "their_own"}
	mustCreate(t, store, &private)

	if _, err := SaveLibraryExerciseAsMine(store.DB, user, private.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound for a row the user cannot see, got %v", err)
	}
}
