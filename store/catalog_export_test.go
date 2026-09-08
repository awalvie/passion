package store

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"gorm.io/gorm/logger"
)

// shape is the catalog as meaning rather than as rows: slugs, names, numbers, tags, and
// edges named by the slugs at each end. Row ids and content keys are left out, because a
// fresh database issues new ones and comparing them would prove nothing.
func shape(t *testing.T, s *Store) string {
	t.Helper()
	ctx := context.Background()
	var out []string

	var rows []Content
	if err := s.read(ctx).Order("kind, slug").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		tags, err := tagSlugs(s.read(ctx), r.ID)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprintf(
			"content %s/%s name=%q notes=%q source=%q per_side=%v kind=%v color=%q needs=%q pick=%v role=%v "+
				"sets=%v reps=%v kg=%v reps_s=%v rep_rest=%v set_rest=%v prep=%v secs=%v tags=%v",
			r.Kind, r.Slug, r.Name, r.Notes, r.Source, r.PerSide, deref(r.MovementKind),
			r.Color, r.Needs, derefI(r.PickCount), deref(r.BlockKind),
			derefI(r.DSets), derefI(r.DReps), derefF(r.DWeightKg), derefI(r.DRepSeconds),
			derefI(r.DRepRestSeconds), derefI(r.DSetRestSeconds), derefI(r.DPrepSeconds),
			derefI(r.DSeconds), tags))

		var sets []ContentSet
		if err := s.read(ctx).Where("content_id = ?", r.ID).
			Order("set_index, rep_index").Find(&sets).Error; err != nil {
			t.Fatal(err)
		}
		for _, cs := range sets {
			out = append(out, fmt.Sprintf("content_set %s set=%d rep=%d reps=%v kg=%v secs=%v",
				r.Slug, cs.SetIndex, cs.RepIndex, derefI(cs.Reps), derefF(cs.WeightKg), derefI(cs.Seconds)))
		}

		var media []ContentMedia
		if err := s.read(ctx).Where("content_id = ?", r.ID).
			Order("position").Find(&media).Error; err != nil {
			t.Fatal(err)
		}
		for _, m := range media {
			out = append(out, fmt.Sprintf("media %s pos=%d url=%q thumb=%q",
				r.Slug, m.Position, m.URL, m.ThumbURL))
		}
	}

	var edges []struct {
		Parent, Child string
		Position      int
		Sets, Reps    *int
	}
	if err := s.read(ctx).Raw(`
		SELECT p.slug AS parent, c.slug AS child, i.position, i.sets, i.reps
		FROM content_item i
		JOIN content p ON p.id = i.parent_id
		JOIN content c ON c.id = i.child_id`).Scan(&edges).Error; err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		out = append(out, fmt.Sprintf("edge %s -> %s pos=%d sets=%v reps=%v",
			e.Parent, e.Child, e.Position, derefI(e.Sets), derefI(e.Reps)))
	}

	sort.Strings(out)
	return strings.Join(out, "\n")
}

func deref(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func derefI(p *int) string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprint(*p)
}

func derefF(p *float64) string {
	if p == nil {
		return "<nil>"
	}
	return fmt.Sprint(*p)
}

// freshSQLite is the destination for a round trip: a database that has never seen the tree.
// Always SQLite, even when the source is Postgres, because every Postgres test shares one
// schema on one server and a second store would land in the same one. What is under test
// here is the format, not the engine.
func freshSQLite(t *testing.T) *Store {
	t.Helper()
	return open(t, Config{
		Engine:   EngineSQLite,
		DSN:      filepath.Join(t.TempDir(), "roundtrip.db"),
		LogLevel: logger.Silent,
	})
}

// asFS turns an export back into something the loader can read, so a round trip needs no
// temporary directory.
func asFS(files map[string][]byte) fstest.MapFS {
	out := fstest.MapFS{}
	for p, b := range files {
		out[p] = &fstest.MapFile{Data: b}
	}
	return out
}

