package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// snapshot is every row in every catalog table, as sorted text. Comparing two snapshots
// catches a rewritten updated_at and a reissued row id, which a row count would not.
func snapshot(t *testing.T, s *Store) string {
	t.Helper()
	var b strings.Builder
	for _, tbl := range []string{
		"content", "content_set", "content_item", "content_item_set",
		"content_media", "tag", "content_tag",
	} {
		rows, err := s.read(context.Background()).Raw("SELECT * FROM " + tbl).Rows()
		if err != nil {
			t.Fatal(err)
		}
		cols, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		var lines []string
		for rows.Next() {
			cells := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range cells {
				ptrs[i] = &cells[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			var parts []string
			for i, c := range cells {
				parts = append(parts, fmt.Sprintf("%s=%v", cols[i], c))
			}
			lines = append(lines, strings.Join(parts, " "))
		}
		_ = rows.Close()
		sort.Strings(lines) // row order is not part of the state
		fmt.Fprintf(&b, "== %s (%d)\n%s\n", tbl, len(lines), strings.Join(lines, "\n"))
	}
	return b.String()
}

// shippedOnly is a one-movement tree, for tests that need a shipped row to exist next to
// an owned tree without the two sharing any slug.
func shippedOnly() fstest.MapFS {
	return fstest.MapFS{
		"catalog.yaml": &fstest.MapFile{Data: []byte("format_version: 1\n")},
		"tags.yaml":    &fstest.MapFile{Data: []byte("- slug: strength\n  name: Strength\n")},
		"movements/bench_press.yaml": &fstest.MapFile{Data: []byte(`
name: "Bench Press"
slug: "bench_press"
kind: "reps_and_sets"
tags: ["strength"]
sets: 3
reps: 5
`)},
	}
}

func loadGood(t *testing.T, fsys fstest.MapFS, name string) *Tree {
	t.Helper()
	tree, err := Load(fsys, name, nil)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestImportingTheShippedTree(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		res, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree))
		if err != nil {
			t.Fatalf("the import failed: %v", err)
		}
		if res.Inserted != 7 {
			t.Errorf("inserted %d rows, want 7 (3 movements, 1 menu, 2 blocks, 1 session): %s", res.Inserted, res)
		}

		// Shipped means no author, which puts it out of reach of any account's cascade.
		if n := count(t, s, `SELECT count(*) FROM content WHERE author_id IS NOT NULL`); n != 0 {
			t.Errorf("%d shipped rows have an author", n)
		}
		if n := count(t, s, `SELECT count(*) FROM content WHERE source_tree = ?`, ShippedTree); n != 7 {
			t.Errorf("%d rows carry the tree name, want 7", n)
		}
		if n := count(t, s, `SELECT count(*) FROM tag`); n != 3 {
			t.Errorf("%d tags, want 3", n)
		}

		// The edges, and the kinds on them. A session holds a block; a block holds a menu;
		// a menu holds movements.
		// 2 in the menu, 1 in each block, 2 in the session.
		if n := count(t, s, `SELECT count(*) FROM content_item`); n != 6 {
			t.Errorf("%d edges, want 6", n)
		}
		if n := count(t, s,
			`SELECT count(*) FROM content_item WHERE parent_kind='session' AND child_kind='block'`); n != 2 {
			t.Errorf("%d session-to-block edges, want 2", n)
		}
		if n := count(t, s,
			`SELECT count(*) FROM content_item WHERE parent_kind='menu' AND child_kind='movement'`); n != 2 {
			t.Errorf("%d menu-to-movement edges, want 2", n)
		}

		// The ladder landed on the movement, not on an edge.
		if n := count(t, s, `SELECT count(*) FROM content_set`); n != 3 {
			t.Errorf("%d ladder rows, want 3", n)
		}
		if n := count(t, s, `SELECT count(*) FROM content_item_set`); n != 0 {
			t.Errorf("%d per-use set rows, want 0", n)
		}

		// Every row got a key. No two share one, because a shared key means a fork and there
		// are none here.
		if n := count(t, s, `SELECT count(DISTINCT content_key) FROM content`); n != 7 {
			t.Errorf("%d distinct content keys across 7 rows", n)
		}
	})
}

