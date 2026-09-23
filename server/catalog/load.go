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
	app := Tree{Name: "catalog", FS: shipped}
	result, warnings, err := Load(ctx, pool, nil, nil, app)
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
		result, warnings, err := Load(ctx, pool, &account.ID, &app, trees[email]...)
		if err != nil {
			return fmt.Errorf("the private catalog for %s: %w", email, err)
		}
		logLoad(log, email, result, warnings)
	}

	for _, id := range stale {
		if configured[id] {
			continue
		}
		result, _, err := Load(ctx, pool, &id, nil)
		if err != nil {
			return fmt.Errorf("retiring the catalog of account %s: %w", id, err)
		}
		log.Info("retired a private catalog that left the config", "account", id,
			"exercises", result.Exercises.Retired, "sessions", result.Sessions.Retired)
	}
	return nil
}

// Result counts what one load wrote to each table.
type Result struct {
	Exercises db.LoadResult
	Sessions  db.LoadResult
}

func logLoad(log *slog.Logger, owner string, result Result, warnings []string) {
	log.Info("loaded a catalog", "owner", owner,
		"exercises_written", result.Exercises.Written, "exercises_retired", result.Exercises.Retired,
		"sessions_written", result.Sessions.Written, "sessions_retired", result.Sessions.Retired)
	if len(warnings) > 0 {
		log.Warn("some files use keys that are not stored yet. Set log.level to debug to list them", "owner", owner, "files", len(warnings))
	}
	for _, w := range warnings {
		log.Debug(w)
	}
}

// Load reads every tree one owner loads and writes what changed: exercises
// first, then the sessions whose steps hold their ids. owner is nil for the
// catalog the app ships, and app is then nil too. Otherwise app is the
// shipped catalog, which must have loaded first. Every file is read and
// checked before anything is written, so a bad file loads nothing at all.
func Load(ctx context.Context, pool *pgxpool.Pool, owner *string, app *Tree, trees ...Tree) (Result, []string, error) {
	exercises, warnings, err := Read(trees...)
	if err != nil {
		return Result{}, warnings, err
	}
	sessions, sessionWarnings, err := ReadSessions(app, exercises, trees...)
	warnings = append(warnings, sessionWarnings...)
	if err != nil {
		return Result{}, warnings, err
	}

	files := make([]db.FileExercise, 0, len(exercises))
	for _, e := range exercises {
		files = append(files, db.FileExercise{FileID: e.FileID, Slug: e.Slug, Hash: e.Hash, Fields: e.Fields})
	}
	var result Result
	if result.Exercises, err = db.LoadExercises(ctx, pool, owner, files); err != nil {
		return Result{}, warnings, err
	}

	ownIDs, err := db.ExerciseIDsByFileID(ctx, pool, owner)
	if err != nil {
		return Result{}, warnings, err
	}
	appIDs := ownIDs
	if owner != nil {
		if appIDs, err = db.ExerciseIDsByFileID(ctx, pool, nil); err != nil {
			return Result{}, warnings, err
		}
	}

	templates := make([]db.FileSessionTemplate, 0, len(sessions))
	for _, s := range sessions {
		fields, err := withRowIDs(s, ownIDs, appIDs)
		if err != nil {
			return Result{}, warnings, fmt.Errorf("%s: %s: %w", s.Tree, s.Path, err)
		}
		templates = append(templates, db.FileSessionTemplate{FileID: s.FileID, Slug: s.Slug, Hash: Hash(s.Slug, fields), Fields: fields})
	}
	result.Sessions, err = db.LoadSessionTemplates(ctx, pool, owner, templates)
	return result, warnings, err
}

// withRowIDs swaps each step's movement file id for the row the load wrote.
// It builds a new body, because blocks share their sections between sessions.
func withRowIDs(s Session, own, app map[string]string) (db.SessionTemplateFields, error) {
	i := 0
	swap := func(step db.Step) (db.Step, error) {
		ids := own
		if s.AppSteps[i] {
			ids = app
		}
		i++
		id, ok := ids[step.Exercise]
		if !ok {
			return db.Step{}, fmt.Errorf("no loaded exercise for the movement with id %s", step.Exercise)
		}
		step.Exercise = id
		return step, nil
	}

	f := s.Fields
	sections := make([]db.Section, 0, len(f.Body.Sections))
	for _, section := range f.Body.Sections {
		items := make([]db.Item, 0, len(section.Items))
		for _, item := range section.Items {
			switch {
			case item.Step != nil:
				step, err := swap(*item.Step)
				if err != nil {
					return db.SessionTemplateFields{}, err
				}
				items = append(items, db.Item{Step: &step})
			case item.Choice != nil:
				choice := *item.Choice
				choice.Options = make([]db.Step, 0, len(item.Choice.Options))
				for _, option := range item.Choice.Options {
					step, err := swap(option)
					if err != nil {
						return db.SessionTemplateFields{}, err
					}
					choice.Options = append(choice.Options, step)
				}
				items = append(items, db.Item{Choice: &choice})
			}
		}
		section.Items = items
		sections = append(sections, section)
	}
	f.Body = db.SessionBody{Sections: sections}
	return f, nil
}
