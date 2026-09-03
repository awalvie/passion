package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Addr != ":3000" || c.Server.DBPath != "passion.db" || c.Server.Seed || c.Auth.JWTSecret == "" || c.Auth.JWTTTLHours <= 0 || c.Auth.DevAuthBypass || c.YAMLImport.Enabled {
		t.Fatalf("defaults: %+v", c)
	}
	if firstDir(c.YAMLImport.ExercisesDir) == "" || firstDir(c.YAMLImport.SessionTemplatesDir) == "" || c.YAMLImport.OwnerID == 0 {
		t.Fatalf("yaml defaults: %+v", c)
	}
}

func TestLoadFileAndEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("Server:\n  Addr: \":9999\"\n  DBPath: \"x.db\"\n  Seed: true\nAuth:\n  JWTSecret: \"file-secret\"\n  JWTTTLHours: 48\n  DevAuthBypass: true\nYAMLImport:\n  Enabled: true\n  ExercisesDir: \"fixtures/exercises\"\n  SessionTemplatesDir: \"fixtures/templates\"\n  OwnerID: 7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PASSION_ADDR", ":7777")
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Addr != ":7777" {
		t.Fatalf("addr: want :7777 got %q", c.Server.Addr)
	}
	if c.Server.DBPath != "x.db" {
		t.Fatalf("DBPath: %q", c.Server.DBPath)
	}
	if !c.Server.Seed {
		t.Fatal("seed should stay true from file when PASSION_SEED unset")
	}
	if c.Auth.JWTSecret != "file-secret" {
		t.Fatalf("JWTSecret: %q", c.Auth.JWTSecret)
	}
	if c.Auth.JWTTTLHours != 48 {
		t.Fatalf("JWTTTLHours: %d", c.Auth.JWTTTLHours)
	}
	if !c.Auth.DevAuthBypass {
		t.Fatal("DevAuthBypass should be true from file")
	}
	if !c.YAMLImport.Enabled || firstDir(c.YAMLImport.ExercisesDir) != "fixtures/exercises" || firstDir(c.YAMLImport.SessionTemplatesDir) != "fixtures/templates" || c.YAMLImport.OwnerID != 7 {
		t.Fatalf("yaml import file values failed: %+v", c)
	}
}

func TestEnvSeedOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("Server:\n  Seed: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PASSION_SEED", "0")
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Seed {
		t.Fatal("PASSION_SEED should override")
	}
}

func TestJWTEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("Auth:\n  JWTSecret: \"file-secret\"\n  JWTTTLHours: 24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PASSION_JWT_SECRET", "env-secret")
	t.Setenv("PASSION_JWT_TTL_HOURS", "72")
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Auth.JWTSecret != "env-secret" {
		t.Fatalf("jwt secret override failed: %q", c.Auth.JWTSecret)
	}
	if c.Auth.JWTTTLHours != 72 {
		t.Fatalf("jwt ttl override failed: %d", c.Auth.JWTTTLHours)
	}
}

func TestDevAuthBypassEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("Auth:\n  DevAuthBypass: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PASSION_DEV_AUTH_BYPASS", "1")
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Auth.DevAuthBypass {
		t.Fatal("PASSION_DEV_AUTH_BYPASS should override file value")
	}
}

func TestYAMLImportEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(path, []byte("YAMLImport:\n  Enabled: false\n  ExercisesDir: \"a\"\n  SessionTemplatesDir: \"b\"\n  OwnerID: 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PASSION_YAML_IMPORT_ENABLED", "1")
	t.Setenv("PASSION_YAML_EXERCISES_DIR", "catalog/exercises")
	t.Setenv("PASSION_YAML_SESSION_TEMPLATES_DIR", "catalog/session_templates")
	t.Setenv("PASSION_YAML_IMPORT_OWNER_ID", "42")
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !c.YAMLImport.Enabled {
		t.Fatal("PASSION_YAML_IMPORT_ENABLED should override file value")
	}
	if firstDir(c.YAMLImport.ExercisesDir) != "catalog/exercises" || firstDir(c.YAMLImport.SessionTemplatesDir) != "catalog/session_templates" {
		t.Fatalf("yaml dirs env override failed: %+v", c)
	}
	if c.YAMLImport.OwnerID != 42 {
		t.Fatalf("yaml owner env override failed: %+v", c)
	}
}

// firstDir reads the head of a DirList so the single-directory assertions in this file
// stay readable now that the field holds a list.
func firstDir(d DirList) string {
	if len(d) == 0 {
		return ""
	}
	return d[0]
}

