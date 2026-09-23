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

// maxCycleDays keeps a build to a year of rows.
const maxCycleDays = 366

// Cycle repeats a block of days from starts to ends.
type Cycle struct {
	ID        string    `db:"id"`
	Owner     string    `db:"owner"`
	Name      string    `db:"name"`
	Starts    time.Time `db:"starts"`
	Ends      time.Time `db:"ends"`
	BlockDays int       `db:"block_days"`
	Body      CycleBody `db:"body"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// CycleBody is the block's shape: which session falls on which of its days.
type CycleBody struct {
	Days []CycleDay `json:"days"`
}

// CycleDay places a session on a day of the block, from 1.
type CycleDay struct {
	Day      int    `json:"day"`
	Template string `json:"template"`
}

// Slot is one session on one date.
type Slot struct {
	LocalDate time.Time
	Template  string
}

// ErrNoCycle covers a cycle that does not exist and one this person cannot
// see.
var ErrNoCycle = errors.New("no such cycle")

// UnknownTemplatesError names, by path, each day whose session template the
// person cannot see.
type UnknownTemplatesError struct {
	Problems map[string]string
}

func (e *UnknownTemplatesError) Error() string {
	return fmt.Sprintf("%d days name a session template you cannot see", len(e.Problems))
}

// CycleFields is what the person sets on a cycle.
type CycleFields struct {
	Name      string
	Starts    time.Time
	Ends      time.Time
	BlockDays int
	Body      CycleBody
}

func (f CycleFields) Clean() (CycleFields, map[string]string) {
	problems := map[string]string{}
	f.Name = cleanName(f.Name, "name", problems)

	length := days(f.Starts, f.Ends) + 1
	switch {
	case length < 1:
		problems["ends"] = "must be on or after starts"
	case length > maxCycleDays:
		problems["ends"] = "must be within a year of starts"
	}
	if f.BlockDays < 1 || (length >= 1 && f.BlockDays > length) {
		problems["block_days"] = "must be 1 or more, and no longer than the cycle"
	}

	if f.Body.Days == nil {
		f.Body.Days = []CycleDay{}
	}
	for i, d := range f.Body.Days {
		path := fmt.Sprintf("days[%d]", i)
		d.Template = strings.ToLower(strings.TrimSpace(d.Template))
		f.Body.Days[i] = d
		if d.Day < 1 || d.Day > f.BlockDays {
			problems[path+".day"] = "must be 1 to block_days"
		}
		if !uuidText.MatchString(d.Template) {
			problems[path+".template"] = "must be a session template id"
		}
		if slices.Contains(f.Body.Days[:i], d) {
			problems[path] = "places the same session on the same day twice"
		}
	}
	return f, problems
}

// days counts whole days from a to b.
func days(a, b time.Time) int {
	return int(b.Sub(a).Hours() / 24)
}

// slots lists every session the cycle places on or after from, in date order.
func (c Cycle) slots(from time.Time) []Slot {
	var out []Slot
	for _, d := range c.Body.Days {
		for date := c.Starts.AddDate(0, 0, d.Day-1); !date.After(c.Ends); date = date.AddDate(0, 0, c.BlockDays) {
			if !date.Before(from) {
				out = append(out, Slot{LocalDate: date, Template: d.Template})
			}
		}
	}
	slices.SortFunc(out, func(a, b Slot) int {
		if c := a.LocalDate.Compare(b.LocalDate); c != 0 {
			return c
		}
		return strings.Compare(a.Template, b.Template)
	})
	return out
}

// PutCycle creates the cycle under the id the client chose, or replaces it.
// When the shape changes it builds the cycle's rows again from today, and
// answers the slots it left out because that day already held the session. A
// rename leaves the rows, and any the person moved, alone. It takes fields
// that went through Clean.
func PutCycle(ctx context.Context, pool *pgxpool.Pool, owner, id string, f CycleFields) (Cycle, []Slot, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !uuidText.MatchString(id) {
		return Cycle{}, nil, ErrNoCycle
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return Cycle{}, nil, fmt.Errorf("begin cycle: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := checkTemplates(ctx, tx, owner, f.Body); err != nil {
		return Cycle{}, nil, err
	}

	rows, err := tx.Query(ctx, `SELECT * FROM cycle WHERE id = $1 AND owner = $2 FOR UPDATE`, id, owner)
	if err != nil {
		return Cycle{}, nil, fmt.Errorf("select cycle: %w", err)
	}
	old, err := oneCycle(rows)
	if err != nil && !errors.Is(err, ErrNoCycle) {
		return Cycle{}, nil, err
	}
	built := err == nil

	// An id another account holds matches no row here, so it reads as a
	// cycle that does not exist.
	rows, err = tx.Query(ctx, `
		INSERT INTO cycle (id, owner, name, starts, ends, block_days, body)
		VALUES (@id, @owner, @name, @starts, @ends, @block_days, @body)
		ON CONFLICT (id) DO UPDATE SET
			name = excluded.name, starts = excluded.starts, ends = excluded.ends,
			block_days = excluded.block_days, body = excluded.body
		WHERE cycle.owner = excluded.owner
		RETURNING *`, pgx.NamedArgs{
		"id":         id,
		"owner":      owner,
		"name":       f.Name,
		"starts":     f.Starts,
		"ends":       f.Ends,
		"block_days": f.BlockDays,
		"body":       f.Body,
	})
	if err != nil {
		return Cycle{}, nil, fmt.Errorf("write cycle: %w", err)
	}
	cycle, err := oneCycle(rows)
	if err != nil {
		return Cycle{}, nil, err
	}

	leftOut := []Slot{}
	if !built || !sameShape(old, cycle) {
		if leftOut, err = build(ctx, tx, cycle); err != nil {
			return Cycle{}, nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Cycle{}, nil, fmt.Errorf("commit cycle: %w", err)
	}
	return cycle, leftOut, nil
}

func sameShape(a, b Cycle) bool {
	return a.Starts.Equal(b.Starts) && a.Ends.Equal(b.Ends) && a.BlockDays == b.BlockDays &&
		slices.Equal(a.Body.Days, b.Body.Days)
}

// build replaces the cycle's rows from today on. A past day is what the runs
// recorded, so it is never planned again, and a row a run was started from
// stays.
func build(ctx context.Context, tx pgx.Tx, c Cycle) ([]Slot, error) {
	var today time.Time
	if err := tx.QueryRow(ctx, `SELECT (now() AT TIME ZONE timezone)::date FROM account WHERE id = $1`,
		c.Owner).Scan(&today); err != nil {
		return nil, fmt.Errorf("select today: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM scheduled_session s
		WHERE s.cycle = $1 AND s.local_date >= $2
			AND NOT EXISTS (SELECT 1 FROM run WHERE run.scheduled = s.id)`, c.ID, today); err != nil {
		return nil, fmt.Errorf("clear the cycle's days: %w", err)
	}

	slots := c.slots(today)
	dates := make([]time.Time, 0, len(slots))
	templates := make([]string, 0, len(slots))
	for _, s := range slots {
		dates = append(dates, s.LocalDate)
		templates = append(templates, s.Template)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO scheduled_session (owner, cycle, template, local_date)
		SELECT $1, $2, s.template, s.local_date
		FROM unnest($3::date[], $4::uuid[]) AS s (local_date, template)
		ON CONFLICT (owner, local_date, template) DO NOTHING`, c.Owner, c.ID, dates, templates); err != nil {
		return nil, fmt.Errorf("write the cycle's days: %w", err)
	}

	// A slot is left out when a row of another cycle, or a one-off, holds it.
	rows, err := tx.Query(ctx, `
		SELECT local_date, template::text FROM scheduled_session
		WHERE cycle = $1 AND local_date >= $2`, c.ID, today)
	if err != nil {
		return nil, fmt.Errorf("select the cycle's days: %w", err)
	}
	held, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Slot, error) {
		var s Slot
		err := row.Scan(&s.LocalDate, &s.Template)
		return s, err
	})
	if err != nil {
		return nil, fmt.Errorf("read the cycle's days: %w", err)
	}

	leftOut := []Slot{}
	for _, s := range slots {
		if !slices.ContainsFunc(held, func(w Slot) bool { return w.LocalDate.Equal(s.LocalDate) && w.Template == s.Template }) {
			leftOut = append(leftOut, s)
		}
	}
	return leftOut, nil
}

// checkTemplates lets a retired template through, as a step keeps a retired
// exercise.
func checkTemplates(ctx context.Context, tx pgx.Tx, owner string, body CycleBody) error {
	if len(body.Days) == 0 {
		return nil
	}
	ids := make([]string, 0, len(body.Days))
	for _, d := range body.Days {
		ids = append(ids, d.Template)
	}
	found, err := visibleTemplates(ctx, tx, owner, ids)
	if err != nil {
		return err
	}

	problems := map[string]string{}
	for i, d := range body.Days {
		if !slices.Contains(found, d.Template) {
			problems[fmt.Sprintf("days[%d].template", i)] = "is not a session template you can see"
		}
	}
	if len(problems) > 0 {
		return &UnknownTemplatesError{Problems: problems}
	}
	return nil
}

// visibleTemplates keeps the ids of templates the person can see: shipped
// ones and their own, retired ones included.
func visibleTemplates(ctx context.Context, q querier, owner string, ids []string) ([]string, error) {
	rows, err := q.Query(ctx, `
		SELECT id::text FROM session_template
		WHERE id = ANY($1::uuid[]) AND (owner IS NULL OR owner = $2)`, ids, owner)
	if err != nil {
		return nil, fmt.Errorf("select session templates: %w", err)
	}
	found, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("read session templates: %w", err)
	}
	return found, nil
}

// ListCycles is every cycle of the person's, latest start first.
func ListCycles(ctx context.Context, pool *pgxpool.Pool, owner string) ([]Cycle, error) {
	rows, err := pool.Query(ctx, `SELECT * FROM cycle WHERE owner = $1 ORDER BY starts DESC, id`, owner)
	if err != nil {
		return nil, fmt.Errorf("select cycles: %w", err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[Cycle])
	if err != nil {
		return nil, fmt.Errorf("read cycles: %w", err)
	}
	return list, nil
}

func GetCycle(ctx context.Context, pool *pgxpool.Pool, owner, id string) (Cycle, error) {
	rows, err := pool.Query(ctx, `SELECT * FROM cycle WHERE owner = $1 AND id = $2`, owner, id)
	if err != nil {
		return Cycle{}, fmt.Errorf("select cycle: %w", err)
	}
	return oneCycle(rows)
}

// DeleteCycle removes the plan and every day it placed. The runs done under
// it stay, and so do the one-off days added by hand.
func DeleteCycle(ctx context.Context, pool *pgxpool.Pool, owner, id string) error {
	tag, err := pool.Exec(ctx, `DELETE FROM cycle WHERE owner = $1 AND id = $2`, owner, id)
	if isInvalidText(err) {
		return ErrNoCycle
	}
	if err != nil {
		return fmt.Errorf("delete cycle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoCycle
	}
	return nil
}

func oneCycle(rows pgx.Rows) (Cycle, error) {
	cycle, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Cycle])
	if errors.Is(err, pgx.ErrNoRows) || isInvalidText(err) {
		return Cycle{}, ErrNoCycle
	}
	if err != nil {
		return Cycle{}, fmt.Errorf("read cycle: %w", err)
	}
	return cycle, nil
}
