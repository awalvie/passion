package catalog

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/db"
)

// Load reads every tree one owner loads and writes what changed. owner is nil
// for the catalog the app ships. A bad file loads nothing at all.
func Load(ctx context.Context, pool *pgxpool.Pool, owner *string, trees ...Tree) (db.LoadResult, []string, error) {
	exercises, warnings, err := Read(trees...)
	if err != nil {
		return db.LoadResult{}, warnings, err
	}

	files := make([]db.FileExercise, 0, len(exercises))
	for _, e := range exercises {
		files = append(files, db.FileExercise{FileID: e.FileID, Slug: e.Slug, Hash: e.Hash, Fields: e.Fields})
	}
	result, err := db.LoadExercises(ctx, pool, owner, files)
	return result, warnings, err
}
