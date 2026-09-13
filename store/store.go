package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	sqlitedriver "github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

const (
	EngineSQLite   = "sqlite"
	EnginePostgres = "postgres"
)

var (
	// ErrNotFound is returned instead of "not yours". A 403 confirms the row exists.
	ErrNotFound = gorm.ErrRecordNotFound
	// ErrShipped means a write targeted content the app ships.
	ErrShipped = errors.New("store: shipped content cannot be changed")
)

type Config struct {
	Engine       string
	DSN          string
	MaxOpenConns int
	LogLevel     logger.LogLevel
}

// Store owns the only database handle in the process. The field is unexported and there is
// no accessor, so a handler cannot write a query of its own.
type Store struct {
	db     *gorm.DB
	engine string
}

func Open(ctx context.Context, cfg Config) (*Store, error) {
	var dial gorm.Dialector
	switch cfg.Engine {
	case EngineSQLite:
		dial = sqlitedriver.Open(sqliteDSN(cfg.DSN))
	case EnginePostgres:
		dial = postgres.Open(cfg.DSN)
	default:
		return nil, fmt.Errorf("store: unknown engine %q, want %q or %q",
			cfg.Engine, EngineSQLite, EnginePostgres)
	}

	lvl := cfg.LogLevel
	if lvl == 0 {
		lvl = logger.Warn
	}

	db, err := gorm.Open(dial, &gorm.Config{
		// GORM pluralizes table names by default, which would look for "contents" and
		// "accounts" while the migrations create "content" and "account".
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
		// Turns a driver-specific duplicate-key string into gorm.ErrDuplicatedKey, which
		// the error-to-status mapping depends on.
		TranslateError: true,
		// "record not found" is a normal answer here: every upsert asks whether a row exists
		// before it writes. Left on, a first boot prints one per row.
		Logger: logger.New(log.New(os.Stderr, "\r\n", log.LstdFlags), logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  lvl,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", cfg.Engine, err)
	}

	pool, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("store: pool: %w", err)
	}
	switch cfg.Engine {
	case EngineSQLite:
		// SQLite takes one writer at a time. WAL plus a busy timeout makes one enough.
		pool.SetMaxOpenConns(1)
	default:
		if cfg.MaxOpenConns > 0 {
			pool.SetMaxOpenConns(cfg.MaxOpenConns)
		}
	}

	s := &Store{db: db.WithContext(ctx), engine: cfg.Engine}
	if err := s.verifyForeignKeys(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Engine() string { return s.engine }

func (s *Store) Close() error {
	pool, err := s.db.DB()
	if err != nil {
		return err
	}
	return pool.Close()
}

// sqliteDSN appends the pragmas this schema depends on. Never taken from configuration: an
// unrecognised parameter is accepted silently, so a typo would leave foreign keys off.
//
// These are the pure-Go (modernc) spellings. The cgo driver wants _foreign_keys=on and
// _busy_timeout=5000 instead, and verifyForeignKeys catches the mix-up.
func sqliteDSN(path string) string {
	if strings.HasPrefix(path, "file:") {
		path = strings.TrimPrefix(path, "file:")
	}
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}

	q := url.Values{}
	// Off by default in SQLite, and per connection, which is why it belongs in the DSN.
	q.Add("_pragma", "foreign_keys(1)")
	// A reader no longer blocks a writer.
	q.Add("_pragma", "journal_mode(WAL)")
	// Wait rather than return SQLITE_BUSY the instant another write is in flight.
	q.Add("_pragma", "busy_timeout(5000)")
	// SQLite skips the busy handler when a deferred transaction upgrades from read to write,
	// so the busy timeout does not cover that case. Take the write lock up front instead.
	q.Add("_txlock", "immediate")

	return "file:" + path + "?" + q.Encode()
}

// verifyForeignKeys proves the pragma took. A silently-ignored DSN parameter would leave
// every cascade and kind check inert while the test suite stayed green.
func (s *Store) verifyForeignKeys(ctx context.Context) error {
	if s.engine != EngineSQLite {
		return nil
	}
	var on int
	if err := s.db.WithContext(ctx).Raw("PRAGMA foreign_keys").Scan(&on).Error; err != nil {
		return fmt.Errorf("store: reading foreign_keys pragma: %w", err)
	}
	if on != 1 {
		return errors.New("store: foreign keys are off — the DSN pragma was not applied, " +
			"which leaves every cascade and kind check in the schema inert")
	}
	return nil
}

// WithTx runs fn inside one transaction.
//
// GORM decides whether it is inside a transaction from the handle, not globally, so a
// helper that reaches for s.db while an outer transaction is open silently starts a second
// one. The convention here: exported methods open the transaction, and every helper is a
// package-level function taking tx as its first argument, with no way to reach s.db.
func (s *Store) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return s.db.WithContext(ctx).Transaction(fn)
}

// read hands out the handle for a single read. Kept unexported so the only way out of this
// package is a method on Store.
func (s *Store) read(ctx context.Context) *gorm.DB { return s.db.WithContext(ctx) }

// sqlDB is for goose, which works on database/sql rather than GORM.
func (s *Store) sqlDB() (*sql.DB, error) { return s.db.DB() }
