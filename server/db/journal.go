package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JournalFields is what the person writes in a journal entry.
type JournalFields struct {
	LocalDate time.Time
	Body      string
	// The session it was written during. Only a new entry takes it.
	Run *string
}

func (f JournalFields) Clean() (JournalFields, map[string]string) {
	problems := map[string]string{}
	f.Body = strings.TrimSpace(f.Body)
	if f.Body == "" {
		problems["body"] = "is required"
	}
	// Postgres text cannot hold a NUL, so it would fail as a server error.
	if strings.ContainsRune(f.Body, 0) {
		problems["body"] = "has a character that cannot be saved"
	}
	if f.Run != nil {
		id, ok := cleanID(*f.Run)
		f.Run = &id
		if !ok {
			problems["run"] = "must be a session"
		}
	}
	return f, problems
}

type JournalEntry struct {
	ID        string     `db:"id"`
	LocalDate time.Time  `db:"local_date"`
	Body      string     `db:"body"`
	Run       *string    `db:"run"`
	PinnedAt  *time.Time `db:"pinned_at"`

	// The run's name and length, read with the entry, for its link.
	RunName           *string `db:"run_name"`
	RunElapsedSeconds *int    `db:"run_elapsed_seconds"`
}

// SessionJournal is a run's end-of-session journal, as the journal lists it.
type SessionJournal struct {
	Run       string    `db:"run"`
	Name      string    `db:"name"`
	LocalDate time.Time `db:"local_date"`
	RPE       *int      `db:"rpe"`
	WentWell  *string   `db:"went_well"`
	NextFocus *string   `db:"next_focus"`
	Notes     *string   `db:"notes"`
}

// ErrNoJournalEntry covers an entry that does not exist and one this person
// cannot see.
var ErrNoJournalEntry = errors.New("no such journal entry")

// ErrNoJournalRun is a run that does not exist or is not the person's.
var ErrNoJournalRun = errors.New("no such run for the journal entry")

// readEntries reads the rows of a query named e, with each one's run.
const readEntries = `
	SELECT e.id::text, e.local_date, e.body, e.run::text, e.pinned_at,
		r.name AS run_name, r.elapsed_seconds AS run_elapsed_seconds
	FROM e LEFT JOIN run r ON r.id = e.run`

// PutJournalEntry creates the entry under the id the client chose, or replaces
// its date and text. The run is set when the entry is made and never after.
// It takes fields that went through Clean.
func PutJournalEntry(ctx context.Context, pool *pgxpool.Pool, owner, id string, f JournalFields) (JournalEntry, error) {
	id, ok := cleanID(id)
	if !ok {
		return JournalEntry{}, ErrNoJournalEntry
	}
	// An id another account holds matches no row here, so it reads as an
	// entry that does not exist.
	rows, err := pool.Query(ctx, `
		WITH e AS (
			INSERT INTO journal_entry (id, owner, local_date, body, run)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE SET local_date = excluded.local_date, body = excluded.body
			WHERE journal_entry.owner = excluded.owner
			RETURNING *
		)`+readEntries, id, owner, f.LocalDate, f.Body, f.Run)
	if err != nil {
		return JournalEntry{}, fmt.Errorf("put journal entry: %w", err)
	}
	entry, err := oneRow[JournalEntry](rows, ErrNoJournalEntry, "journal entry")
	if hasCode(err, foreignKeyViolation) {
		return JournalEntry{}, ErrNoJournalRun
	}
	return entry, err
}

// PinJournalEntry pins or unpins an entry. Pinning a pinned entry keeps the
// time it was first pinned, so its place among the pins does not move.
func PinJournalEntry(ctx context.Context, pool *pgxpool.Pool, owner, id string, pinned bool) (JournalEntry, error) {
	rows, err := pool.Query(ctx, `
		WITH e AS (
			UPDATE journal_entry
			SET pinned_at = CASE WHEN $3 THEN coalesce(pinned_at, now()) END
			WHERE owner = $1 AND id = $2
			RETURNING *
		)`+readEntries, owner, id, pinned)
	if err != nil {
		return JournalEntry{}, fmt.Errorf("pin journal entry: %w", err)
	}
	return oneRow[JournalEntry](rows, ErrNoJournalEntry, "journal entry")
}

// ListJournal is every entry of the person's, newest day first, and every
// run with something written in its journal.
func ListJournal(ctx context.Context, pool *pgxpool.Pool, owner string) ([]JournalEntry, []SessionJournal, error) {
	rows, err := pool.Query(ctx, `WITH e AS (SELECT * FROM journal_entry WHERE owner = $1)`+readEntries+`
		ORDER BY e.local_date DESC, e.created_at DESC, e.id DESC`, owner)
	if err != nil {
		return nil, nil, fmt.Errorf("select journal entries: %w", err)
	}
	entries, err := pgx.CollectRows(rows, pgx.RowToStructByName[JournalEntry])
	if err != nil {
		return nil, nil, fmt.Errorf("read journal entries: %w", err)
	}

	rows, err = pool.Query(ctx, `
		SELECT id::text AS run, name, local_date, rpe, went_well, next_focus, notes
		FROM run
		WHERE owner = $1 AND num_nonnulls(went_well, next_focus, notes) > 0
		ORDER BY local_date DESC, started_at DESC, id DESC`, owner)
	if err != nil {
		return nil, nil, fmt.Errorf("select session journals: %w", err)
	}
	sessions, err := pgx.CollectRows(rows, pgx.RowToStructByName[SessionJournal])
	if err != nil {
		return nil, nil, fmt.Errorf("read session journals: %w", err)
	}
	return entries, sessions, nil
}

func GetJournalEntry(ctx context.Context, pool *pgxpool.Pool, owner, id string) (JournalEntry, error) {
	rows, err := pool.Query(ctx, `WITH e AS (SELECT * FROM journal_entry WHERE owner = $1 AND id = $2)`+readEntries, owner, id)
	if err != nil {
		return JournalEntry{}, fmt.Errorf("select journal entry: %w", err)
	}
	return oneRow[JournalEntry](rows, ErrNoJournalEntry, "journal entry")
}

func DeleteJournalEntry(ctx context.Context, pool *pgxpool.Pool, owner, id string) error {
	return execOne(ctx, pool, ErrNoJournalEntry, "delete journal entry", `DELETE FROM journal_entry WHERE owner = $1 AND id = $2`, owner, id)
}
