package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// App is the V2 configuration. It lives beside the old Config while both stacks exist;
// phase 12 deletes the old one and this takes the name.
//
// Every setting is either kept from V1 with its meaning intact, or new because V2 supports
// two engines. Three V1 settings are gone and their absence is deliberate: db_path became
// database.dsn, demo_owner_id had no meaning once the catalog stopped having an owner, and
// yaml_import.owner_id likewise — shipped content is the rows with no author.
type App struct {
	Server   Server   `yaml:"server"`
	Database Database `yaml:"database"`
	Auth     AuthCfg  `yaml:"auth"`
	Catalog  Catalog  `yaml:"catalog"`
	Log      LogCfg   `yaml:"log"`
	Defaults Defaults `yaml:"defaults"`

	// Migrate is "up" or "off". Up applies every pending migration before the listener
	// opens, so a self-hoster runs one binary and never has to remember a second command.
	Migrate string `yaml:"migrate"`
}

type Server struct {
	Addr string `yaml:"addr"`
}

type Database struct {
	// Engine is "sqlite" or "postgres". Anything else refuses to boot rather than falling
	// back to a default, because falling back would silently use the wrong database.
	Engine string `yaml:"engine"`

	// DSN is a file path for SQLite or a URL for Postgres. The SQLite pragmas are appended
	// by the store and are never taken from here: the two drivers spell them differently
	// and an unrecognised parameter is accepted silently.
	DSN string `yaml:"dsn"`

	// MaxOpenConns is ignored on SQLite, which is pinned to one connection.
	MaxOpenConns int `yaml:"max_open_conns"`
}

type AuthCfg struct {
	JWTSecret       string `yaml:"jwt_secret"`
	JWTTTLHours     int    `yaml:"jwt_ttl_hours"`
	DevAuthBypass   bool   `yaml:"dev_auth_bypass"`
	InsecureCookies bool   `yaml:"insecure_cookies"`
}

type Catalog struct {
	// Import defaults to true. The importer is idempotent and writes only rows with no
	// author, so there is nothing to protect against by leaving it off.
	Import bool `yaml:"import"`

	// Dirs are extra on-disk trees merged before the embedded catalog, with refs resolving
	// across all of them. This is how the private catalog loads: it is a separate
	// repository and cannot be embedded from this one.
	Dirs []string `yaml:"dirs"`
}

type LogCfg struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type Defaults struct {
	// TimeZone is what a new account gets. "Today" has to be the athlete's day, not the
	// server's, or every heatmap and streak boundary is wrong for anyone not beside it.
	TimeZone string `yaml:"time_zone"`
}

func DefaultApp() App {
	return App{
		Server:   Server{Addr: ":3000"},
		Database: Database{Engine: "sqlite", DSN: "passion.db"},
		Auth:     AuthCfg{JWTTTLHours: 168},
		Catalog:  Catalog{Import: true},
		Log:      LogCfg{Level: "info", Format: "text"},
		Defaults: Defaults{TimeZone: "UTC"},
		Migrate:  "up",
	}
}

func (a App) TTL() time.Duration { return time.Duration(a.Auth.JWTTTLHours) * time.Hour }

// LoadApp reads the YAML file when path is non-empty, then applies environment overrides.
// Env always wins, so a deploy can override a checked-in file.
func LoadApp(path string) (App, error) {
	cfg := DefaultApp()

	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return cfg, fmt.Errorf("reading %s: %w", path, err)
		}
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return cfg, fmt.Errorf("parsing %s: %w", path, err)
		}
	}

	applyAppEnv(&cfg)
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func applyAppEnv(c *App) {
	str := func(key string, dst *string) {
		if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
			*dst = v
		}
	}
	num := func(key string, dst *int) {
		if v, ok := os.LookupEnv(key); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				*dst = n
			}
		}
	}
	flag := func(key string, dst *bool) {
		if v, ok := os.LookupEnv(key); ok {
			*dst = truthy(v)
		}
	}

	str("PASSION_ADDR", &c.Server.Addr)
	str("PASSION_DB_ENGINE", &c.Database.Engine)
	str("PASSION_DB_DSN", &c.Database.DSN)
	num("PASSION_DB_MAX_OPEN_CONNS", &c.Database.MaxOpenConns)
	str("PASSION_JWT_SECRET", &c.Auth.JWTSecret)
	num("PASSION_JWT_TTL_HOURS", &c.Auth.JWTTTLHours)
	flag("PASSION_DEV_AUTH_BYPASS", &c.Auth.DevAuthBypass)
	flag("PASSION_INSECURE_COOKIES", &c.Auth.InsecureCookies)
	flag("PASSION_CATALOG_IMPORT", &c.Catalog.Import)
	str("PASSION_LOG_LEVEL", &c.Log.Level)
	str("PASSION_LOG_FORMAT", &c.Log.Format)
	str("PASSION_TIME_ZONE", &c.Defaults.TimeZone)
	str("PASSION_MIGRATE", &c.Migrate)
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func (a App) Validate() error {
	if strings.TrimSpace(a.Server.Addr) == "" {
		return fmt.Errorf("server.addr must not be empty")
	}

	switch a.Database.Engine {
	case "sqlite", "postgres":
	default:
		return fmt.Errorf("database.engine is %q, want \"sqlite\" or \"postgres\"", a.Database.Engine)
	}
	if strings.TrimSpace(a.Database.DSN) == "" {
		return fmt.Errorf("database.dsn must not be empty")
	}

	// Dev bypass short-circuits token verification, so it must never reach a Postgres
	// deployment — that is the shape a hosted instance takes.
	if a.Auth.DevAuthBypass && a.Database.Engine == "postgres" {
		return fmt.Errorf("auth.dev_auth_bypass must not be set with database.engine=postgres")
	}

	if strings.TrimSpace(a.Auth.JWTSecret) == "" && !a.Auth.DevAuthBypass {
		return fmt.Errorf("auth.jwt_secret must not be empty")
	}
	if err := validateJWTSecret(a.Auth.JWTSecret, a.Auth.DevAuthBypass); err != nil {
		return err
	}
	if a.Auth.JWTTTLHours <= 0 {
		return fmt.Errorf("auth.jwt_ttl_hours must be a positive integer, got %d", a.Auth.JWTTTLHours)
	}

	switch a.Migrate {
	case "up", "off":
	default:
		return fmt.Errorf("migrate is %q, want \"up\" or \"off\"", a.Migrate)
	}

	if _, err := time.LoadLocation(a.Defaults.TimeZone); err != nil {
		return fmt.Errorf("defaults.time_zone %q is not a known zone: %w", a.Defaults.TimeZone, err)
	}
	return nil
}
