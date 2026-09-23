package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ScheduledSession is one session planned for one day. A future one follows
// its template, so editing the template tonight changes what tomorrow asks
// (rule 5).
type ScheduledSession struct {
	ID        string    `db:"id"`
	Owner     string    `db:"owner"`
	Cycle     *string   `db:"cycle"`
	Template  string    `db:"template"`
	LocalDate time.Time `db:"local_date"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

var (
	// ErrNoScheduledSession covers a row that does not exist and one this
	// person cannot see.
	ErrNoScheduledSession = errors.New("no such scheduled session")

	// ErrAlreadyScheduled refuses a second copy of a session on one day
	// (rule 37).
	ErrAlreadyScheduled = errors.New("that day already holds that session")
)

// ScheduleSession plans a one-off session, with no cycle behind it.
func ScheduleSession(ctx context.Context, pool *pgxpool.Pool, owner, template string, day time.Time) (ScheduledSession, error) {
	template = strings.ToLower(strings.TrimSpace(template))
	if !uuidText.MatchString(template) {
		return ScheduledSession{}, ErrNoSessionTemplate
	}
	found, err := visibleTemplates(ctx, pool, owner, []string{template})
	if err != nil {
		return ScheduledSession{}, err
	}
	if len(found) == 0 {
		return ScheduledSession{}, ErrNoSessionTemplate
	}

	rows, err := pool.Query(ctx, `
		INSERT INTO scheduled_session (owner, template, local_date) VALUES ($1, $2, $3)
		ON CONFLICT (owner, local_date, template) DO NOTHING
		RETURNING *`, owner, template, day)
	if err != nil {
		return ScheduledSession{}, fmt.Errorf("insert scheduled session: %w", err)
	}
	s, err := oneScheduledSession(rows)
	if errors.Is(err, ErrNoScheduledSession) {
		return ScheduledSession{}, ErrAlreadyScheduled
	}
	return s, err
}

// MoveScheduledSession puts one row on another day and touches no other.
func MoveScheduledSession(ctx context.Context, pool *pgxpool.Pool, owner, id string, day time.Time) (ScheduledSession, error) {
	rows, err := pool.Query(ctx, `
		UPDATE scheduled_session SET local_date = $3
		WHERE owner = $1 AND id = $2
		RETURNING *`, owner, id, day)
	if err != nil {
		return ScheduledSession{}, fmt.Errorf("move scheduled session: %w", err)
	}
	return oneScheduledSession(rows)
}

// DeleteScheduledSession takes one session off its day, whether a cycle or
// the person put it there.
func DeleteScheduledSession(ctx context.Context, pool *pgxpool.Pool, owner, id string) error {
	tag, err := pool.Exec(ctx, `DELETE FROM scheduled_session WHERE owner = $1 AND id = $2`, owner, id)
	if isInvalidText(err) {
		return ErrNoScheduledSession
	}
	if err != nil {
		return fmt.Errorf("delete scheduled session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoScheduledSession
	}
	return nil
}

func oneScheduledSession(rows pgx.Rows) (ScheduledSession, error) {
	s, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[ScheduledSession])
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows) || isInvalidText(err):
		return ScheduledSession{}, ErrNoScheduledSession
	case errors.As(err, &pgErr) && pgErr.Code == uniqueViolation:
		return ScheduledSession{}, ErrAlreadyScheduled
	case err != nil:
		return ScheduledSession{}, fmt.Errorf("read scheduled session: %w", err)
	}
	return s, nil
}
