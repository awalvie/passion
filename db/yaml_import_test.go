package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImportYAMLUpsertByName(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	store, err := NewSqlite(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	exercisesDir := filepath.Join(tmp, "exercises")
	templatesDir := filepath.Join(tmp, "templates")
	if err := os.MkdirAll(exercisesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(exercisesDir, "pullups.yaml"), []byte(`
name: "Weighted Pull-ups"
slug: "weighted_pull_ups"
kind: "reps_and_sets"
sets: 5
reps: 5
rep_seconds: 5
set_rest_seconds: 120
weight_kg: 10
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "strength.yaml"), []byte(`
name: "Strength Base Session"
slug: "strength_base_session"
color: "#ef4444"
activities:
  - type: "activity"
    exercises:
      - ref: "Weighted Pull-ups"
  - type: "cooldown"
    exercises:
      - name: "Lat + Pec Stretch"
        slug: "lat_pec_stretch"
        kind: "session"
        session_duration_seconds: 300
`), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := YAMLImportOptions{
		OwnerID:             1,
		ExercisesDir:        []string{exercisesDir},
		SessionTemplatesDir: []string{templatesDir},
	}
	if err := store.ImportYAML(opts); err != nil {
		t.Fatal(err)
	}

	var libCount int64
	if err := store.DB.Model(&LibraryExercise{}).Where("owner_id = ?", 1).Count(&libCount).Error; err != nil {
		t.Fatal(err)
	}
	if libCount != 1 {
		t.Fatalf("expected one library exercise, got %d", libCount)
	}
	var tplCount int64
	if err := store.DB.Model(&SessionTemplate{}).Where("owner_id = ?", 1).Count(&tplCount).Error; err != nil {
		t.Fatal(err)
	}
	if tplCount != 1 {
		t.Fatalf("expected one session template, got %d", tplCount)
	}

	// Re-import changed YAML: same names should update, not duplicate.
	if err := os.WriteFile(filepath.Join(exercisesDir, "pullups.yaml"), []byte(`
name: "Weighted Pull-ups"
slug: "weighted_pull_ups"
kind: "reps_and_sets"
sets: 5
reps: 6
rep_seconds: 5
set_rest_seconds: 150
weight_kg: 12.5
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "strength.yaml"), []byte(`
name: "Strength Base Session"
slug: "strength_base_session"
color: "#3b82f6"
activities:
  - type: "activity"
    exercises:
      - ref: "Weighted Pull-ups"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := store.ImportYAML(opts); err != nil {
		t.Fatal(err)
	}

	if err := store.DB.Model(&LibraryExercise{}).Where("owner_id = ?", 1).Count(&libCount).Error; err != nil {
		t.Fatal(err)
	}
	if libCount != 1 {
		t.Fatalf("expected one library exercise after re-import, got %d", libCount)
	}
	if err := store.DB.Model(&SessionTemplate{}).Where("owner_id = ?", 1).Count(&tplCount).Error; err != nil {
		t.Fatal(err)
	}
	if tplCount != 1 {
		t.Fatalf("expected one session template after re-import, got %d", tplCount)
	}

	var lib LibraryExercise
	if err := store.DB.Where("owner_id = ? AND name = ?", 1, "Weighted Pull-ups").First(&lib).Error; err != nil {
		t.Fatal(err)
	}
	if lib.Reps != 6 || lib.SetRestSeconds != 150 || lib.WeightKg != 12.5 {
		t.Fatalf("expected updated library fields, got %+v", lib)
	}

	var tpl SessionTemplate
	if err := store.DB.Where("owner_id = ? AND name = ?", 1, "Strength Base Session").First(&tpl).Error; err != nil {
		t.Fatal(err)
	}
	if tpl.Color != "#3b82f6" {
		t.Fatalf("expected template color update, got %q", tpl.Color)
	}

	var activityCount int64
	if err := store.DB.Model(&Activity{}).Where("owner_id = ? AND session_template_id = ?", 1, tpl.ID).Count(&activityCount).Error; err != nil {
		t.Fatal(err)
	}
	if activityCount != 1 {
		t.Fatalf("expected replaced activities, got %d", activityCount)
	}
}

func TestImportYAMLUnknownReferenceFails(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	store, err := NewSqlite(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	exercisesDir := filepath.Join(tmp, "exercises")
	templatesDir := filepath.Join(tmp, "templates")
	if err := os.MkdirAll(exercisesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "broken.yaml"), []byte(`
name: "Broken Session"
slug: "broken_session"
activities:
  - type: "activity"
    exercises:
      - ref: "Does Not Exist"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	err = store.ImportYAML(YAMLImportOptions{
		OwnerID:             1,
		ExercisesDir:        []string{exercisesDir},
		SessionTemplatesDir: []string{templatesDir},
	})
	if err == nil {
		t.Fatal("expected unknown reference import error")
	}
}

// TestImportYAMLPrunesRenameOrphansButProtectsInUseAndUIRows covers pruneCatalogOrphans,
// the cleanup pass at the end of ImportYAML that removes catalog-managed rows whose names
// disappeared from the YAML (e.g. an exercise or template that was renamed). It asserts the
// four guard rails: the orphan is actually deleted, a UI-created row (managed_by_catalog =
// false) with a name absent from the YAML is never touched, a session template still
// referenced by a ScheduledSession survives even though it dropped out of the YAML, and an
// import with zero session templates (an empty directory) does not wipe existing managed
// session templates.
func TestImportYAMLPrunesRenameOrphansButProtectsInUseAndUIRows(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewSqlite(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}

	exercisesDir := filepath.Join(tmp, "exercises")
	templatesDir := filepath.Join(tmp, "templates")
	if err := os.MkdirAll(exercisesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeFile(filepath.Join(exercisesDir, "pullups.yaml"), `
name: "Pull Ups"
slug: "pull_ups"
kind: "reps_and_sets"
sets: 3
reps: 10
`)
	writeFile(filepath.Join(exercisesDir, "pushups.yaml"), `
name: "Push Ups"
slug: "push_ups"
kind: "reps_and_sets"
sets: 3
reps: 15
`)
	writeFile(filepath.Join(templatesDir, "strength.yaml"), `
name: "Strength Session"
slug: "strength_session"
activities:
  - type: "activity"
    exercises:
      - ref: "Pull Ups"
`)
	writeFile(filepath.Join(templatesDir, "old.yaml"), `
name: "Old Session"
slug: "old_session"
activities:
  - type: "activity"
    exercises:
      - ref: "Push Ups"
`)

	const ownerID uint = 1
	opts := YAMLImportOptions{OwnerID: ownerID, ExercisesDir: []string{exercisesDir}, SessionTemplatesDir: []string{templatesDir}}
	if err := store.ImportYAML(opts); err != nil {
		t.Fatalf("initial import: %v", err)
	}

	// A UI-created library exercise: managed_by_catalog stays false and its name is
	// never in the YAML, so it must survive every future prune.
	uiExercise := LibraryExercise{OwnerID: ownerID, Name: "Custom Stretch", Kind: "reps_and_sets"}
	if err := store.DB.Create(&uiExercise).Error; err != nil {
		t.Fatal(err)
	}

	var oldSession SessionTemplate
	if err := store.DB.Where("owner_id = ? AND name = ?", ownerID, "Old Session").First(&oldSession).Error; err != nil {
		t.Fatalf("lookup Old Session: %v", err)
	}
	// A scheduled session pins "Old Session" in place even after it drops out of the YAML.
	scheduled := ScheduledSession{OwnerID: ownerID, ScheduledDate: time.Now(), SessionTemplateID: oldSession.ID}
	if err := store.DB.Create(&scheduled).Error; err != nil {
		t.Fatal(err)
	}

	// Simulate renames: "Push Ups" and "Old Session" drop out of the YAML entirely.
	if err := os.Remove(filepath.Join(exercisesDir, "pushups.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(templatesDir, "old.yaml")); err != nil {
		t.Fatal(err)
	}

	if err := store.ImportYAML(opts); err != nil {
		t.Fatalf("re-import after rename: %v", err)
	}

	// "Push Ups" left the YAML, but "Old Session" refs it and is pinned in place by a
	// scheduled session — so a template exercise still describes it, and that exercise now
	// carries a link back to this library row. Prune protects a row that is still in use.
	//
	// This assertion used to expect the row to be deleted. It was deleted only because the
	// link did not exist: the importer never set LibraryExerciseID on a ref, so prune saw
	// no user and removed the row while a live exercise still pointed at it in spirit. The
	// link is what makes the protection work.
	var pushUpsCount int64
	if err := store.DB.Model(&LibraryExercise{}).
		Where("owner_id = ? AND name = ?", ownerID, "Push Ups").Count(&pushUpsCount).Error; err != nil {
		t.Fatal(err)
	}
	if pushUpsCount != 1 {
		t.Errorf("'Push Ups' is still used by a pinned template, so it must survive prune; %d row(s) remain", pushUpsCount)
	}
	var linked int64
	if err := store.DB.Model(&Exercise{}).
		Where("library_exercise_id = (SELECT id FROM library_exercises WHERE owner_id = ? AND name = ?)",
			ownerID, "Push Ups").Count(&linked).Error; err != nil {
		t.Fatal(err)
	}
	if linked == 0 {
		t.Error("nothing links back to 'Push Ups', so prune had no reason to protect it")
	}

	var uiCount int64
	if err := store.DB.Model(&LibraryExercise{}).
		Where("owner_id = ? AND name = ? AND managed_by_catalog = ?", ownerID, "Custom Stretch", false).
		Count(&uiCount).Error; err != nil {
		t.Fatal(err)
	}
	if uiCount != 1 {
		t.Errorf("expected UI-created 'Custom Stretch' to survive the prune, got count %d", uiCount)
	}

	var oldSessionCount int64
	if err := store.DB.Model(&SessionTemplate{}).
		Where("owner_id = ? AND name = ?", ownerID, "Old Session").Count(&oldSessionCount).Error; err != nil {
		t.Fatal(err)
	}
	if oldSessionCount != 1 {
		t.Errorf("expected 'Old Session' to survive because it has a scheduled session, got count %d", oldSessionCount)
	}

	// Empty-category guard: an import whose session-templates directory has zero YAML
	// files must not wipe the managed session templates that already exist.
	emptyTemplatesDir := filepath.Join(tmp, "templates_empty")
	if err := os.MkdirAll(emptyTemplatesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.ImportYAML(YAMLImportOptions{
		OwnerID:             ownerID,
		ExercisesDir:        []string{exercisesDir},
		SessionTemplatesDir: []string{emptyTemplatesDir},
	}); err != nil {
		t.Fatalf("import with empty session-templates dir: %v", err)
	}

	var tplCount int64
	if err := store.DB.Model(&SessionTemplate{}).Where("owner_id = ?", ownerID).Count(&tplCount).Error; err != nil {
		t.Fatal(err)
	}
	if tplCount != 2 {
		t.Errorf("expected both session templates to survive an empty-category import, got count %d", tplCount)
	}
}

// TestImportYAMLWalksSubdirectories guards that the catalog can be organised into
// folders. An entry's identity is its name, not its path, so files must import the same
// whether they sit at the top level or nested — and moving one must not look like a
// rename (which the prune would act on).
func TestImportYAMLWalksSubdirectories(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSqlite(filepath.Join(dir, "folders.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Create(&User{Email: "f@f.com", PasswordHash: "x"}).Error; err != nil {
		t.Fatal(err)
	}

	exDir := filepath.Join(dir, "exercises")
	nested := filepath.Join(exDir, "ondra", "mobility")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	stDir := filepath.Join(dir, "session_templates")
	if err := os.MkdirAll(filepath.Join(stDir, "programs"), 0o755); err != nil {
		t.Fatal(err)
	}
	// One exercise at the top level, one two folders deep.
	write := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(exDir, "flat.yaml"), "name: \"Flat Move\"\nslug: \"flat_move\"\nkind: \"session\"\n")
	write(filepath.Join(nested, "deep.yaml"), "name: \"Deep Move\"\nslug: \"deep_move\"\nkind: \"session\"\n")
	// A session template in a subfolder that refs the nested exercise.
	write(filepath.Join(stDir, "programs", "s.yaml"),
		"name: \"Folder Session\"\nslug: \"folder_session\"\nactivities:\n  - type: \"activity\"\n    exercises:\n      - ref: \"Deep Move\"\n")

	if err := store.ImportYAML(YAMLImportOptions{
		OwnerID: 1, ExercisesDir: []string{exDir}, SessionTemplatesDir: []string{stDir},
	}); err != nil {
		t.Fatalf("import with nested dirs failed: %v", err)
	}

	for _, name := range []string{"Flat Move", "Deep Move"} {
		var n int64
		store.DB.Model(&LibraryExercise{}).Where("owner_id = ? AND name = ?", 1, name).Count(&n)
		if n != 1 {
			t.Errorf("exercise %q: got %d rows, want 1 — nested files must import", name, n)
		}
	}
	var tn int64
	store.DB.Model(&SessionTemplate{}).Where("owner_id = ? AND name = ?", 1, "Folder Session").Count(&tn)
	if tn != 1 {
		t.Errorf("session template in a subfolder did not import (got %d rows)", tn)
	}
}

// mustWrite is a small helper for the multi-directory tests below, which set up several
// catalog trees each and would otherwise be mostly error checks.
func mustWrite(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestImportYAMLMultipleDirectories covers the catalog split: content lives in a
// published tree and a private tree, refs must resolve across both, and a name defined
// in both trees must be rejected rather than silently shadowing.
func TestImportYAMLMultipleDirectories(t *testing.T) {
	t.Run("ref resolves across directories", func(t *testing.T) {
		tmp := t.TempDir()
		store, err := NewSqlite(filepath.Join(tmp, "test.db"))
		if err != nil {
			t.Fatal(err)
		}
		pubEx := filepath.Join(tmp, "public", "exercises")
		privEx := filepath.Join(tmp, "private", "exercises")
		pubST := filepath.Join(tmp, "public", "templates")
		mustWrite(t, pubEx, "rows.yaml", "name: \"Ring Rows\"\nslug: \"ring_rows\"\nkind: \"reps_and_sets\"\nsets: 3\nreps: 10\n")
		mustWrite(t, privEx, "ladder.yaml", "name: \"Private Ladder\"\nslug: \"private_ladder\"\nkind: \"reps_and_sets\"\nsets: 2\nreps: 6\n")
		// The session lives in the published tree but references the private exercise.
		mustWrite(t, pubST, "day.yaml", `
name: "Mixed Day"
slug: "mixed_day"
activities:
  - type: "activity"
    exercises:
      - ref: "Ring Rows"
      - ref: "Private Ladder"
`)
		opts := YAMLImportOptions{
			OwnerID:             1,
			ExercisesDir:        []string{pubEx, privEx},
			SessionTemplatesDir: []string{pubST},
		}
		if err := store.ImportYAML(opts); err != nil {
			t.Fatalf("import across two exercise dirs: %v", err)
		}
		var count int64
		if err := store.DB.Model(&LibraryExercise{}).Where("owner_id = ?", 1).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("library exercises = %d, want 2", count)
		}
	})

	t.Run("duplicate name across directories is rejected", func(t *testing.T) {
		tmp := t.TempDir()
		store, err := NewSqlite(filepath.Join(tmp, "test.db"))
		if err != nil {
			t.Fatal(err)
		}
		a := filepath.Join(tmp, "a", "exercises")
		b := filepath.Join(tmp, "b", "exercises")
		st := filepath.Join(tmp, "templates")
		body := "name: \"Ring Rows\"\nslug: \"ring_rows\"\nkind: \"reps_and_sets\"\nsets: 3\nreps: 10\n"
		mustWrite(t, a, "rows.yaml", body)
		mustWrite(t, b, "rows.yaml", body)
		mustWrite(t, st, "day.yaml", "name: \"Day\"\nslug: \"day\"\nactivities: []\n")
		err = store.ImportYAML(YAMLImportOptions{
			OwnerID:             1,
			ExercisesDir:        []string{a, b},
			SessionTemplatesDir: []string{st},
		})
		if err == nil {
			t.Fatal("want an error for a name defined in two directories, got nil")
		}
		if !strings.Contains(err.Error(), "Ring Rows") {
			t.Fatalf("error should name the duplicate, got %v", err)
		}
	})

	t.Run("missing directory names which one", func(t *testing.T) {
		tmp := t.TempDir()
		store, err := NewSqlite(filepath.Join(tmp, "test.db"))
		if err != nil {
			t.Fatal(err)
		}
		ex := filepath.Join(tmp, "exercises")
		st := filepath.Join(tmp, "templates")
		mustWrite(t, ex, "rows.yaml", "name: \"Ring Rows\"\nslug: \"ring_rows\"\nkind: \"reps_and_sets\"\nsets: 3\nreps: 10\n")
		mustWrite(t, st, "day.yaml", "name: \"Day\"\nslug: \"day\"\nactivities: []\n")
		absent := filepath.Join(tmp, "not-there")
		err = store.ImportYAML(YAMLImportOptions{
			OwnerID:             1,
			ExercisesDir:        []string{ex, absent},
			SessionTemplatesDir: []string{st},
		})
		if err == nil {
			t.Fatal("want an error for a missing directory, got nil")
		}
		if !strings.Contains(err.Error(), "not-there") {
			t.Fatalf("error should name the missing directory, got %v", err)
		}
	})

	t.Run("blank entries are ignored", func(t *testing.T) {
		tmp := t.TempDir()
		store, err := NewSqlite(filepath.Join(tmp, "test.db"))
		if err != nil {
			t.Fatal(err)
		}
		ex := filepath.Join(tmp, "exercises")
		st := filepath.Join(tmp, "templates")
		mustWrite(t, ex, "rows.yaml", "name: \"Ring Rows\"\nslug: \"ring_rows\"\nkind: \"reps_and_sets\"\nsets: 3\nreps: 10\n")
		mustWrite(t, st, "day.yaml", "name: \"Day\"\nslug: \"day\"\nactivities: []\n")
		// A trailing comma in configuration produces an empty entry; it must not be
		// read as a directory named "".
		if err := store.ImportYAML(YAMLImportOptions{
			OwnerID:             1,
			ExercisesDir:        []string{ex, "  "},
			SessionTemplatesDir: []string{st, ""},
		}); err != nil {
			t.Fatalf("blank directory entries should be ignored, got %v", err)
		}
	})
}

// TestImportYAMLSkipsEditedRows covers the edited-row flag. The importer overwrites every
// field of the row it matches by name, and for blocks and sessions it deletes and
// recreates the child rows — so before this flag existed, any edit made in the app was
// gone on the next restart and a rename was deleted outright by pruneCatalogOrphans.
func TestImportYAMLSkipsEditedRows(t *testing.T) {
	// setup writes one exercise, one block and one session, imports them once, and
	// returns the store plus the options needed to import again.
	setup := func(t *testing.T) (*Store, YAMLImportOptions) {
		t.Helper()
		tmp := t.TempDir()
		store, err := NewSqlite(filepath.Join(tmp, "test.db"))
		if err != nil {
			t.Fatal(err)
		}
		exDir := filepath.Join(tmp, "exercises")
		atDir := filepath.Join(tmp, "blocks")
		stDir := filepath.Join(tmp, "sessions")
		mustWrite(t, exDir, "rows.yaml", `
name: "Ring Rows"
slug: "ring_rows"
kind: "reps_and_sets"
sets: 3
reps: 10
notes: "from the catalog"
`)
		mustWrite(t, atDir, "warmup.yaml", `
name: "Warm Up"
slug: "warm_up"
label: "warmup"
exercises:
  - ref: "Ring Rows"
`)
		mustWrite(t, stDir, "day.yaml", `
name: "Pull Day"
slug: "pull_day"
label: "strength"
activities:
  - ref: "Warm Up"
`)
		opts := YAMLImportOptions{
			OwnerID:              1,
			ExercisesDir:         []string{exDir},
			ActivityTemplatesDir: []string{atDir},
			SessionTemplatesDir:  []string{stDir},
		}
		if err := store.ImportYAML(opts); err != nil {
			t.Fatalf("first import: %v", err)
		}
		return store, opts
	}

	markEdited := func(t *testing.T, store *Store, model any, id uint) {
		t.Helper()
		now := time.Now()
		if err := store.DB.Model(model).Where("id = ?", id).
			Update("catalog_edited_at", now).Error; err != nil {
			t.Fatal(err)
		}
	}

	t.Run("edited library exercise keeps its fields", func(t *testing.T) {
		store, opts := setup(t)
		var ex LibraryExercise
		if err := store.DB.Where("name = ?", "Ring Rows").First(&ex).Error; err != nil {
			t.Fatal(err)
		}
		if err := store.DB.Model(&ex).Updates(map[string]any{"reps": 5, "notes": "my own cue"}).Error; err != nil {
			t.Fatal(err)
		}
		markEdited(t, store, &LibraryExercise{}, ex.ID)

		if err := store.ImportYAML(opts); err != nil {
			t.Fatalf("second import: %v", err)
		}
		var after LibraryExercise
		if err := store.DB.First(&after, ex.ID).Error; err != nil {
			t.Fatal(err)
		}
		if after.Reps != 5 {
			t.Fatalf("reps = %d, want 5 (the edit must survive re-import)", after.Reps)
		}
		if after.Notes != "my own cue" {
			t.Fatalf("notes = %q, want the edited value", after.Notes)
		}
		if !after.ManagedByCatalog {
			t.Fatal("managed_by_catalog must stay true — it records where the row came from")
		}
	})

	t.Run("edited block keeps its child exercises", func(t *testing.T) {
		store, opts := setup(t)
		var at ActivityTemplate
		if err := store.DB.Where("name = ?", "Warm Up").First(&at).Error; err != nil {
			t.Fatal(err)
		}
		var before []uint
		if err := store.DB.Model(&Exercise{}).
			Where("activity_template_id = ?", at.ID).
			Order("id asc").Pluck("id", &before).Error; err != nil {
			t.Fatal(err)
		}
		if len(before) == 0 {
			t.Fatal("block has no exercises to begin with")
		}
		markEdited(t, store, &ActivityTemplate{}, at.ID)

		if err := store.ImportYAML(opts); err != nil {
			t.Fatalf("second import: %v", err)
		}
		var after []uint
		if err := store.DB.Model(&Exercise{}).
			Where("activity_template_id = ?", at.ID).
			Order("id asc").Pluck("id", &after).Error; err != nil {
			t.Fatal(err)
		}
		if len(after) != len(before) {
			t.Fatalf("exercise count = %d, want %d", len(after), len(before))
		}
		for i := range before {
			if after[i] != before[i] {
				t.Fatalf("exercise ids changed: %v -> %v (the importer deleted and recreated them)", before, after)
			}
		}
	})

	t.Run("edited session keeps its activities", func(t *testing.T) {
		store, opts := setup(t)
		var st SessionTemplate
		if err := store.DB.Where("name = ?", "Pull Day").First(&st).Error; err != nil {
			t.Fatal(err)
		}
		var before []uint
		if err := store.DB.Model(&Activity{}).
			Where("session_template_id = ?", st.ID).
			Order("id asc").Pluck("id", &before).Error; err != nil {
			t.Fatal(err)
		}
		if len(before) == 0 {
			t.Fatal("session has no activities to begin with")
		}
		markEdited(t, store, &SessionTemplate{}, st.ID)

		if err := store.ImportYAML(opts); err != nil {
			t.Fatalf("second import: %v", err)
		}
		var after []uint
		if err := store.DB.Model(&Activity{}).
			Where("session_template_id = ?", st.ID).
			Order("id asc").Pluck("id", &after).Error; err != nil {
			t.Fatal(err)
		}
		if len(after) != len(before) {
			t.Fatalf("activity count = %d, want %d", len(after), len(before))
		}
		for i := range before {
			if after[i] != before[i] {
				t.Fatalf("activity ids changed: %v -> %v", before, after)
			}
		}
	})

	t.Run("renaming an edited row does not delete it", func(t *testing.T) {
		store, opts := setup(t)
		var ex LibraryExercise
		if err := store.DB.Where("name = ?", "Ring Rows").First(&ex).Error; err != nil {
			t.Fatal(err)
		}
		// A rename puts the row's name outside the YAML's keep set, which is what used
		// to make pruneCatalogOrphans delete it.
		if err := store.DB.Model(&ex).Update("name", "My Rows").Error; err != nil {
			t.Fatal(err)
		}
		markEdited(t, store, &LibraryExercise{}, ex.ID)

		if err := store.ImportYAML(opts); err != nil {
			t.Fatalf("second import: %v", err)
		}
		var count int64
		if err := store.DB.Model(&LibraryExercise{}).Where("id = ?", ex.ID).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatal("renamed edited row was deleted by prune")
		}
	})

	t.Run("an unedited row is still overwritten and still pruned", func(t *testing.T) {
		store, opts := setup(t)
		var ex LibraryExercise
		if err := store.DB.Where("name = ?", "Ring Rows").First(&ex).Error; err != nil {
			t.Fatal(err)
		}
		if err := store.DB.Model(&ex).Update("reps", 5).Error; err != nil {
			t.Fatal(err)
		}
		// No stamp this time.
		if err := store.ImportYAML(opts); err != nil {
			t.Fatalf("second import: %v", err)
		}
		var after LibraryExercise
		if err := store.DB.First(&after, ex.ID).Error; err != nil {
			t.Fatal(err)
		}
		if after.Reps != 10 {
			t.Fatalf("reps = %d, want 10 — an unstamped row must still be overwritten", after.Reps)
		}

		// And a rename with no stamp is still pruned.
		if err := store.DB.Model(&after).Update("name", "Orphan Rows").Error; err != nil {
			t.Fatal(err)
		}
		if err := store.ImportYAML(opts); err != nil {
			t.Fatalf("third import: %v", err)
		}
		var count int64
		if err := store.DB.Model(&LibraryExercise{}).Where("name = ?", "Orphan Rows").Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatal("unstamped rename orphan should have been pruned")
		}
	})
}
