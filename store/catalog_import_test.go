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
			ptrs := make([]any, len(cells))
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
	return withIDs(fstest.MapFS{
		"catalog.yaml": &fstest.MapFile{Data: []byte("format_version: 2\n")},
		"movements/bench_press.yaml": &fstest.MapFile{Data: []byte(`
name: "Bench Press"
style: "reps_and_sets"
tags: ["strength"]
sets: 3
reps: 5
`)},
	})
}

func loadGood(t *testing.T, fsys fstest.MapFS, name string) *Tree {
	t.Helper()
	tree, err := Load(fsys, name, nil)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// goodTree holds 3 movements, 2 blocks, 1 session and 1 menu inside a block: 7 rows.
const goodTreeRows = 7

func TestImportingTheShippedTree(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		res, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree))
		if err != nil {
			t.Fatalf("the import failed: %v", err)
		}
		if res.Inserted != goodTreeRows {
			t.Errorf("inserted %d rows, want %d: %s", res.Inserted, goodTreeRows, res)
		}

		// Shipped means no author, which puts it out of reach of any account's cascade.
		if n := count(t, s, `SELECT count(*) FROM content WHERE author_id IS NOT NULL`); n != 0 {
			t.Errorf("%d shipped rows have an author", n)
		}
		if n := count(t, s, `SELECT count(*) FROM content WHERE source_tree = ?`, ShippedTree); n != goodTreeRows {
			t.Errorf("%d rows carry the tree name, want %d", n, goodTreeRows)
		}
		if n := count(t, s, `SELECT count(*) FROM tag`); n != 3 {
			t.Errorf("%d tags, want 3", n)
		}

		// The menu is a row, with the slug its position gives it. Nobody types this name.
		if n := count(t, s,
			`SELECT count(*) FROM content WHERE kind='menu' AND slug='fingers_0'`); n != 1 {
			t.Error("the inline menu did not become a row named after its block and position")
		}

		// The edges, and the kinds on them. A session holds a block; a block holds a menu;
		// a menu holds movements. 2 in the menu, 1 in each block, 2 in the session.
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
	})
}

// A second import of an unchanged tree must write nothing at all, rather than the same
// values again. Delete-and-reinsert would pass a row count and fail this.
//
// This is also what proves a menu is matched rather than rebuilt. A menu has no file, so
// the obvious implementation deletes and recreates it on every start, and this test is what
// says no.
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
		if res.Unchanged != goodTreeRows {
			t.Errorf("%d rows reported unchanged, want %d: %s", res.Unchanged, goodTreeRows, res)
		}

		if after := snapshot(t, s); after != before {
			t.Errorf("a second import changed the database.\n--- before\n%s\n--- after\n%s", before, after)
		}
	})
}