// TestDirListAcceptsScalarAndList guards backward compatibility: config files written
// before the catalog was split give a single directory as a scalar, and must keep
// working now that the field holds a list.
func TestDirListAcceptsScalarAndList(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want []string
	}{
		{"scalar", "YAMLImport:\n  ExercisesDir: catalog/exercises\n", []string{"catalog/exercises"}},
		{"list", "YAMLImport:\n  ExercisesDir:\n    - catalog/exercises\n    - private/exercises\n", []string{"catalog/exercises", "private/exercises"}},
		{"inline list", "YAMLImport:\n  ExercisesDir: [a, b]\n", []string{"a", "b"}},
		{"blank scalar", "YAMLImport:\n  ExercisesDir: \"\"\n", []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "passion.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			c, err := Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got := []string(c.YAMLImport.ExercisesDir)
			if len(got) != len(tc.want) {
				t.Fatalf("ExercisesDir = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("ExercisesDir = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// TestParseDirListSplitsAndTrims covers the environment-variable form.
func TestParseDirListSplitsAndTrims(t *testing.T) {
	got := parseDirList(" catalog/exercises , private/exercises ,, ")
	want := []string{"catalog/exercises", "private/exercises"}
	if len(got) != len(want) {
		t.Fatalf("parseDirList = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("parseDirList = %v, want %v", got, want)
		}
	}
}

// validConfig returns a config that passes Validate, so each test below can break
// exactly one field and prove that field is what Validate rejects.
func validConfig() *Config {
	c := defaultConfig()
	c.Auth.JWTSecret = "0FA7pQxK3mZr8vN2wLtY6cJdHsB4eUgX"
	return c
}

func TestValidateRejectsPlaceholderJWTSecrets(t *testing.T) {
	for _, secret := range []string{"change-me-in-production", "__JWT_SECRET__"} {
		c := validConfig()
		c.Auth.JWTSecret = secret
		err := c.Validate()
		if err == nil {
			t.Fatalf("Validate accepted the placeholder secret %q", secret)
		}
		if !strings.Contains(err.Error(), secret) {
			t.Fatalf("error should name the offending secret, got: %v", err)
		}
	}
}

// The shipped defaults must not be deployable. This is the misconfiguration the
// example file used to invite: copy it, deploy it, sign tokens with a public key.
func TestValidateRejectsDefaultConfig(t *testing.T) {
	if err := defaultConfig().Validate(); err == nil {
		t.Fatal("the default config must not validate — its JWT secret is public")
	}
}

func TestValidateRejectsShortJWTSecret(t *testing.T) {
	c := validConfig()
	c.Auth.JWTSecret = strings.Repeat("a", minJWTSecretLen-1)
	if err := c.Validate(); err == nil {
		t.Fatalf("Validate accepted a %d-character secret", minJWTSecretLen-1)
	}
	c.Auth.JWTSecret = strings.Repeat("a", minJWTSecretLen)
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate rejected a secret of exactly the minimum length: %v", err)
	}
}

// Dev auth bypass short-circuits token verification, so the secret signs nothing.
// Requiring ceremony there would only push developers to invent a throwaway value.
func TestValidateSkipsSecretChecksUnderDevAuthBypass(t *testing.T) {
	c := validConfig()
	c.Auth.JWTSecret = "change-me-in-production"
	c.Auth.DevAuthBypass = true
	if err := c.Validate(); err != nil {
		t.Fatalf("dev auth bypass should not require a real secret: %v", err)
	}
}

// Owner id 0 is the "no owner" sentinel across the data layer, and EnsureSeedUser
// force-sets the primary key from this value.
func TestValidateRejectsZeroDemoOwnerID(t *testing.T) {
	c := validConfig()
	c.Server.DemoOwnerID = 0
	if err := c.Validate(); err == nil {
		t.Fatal("Validate accepted server.demo_owner_id = 0")
	}
}

func TestValidateRejectsZeroImportOwnerID(t *testing.T) {
	c := validConfig()
	c.YAMLImport.Enabled = true
	c.YAMLImport.OwnerID = 0
	if err := c.Validate(); err == nil {
		t.Fatal("Validate accepted yaml_import.owner_id = 0 with import enabled")
	}
	// Disabled import has no owner to check, so the zero must not block startup.
	c.YAMLImport.Enabled = false
	if err := c.Validate(); err != nil {
		t.Fatalf("a zero import owner must be ignored when import is off: %v", err)
	}
}

func TestValidateAcceptsAGoodConfig(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("a well-formed config must validate: %v", err)
	}
}
