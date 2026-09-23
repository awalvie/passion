package db

import (
	"fmt"
	"regexp"
	"strings"
)

// SessionTemplateFields is what a person sets on a session template of their
// own.
type SessionTemplateFields struct {
	Name   string
	Notes  *string
	Source *string
	Color  *string
	Needs  *string
	Tags   []string
	Body   SessionBody
}

// SessionBody is the jsonb body of a session template: its sections, in order.
type SessionBody struct {
	Sections []Section `json:"sections"`
}

type Section struct {
	Name  string  `json:"name"`
	Notes *string `json:"notes"`
	Items []Item  `json:"items"`
}

// Item holds one step or one choice, never both.
type Item struct {
	Step   *Step   `json:"step,omitempty"`
	Choice *Choice `json:"choice,omitempty"`
}

// Choice asks for at least Pick of its options. A Pick of 0 lets the whole
// choice be skipped.
type Choice struct {
	Name    string  `json:"name"`
	Notes   *string `json:"notes"`
	Pick    int     `json:"pick"`
	Options []Step  `json:"options"`
}

// Step is a library exercise's id and a copy of its fields, taken when the
// step was added. The session shows the copy.
type Step struct {
	Exercise string `json:"exercise"`
	ExerciseFields
}

var hexColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// Clean trims and checks the fields, and names each problem by its path in
// the body, such as sections[0].items[2].choice.pick.
func (f SessionTemplateFields) Clean() (SessionTemplateFields, map[string]string) {
	problems := map[string]string{}

	f.Name = cleanName(f.Name, "name", problems)
	f.Notes = optional(f.Notes)
	f.Source = optional(f.Source)
	f.Needs = optional(f.Needs)
	f.Tags = cleanTags(f.Tags)

	f.Color = optional(f.Color)
	if f.Color != nil {
		lower := strings.ToLower(*f.Color)
		f.Color = &lower
		if !hexColor.MatchString(lower) {
			problems["color"] = "must look like #5d86c9"
		}
	}

	sections := []Section{}
	for i, s := range f.Body.Sections {
		path := fmt.Sprintf("sections[%d]", i)
		s.Name = cleanName(s.Name, path+".name", problems)
		s.Notes = optional(s.Notes)

		items := []Item{}
		for j, item := range s.Items {
			items = append(items, item.clean(fmt.Sprintf("%s.items[%d]", path, j), problems))
		}
		s.Items = items
		sections = append(sections, s)
	}
	f.Body.Sections = sections

	return f, problems
}

func (item Item) clean(path string, problems map[string]string) Item {
	switch {
	case item.Step != nil && item.Choice != nil:
		problems[path] = "must hold a step or a choice, not both"
	case item.Step != nil:
		step := item.Step.clean(path+".step", problems)
		item.Step = &step
	case item.Choice != nil:
		choice := item.Choice.clean(path+".choice", problems)
		item.Choice = &choice
	default:
		problems[path] = "must hold a step or a choice"
	}
	return item
}

func (c Choice) clean(path string, problems map[string]string) Choice {
	c.Name = cleanName(c.Name, path+".name", problems)
	c.Notes = optional(c.Notes)

	options := []Step{}
	for i, option := range c.Options {
		options = append(options, option.clean(fmt.Sprintf("%s.options[%d]", path, i), problems))
	}
	c.Options = options

	if len(options) == 0 {
		problems[path+".options"] = "must hold at least one exercise"
	}
	switch {
	case c.Pick < 0:
		problems[path+".pick"] = "cannot be negative"
	case c.Pick > len(options):
		problems[path+".pick"] = "cannot be more than the options"
	}
	return c
}

func (s Step) clean(path string, problems map[string]string) Step {
	s.Exercise = strings.TrimSpace(s.Exercise)
	if s.Exercise == "" {
		problems[path+".exercise"] = "is required"
	}

	fields, fieldProblems := s.ExerciseFields.Clean()
	for key, problem := range fieldProblems {
		problems[path+"."+key] = problem
	}
	s.ExerciseFields = fields
	return s
}
