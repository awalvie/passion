package db

import (
	"path/filepath"
	"testing"
)

// The trap in the whole shared-catalog change: child rows used to be filtered by the
// reader's own owner id, and the importer gives children the same owner as their parent.
// So widening only the top-level query made a catalog session open with no activities, no
// exercises, and no error. It rendered blank.
//
// A child is reached only through a parent the caller was already authorised for, so the
// filter answered a question already answered. These tests pin that it is gone.

// sharedTemplateFixture builds a catalog session owned by one account, and a second
// account that has never seen it.
func sharedTemplateFixture(t *testing.T, name string) (*Store, uint, uint, SessionTemplate) {
	t.Helper()
	store, err := NewSqlite(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	importer := User{Email: "importer@example.com", PasswordHash: "x"}
	reader := User{Email: "reader@example.com", PasswordHash: "x"}
	mustCreate(t, store, &importer)
	mustCreate(t, store, &reader)

	// The parent belongs to the reader so the top-level query is not what is under test.
	// Its children carry a different owner — the state a shared catalog creates, and the
	// state the importer already creates today.
	tpl := SessionTemplate{OwnerID: reader.ID, Name: "Boulder Session", Slug: "boulder", Shared: true}
	mustCreate(t, store, &tpl)
	warm := Activity{OwnerID: importer.ID, SessionTemplateID: tpl.ID, Type: "warmup", Name: "Warm Up", OrderIndex: 0}
	mustCreate(t, store, &warm)
	main := Activity{OwnerID: importer.ID, SessionTemplateID: tpl.ID, Type: "activity", Name: "Main", OrderIndex: 1}
	mustCreate(t, store, &main)
	for i, spec := range []struct {
		act  *Activity
		name string
	}{{&warm, "Row"}, {&warm, "Band Prep"}, {&main, "Boulder Projects"}} {
		ex := Exercise{OwnerID: importer.ID, ActivityID: &spec.act.ID, Name: spec.name, OrderIndex: i, Sets: 3}
		mustCreate(t, store, &ex)
		mustCreate(t, store, &ExerciseMedia{OwnerID: importer.ID, ExerciseID: &ex.ID, VideoURL: "https://example.com/v"})
	}
	return store, importer.ID, reader.ID, tpl
}

func TestCatalogTemplateLoadsItsChildrenForAnotherReader(t *testing.T) {
	store, _, reader, tpl := sharedTemplateFixture(t, "graph-template.db")

	got, err := GetTemplateWithGraph(store.DB, reader, tpl.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Activities) != 2 {
		t.Fatalf("a catalog session must load its activities for any reader, got %d", len(got.Activities))
	}
	total := 0
	media := 0
	for _, a := range got.Activities {
		total += len(a.Exercises)
		for _, e := range a.Exercises {
			media += len(e.Media)
		}
	}
	if total != 3 {
		t.Errorf("want 3 exercises across the blocks, got %d", total)
	}
	if media != 3 {
		t.Errorf("want media on each exercise, got %d", media)
	}
	if got.Activities[0].Name != "Warm Up" || got.Activities[1].Name != "Main" {
		t.Errorf("block order lost: %q then %q", got.Activities[0].Name, got.Activities[1].Name)
	}
}

// The same, one level down: an activity template's exercises.
func TestCatalogActivityTemplateLoadsItsExercisesForAnotherReader(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "graph-block.db"))
	if err != nil {
		t.Fatal(err)
	}
	const importer, reader uint = 1, 2
	// Parent to the reader, children to someone else — the child filter is what is tested.
	at := ActivityTemplate{OwnerID: reader, Name: "Finger Prep", Slug: "finger_prep", Shared: true}
	mustCreate(t, store, &at)
	for i, n := range []string{"Warm-up Pulls", "Density Hangs"} {
		mustCreate(t, store, &Exercise{OwnerID: importer, ActivityTemplateID: &at.ID, Name: n, OrderIndex: i})
	}

	got, err := GetActivityTemplateWithExercises(store.DB, reader, at.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Exercises) != 2 {
		t.Fatalf("a catalog block must load its exercises for any reader, got %d", len(got.Exercises))
	}
}

