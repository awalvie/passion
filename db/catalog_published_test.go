package db

import (
	"path/filepath"
	"testing"
)

// TestPublishedCatalogImportsOnItsOwn is the guard that keeps the catalog split honest.
// The published tree ships in the public repo; the licensed tree does not. A published
// session that references a licensed block would resolve fine on the owner's machine and
// then exit the server at boot for everybody else, because an unknown ref is a hard error.
//
// Paths are relative to the db package directory, which is where `go test` runs.
func TestPublishedCatalogImportsOnItsOwn(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "published.db"))
	if err != nil {
		t.Fatal(err)
	}
	opts := YAMLImportOptions{
		OwnerID:              1,
		ExercisesDir:         []string{filepath.Join("..", "catalog", "exercises")},
		ActivityTemplatesDir: []string{filepath.Join("..", "catalog", "activity_templates")},
		SessionTemplatesDir:  []string{filepath.Join("..", "catalog", "session_templates")},
	}
	if err := store.ImportYAML(opts); err != nil {
		t.Fatalf("the published catalog does not import on its own: %v", err)
	}

	for _, c := range []struct {
		what  string
		model any
	}{
		{"library exercises", &LibraryExercise{}},
		{"activity templates", &ActivityTemplate{}},
		{"session templates", &SessionTemplate{}},
	} {
		var n int64
		if err := store.DB.Model(c.model).Where("owner_id = ?", 1).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			t.Errorf("no %s were imported", c.what)
		}
	}

	// Every session must resolve at least one activity. A session whose activities all
	// failed to resolve would otherwise import as an empty shell.
	var sessions []SessionTemplate
	if err := store.DB.Where("owner_id = ?", 1).Find(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	for _, st := range sessions {
		var acts int64
		if err := store.DB.Model(&Activity{}).
			Where("owner_id = ? AND session_template_id = ?", 1, st.ID).
			Count(&acts).Error; err != nil {
			t.Fatal(err)
		}
		if acts == 0 {
			t.Errorf("session %q imported with no activities", st.Name)
		}
	}

	// And every activity must hold at least one exercise, for the same reason.
	var activities []Activity
	if err := store.DB.Where("owner_id = ?", 1).Find(&activities).Error; err != nil {
		t.Fatal(err)
	}
	for _, act := range activities {
		var exs int64
		if err := store.DB.Model(&Exercise{}).
			Where("owner_id = ? AND activity_id = ?", 1, act.ID).
			Count(&exs).Error; err != nil {
			t.Fatal(err)
		}
		if exs == 0 {
			t.Errorf("activity %d (%s) imported with no exercises", act.ID, act.Type)
		}
	}
}
