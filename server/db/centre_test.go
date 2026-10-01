package db_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"passion/server/db"
	"passion/server/db/dbtest"
)

func centreFields(t *testing.T, name string, sessions ...string) db.CentreFields {
	t.Helper()
	f, problems := db.CentreFields{Name: name, Sessions: sessions}.Clean()
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	return f
}

func TestPutCentre(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	hang := insertSessionTemplate(t, pool, ada, "Hang")
	boulder := insertSessionTemplate(t, pool, "", "Boulder")

	// A retry makes one centre.
	for range 2 {
		centre, err := db.PutCentre(ctx, pool, ada, y1, centreFields(t, "The Arch", boulder, hang))
		if err != nil {
			t.Fatal(err)
		}
		if centre.ID != y1 || centre.Name != "The Arch" || !slices.Equal(centre.Sessions, []string{boulder, hang}) {
			t.Fatalf("centre %+v", centre)
		}
	}
	if _, err := db.PutCentre(ctx, pool, ada, y2, centreFields(t, "Arena")); err != nil {
		t.Fatal(err)
	}

	list, err := db.ListCentres(ctx, pool, ada)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "Arena" || len(list[0].Sessions) != 0 || list[1].ID != y1 {
		t.Fatalf("list %+v, want Arena with no sessions, then The Arch", list)
	}
}

func TestPutCentreRefuses(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	bobs := insertSessionTemplate(t, pool, bob, "Bob's")
	if _, err := db.PutCentre(ctx, pool, bob, y1, centreFields(t, "Bob's wall")); err != nil {
		t.Fatal(err)
	}

	var unknown *db.UnknownTemplatesError
	for name, c := range map[string]struct {
		id   string
		f    db.CentreFields
		want func(error) bool
	}{
		"someone else's template":  {y2, centreFields(t, "Mine", bobs), func(err error) bool { return errors.As(err, &unknown) }},
		"someone else's centre id": {y1, centreFields(t, "Mine"), func(err error) bool { return errors.Is(err, db.ErrNoCentre) }},
		"an id that is not uuid":   {"nope", centreFields(t, "Mine"), func(err error) bool { return errors.Is(err, db.ErrNoCentre) }},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := db.PutCentre(ctx, pool, ada, c.id, c.f); !c.want(err) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if list, err := db.ListCentres(ctx, pool, bob); err != nil || len(list) != 1 || list[0].Name != "Bob's wall" {
		t.Fatalf("bob's centres %+v, %v, want them untouched", list, err)
	}
}

func TestDeleteCentre(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	if _, err := db.PutCentre(ctx, pool, ada, y1, centreFields(t, "The Arch")); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteCentre(ctx, pool, ada, y1); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteCentre(ctx, pool, ada, y1); !errors.Is(err, db.ErrNoCentre) {
		t.Fatalf("deleted twice: %v", err)
	}
}

func TestCleanCentreFields(t *testing.T) {
	f, problems := db.CentreFields{Name: "  The Arch ", Sessions: []string{y1, " " + y1 + " ", "nope"}}.Clean()
	if f.Name != "The Arch" || !slices.Equal(f.Sessions, []string{y1}) {
		t.Fatalf("fields %+v, want a trimmed name and one session", f)
	}
	if problems["sessions[2]"] == "" || len(problems) != 1 {
		t.Fatalf("problems %v, want only sessions[2]", problems)
	}
	if _, problems := (db.CentreFields{Name: " "}).Clean(); problems["name"] == "" {
		t.Fatalf("problems %v, want the name required", problems)
	}
}
