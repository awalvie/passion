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
	"path/filepath"
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
	// Checked before the server's flags are parsed. It reads files and nothing else.
	if len(os.Args) > 1 && os.Args[1] == "catalog" {
		os.Exit(runCatalog(os.Args[2:]))
	}
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

	if cfg.Catalog.Import {
		if err := importCatalog(ctx, st, cfg, log); err != nil {
			return err
		}
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
// production.
func warnAboutFootguns(log *slog.Logger, cfg config.App) {
	if cfg.Auth.DevAuthBypass {
		log.Warn("auth.dev_auth_bypass is on: every request is authenticated as the first account")
	}
	if cfg.Auth.InsecureCookies {
		log.Warn("auth.insecure_cookies is on: the session cookie will be sent over plain HTTP")
	}
}

// importCatalog brings every tree into the database, shipped first. A private tree names
// shipped rows with app:<slug>, so those rows have to exist before it is read.
func importCatalog(ctx context.Context, st *store.Store, cfg config.App, log *slog.Logger) error {
	shipped, err := fs.Sub(catalogFS, "catalog")
	if err != nil {
		return err
	}
	// The shipped tree is embedded, so nothing can write an id into it at run time. A file
	// with no id there is a build fault, caught in CI.
	app, err := store.Load(shipped, store.ShippedTree, nil)
	if err != nil {
		return fmt.Errorf("the catalog built into this binary does not load: %w.\n"+
			"Run `passion catalog lint --fix ./catalog` and rebuild", err)
	}
	res, err := st.ImportShipped(ctx, app)
	if err != nil {
		return err
	}
	logImport(log, res)

	known := app.Index(true)
	for _, pt := range cfg.Catalog.Private {
		if err := importPrivate(ctx, st, pt, known, log); err != nil {
			return err
		}
	}
	return nil
}

func importPrivate(ctx context.Context, st *store.Store, pt config.PrivateTree,
	known map[store.Ref]bool, log *slog.Logger) error {

	// Only write when something is missing, so an ordinary boot never touches the tree.
	missing, err := store.MissingIDs(pt.Dir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", pt.Dir, err)
	}
	if len(missing) > 0 {
		wrote, err := store.MintIDs(pt.Dir)
		if err != nil {
			return fmt.Errorf("%s has %d files with no id and cannot be written: %w.\n"+
				"Run `passion catalog lint --fix %s` where the tree can be written, and "+
				"commit the result. The first is %s",
				pt.Dir, len(missing), err, pt.Dir, missing[0])
		}
		log.Info("wrote ids into a catalog tree", "tree", pt.Dir, "files", len(wrote))
	}

	tree, err := store.Load(os.DirFS(pt.Dir), filepath.Base(pt.Dir), known)
	if err != nil {
		return fmt.Errorf("catalog tree %s: %w", pt.Dir, err)
	}

	res, err := st.ImportOwned(ctx, tree, pt.Owner)
	if errors.Is(err, store.ErrNoSuchOwner) {
		log.Warn("skipping a private catalog tree: no account holds its owner email",
			"tree", pt.Dir, "owner", pt.Owner)
		return nil
	}
	if err != nil {
		return err
	}
	logImport(log, res)
	return nil
}

func logImport(log *slog.Logger, res store.ImportResult) {
	log.Info("catalog imported",
		"tree", res.Tree, "inserted", res.Inserted, "updated", res.Updated,
		"unchanged", res.Unchanged, "retired", res.Retired, "skipped", len(res.Skipped))
	// Named one by one: nothing else says that the file has stopped having any effect.
	for _, f := range res.Skipped {
		log.Warn("a catalog file is ignored: its row was edited in the app",
			"tree", res.Tree, "file", f)
	}
}
