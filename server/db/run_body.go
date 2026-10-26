package db

import "fmt"

// RunBody is a run's working list: the sections it started with, as the
// person changes them while they train.
type RunBody struct {
	Sections []RunSection `json:"sections"`
}

type RunSection struct {
	Name  string    `json:"name"`
	Notes *string   `json:"notes"`
	Items []RunItem `json:"items"`
}

// RunItem holds one step or one choice, never both.
type RunItem struct {
	Step   *RunStep   `json:"step,omitempty"`
	Choice *RunChoice `json:"choice,omitempty"`
}

// RunChoice is an offer nobody has taken yet. A pick replaces it with steps.
type RunChoice struct {
	ID string `json:"id"`
	Choice
	Skipped bool `json:"skipped"`
}

// RunStep is a step and what happened to it. FromChoice is the offer a picked
// step came from, so that the pick can be changed.
type RunStep struct {
	ID string `json:"id"`
	Step
	Status         *string    `json:"status"`
	RunNotes       *string    `json:"run_notes"`
	ElapsedSeconds *int       `json:"elapsed_seconds"`
	FromChoice     *RunChoice `json:"from_choice,omitempty"`
}

// A step's status is null until the run reaches it.
const (
	StepDone    = "done"
	StepSkipped = "skipped"
)

// Clean trims and checks the body, and names each problem by its path, such
// as sections[0].items[2].step.status. A run does not check its exercises
// against the library: a step typed in mid-run has an id of its own.
func (b RunBody) Clean() (RunBody, map[string]string) {
	problems := map[string]string{}
	seen := map[string]bool{}
	id := func(raw, path string) string {
		v, ok := cleanID(raw)
		switch {
		case !ok:
			problems[path] = "must be a uuid"
		case seen[v]:
			problems[path] = "is used twice"
		}
		seen[v] = true
		return v
	}

	sections := []RunSection{}
	for i, s := range b.Sections {
		path := fmt.Sprintf("sections[%d]", i)
		s.Name = cleanName(s.Name, path+".name", problems)
		s.Notes = optional(s.Notes)

		items := []RunItem{}
		for j, item := range s.Items {
			at := fmt.Sprintf("%s.items[%d]", path, j)
			switch {
			case item.Step != nil && item.Choice != nil:
				problems[at] = "must hold a step or a choice, not both"
			case item.Step != nil:
				step := item.Step.clean(at+".step", problems)
				step.ID = id(step.ID, at+".step.id")
				item.Step = &step
			case item.Choice != nil:
				choice := item.Choice.clean(at+".choice", problems)
				choice.ID = id(choice.ID, at+".choice.id")
				item.Choice = &choice
			default:
				problems[at] = "must hold a step or a choice"
			}
			items = append(items, item)
		}
		s.Items = items
		sections = append(sections, s)
	}
	b.Sections = sections
	return b, problems
}

func (c RunChoice) clean(path string, problems map[string]string) RunChoice {
	var ok bool
	if c.ID, ok = cleanID(c.ID); !ok {
		problems[path+".id"] = "must be a uuid"
	}
	c.Choice = c.Choice.clean(path, problems)
	return c
}

func (s RunStep) clean(path string, problems map[string]string) RunStep {
	s.Step = s.Step.clean(path, problems)
	if s.Status != nil && *s.Status != StepDone && *s.Status != StepSkipped {
		problems[path+".status"] = "must be done or skipped"
	}
	s.RunNotes = optional(s.RunNotes)
	checkCount(s.ElapsedSeconds, path+".elapsed_seconds", problems)
	// The steps of one pick share their offer, so its id is not unique.
	if s.FromChoice != nil {
		choice := s.FromChoice.clean(path+".from_choice", problems)
		s.FromChoice = &choice
	}
	return s
}

// runBody copies a template's body for a new run, with an id on every step
// and every choice.
func runBody(t SessionBody) RunBody {
	b := RunBody{Sections: make([]RunSection, 0, len(t.Sections))}
	for _, s := range t.Sections {
		section := RunSection{Name: s.Name, Notes: s.Notes, Items: make([]RunItem, 0, len(s.Items))}
		for _, item := range s.Items {
			switch {
			case item.Step != nil:
				section.Items = append(section.Items, RunItem{Step: &RunStep{ID: NewID(), Step: *item.Step}})
			case item.Choice != nil:
				section.Items = append(section.Items, RunItem{Choice: &RunChoice{ID: NewID(), Choice: *item.Choice}})
			}
		}
		b.Sections = append(b.Sections, section)
	}
	return b
}
