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
	// InsecureCookies drops the Secure flag from the auth cookie, so a browser will keep
	// it over plain HTTP. Without this, running the app locally on http:// means the login
	// succeeds, the browser discards the cookie, and the visitor lands back on the login
	// page with no session — Safari in particular refuses a Secure cookie on localhost.
	//
	// Defaults to false. Never set it anywhere the site is reachable by anyone else: it
	// lets the session cookie travel in the clear.
	InsecureCookies bool `yaml:"InsecureCookies"`
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

// minJWTSecretLen is the shortest secret accepted for a real deployment. HS256 keys
// shorter than the 256-bit hash they feed add nothing but a false sense of length.
const minJWTSecretLen = 32

// placeholderJWTSecrets are the values that must never sign a live token. The first is
// the example file's value. The second is the deploy template's placeholder — if the CI
// substitution ever fails quietly, the app would otherwise boot and sign tokens with the
// literal string, which is public in this repository.
var placeholderJWTSecrets = []string{
	"change-me-in-production",
	"__JWT_SECRET__",
}

// validateJWTSecret rejects secrets that are public knowledge or too short to be worth
// signing with. Dev auth bypass skips the check: it short-circuits token verification
// entirely, so the secret is not load-bearing and local dev needs no ceremony.
func validateJWTSecret(secret string, devAuthBypass bool) error {
	if devAuthBypass {
		return nil
	}
	secret = strings.TrimSpace(secret)
	for _, bad := range placeholderJWTSecrets {
		if secret == bad {
			return fmt.Errorf(
				"auth.jwt_secret is still the placeholder %q — set a real secret, "+
					"for example with: openssl rand -base64 48", bad)
		}
	}
	if len(secret) < minJWTSecretLen {
		return fmt.Errorf(
			"auth.jwt_secret must be at least %d characters, got %d — "+
				"generate one with: openssl rand -base64 48", minJWTSecretLen, len(secret))
	}
	return nil
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
	if err := validateJWTSecret(c.Auth.JWTSecret, c.Auth.DevAuthBypass); err != nil {
		return err
	}
	if c.Auth.JWTTTLHours <= 0 {
		return fmt.Errorf("auth.jwt_ttl_hours must be a positive integer, got %d", c.Auth.JWTTTLHours)
	}
	// Owner id 0 is the sentinel for "no owner" throughout the data layer, and
	// EnsureSeedUser force-sets the primary key from this value. A zero here creates a
	// real, log-in-able account at id 0 that owns rows nothing else can be scoped against.
	if c.Server.DemoOwnerID == 0 {
		return fmt.Errorf("server.demo_owner_id must not be 0")
	}
	if c.YAMLImport.Enabled {
		// ImportYAML refuses owner 0 at the call site. Catching it here names the
		// setting that is wrong instead of failing later with an opaque import error.
		if c.YAMLImport.OwnerID == 0 {
			return fmt.Errorf("yaml_import.owner_id must not be 0 when import is enabled")
		}
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
	if _, ok := os.LookupEnv("PASSION_INSECURE_COOKIES"); ok {
		c.Auth.InsecureCookies = parseTruthy(os.Getenv("PASSION_INSECURE_COOKIES"))
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
