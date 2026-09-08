// Command passion serves the V2 stack.
//
// The embeds live here and nowhere else: //go:embed patterns cannot contain "..", and the
// repo root is the only package that can see templates, static and catalog at once.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"passion/config"
	"passion/store"
	"passion/web"
)

//go:embed templates
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

//go:embed catalog
var catalogFS embed.FS

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "passion:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		configPath    = flag.String("config", "", "path to a passion.yaml")
		migrateStatus = flag.Bool("migrate-status", false, "print the schema version and exit")
		migrateOnly   = flag.Bool("migrate-only", false, "apply migrations and exit")
		showVersion   = flag.Bool("version", false, "print the schema version this binary carries and exit")
	)
	flag.Parse()

	cfg, err := config.LoadApp(*configPath)
	if err != nil {
		return err
	}

	log := newLogger(cfg.Log)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, store.Config{
		Engine:       cfg.Database.Engine,
		DSN:          cfg.Database.DSN,
		MaxOpenConns: cfg.Database.MaxOpenConns,
	})
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	if *migrateStatus || *showVersion {
		v, err := st.SchemaVersion(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("engine=%s schema_version=%d\n", cfg.Database.Engine, v)
		return nil
	}

	if cfg.Migrate == "up" || *migrateOnly {
		if err := st.Migrate(ctx); err != nil {
			return err
		}
		v, _ := st.SchemaVersion(ctx)
		log.Info("schema migrated", "engine", cfg.Database.Engine, "version", v)
	}
	if *migrateOnly {
		return nil
	}

	// Sub the embedded trees so a template path is "login.html" rather than
	// "templates/login.html", and a static path is "passion.css".
	templates, err := fs.Sub(templatesFS, "templates")
	if err != nil {
		return err
	}
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		return err
	}

	srv, err := web.New(cfg, st, templates, static, log)
	if err != nil {
		return err
	}

	warnAboutFootguns(log, cfg)

	httpSrv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Server.Addr, "engine", cfg.Database.Engine)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutCtx)
	}
}

func newLogger(cfg config.LogCfg) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	if strings.ToLower(cfg.Format) == "json" {
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}

// warnAboutFootguns says once, loudly, when something is on that must not be on in
// production. Both are refused outright in the configurations where they would be worst.
func warnAboutFootguns(log *slog.Logger, cfg config.App) {
	if cfg.Auth.DevAuthBypass {
		log.Warn("auth.dev_auth_bypass is on: every request is authenticated as the first account")
	}
	if cfg.Auth.InsecureCookies {
		log.Warn("auth.insecure_cookies is on: the session cookie will be sent over plain HTTP")
	}
}

// catalogTrees is what phase 2 will pass to the importer: the embedded tree first, then
// any extra on-disk trees from configuration. Referenced here so the embed is not unused
// before the importer exists.
func catalogTrees(cfg config.App) (fs.FS, []string) {
	sub, err := fs.Sub(catalogFS, "catalog")
	if err != nil {
		return nil, cfg.Catalog.Dirs
	}
	return sub, cfg.Catalog.Dirs
}

var _ = catalogTrees
