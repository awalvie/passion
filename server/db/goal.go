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
)

var goalPeriods = []string{"week", "month", "year", "custom"}

// PeriodGoalFields is what the person sets on a goal. A custom goal runs once
// over Span.
type PeriodGoalFields struct {
	Text   string
	Period string
	Span   *GoalSpanFields
}

// GoalSpanFields is the days a custom goal runs over. Every goal in the span
// shares them, so a change here changes them all.
type GoalSpanFields struct {
	ID     string
	Name   string
	Starts time.Time
	Ends   time.Time
}

// Clean leaves the order of the span's days alone when either did not parse,
// so the person reads why that one failed.
func (f PeriodGoalFields) Clean() (PeriodGoalFields, map[string]string) {
	problems := map[string]string{}
	f.Text = cleanName(f.Text, "text", problems)
	if !slices.Contains(goalPeriods, f.Period) {
		problems["period"] = "must be week, month, year or custom"
	}
	switch {
	case f.Period == "custom" && f.Span == nil:
		problems["span"] = "is required"
	case f.Period != "custom" && f.Span != nil:
		problems["span"] = "is only for a custom goal"
	case f.Span != nil:
		span := *f.Span
		var ok bool
		if span.ID, ok = cleanID(span.ID); !ok {
			problems["span.id"] = "is required"
		}
		if span.Name = strings.TrimSpace(span.Name); span.Name != "" {
			span.Name = cleanName(span.Name, "span.name", problems)
		}
		if !span.Starts.IsZero() && !span.Ends.IsZero() && span.Ends.Before(span.Starts) {
			problems["span.ends"] = "must be on or after the first day"
		}
		f.Span = &span
	}
	return f, problems
}

// PeriodGoal is a goal for every week, month or year, or for a span of days
// the person picked. A cycle's own goals are Goal.
type PeriodGoal struct {
	ID      string     `db:"id"`
	Text    string     `db:"text"`
	Period  string     `db:"period"`
	Starts  time.Time  `db:"starts"`
	Removed *time.Time `db:"removed"`
	Span    *GoalSpan  `db:"span"`
	Ticks   []GoalTick `db:"ticks"`
}

// GoalSpan is a custom goal's days, written YYYY-MM-DD. With no name, they
// are known by their dates.
type GoalSpan struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Starts string `json:"starts"`
	Ends   string `json:"ends"`
}

// GoalTick is a period the goal was met, with the goal's text as it read then.
type GoalTick struct {
	PeriodStart string `json:"period_start"`
	Text        string `json:"text"`
}

var (
	// ErrNoGoal covers a goal or span that does not exist, one this person
	// cannot see, and a goal they removed.
	ErrNoGoal = errors.New("no such goal")
	// ErrPeriodLocked is a replace that changes the goal's period.
	ErrPeriodLocked = errors.New("a goal's period cannot change")
	// ErrSpanLocked is a replace that moves a custom goal to another span.
	ErrSpanLocked = errors.New("a goal's span cannot change")
	// ErrNotAPeriod is a tick on a day that does not start one of the goal's
	// periods, one before the goal was set, or one still to come.
	ErrNotAPeriod = errors.New("not one of the goal's periods")
)

// readGoals reads the goals in g with their spans and ticks.
const readGoals = `
	SELECT g.id::text, g.text, g.period, g.starts, g.removed,
		(SELECT json_build_object('id', s.id, 'name', s.name, 'starts', s.starts, 'ends', s.ends)
			FROM goal_span s WHERE s.id = g.span) AS span,
		coalesce((SELECT json_agg(json_build_object('period_start', t.period_start, 'text', t.text) ORDER BY t.period_start)
			FROM goal_tick t WHERE t.goal = g.id), '[]') AS ticks
	FROM g`

// PutGoal creates the goal under the id the client chose, set today, or
// replaces its text and its span's name and days. It takes fields that went
// through Clean.
func PutGoal(ctx context.Context, pool *pgxpool.Pool, owner, id string, f PeriodGoalFields) (PeriodGoal, error) {
	id, ok := cleanID(id)
	if !ok {
		return PeriodGoal{}, ErrNoGoal
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return PeriodGoal{}, fmt.Errorf("begin goal: %w", err)
	}
	defer tx.Rollback(ctx)

	var span *string
	if f.Span != nil {
		if err := putSpan(ctx, tx, owner, *f.Span); err != nil {
			return PeriodGoal{}, err
		}
		span = &f.Span.ID
	}

	// An id another account holds matches no row here, so it reads as a goal
	// that does not exist.
	rows, err := tx.Query(ctx, `
		WITH g AS (
			INSERT INTO goal (id, owner, text, period, starts, span)
			SELECT $1, $2, $3, $4, (now() AT TIME ZONE timezone)::date, $5 FROM account WHERE id = $2
			ON CONFLICT (id) DO UPDATE SET text = excluded.text
			WHERE goal.owner = excluded.owner AND goal.removed IS NULL
				AND goal.period = excluded.period AND goal.span IS NOT DISTINCT FROM excluded.span
			RETURNING *
		)`+readGoals, id, owner, f.Text, f.Period, span)
	if err != nil {
		return PeriodGoal{}, fmt.Errorf("put goal: %w", err)
	}
	g, err := oneRow[PeriodGoal](rows, ErrNoGoal, "goal")
	if errors.Is(err, ErrNoGoal) {
		tx.Rollback(ctx)
		if old, err := getGoal(ctx, pool, owner, id); err == nil {
			if old.Period != f.Period {
				return PeriodGoal{}, ErrPeriodLocked
			}
			return PeriodGoal{}, ErrSpanLocked
		}
	}
	if err != nil {
		return PeriodGoal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PeriodGoal{}, fmt.Errorf("commit goal: %w", err)
	}
	return g, nil
}

