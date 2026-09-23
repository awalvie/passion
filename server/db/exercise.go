package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Exercise struct {
	ID    string  `db:"id"`
	Owner *string `db:"owner"`
	Slug  *string `db:"slug"`

	Name   string   `db:"name"`
	Kind   string   `db:"kind"`
	Notes  *string  `db:"notes"`
	Source *string  `db:"source"`
	Tags   []string `db:"tags"`

	Sets            *int `db:"sets"`
	Reps            *int `db:"reps"`
	SetRestSeconds  *int `db:"set_rest_seconds"`
	RepSeconds      *int `db:"rep_seconds"`
	RepRestSeconds  *int `db:"rep_rest_seconds"`
	PrepSeconds     *int `db:"prep_seconds"`
	DurationSeconds *int `db:"duration_seconds"`

	VideoURL     *string `db:"video_url"`
	ThumbnailURL *string `db:"thumbnail_url"`

	RetiredAt *time.Time `db:"retired_at"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
}

// ExerciseFields is what a person sets on an exercise of their own.
type ExerciseFields struct {
	Name   string
	Kind   string
	Notes  *string
	Source *string
	Tags   []string

	Sets            *int
	Reps            *int
	SetRestSeconds  *int
	RepSeconds      *int
	RepRestSeconds  *int
	PrepSeconds     *int
	DurationSeconds *int

	VideoURL     *string
	ThumbnailURL *string
}

// ErrNoExercise covers an exercise that does not exist and one this person
// cannot see or change.
var ErrNoExercise = errors.New("no such exercise")

const invalidTextRepresentation = "22P02"

func (f ExerciseFields) args() pgx.NamedArgs {
	tags := f.Tags
	if tags == nil {
		tags = []string{}
	}
	return pgx.NamedArgs{
		"name":             f.Name,
		"kind":             f.Kind,
		"notes":            f.Notes,
		"source":           f.Source,
		"tags":             tags,
		"sets":             f.Sets,
		"reps":             f.Reps,
		"set_rest_seconds": f.SetRestSeconds,
		"rep_seconds":      f.RepSeconds,
		"rep_rest_seconds": f.RepRestSeconds,
		"prep_seconds":     f.PrepSeconds,
		"duration_seconds": f.DurationSeconds,
		"video_url":        f.VideoURL,
		"thumbnail_url":    f.ThumbnailURL,
	}
}

func CreateExercise(ctx context.Context, pool *pgxpool.Pool, owner string, f ExerciseFields) (Exercise, error) {
	args := f.args()
	args["owner"] = owner

	rows, err := pool.Query(ctx, `
		INSERT INTO exercise (
			owner, name, kind, notes, source, tags,
			sets, reps, set_rest_seconds, rep_seconds, rep_rest_seconds, prep_seconds,
			duration_seconds, video_url, thumbnail_url)
		VALUES (
			@owner, @name, @kind, @notes, @source, @tags,
			@sets, @reps, @set_rest_seconds, @rep_seconds, @rep_rest_seconds, @prep_seconds,
			@duration_seconds, @video_url, @thumbnail_url)
		RETURNING *`, args)
	if err != nil {
		return Exercise{}, fmt.Errorf("insert exercise: %w", err)
	}
	return oneExercise(rows)
}

// ListExercises is the person's library: what the app ships and what they
// wrote, without anything retired.
func ListExercises(ctx context.Context, pool *pgxpool.Pool, owner string) ([]Exercise, error) {
	rows, err := pool.Query(ctx, `
		SELECT * FROM exercise
		WHERE (owner IS NULL OR owner = $1) AND retired_at IS NULL
		ORDER BY lower(name), id`, owner)
	if err != nil {
		return nil, fmt.Errorf("select exercises: %w", err)
	}

	list, err := pgx.CollectRows(rows, pgx.RowToStructByName[Exercise])
	if err != nil {
		return nil, fmt.Errorf("read exercises: %w", err)
	}
	return list, nil
}

// GetExercise finds a shipped exercise or one of the person's own. A retired
// one is still found, because it keeps working wherever it is already used.
func GetExercise(ctx context.Context, pool *pgxpool.Pool, owner, id string) (Exercise, error) {
	rows, err := pool.Query(ctx, `
		SELECT * FROM exercise
		WHERE (owner IS NULL OR owner = $1) AND id = $2`, owner, id)
	if err != nil {
		return Exercise{}, fmt.Errorf("select exercise: %w", err)
	}
	return oneExercise(rows)
}

// UpdateExercise changes one of the person's own exercises. A shipped one
// cannot be changed.
func UpdateExercise(ctx context.Context, pool *pgxpool.Pool, owner, id string, f ExerciseFields) (Exercise, error) {
	args := f.args()
	args["owner"] = owner
	args["id"] = id

	rows, err := pool.Query(ctx, `
		UPDATE exercise SET
			name = @name, kind = @kind, notes = @notes, source = @source, tags = @tags,
			sets = @sets, reps = @reps, set_rest_seconds = @set_rest_seconds,
			rep_seconds = @rep_seconds, rep_rest_seconds = @rep_rest_seconds,
			prep_seconds = @prep_seconds, duration_seconds = @duration_seconds,
			video_url = @video_url, thumbnail_url = @thumbnail_url
		WHERE id = @id AND owner = @owner
		RETURNING *`, args)
	if err != nil {
		return Exercise{}, fmt.Errorf("update exercise: %w", err)
	}
	return oneExercise(rows)
}

// RetireExercise takes one of the person's own exercises out of their library.
// Retiring it twice keeps the first date.
func RetireExercise(ctx context.Context, pool *pgxpool.Pool, owner, id string) error {
	tag, err := pool.Exec(ctx, `
		UPDATE exercise SET retired_at = coalesce(retired_at, now())
		WHERE owner = $1 AND id = $2`, owner, id)
	if isInvalidText(err) {
		return ErrNoExercise
	}
	if err != nil {
		return fmt.Errorf("retire exercise: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoExercise
	}
	return nil
}

func oneExercise(rows pgx.Rows) (Exercise, error) {
	exercise, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[Exercise])
	if errors.Is(err, pgx.ErrNoRows) || isInvalidText(err) {
		return Exercise{}, ErrNoExercise
	}
	if err != nil {
		return Exercise{}, fmt.Errorf("read exercise: %w", err)
	}
	return exercise, nil
}

// isInvalidText is true when an id is not a uuid, so it cannot name any row.
func isInvalidText(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == invalidTextRepresentation
}