// The round trip. Import a tree, write it back out, load that into an empty database, and
// the catalog must mean exactly the same thing. This is what proves no column is dropped on
// the way out — the failure a one-way importer cannot detect.
func TestExportAndReimportMeansTheSameThing(t *testing.T) {
	eachEngine(t, func(t *testing.T, first *Store) {
		ctx := context.Background()
		if _, err := first.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}
		before := shape(t, first)

		files, err := first.ExportTree(ctx, ShippedTree, nil)
		if err != nil {
			t.Fatalf("exporting: %v", err)
		}
		for _, want := range []string{
			"catalog.yaml", "tags.yaml",
			"movements/half_crimp_hang.yaml", "movements/hangboard_ladder.yaml",
			"menus/hang_choice.yaml", "blocks/warm_up.yaml", "sessions/finger_day.yaml",
		} {
			if _, ok := files[want]; !ok {
				t.Errorf("the export is missing %s", want)
			}
		}

		// Straight back in, to a database that has never seen this tree.
		second := freshSQLite(t)
		tree, err := Load(asFS(files), ShippedTree, nil)
		if err != nil {
			t.Fatalf("the export does not load: %v\n%s", err, dumpFiles(files))
		}
		if _, err := second.ImportShipped(ctx, tree); err != nil {
			t.Fatalf("re-importing the export: %v", err)
		}

		if after := shape(t, second); after != before {
			t.Errorf("the round trip changed the catalog.\n--- before\n%s\n--- after\n%s", before, after)
		}
	})
}

// Exporting twice must give the same bytes, or a tree kept in git would churn on every run.
func TestExportingTwiceGivesTheSameFiles(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}
		a, err := s.ExportTree(ctx, ShippedTree, nil)
		if err != nil {
			t.Fatal(err)
		}
		b, err := s.ExportTree(ctx, ShippedTree, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(a) != len(b) {
			t.Fatalf("%d files then %d files", len(a), len(b))
		}
		for p := range a {
			if string(a[p]) != string(b[p]) {
				t.Errorf("%s differs between two exports:\n%s\n---\n%s", p, a[p], b[p])
			}
		}
	})
}

// An absent number must not come back as a key. Otherwise every export would fill in zeros
// and the difference between "no target" and "a target of zero" would be lost.
func TestExportOmitsWhatWasNeverSet(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}
		files, err := s.ExportTree(ctx, ShippedTree, nil)
		if err != nil {
			t.Fatal(err)
		}

		// general_warmup carries a kind and tags and nothing else.
		got := string(files["movements/general_warmup.yaml"])
		for _, absent := range []string{"sets:", "reps:", "weight_kg:", "media:", "per_side:", "source:"} {
			if strings.Contains(got, absent) {
				t.Errorf("the export wrote %s for a movement that never had it:\n%s", absent, got)
			}
		}
		for _, present := range []string{"name:", "slug:", "kind:", "tags:"} {
			if !strings.Contains(got, present) {
				t.Errorf("the export dropped %s:\n%s", present, got)
			}
		}
	})
}

// A private tree exports without a tags.yaml, because the vocabulary belongs to the shipped
// tree. An export that carried a copy would be a second list to keep in step.
func TestExportingAPrivateTreeLeavesTheVocabularyAlone(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "a@b.c")
		if _, err := s.ImportShipped(ctx, loadGood(t, shippedOnly(), ShippedTree)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "a@b.c"); err != nil {
			t.Fatal(err)
		}

		one := int64(1)
		files, err := s.ExportTree(ctx, "private", &one)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := files["tags.yaml"]; ok {
			t.Error("the private tree exported a copy of the vocabulary")
		}
		if _, ok := files["movements/half_crimp_hang.yaml"]; !ok {
			t.Error("the private tree's own movements did not export")
		}
		// The shipped movement belongs to the other tree and must not appear here.
		if _, ok := files["movements/bench_press.yaml"]; ok {
			t.Error("the export pulled in a row from another tree")
		}
	})
}

func dumpFiles(files map[string][]byte) string {
	var paths []string
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var b strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&b, "--- %s\n%s\n", p, files[p])
	}
	return b.String()
}
