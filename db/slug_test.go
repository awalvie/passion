package db

import (
	"path/filepath"
	"strings"
	"testing"
)

// Slugify has to match the generator that wrote the YAML trees exactly. If it does not, a
// backfilled row and its YAML entry never find each other, and the importer treats every
// catalog entry as new.
func TestSlugifyMatchesTheGeneratorUsedOnTheYAML(t *testing.T) {
	cases := map[string]string{
		"Density Hangs (20 mm Edge)": "density_hangs_20_mm_edge",
		"Drills (Paradigm)":          "drills_paradigm",
		"Weighted Pull-ups":          "weighted_pull_ups",
		"Do More | Power Company":    "do_more_power_company",
		"Eyes, Feet, Hips, Hands":    "eyes_feet_hips_hands",
		"90/90 Hip Rotation":         "90_90_hip_rotation",
		"Farmer's Carry":             "farmers_carry",
		"  Padded  Spaces  ":         "padded_spaces",
		"Final Exam I":               "final_exam_i",
	}
	for name, want := range cases {
		if got := Slugify(name); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestBackfillSlugsFillsEveryEmptyRow(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "slug-backfill.db"))
	if err != nil {
		t.Fatal(err)
	}
	seedImportOwner(t, store, 1)
	mustCreate(t, store, &LibraryExercise{OwnerID: 1, Name: "Max Hangs"})
	mustCreate(t, store, &ActivityTemplate{OwnerID: 1, Name: "Warm Up"})
	mustCreate(t, store, &SessionTemplate{OwnerID: 1, Name: "Boulder Session"})
	// One row already has a slug and must be left exactly as it is.
	keep := LibraryExercise{OwnerID: 1, Name: "Pull-ups", Slug: "handwritten"}
	mustCreate(t, store, &keep)

	rep, err := BackfillSlugs(store.DB, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 3 {
		t.Fatalf("want 3 rows filled, got %d (%v)", rep.Total, rep.ByTable)
	}

	var lib LibraryExercise
	if err := store.DB.Where("name = ?", "Max Hangs").First(&lib).Error; err != nil {
		t.Fatal(err)
	}
	if lib.Slug != "max_hangs" {
		t.Errorf("slug is %q, want max_hangs", lib.Slug)
	}
	var untouched LibraryExercise
	if err := store.DB.Where("id = ?", keep.ID).First(&untouched).Error; err != nil {
		t.Fatal(err)
	}
	if untouched.Slug != "handwritten" {
		t.Errorf("an existing slug was overwritten: %q", untouched.Slug)
	}
}

// Safe to run twice, because a boot-time migration will.
func TestBackfillSlugsIsIdempotent(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "slug-twice.db"))
	if err != nil {
		t.Fatal(err)
	}
	seedImportOwner(t, store, 1)
	mustCreate(t, store, &LibraryExercise{OwnerID: 1, Name: "Max Hangs"})

	if _, err := BackfillSlugs(store.DB, false); err != nil {
		t.Fatal(err)
	}
	rep, err := BackfillSlugs(store.DB, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 0 {
		t.Fatalf("a second pass filled %d rows", rep.Total)
	}
}

// Slug is unique per owner, so two rows one account named the same thing cannot share one.
// It is worth reporting rather than silently numbering, because the importer is about to
// match on it.
func TestBackfillSlugsSeparatesAndReportsCollisions(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "slug-collide.db"))
	if err != nil {
		t.Fatal(err)
	}
	seedImportOwner(t, store, 1)
	mustCreate(t, store, &LibraryExercise{OwnerID: 1, Name: "Max Hangs"})
	mustCreate(t, store, &LibraryExercise{OwnerID: 1, Name: "max hangs"})
	// The same name under a different account is not a collision: slug is per owner.
	mustCreate(t, store, &LibraryExercise{OwnerID: 2, Name: "Max Hangs"})

	rep, err := BackfillSlugs(store.DB, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Collisions) != 1 {
		t.Errorf("want 1 collision reported, got %v", rep.Collisions)
	}

	var slugs []string
	if err := store.DB.Model(&LibraryExercise{}).Where("owner_id = ?", 1).
		Order("id").Pluck("slug", &slugs).Error; err != nil {
		t.Fatal(err)
	}
	if len(slugs) != 2 || slugs[0] == slugs[1] {
		t.Fatalf("the two rows share a slug: %v", slugs)
	}
	var other string
	if err := store.DB.Model(&LibraryExercise{}).Where("owner_id = ?", 2).
		Pluck("slug", &other).Error; err != nil {
		t.Fatal(err)
	}
	if other != "max_hangs" {
		t.Errorf("a different owner should keep the plain slug, got %q", other)
	}
}

func TestBackfillSlugsDryRunChangesNothing(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "slug-dry.db"))
	if err != nil {
		t.Fatal(err)
	}
	seedImportOwner(t, store, 1)
	mustCreate(t, store, &LibraryExercise{OwnerID: 1, Name: "Max Hangs"})

	rep, err := BackfillSlugs(store.DB, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Total != 1 {
		t.Fatalf("the dry run should report 1 row, reported %d", rep.Total)
	}
	var slug string
	if err := store.DB.Model(&LibraryExercise{}).Pluck("slug", &slug).Error; err != nil {
		t.Fatal(err)
	}
	if slug != "" {
		t.Errorf("a dry run wrote a slug: %q", slug)
	}
}

