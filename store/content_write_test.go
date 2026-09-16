package store

import (
	"context"
	"errors"
	"testing"
)

func str(s string) *string { return &s }
func num(n int) *int       { return &n }

func TestEditingAFileOwnedRowDetachesItFromItsFile(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "mine@b.c")
		if _, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "mine@b.c"); err != nil {
			t.Fatal(err)
		}

		before := firstOfKind(t, s, 1, KindSession)
		if before.SourceTree == nil {
			t.Fatal("the imported row is already detached")
		}

		after, err := s.UpdateContent(ctx, 1, before.ID, ContentUpdate{Name: str("Renamed")})
		if err != nil {
			t.Fatal(err)
		}
		if after.SourceTree != nil {
			t.Error("editing did not detach the row from its file")
		}
		if after.ID != before.ID || after.Slug != before.Slug || after.UUID != before.UUID {
			t.Error("editing moved the row instead of changing it")
		}
		if after.Name != "Renamed" {
			t.Errorf("the name is %q", after.Name)
		}
	})
}

// The next import must name the file rather than putting the old value back.
func TestAnImportSkipsAndReportsADetachedRow(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "mine@b.c")
		if _, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "mine@b.c"); err != nil {
			t.Fatal(err)
		}

		row := firstOfKind(t, s, 1, KindSession)
		if _, err := s.UpdateContent(ctx, 1, row.ID, ContentUpdate{Name: str("Mine now")}); err != nil {
			t.Fatal(err)
		}

		res, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "mine@b.c")
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Skipped) == 0 {
			t.Fatal("the import reported nothing skipped")
		}
		again, err := s.ContentByID(ctx, 1, row.ID)
		if err != nil {
			t.Fatal(err)
		}
		if again.Name != "Mine now" {
			t.Errorf("the import overwrote the edit: name is %q", again.Name)
		}
	})
}

func TestAShippedRowCannotBeEdited(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "mine@b.c")
		if _, err := s.ImportShipped(ctx, loadGood(t, goodTree(), ShippedTree)); err != nil {
			t.Fatal(err)
		}

		row := firstOfKind(t, s, 1, KindSession)
		if _, err := s.UpdateContent(ctx, 1, row.ID, ContentUpdate{Name: str("No")}); !errors.Is(err, ErrShipped) {
			t.Errorf("want ErrShipped, got %v", err)
		}
	})
}

// Another account's row answers not-found, never a refusal: a refusal would confirm it exists.
func TestAnotherAccountsRowIsNotFound(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "mine@b.c")
		seedAccount(t, s, 2, "other@b.c")
		if _, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "mine@b.c"); err != nil {
			t.Fatal(err)
		}

		row := firstOfKind(t, s, 1, KindSession)
		_, err := s.UpdateContent(ctx, 2, row.ID, ContentUpdate{Name: str("No")})
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("want not-found, got %v", err)
		}
	})
}

func TestAnEditLeavesTheFieldsItWasNotGiven(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "mine@b.c")
		if _, err := s.ImportOwned(ctx, loadGood(t, goodTree(), "private"), "mine@b.c"); err != nil {
			t.Fatal(err)
		}

		before := firstOfKind(t, s, 1, KindMovement)
		after, err := s.UpdateContent(ctx, 1, before.ID, ContentUpdate{Notes: str("A note")})
		if err != nil {
			t.Fatal(err)
		}
		if after.Name != before.Name {
			t.Errorf("the name changed from %q to %q", before.Name, after.Name)
		}
		if !eqInt(after.DSets, before.DSets) || !eqInt(after.DReps, before.DReps) {
			t.Error("editing the notes cleared the movement's numbers")
		}
	})
}
