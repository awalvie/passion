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

	"passion/server/grades"
)

type Account struct {
	ID           string    `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	DisplayName  string    `db:"display_name"`
	Timezone     string    `db:"timezone"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`

	Grades
}

// Grades are the scales the client offers first. A climb keeps the scale it
// was logged in, so changing them rewrites nothing.
type Grades struct {
	Boulder string `db:"boulder_grades"`
	Route   string `db:"route_grades"`
}

// Check names each setting that is not a scale for its kind of climb.
func (g Grades) Check() map[string]string {
	problems := map[string]string{}
	if scale, ok := grades.Find(g.Boulder); !ok || !scale.Boulder {
		problems["boulder_grades"] = "must be font or v"
	}
	if scale, ok := grades.Find(g.Route); !ok || scale.Boulder {
		problems["route_grades"] = "must be french or yds"
	}
	return problems
}

// SetGrades changes the account's grade settings. It takes Grades that
// passed Check.
func SetGrades(ctx context.Context, pool *pgxpool.Pool, id string, g Grades) (Account, error) {
	rows, err := pool.Query(ctx, `
		UPDATE account SET boulder_grades = $2, route_grades = $3
		WHERE id = $1
		RETURNING *`, id, g.Boulder, g.Route)
	if err != nil {
		return Account{}, fmt.Errorf("update grades: %w", err)
	}
	return oneAccount(rows)
}

var (
	ErrEmailTaken  = errors.New("that email address is already registered")
	ErrUnknownZone = errors.New("postgres does not know that time zone")
	ErrNoAccount   = errors.New("no such account")
)

const uniqueViolation = "23505"

// NormaliseEmail is the one place an address is cleaned up. The unique index
// on lower(email) is the backstop, not the rule.
func NormaliseEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func CreateAccount(ctx context.Context, pool *pgxpool.Pool, email, passwordHash, displayName, timezone string) (Account, error) {
	known, err := knownTimezone(ctx, pool, timezone)
	if err != nil {
		return Account{}, err
	}
	if !known {
		return Account{}, ErrUnknownZone
	}

	rows, err := pool.Query(ctx, `
		INSERT INTO account (email, password_hash, display_name, timezone)
		VALUES ($1, $2, $3, $4)
		RETURNING *`,
		NormaliseEmail(email), passwordHash, displayName, timezone)
	if err != nil {
		return Account{}, fmt.Errorf("insert account: %w", err)
	}

	account, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Account])
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return Account{}, ErrEmailTaken
		}
		return Account{}, fmt.Errorf("insert account: %w", err)
	}
	return account, nil
}

func AccountByEmail(ctx context.Context, pool *pgxpool.Pool, email string) (Account, error) {
	rows, err := pool.Query(ctx,
		`SELECT * FROM account WHERE lower(email) = $1`, NormaliseEmail(email))
	if err != nil {
		return Account{}, fmt.Errorf("select account: %w", err)
	}
	return oneAccount(rows)
}

func oneAccount(rows pgx.Rows) (Account, error) {
	account, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Account])
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNoAccount
	}
	if err != nil {
		return Account{}, fmt.Errorf("read account: %w", err)
	}
	return account, nil
}

// knownTimezone asks Postgres, because it is Postgres that computes local
// dates. Go carries its own copy of the zone database and the two drift apart
// over years of upgrades.
//
// The list includes fixed-offset names such as Etc/GMT-5, so this accepts
// them. The client offers a picker of real zones.
func knownTimezone(ctx context.Context, pool *pgxpool.Pool, name string) (bool, error) {
	var known bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_timezone_names WHERE name = $1)`, name).Scan(&known)
	if err != nil {
		return false, fmt.Errorf("check the time zone: %w", err)
	}
	return known, nil
}