// putSpan creates the span or changes it. A new first day takes its goals'
// ticks with it, as each names the span's first day. The last write wins, as
// a cycle's does.
func putSpan(ctx context.Context, tx pgx.Tx, owner string, f GoalSpanFields) error {
	var old time.Time
	err := tx.QueryRow(ctx, `SELECT starts FROM goal_span WHERE id = $1 AND owner = $2 FOR UPDATE`, f.ID, owner).Scan(&old)
	moved := err == nil && !old.Equal(f.Starts)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("read goal span: %w", err)
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO goal_span (id, owner, name, starts, ends) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET name = excluded.name, starts = excluded.starts, ends = excluded.ends
		WHERE goal_span.owner = excluded.owner`, f.ID, owner, f.Name, f.Starts, f.Ends)
	if err != nil {
		return fmt.Errorf("put goal span: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoGoal
	}
	if moved {
		_, err = tx.Exec(ctx, `
			UPDATE goal_tick t SET period_start = $3 FROM goal g
			WHERE t.goal = g.id AND g.span = $1 AND t.owner = $2 AND t.period_start = $4`, f.ID, owner, f.Starts, old)
		if err != nil {
			return fmt.Errorf("move goal ticks: %w", err)
		}
	}
	return nil
}

// ListGoals is every goal of the person's, oldest first. Removed ones are
// there for the periods they counted in.
func ListGoals(ctx context.Context, pool *pgxpool.Pool, owner string) ([]PeriodGoal, error) {
	rows, err := pool.Query(ctx, `WITH g AS (SELECT * FROM goal WHERE owner = $1)`+readGoals+`
		ORDER BY g.created_at, g.id`, owner)
	if err != nil {
		return nil, fmt.Errorf("select goals: %w", err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[PeriodGoal])
	if err != nil {
		return nil, fmt.Errorf("read goals: %w", err)
	}
	return list, nil
}

func getGoal(ctx context.Context, pool *pgxpool.Pool, owner, id string) (PeriodGoal, error) {
	rows, err := pool.Query(ctx, `WITH g AS (SELECT * FROM goal WHERE owner = $1 AND id = $2 AND removed IS NULL)`+readGoals,
		owner, id)
	if err != nil {
		return PeriodGoal{}, fmt.Errorf("select goal: %w", err)
	}
	return oneRow[PeriodGoal](rows, ErrNoGoal, "goal")
}

// RemoveGoal takes the goal off the person's list from today. Its ticks stay.
func RemoveGoal(ctx context.Context, pool *pgxpool.Pool, owner, id string) error {
	return execOne(ctx, pool, ErrNoGoal, "remove goal", `
		UPDATE goal SET removed = (SELECT (now() AT TIME ZONE timezone)::date FROM account WHERE id = $1)
		WHERE owner = $1 AND id = $2 AND removed IS NULL`, owner, id)
}

// TickGoal marks the period that starts on day as met, or not. A custom goal
// has one period, which starts on its span's first day. Ticking twice keeps
// the first tick.
func TickGoal(ctx context.Context, pool *pgxpool.Pool, owner, id string, day time.Time, ticked bool) (PeriodGoal, error) {
	g, err := getGoal(ctx, pool, owner, id)
	if err != nil {
		return PeriodGoal{}, err
	}
	var ok bool
	if g.Span != nil {
		ok = day.Format(time.DateOnly) == g.Span.Starts
	} else {
		ok = periodStart(g.Period, day).Equal(day) && !day.Before(periodStart(g.Period, g.Starts))
	}
	var today time.Time
	if err := pool.QueryRow(ctx, `SELECT (now() AT TIME ZONE timezone)::date FROM account WHERE id = $1`, owner).Scan(&today); err != nil {
		return PeriodGoal{}, fmt.Errorf("select today: %w", err)
	}
	// A span moved later takes its tick to a day still to come, so an untick
	// may name one.
	if !ok || ticked && day.After(today) {
		return PeriodGoal{}, ErrNotAPeriod
	}
	if ticked {
		_, err = pool.Exec(ctx, `
			INSERT INTO goal_tick (goal, owner, period_start, text)
			SELECT id, owner, $3, text FROM goal WHERE owner = $1 AND id = $2
			ON CONFLICT DO NOTHING`, owner, g.ID, day)
	} else {
		_, err = pool.Exec(ctx, `DELETE FROM goal_tick WHERE owner = $1 AND goal = $2 AND period_start = $3`, owner, g.ID, day)
	}
	if err != nil {
		return PeriodGoal{}, fmt.Errorf("tick goal: %w", err)
	}
	return getGoal(ctx, pool, owner, g.ID)
}

// periodStart is the first day of the period holding day: its Monday, the
// 1st of its month, or 1 January.
func periodStart(period string, day time.Time) time.Time {
	switch period {
	case "week":
		return day.AddDate(0, 0, -(int(day.Weekday())+6)%7)
	case "month":
		return time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, day.Location())
	default:
		return time.Date(day.Year(), 1, 1, 0, 0, 0, 0, day.Location())
	}
}
