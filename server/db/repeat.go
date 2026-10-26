package db

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A repeat is built this many days ahead, and topped up once fewer than
// repeatTopUp are left.
const (
	repeatAhead = 366
	repeatTopUp = 300
)

// RepeatFields is what the person sets on a repeat.
type RepeatFields struct {
	Template string
	Starts   time.Time
	// The last day, inclusive. Nil repeats for ever.
	Ends     *time.Time
	Every    int
	Unit     string
	Weekdays []int
}

func (f RepeatFields) Clean() (RepeatFields, map[string]string) {
	problems := map[string]string{}
	var ok bool
	if f.Template, ok = cleanID(f.Template); !ok {
		problems["template"] = "must be a session template id"
	}
	if f.Ends != nil && f.Ends.Before(f.Starts) {
		problems["ends"] = "must be on or after starts"
	}
	if f.Every < 1 || f.Every > 99 {
		problems["every"] = "must be 1 to 99"
	}
	switch f.Unit {
	case "day":
		f.Weekdays = []int{}
	case "week":
		f.Weekdays = slices.Compact(slices.Sorted(slices.Values(f.Weekdays)))
		switch {
		case len(f.Weekdays) == 0:
			problems["weekdays"] = "must name at least one day"
		case f.Weekdays[0] < 1 || f.Weekdays[len(f.Weekdays)-1] > 7:
			problems["weekdays"] = "must be 1 (Monday) to 7"
		}
	default:
		f.Weekdays = []int{}
		problems["unit"] = "must be day or week"
	}
	return f, problems
}

