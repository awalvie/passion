package catalog

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"slices"

	"github.com/jackc/pgx/v5/pgxpool"

	"passion/server/config"
	"passion/server/db"
)

// LoadAll loads the shipped catalog, then every private one for each of its
// owners. All of one owner's trees load together, because they share one set
// of rows. An owner with no account yet is skipped, and loads on the first
// start after they sign up.
func LoadAll(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger, shipped fs.FS, private []config.PrivateCatalog) error {
	result, warnings, err := Load(ctx, pool, nil, Tree{Name: "catalog", FS: shipped})
	if err != nil {
		return fmt.Errorf("the shipped catalog: %w", err)
	}
	logLoad(log, "catalog", result, warnings)

	var owners []string
	trees := map[string][]Tree{}
	for _, p := range private {
		for _, owner := range p.Owner {
			email := db.NormaliseEmail(owner)
			if _, ok := trees[email]; !ok {
				owners = append(owners, email)
			}
			if !slices.ContainsFunc(trees[email], func(t Tree) bool { return t.Name == p.Location }) {
				trees[email] = append(trees[email], Tree{Name: p.Location, FS: os.DirFS(p.Location)})
			}
		}
	}

	// Anyone who had rows loaded and is no longer in the config gets an empty
	// load, which retires them.
	stale, err := db.OwnersWithLoadedRows(ctx, pool)
	if err != nil {
		return err
	}
	configured := map[string]bool{}

	for _, email := range owners {
		account, err := db.AccountByEmail(ctx, pool, email)
		if errors.Is(err, db.ErrNoAccount) {
			log.Warn("skipped a private catalog: no account yet", "owner", email)
			continue
		}
		if err != nil {
			return err
		}
		configured[account.ID] = true
		result, warnings, err := Load(ctx, pool, &account.ID, trees[email]...)
		if err != nil {
			return fmt.Errorf("the private catalog for %s: %w", email, err)
		}
		logLoad(log, email, result, warnings)
	}

	for _, id := range stale {
		if configured[id] {
			continue
		}
		result, _, err := Load(ctx, pool, &id)
		if err != nil {
			return fmt.Errorf("retiring the catalog of account %s: %w", id, err)
		}
		log.Info("retired a private catalog that left the config", "account", id, "retired", result.Retired)
	}
	return nil
}

func logLoad(log *slog.Logger, owner string, result db.LoadResult, warnings []string) {
	log.Info("loaded a catalog", "owner", owner, "written", result.Written, "retired", result.Retired)
	if len(warnings) > 0 {
		log.Warn("some files use keys that are not stored yet. Set log.level to debug to list them", "owner", owner, "files", len(warnings))
	}
	for _, w := range warnings {
		log.Debug(w)
	}
}

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
