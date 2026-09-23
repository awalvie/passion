package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FileExercise is one catalog file, as the loader writes it.
type FileExercise struct {
	FileID string
	Slug   string
	Hash   string
	Fields ExerciseFields
}

// LoadResult counts what one load wrote.
type LoadResult struct {
	Written int
	Retired int
}

// LoadExercises makes one owner's rows match their catalog files, in one
// transaction. owner is nil for the catalog the app ships. A file whose hash
// matches its row's loaded_hash is skipped, so a start with no edited files
// writes nothing, and a row edited or retired in the app keeps that until its
// file changes. Rows made in the app have no file_id and are never touched.
// No two files may share a file id or a slug, which catalog.Read makes sure of.
func LoadExercises(ctx context.Context, pool *pgxpool.Pool, owner *string, files []FileExercise) (LoadResult, error) {
	// The file id indexes are partial, so each statement names the owner the
	// way one of them does.
	scope := "owner = @owner"
	conflict := "(owner, file_id) WHERE owner IS NOT NULL AND file_id IS NOT NULL"
	if owner == nil {
		scope = "owner IS NULL"
		conflict = "(file_id) WHERE owner IS NULL AND file_id IS NOT NULL"
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return LoadResult{}, fmt.Errorf("begin load: %w", err)
	}
	defer tx.Rollback(ctx)

	// Two servers starting at once would otherwise load the same rows together.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('passion catalog load'))`); err != nil {
		return LoadResult{}, fmt.Errorf("lock load: %w", err)
	}

	rows, err := tx.Query(ctx, `SELECT file_id, loaded_hash FROM exercise WHERE `+scope+` AND file_id IS NOT NULL`,
		pgx.NamedArgs{"owner": owner})
	if err != nil {
		return LoadResult{}, fmt.Errorf("select loaded exercises: %w", err)
	}
	loaded := map[string]*string{}
	var fileID string
	var hash *string
	if _, err := pgx.ForEachRow(rows, []any{&fileID, &hash}, func() error {
		loaded[fileID] = hash
		return nil
	}); err != nil {
		return LoadResult{}, fmt.Errorf("read loaded exercises: %w", err)
	}

	var changed []FileExercise
	var touched []string
	seen := map[string]bool{}
	for _, f := range files {
		seen[f.FileID] = true
		if h, ok := loaded[f.FileID]; ok && h != nil && *h == f.Hash {
			continue
		}
		changed = append(changed, f)
		if _, ok := loaded[f.FileID]; ok {
			touched = append(touched, f.FileID)
		}
	}
	var gone []string
	for id := range loaded {
		if !seen[id] {
			gone = append(gone, id)
		}
	}

	// A gone row gives up its slug, so a new file can take the name, and its
	// hash, so the file counts as changed if it comes back.
	tag, err := tx.Exec(ctx, `
		UPDATE exercise SET retired_at = coalesce(retired_at, now()), slug = NULL, loaded_hash = NULL
		WHERE `+scope+` AND file_id = ANY(@gone) AND loaded_hash IS NOT NULL`,
		pgx.NamedArgs{"owner": owner, "gone": gone})
	if err != nil {
		return LoadResult{}, fmt.Errorf("retire gone exercises: %w", err)
	}
	result := LoadResult{Retired: int(tag.RowsAffected())}

	// The slug indexes check each statement on its own, so two files that
	// swap names would clash half way. Clearing first lets any order work.
	if _, err := tx.Exec(ctx, `UPDATE exercise SET slug = NULL WHERE `+scope+` AND file_id = ANY(@touched)`,
		pgx.NamedArgs{"owner": owner, "touched": touched}); err != nil {
		return LoadResult{}, fmt.Errorf("clear slugs: %w", err)
	}

	upsert := `
		INSERT INTO exercise (
			owner, file_id, slug, loaded_hash, name, kind, notes, source, tags,
			sets, reps, set_rest_seconds, rep_seconds, rep_rest_seconds, prep_seconds,
			duration_seconds, media)
		VALUES (
			@owner, @file_id, @slug, @loaded_hash, @name, @kind, @notes, @source, @tags,
			@sets, @reps, @set_rest_seconds, @rep_seconds, @rep_rest_seconds, @prep_seconds,
			@duration_seconds, @media)
		ON CONFLICT ` + conflict + ` DO UPDATE SET
			slug = excluded.slug, loaded_hash = excluded.loaded_hash,
			name = excluded.name, kind = excluded.kind, notes = excluded.notes,
			source = excluded.source, tags = excluded.tags, sets = excluded.sets,
			reps = excluded.reps, set_rest_seconds = excluded.set_rest_seconds,
			rep_seconds = excluded.rep_seconds, rep_rest_seconds = excluded.rep_rest_seconds,
			prep_seconds = excluded.prep_seconds, duration_seconds = excluded.duration_seconds,
			media = excluded.media, retired_at = NULL`
	for _, f := range changed {
		args := f.Fields.args()
		args["owner"] = owner
		args["file_id"] = f.FileID
		args["slug"] = f.Slug
		args["loaded_hash"] = f.Hash
		if _, err := tx.Exec(ctx, upsert, args); err != nil {
			return LoadResult{}, fmt.Errorf("write %s: %w", f.Slug, err)
		}
	}
	result.Written = len(changed)

	if err := tx.Commit(ctx); err != nil {
		return LoadResult{}, fmt.Errorf("commit load: %w", err)
	}
	return result, nil
}
