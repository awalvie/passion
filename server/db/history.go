package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HistoryRun is one finished run that logged an exercise, with what was
// logged for it.
type HistoryRun struct {
	Run       string    `db:"id"`
	Name      string    `db:"name"`
	LocalDate time.Time `db:"local_date"`
	Sets      []Set     `db:"-"`
	Climbs    []Climb   `db:"-"`
}

// ExerciseHistory is every finished run that logged the exercise, newest day
// first. An unfinished run counts for nothing (rule 45), and a skipped step
// keeps no rows (rule 47). The id need not be in the library: a step typed
// into a run has an id of its own, and its history is its own (rule 12).
func ExerciseHistory(ctx context.Context, pool *pgxpool.Pool, owner, exercise string) ([]HistoryRun, error) {
	exercise = strings.ToLower(strings.TrimSpace(exercise))
	if !uuidText.MatchString(exercise) {
		return nil, ErrNoExercise
	}

	rows, err := pool.Query(ctx, `
		SELECT r.id, r.name, r.local_date FROM run r
		WHERE r.owner = $1 AND r.finished_at IS NOT NULL AND (
			EXISTS (SELECT 1 FROM run_set s WHERE s.run = r.id AND s.exercise = $2)
			OR EXISTS (SELECT 1 FROM climb c WHERE c.run = r.id AND c.exercise = $2))
		ORDER BY r.local_date DESC, r.started_at DESC, r.id DESC`, owner, exercise)
	if err != nil {
		return nil, fmt.Errorf("select history runs: %w", err)
	}
	runs, err := pgx.CollectRows(rows, pgx.RowToStructByName[HistoryRun])
	if err != nil {
		return nil, fmt.Errorf("read history runs: %w", err)
	}
	ids := make([]string, 0, len(runs))
	byID := make(map[string]*HistoryRun, len(runs))
	for i := range runs {
		runs[i].Sets, runs[i].Climbs = []Set{}, []Climb{}
		ids = append(ids, runs[i].Run)
		byID[runs[i].Run] = &runs[i]
	}

	rows, err = pool.Query(ctx, `
		SELECT run::text, step, number, exercise, reps, seconds, weight_kg FROM run_set
		WHERE owner = $1 AND exercise = $2 AND run = ANY($3::uuid[])
		ORDER BY step, number`, owner, exercise, ids)
	if err != nil {
		return nil, fmt.Errorf("select history sets: %w", err)
	}
	type setOfRun struct {
		Run string `db:"run"`
		Set
	}
	sets, err := pgx.CollectRows(rows, pgx.RowToStructByName[setOfRun])
	if err != nil {
		return nil, fmt.Errorf("read history sets: %w", err)
	}
	for _, s := range sets {
		byID[s.Run].Sets = append(byID[s.Run].Sets, s.Set)
	}

	rows, err = pool.Query(ctx, `
		SELECT run::text, `+climbColumns+` FROM climb
		WHERE owner = $1 AND exercise = $2 AND run = ANY($3::uuid[])
		ORDER BY position, created_at, id`, owner, exercise, ids)
	if err != nil {
		return nil, fmt.Errorf("select history climbs: %w", err)
	}
	type climbOfRun struct {
		Run string `db:"run"`
		Climb
	}
	climbs, err := pgx.CollectRows(rows, pgx.RowToStructByName[climbOfRun])
	if err != nil {
		return nil, fmt.Errorf("read history climbs: %w", err)
	}
	for _, c := range climbs {
		byID[c.Run].Climbs = append(byID[c.Run].Climbs, c.Climb)
	}
	return runs, nil
}
