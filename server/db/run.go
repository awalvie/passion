package db

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

	// The scheduled session it was started from, if any.
	Scheduled *string `db:"scheduled"`

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
	Timezone       string     `db:"timezone"`
	FinishedAt     *time.Time `db:"finished_at"`
	ElapsedSeconds *int       `db:"elapsed_seconds"`
	Place          *string    `db:"place"`
	// Anything written in the journal, notes or place, as History shows it.
	HasJournal bool `db:"has_journal"`
}

// ErrNoRun covers a run that does not exist and one this person cannot see.
var ErrNoRun = errors.New("no such run")

// RunStart starts a run. With a Template it is a planned run, and a
// Scheduled session names its template. With neither, Name names an open run
// that starts empty. LocalDate is for a write-up of a day already gone.
type RunStart struct {
	Template  *string
	Scheduled *string
	Name      string
	StartedAt time.Time
	LocalDate *time.Time
}

func (s RunStart) Clean() (RunStart, map[string]string) {
	problems := map[string]string{}
	checkStartedAt(s.StartedAt, problems)
	if s.Scheduled != nil {
		id, ok := cleanID(*s.Scheduled)
		s.Scheduled = &id
		if !ok {
			problems["scheduled"] = "must be a scheduled session id"
		}
		if s.Template != nil {
			problems["template"] = "comes from the scheduled session, so leave it out"
		}
		return s, problems
	}
	if s.Template != nil {
		id, ok := cleanID(*s.Template)
		s.Template = &id
		if !ok {
			problems["template"] = "must be a session template id"
		}
		return s, problems
	}
	s.Name = cleanName(s.Name, "name", problems)
	return s, problems
}

// StartRun copies the template now, so that editing it later never reaches
// the run. It takes a RunStart that went through Clean.
func StartRun(ctx context.Context, pool *pgxpool.Pool, owner string, s RunStart) (Run, error) {
	if s.Scheduled != nil {
		var template string
		err := pool.QueryRow(ctx, `SELECT template::text FROM scheduled_session WHERE owner = $1 AND id = $2`,
			owner, *s.Scheduled).Scan(&template)
		if errors.Is(err, pgx.ErrNoRows) {
			return Run{}, ErrNoScheduledSession
		}
		if err != nil {
			return Run{}, fmt.Errorf("select scheduled session: %w", err)
		}
		s.Template = &template
	}

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
		INSERT INTO run (owner, template, scheduled, name, plan, body, started_at, timezone, local_date)
		SELECT a.id, @template::uuid, @scheduled::uuid, @name::text, @plan::jsonb, @body::jsonb,
			@started_at::timestamptz, a.timezone,
			coalesce(@local_date::date, (@started_at::timestamptz AT TIME ZONE a.timezone)::date)
		FROM account a
		WHERE a.id = @owner
		RETURNING *`, pgx.NamedArgs{
		"owner":      owner,
		"template":   s.Template,
		"scheduled":  s.Scheduled,
		"name":       name,
		"plan":       plan,
		"body":       body,
		"started_at": s.StartedAt,
		"local_date": s.LocalDate,
	})
	if err != nil {
		return Run{}, fmt.Errorf("insert run: %w", err)
	}
	run, err := oneRow[Run](rows, ErrNoRun, "run")
	// The scheduled session can be taken out between the read and the insert.
	if hasCode(err, foreignKeyViolation) {
		return Run{}, ErrNoScheduledSession
	}
	if err != nil {
		return Run{}, err
	}
	run.Sets, run.Climbs = []Set{}, []Climb{}
	return run, nil
}

// NewID is a random version 4 uuid.
func NewID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ListRuns is every run of the person's, finished or not, newest day first.
func ListRuns(ctx context.Context, pool *pgxpool.Pool, owner string) ([]RunSummary, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, template, name, local_date, started_at, timezone, finished_at, elapsed_seconds, place,
			num_nonnulls(sleep, energy, rpe, focus, setting, went_well, next_focus, notes, place) > 0 AS has_journal
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
	checkStartedAt(f.StartedAt, problems)
	checkCount(f.ElapsedSeconds, "elapsed_seconds", problems)
	f.Place = optional(f.Place)
	f.Notes = optional(f.Notes)
	f.Journal = f.Journal.clean(problems)
	return f, problems
}

// checkStartedAt keeps a start inside years JSON can write. One outside them
// would leave the run list unreadable.
func checkStartedAt(at time.Time, problems map[string]string) {
	if y := at.UTC().Year(); y < 1900 || y > 2200 {
		problems["started_at"] = "must be between 1900 and 2200"
	}
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
	run, err := oneRow[Run](rows, ErrNoRun, "run")
	if err != nil {
		return Run{}, err
	}
	if err := prune(ctx, tx, "run_set", "", run.ID, run.Body); err != nil {
		return Run{}, err
	}
	if err := prune(ctx, tx, "climb", "climbing", run.ID, run.Body); err != nil {
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
func DeleteRun(ctx context.Context, pool *pgxpool.Pool, owner, id string) error {
	return execOne(ctx, pool, ErrNoRun, "delete run", `DELETE FROM run WHERE owner = $1 AND id = $2`, owner, id)
}

// withLog reads one run from rows, then what was logged in it.
func withLog(ctx context.Context, q querier, rows pgx.Rows) (Run, error) {
	run, err := oneRow[Run](rows, ErrNoRun, "run")
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
