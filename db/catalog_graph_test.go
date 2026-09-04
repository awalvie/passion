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
