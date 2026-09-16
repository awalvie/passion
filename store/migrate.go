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

// One directory per dialect, one numbering across both. A .sql file has no per-dialect
// branch, so two directories is the only seam goose gives. Both are generated from
// docs/SCHEMA_V2.sql.
func migrationDir(engine string) (string, error) {
	switch engine {
	case EngineSQLite, EnginePostgres:
		return "migrations/" + engine, nil
	default:
		return "", fmt.Errorf("store: no migrations for engine %q", engine)
	}
}

// Migrate applies every pending migration, at boot and before the listener opens.
//
// There is no Down. SQLite can neither drop a CHECK nor alter a primary key, so a down
// migration could not honestly reverse this schema. Rolling back means restoring a backup.
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

	// A database newer than the binary is a rolled-back deploy. Running on would write rows
	// this code cannot read back.
	if have > want {
		return fmt.Errorf(
			"store: database schema is version %d but this binary only knows %d — "+
				"this binary is older than the database it was pointed at", have, want)
	}

	if err := goose.UpContext(ctx, db, dir); err != nil {
		return fmt.Errorf("store: migrate %s: %w", s.engine, err)
	}
	return s.verifySchema(ctx)
}

// verifySchema catches a database whose recorded version is current but whose tables are not.
// Migration 001 is still edited in place while nothing has shipped, so a database created
// before the latest edit records "001 applied" and never receives it.
//
// Delete this with the first migration 002.
func (s *Store) verifySchema(ctx context.Context) error {
	// The model, not the table name: with a bare string GORM cannot resolve the column and
	// answers false on PostgreSQL for a column that is there.
	m := s.db.WithContext(ctx).Migrator()
	for _, want := range []struct {
		model  any
		table  string
		column string
	}{
		{&Content{}, "content", "uuid"},
		{&Content{}, "content", "family"},
		{&Log{}, "log", "session_family"},
		{&LogEntry{}, "log_entry", "movement_family"},
	} {
		if !m.HasColumn(want.model, want.column) {
			return fmt.Errorf("store: this database is older than the schema: %s.%s is missing "+
				"while the migration is recorded as applied. Migration 001 is still edited in "+
				"place, so delete the database and boot again (make reseed-v2)",
				want.table, want.column)
		}
	}
	return nil
}

// TargetVersion is the newest migration this binary carries. A package function, not a
// method: -version answers it with no config, no database and no connection.
func TargetVersion(engine string) (int64, error) {
	dir, err := migrationDir(engine)
	if err != nil {
		return 0, err
	}
	goose.SetBaseFS(migrationsFS)
	return latestVersion(dir)
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
// engines carry the same list, so a migration written for one dialect only fails the suite.
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
