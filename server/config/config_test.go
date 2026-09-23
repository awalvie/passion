package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// clearEnv keeps the developer's shell out of the test. The nix shell sets
// DATABASE_URL, and Load treats an empty value as unset.
func clearEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PASSION_ADDR", "")
}

func writeFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "passion.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://env")

	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Addr != ":8080" {
		t.Errorf("addr %q", c.Server.Addr)
	}
	if c.Auth.TokenLife != 30*24*time.Hour {
		t.Errorf("token life %v", c.Auth.TokenLife)
	}
	if c.Log.Level != slog.LevelInfo || c.Log.Format != "text" {
		t.Errorf("log %+v", c.Log)
	}
	if len(c.Catalog.Private) != 0 {
		t.Errorf("private %+v", c.Catalog.Private)
	}
}

func TestLoadFile(t *testing.T) {
	clearEnv(t)
	path := writeFile(t, `
server:
  addr: ":9000"
database:
  url: postgres://file
auth:
  token_life: 48h
log:
  level: debug
  format: json
catalog:
  private:
    - location: /srv/catalogs/paradigm
      owner: [ada@example.com, grace@example.com]
`)

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Addr != ":9000" {
		t.Errorf("addr %q", c.Server.Addr)
	}
	if c.Database.URL != "postgres://file" {
		t.Errorf("url %q", c.Database.URL)
	}
	if c.Auth.TokenLife != 48*time.Hour {
		t.Errorf("token life %v", c.Auth.TokenLife)
	}
	if c.Log.Level != slog.LevelDebug || c.Log.Format != "json" {
		t.Errorf("log %+v", c.Log)
	}
	if len(c.Catalog.Private) != 1 {
		t.Fatalf("private %+v", c.Catalog.Private)
	}
	p := c.Catalog.Private[0]
	if p.Location != "/srv/catalogs/paradigm" || strings.Join(p.Owner, ",") != "ada@example.com,grace@example.com" {
		t.Errorf("private %+v", p)
	}
}

// Setting one key in a section must leave that section's other defaults alone.
func TestPartialSectionKeepsDefaults(t *testing.T) {
	clearEnv(t)
	path := writeFile(t, "database:\n  url: postgres://file\nlog:\n  format: json\n")

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Log.Level != slog.LevelInfo {
		t.Errorf("level %v", c.Log.Level)
	}
	if c.Server.Addr != ":8080" {
		t.Errorf("addr %q", c.Server.Addr)
	}
}

func TestEnvWinsOverFile(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://env")
	t.Setenv("PASSION_ADDR", ":7000")
	path := writeFile(t, "server:\n  addr: \":9000\"\ndatabase:\n  url: postgres://file\n")

	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Database.URL != "postgres://env" || c.Server.Addr != ":7000" {
		t.Errorf("got %q %q", c.Database.URL, c.Server.Addr)
	}
}

// A file that sets only the log level must work when the database comes from
// the environment, as it does under docker.
func TestDatabaseURLCanComeFromEnvAlone(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://env")

	if _, err := Load(writeFile(t, "log:\n  level: warn\n")); err != nil {
		t.Fatal(err)
	}
}

func TestCommentsOnlyFile(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://env")

	if _, err := Load(writeFile(t, "# nothing set yet\n")); err != nil {
		t.Fatal(err)
	}
}

func TestMissingFile(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://env")

	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("no error")
	}
}

func TestInvalid(t *testing.T) {
	cases := map[string]struct {
		body string
		want string
	}{
		"unknown key": {
			"database:\n  url: postgres://file\nserver:\n  adress: \":9000\"\n",
			"line 4: field adress not found",
		},
		"no database url": {
			"server:\n  addr: \":9000\"\n",
			"database.url is required",
		},
		"empty addr": {
			"database:\n  url: postgres://file\nserver:\n  addr: \"\"\n",
			"server.addr is empty",
		},
		"duration without a unit": {
			"database:\n  url: postgres://file\nauth:\n  token_life: 72\n",
			"line 4:",
		},
		"token life too short to slide": {
			"database:\n  url: postgres://file\nauth:\n  token_life: 24h\n",
			"auth.token_life must be longer than 24h",
		},
		"unknown level": {
			"database:\n  url: postgres://file\nlog:\n  level: loud\n",
			"loud",
		},
		"unknown format": {
			"database:\n  url: postgres://file\nlog:\n  format: xml\n",
			`log.format is "xml"`,
		},
		"private without a location": {
			"database:\n  url: postgres://file\ncatalog:\n  private:\n    - owner: [ada@example.com]\n",
			"catalog.private[0].location is required",
		},
		"private without an owner": {
			"database:\n  url: postgres://file\ncatalog:\n  private:\n    - location: /srv/x\n",
			"catalog.private[0].owner needs at least one email",
		},
		"blank owner": {
			"database:\n  url: postgres://file\ncatalog:\n  private:\n    - location: /srv/x\n      owner: [\"\"]\n",
			"catalog.private[0].owner[0] is empty",
		},
		"owner not a list": {
			"database:\n  url: postgres://file\ncatalog:\n  private:\n    - location: /srv/x\n      owner: ada@example.com\n",
			"line 6:",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			clearEnv(t)
			_, err := Load(writeFile(t, c.body))
			if err == nil {
				t.Fatal("no error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %q does not mention %q", err, c.want)
			}
		})
	}
}