// An edit updates the row in place and keeps its id. movement_pref points at an id, so a
// re-import that reissued one would silently drop a person's saved numbers.
func TestEditingAFileUpdatesTheRowAndKeepsItsID(t *testing.T) {
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
style: "open"
tags: ["warmup", "strength"]
notes: |
  Ten minutes, easy.
`)
		res, err := s.ImportShipped(ctx, loadGood(t, edited, ShippedTree))
		if err != nil {
			t.Fatal(err)
		}
		if res.Updated != 1 || res.Unchanged != goodTreeRows-1 {
			t.Errorf("one edited file gave %s", res)
		}

		var after Content
		if err := s.read(ctx).Where("slug = ?", "general_warmup").First(&after).Error; err != nil {
			t.Fatal(err)
		}
		if after.Name != "General Warm-up, revised" {
			t.Errorf("the name did not update: %q", after.Name)
		}
		if after.ID != before.ID {
			t.Error("the row id changed, which would orphan any movement_pref pointing at it")
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

// A row edited in the app is SKIPPED, and named in the result. Not refused.
//
// Refusing was the old behaviour and it was wrong. Editing a file-owned row in the app is
// ordinary, and it clears source_tree — so a refusal would stop the whole tree importing
// from then on, on every start, because of one edit. The report is what stops the skip
// being silent.
func TestARowEditedInTheAppIsSkippedAndReported(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		one := int64(1)
		seedAccount(t, s, 1, "a@b.c")

		// The row this file owns, edited in the app. It keeps the file's id -- that is what
		// makes it the same row -- and loses its source_tree, which is what "edited here"
		// means.
		fileID := testID("movements/general_warmup.yaml")
		mustExec(t, s, `INSERT INTO content
		     (uuid,family,kind,slug,name,author_id,movement_style,created_at,updated_at)
		     VALUES (?,?,'movement','general_warmup','Mine, by hand',?,'open',?,?)`,
			fileID, fileID, one, time.Now(), time.Now())

		res, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "a@b.c")
		if err != nil {
			t.Fatalf("the import refused rather than skipping: %v", err)
		}
		if len(res.Skipped) != 1 {
			t.Fatalf("skipped %v, want exactly the one edited file", res.Skipped)
		}
		if !strings.Contains(res.Skipped[0], "general_warmup") {
			t.Errorf("the report does not name the file: %q", res.Skipped[0])
		}

		// The person's row is untouched.
		var mine Content
		if err := s.read(ctx).Where("slug = ? AND author_id = ?", "general_warmup", one).
			First(&mine).Error; err != nil {
			t.Fatal(err)
		}
		if mine.Name != "Mine, by hand" {
			t.Errorf("the import overwrote the row: %q", mine.Name)
		}
		if mine.SourceTree != nil {
			t.Errorf("the import re-attached the row to the tree: %v", *mine.SourceTree)
		}
		// And the rest of the tree still imported.
		if res.Inserted != goodTreeRows-1 {
			t.Errorf("inserted %d, want %d: one skip must not stop the tree",
				res.Inserted, goodTreeRows-1)
		}
	})
}

// A row typed in the app from scratch, taking a slug a file also wants, is two different
// things wanting one name. The importer says which two, rather than letting the unique index
// fail with a constraint name.
func TestASlugHeldByAHandTypedRowIsRefusedByName(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		one := int64(1)
		seedAccount(t, s, 1, "a@b.c")
		mustExec(t, s, `INSERT INTO content
		     (uuid,family,kind,slug,name,author_id,movement_style,created_at,updated_at)
		     VALUES (?,?,'movement','general_warmup','Mine, by hand',?,'open',?,?)`,
			testUUID(900), testUUID(900), one, time.Now(), time.Now())

		_, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "a@b.c")
		if err == nil {
			t.Fatal("two rows were allowed to claim one slug")
		}
		for _, want := range []string{"general_warmup", "edited in the app"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the error does not mention %q: %v", want, err)
			}
		}
	})
}

// A skipped row keeps its own children. One map used to serve both reference resolution and
// the decision about what to write, so a skipped parent's edges were rewritten from the
// file anyway.
func TestASkippedRowKeepsItsChildren(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		one := int64(1)
		seedAccount(t, s, 1, "a@b.c")
		now := time.Now()

		// A block the person made in the app, holding a movement of their own. It takes the
		// slug the tree's own block wants.
		blockID := testID("blocks/warm_up.yaml")
		mustExec(t, s, `INSERT INTO content
		     (uuid,family,id,kind,slug,name,author_id,movement_style,created_at,updated_at)
		     VALUES (?,?,900,'movement','my_own_move','Mine',?,'open',?,?)`,
			testUUID(900), testUUID(900), one, now, now)
		mustExec(t, s, `INSERT INTO content
		     (uuid,family,id,kind,slug,name,author_id,created_at,updated_at)
		     VALUES (?,?,901,'block','warm_up','My Warm-up',?,?,?)`,
			blockID, blockID, one, now, now)
		mustExec(t, s, `INSERT INTO content_item
		     (parent_id,parent_kind,child_id,child_kind,position)
		     VALUES (901,'block',900,'movement',0)`)

		if _, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "a@b.c"); err != nil {
			t.Fatal(err)
		}

		var kids []string
		if err := s.read(ctx).Raw(`
			SELECT c.slug FROM content_item i JOIN content c ON c.id = i.child_id
			WHERE i.parent_id = 901 ORDER BY i.position`).Scan(&kids).Error; err != nil {
			t.Fatal(err)
		}
		if len(kids) != 1 || kids[0] != "my_own_move" {
			t.Errorf("the import rewrote a skipped block's children: %v, want [my_own_move]", kids)
		}
	})
}

// A private tree references content the shipped tree defines, rather than carrying its own
// copy. The real private tree does this 39 times, and every one of them now says app:.
func TestAPrivateTreeCanReferenceShippedContent(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "a@b.c")

		shipped := loadGood(t, shippedOnly(), ShippedTree)
		if _, err := s.ImportShipped(ctx, shipped); err != nil {
			t.Fatal(err)
		}

		priv := withIDs(fstest.MapFS{
			"catalog.yaml": &fstest.MapFile{Data: []byte("format_version: 2\n")},
			"blocks/my_block.yaml": &fstest.MapFile{Data: []byte(`
name: "My Block"
tags: ["strength"]
items:
  - movement: "app:bench_press"
    sets: 5
`)},
		})

		// Loading needs the app's index, or the reference has nothing to check against.
		tree, err := Load(priv, "private", shipped.Index(true))
		if err != nil {
			t.Fatalf("a private tree could not reference the app's catalog: %v", err)
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

// A bare name never reaches the app's catalog, at the importer as well as the loader. The
// loader can be given the wrong index; the importer is the one that touches rows.
func TestABareNameDoesNotResolveToAppContentAtImport(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "a@b.c")
		if _, err := s.ImportShipped(ctx, loadGood(t, shippedOnly(), ShippedTree)); err != nil {
			t.Fatal(err)
		}

		priv := withIDs(fstest.MapFS{
			"catalog.yaml": &fstest.MapFile{Data: []byte("format_version: 2\n")},
			"blocks/my_block.yaml": &fstest.MapFile{Data: []byte(`
name: "My Block"
items:
  - movement: "bench_press"
`)},
		})
		// The loader is told, wrongly, that a bare bench_press exists in this tree.
		tree, err := Load(priv, "private", map[Ref]bool{
			{Kind: KindMovement, Slug: "bench_press"}: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.ImportOwned(ctx, tree, "a@b.c")
		if err == nil {
			t.Fatal("a bare name resolved to the app's catalog at import time")
		}
		if !strings.Contains(err.Error(), "bench_press") {
			t.Errorf("the error does not name the reference: %v", err)
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
		if res.Inserted != goodTreeRows {
			t.Errorf("inserted %d, want %d: %s", res.Inserted, goodTreeRows, res)
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
		if got, want := visible(1), int64(1+goodTreeRows); got != want {
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

		// Drop the ladder, and the menu option that named it.
		reduced := without(goodTree(), "movements/hangboard_ladder.yaml")
		reduced = with(reduced, "blocks/fingers.yaml", `
name: "Fingers"
tags: ["fingers"]
items:
  - menu:
      name: "Pick a hang"
      pick: 1
      notes: |
        One of these per session.
      of: ["half_crimp_hang"]
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

// A menu is DELETED when it leaves its block, not retired. Nothing can reference a menu,
// its slug is derived rather than typed, and a log freezes a block's name as text — so
// there is nothing a retired menu would protect. A retired one would also keep its derived
// slug, and the next import of a shortened block could not reuse it.
func TestAMenuIsDeletedWhenItLeavesItsBlock(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}
		if n := count(t, s, `SELECT count(*) FROM content WHERE kind='menu'`); n != 1 {
			t.Fatalf("%d menus to begin with", n)
		}

		// Replace the menu with a plain movement.
		flat := with(goodTree(), "blocks/fingers.yaml", `
name: "Fingers"
tags: ["fingers"]
items:
  - movement: "half_crimp_hang"
`)
		if _, err := s.ImportShipped(ctx, loadGood(t, flat, ShippedTree)); err != nil {
			t.Fatal(err)
		}
		if n := count(t, s, `SELECT count(*) FROM content WHERE kind='menu'`); n != 0 {
			t.Errorf("%d menu rows left, want 0", n)
		}
		if n := count(t, s, `SELECT count(*) FROM content_item WHERE child_kind='menu'`); n != 0 {
			t.Errorf("%d edges still point at a deleted menu", n)
		}
		// The ladder lost its only reference but is retired, not deleted.
		if n := count(t, s,
			`SELECT count(*) FROM content WHERE slug='hangboard_ladder' AND retired_on IS NULL`); n != 1 {
			t.Error("a movement that lost its reference was retired or deleted; it should be neither")
		}
	})
}

