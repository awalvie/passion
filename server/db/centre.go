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

// CentreFields is what the person sets on a climbing centre.
type CentreFields struct {
	Name     string
	Sessions []string
}

// Clean trims the name, and drops repeats from the sessions.
func (f CentreFields) Clean() (CentreFields, map[string]string) {
	problems := map[string]string{}
	f.Name = cleanName(f.Name, "name", problems)

	sessions := []string{}
	for i, raw := range f.Sessions {
		id, ok := cleanID(raw)
		switch {
		case !ok:
			problems[fmt.Sprintf("sessions[%d]", i)] = "must be a session template id"
		case !slices.Contains(sessions, id):
			sessions = append(sessions, id)
		}
	}
	f.Sessions = sessions
	return f, problems
}

type Centre struct {
	ID        string    `db:"id"`
	Owner     string    `db:"owner"`
	Name      string    `db:"name"`
	Sessions  []string  `db:"sessions"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// ErrNoCentre covers a centre that does not exist and one this person cannot
// see.
var ErrNoCentre = errors.New("no such centre")

const centreColumns = `id::text, owner::text, name, sessions::text[] AS sessions, created_at, updated_at`

// PutCentre creates the centre under the id the client chose, or replaces it.
// It takes fields that went through Clean.
func PutCentre(ctx context.Context, pool *pgxpool.Pool, owner, id string, f CentreFields) (Centre, error) {
	id, ok := cleanID(id)
	if !ok {
		return Centre{}, ErrNoCentre
	}

	byPath := map[string]string{}
	for i, s := range f.Sessions {
		byPath[fmt.Sprintf("sessions[%d]", i)] = s
	}
	problems, err := missing(ctx, pool, "session_template", owner, byPath, "is not a session template you can see")
	if err != nil {
		return Centre{}, err
	}
	if len(problems) > 0 {
		return Centre{}, &UnknownTemplatesError{Problems: problems}
	}

	// An id another account holds matches no row here, so it reads as a
	// centre that does not exist.
	rows, err := pool.Query(ctx, `
		INSERT INTO centre (id, owner, name, sessions)
		VALUES ($1, $2, $3, $4::uuid[])
		ON CONFLICT (id) DO UPDATE SET name = excluded.name, sessions = excluded.sessions
		WHERE centre.owner = excluded.owner
		RETURNING `+centreColumns, id, owner, f.Name, f.Sessions)
	if err != nil {
		return Centre{}, fmt.Errorf("put centre: %w", err)
	}
	return oneRow[Centre](rows, ErrNoCentre, "centre")
}

// ListCentres is every centre of the person's, by name.
func ListCentres(ctx context.Context, pool *pgxpool.Pool, owner string) ([]Centre, error) {
	rows, err := pool.Query(ctx, `SELECT `+centreColumns+` FROM centre WHERE owner = $1 ORDER BY lower(name), id`, owner)
	if err != nil {
		return nil, fmt.Errorf("select centres: %w", err)
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[Centre])
	if err != nil {
		return nil, fmt.Errorf("read centres: %w", err)
	}
	return list, nil
}

func DeleteCentre(ctx context.Context, pool *pgxpool.Pool, owner, id string) error {
	return execOne(ctx, pool, ErrNoCentre, "delete centre", `DELETE FROM centre WHERE owner = $1 AND id = $2`, owner, id)
}
