package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Set is one logged set of one step.
type Set struct {
	Step     string   `db:"step"`
	Number   int      `db:"number"`
	Exercise string   `db:"exercise"`
	Reps     *int     `db:"reps"`
	Seconds  *int     `db:"seconds"`
	WeightKG *float64 `db:"weight_kg"`
}

// SetFields is what the person logs for one set. Every number is optional,
// because a run can start with numbers left empty.
type SetFields struct {
	Reps     *int
	Seconds  *int
	WeightKG *float64
}

// maxWeightKG is the most numeric(6,2) holds.
const maxWeightKG = 9999.99

// CheckSets names each problem by its place in the list, such as
// sets[2].reps.
func CheckSets(sets []SetFields) map[string]string {
	problems := map[string]string{}
	for i, s := range sets {
		path := fmt.Sprintf("sets[%d]", i)
		checkCount(s.Reps, path+".reps", problems)
		checkCount(s.Seconds, path+".seconds", problems)
		if s.WeightKG != nil && math.Abs(*s.WeightKG) > maxWeightKG {
			problems[path+".weight_kg"] = "is too large"
		}
	}
	return problems
}

// ErrNoStep covers a step the run's body does not hold.
var ErrNoStep = errors.New("no such step")

// StepProblem says why a step cannot take what was sent to it.
type StepProblem string

func (p StepProblem) Error() string { return string(p) }

// ReplaceSets writes one step's sets whole: rows past the new count go, and
// the rest are upserted, so a retry writes the same rows (rule 36). A step
// with sets counts as done.
func ReplaceSets(ctx context.Context, pool *pgxpool.Pool, owner, runID, stepID string, sets []SetFields) ([]Set, error) {
	stepID = strings.ToLower(stepID)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin sets: %w", err)
	}
	defer tx.Rollback(ctx)

	body, err := lockRunBody(ctx, tx, owner, runID)
	if err != nil {
		return nil, err
	}
	step := body.step(stepID)
	switch {
	case step == nil:
		return nil, ErrNoStep
	case step.Status != nil && *step.Status == StepSkipped:
		return nil, StepProblem("is skipped, so it keeps no sets")
	case step.Kind == "climbing":
		return nil, StepProblem("logs climbs, not sets")
	}

	if _, err := tx.Exec(ctx, `DELETE FROM run_set WHERE run = $1 AND step = $2 AND number > $3`,
		runID, stepID, len(sets)); err != nil {
		return nil, fmt.Errorf("trim sets: %w", err)
	}
	for i, s := range sets {
		if _, err := tx.Exec(ctx, `
			INSERT INTO run_set (run, owner, step, number, exercise, reps, seconds, weight_kg)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (run, step, number) DO UPDATE SET
				exercise = excluded.exercise, reps = excluded.reps,
				seconds = excluded.seconds, weight_kg = excluded.weight_kg`,
			runID, owner, stepID, i+1, step.Exercise, s.Reps, s.Seconds, s.WeightKG); err != nil {
			return nil, fmt.Errorf("write set %d: %w", i+1, err)
		}
	}

	if len(sets) > 0 && step.Status == nil {
		done := StepDone
		step.Status = &done
		if _, err := tx.Exec(ctx, `UPDATE run SET body = $3 WHERE owner = $1 AND id = $2`, owner, runID, body); err != nil {
			return nil, fmt.Errorf("mark the step done: %w", err)
		}
	}

	out, err := runSets(ctx, tx, runID, &stepID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit sets: %w", err)
	}
	return out, nil
}

// lockRunBody reads a run's body and holds the row until the transaction
// ends, so a log write and a body write never cross.
func lockRunBody(ctx context.Context, tx pgx.Tx, owner, runID string) (RunBody, error) {
	var body RunBody
	err := tx.QueryRow(ctx, `SELECT body FROM run WHERE owner = $1 AND id = $2 FOR UPDATE`, owner, runID).Scan(&body)
	if errors.Is(err, pgx.ErrNoRows) || isInvalidText(err) {
		return RunBody{}, ErrNoRun
	}
	if err != nil {
		return RunBody{}, fmt.Errorf("select run: %w", err)
	}
	return body, nil
}

// step finds a step that stands in the body. A choice's options are not
// steps until they are picked.
func (b RunBody) step(id string) *RunStep {
	for _, s := range b.Sections {
		for _, item := range s.Items {
			if item.Step != nil && item.Step.ID == id {
				return item.Step
			}
		}
	}
	return nil
}

// pruneSets deletes the sets of every step the body no longer holds, now
// skips, or now points at another exercise. Swapping an exercise makes a new
// step.
func pruneSets(ctx context.Context, tx pgx.Tx, runID string, body RunBody) error {
	var steps, exercises []string
	for _, s := range body.Sections {
		for _, item := range s.Items {
			if st := item.Step; st != nil && (st.Status == nil || *st.Status != StepSkipped) {
				steps = append(steps, st.ID)
				exercises = append(exercises, st.Exercise)
			}
		}
	}
	_, err := tx.Exec(ctx, `
		DELETE FROM run_set s
		WHERE s.run = $1 AND NOT EXISTS (
			SELECT 1 FROM unnest($2::uuid[], $3::uuid[]) AS keep (step, exercise)
			WHERE keep.step = s.step AND keep.exercise = s.exercise)`, runID, steps, exercises)
	if err != nil {
		return fmt.Errorf("prune sets: %w", err)
	}
	return nil
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// runSets lists a run's sets, or one step's when step is not nil.
func runSets(ctx context.Context, q querier, runID string, step *string) ([]Set, error) {
	rows, err := q.Query(ctx, `
		SELECT step, number, exercise, reps, seconds, weight_kg FROM run_set
		WHERE run = $1 AND ($2::uuid IS NULL OR step = $2)
		ORDER BY step, number`, runID, step)
	if err != nil {
		return nil, fmt.Errorf("select sets: %w", err)
	}
	sets, err := pgx.CollectRows(rows, pgx.RowToStructByName[Set])
	if err != nil {
		return nil, fmt.Errorf("read sets: %w", err)
	}
	return sets, nil
}
