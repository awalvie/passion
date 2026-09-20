package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Authenticated is who is asking, read in one query from a bearer token.
// It carries no password hash, so that value never travels this path.
type Authenticated struct {
	AccountID      string    `db:"id"`
	Email          string    `db:"email"`
	DisplayName    string    `db:"display_name"`
	Timezone       string    `db:"timezone"`
	TokenID        string    `db:"token_id"`
	TokenExpiresAt time.Time `db:"token_expires_at"`
}

// ErrNoAuthToken covers a token that does not exist and one that has expired.
// They are deliberately the same, so the answer tells an attacker nothing.
var ErrNoAuthToken = errors.New("no such auth token")

func CreateAuthToken(ctx context.Context, pool *pgxpool.Pool, accountID string, hash []byte, expiresAt time.Time) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO auth_token (account_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id`, accountID, hash, expiresAt).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert auth token: %w", err)
	}
	return id, nil
}

// AuthenticateByToken finds who a live token belongs to. Expiry is filtered in
// SQL so no caller can forget to check it.
func AuthenticateByToken(ctx context.Context, pool *pgxpool.Pool, hash []byte) (Authenticated, error) {
	rows, err := pool.Query(ctx, `
		SELECT a.id, a.email, a.display_name, a.timezone,
		       t.id AS token_id, t.expires_at AS token_expires_at
		FROM auth_token t
		JOIN account a ON a.id = t.account_id
		WHERE t.token_hash = $1 AND t.expires_at > now()`, hash)
	if err != nil {
		return Authenticated{}, fmt.Errorf("select auth token: %w", err)
	}

	who, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Authenticated])
	if errors.Is(err, pgx.ErrNoRows) {
		return Authenticated{}, ErrNoAuthToken
	}
	if err != nil {
		return Authenticated{}, fmt.Errorf("read auth token: %w", err)
	}
	return who, nil
}

// SlideAuthToken pushes the expiry of a live token forward.
func SlideAuthToken(ctx context.Context, pool *pgxpool.Pool, id string, expiresAt time.Time) error {
	_, err := pool.Exec(ctx,
		`UPDATE auth_token SET expires_at = $2 WHERE id = $1`, id, expiresAt)
	if err != nil {
		return fmt.Errorf("slide auth token: %w", err)
	}
	return nil
}

// DeleteAuthToken signs one device out. Deleting a token that is already gone
// is not an error, so signing out twice is harmless.
func DeleteAuthToken(ctx context.Context, pool *pgxpool.Pool, id string) error {
	if _, err := pool.Exec(ctx, `DELETE FROM auth_token WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete auth token: %w", err)
	}
	return nil
}
