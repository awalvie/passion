package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	shipped "passion/catalog"
	"passion/server/api"
	"passion/server/catalog"
	"passion/server/config"
	"passion/server/db"
	"passion/server/web"
)

func main() {
	cfg, err := config.Load(os.Getenv("PASSION_CONFIG"))
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	log := slog.New(logHandler(cfg.Log))
	dsn := cfg.Database.URL
	addr := cfg.Server.Addr

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Rule 51: upgrading must never require a second command, so the binary
	// migrates itself on the way up.
	if err := db.Migrate(ctx, dsn); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}

	pool, err := db.Open(ctx, dsn)
	if err != nil {
		log.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := catalog.LoadAll(ctx, pool, log, shipped.Files, cfg.Catalog.Private); err != nil {
		log.Error("catalog", "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.New(pool, log, cfg.Auth.TokenLife).Routes(web.Handler()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server stopped", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil {
		log.Error("shutdown failed", "err", err)
		os.Exit(1)
	}
}

func logHandler(c config.Log) slog.Handler {
	opts := &slog.HandlerOptions{Level: c.Level}
	if c.Format == "json" {
		return slog.NewJSONHandler(os.Stderr, opts)
	}
	return slog.NewTextHandler(os.Stderr, opts)
}