// Dropping the child filter must not become a way to read another user's private session.
// Authorisation lives on the parent, so a private parent stays unreachable.
func TestAPrivateTemplateIsStillUnreachable(t *testing.T) {
	store, importer, reader, _ := sharedTemplateFixture(t, "graph-private.db")

	private := SessionTemplate{OwnerID: importer, Name: "My Secret Session", Slug: "secret"}
	mustCreate(t, store, &private)
	act := Activity{OwnerID: importer, SessionTemplateID: private.ID, Type: "activity"}
	mustCreate(t, store, &act)
	mustCreate(t, store, &Exercise{OwnerID: importer, ActivityID: &act.ID, Name: "Hidden"})

	var rows []SessionTemplate
	if err := store.DB.Scopes(Visible(reader)).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Name == "My Secret Session" {
			t.Fatal("a private template became visible to another reader")
		}
	}
}

// The point of the whole change: a second account sees the catalog on its first login,
// with nothing copied to it. Every list and detail read goes through Visible, so this is
// the test that says the read side works end to end.
func TestASecondAccountSeesTheCatalogWithoutOwningAnyOfIt(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "catalog-visible.db"))
	if err != nil {
		t.Fatal(err)
	}
	const app, newcomer uint = 1, 2

	tpl := SessionTemplate{OwnerID: app, Name: "Boulder Session", Slug: "boulder",
		Shared: true, Source: "Passion", Label: "bouldering"}
	mustCreate(t, store, &tpl)
	act := Activity{OwnerID: app, SessionTemplateID: tpl.ID, Type: "activity", Name: "Main"}
	mustCreate(t, store, &act)
	mustCreate(t, store, &Exercise{OwnerID: app, ActivityID: &act.ID, Name: "Boulder Projects", Sets: 4})

	block := ActivityTemplate{OwnerID: app, Name: "Warm Up", Slug: "warm_up",
		Shared: true, Source: "Passion", Label: "warmup"}
	mustCreate(t, store, &block)
	mustCreate(t, store, &Exercise{OwnerID: app, ActivityTemplateID: &block.ID, Name: "Row"})

	lib := LibraryExercise{OwnerID: app, Name: "Max Hangs", Slug: "max_hangs",
		Shared: true, Source: "Passion", Label: "fingers"}
	mustCreate(t, store, &lib)

	// The newcomer owns nothing at all.
	var owned int64
	for _, m := range catalogModels() {
		var n int64
		if err := store.DB.Model(m).Where("owner_id = ?", newcomer).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		owned += n
	}
	if owned != 0 {
		t.Fatalf("the newcomer should own nothing, owns %d rows", owned)
	}

	// Lists.
	tpls, err := ListTemplates(store.DB, newcomer, "", "", "")
	if err != nil || len(tpls) != 1 {
		t.Fatalf("ListTemplates: %d templates, err %v", len(tpls), err)
	}
	blocks, err := ListActivityTemplates(store.DB, newcomer, "", "", "")
	if err != nil || len(blocks) != 1 {
		t.Fatalf("ListActivityTemplates: %d blocks, err %v", len(blocks), err)
	}
	libs, err := ListLibraryExercises(store.DB, newcomer)
	if err != nil || len(libs) != 1 {
		t.Fatalf("ListLibraryExercises: %d exercises, err %v", len(libs), err)
	}

	// The filter dropdowns, which read from the same rows.
	for name, fn := range map[string]func() ([]string, error){
		"template sources": func() ([]string, error) { return DistinctTemplateSources(store.DB, newcomer) },
		"template tags":    func() ([]string, error) { return DistinctTemplateTags(store.DB, newcomer) },
		"library sources":  func() ([]string, error) { return DistinctLibrarySources(store.DB, newcomer) },
		"library tags":     func() ([]string, error) { return DistinctLibraryTags(store.DB, newcomer) },
		"block sources":    func() ([]string, error) { return DistinctActivityTemplateSources(store.DB, newcomer) },
		"block tags":       func() ([]string, error) { return DistinctActivityTemplateTags(store.DB, newcomer) },
	} {
		got, err := fn()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(got) == 0 {
			t.Errorf("%s: empty, so the filter offers nothing for a catalog the user can see", name)
		}
	}

	// Detail, with children.
	full, err := GetTemplateWithGraph(store.DB, newcomer, tpl.ID)
	if err != nil {
		t.Fatalf("the newcomer cannot open the catalog session: %v", err)
	}
	if len(full.Activities) != 1 || len(full.Activities[0].Exercises) != 1 {
		t.Errorf("the catalog session opened without its contents: %d blocks", len(full.Activities))
	}
	fullBlock, err := GetActivityTemplateWithExercises(store.DB, newcomer, block.ID)
	if err != nil {
		t.Fatalf("the newcomer cannot open the catalog block: %v", err)
	}
	if len(fullBlock.Exercises) != 1 {
		t.Errorf("the catalog block opened without its exercises")
	}
}