// Reordering a block reuses the derived slugs rather than piling up new ones. A menu at
// position 1 becomes the menu at position 0, and there is exactly one menu either way.
func TestAReorderedBlockDoesNotPileUpMenus(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		two := with(goodTree(), "blocks/fingers.yaml", `
name: "Fingers"
items:
  - movement: "general_warmup"
  - menu:
      name: "Pick a hang"
      pick: 1
      of: ["half_crimp_hang", "hangboard_ladder"]
`)
		if _, err := s.ImportShipped(ctx, loadGood(t, two, ShippedTree)); err != nil {
			t.Fatal(err)
		}
		if n := count(t, s, `SELECT count(*) FROM content WHERE slug='fingers_1'`); n != 1 {
			t.Fatalf("the menu at position 1 is not named fingers_1")
		}

		swapped := with(two, "blocks/fingers.yaml", `
name: "Fingers"
items:
  - menu:
      name: "Pick a hang"
      pick: 1
      of: ["half_crimp_hang", "hangboard_ladder"]
  - movement: "general_warmup"
`)
		if _, err := s.ImportShipped(ctx, loadGood(t, swapped, ShippedTree)); err != nil {
			t.Fatal(err)
		}
		if n := count(t, s, `SELECT count(*) FROM content WHERE kind='menu'`); n != 1 {
			t.Errorf("%d menu rows after a reorder, want 1", n)
		}
		if n := count(t, s, `SELECT count(*) FROM content WHERE slug='fingers_0' AND kind='menu'`); n != 1 {
			t.Error("the menu did not move to the slug for position 0")
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
color: "#ef4444"
tags: ["fingers"]
items:
  - block: "fingers"
  - block: "warm_up"
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
		if len(got) != 2 || got[0].Slug != "fingers" || got[1].Slug != "warm_up" {
			t.Errorf("the reordered list read back as %+v", got)
		}
	})
}