// A second import of an unchanged tree must write nothing at all, rather than the same
// values again. Delete-and-reinsert would pass a row count and fail this.
func TestASecondImportChangesNothing(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		tree := loadGood(t, goodTree(), ShippedTree)

		if _, err := s.ImportShipped(ctx, tree); err != nil {
			t.Fatal(err)
		}
		before := snapshot(t, s)

		res, err := s.ImportShipped(ctx, tree)
		if err != nil {
			t.Fatal(err)
		}
		if res.Inserted != 0 || res.Updated != 0 || res.Retired != 0 {
			t.Errorf("a second import was not a no-op: %s", res)
		}
		if res.Unchanged != 7 {
			t.Errorf("%d rows reported unchanged, want 7: %s", res.Unchanged, res)
		}

		if after := snapshot(t, s); after != before {
			t.Errorf("a second import changed the database.\n--- before\n%s\n--- after\n%s", before, after)
		}
	})
}

// An edit updates the row and keeps its content_key. Finished runs point at the key, so
// renaming a movement must not detach its history.
func TestEditingAFileUpdatesTheRowAndKeepsItsKey(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}

		var before Content
		if err := s.read(ctx).Where("slug = ?", "general_warmup").First(&before).Error; err != nil {
			t.Fatal(err)
		}

		edited := with(goodTree(), "movements/general_warmup.yaml", `
name: "General Warm-up, revised"
slug: "general_warmup"
kind: "open"
tags: ["warmup", "strength"]
notes: |
  Ten minutes, easy.
`)
		res, err := s.ImportShipped(ctx, loadGood(t, edited, ShippedTree))
		if err != nil {
			t.Fatal(err)
		}
		if res.Updated != 1 || res.Unchanged != 6 {
			t.Errorf("one edited file gave %s", res)
		}

		var after Content
		if err := s.read(ctx).Where("slug = ?", "general_warmup").First(&after).Error; err != nil {
			t.Fatal(err)
		}
		if after.Name != "General Warm-up, revised" {
			t.Errorf("the name did not update: %q", after.Name)
		}
		if after.ContentKey != before.ContentKey {
			t.Error("the content key changed, which would detach every run that used it")
		}
		if after.ID != before.ID {
			t.Error("the row id changed")
		}
		if !after.UpdatedAt.After(before.UpdatedAt) {
			t.Error("updated_at did not move on a real change")
		}
		// The second tag arrived and the first stayed.
		if n := count(t, s,
			`SELECT count(*) FROM content_tag WHERE content_id = ?`, after.ID); n != 2 {
			t.Errorf("%d tags on the edited row, want 2", n)
		}
	})
}

// A file whose slug is already taken by something you made is refused. The importer must
// not overwrite your row, and it must not quietly leave it either: every other file
// pointing at that slug would then get your row instead of the one the tree describes, so
// the import would report success and the tree would mean something else.
func TestAFileCannotTakeASlugYouAlreadyUsed(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		one := int64(1)
		seedAccount(t, s, 1, "a@b.c")

		// A row this account typed in the app, sharing a slug with a file in the tree.
		mustExec(t, s, `INSERT INTO content
		     (kind,slug,name,content_key,author_id,movement_kind,created_at,updated_at)
		     VALUES ('movement','general_warmup','Mine, by hand',?,?,'open',?,?)`,
			"11111111-1111-1111-1111-111111111111", one, time.Now(), time.Now())

		_, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "a@b.c")
		if err == nil {
			t.Fatal("the import accepted a file whose slug a hand-made row already held")
		}
		for _, want := range []string{"general_warmup", "Rename yours"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the error does not mention %q: %v", want, err)
			}
		}

		// The whole import is one transaction, so a refusal leaves nothing behind.
		var mine Content
		if err := s.read(ctx).Where("slug = ? AND author_id = ?", "general_warmup", one).
			First(&mine).Error; err != nil {
			t.Fatal(err)
		}
		if mine.Name != "Mine, by hand" {
			t.Errorf("the row was changed: %q", mine.Name)
		}
		if n := count(t, s, `SELECT count(*) FROM content WHERE source_tree IS NOT NULL`); n != 0 {
			t.Errorf("%d rows from the tree survived a refused import", n)
		}
	})
}

