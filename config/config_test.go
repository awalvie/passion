package config

import (
	"os"
	"path/filepath"
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
	if c.YAMLImport.ExercisesDir == "" || c.YAMLImport.SessionTemplatesDir == "" || c.YAMLImport.OwnerID == 0 {
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
	if !c.YAMLImport.Enabled || c.YAMLImport.ExercisesDir != "fixtures/exercises" || c.YAMLImport.SessionTemplatesDir != "fixtures/templates" || c.YAMLImport.OwnerID != 7 {
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
	if c.YAMLImport.ExercisesDir != "catalog/exercises" || c.YAMLImport.SessionTemplatesDir != "catalog/session_templates" {
		t.Fatalf("yaml dirs env override failed: %+v", c)
	}
	if c.YAMLImport.OwnerID != 42 {
		t.Fatalf("yaml owner env override failed: %+v", c)
	}
}
