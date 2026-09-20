package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate applies every migration the binary carries. The server calls it on
// the way up, so upgrading is one command.
//
// goose wants a database/sql handle, so this opens its own rather than sharing
// the pool.
func Migrate(ctx context.Context, dsn string) error {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open for migration: %w", err)
	}
	defer sqlDB.Close()

	// The advisory lock lives on one session, so the pool must not hand the
	// unlock to a different connection.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("build the migration lock: %w", err)
	}

	dir, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("read the migrations: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, dir,
		goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("build the migration provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
