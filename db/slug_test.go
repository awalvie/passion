package db

import (
	"path/filepath"
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
