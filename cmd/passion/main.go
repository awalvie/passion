package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
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
	mintInvites := flag.Int("mint-invites", 0, "print this many new signup invite codes, then exit")
	inviteNote := flag.String("invite-note", "", "note stored with minted invite codes, e.g. who they are for")
	listInvites := flag.Bool("list-invites", false, "list every invite code and whether it has been used, then exit")
	purgeOrphans := flag.Bool("purge-orphans", false, "delete exercises orphaned by the old importer bug, then exit — run the dry run first")
	purgeDryRun := flag.Bool("purge-orphans-dry-run", false, "count the orphaned exercises without deleting anything, then exit")
	backfillRuns := flag.Bool("backfill-runs", false, "give every past run its own copy of the exercises its records point at, then exit")
	backfillDryRun := flag.Bool("backfill-runs-dry-run", false, "report what --backfill-runs would change, then exit")
	keepOnlyUser := flag.Uint("delete-users-except", 0, "permanently delete every account except this user id, and everything those accounts own")
	confirmDelete := flag.Bool("i-have-a-backup", false, "required with --delete-users-except: without it the deletion is only a dry run")
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
	// Invite administration runs against the database and exits. It deliberately happens
	// before seeding and the YAML import, so minting a code never waits on a full import.
	if *mintInvites > 0 || *listInvites {
		if err := runInviteCommand(store, *mintInvites, *inviteNote, *listInvites); err != nil {
			slog.Error("invite command failed", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *backfillRuns || *backfillDryRun {
		if err := runBackfillCommand(store, cfg.Server.DBPath, *backfillDryRun); err != nil {
			slog.Error("backfill failed", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *keepOnlyUser > 0 {
		if err := runDeleteUsersCommand(store, cfg.Server.DBPath, *keepOnlyUser, *confirmDelete); err != nil {
			slog.Error("delete users failed", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *purgeOrphans || *purgeDryRun {
		if err := runPurgeCommand(store, cfg.Server.DBPath, *purgeDryRun); err != nil {
			slog.Error("purge failed", "error", err)
			os.Exit(1)
		}
		os.Exit(0)
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

// runDeleteUsersCommand shows exactly what deleting every other account would remove, and
// removes it only when the caller confirms a backup exists. There is no undo.
func runDeleteUsersCommand(store *db.Store, dbPath string, keepUserID uint, confirmed bool) error {
	plan, err := db.PlanDeleteAllUsersExcept(store.DB, keepUserID)
	if err != nil {
		return err
	}

	fmt.Printf("database:  %s\n", dbPath)
	fmt.Printf("keeping:   user %d  <%s>\n\n", plan.KeepUserID, plan.KeepEmail)

	if len(plan.DeleteUsers) == 0 {
		fmt.Println("No other accounts exist. Nothing to do.")
		return nil
	}

	fmt.Printf("DELETING %d account(s):\n", len(plan.DeleteUsers))
	for _, u := range plan.DeleteUsers {
		fmt.Printf("  user %-4d %s\n", u.ID, u.Email)
	}

	fmt.Printf("\nand %d row(s) they own:\n", plan.TotalRows)
	tables := make([]string, 0, len(plan.RowsByTable))
	for t := range plan.RowsByTable {
		tables = append(tables, t)
	}
	sort.Strings(tables)
	for _, t := range tables {
		fmt.Printf("  %-32s %8d\n", t, plan.RowsByTable[t])
	}
	if plan.InviteCodes > 0 {
		fmt.Printf("  %-32s %8d\n", "invite_codes (redeemed by them)", plan.InviteCodes)
	}

	if !confirmed {
		fmt.Println("\nDry run. Nothing was changed.")
		fmt.Println("Read the list above and make sure it holds no account you want.")
		fmt.Println("Then back up the database and run again with --i-have-a-backup.")
		return nil
	}

	fmt.Println("\nDeleting now. This cannot be undone.")
	applied, err := db.DeleteAllUsersExcept(store.DB, keepUserID)
	if err != nil {
		return err
	}
	fmt.Printf("Deleted %d account(s) and everything they owned.\n", applied.DeletedUsers)
	fmt.Println("The rows are gone but the file is the same size. Run VACUUM to reclaim the space.")
	return nil
}

// runBackfillCommand gives past runs their own exercises. A run used to read the live
// template to render itself, so an import rewrote what a finished session said the athlete
// did. Output goes to stdout so the owner can read the numbers before and after.
func runBackfillCommand(store *db.Store, dbPath string, dryRun bool) error {
	rep, err := db.BackfillRunExercises(store.DB, dryRun)
	if err != nil {
		return err
	}
	fmt.Printf("database:            %s\n", dbPath)
	fmt.Printf("runs examined:       %d\n", rep.RunsExamined)
	fmt.Printf("runs to change:      %d\n", rep.RunsChanged)
	fmt.Printf("exercises to copy:   %d\n", rep.Copied)
	fmt.Printf("empty rows to clear: %d\n", rep.EmptyRemoved)
	if rep.DryRun {
		fmt.Println("\nDry run. Nothing was changed.")
		fmt.Println("Back up the database, then run again with --backfill-runs.")
		return nil
	}
	fmt.Println("\nDone. Every past run now renders from its own rows.")
	return nil
}

// runPurgeCommand reports the orphaned exercises left by the old importer bug, and on a
// real run deletes the ones no run history references. Output goes to stdout so the owner
// can read the numbers before and after.
func runPurgeCommand(store *db.Store, dbPath string, dryRun bool) error {
	rep, err := db.PurgeOrphanedExercises(store.DB, dryRun)
	if err != nil {
		return err
	}

	fmt.Printf("database:              %s\n", dbPath)
	fmt.Printf("live exercises:        %d\n", rep.LiveExercisesBefore)
	fmt.Printf("orphaned:              %d\n", rep.OrphanCandidates)
	fmt.Printf("  held by history:     %d  (kept)\n", rep.KeptForHistory)
	fmt.Printf("  safe to remove:      %d\n", rep.SafeToPurge)

	if rep.DryRun {
		fmt.Println("\nDry run. Nothing was changed.")
		fmt.Println("Back up the database, then run again with --purge-orphans.")
		return nil
	}
	fmt.Printf("\nremoved exercises:     %d\n", rep.DeletedExercises)
	fmt.Printf("removed activities:    %d\n", rep.DeletedActivities)
	fmt.Println("\nThe rows are gone but the file is the same size. Run VACUUM to reclaim the space.")
	return nil
}

// runInviteCommand mints or lists signup invite codes and returns. Output goes to stdout
// rather than the structured logger, because a code is meant to be copied by hand.
func runInviteCommand(store *db.Store, mint int, note string, list bool) error {
	for i := 0; i < mint; i++ {
		code, err := db.CreateInviteCode(store.DB, nil, note, nil)
		if err != nil {
			return err
		}
		fmt.Println(code.Code)
	}
	if !list {
		return nil
	}
	codes, err := db.ListInviteCodes(store.DB)
	if err != nil {
		return err
	}
	if len(codes) == 0 {
		fmt.Println("no invite codes yet — mint some with --mint-invites=5")
		return nil
	}
	fmt.Printf("%-16s %-10s %-20s %s\n", "CODE", "STATE", "USED", "NOTE")
	for _, c := range codes {
		state, used := "open", ""
		switch {
		case c.Redeemed():
			state = "used"
			used = c.UsedAt.Format("2006-01-02 15:04")
		case c.Expired(time.Now()):
			state = "expired"
		}
		fmt.Printf("%-16s %-10s %-20s %s\n", db.FormatInviteCode(c.Code), state, used, c.Note)
	}
	return nil
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
