package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/app"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// runMigrations applies all pending SQL migration files in lexicographic order.
//
// P1-01 FIX: The previous implementation logged a warning on SQL failure but
// continued, marking the failed migration as "applied". This silently corrupted
// the schema and prevented the failed script from ever being retried.
//
// Correct behaviour:
//   - If a migration SQL file fails, return an error immediately.
//   - Do NOT insert the filename into schema_migrations.
//   - The caller (main) must treat this as a fatal error and halt startup.
func runMigrations(ctx context.Context, dsn string, logger *zap.Logger) error {
	logger.Info("Running database migrations...")

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("migration: connect failed: %w", err)
	}
	defer pool.Close()

	// Create migrations tracking table
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT        PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)
	if err != nil {
		return fmt.Errorf("migration: create tracking table: %w", err)
	}

	// Locate migration files
	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "./migrations"
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		logger.Warn("No migrations directory found, skipping", zap.String("dir", migrationsDir))
		return nil
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	applied := 0
	skipped := 0

	for _, fname := range files {
		// Check if already applied
		var exists bool
		if err := pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename=$1)`, fname,
		).Scan(&exists); err != nil {
			return fmt.Errorf("migration: check applied status for %s: %w", fname, err)
		}
		if exists {
			skipped++
			continue
		}

		// Read the SQL file
		content, err := os.ReadFile(filepath.Join(migrationsDir, fname))
		if err != nil {
			return fmt.Errorf("migration: read %s: %w", fname, err)
		}

		// Execute inside a transaction so a partial failure rolls back cleanly.
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("migration: begin tx for %s: %w", fname, err)
		}

		if _, err = tx.Exec(ctx, string(content)); err != nil {
			// P1-01 FIX: Roll back and return a hard error — do NOT mark as applied.
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration: FAILED executing %s — halting startup: %w", fname, err)
		}

		// Record the migration only after successful execution.
		if _, err = tx.Exec(ctx,
			`INSERT INTO schema_migrations (filename) VALUES ($1) ON CONFLICT DO NOTHING`, fname,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration: record %s: %w", fname, err)
		}

		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("migration: commit %s: %w", fname, err)
		}

		logger.Info("Migration applied", zap.String("file", fname))
		applied++
	}

	logger.Info("Migrations complete",
		zap.Int("applied", applied),
		zap.Int("skipped", skipped),
	)
	return nil
}

// runSeeds applies seed data files. Seeds are idempotent (ON CONFLICT DO NOTHING)
// so failures are logged but do not halt startup — seed failures are non-fatal.
func runSeeds(ctx context.Context, dsn string, logger *zap.Logger) error {
	seedsDir := os.Getenv("SEEDS_DIR")
	if seedsDir == "" {
		seedsDir = "./seeds"
	}

	entries, err := os.ReadDir(seedsDir)
	if err != nil {
		logger.Warn("No seeds directory found, skipping", zap.String("dir", seedsDir))
		return nil
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("seeds: connect failed: %w", err)
	}
	defer pool.Close()

	// Collect all seed files
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	// Only seed core data if districts table is empty (avoid re-seeding)
	var count int
	pool.QueryRow(ctx, `SELECT COUNT(*) FROM districts`).Scan(&count)
	alreadySeeded := count > 0

	applied := 0
	for _, fname := range files {
		// Always apply password hash seed (007) even if already seeded
		// This ensures passwords are set even on existing databases
		isPasswordSeed := strings.Contains(fname, "007_password")
		if alreadySeeded && !isPasswordSeed {
			continue
		}
		fileContent, err := os.ReadFile(filepath.Join(seedsDir, fname))
		if err != nil {
			logger.Warn("Seed read failed", zap.String("file", fname), zap.Error(err))
			continue
		}
		_, err = pool.Exec(ctx, string(fileContent))
		if err != nil {
			logger.Warn("Seed failed (non-fatal, continuing)", zap.String("file", fname), zap.Error(err))
			continue
		}
		logger.Info("Seed applied", zap.String("file", fname))
		applied++
	}
	if alreadySeeded && applied == 0 {
		logger.Info("Database already seeded, skipping core seeds")
	} else {
		logger.Info("Seeding complete", zap.Int("applied", applied))
	}
	return nil
}

func main() {
	// Logger
	var logger *zap.Logger
	var logErr error
	if os.Getenv("APP_ENV") == "production" || os.Getenv("APP_ENV") == "staging" {
		logger, logErr = zap.NewProduction()
	} else {
		logger, logErr = zap.NewDevelopment()
	}
	if logErr != nil {
		panic("failed to create logger: " + logErr.Error())
	}
	defer logger.Sync()

	logger.Info("GN-WAAS API Gateway starting",
		zap.String("version", "1.0.0"),
		zap.String("build_date", "2026-03-11"),
	)

	// Config
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}
	if err := cfg.Validate(); err != nil {
		logger.Fatal("Invalid configuration", zap.Error(err))
	}

	// Run migrations before starting the server.
	// P1-01 FIX: A migration failure is now FATAL — the server will not start
	// with a partially-migrated schema. This prevents silent data corruption.
	if os.Getenv("SKIP_MIGRATIONS") != "true" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if err := runMigrations(ctx, cfg.Database.DSN(), logger); err != nil {
			cancel()
			logger.Fatal("Migration failed — halting startup to prevent data corruption", zap.Error(err))
		}
		cancel()
	}

	// Run seeds after migrations (non-fatal).
	if os.Getenv("SKIP_MIGRATIONS") != "true" {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Minute)
		if err := runSeeds(ctx2, cfg.Database.DSN(), logger); err != nil {
			logger.Warn("Seeds failed (non-fatal)", zap.Error(err))
		}
		cancel2()
	}

	// Application
	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialise application", zap.Error(err))
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := application.Start(); err != nil {
			logger.Error("Server error", zap.Error(err))
		}
	}()

	<-quit
	logger.Info("Shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		logger.Error("Graceful shutdown failed", zap.Error(err))
	} else {
		logger.Info("API Gateway stopped cleanly")
	}
}
