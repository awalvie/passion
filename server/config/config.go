// Package config reads the server's settings: defaults, then an optional YAML
// file, then the environment.
package config

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	Server   Server   `yaml:"server"`
	Database Database `yaml:"database"`
	Auth     Auth     `yaml:"auth"`
	Log      Log      `yaml:"log"`
	Catalog  Catalog  `yaml:"catalog"`
}

type Server struct {
	Addr string `yaml:"addr"`
}

type Database struct {
	URL string `yaml:"url"`
}

type Auth struct {
	// TokenLife is how long a sign-in lasts unused. Use slides it forward.
	TokenLife time.Duration `yaml:"token_life"`
}

type Log struct {
	Level  slog.Level `yaml:"level"`
	Format string     `yaml:"format"`
}

type Catalog struct {
	Private []PrivateCatalog `yaml:"private"`
}

// PrivateCatalog is a tree of catalog files. Each owner gets their own copy,
// and nobody else sees it.
type PrivateCatalog struct {
	Location string   `yaml:"location"`
	Owner    []string `yaml:"owner"`
}

func defaults() Config {
	return Config{
		Server: Server{Addr: ":8080"},
		// Thirty days is the longest a reauthentication timeout should be at
		// this assurance level.
		Auth: Auth{TokenLife: 30 * 24 * time.Hour},
		Log:  Log{Level: slog.LevelInfo, Format: "text"},
	}
}

// Load reads the file at path over the defaults, when path is set. Then
// DATABASE_URL and PASSION_ADDR win over the file, so a deploy can set those
// alone and need no file.
func Load(path string) (Config, error) {
	c := defaults()
	if path != "" {
		if err := decodeFile(path, &c); err != nil {
			return Config{}, err
		}
	}

	if v := os.Getenv("DATABASE_URL"); v != "" {
		c.Database.URL = v
	}
	if v := os.Getenv("PASSION_ADDR"); v != "" {
		c.Server.Addr = v
	}

	if err := c.check(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func decodeFile(path string, c *Config) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// A misspelt key must stop the server, not be dropped and leave the
	// default in place.
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)

	// A file holding only comments has no settings, which is not a mistake.
	if err := dec.Decode(c); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func (c Config) check() error {
	var problems []error
	if c.Database.URL == "" {
		problems = append(problems, errors.New("database.url is required, in the file or as DATABASE_URL"))
	}
	if c.Server.Addr == "" {
		problems = append(problems, errors.New("server.addr is empty"))
	}
	// A token slides forward once a day of its life has gone, so a life of a
	// day or less would never slide.
	if c.Auth.TokenLife <= 24*time.Hour {
		problems = append(problems, errors.New("auth.token_life must be longer than 24h"))
	}
	if c.Log.Format != "text" && c.Log.Format != "json" {
		problems = append(problems, fmt.Errorf("log.format is %q, want text or json", c.Log.Format))
	}

	for i, p := range c.Catalog.Private {
		if strings.TrimSpace(p.Location) == "" {
			problems = append(problems, fmt.Errorf("catalog.private[%d].location is required", i))
		}
		if len(p.Owner) == 0 {
			problems = append(problems, fmt.Errorf("catalog.private[%d].owner needs at least one email", i))
		}
		for j, owner := range p.Owner {
			if strings.TrimSpace(owner) == "" {
				problems = append(problems, fmt.Errorf("catalog.private[%d].owner[%d] is empty", i, j))
			}
		}
	}
	return errors.Join(problems...)
}
