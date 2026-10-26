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

var timeOffKinds = []string{"trip", "injury", "illness", "other"}

// TimeOffFields is what the person sets on a time off.
type TimeOffFields struct {
	Kind   string
	Name   string
	Starts time.Time
	// The last day, inclusive. Nil while the end is not known yet.
	Ends   *time.Time
	Pauses bool
}

func (f TimeOffFields) Clean() (TimeOffFields, map[string]string) {
	problems := map[string]string{}
	if !slices.Contains(timeOffKinds, f.Kind) {
		problems["kind"] = "must be trip, injury, illness or other"
	}
	f.Name = cleanName(f.Name, "name", problems)
	if f.Ends != nil && f.Ends.Before(f.Starts) {
		problems["ends"] = "must be on or after starts"
	}
	return f, problems
}

type TimeOff struct {
	ID     string     `db:"id"`
	Kind   string     `db:"kind"`
	Name   string     `db:"name"`
	Starts time.Time  `db:"starts"`
	Ends   *time.Time `db:"ends"`
	Pauses bool       `db:"pauses"`
}

// ErrNoTimeOff covers a time off that does not exist and one this person
// cannot see.
var ErrNoTimeOff = errors.New("no such time off")

const timeOffColumns = `id::text, kind, name, starts, ends, pauses`

// PutTimeOff creates the time off under the id the client chose, or replaces
// it. It takes fields that went through Clean.
func PutTimeOff(ctx context.Context, pool *pgxpool.Pool, owner, id string, f TimeOffFields) (TimeOff, error) {
	id, ok := cleanID(id)
	if !ok {
		return TimeOff{}, ErrNoTimeOff
	}
	// An id another account holds matches no row here, so it reads as a time
	// off that does not exist.
	rows, err := pool.Query(ctx, `
		INSERT INTO time_off (id, owner, kind, name, starts, ends, pauses)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET kind = excluded.kind, name = excluded.name,
			starts = excluded.starts, ends = excluded.ends, pauses = excluded.pauses
		WHERE time_off.owner = excluded.owner
		RETURNING `+timeOffColumns, id, owner, f.Kind, f.Name, f.Starts, f.Ends, f.Pauses)
	if err != nil {
		return TimeOff{}, fmt.Errorf("put time off: %w", err)
	}
	return oneRow[TimeOff](rows, ErrNoTimeOff, "time off")
}

// ListTimeOff is every time off of the person's, latest start first.
func ListTimeOff(ctx context.Context, pool *pgxpool.Pool, owner string) ([]TimeOff, error) {
	rows, err := pool.Query(ctx, `SELECT `+timeOffColumns+` FROM time_off WHERE owner = $1 ORDER BY starts DESC, id`, owner)
	if err != nil {
		return nil, fmt.Errorf("select time off: %w", err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[TimeOff])
	if err != nil {
		return nil, fmt.Errorf("read time off: %w", err)
	}
	return list, nil
}

func GetTimeOff(ctx context.Context, pool *pgxpool.Pool, owner, id string) (TimeOff, error) {
	rows, err := pool.Query(ctx, `SELECT `+timeOffColumns+` FROM time_off WHERE owner = $1 AND id = $2`, owner, id)
	if err != nil {
		return TimeOff{}, fmt.Errorf("select time off: %w", err)
	}
	return oneRow[TimeOff](rows, ErrNoTimeOff, "time off")
}

func DeleteTimeOff(ctx context.Context, pool *pgxpool.Pool, owner, id string) error {
	return execOne(ctx, pool, ErrNoTimeOff, "delete time off", `DELETE FROM time_off WHERE owner = $1 AND id = $2`, owner, id)
}
