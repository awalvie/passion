package db_test

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"passion/server/db"
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
				{Step: ptr(step(" e1 ", " Pulse raiser "))},
				{Choice: &db.Choice{Name: "Shoulders", Pick: 1, Options: []db.Step{step("e2", "Y raises")}}},
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
	if s := section.Items[0].Step; s.Exercise != "e1" || s.Name != "Pulse raiser" || s.Tags == nil {
		t.Fatalf("step %+v, want it trimmed and its tags an empty list", s)
	}
}

func TestCleanSessionTemplateProblems(t *testing.T) {
	both := db.Item{Step: ptr(step("e1", "Hang")), Choice: &db.Choice{Name: "Pick", Options: []db.Step{step("e1", "Hang")}}}

	_, problems := db.SessionTemplateFields{
		Color: ptr("blue"),
		Body: db.SessionBody{Sections: []db.Section{{
			Items: []db.Item{
				{},
				both,
				{Step: &db.Step{ExerciseFields: db.ExerciseFields{Name: "Hang", Kind: "nope"}}},
				{Choice: &db.Choice{Name: "Shoulders", Pick: 2, Options: []db.Step{step("e1", "Y raises")}}},
				{Choice: &db.Choice{Name: "Shoulders", Pick: -1, Options: []db.Step{step("e1", "Y raises")}}},
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
	body, err := json.Marshal(db.Item{Step: ptr(step("e1", "Hang"))})
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
	if got["step"]["exercise"] != "e1" || got["step"]["name"] != "Hang" || got["step"]["kind"] != "open" {
		t.Fatalf("got %s", body)
	}
}
