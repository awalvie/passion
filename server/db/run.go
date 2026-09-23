package db

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
		v := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case !uuidText.MatchString(v):
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
	c.ID = strings.ToLower(strings.TrimSpace(c.ID))
	if !uuidText.MatchString(c.ID) {
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

// checkCount holds a number to what an int column takes, and to 0 or more.
func checkCount(n *int, key string, problems map[string]string) {
	switch {
	case n == nil:
	case *n < 0:
		problems[key] = "cannot be negative"
	case *n > math.MaxInt32:
		problems[key] = "is too large"
	}
}

// Journal is V1's end-of-run journal. Every field is optional.
type Journal struct {
	Sleep     *int    `db:"sleep"`
	Energy    *int    `db:"energy"`
	RPE       *int    `db:"rpe"`
	Focus     *string `db:"focus"`
	Setting   *string `db:"setting"`
	WentWell  *string `db:"went_well"`
	NextFocus *string `db:"next_focus"`
}

var (
	focuses  = map[string]bool{"strength": true, "endurance": true, "technique": true, "projects": true, "general": true}
	settings = map[string]bool{"indoor": true, "outdoor": true}
)

func (j Journal) clean(problems map[string]string) Journal {
	for _, score := range []struct {
		key string
		n   *int
		max int
	}{{"sleep", j.Sleep, 5}, {"energy", j.Energy, 5}, {"rpe", j.RPE, 10}} {
		if score.n != nil && (*score.n < 1 || *score.n > score.max) {
			problems[score.key] = fmt.Sprintf("must be 1 to %d", score.max)
		}
	}

	j.Focus = optional(j.Focus)
	if j.Focus != nil && !focuses[*j.Focus] {
		problems["focus"] = "must be strength, endurance, technique, projects or general"
	}
	j.Setting = optional(j.Setting)
	if j.Setting != nil && !settings[*j.Setting] {
		problems["setting"] = "must be indoor or outdoor"
	}
	j.WentWell = optional(j.WentWell)
	j.NextFocus = optional(j.NextFocus)
	return j
}

type Run struct {
	ID       string       `db:"id"`
	Owner    string       `db:"owner"`
	Template *string      `db:"template"`
	Name     string       `db:"name"`
	Plan     *SessionBody `db:"plan"`
	Body     RunBody      `db:"body"`

	StartedAt      time.Time  `db:"started_at"`
	Timezone       string     `db:"timezone"`
	LocalDate      time.Time  `db:"local_date"`
	FinishedAt     *time.Time `db:"finished_at"`
	ElapsedSeconds *int       `db:"elapsed_seconds"`
	Place          *string    `db:"place"`
	Notes          *string    `db:"notes"`

	Journal

	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`

	Sets   []Set   `db:"-"`
	Climbs []Climb `db:"-"`
}

// RunSummary is a run as a list shows it.
type RunSummary struct {
	ID             string     `db:"id"`
	Template       *string    `db:"template"`
	Name           string     `db:"name"`
	LocalDate      time.Time  `db:"local_date"`
	StartedAt      time.Time  `db:"started_at"`
	FinishedAt     *time.Time `db:"finished_at"`
	ElapsedSeconds *int       `db:"elapsed_seconds"`
	Place          *string    `db:"place"`
}

// ErrNoRun covers a run that does not exist and one this person cannot see.
var ErrNoRun = errors.New("no such run")

// RunStart starts a run. With a Template it is a planned run; without one,
// Name names an open run that starts empty. LocalDate is for a write-up of a
// day already gone.
type RunStart struct {
	Template  *string
	Name      string
	StartedAt time.Time
	LocalDate *time.Time
}

func (s RunStart) Clean() (RunStart, map[string]string) {
	problems := map[string]string{}
	if s.Template != nil {
		id := strings.ToLower(strings.TrimSpace(*s.Template))
		s.Template = &id
		if !uuidText.MatchString(id) {
			problems["template"] = "must be a session template id"
		}
		return s, problems
	}
	s.Name = cleanName(s.Name, "name", problems)
	return s, problems
}

// StartRun copies the template now, so that editing it later never reaches
// the run (rule 5). It takes a RunStart that went through Clean.
func StartRun(ctx context.Context, pool *pgxpool.Pool, owner string, s RunStart) (Run, error) {
	name, plan, body := s.Name, (*SessionBody)(nil), RunBody{Sections: []RunSection{}}
	if s.Template != nil {
		t, err := GetSessionTemplate(ctx, pool, owner, *s.Template)
		if err != nil {
			return Run{}, err
		}
		name, plan, body = t.Name, &t.Body, runBody(t.Body)
	}

	// Postgres works out the local date, from the zone it checked when the
	// account was made.
	rows, err := pool.Query(ctx, `
		INSERT INTO run (owner, template, name, plan, body, started_at, timezone, local_date)
		SELECT a.id, @template::uuid, @name::text, @plan::jsonb, @body::jsonb, @started_at::timestamptz,
			a.timezone, coalesce(@local_date::date, (@started_at::timestamptz AT TIME ZONE a.timezone)::date)
		FROM account a
		WHERE a.id = @owner
		RETURNING *`, pgx.NamedArgs{
		"owner":      owner,
		"template":   s.Template,
		"name":       name,
		"plan":       plan,
		"body":       body,
		"started_at": s.StartedAt,
		"local_date": s.LocalDate,
	})
	if err != nil {
		return Run{}, fmt.Errorf("insert run: %w", err)
	}
	run, err := oneRun(rows)
	if err != nil {
		return Run{}, err
	}
	run.Sets, run.Climbs = []Set{}, []Climb{}
	return run, nil
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
				section.Items = append(section.Items, RunItem{Step: &RunStep{ID: newID(), Step: *item.Step}})
			case item.Choice != nil:
				section.Items = append(section.Items, RunItem{Choice: &RunChoice{ID: newID(), Choice: *item.Choice}})
			}
		}
		b.Sections = append(b.Sections, section)
	}
	return b
}

// newID is a random version 4 uuid.
func newID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ListRuns is every run of the person's, finished or not, newest day first.
func ListRuns(ctx context.Context, pool *pgxpool.Pool, owner string) ([]RunSummary, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, template, name, local_date, started_at, finished_at, elapsed_seconds, place
		FROM run
		WHERE owner = $1
		ORDER BY local_date DESC, started_at DESC, id DESC`, owner)
	if err != nil {
		return nil, fmt.Errorf("select runs: %w", err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[RunSummary])
	if err != nil {
		return nil, fmt.Errorf("read runs: %w", err)
	}
	return list, nil
}

// GetRun reads a run with everything logged in it.
func GetRun(ctx context.Context, pool *pgxpool.Pool, owner, id string) (Run, error) {
	rows, err := pool.Query(ctx, `SELECT * FROM run WHERE owner = $1 AND id = $2`, owner, id)
	if err != nil {
		return Run{}, fmt.Errorf("select run: %w", err)
	}
	return withLog(ctx, pool, rows)
}

// RunFields is what the person changes on a run. The plan, the template and
// the finish are never among them.
type RunFields struct {
	Name           string
	Body           RunBody
	StartedAt      time.Time
	LocalDate      time.Time
	ElapsedSeconds *int
	Place          *string
	Notes          *string
	Journal
}

func (f RunFields) Clean() (RunFields, map[string]string) {
	body, problems := f.Body.Clean()
	f.Body = body
	f.Name = cleanName(f.Name, "name", problems)
	checkCount(f.ElapsedSeconds, "elapsed_seconds", problems)
	f.Place = optional(f.Place)
	f.Notes = optional(f.Notes)
	f.Journal = f.Journal.clean(problems)
	return f, problems
}

// UpdateRun replaces what the person can change on one of their runs, and
// drops what was logged against steps the body no longer keeps. It takes
// fields that went through Clean.
func UpdateRun(ctx context.Context, pool *pgxpool.Pool, owner, id string, f RunFields) (Run, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Run{}, fmt.Errorf("begin update: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		UPDATE run SET
			name = @name, body = @body, started_at = @started_at, local_date = @local_date,
			elapsed_seconds = @elapsed_seconds, place = @place, notes = @notes,
			sleep = @sleep, energy = @energy, rpe = @rpe, focus = @focus, setting = @setting,
			went_well = @went_well, next_focus = @next_focus
		WHERE owner = @owner AND id = @id
		RETURNING *`, pgx.NamedArgs{
		"owner":           owner,
		"id":              id,
		"name":            f.Name,
		"body":            f.Body,
		"started_at":      f.StartedAt,
		"local_date":      f.LocalDate,
		"elapsed_seconds": f.ElapsedSeconds,
		"place":           f.Place,
		"notes":           f.Notes,
		"sleep":           f.Sleep,
		"energy":          f.Energy,
		"rpe":             f.RPE,
		"focus":           f.Focus,
		"setting":         f.Setting,
		"went_well":       f.WentWell,
		"next_focus":      f.NextFocus,
	})
	if err != nil {
		return Run{}, fmt.Errorf("update run: %w", err)
	}
	run, err := oneRun(rows)
	if err != nil {
		return Run{}, err
	}
	if err := pruneSets(ctx, tx, run.ID, run.Body); err != nil {
		return Run{}, err
	}
	if err := pruneClimbs(ctx, tx, run.ID, run.Body); err != nil {
		return Run{}, err
	}
	if run.Sets, err = runSets(ctx, tx, run.ID, nil); err != nil {
		return Run{}, err
	}
	if run.Climbs, err = runClimbs(ctx, tx, run.ID); err != nil {
		return Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, fmt.Errorf("commit update: %w", err)
	}
	return run, nil
}

// FinishRun keeps the first finish time. Every step the run never reached
// counts as skipped, as V1 did when a run finished early. A step with sets or
// climbs was reached, so it counts as done and keeps them.
func FinishRun(ctx context.Context, pool *pgxpool.Pool, owner, id string) (Run, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Run{}, fmt.Errorf("begin finish: %w", err)
	}
	defer tx.Rollback(ctx)

	body, err := lockRunBody(ctx, tx, owner, id)
	if err != nil {
		return Run{}, err
	}
	rows, err := tx.Query(ctx, `SELECT step FROM run_set WHERE run = $1 UNION SELECT step FROM climb WHERE run = $1`, id)
	if err != nil {
		return Run{}, fmt.Errorf("select logged steps: %w", err)
	}
	logged, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return Run{}, fmt.Errorf("read logged steps: %w", err)
	}

	done, skipped := StepDone, StepSkipped
	for _, s := range body.Sections {
		for _, item := range s.Items {
			switch st := item.Step; {
			case st == nil || st.Status != nil:
			case slices.Contains(logged, st.ID):
				st.Status = &done
			default:
				st.Status = &skipped
			}
		}
	}

	rows, err = tx.Query(ctx, `
		UPDATE run SET body = $3, finished_at = coalesce(finished_at, now())
		WHERE owner = $1 AND id = $2
		RETURNING *`, owner, id, body)
	if err != nil {
		return Run{}, fmt.Errorf("finish run: %w", err)
	}
	run, err := withLog(ctx, tx, rows)
	if err != nil {
		return Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, fmt.Errorf("commit finish: %w", err)
	}
	return run, nil
}

// DeleteRun removes one of the person's runs with everything logged in it.
// A record belongs to its owner, who may delete it (rule 6).
func DeleteRun(ctx context.Context, pool *pgxpool.Pool, owner, id string) error {
	tag, err := pool.Exec(ctx, `DELETE FROM run WHERE owner = $1 AND id = $2`, owner, id)
	if isInvalidText(err) {
		return ErrNoRun
	}
	if err != nil {
		return fmt.Errorf("delete run: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoRun
	}
	return nil
}

// withLog reads one run from rows, then what was logged in it.
func withLog(ctx context.Context, q querier, rows pgx.Rows) (Run, error) {
	run, err := oneRun(rows)
	if err != nil {
		return Run{}, err
	}
	if run.Sets, err = runSets(ctx, q, run.ID, nil); err != nil {
		return Run{}, err
	}
	if run.Climbs, err = runClimbs(ctx, q, run.ID); err != nil {
		return Run{}, err
	}
	return run, nil
}

func oneRun(rows pgx.Rows) (Run, error) {
	run, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Run])
	if errors.Is(err, pgx.ErrNoRows) || isInvalidText(err) {
		return Run{}, ErrNoRun
	}
	if err != nil {
		return Run{}, fmt.Errorf("read run: %w", err)
	}
	return run, nil
}
