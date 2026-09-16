package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func firstOfKind(t *testing.T, s *Store, accountID int64, kind string) Content {
	t.Helper()
	rows, err := s.ListContent(context.Background(), accountID, kind)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatalf("no %s to work with", kind)
	}
	for _, r := range rows {
		if !r.Mine() {
			return r
		}
	}
	return rows[0]
}

func countKind(t *testing.T, s *Store, kind string) int64 {
	t.Helper()
	var n int64
	if err := s.read(context.Background()).Model(&Content{}).
		Where("kind = ?", kind).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestACopyKeepsItsNameAndMakesNoNewMovements(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "mine@b.c")
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}

		src := firstOfKind(t, s, 1, KindSession)
		movementsBefore := countKind(t, s, KindMovement)

		copied, err := s.CopyToMine(ctx, 1, src.ID)
		if err != nil {
			t.Fatal(err)
		}

		if copied.Name != src.Name || copied.Slug != src.Slug {
			t.Errorf("the copy is %q/%q, want %q/%q",
				copied.Name, copied.Slug, src.Name, src.Slug)
		}
		if copied.AuthorID == nil || *copied.AuthorID != 1 {
			t.Error("the copy does not belong to the account that asked for it")
		}
		if copied.UUID == src.UUID {
			t.Error("the copy kept the original's uuid")
		}
		if copied.Family != src.Family {
			t.Errorf("the copy's family is %q, want the original's %q", copied.Family, src.Family)
		}
		if after := countKind(t, s, KindMovement); after != movementsBefore {
			t.Errorf("copying a session made %d movement rows", after-movementsBefore)
		}
	})
}

// The original must come back byte for byte. A copy that touched it would change what
// everyone else sees.
func TestACopyLeavesTheOriginalAlone(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "mine@b.c")
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}

		src := firstOfKind(t, s, 1, KindSession)
		before, err := s.ContentTree(ctx, 1, src.ID)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := s.CopyToMine(ctx, 1, src.ID); err != nil {
			t.Fatal(err)
		}

		after, err := s.ContentTree(ctx, 1, src.ID)
		if err != nil {
			t.Fatal(err)
		}
		if after.UUID != before.UUID || after.Name != before.Name ||
			len(after.Children) != len(before.Children) {
			t.Error("copying changed the row it copied")
		}
	})
}

func TestACopiedSessionHoldsItsOwnBlocks(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "mine@b.c")
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}

		src := firstOfKind(t, s, 1, KindSession)
		copied, err := s.CopyToMine(ctx, 1, src.ID)
		if err != nil {
			t.Fatal(err)
		}

		was, err := s.ContentTree(ctx, 1, src.ID)
		if err != nil {
			t.Fatal(err)
		}
		now, err := s.ContentTree(ctx, 1, copied.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(now.Children) != len(was.Children) {
			t.Fatalf("the copy holds %d blocks, the original %d",
				len(now.Children), len(was.Children))
		}
		for i := range now.Children {
			if now.Children[i].ID == was.Children[i].ID {
				t.Errorf("block %q is shared with the original rather than copied",
					now.Children[i].Slug)
			}
			if !now.Children[i].Mine() {
				t.Errorf("copied block %q does not belong to the account",
					now.Children[i].Slug)
			}
			// The movements under it are pointed at, not copied.
			for j := range now.Children[i].Children {
				mine := now.Children[i].Children[j]
				theirs := was.Children[i].Children[j]
				if mine.Kind == KindMovement && mine.ID != theirs.ID {
					t.Errorf("movement %q was copied instead of referenced", mine.Slug)
				}
			}
		}
	})
}

func TestAMovementCannotBeCopied(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "mine@b.c")
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}

		m := firstOfKind(t, s, 1, KindMovement)
		if _, err := s.CopyToMine(ctx, 1, m.ID); !errors.Is(err, ErrNotCopyable) {
			t.Errorf("want ErrNotCopyable, got %v", err)
		}
	})
}

func TestCopyingTwiceIsRefusedByName(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "mine@b.c")
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}

		src := firstOfKind(t, s, 1, KindSession)
		if _, err := s.CopyToMine(ctx, 1, src.ID); err != nil {
			t.Fatal(err)
		}
		_, err := s.CopyToMine(ctx, 1, src.ID)
		if err == nil {
			t.Fatal("a second copy was accepted")
		}
		if !strings.Contains(err.Error(), src.Slug) {
			t.Errorf("the error does not name what clashed: %v", err)
		}
	})
}