// A shipped file may not put an absolute weight on a slot. The slot beats a person's own
// saved weight by design, so a weight here would override every account's number with no
// way for them to escape short of copying the whole session.
//
// This is a rule rather than an observation about today's files. It is the one thing that
// keeps movement_pref meaningful.
func TestAShippedFileCannotSetAWeightOnASlot(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		heavy := with(goodTree(), "blocks/warm_up.yaml", `
name: "Warm-up"
items:
  - movement: "general_warmup"
    weight_kg: 20
`)
		_, err := s.ImportShipped(ctx, loadGood(t, heavy, ShippedTree))
		if err == nil {
			t.Fatal("a shipped slot with an absolute weight was accepted")
		}
		if !strings.Contains(err.Error(), "weight_kg") {
			t.Errorf("the error does not name the key: %v", err)
		}

		// The same file is fine in a private tree: it is that person's own prescription.
		seedAccount(t, s, 1, "a@b.c")
		if _, err := s.ImportOwned(ctx, loadGood(t, heavy, "private"), "a@b.c"); err != nil {
			t.Fatalf("a private tree was refused a weight on a slot: %v", err)
		}
	})
}

// Renaming a file moves the row it owns and writes nothing to history. The importer
// matches on the id, so the file may be called anything; history groups on the family, and
// a rename does not change one.
func TestARenameMovesTheRowAndWritesNothingToHistory(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "a@b.c")
		if _, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "a@b.c"); err != nil {
			t.Fatal(err)
		}

		var before Content
		if err := s.read(ctx).Where("slug = ?", "general_warmup").First(&before).Error; err != nil {
			t.Fatal(err)
		}
		seedLogEntry(t, s, 1, "general_warmup", before.ID)

		// The file is renamed, and carries the id it has always had.
		renamed := without(goodTree(), "movements/general_warmup.yaml")
		renamed = with(renamed, "movements/easy_start.yaml", `
id: `+testID("movements/general_warmup.yaml")+`
name: "Easy Start"
style: "open"
tags: ["warmup"]
`)
		renamed = with(renamed, "blocks/warm_up.yaml", `
id: `+testID("blocks/warm_up.yaml")+`
name: "Warm-up"
tags: ["warmup"]
role: "warmup"
items:
  - movement: "easy_start"
`)
		res, err := s.ImportOwned(ctx, loadGood(t, renamed, "private"), "a@b.c")
		if err != nil {
			t.Fatalf("a rename was refused: %v", err)
		}
		if res.Inserted != 0 {
			t.Errorf("a rename created %d new rows, want 0: %s", res.Inserted, res)
		}

		// One row, moved. Not two, and not retired either.
		if n := count(t, s, `SELECT count(*) FROM content WHERE kind='movement' AND slug='general_warmup'`); n != 0 {
			t.Error("the old slug still holds a row")
		}
		var after Content
		if err := s.read(ctx).Where("slug = ?", "easy_start").First(&after).Error; err != nil {
			t.Fatal(err)
		}
		if after.ID != before.ID {
			t.Error("the rename made a new row rather than moving the old one")
		}
		if after.RetiredOn != nil {
			t.Error("the renamed row was retired")
		}

		// History was not touched, and still finds the row through the family.
		if n := count(t, s,
			`SELECT count(*) FROM log_entry WHERE movement_slug='general_warmup'`); n != 1 {
			t.Error("a rename rewrote finished history")
		}
		if n := count(t, s,
			`SELECT count(*) FROM log_entry WHERE movement_family=?`, after.Family); n != 1 {
			t.Error("the entry no longer groups with the row it was run from")
		}
	})
}

