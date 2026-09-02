// Package config loads server settings from an optional YAML file and environment variables.
// Env vars override file values (12-factor friendly).
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds HTTP server and database settings.
type ServerConfig struct {
	// Addr is the listen address, e.g. ":3000", "127.0.0.1:3000", "0.0.0.0:8080".
	Addr string `yaml:"Addr"`
	// DBPath is the SQLite database file path.
	DBPath string `yaml:"DBPath"`
	// Seed, when true, runs dev seed data if the database has no templates (see db.SeedDevIfEmpty).
	Seed bool `yaml:"Seed"`
	// DemoOwnerID is the user ID used for demo seeding.
	DemoOwnerID uint `yaml:"DemoOwnerID"`
}

// AuthConfig holds JWT authentication settings.
type AuthConfig struct {
	// JWTSecret signs authentication tokens.
	JWTSecret string `yaml:"JWTSecret"`
	// JWTTTLHours controls token validity window in hours.
	JWTTTLHours int `yaml:"JWTTTLHours"`
	// DevAuthBypass auto-authenticates as demo user in development.
	DevAuthBypass bool `yaml:"DevAuthBypass"`
}

// DirList is one or more catalog directories. It accepts either a single scalar or a
// sequence in YAML, so config files written before the catalog was split keep working.
type DirList []string

// UnmarshalYAML accepts `Dir: a` and `Dir: [a, b]` alike.
func (d *DirList) UnmarshalYAML(value *yaml.Node) error {
	var one string
	if err := value.Decode(&one); err == nil {
		*d = cleanDirs([]string{one})
		return nil
	}
	var many []string
	if err := value.Decode(&many); err != nil {
		return fmt.Errorf("catalog directory must be a string or a list of strings: %w", err)
	}
	*d = cleanDirs(many)
	return nil
}

// cleanDirs trims each entry and drops the empty ones, so a trailing comma in an
// environment variable is not read as a directory named "".
func cleanDirs(in []string) DirList {
	out := make(DirList, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// parseDirList splits a comma-separated environment variable into directories.
func parseDirList(s string) DirList {
	return cleanDirs(strings.Split(s, ","))
}

// YAMLImportConfig holds startup YAML import settings.
type YAMLImportConfig struct {
	// Enabled loads exercise/template YAML content on startup.
	Enabled bool `yaml:"Enabled"`
	// ExercisesDir contains YAML files for library exercises. Several directories may
	// be listed — the published catalog plus a private one, say — and refs resolve
	// across all of them.
	ExercisesDir DirList `yaml:"ExercisesDir"`
	// SessionTemplatesDir contains YAML files for session templates.
	SessionTemplatesDir DirList `yaml:"SessionTemplatesDir"`
	// ActivityTemplatesDir contains YAML files for activity templates.
	// Optional — if empty, activity template import is skipped.
	ActivityTemplatesDir DirList `yaml:"ActivityTemplatesDir"`
	// OwnerID is the owner receiving imported records.
	OwnerID uint `yaml:"OwnerID"`
}

// Config holds Passion process settings.
type Config struct {
	Server     ServerConfig     `yaml:"Server"`
	Auth       AuthConfig       `yaml:"Auth"`
	YAMLImport YAMLImportConfig `yaml:"YAMLImport"`
}

// defaultConfig returns built-in defaults when no file is loaded.
func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Addr:        ":3000",
			DBPath:      "passion.db",
			Seed:        false,
			DemoOwnerID: 1,
		},
		Auth: AuthConfig{
			JWTSecret:     "change-me-in-production",
			JWTTTLHours:   24 * 30,
			DevAuthBypass: false,
		},
		YAMLImport: YAMLImportConfig{
			Enabled:              false,
			ExercisesDir:         DirList{"catalog/exercises"},
			SessionTemplatesDir:  DirList{"catalog/session_templates"},
			ActivityTemplatesDir: DirList{"catalog/activity_templates"},
			OwnerID:              1,
		},
	}
}

