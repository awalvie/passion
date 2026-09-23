package db_test

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/db"
	"passion/server/db/dbtest"
)

const (
	e1 = "0199c3a0-0000-7000-8000-000000000001"
	e2 = "0199c3a0-0000-7000-8000-000000000002"
)

func step(exercise, name string) db.Step {
	return db.Step{Exercise: exercise, ExerciseFields: db.ExerciseFields{Name: name, Kind: "open"}}
}

func TestCleanSessionTemplateFields(t *testing.T) {
	f, problems := db.SessionTemplateFields{
		Name:  " Power ",
		Color: ptr("#5D86C9"),
		Notes: ptr(" "),
		Body: db.SessionBody{Sections: []db.Section{{
			Name: " Warm-up ",
			Items: []db.Item{
				{Step: ptr(step(" "+strings.ToUpper(e1)+" ", " Pulse raiser "))},
				{Choice: &db.Choice{Name: "Shoulders", Pick: 1, Options: []db.Step{step(e2, "Y raises")}}},
			},
		}}},
	}.Clean()

	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	if f.Name != "Power" || f.Notes != nil || *f.Color != "#5d86c9" {
		t.Fatalf("name %q, notes %v, color %q", f.Name, f.Notes, *f.Color)
	}
	if f.Tags == nil {
		t.Fatal("tags are nil, want an empty list")
	}
	section := f.Body.Sections[0]
	if section.Name != "Warm-up" {
		t.Fatalf("section name %q", section.Name)
	}
	if s := section.Items[0].Step; s.Exercise != e1 || s.Name != "Pulse raiser" || s.Tags == nil {
		t.Fatalf("step %+v, want it trimmed and its tags an empty list", s)
	}
}

func TestCleanSessionTemplateProblems(t *testing.T) {
	both := db.Item{Step: ptr(step(e1, "Hang")), Choice: &db.Choice{Name: "Pick", Options: []db.Step{step(e1, "Hang")}}}

	_, problems := db.SessionTemplateFields{
		Color: ptr("blue"),
		Body: db.SessionBody{Sections: []db.Section{{
			Items: []db.Item{
				{},
				both,
				{Step: &db.Step{Exercise: "nope", ExerciseFields: db.ExerciseFields{Name: "Hang", Kind: "nope"}}},
				{Choice: &db.Choice{Name: "Shoulders", Pick: 2, Options: []db.Step{step(e1, "Y raises")}}},
				{Choice: &db.Choice{Name: "Shoulders", Pick: -1, Options: []db.Step{step(e1, "Y raises")}}},
				{Choice: &db.Choice{Name: "Shoulders"}},
			},
		}}},
	}.Clean()

	want := []string{
		"color",
		"name",
		"sections[0].items[0]",
		"sections[0].items[1]",
		"sections[0].items[2].step.exercise",
		"sections[0].items[2].step.kind",
		"sections[0].items[3].choice.pick",
		"sections[0].items[4].choice.pick",
		"sections[0].items[5].choice.options",
		"sections[0].name",
	}
	if got := slices.Sorted(maps.Keys(problems)); !slices.Equal(got, want) {
		t.Fatalf("problems at\n%q\nwant\n%q", got, want)
	}
}

// A step's exercise fields sit next to its exercise id in the body, not
// under a key of their own.
func TestSessionBodyJSON(t *testing.T) {
	body, err := json.Marshal(db.Item{Step: ptr(step(e1, "Hang"))})
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["choice"]; ok {
		t.Fatalf("%s holds a choice key", body)
	}
	if got["step"]["exercise"] != e1 || got["step"]["name"] != "Hang" || got["step"]["kind"] != "open" {
		t.Fatalf("got %s", body)
	}
}

