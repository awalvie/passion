package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Set is one logged set of one step.
type Set struct {
	Step       string   `db:"step"`
	Number     int      `db:"number"`
	Exercise   string   `db:"exercise"`
	Reps       *int     `db:"reps"`
	Seconds    *int     `db:"seconds"`
	RepSeconds []int    `db:"rep_seconds"`
	WeightKG   *float64 `db:"weight_kg"`
}

// SetFields is what the person logs for one set. Every number is optional,
// because a run can start with numbers left empty. RepSeconds holds each
// rep's time when they differ, and Seconds is then empty.
type SetFields struct {
	Reps       *int
	Seconds    *int
	RepSeconds []int
	WeightKG   *float64
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
		for _, n := range s.RepSeconds {
			checkCount(&n, path+".rep_seconds", problems)
		}
		if s.Seconds != nil && len(s.RepSeconds) > 0 {
			problems[path+".rep_seconds"] = "cannot be set with seconds"
		}
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
// the rest are upserted, so a retry writes the same rows. A step
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
		if len(s.RepSeconds) == 0 {
			s.RepSeconds = nil
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO run_set (run, owner, step, number, exercise, reps, seconds, rep_seconds, weight_kg)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (run, step, number) DO UPDATE SET
				exercise = excluded.exercise, reps = excluded.reps, seconds = excluded.seconds,
				rep_seconds = excluded.rep_seconds, weight_kg = excluded.weight_kg`,
			runID, owner, stepID, i+1, step.Exercise, s.Reps, s.Seconds, s.RepSeconds, s.WeightKG); err != nil {
			return nil, fmt.Errorf("write set %d: %w", i+1, err)
		}
	}

	if len(sets) > 0 {
		if err := markDone(ctx, tx, owner, runID, body, step); err != nil {
			return nil, err
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

// markDone marks a step done the first time something is logged for it. step
// points into body, which the caller locked.
func markDone(ctx context.Context, tx pgx.Tx, owner, runID string, body RunBody, step *RunStep) error {
	if step.Status != nil {
		return nil
	}
	done := StepDone
	step.Status = &done
	if _, err := tx.Exec(ctx, `UPDATE run SET body = $3 WHERE owner = $1 AND id = $2`, owner, runID, body); err != nil {
		return fmt.Errorf("mark the step done: %w", err)
	}
	return nil
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

// prune deletes from table, always a constant, what was logged for every step
// the body no longer holds, now skips, or now points at another exercise.
// Swapping an exercise makes a new step. climbs says whether the table holds
// climbs, which only a climbing step keeps; sets go with any other kind.
func prune(ctx context.Context, tx pgx.Tx, table string, climbs bool, runID string, body RunBody) error {
	var steps, exercises []string
	for _, s := range body.Sections {
		for _, item := range s.Items {
			st := item.Step
			if st != nil && (st.Status == nil || *st.Status != StepSkipped) && (st.Kind == "climbing") == climbs {
				steps = append(steps, st.ID)
				exercises = append(exercises, st.Exercise)
			}
		}
	}
	_, err := tx.Exec(ctx, `
		DELETE FROM `+table+` t
		WHERE t.run = $1 AND NOT EXISTS (
			SELECT 1 FROM unnest($2::uuid[], $3::uuid[]) AS keep (step, exercise)
			WHERE keep.step = t.step AND keep.exercise = t.exercise)`, runID, steps, exercises)
	if err != nil {
		return fmt.Errorf("prune %s: %w", table, err)
	}
	return nil
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// runSets lists a run's sets, or one step's when step is not nil.
func runSets(ctx context.Context, q querier, runID string, step *string) ([]Set, error) {
	rows, err := q.Query(ctx, `
		SELECT step, number, exercise, reps, seconds, rep_seconds, weight_kg FROM run_set
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

// DatedSet is one set of a finished run, with the run's day and the step as
// the run held it. A step typed in mid-run has no row in the library, so its
// name, kind and tags come from the run.
type DatedSet struct {
	Run       string    `db:"run"`
	LocalDate time.Time `db:"local_date"`
	Name      string    `db:"name"`
	Kind      string    `db:"kind"`
	Tags      []string  `db:"tags"`
	Set
}

// ListSets is every set of the owner's finished runs from one day to
// another, both included, by day and then in the order they were logged.
func ListSets(ctx context.Context, pool *pgxpool.Pool, owner string, from, to time.Time) ([]DatedSet, error) {
	rows, err := pool.Query(ctx, `
		SELECT s.run::text AS run, r.local_date, s.step, s.number, s.exercise, s.reps, s.seconds, s.rep_seconds, s.weight_kg,
			st->>'name' AS name, st->>'kind' AS kind, coalesce(st->'tags', '[]'::jsonb) AS tags
		FROM run_set s
		JOIN run r ON r.id = s.run
		CROSS JOIN LATERAL (
			SELECT item->'step' AS st
			FROM jsonb_array_elements(r.body->'sections') section, jsonb_array_elements(section->'items') item
			WHERE item->'step'->>'id' = s.step::text
		) step
		WHERE s.owner = $1 AND r.owner = $1 AND r.finished_at IS NOT NULL AND r.local_date BETWEEN $2 AND $3
		ORDER BY r.local_date, r.started_at, r.id, s.step, s.number`, owner, from, to)
	if err != nil {
		return nil, fmt.Errorf("select sets: %w", err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[DatedSet])
	if err != nil {
		return nil, fmt.Errorf("read sets: %w", err)
	}
	return list, nil
}
