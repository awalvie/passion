package db_test

import (
	"context"
	"slices"
	"testing"

	"passion/server/db"
	"passion/server/db/dbtest"
)

const powerFile = "22222222-2222-4222-8222-222222222222"

func sessionFile(t *testing.T, slug, hash, exercise string) db.FileSessionTemplate {
	t.Helper()
	f, problems := db.SessionTemplateFields{
		Name: slug,
		Body: db.SessionBody{Sections: []db.Section{{
			Name:  "Main",
			Items: []db.Item{{Step: ptr(step(exercise, "Max Hangs"))}},
		}}},
	}.Clean()
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	return db.FileSessionTemplate{FileID: powerFile, Slug: slug, Hash: hash, Fields: f}
}

// The rules are the exercise loader's, which its own tests cover. This checks
// that a session template goes through them whole.
func TestLoadSessionTemplates(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	load(t, pool, nil, file(hangFile, "max_hangs", "h1"))
	ids, err := db.ExerciseIDsByFileID(ctx, pool, nil)
	if err != nil {
		t.Fatalf("ids: %v", err)
	}
	hang := ids[hangFile]

	power := sessionFile(t, "power", "s1", hang)
	result, err := db.LoadSessionTemplates(ctx, pool, nil, []db.FileSessionTemplate{power})
	if err != nil || result.Written != 1 {
		t.Fatalf("first load wrote %d, err %v", result.Written, err)
	}

	list, err := db.ListSessionTemplates(ctx, pool, newAccount(t, pool, "ada@example.com"))
	if err != nil || len(list) != 1 {
		t.Fatalf("listed %d, err %v", len(list), err)
	}
	if got := list[0].Body.Sections[0].Items[0].Step.Exercise; got != hang {
		t.Fatalf("step exercise %s, want %s", got, hang)
	}

	if result, _ := db.LoadSessionTemplates(ctx, pool, nil, []db.FileSessionTemplate{power}); result.Written != 0 {
		t.Fatalf("a load with nothing changed wrote %d", result.Written)
	}
	if result, _ := db.LoadSessionTemplates(ctx, pool, nil, nil); result.Retired != 1 {
		t.Fatalf("a load with the file gone retired %d, want 1", result.Retired)
	}
}

func TestExerciseIDsByFileID(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")

	load(t, pool, nil, file(hangFile, "max_hangs", "h1"))
	load(t, pool, &ada, file(hangFile, "max_hangs", "h1"), file(pullFile, "pull", "p1"))
	load(t, pool, &ada, file(hangFile, "max_hangs", "h1"))

	shipped, err := db.ExerciseIDsByFileID(ctx, pool, nil)
	if err != nil {
		t.Fatalf("shipped ids: %v", err)
	}
	own, err := db.ExerciseIDsByFileID(ctx, pool, &ada)
	if err != nil {
		t.Fatalf("own ids: %v", err)
	}

	if len(shipped) != 1 || len(own) != 2 {
		t.Fatalf("%d shipped and %d own, want 1 and 2 with the retired one kept", len(shipped), len(own))
	}
	if shipped[hangFile] == own[hangFile] {
		t.Fatal("one file id gave the shipped row and ada's copy the same row")
	}
}

func TestOwnersWithLoadedRows(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")
	newAccount(t, pool, "cy@example.com")

	load(t, pool, nil, file(hangFile, "max_hangs", "h1"))
	ids, err := db.ExerciseIDsByFileID(ctx, pool, nil)
	if err != nil {
		t.Fatalf("ids: %v", err)
	}

	load(t, pool, &ada, file(pullFile, "pull", "p1"))
	if _, err := db.LoadSessionTemplates(ctx, pool, &bob, []db.FileSessionTemplate{sessionFile(t, "power", "s1", ids[hangFile])}); err != nil {
		t.Fatalf("load bob's session: %v", err)
	}

	owners, err := db.OwnersWithLoadedRows(ctx, pool)
	if err != nil {
		t.Fatalf("owners: %v", err)
	}
	slices.Sort(owners)
	want := []string{ada, bob}
	slices.Sort(want)
	if !slices.Equal(owners, want) {
		t.Fatalf("owners %v, want ada for an exercise and bob for a session", owners)
	}
}