// Validate checks that required fields are present and well-formed.
// Call after Load. Returns a non-nil error on the first problem found.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Server.Addr) == "" {
		return fmt.Errorf("server.addr must not be empty")
	}
	if strings.TrimSpace(c.Server.DBPath) == "" {
		return fmt.Errorf("server.db_path must not be empty")
	}
	if strings.TrimSpace(c.Auth.JWTSecret) == "" {
		return fmt.Errorf("auth.jwt_secret must not be empty")
	}
	if c.Auth.JWTTTLHours <= 0 {
		return fmt.Errorf("auth.jwt_ttl_hours must be a positive integer, got %d", c.Auth.JWTTTLHours)
	}
	if c.YAMLImport.Enabled {
		if len(c.YAMLImport.ExercisesDir) == 0 {
			return fmt.Errorf("yaml_import.exercises_dir must not be empty when import is enabled")
		}
		if len(c.YAMLImport.SessionTemplatesDir) == 0 {
			return fmt.Errorf("yaml_import.session_templates_dir must not be empty when import is enabled")
		}
		all := make([]string, 0, 8)
		all = append(all, c.YAMLImport.ExercisesDir...)
		all = append(all, c.YAMLImport.SessionTemplatesDir...)
		all = append(all, c.YAMLImport.ActivityTemplatesDir...)
		for _, dir := range all {
			if _, err := os.Stat(dir); err != nil {
				return fmt.Errorf("yaml_import directory %q: %w", dir, err)
			}
		}
	}
	return nil
}

// Load reads an optional YAML file, then applies environment overrides.
// If path is empty, only defaults + env are used.
func Load(path string) (*Config, error) {
	c := defaultConfig()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config file %q: %w", path, err)
		}
		if err := yaml.Unmarshal(data, c); err != nil {
			return nil, fmt.Errorf("parse config YAML: %w", err)
		}
	}
	applyEnv(c)
	return c, nil
}

func applyEnv(c *Config) {
	if v := strings.TrimSpace(os.Getenv("PASSION_ADDR")); v != "" {
		c.Server.Addr = v
	}
	if v := strings.TrimSpace(os.Getenv("PASSION_DB_PATH")); v != "" {
		c.Server.DBPath = v
	}
	if _, ok := os.LookupEnv("PASSION_SEED"); ok {
		c.Server.Seed = parseTruthy(os.Getenv("PASSION_SEED"))
	}
	if v := strings.TrimSpace(os.Getenv("PASSION_JWT_SECRET")); v != "" {
		c.Auth.JWTSecret = v
	}
	if v := strings.TrimSpace(os.Getenv("PASSION_JWT_TTL_HOURS")); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			c.Auth.JWTTTLHours = n
		}
	}
	if _, ok := os.LookupEnv("PASSION_DEV_AUTH_BYPASS"); ok {
		c.Auth.DevAuthBypass = parseTruthy(os.Getenv("PASSION_DEV_AUTH_BYPASS"))
	}
	if _, ok := os.LookupEnv("PASSION_YAML_IMPORT_ENABLED"); ok {
		c.YAMLImport.Enabled = parseTruthy(os.Getenv("PASSION_YAML_IMPORT_ENABLED"))
	}
	if v := strings.TrimSpace(os.Getenv("PASSION_YAML_EXERCISES_DIR")); v != "" {
		c.YAMLImport.ExercisesDir = parseDirList(v)
	}
	if v := strings.TrimSpace(os.Getenv("PASSION_YAML_SESSION_TEMPLATES_DIR")); v != "" {
		c.YAMLImport.SessionTemplatesDir = parseDirList(v)
	}
	if v := strings.TrimSpace(os.Getenv("PASSION_YAML_ACTIVITY_TEMPLATES_DIR")); v != "" {
		c.YAMLImport.ActivityTemplatesDir = parseDirList(v)
	}
	if v := strings.TrimSpace(os.Getenv("PASSION_YAML_IMPORT_OWNER_ID")); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			c.YAMLImport.OwnerID = uint(n)
		}
	}
}

func parseTruthy(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "1" || s == "true" || s == "yes" || s == "on"
}

func parsePositiveInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid positive int")
	}
	return n, nil
}