// A private tree references content the shipped tree defines, rather than carrying its own
// copy of it. The real private tree does this 39 times.
func TestAPrivateTreeCanReferenceShippedContent(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "a@b.c")

		shipped := loadGood(t, shippedOnly(), ShippedTree)
		if _, err := s.ImportShipped(ctx, shipped); err != nil {
			t.Fatal(err)
		}

		// A private tree holding one block that points at a shipped movement, and nothing
		// of its own to point at.
		priv := fstest.MapFS{
			"catalog.yaml": &fstest.MapFile{Data: []byte("format_version: 1\n")},
			"blocks/my_block.yaml": &fstest.MapFile{Data: []byte(`
name: "My Block"
slug: "my_block"
tags: ["strength"]
items:
  - ref: "bench_press"
    sets: 5
`)},
		}

		// Loading needs the shipped tree's index, or the ref has nothing to check against.
		tree, err := Load(priv, "private", shipped.Index())
		if err != nil {
			t.Fatalf("a private tree could not reference a shipped slug: %v", err)
		}
		if _, err := s.ImportOwned(ctx, tree, "a@b.c"); err != nil {
			t.Fatal(err)
		}

		// The edge crosses the boundary: an owned parent, a shipped child.
		var got struct {
			ParentAuthor *int64
			ChildAuthor  *int64
			ChildSlug    string
			Sets         *int
		}
		if err := s.read(ctx).Raw(`
			SELECT p.author_id AS parent_author, c.author_id AS child_author,
			       c.slug AS child_slug, i.sets
			FROM content_item i
			JOIN content p ON p.id = i.parent_id
			JOIN content c ON c.id = i.child_id`).Scan(&got).Error; err != nil {
			t.Fatal(err)
		}
		if got.ParentAuthor == nil {
			t.Error("the block was not owned by the account")
		}
		if got.ChildAuthor != nil {
			t.Error("the shipped movement was copied instead of referenced")
		}
		if got.ChildSlug != "bench_press" {
			t.Errorf("the edge points at %q", got.ChildSlug)
		}
		if got.Sets == nil || *got.Sets != 5 {
			t.Errorf("the per-use number did not land: %v", got.Sets)
		}

		// Deleting the owner takes the block and leaves the shipped movement.
		if err := s.DeleteAccount(ctx, 1); err != nil {
			t.Fatalf("deleting the owner of a block that points at shipped content: %v", err)
		}
		if n := count(t, s, `SELECT count(*) FROM content WHERE slug='bench_press'`); n != 1 {
			t.Error("the shipped movement went with the account")
		}
		if n := count(t, s, `SELECT count(*) FROM content_item`); n != 0 {
			t.Errorf("%d edges outlived their owner", n)
		}
	})
}

func TestImportOwnedNeedsAnAccount(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		_, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "nobody@example.com")
		if !errors.Is(err, ErrNoSuchOwner) {
			t.Fatalf("error was %v, want ErrNoSuchOwner", err)
		}
		if n := count(t, s, `SELECT count(*) FROM content`); n != 0 {
			t.Errorf("%d rows were written for an owner that does not exist", n)
		}

		// The same tree imports once the account exists. That is the first-boot path: warn,
		// carry on, import on the next start.
		seedAccount(t, s, 1, "later@example.com")
		res, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "later@example.com")
		if err != nil {
			t.Fatalf("the import failed once the account existed: %v", err)
		}
		if res.Inserted != 7 {
			t.Errorf("inserted %d, want 7: %s", res.Inserted, res)
		}
	})
}

