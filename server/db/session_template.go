package db

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

var (
	hexColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)
	uuidText = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

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
	s.Exercise = strings.ToLower(strings.TrimSpace(s.Exercise))
	if !uuidText.MatchString(s.Exercise) {
		problems[path+".exercise"] = "must be an exercise id"
	}

	fields, fieldProblems := s.ExerciseFields.Clean()
	for key, problem := range fieldProblems {
		problems[path+"."+key] = problem
	}
	s.ExerciseFields = fields
	return s
}

type SessionTemplate struct {
	ID         string  `db:"id"`
	Owner      *string `db:"owner"`
	Slug       *string `db:"slug"`
	FileID     *string `db:"file_id"`
	LoadedHash *string `db:"loaded_hash"`

	Name   string      `db:"name"`
	Notes  *string     `db:"notes"`
	Source *string     `db:"source"`
	Color  *string     `db:"color"`
	Needs  *string     `db:"needs"`
	Tags   []string    `db:"tags"`
	Body   SessionBody `db:"body"`

	RetiredAt *time.Time `db:"retired_at"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}

// ErrNoSessionTemplate covers a session template that does not exist and one
// this person cannot see or change.
var ErrNoSessionTemplate = errors.New("no such session template")

// UnknownExercisesError names, by path, each step whose exercise is neither
// shipped nor the person's own.
type UnknownExercisesError struct {
	Problems map[string]string
}

func (e *UnknownExercisesError) Error() string {
	return fmt.Sprintf("%d steps name an exercise outside the library", len(e.Problems))
}

// checkExercises lets a retired exercise through, because a step that holds
// one keeps working.
func checkExercises(ctx context.Context, pool *pgxpool.Pool, owner string, body SessionBody) error {
	steps := map[string]string{}
	for i, s := range body.Sections {
		for j, item := range s.Items {
			path := fmt.Sprintf("sections[%d].items[%d]", i, j)
			if item.Step != nil {
				steps[path+".step.exercise"] = item.Step.Exercise
			}
			if item.Choice != nil {
				for k, option := range item.Choice.Options {
					steps[fmt.Sprintf("%s.choice.options[%d].exercise", path, k)] = option.Exercise
				}
			}
		}
	}
	if len(steps) == 0 {
		return nil
	}

	ids := slices.Collect(maps.Values(steps))
	rows, err := pool.Query(ctx, `
		SELECT id::text FROM exercise
		WHERE id = ANY($1::uuid[]) AND (owner IS NULL OR owner = $2)`, ids, owner)
	if err != nil {
		return fmt.Errorf("select step exercises: %w", err)
	}
	found, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("read step exercises: %w", err)
	}

	problems := map[string]string{}
	for path, id := range steps {
		if !slices.Contains(found, id) {
			problems[path] = "is not in your library"
		}
	}
	if len(problems) > 0 {
		return &UnknownExercisesError{Problems: problems}
	}
	return nil
}

func (f SessionTemplateFields) args() pgx.NamedArgs {
	return pgx.NamedArgs{
		"name":   f.Name,
		"notes":  f.Notes,
		"source": f.Source,
		"color":  f.Color,
		"needs":  f.Needs,
		"tags":   f.Tags,
		"body":   f.Body,
	}
}

// CreateSessionTemplate takes fields that went through Clean.
func CreateSessionTemplate(ctx context.Context, pool *pgxpool.Pool, owner string, f SessionTemplateFields) (SessionTemplate, error) {
	if err := checkExercises(ctx, pool, owner, f.Body); err != nil {
		return SessionTemplate{}, err
	}

	args := f.args()
	args["owner"] = owner
	rows, err := pool.Query(ctx, `
		INSERT INTO session_template (owner, name, notes, source, color, needs, tags, body)
		VALUES (@owner, @name, @notes, @source, @color, @needs, @tags, @body)
		RETURNING *`, args)
	if err != nil {
		return SessionTemplate{}, fmt.Errorf("insert session template: %w", err)
	}
	return oneSessionTemplate(rows)
}

// ListSessionTemplates is what the app ships and what the person wrote,
// without anything retired.
func ListSessionTemplates(ctx context.Context, pool *pgxpool.Pool, owner string) ([]SessionTemplate, error) {
	rows, err := pool.Query(ctx, `
		SELECT * FROM session_template
		WHERE (owner IS NULL OR owner = $1) AND retired_at IS NULL
		ORDER BY lower(name), id`, owner)
	if err != nil {
		return nil, fmt.Errorf("select session templates: %w", err)
	}

	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[SessionTemplate])
	if err != nil {
		return nil, fmt.Errorf("read session templates: %w", err)
	}
	return list, nil
}

// GetSessionTemplate finds a shipped session template or one of the person's
// own, retired ones included.
func GetSessionTemplate(ctx context.Context, pool *pgxpool.Pool, owner, id string) (SessionTemplate, error) {
	rows, err := pool.Query(ctx, `
		SELECT * FROM session_template
		WHERE (owner IS NULL OR owner = $1) AND id = $2`, owner, id)
	if err != nil {
		return SessionTemplate{}, fmt.Errorf("select session template: %w", err)
	}
	return oneSessionTemplate(rows)
}

// UpdateSessionTemplate replaces one of the person's own session templates. A
// shipped one cannot be changed. It takes fields that went through Clean.
func UpdateSessionTemplate(ctx context.Context, pool *pgxpool.Pool, owner, id string, f SessionTemplateFields) (SessionTemplate, error) {
	if err := checkExercises(ctx, pool, owner, f.Body); err != nil {
		return SessionTemplate{}, err
	}

	args := f.args()
	args["owner"] = owner
	args["id"] = id
	rows, err := pool.Query(ctx, `
		UPDATE session_template SET
			name = @name, notes = @notes, source = @source, color = @color,
			needs = @needs, tags = @tags, body = @body
		WHERE id = @id AND owner = @owner
		RETURNING *`, args)
	if err != nil {
		return SessionTemplate{}, fmt.Errorf("update session template: %w", err)
	}
	return oneSessionTemplate(rows)
}

// RetireSessionTemplate takes one of the person's own session templates out
// of their list. Retiring it twice keeps the first date.
func RetireSessionTemplate(ctx context.Context, pool *pgxpool.Pool, owner, id string) error {
	tag, err := pool.Exec(ctx, `
		UPDATE session_template SET retired_at = coalesce(retired_at, now())
		WHERE owner = $1 AND id = $2`, owner, id)
	if isInvalidText(err) {
		return ErrNoSessionTemplate
	}
	if err != nil {
		return fmt.Errorf("retire session template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoSessionTemplate
	}
	return nil
}

func oneSessionTemplate(rows pgx.Rows) (SessionTemplate, error) {
	template, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[SessionTemplate])
	if errors.Is(err, pgx.ErrNoRows) || isInvalidText(err) {
		return SessionTemplate{}, ErrNoSessionTemplate
	}
	if err != nil {
		return SessionTemplate{}, fmt.Errorf("read session template: %w", err)
	}
	return template, nil
}
