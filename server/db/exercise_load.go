package db

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

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

// FileSessionTemplate is one sessions/ file, with its blocks copied in and
// every step's exercise id filled.
type FileSessionTemplate struct {
	FileID string
	Slug   string
	Hash   string
	Fields SessionTemplateFields
}

// OwnersWithLoadedRows lists the accounts that hold rows a catalog load wrote
// and has not retired, so that an owner dropped from the config can have
// theirs retired.
func OwnersWithLoadedRows(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, `
		SELECT owner FROM exercise
		WHERE owner IS NOT NULL AND file_id IS NOT NULL AND loaded_hash IS NOT NULL
		UNION
		SELECT owner FROM session_template
		WHERE owner IS NOT NULL AND file_id IS NOT NULL AND loaded_hash IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("select loaded owners: %w", err)
	}
	owners, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("read loaded owners: %w", err)
	}
	return owners, nil
}

// ExerciseIDsByFileID maps each file id one owner's catalog loaded to its
// row id, retired rows included. owner is nil for the catalog the app ships.
func ExerciseIDsByFileID(ctx context.Context, pool *pgxpool.Pool, owner *string) (map[string]string, error) {
	scope := "owner = @owner"
	if owner == nil {
		scope = "owner IS NULL"
	}
	rows, err := pool.Query(ctx, `SELECT file_id::text, id::text FROM exercise WHERE `+scope+` AND file_id IS NOT NULL`,
		pgx.NamedArgs{"owner": owner})
	if err != nil {
		return nil, fmt.Errorf("select exercise ids: %w", err)
	}
	ids := map[string]string{}
	var fileID, id string
	if _, err := pgx.ForEachRow(rows, []any{&fileID, &id}, func() error {
		ids[fileID] = id
		return nil
	}); err != nil {
		return nil, fmt.Errorf("read exercise ids: %w", err)
	}
	return ids, nil
}

// LoadExercises makes one owner's exercise rows match their catalog files.
func LoadExercises(ctx context.Context, pool *pgxpool.Pool, owner *string, files []FileExercise) (LoadResult, error) {
	rows := make([]fileRow, 0, len(files))
	for _, f := range files {
		rows = append(rows, fileRow{FileID: f.FileID, Slug: f.Slug, Hash: f.Hash, Args: f.Fields.args()})
	}
	return loadFiles(ctx, pool, "exercise", owner, rows)
}

// LoadSessionTemplates makes one owner's session template rows match their
// catalog files. It runs after LoadExercises, whose ids the steps hold.
func LoadSessionTemplates(ctx context.Context, pool *pgxpool.Pool, owner *string, files []FileSessionTemplate) (LoadResult, error) {
	rows := make([]fileRow, 0, len(files))
	for _, f := range files {
		rows = append(rows, fileRow{FileID: f.FileID, Slug: f.Slug, Hash: f.Hash, Args: f.Fields.args()})
	}
	return loadFiles(ctx, pool, "session_template", owner, rows)
}

// fileRow is one catalog file for any table the loader writes. Args holds the
// table's own columns, and every row of one load holds the same keys.
type fileRow struct {
	FileID string
	Slug   string
	Hash   string
	Args   pgx.NamedArgs
}

// loadFiles makes one owner's rows in table match their catalog files, in one
// transaction. owner is nil for the catalog the app ships. A file whose hash
// matches its row's loaded_hash is skipped, so a start with no edited files
// writes nothing, and a row edited or retired in the app keeps that until its
// file changes. Rows made in the app have no file_id and are never touched.
// No two files may share a file id or a slug, which catalog.Read makes sure of.
// table is always a constant, never input.
func loadFiles(ctx context.Context, pool *pgxpool.Pool, table string, owner *string, files []fileRow) (LoadResult, error) {
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

	rows, err := tx.Query(ctx, `SELECT file_id, loaded_hash FROM `+table+` WHERE `+scope+` AND file_id IS NOT NULL`,
		pgx.NamedArgs{"owner": owner})
	if err != nil {
		return LoadResult{}, fmt.Errorf("select loaded %s rows: %w", table, err)
	}
	loaded := map[string]*string{}
	var fileID string
	var hash *string
	if _, err := pgx.ForEachRow(rows, []any{&fileID, &hash}, func() error {
		loaded[fileID] = hash
		return nil
	}); err != nil {
		return LoadResult{}, fmt.Errorf("read loaded %s rows: %w", table, err)
	}

	var changed []fileRow
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
		UPDATE `+table+` SET retired_at = coalesce(retired_at, now()), slug = NULL, loaded_hash = NULL
		WHERE `+scope+` AND file_id = ANY(@gone) AND loaded_hash IS NOT NULL`,
		pgx.NamedArgs{"owner": owner, "gone": gone})
	if err != nil {
		return LoadResult{}, fmt.Errorf("retire gone %s rows: %w", table, err)
	}
	result := LoadResult{Retired: int(tag.RowsAffected())}

	// The slug indexes check each statement on its own, so two files that
	// swap names would clash half way. Clearing first lets any order work.
	if _, err := tx.Exec(ctx, `UPDATE `+table+` SET slug = NULL WHERE `+scope+` AND file_id = ANY(@touched)`,
		pgx.NamedArgs{"owner": owner, "touched": touched}); err != nil {
		return LoadResult{}, fmt.Errorf("clear %s slugs: %w", table, err)
	}

	if len(changed) > 0 {
		columns := slices.Sorted(maps.Keys(changed[0].Args))
		var params, updates []string
		for _, c := range columns {
			params = append(params, "@"+c)
			updates = append(updates, c+" = excluded."+c)
		}
		upsert := `
			INSERT INTO ` + table + ` (owner, file_id, slug, loaded_hash, ` + strings.Join(columns, ", ") + `)
			VALUES (@owner, @file_id, @slug, @loaded_hash, ` + strings.Join(params, ", ") + `)
			ON CONFLICT ` + conflict + ` DO UPDATE SET
				slug = excluded.slug, loaded_hash = excluded.loaded_hash, ` + strings.Join(updates, ", ") + `,
				retired_at = NULL`

		for _, f := range changed {
			args := maps.Clone(f.Args)
			args["owner"] = owner
			args["file_id"] = f.FileID
			args["slug"] = f.Slug
			args["loaded_hash"] = f.Hash
			if _, err := tx.Exec(ctx, upsert, args); err != nil {
				return LoadResult{}, fmt.Errorf("write %s %s: %w", table, f.Slug, err)
			}
		}
	}
	result.Written = len(changed)

	if err := tx.Commit(ctx); err != nil {
		return LoadResult{}, fmt.Errorf("commit load: %w", err)
	}
	return result, nil
}