type Repeat struct {
	ID         string     `db:"id"`
	Owner      string     `db:"owner"`
	Template   string     `db:"template"`
	Starts     time.Time  `db:"starts"`
	Ends       *time.Time `db:"ends"`
	Every      int        `db:"every"`
	Unit       string     `db:"unit"`
	Weekdays   []int      `db:"weekdays"`
	BuiltUntil time.Time  `db:"built_until"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
}

// ErrNoRepeat covers a repeat that does not exist and one this person cannot
// see.
var ErrNoRepeat = errors.New("no such repeat")

// on says whether the repeat falls on day. Every few weeks counts from the
// Monday of the week it starts in.
func (r Repeat) on(day time.Time) bool {
	if day.Before(r.Starts) || (r.Ends != nil && day.After(*r.Ends)) {
		return false
	}
	if r.Unit == "day" {
		return days(r.Starts, day)%r.Every == 0
	}
	return slices.Contains(r.Weekdays, weekday(day)) && days(monday(r.Starts), monday(day))/7%r.Every == 0
}

// slots lists the repeat's days from from to until, both included.
func (r Repeat) slots(from, until time.Time) []Slot {
	var out []Slot
	for d := from; !d.After(until); d = d.AddDate(0, 0, 1) {
		if r.on(d) {
			out = append(out, Slot{LocalDate: d, Template: r.Template})
		}
	}
	return out
}

func sameRule(r Repeat, f RepeatFields) bool {
	sameEnd := (r.Ends == nil) == (f.Ends == nil) && (r.Ends == nil || r.Ends.Equal(*f.Ends))
	return r.Template == f.Template && r.Starts.Equal(f.Starts) && sameEnd && r.Every == f.Every &&
		r.Unit == f.Unit && slices.Equal(r.Weekdays, f.Weekdays)
}

func builtUntil(ends *time.Time, today time.Time) time.Time {
	until := today.AddDate(0, 0, repeatAhead)
	if ends != nil && ends.Before(until) {
		return *ends
	}
	return until
}

// weekday is 1 for Monday to 7 for Sunday.
func weekday(d time.Time) int {
	return (int(d.Weekday())+6)%7 + 1
}

func monday(d time.Time) time.Time {
	return d.AddDate(0, 0, 1-weekday(d))
}

// PutRepeat creates the repeat under the id the client chose, or replaces it.
// When the rule changes it builds the repeat's rows again from today, and
// answers the slots it left out because that day already held the session. A
// retry leaves the rows, and any the person moved or removed, alone. It takes
// fields that went through Clean.
func PutRepeat(ctx context.Context, pool *pgxpool.Pool, owner, id string, f RepeatFields) (Repeat, []Slot, error) {
	id, ok := cleanID(id)
	if !ok {
		return Repeat{}, nil, ErrNoRepeat
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return Repeat{}, nil, fmt.Errorf("begin repeat: %w", err)
	}
	defer tx.Rollback(ctx)

	unseen, err := missing(ctx, tx, "session_template", owner, map[string]string{"template": f.Template}, "")
	if err != nil {
		return Repeat{}, nil, err
	}
	if len(unseen) > 0 {
		return Repeat{}, nil, ErrNoSessionTemplate
	}
	today, err := localToday(ctx, tx, owner)
	if err != nil {
		return Repeat{}, nil, err
	}

	rows, err := tx.Query(ctx, `SELECT * FROM repeat WHERE owner = $1 AND id = $2 FOR UPDATE`, owner, id)
	if err != nil {
		return Repeat{}, nil, fmt.Errorf("select repeat: %w", err)
	}
	old, err := oneRow[Repeat](rows, ErrNoRepeat, "repeat")
	if err == nil && sameRule(old, f) {
		return old, []Slot{}, tx.Commit(ctx)
	}
	if err != nil && !errors.Is(err, ErrNoRepeat) {
		return Repeat{}, nil, err
	}

	// An id another account holds matches no row here, so it reads as a
	// repeat that does not exist.
	until := builtUntil(f.Ends, today)
	rows, err = tx.Query(ctx, `
		INSERT INTO repeat (id, owner, template, starts, ends, every, unit, weekdays, built_until)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET template = excluded.template, starts = excluded.starts,
			ends = excluded.ends, every = excluded.every, unit = excluded.unit,
			weekdays = excluded.weekdays, built_until = excluded.built_until
		WHERE repeat.owner = excluded.owner
		RETURNING *`, id, owner, f.Template, f.Starts, f.Ends, f.Every, f.Unit, f.Weekdays, until)
	if err != nil {
		return Repeat{}, nil, fmt.Errorf("write repeat: %w", err)
	}
	r, err := oneRow[Repeat](rows, ErrNoRepeat, "repeat")
	if err != nil {
		return Repeat{}, nil, err
	}

	leftOut, err := build(ctx, tx, owner, "repeat", r.ID, r.slots(today, until), today)
	if err != nil {
		return Repeat{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Repeat{}, nil, fmt.Errorf("commit repeat: %w", err)
	}
	return r, leftOut, nil
}

// TopUpRepeats builds each repeat a year ahead again once less than
// repeatTopUp days of it are built, so one with no end never runs out. A day
// that already holds the session is skipped.
func TopUpRepeats(ctx context.Context, pool *pgxpool.Pool, owner string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin top-up: %w", err)
	}
	defer tx.Rollback(ctx)

	today, err := localToday(ctx, tx, owner)
	if err != nil {
		return err
	}
	// The row lock makes a second read wait, and then find nothing to do.
	rows, err := tx.Query(ctx, `
		UPDATE repeat SET built_until = least(coalesce(ends, 'infinity'), $2::date + $3::int)
		WHERE owner = $1 AND built_until < least(coalesce(ends, 'infinity'), $2::date + $4::int)
		RETURNING new.*, old.built_until AS old_built_until`, owner, today, repeatAhead, repeatTopUp)
	if err != nil {
		return fmt.Errorf("top up repeats: %w", err)
	}
	type toppedUp struct {
		Repeat
		OldBuiltUntil time.Time `db:"old_built_until"`
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[toppedUp])
	if err != nil {
		return fmt.Errorf("read topped-up repeats: %w", err)
	}
	for _, r := range list {
		from := r.OldBuiltUntil.AddDate(0, 0, 1)
		if from.Before(today) {
			from = today
		}
		if err := place(ctx, tx, owner, "repeat", r.ID, r.slots(from, r.BuiltUntil)); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit top-up: %w", err)
	}
	return nil
}

// EndRepeat stops a repeat before day: it takes off the repeat's rows from
// day on, except one a run was started from, and leaves the earlier days as
// they are. A past day counts as today, and a day after the repeat ends
// changes nothing. Ending it on or before its first day deletes it.
func EndRepeat(ctx context.Context, pool *pgxpool.Pool, owner, id string, day time.Time) error {
	id, ok := cleanID(id)
	if !ok {
		return ErrNoRepeat
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin end repeat: %w", err)
	}
	defer tx.Rollback(ctx)

	today, err := localToday(ctx, tx, owner)
	if err != nil {
		return err
	}
	if day.Before(today) {
		day = today
	}
	tag, err := tx.Exec(ctx, `
		UPDATE repeat SET ends = least(coalesce(ends, 'infinity'), $3::date - 1)
		WHERE owner = $1 AND id = $2 AND starts < $3`, owner, id, day)
	if err != nil {
		return fmt.Errorf("end repeat: %w", err)
	}
	if tag.RowsAffected() == 0 {
		tag, err = tx.Exec(ctx, `DELETE FROM repeat WHERE owner = $1 AND id = $2`, owner, id)
		if err != nil {
			return fmt.Errorf("delete repeat: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNoRepeat
		}
	} else {
		// As in build, take the rows first so a run starting from one is seen.
		if _, err := tx.Exec(ctx, `SELECT FROM scheduled_session WHERE repeat = $1 AND local_date >= $2 FOR UPDATE`,
			id, day); err != nil {
			return fmt.Errorf("lock the repeat's days: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM scheduled_session s
			WHERE s.repeat = $1 AND s.local_date >= $2
				AND NOT EXISTS (SELECT 1 FROM run WHERE run.scheduled = s.id)`, id, day); err != nil {
			return fmt.Errorf("clear the repeat's days: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit end repeat: %w", err)
	}
	return nil
}

// ListRepeats is every repeat of the person's, latest start first.
func ListRepeats(ctx context.Context, pool *pgxpool.Pool, owner string) ([]Repeat, error) {
	rows, err := pool.Query(ctx, `SELECT * FROM repeat WHERE owner = $1 ORDER BY starts DESC, id`, owner)
	if err != nil {
		return nil, fmt.Errorf("select repeats: %w", err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[Repeat])
	if err != nil {
		return nil, fmt.Errorf("read repeats: %w", err)
	}
	return list, nil
}

// DeleteRepeat removes the repeat and every day it placed. The runs done
// from them stay.
func DeleteRepeat(ctx context.Context, pool *pgxpool.Pool, owner, id string) error {
	return execOne(ctx, pool, ErrNoRepeat, "delete repeat", `DELETE FROM repeat WHERE owner = $1 AND id = $2`, owner, id)
}