// Two movements with one slug -- the app's and a person's own -- are two families, so their
// histories stay apart. Grouping by name merged them, which is the fault the family removes.
func TestTwoMovementsSharingASlugKeepSeparateHistories(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "a@b.c")

		if _, err := s.ImportShipped(ctx, loadGood(t, shippedOnly(), ShippedTree)); err != nil {
			t.Fatal(err)
		}
		var appRow Content
		if err := s.read(ctx).Where("slug = ? AND author_id IS NULL", "bench_press").
			First(&appRow).Error; err != nil {
			t.Fatal(err)
		}

		priv := fstest.MapFS{
			"catalog.yaml": &fstest.MapFile{Data: []byte("format_version: 2\n")},
			"movements/bench_press.yaml": &fstest.MapFile{Data: []byte(`
id: 99999999-8888-4777-8666-555555555555
name: "My Bench Press"
style: "reps_and_sets"
`)},
		}
		if _, err := s.ImportOwned(ctx, loadGood(t, withIDs(priv), "private"), "a@b.c"); err != nil {
			t.Fatal(err)
		}
		var mine Content
		if err := s.read(ctx).Where("slug = ? AND author_id = ?", "bench_press", 1).
			First(&mine).Error; err != nil {
			t.Fatal(err)
		}

		if mine.Family == appRow.Family {
			t.Fatal("two unrelated movements were given one family")
		}

		seedLogEntry(t, s, 1, "bench_press", appRow.ID)
		seedLogEntry(t, s, 1, "bench_press", mine.ID)

		for _, c := range []struct {
			what   string
			family string
		}{{"the app's", appRow.Family}, {"the person's own", mine.Family}} {
			if n := count(t, s,
				`SELECT count(*) FROM log_entry WHERE movement_family=?`, c.family); n != 1 {
				t.Errorf("%d entries under %s family, want 1", n, c.what)
			}
		}
	})
}

