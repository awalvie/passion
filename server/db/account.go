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

type Account struct {
	ID           string    `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	DisplayName  string    `db:"display_name"`
	Timezone     string    `db:"timezone"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
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
