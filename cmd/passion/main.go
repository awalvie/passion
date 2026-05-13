package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"passion/config"
	"passion/db"
	web "passion/http/server"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	configPath := flag.String("config", "", "path to YAML config file (optional; see passion.example.yaml)")
	exitAfterSeed := flag.Bool("exit-after-seed", false, "seed the database then exit immediately (for make reseed)")
	flag.Parse()

	cfgPath := strings.TrimSpace(*configPath)
	if cfgPath == "" {
		cfgPath = strings.TrimSpace(os.Getenv("PASSION_CONFIG"))
	}
	// Default to local passion.yaml when no explicit config path is provided.
	if cfgPath == "" {
		if _, err := os.Stat("passion.yaml"); err == nil {
			cfgPath = "passion.yaml"
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	if cfgPath != "" {
		slog.Info("config loaded", "path", cfgPath)
	}
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
	}

	store, err := db.NewSqlite(cfg.Server.DBPath)
	if err != nil {
		slog.Error("failed to open database", "path", cfg.Server.DBPath, "error", err)
		os.Exit(1)
	}
	if cfg.Server.Seed {
		demoOwnerID := cfg.Server.DemoOwnerID
		demoHash, err := bcrypt.GenerateFromPassword([]byte("demo12345"), bcrypt.DefaultCost)
		if err != nil {
			slog.Error("failed to hash demo password", "error", err)
			os.Exit(1)
		}
		if err := store.EnsureSeedUser(demoOwnerID, "demo@passion.local", string(demoHash)); err != nil {
			slog.Error("failed to ensure demo user", "owner_id", demoOwnerID, "error", err)
			os.Exit(1)
		}
		slog.Info("dev seeding enabled", "owner_id", demoOwnerID, "db_path", cfg.Server.DBPath)
		if err := store.SeedDevIfEmpty(demoOwnerID); err != nil {
			slog.Error("dev seeding failed", "owner_id", demoOwnerID, "error", err)
			os.Exit(1)
		}
		if *exitAfterSeed {
			slog.Info("--exit-after-seed: done, exiting")
			os.Exit(0)
		}
	}
	var yamlImport *db.YAMLImportOptions
	if cfg.YAMLImport.Enabled {
		yamlImport = &db.YAMLImportOptions{
			ExercisesDir:         cfg.YAMLImport.ExercisesDir,
			SessionTemplatesDir:  cfg.YAMLImport.SessionTemplatesDir,
			ActivityTemplatesDir: cfg.YAMLImport.ActivityTemplatesDir,
		}
		slog.Info(
			"yaml import enabled",
			"exercises_dir", yamlImport.ExercisesDir,
			"activity_templates_dir", yamlImport.ActivityTemplatesDir,
			"session_templates_dir", yamlImport.SessionTemplatesDir,
		)
		// Import for all existing users at startup (idempotent upsert).
		var users []db.User
		if err := store.DB.Find(&users).Error; err != nil {
			slog.Error("yaml import: failed to list users", "error", err)
			os.Exit(1)
		}
		for _, u := range users {
			opts := *yamlImport
			opts.OwnerID = u.ID
			if err := store.ImportYAML(opts); err != nil {
				slog.Error("yaml import failed", "owner_id", u.ID, "error", err)
				os.Exit(1)
			}
		}
	}

	if cfg.Auth.JWTSecret == "change-me-in-production" {
		slog.Warn("jwt secret is set to the default example value — change it before deploying")
	}
	if cfg.Auth.DevAuthBypass {
		slog.Warn("dev auth bypass is enabled — all requests are auto-authenticated; disable before deploying")
	}

	srv, err := web.NewServer(store, cfg.Auth.JWTSecret, time.Duration(cfg.Auth.JWTTTLHours)*time.Hour, cfg.Auth.DevAuthBypass, yamlImport)
	if err != nil {
		slog.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	handler := srv.Routes()

	httpServer := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine so we can handle shutdown signals.
	go func() {
		slog.Info("server listening", "addr", cfg.Server.Addr, "hint", listenHint(cfg.Server.Addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server exited with error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal for graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

// listenHint returns a human-friendly URL hint for common listen addresses.
func listenHint(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	// Strip leading ":" for port-only form.
	if strings.HasPrefix(addr, ":") {
		return "http://127.0.0.1" + addr + " (or this machine's LAN/Tailscale IP with same port)"
	}
	if strings.HasPrefix(addr, "0.0.0.0:") {
		port := strings.TrimPrefix(addr, "0.0.0.0:")
		return "http://127.0.0.1:" + port + " on this host; use your Tailscale/LAN IP with :" + port + " from other devices"
	}
	return "open http://" + addr + " if TCP host:port"
}
