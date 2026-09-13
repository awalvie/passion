package store

import (
	"context"
	"testing"
)

// twoTrees gives one account the private tree and leaves the app's tree unowned, which is
// the arrangement every visibility rule below is about.
func twoTrees(t *testing.T, s *Store) (mine int64, other int64) {
	t.Helper()
	ctx := context.Background()
	seedAccount(t, s, 1, "mine@b.c")
	seedAccount(t, s, 2, "other@b.c")
	if _, err := s.ImportShipped(ctx, loadGood(t, shippedOnly(), ShippedTree)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "mine@b.c"); err != nil {
		t.Fatal(err)
	}
	return 1, 2
}

func TestImportingAPrivateTreeDoesNotChangeTheAppsCount(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "mine@b.c")
		if _, err := s.ImportShipped(ctx, loadGood(t, shippedOnly(), ShippedTree)); err != nil {
			t.Fatal(err)
		}
		before := countShipped(t, s)

		if _, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "mine@b.c"); err != nil {
			t.Fatal(err)
		}
		if after := countShipped(t, s); after != before {
			t.Errorf("the app's row count moved from %d to %d when a private tree imported",
				before, after)
		}
	})
}

func TestASecondAccountSeesTheAppsTreeAndNoneOfYours(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		mine, other := twoTrees(t, s)

		yours, err := s.ListContent(ctx, mine, KindMovement)
		if err != nil {
			t.Fatal(err)
		}
		theirs, err := s.ListContent(ctx, other, KindMovement)
		if err != nil {
			t.Fatal(err)
		}
		if len(theirs) >= len(yours) {
			t.Fatalf("the other account sees %d movements and you see %d", len(theirs), len(yours))
		}
		for _, c := range theirs {
			if c.AuthorID != nil {
				t.Errorf("the other account can read %q, which belongs to account %d",
					c.Slug, *c.AuthorID)
			}
		}
	})
}

func TestTheAppsRowsAreNotMineAndThePrivateTreesAre(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		mine, _ := twoTrees(t, s)

		rows, err := s.ListContent(ctx, mine, KindMovement)
		if err != nil {
			t.Fatal(err)
		}
		var app, own int
		for _, c := range rows {
			if c.Mine() {
				own++
			} else {
				app++
			}
		}
		if app == 0 || own == 0 {
			t.Fatalf("want both kinds in the library, got %d the app's and %d mine", app, own)
		}
	})
}

func TestATreeReadHoldsTheSlotsNumbersNotTheMovements(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		mine, _ := twoTrees(t, s)

		sessions, err := s.ListContent(ctx, mine, KindSession)
		if err != nil {
			t.Fatal(err)
		}
		if len(sessions) == 0 {
			t.Fatal("the private tree imported no sessions")
		}
		got, err := s.ContentTree(ctx, mine, sessions[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Children) == 0 {
			t.Fatal("the session read back with no blocks")
		}
		var movements int
		for _, b := range got.Children {
			if b.Kind != KindBlock {
				t.Errorf("a session holds a %s", b.Kind)
			}
			movements += len(b.Children)
		}
		if movements == 0 {
			t.Error("no block in the session holds anything")
		}
	})
}

func TestDeletingTheOwnerLeavesTheAppsContentAlone(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		mine, other := twoTrees(t, s)
		before := countShipped(t, s)

		if err := s.DeleteAccount(ctx, mine); err != nil {
			t.Fatal(err)
		}
		if after := countShipped(t, s); after != before {
			t.Errorf("deleting an owner took the app's rows from %d to %d", before, after)
		}

		left, err := s.ListContent(ctx, other, KindMovement)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range left {
			if c.AuthorID != nil {
				t.Errorf("%q survived its owner's deletion", c.Slug)
			}
		}
	})
}

func countShipped(t *testing.T, s *Store) int64 {
	t.Helper()
	var n int64
	if err := s.read(context.Background()).Model(&Content{}).
		Where("author_id IS NULL").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}
