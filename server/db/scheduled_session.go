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

// ScheduledDay is a scheduled session as a calendar shows it.
type ScheduledDay struct {
	ScheduledSession

	// The template's name today, as a future session follows its template.
	TemplateName string `db:"template_name"`

	// The template's icon today.
	TemplateIcon *string `db:"template_icon"`

	// The run started from it, a finished one first.
	Run *string `db:"run"`

	// done (a finished run), started (a run not finished yet), missed (a day
	// gone with no run), or planned.
	Status string `db:"status"`
}

// ListScheduledSessions lists the person's days from one date to another,
// both included. "Today" is the person's, in their own zone (rule 43).
func ListScheduledSessions(ctx context.Context, pool *pgxpool.Pool, owner string, from, to time.Time) ([]ScheduledDay, error) {
	rows, err := pool.Query(ctx, `
		SELECT s.*, t.name AS template_name, t.icon AS template_icon, r.id AS run,
			CASE
				WHEN r.finished_at IS NOT NULL THEN 'done'
				WHEN r.id IS NOT NULL THEN 'started'
				WHEN s.local_date < (now() AT TIME ZONE a.timezone)::date THEN 'missed'
				ELSE 'planned'
			END AS status
		FROM scheduled_session s
		JOIN account a ON a.id = s.owner
		JOIN session_template t ON t.id = s.template
		LEFT JOIN LATERAL (
			SELECT id, finished_at FROM run
			WHERE run.scheduled = s.id
			ORDER BY finished_at IS NULL, started_at DESC
			LIMIT 1
		) r ON true
		WHERE s.owner = $1 AND s.local_date BETWEEN $2 AND $3
		ORDER BY s.local_date, lower(t.name), s.id`, owner, from, to)
	if err != nil {
		return nil, fmt.Errorf("select scheduled sessions: %w", err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[ScheduledDay])
	if err != nil {
		return nil, fmt.Errorf("read scheduled sessions: %w", err)
	}
	return list, nil
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
