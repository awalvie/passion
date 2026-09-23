package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Exercise struct {
	ID     string  `db:"id"`
	Owner  *string `db:"owner"`
	Slug   *string `db:"slug"`
	FileID *string `db:"file_id"`

	LoadedHash *string `db:"loaded_hash"`

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

	Media []Media `db:"media"`

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

	Media []Media
}

// Media is one clip, or an image with no video. The tags name its keys in the
// jsonb column.
type Media struct {
	URL      *string `json:"url"`
	ThumbURL *string `json:"thumb_url"`
}

const maxExerciseName = 200

var exerciseKinds = map[string]bool{
	"reps_and_sets": true,
	"timed_reps":    true,
	"climbing":      true,
	"open":          true,
}

// Clean trims and checks the fields, and names each problem by its column.
// The API and the catalog loader both use it, so the same content always
// comes out the same, which the loader's hash depends on.
func (f ExerciseFields) Clean() (ExerciseFields, map[string]string) {
	problems := map[string]string{}

	f.Name = strings.TrimSpace(f.Name)
	switch {
	case f.Name == "":
		problems["name"] = "is required"
	case utf8.RuneCountInString(f.Name) > maxExerciseName:
		problems["name"] = "is too long"
	}

	if !exerciseKinds[f.Kind] {
		problems["kind"] = "must be reps_and_sets, timed_reps, climbing or open"
	}

	// The table checks none of these, and a column holds at most a 32-bit int.
	for column, n := range map[string]*int{
		"sets":             f.Sets,
		"reps":             f.Reps,
		"set_rest_seconds": f.SetRestSeconds,
		"rep_seconds":      f.RepSeconds,
		"rep_rest_seconds": f.RepRestSeconds,
		"prep_seconds":     f.PrepSeconds,
		"duration_seconds": f.DurationSeconds,
	} {
		switch {
		case n == nil:
		case *n < 0:
			problems[column] = "cannot be negative"
		case *n > math.MaxInt32:
			problems[column] = "is too large"
		}
	}

	f.Notes = optional(f.Notes)
	f.Source = optional(f.Source)
	media := []Media{}
	for i, m := range f.Media {
		m = Media{URL: optional(m.URL), ThumbURL: optional(m.ThumbURL)}
		if m.URL == nil && m.ThumbURL == nil {
			continue
		}
		for _, link := range []*string{m.URL, m.ThumbURL} {
			if link != nil && !isWebLink(*link) {
				problems[fmt.Sprintf("media[%d]", i)] = "must hold only http or https links"
			}
		}
		media = append(media, m)
	}
	f.Media = media

	tags := []string{}
	for _, tag := range f.Tags {
		if tag = strings.TrimSpace(tag); tag != "" {
			tags = append(tags, tag)
		}
	}
	f.Tags = tags

	return f, problems
}

// optional trims a text field, and treats a blank one as not set, so a form
// that sends "" for an empty box stores nothing.
func optional(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// isWebLink keeps javascript: and similar links out of anything the client
// renders as an href.
func isWebLink(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
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
	media := f.Media
	if media == nil {
		media = []Media{}
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
		"media":            media,
	}
}

func CreateExercise(ctx context.Context, pool *pgxpool.Pool, owner string, f ExerciseFields) (Exercise, error) {
	args := f.args()
	args["owner"] = owner

	rows, err := pool.Query(ctx, `
		INSERT INTO exercise (
			owner, name, kind, notes, source, tags,
			sets, reps, set_rest_seconds, rep_seconds, rep_rest_seconds, prep_seconds,
			duration_seconds, media)
		VALUES (
			@owner, @name, @kind, @notes, @source, @tags,
			@sets, @reps, @set_rest_seconds, @rep_seconds, @rep_rest_seconds, @prep_seconds,
			@duration_seconds, @media)
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
			media = @media
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