// This is the test that was missing, and the reason production ended up with two copies of
// its whole catalog on 4 September 2026.
//
// The importer matches on slug and runs automatically at boot. A deploy that carries the
// match-key switch therefore imports against whatever the database holds at that moment.
// The backfill existed and was tested, but it lived in a command an operator had to
// remember — so the deploy ran the import first, nothing matched, and the catalog was
// duplicated. The importer now refuses to run against unslugged catalog rows rather than
// silently backfilling them: numbering colliding slugs apart is a decision an operator
// should read a dry run for, not something an import does on its way past.
func TestImportRefusesToRunAgainstUnsluggedCatalogRows(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSqlite(filepath.Join(dir, "unslugged.db"))
	if err != nil {
		t.Fatal(err)
	}
	seedImportOwner(t, store, 1)
	for _, row := range []any{
		&LibraryExercise{OwnerID: 1, Name: "Weighted Pull-ups", ManagedByCatalog: true},
		&ActivityTemplate{OwnerID: 1, Name: "Warm Up", ManagedByCatalog: true},
		&SessionTemplate{OwnerID: 1, Name: "Strength Day", ManagedByCatalog: true},
	} {
		if err := store.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	// A database as it stood before the switch: catalog rows, no slugs.
	for _, tbl := range []string{"library_exercises", "activity_templates", "session_templates"} {
		if err := store.DB.Exec("UPDATE " + tbl + " SET slug = ''").Error; err != nil {
			t.Fatal(err)
		}
	}

	opts := writeMinimalImportFixture(t, dir)
	before := countCatalogRows(t, store)
	err = store.ImportYAML(opts)
	if err == nil {
		t.Fatal("import ran against unslugged rows; it would duplicate them and prune the originals")
	}
	if !strings.Contains(err.Error(), "no slug") {
		t.Fatalf("error should name the cause, got: %v", err)
	}
	if after := countCatalogRows(t, store); after != before {
		t.Errorf("a refused import still changed the row count from %d to %d", before, after)
	}
}

// And once the backfill has run, the import matches instead of creating a second copy.
// This is the pairing that makes the refusal above a guard rather than a wall.
func TestImportMatchesBySlugOnceTheBackfillHasRun(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSqlite(filepath.Join(dir, "backfilled.db"))
	if err != nil {
		t.Fatal(err)
	}
	seedImportOwner(t, store, 1)
	for _, row := range []any{
		&LibraryExercise{OwnerID: 1, Name: "Weighted Pull-ups", ManagedByCatalog: true},
		&ActivityTemplate{OwnerID: 1, Name: "Warm Up", ManagedByCatalog: true},
		&SessionTemplate{OwnerID: 1, Name: "Strength Day", ManagedByCatalog: true},
	} {
		if err := store.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, tbl := range []string{"library_exercises", "activity_templates", "session_templates"} {
		if err := store.DB.Exec("UPDATE " + tbl + " SET slug = ''").Error; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := BackfillSlugs(store.DB, false); err != nil {
		t.Fatal(err)
	}

	opts := writeMinimalImportFixture(t, dir)
	before := countCatalogRows(t, store)
	if err := store.ImportYAML(opts); err != nil {
		t.Fatalf("import after backfill: %v", err)
	}
	if after := countCatalogRows(t, store); after != before {
		t.Errorf("the import changed the row count from %d to %d; every entry matched an "+
			"existing row by slug, so nothing should have been created", before, after)
	}
}

// writeSlugBootFixture writes a minimal catalog whose one entry matches the seeded row by
// slug, plus one session, so the import has something to resolve.

func countCatalogRows(t *testing.T, store *Store) int64 {
	t.Helper()
	var total int64
	for _, tbl := range []string{"library_exercises", "activity_templates", "session_templates"} {
		var n int64
		if err := store.DB.Table(tbl).Where("deleted_at IS NULL").Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		total += n
	}
	return total
}

// The other half of the same asymmetry, stated directly so the failure above is easy to
// read. If the guard is meant to count soft-deleted rows, the backfill has to fill them.
func TestBackfillSlugsFillsSoftDeletedCatalogRows(t *testing.T) {
	store, err := NewSqlite(filepath.Join(t.TempDir(), "softbackfill.db"))
	if err != nil {
		t.Fatal(err)
	}
	seedImportOwner(t, store, 1)

	row := LibraryExercise{OwnerID: 1, Name: "Retired Hangs", ManagedByCatalog: true}
	mustCreate(t, store, &row)
	if err := store.DB.Exec("UPDATE library_exercises SET slug = ''").Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Delete(&LibraryExercise{}, row.ID).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := BackfillSlugs(store.DB, false); err != nil {
		t.Fatal(err)
	}

	var slug string
	if err := store.DB.Table("library_exercises").
		Where("id = ?", row.ID).Pluck("slug", &slug).Error; err != nil {
		t.Fatal(err)
	}
	if slug == "" {
		t.Error("the backfill skipped a soft-deleted catalog row, but refuseUnsluggedCatalog counts it")
	}
}