// template is a cleaned session with one step and one choice.
func template(t *testing.T, name, stepExercise, optionExercise string) db.SessionTemplateFields {
	t.Helper()
	f, problems := db.SessionTemplateFields{
		Name: name,
		Body: db.SessionBody{Sections: []db.Section{{
			Name: "Main",
			Items: []db.Item{
				{Step: ptr(step(stepExercise, "Max Hangs"))},
				{Choice: &db.Choice{Name: "Shoulders", Pick: 1, Options: []db.Step{step(optionExercise, "Y raises")}}},
			},
		}}},
	}.Clean()
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	return f
}

// insertSessionTemplate writes a row directly, as the catalog loader will. An
// empty owner means shipped.
func insertSessionTemplate(t *testing.T, pool *pgxpool.Pool, owner, name string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO session_template (owner, name, body)
		VALUES (nullif($1, '')::uuid, $2, '{"sections": []}')
		RETURNING id`, owner, name).Scan(&id)
	if err != nil {
		t.Fatalf("insert %s: %v", name, err)
	}
	return id
}

func TestCreateSessionTemplate(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	shipped := mustInsertExercise(t, pool, "", "max_hangs", "Max Hangs")
	own := mustInsertExercise(t, pool, ada, "", "Y raises")
	bobs := mustInsertExercise(t, pool, bob, "", "Bob's Squat")

	f := template(t, "Power", shipped, own)
	created, err := db.CreateSessionTemplate(ctx, pool, ada, f)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Owner == nil || *created.Owner != ada || created.Slug != nil {
		t.Fatalf("owner %v, slug %v, want owned by %s with no slug", created.Owner, created.Slug, ada)
	}

	got, err := db.GetSessionTemplate(ctx, pool, ada, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !reflect.DeepEqual(got.Body, f.Body) {
		t.Fatalf("body came back as\n%+v\nwant\n%+v", got.Body, f.Body)
	}

	t.Run("a retired exercise still counts", func(t *testing.T) {
		if err := db.RetireExercise(ctx, pool, ada, own); err != nil {
			t.Fatalf("retire: %v", err)
		}
		if _, err := db.CreateSessionTemplate(ctx, pool, ada, f); err != nil {
			t.Fatalf("create: %v", err)
		}
	})

	t.Run("an exercise outside the library is refused", func(t *testing.T) {
		missing := "0199c3a0-0000-7000-8000-00000000dead"
		_, err := db.CreateSessionTemplate(ctx, pool, ada, template(t, "Power", bobs, missing))

		var unknown *db.UnknownExercisesError
		if !errors.As(err, &unknown) {
			t.Fatalf("got %v, want UnknownExercisesError", err)
		}
		want := []string{
			"sections[0].items[0].step.exercise",
			"sections[0].items[1].choice.options[0].exercise",
		}
		if got := slices.Sorted(maps.Keys(unknown.Problems)); !slices.Equal(got, want) {
			t.Fatalf("problems at %q, want %q", got, want)
		}
	})
}

func TestListSessionTemplates(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	insertSessionTemplate(t, pool, "", "Power")
	insertSessionTemplate(t, pool, ada, "endurance")
	insertSessionTemplate(t, pool, bob, "Bob's Day")
	retired := insertSessionTemplate(t, pool, ada, "Old Day")
	if err := db.RetireSessionTemplate(ctx, pool, ada, retired); err != nil {
		t.Fatalf("retire: %v", err)
	}

	list, err := db.ListSessionTemplates(ctx, pool, ada)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	var names []string
	for _, s := range list {
		names = append(names, s.Name)
	}
	if want := []string{"endurance", "Power"}; !slices.Equal(names, want) {
		t.Fatalf("listed %q, want %q", names, want)
	}
}

func TestGetSessionTemplate(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	shipped := insertSessionTemplate(t, pool, "", "Power")
	own := insertSessionTemplate(t, pool, ada, "Endurance")
	bobs := insertSessionTemplate(t, pool, bob, "Bob's Day")

	for name, id := range map[string]string{"shipped": shipped, "own": own} {
		t.Run(name, func(t *testing.T) {
			got, err := db.GetSessionTemplate(ctx, pool, ada, id)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if got.Body.Sections == nil || got.Tags == nil {
				t.Fatalf("sections %#v, tags %#v, want empty lists", got.Body.Sections, got.Tags)
			}
		})
	}

	t.Run("retired, still found", func(t *testing.T) {
		if err := db.RetireSessionTemplate(ctx, pool, ada, own); err != nil {
			t.Fatalf("retire: %v", err)
		}
		got, err := db.GetSessionTemplate(ctx, pool, ada, own)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.RetiredAt == nil {
			t.Fatal("retired_at is not set")
		}
	})

	for name, id := range map[string]string{"someone else's": bobs, "not a uuid": "nope"} {
		t.Run(name, func(t *testing.T) {
			if _, err := db.GetSessionTemplate(ctx, pool, ada, id); !errors.Is(err, db.ErrNoSessionTemplate) {
				t.Fatalf("got %v, want ErrNoSessionTemplate", err)
			}
		})
	}
}

func TestUpdateSessionTemplate(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	own := insertSessionTemplate(t, pool, ada, "Power")
	shipped := insertSessionTemplate(t, pool, "", "Endurance")
	bobs := insertSessionTemplate(t, pool, bob, "Bob's Day")
	hang := mustInsertExercise(t, pool, "", "max_hangs", "Max Hangs")

	f := template(t, "Power Day", hang, hang)
	got, err := db.UpdateSessionTemplate(ctx, pool, ada, own, f)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.Name != "Power Day" || len(got.Body.Sections) != 1 {
		t.Fatalf("name %q, %d sections, want Power Day and 1", got.Name, len(got.Body.Sections))
	}
	if !got.UpdatedAt.After(got.CreatedAt) {
		t.Fatal("updated_at did not move")
	}

	t.Run("an exercise outside the library is refused", func(t *testing.T) {
		bobsExercise := mustInsertExercise(t, pool, bob, "", "Bob's Squat")
		_, err := db.UpdateSessionTemplate(ctx, pool, ada, own, template(t, "Power Day", hang, bobsExercise))
		var unknown *db.UnknownExercisesError
		if !errors.As(err, &unknown) {
			t.Fatalf("got %v, want UnknownExercisesError", err)
		}
	})

	for name, id := range map[string]string{"shipped": shipped, "someone else's": bobs, "not a uuid": "nope"} {
		t.Run(name, func(t *testing.T) {
			if _, err := db.UpdateSessionTemplate(ctx, pool, ada, id, f); !errors.Is(err, db.ErrNoSessionTemplate) {
				t.Fatalf("got %v, want ErrNoSessionTemplate", err)
			}
		})
	}
}

func TestRetireSessionTemplate(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	ada := newAccount(t, pool, "ada@example.com")
	bob := newAccount(t, pool, "bob@example.com")

	own := insertSessionTemplate(t, pool, ada, "Power")
	shipped := insertSessionTemplate(t, pool, "", "Endurance")
	bobs := insertSessionTemplate(t, pool, bob, "Bob's Day")

	if err := db.RetireSessionTemplate(ctx, pool, ada, own); err != nil {
		t.Fatalf("retire: %v", err)
	}
	first, err := db.GetSessionTemplate(ctx, pool, ada, own)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if err := db.RetireSessionTemplate(ctx, pool, ada, own); err != nil {
		t.Fatalf("retire again: %v", err)
	}
	second, err := db.GetSessionTemplate(ctx, pool, ada, own)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !second.RetiredAt.Equal(*first.RetiredAt) {
		t.Fatalf("retiring twice moved the date from %v to %v", first.RetiredAt, second.RetiredAt)
	}

	for name, id := range map[string]string{"shipped": shipped, "someone else's": bobs, "not a uuid": "nope"} {
		t.Run(name, func(t *testing.T) {
			if err := db.RetireSessionTemplate(ctx, pool, ada, id); !errors.Is(err, db.ErrNoSessionTemplate) {
				t.Fatalf("got %v, want ErrNoSessionTemplate", err)
			}
		})
	}
}