// A private tree belongs to one account. Another account on the same machine sees none of
// it, and deleting the owner takes it.
func TestAPrivateTreeIsOwnedAndNotVisibleToAnotherAccount(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "owner@example.com")
		seedAccount(t, s, 2, "stranger@example.com")

		if _, err := s.ImportShipped(ctx, loadGood(t, shippedOnly(), ShippedTree)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "owner@example.com"); err != nil {
			t.Fatal(err)
		}

		// What each account can see: shipped, plus its own.
		visible := func(id int64) int64 {
			return count(t, s,
				`SELECT count(*) FROM content WHERE author_id IS NULL OR author_id = ?`, id)
		}
		if got, want := visible(1), int64(1+7); got != want {
			t.Errorf("the owner sees %d rows, want %d", got, want)
		}
		if got, want := visible(2), int64(1); got != want {
			t.Errorf("the stranger sees %d rows, want %d — the private tree leaked", got, want)
		}

		// Deleting the owner takes the private tree and leaves the shipped row.
		if err := s.DeleteAccount(ctx, 1); err != nil {
			t.Fatal(err)
		}
		if n := count(t, s, `SELECT count(*) FROM content WHERE author_id IS NULL`); n != 1 {
			t.Errorf("%d shipped rows survived, want 1", n)
		}
		if n := count(t, s, `SELECT count(*) FROM content`); n != 1 {
			t.Errorf("%d rows left, want 1: the private tree should have gone with its owner", n)
		}
	})
}

// A file leaving the tree retires its row rather than deleting it, because a plan or a
// finished run may still point at it.
func TestARemovedFileRetiresItsRowAndComingBackUnretiresIt(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}

		// Drop the ladder. It is only referenced by the menu, so the tree stays valid.
		reduced := goodTree()
		delete(reduced, "movements/hangboard_ladder.yaml")
		reduced = with(reduced, "menus/hang_choice.yaml", `
name: "Pick a hang"
slug: "hang_choice"
pick: 1
options:
  - ref: "half_crimp_hang"
`)
		res, err := s.ImportShipped(ctx, loadGood(t, reduced, ShippedTree))
		if err != nil {
			t.Fatal(err)
		}
		if res.Retired != 1 {
			t.Errorf("%d rows retired, want 1: %s", res.Retired, res)
		}
		if n := count(t, s,
			`SELECT count(*) FROM content WHERE slug='hangboard_ladder' AND retired_on IS NOT NULL`); n != 1 {
			t.Error("the removed movement was not retired")
		}
		// Retired, not deleted.
		if n := count(t, s, `SELECT count(*) FROM content WHERE slug='hangboard_ladder'`); n != 1 {
			t.Error("the removed movement was deleted rather than retired")
		}

		// Put it back.
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}
		if n := count(t, s,
			`SELECT count(*) FROM content WHERE slug='hangboard_ladder' AND retired_on IS NULL`); n != 1 {
			t.Error("the movement came back but stayed retired")
		}
	})
}

// Two trees cannot both claim one slug for one owner. Letting the last import win would
// make the result depend on the order the trees are configured in.
func TestTwoTreesCannotClaimOneSlug(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}
		_, err := s.ImportShipped(ctx, loadGood(t, goodTree(), "another"))
		if err == nil {
			t.Fatal("a second tree claimed the same slugs")
		}
		if !strings.Contains(err.Error(), ShippedTree) || !strings.Contains(err.Error(), "another") {
			t.Errorf("the error does not name both trees: %v", err)
		}
	})
}

// Removing an item from a list deletes that edge and leaves the others, with positions
// renumbered from the file rather than left with a gap.
func TestChangingAListRewritesItsEdges(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}
		if n := count(t, s, `SELECT count(*) FROM content_item WHERE parent_kind='session'`); n != 2 {
			t.Fatalf("%d session edges to begin with", n)
		}

		swapped := with(goodTree(), "sessions/finger_day.yaml", `
name: "Finger Day"
slug: "finger_day"
color: "#ef4444"
tags: ["fingers"]
items:
  - ref: "fingers_block"
  - ref: "warm_up"
`)
		if _, err := s.ImportShipped(ctx, loadGood(t, swapped, ShippedTree)); err != nil {
			t.Fatal(err)
		}

		var got []struct {
			Slug     string
			Position int
		}
		if err := s.read(ctx).Raw(`
			SELECT c.slug, i.position FROM content_item i
			JOIN content c ON c.id = i.child_id
			WHERE i.parent_kind='session' ORDER BY i.position`).Scan(&got).Error; err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Slug != "fingers_block" || got[1].Slug != "warm_up" {
			t.Errorf("the reordered list read back as %+v", got)
		}
	})
}
