package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

// Migrations are embedded here rather than in main.go, because embed patterns cannot
// contain ".." and these live under this package.
//
//go:embed migrations
var migrationsFS embed.FS

// One directory per dialect, one numbering across both. goose's dialect is a process-wide
// setting and a .sql file has no per-dialect branch, but goose.Up takes a directory — so
// two directories is the mechanism. Both are generated from docs/SCHEMA_V2.sql, and
// MigrationVersions exists so a test can assert they have not drifted apart.
func migrationDir(engine string) (string, error) {
	switch engine {
	case EngineSQLite, EnginePostgres:
		return "migrations/" + engine, nil
	default:
		return "", fmt.Errorf("store: no migrations for engine %q", engine)
	}
}

// Migrate applies every pending migration. It is called at boot before the listener opens,
// so a self-hoster runs one binary and never has to remember a second command.
//
// There is no Down. Migrations are append-only after the first boot: SQLite can neither
// drop a CHECK nor alter a primary key, so a down migration could not honestly reverse the
// initial schema. Rolling back means restoring a backup.
func (s *Store) Migrate(ctx context.Context) error {
	dir, err := migrationDir(s.engine)
	if err != nil {
		return err
	}
	db, err := s.sqlDB()
	if err != nil {
		return err
	}

	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect(s.engine); err != nil {
		return fmt.Errorf("store: goose dialect %s: %w", s.engine, err)
	}

	have, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		return fmt.Errorf("store: reading schema version: %w", err)
	}
	want, err := latestVersion(dir)
	if err != nil {
		return err
	}

	// A database newer than the binary is a rolled-back deploy meeting a migrated
	// database. Running on would write rows this code cannot read back, so refuse and name
	// both versions.
	if have > want {
		return fmt.Errorf(
			"store: database schema is version %d but this binary only knows %d — "+
				"this binary is older than the database it was pointed at", have, want)
	}

	if err := goose.UpContext(ctx, db, dir); err != nil {
		return fmt.Errorf("store: migrate %s: %w", s.engine, err)
	}
	return nil
}

// SchemaVersion reports the applied migration version, for /healthz and -migrate-status.
func (s *Store) SchemaVersion(ctx context.Context) (int64, error) {
	db, err := s.sqlDB()
	if err != nil {
		return 0, err
	}
	if err := goose.SetDialect(s.engine); err != nil {
		return 0, err
	}
	return goose.GetDBVersionContext(ctx, db)
}

// MigrationVersions lists the version numbers embedded for one engine. A test asserts both
// engines carry the same list, so a migration added to one dialect and forgotten in the
// other fails the suite instead of failing a deploy.
func MigrationVersions(engine string) ([]string, error) {
	dir, err := migrationDir(engine)
	if err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(migrationsFS, dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

func latestVersion(dir string) (int64, error) {
	migrations, err := goose.CollectMigrations(dir, 0, goose.MaxVersion)
	if err != nil {
		return 0, fmt.Errorf("store: collecting migrations from %s: %w", dir, err)
	}
	if len(migrations) == 0 {
		return 0, fmt.Errorf("store: no migrations found in %s", dir)
	}
	return migrations[len(migrations)-1].Version, nil
}
