package db

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/grades"
)

// Climb is one climb logged under a climbing step.
type Climb struct {
	ID          string    `db:"id"`
	Step        string    `db:"step"`
	Exercise    string    `db:"exercise"`
	Position    int       `db:"position"`
	Discipline  string    `db:"discipline"`
	Setting     string    `db:"setting"`
	Board       *string   `db:"board"`
	RopeStyle   *string   `db:"rope_style"`
	Grade       *string   `db:"grade"`
	GradeSystem *string   `db:"grade_system"`
	GradeRank   *int      `db:"grade_rank"`
	Outcome     *string   `db:"outcome"`
	Attempts    *int      `db:"attempts"`
	Seconds     *int      `db:"seconds"`
	Stars       *int      `db:"stars"`
	Focus       *string   `db:"focus"`
	Notes       *string   `db:"notes"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// Sent says whether the climb counts as a send. It follows from how it was
// climbed, and is never asked (rules 38 and 39).
func (c Climb) Sent() bool {
	return c.GradeSystem != nil && c.Outcome != nil && sends[*c.Outcome]
}

var (
	disciplines = map[string]bool{"boulder": true, "sport": true, "trad": true}
	boards      = map[string]bool{"kilter": true, "moon": true, "tension": true, "spray": true, "custom": true}
	ropeStyles  = map[string]bool{"lead": true, "top_rope": true, "auto_belay": true, "follow": true}
	outcomes    = map[string]bool{"onsight": true, "flash": true, "redpoint": true, "hangdog": true, "working": true}
	sends       = map[string]bool{"onsight": true, "flash": true, "redpoint": true}
)

// ClimbFields is what the person logs for one climb.
type ClimbFields struct {
	Step        string
	Position    int
	Discipline  string
	Setting     string
	Board       *string
	RopeStyle   *string
	Grade       *string
	GradeSystem *string
	Outcome     *string
	Attempts    *int
	Seconds     *int
	Stars       *int
	Focus       *string
	Notes       *string

	rank *int
}

// Clean trims and checks the fields, and works out the grade's rank.
func (f ClimbFields) Clean() (ClimbFields, map[string]string) {
	problems := map[string]string{}

	f.Step = strings.ToLower(strings.TrimSpace(f.Step))
	if !uuidText.MatchString(f.Step) {
		problems["step"] = "must be a step id"
	}
	checkCount(&f.Position, "position", problems)
	if !disciplines[f.Discipline] {
		problems["discipline"] = "must be boulder, sport or trad"
	}
	if !settings[f.Setting] {
		problems["setting"] = "must be indoor or outdoor"
	}

	f.Board = optional(f.Board)
	switch {
	case f.Board == nil:
	case f.Discipline != "boulder":
		problems["board"] = "is only for a boulder"
	case !boards[*f.Board]:
		problems["board"] = "must be kilter, moon, tension, spray or custom"
	}
	f.RopeStyle = optional(f.RopeStyle)
	switch {
	case f.RopeStyle == nil:
	case f.Discipline == "boulder":
		problems["rope_style"] = "is only for a route"
	case !ropeStyles[*f.RopeStyle]:
		problems["rope_style"] = "must be lead, top_rope, auto_belay or follow"
	}

	f.Grade = optional(f.Grade)
	f.GradeSystem = optional(f.GradeSystem)
	f.rank = nil
	switch {
	case f.GradeSystem == nil:
		if f.Grade != nil && !slices.Contains(grades.Ungraded, *f.Grade) {
			problems["grade_system"] = "is required with a grade"
		}
	case f.Grade == nil:
		problems["grade"] = "is required with a grade system"
	default:
		scale, ok := grades.Find(*f.GradeSystem)
		switch {
		case !ok:
			problems["grade_system"] = "must be font, v, french or yds"
		case scale.Boulder != (f.Discipline == "boulder"):
			problems["grade_system"] = "does not grade this kind of climb"
		default:
			if rank, ok := scale.Rank(*f.Grade); ok {
				f.rank = &rank
			} else {
				problems["grade"] = "is not on the " + scale.System + " scale"
			}
		}
	}

	f.Outcome = optional(f.Outcome)
	if f.Outcome != nil && !outcomes[*f.Outcome] {
		problems["outcome"] = "must be onsight, flash, redpoint, hangdog or working"
	}
	checkCount(f.Attempts, "attempts", problems)
	if f.Attempts != nil && *f.Attempts < 1 {
		problems["attempts"] = "must be 1 or more"
	}
	checkCount(f.Seconds, "seconds", problems)
	if f.Stars != nil && (*f.Stars < 1 || *f.Stars > 3) {
		problems["stars"] = "must be 1 to 3"
	}
	f.Focus = optional(f.Focus)
	f.Notes = optional(f.Notes)
	return f, problems
}

// ErrNoClimb covers a climb that does not exist and one this person cannot
// see.
var ErrNoClimb = errors.New("no such climb")

const climbColumns = `id, step, exercise, position, discipline, setting, board, rope_style, grade, grade_system,
	grade_rank, outcome, attempts, seconds, stars, focus, notes, created_at, updated_at`

// PutClimb writes one climb by the id the client picked, so a retry writes
// the same climb (rule 36). A step with a climb counts as done. It takes
// fields that went through Clean.
func PutClimb(ctx context.Context, pool *pgxpool.Pool, owner, runID, climbID string, f ClimbFields) (Climb, error) {
	climbID = strings.ToLower(strings.TrimSpace(climbID))
	if !uuidText.MatchString(climbID) {
		return Climb{}, ErrNoClimb
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return Climb{}, fmt.Errorf("begin climb: %w", err)
	}
	defer tx.Rollback(ctx)

	body, err := lockRunBody(ctx, tx, owner, runID)
	if err != nil {
		return Climb{}, err
	}
	step := body.step(f.Step)
	switch {
	case step == nil:
		return Climb{}, StepProblem("is not a step of this run")
	case step.Kind != "climbing":
		return Climb{}, StepProblem("logs sets, not climbs")
	case step.Status != nil && *step.Status == StepSkipped:
		return Climb{}, StepProblem("is skipped, so it keeps no climbs")
	}

	// An id already used in another run or by another account matches no
	// row here, so it reads as a climb that does not exist.
	rows, err := tx.Query(ctx, `
		INSERT INTO climb (id, run, owner, step, exercise, position, discipline, setting, board, rope_style,
			grade, grade_system, grade_rank, outcome, attempts, seconds, stars, focus, notes)
		VALUES (@id, @run, @owner, @step, @exercise, @position, @discipline, @setting, @board, @rope_style,
			@grade, @grade_system, @grade_rank, @outcome, @attempts, @seconds, @stars, @focus, @notes)
		ON CONFLICT (id) DO UPDATE SET
			step = excluded.step, exercise = excluded.exercise, position = excluded.position,
			discipline = excluded.discipline,
			setting = excluded.setting, board = excluded.board, rope_style = excluded.rope_style,
			grade = excluded.grade, grade_system = excluded.grade_system,
			grade_rank = excluded.grade_rank, outcome = excluded.outcome,
			attempts = excluded.attempts, seconds = excluded.seconds, stars = excluded.stars,
			focus = excluded.focus, notes = excluded.notes
		WHERE climb.run = excluded.run AND climb.owner = excluded.owner
		RETURNING `+climbColumns, pgx.NamedArgs{
		"id":           climbID,
		"run":          runID,
		"owner":        owner,
		"step":         f.Step,
		"exercise":     step.Exercise,
		"position":     f.Position,
		"discipline":   f.Discipline,
		"setting":      f.Setting,
		"board":        f.Board,
		"rope_style":   f.RopeStyle,
		"grade":        f.Grade,
		"grade_system": f.GradeSystem,
		"grade_rank":   f.rank,
		"outcome":      f.Outcome,
		"attempts":     f.Attempts,
		"seconds":      f.Seconds,
		"stars":        f.Stars,
		"focus":        f.Focus,
		"notes":        f.Notes,
	})
	if err != nil {
		return Climb{}, fmt.Errorf("write climb: %w", err)
	}
	climb, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Climb])
	if errors.Is(err, pgx.ErrNoRows) {
		return Climb{}, ErrNoClimb
	}
	if err != nil {
		return Climb{}, fmt.Errorf("read climb: %w", err)
	}

	if step.Status == nil {
		done := StepDone
		step.Status = &done
		if _, err := tx.Exec(ctx, `UPDATE run SET body = $3 WHERE owner = $1 AND id = $2`, owner, runID, body); err != nil {
			return Climb{}, fmt.Errorf("mark the step done: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Climb{}, fmt.Errorf("commit climb: %w", err)
	}
	return climb, nil
}

// DeleteClimb removes one climb from one of the person's runs.
func DeleteClimb(ctx context.Context, pool *pgxpool.Pool, owner, runID, climbID string) error {
	tag, err := pool.Exec(ctx, `DELETE FROM climb WHERE owner = $1 AND run = $2 AND id = $3`, owner, runID, climbID)
	if isInvalidText(err) {
		return ErrNoClimb
	}
	if err != nil {
		return fmt.Errorf("delete climb: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoClimb
	}
	return nil
}

// pruneClimbs deletes the climbs of every step the body no longer holds, now
// skips, no longer counts as climbing, or now points at another exercise, as
// pruneSets does.
func pruneClimbs(ctx context.Context, tx pgx.Tx, runID string, body RunBody) error {
	var steps, exercises []string
	for _, s := range body.Sections {
		for _, item := range s.Items {
			if st := item.Step; st != nil && st.Kind == "climbing" && (st.Status == nil || *st.Status != StepSkipped) {
				steps = append(steps, st.ID)
				exercises = append(exercises, st.Exercise)
			}
		}
	}
	_, err := tx.Exec(ctx, `
		DELETE FROM climb c
		WHERE c.run = $1 AND NOT EXISTS (
			SELECT 1 FROM unnest($2::uuid[], $3::uuid[]) AS keep (step, exercise)
			WHERE keep.step = c.step AND keep.exercise = c.exercise)`, runID, steps, exercises)
	if err != nil {
		return fmt.Errorf("prune climbs: %w", err)
	}
	return nil
}

// DatedClimb is a climb with the run it was logged in and that run's day.
type DatedClimb struct {
	Climb

	Run       string    `db:"run_id"`
	LocalDate time.Time `db:"local_date"`
}

// ListClimbs is every climb in the person's finished runs from one day to
// another, both included, by day and then in the order they were climbed.
func ListClimbs(ctx context.Context, pool *pgxpool.Pool, owner string, from, to time.Time) ([]DatedClimb, error) {
	rows, err := pool.Query(ctx, `
		SELECT `+climbColumns+`, run_id, local_date
		FROM climb
		JOIN (
			SELECT id AS run_id, local_date, started_at FROM run
			WHERE owner = $1 AND finished_at IS NOT NULL AND local_date BETWEEN $2 AND $3
		) r ON climb.run = r.run_id
		ORDER BY local_date, started_at, run_id, position, created_at, id`, owner, from, to)
	if err != nil {
		return nil, fmt.Errorf("select climbs: %w", err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[DatedClimb])
	if err != nil {
		return nil, fmt.Errorf("read climbs: %w", err)
	}
	return list, nil
}

// runClimbs lists a run's climbs in the order they were climbed.
func runClimbs(ctx context.Context, q querier, runID string) ([]Climb, error) {
	rows, err := q.Query(ctx, `SELECT `+climbColumns+` FROM climb WHERE run = $1 ORDER BY position, created_at, id`, runID)
	if err != nil {
		return nil, fmt.Errorf("select climbs: %w", err)
	}
	climbs, err := pgx.CollectRows(rows, pgx.RowToStructByName[Climb])
	if err != nil {
		return nil, fmt.Errorf("read climbs: %w", err)
	}
	return climbs, nil
}