// A copy declares the family of the row it came from, and that is what holds one chart
// together across the copy.
func TestACopyInheritsTheFamily(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "a@b.c")

		if _, err := s.ImportShipped(ctx, loadGood(t, shippedOnly(), ShippedTree)); err != nil {
			t.Fatal(err)
		}
		var appRow Content
		if err := s.read(ctx).Where("slug = ? AND author_id IS NULL", "bench_press").
			First(&appRow).Error; err != nil {
			t.Fatal(err)
		}

		priv := withIDs(fstest.MapFS{
			"catalog.yaml": &fstest.MapFile{Data: []byte("format_version: 2\n")},
			"movements/my_bench.yaml": &fstest.MapFile{Data: []byte(`
id: 11111111-2222-4333-8444-555555555555
family: ` + appRow.Family + `
name: "My Bench Press"
style: "reps_and_sets"
`)},
		})
		if _, err := s.ImportOwned(ctx, loadGood(t, priv, "private"), "a@b.c"); err != nil {
			t.Fatal(err)
		}

		var mine Content
		if err := s.read(ctx).Where("slug = ? AND author_id = ?", "my_bench", 1).
			First(&mine).Error; err != nil {
			t.Fatal(err)
		}
		if mine.Family != appRow.Family {
			t.Errorf("the copy has family %q, want the app row's %q", mine.Family, appRow.Family)
		}
		if mine.UUID == appRow.UUID {
			t.Error("the copy kept the original's id")
		}

		seedLogEntry(t, s, 1, "bench_press", appRow.ID)
		seedLogEntry(t, s, 1, "my_bench", mine.ID)
		if n := count(t, s,
			`SELECT count(*) FROM log_entry WHERE movement_family=?`, appRow.Family); n != 2 {
			t.Errorf("%d entries in the shared family, want 2", n)
		}
	})
}

// Copying a file and forgetting to change its id is the most reported failure in every
// comparable project. Both files are named, and neither is renumbered: only a person can
// say which one was meant to keep it.
func TestTwoFilesClaimingOneIDAreRefused(t *testing.T) {
	dup := "11111111-2222-4333-8444-555555555555"
	tree := with(goodTree(), "movements/half_crimp_hang.yaml", `
id: `+dup+`
name: "Half Crimp Hang"
style: "timed_reps"
`)
	tree = with(tree, "movements/half_crimp_hang_heavy.yaml", `
id: `+dup+`
name: "Half Crimp Hang, Heavy"
style: "timed_reps"
`)
	_, err := Load(tree, "private", nil)
	if err == nil {
		t.Fatal("two files claiming one id were accepted")
	}
	for _, want := range []string{"half_crimp_hang.yaml", "half_crimp_hang_heavy.yaml", dup} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name %q: %v", want, err)
		}
	}
}

// seedLogEntry writes one finished run of one movement, freezing the family the way the
// app does at the moment a run is created.
func seedLogEntry(t *testing.T, s *Store, accountID int64, slug string, movementID int64) {
	t.Helper()
	var row Content
	if err := s.read(context.Background()).Where("id = ?", movementID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	logID := fmt.Sprintf("log-%s-%d", slug, movementID)
	mustExec(t, s, `INSERT INTO log (id,account_id,on_date,state,created_at,updated_at)
	                VALUES (?,?,?,'done',?,?)`, logID, accountID, "2026-09-01", now, now)
	mustExec(t, s, `INSERT INTO log_entry
	     (id,log_id,position,movement_id,movement_family,movement_slug,movement_name,
	      created_at,updated_at)
	     VALUES (?,?,0,?,?,?,?,?,?)`,
		fmt.Sprintf("entry-%s-%d", slug, movementID), logID, movementID, row.Family,
		slug, slug, now, now)
}
