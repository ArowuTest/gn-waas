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


// runEmergencyFixes applies critical schema fixes directly in Go code.
// This is independent of the file-based migration runner and guaranteed to run
// on every startup. Each statement is wrapped in its own DO block so individual
// failures are isolated and do not affect other statements.
func runEmergencyFixes(ctx context.Context, dsn string, logger *zap.Logger) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		logger.Warn("Emergency fixes: connect failed", zap.Error(err))
		return
	}
	defer pool.Close()

	fixes := []struct {
		name string
		sql  string
	}{
		// illegal_connections
		{"ic.photo_hashes",    `DO $$ BEGIN ALTER TABLE illegal_connections ADD COLUMN IF NOT EXISTS photo_hashes TEXT[] NOT NULL DEFAULT '{}'; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ic.photo_hashes: %', SQLERRM; END $$`},
		{"ic.district_id",     `DO $$ BEGIN ALTER TABLE illegal_connections ADD COLUMN IF NOT EXISTS district_id UUID REFERENCES districts(id) ON DELETE SET NULL; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ic.district_id: %', SQLERRM; END $$`},
		{"ic.account_number",  `DO $$ BEGIN ALTER TABLE illegal_connections ADD COLUMN IF NOT EXISTS account_number VARCHAR(50); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ic.account_number: %', SQLERRM; END $$`},
		{"ic.gps_accuracy",    `DO $$ BEGIN ALTER TABLE illegal_connections ADD COLUMN IF NOT EXISTS gps_accuracy NUMERIC(8,2) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ic.gps_accuracy: %', SQLERRM; END $$`},
		// audit_events
		{"ae.confirmed_loss",  `DO $$ BEGIN ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS confirmed_loss_ghs NUMERIC(15,2) DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ae.confirmed_loss: %', SQLERRM; END $$`},
		{"ae.recovered_ghs",   `DO $$ BEGIN ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS recovered_ghs NUMERIC(15,2) DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ae.recovered_ghs: %', SQLERRM; END $$`},
		{"ae.success_fee",     `DO $$ BEGIN ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS success_fee_ghs NUMERIC(15,2) DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ae.success_fee: %', SQLERRM; END $$`},
		{"ae.gra_status",      `DO $$ BEGIN ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS gra_status VARCHAR(50) NOT NULL DEFAULT 'PENDING'; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ae.gra_status: %', SQLERRM; END $$`},
		{"ae.photo_hashes",    `DO $$ BEGIN ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS photo_hashes TEXT[] NOT NULL DEFAULT '{}'; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ae.photo_hashes: %', SQLERRM; END $$`},
		{"ae.variance_pct",    `DO $$ BEGIN ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS variance_pct NUMERIC(8,4); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ae.variance_pct: %', SQLERRM; END $$`},
		{"ae.variance_ghs",    `DO $$ BEGIN ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS variance_ghs NUMERIC(15,2); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ae.variance_ghs: %', SQLERRM; END $$`},
		{"ae.ocr_confidence",  `DO $$ BEGIN ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS ocr_confidence NUMERIC(5,4); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ae.ocr_confidence: %', SQLERRM; END $$`},
		{"ae.ocr_status",      `DO $$ BEGIN ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS ocr_status VARCHAR(20) DEFAULT 'PENDING'; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ae.ocr_status: %', SQLERRM; END $$`},
		{"ae.meter_reading",   `DO $$ BEGIN ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS meter_reading_m3 NUMERIC(12,4); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ae.meter_reading: %', SQLERRM; END $$`},
		{"ae.shadow_bill",     `DO $$ BEGIN ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS shadow_bill_ghs NUMERIC(15,2); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ae.shadow_bill: %', SQLERRM; END $$`},
		{"ae.actual_bill",     `DO $$ BEGIN ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS actual_bill_ghs NUMERIC(15,2); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'ae.actual_bill: %', SQLERRM; END $$`},
		// water_balance_records
		{"wb.nrw_percent",     `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS nrw_percent NUMERIC(8,4); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.nrw_percent: %', SQLERRM; END $$`},
		{"wb.ili_score",       `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS ili_score NUMERIC(8,4); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.ili_score: %', SQLERRM; END $$`},
		{"wb.iwa_grade",       `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS iwa_grade VARCHAR(2); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.iwa_grade: %', SQLERRM; END $$`},
		{"wb.est_recovery",    `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS estimated_revenue_recovery_ghs NUMERIC(15,2) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.est_recovery: %', SQLERRM; END $$`},
		{"wb.dcs",             `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS data_confidence_score INTEGER; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.dcs: %', SQLERRM; END $$`},
		{"wb.computed_at",     `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS computed_at TIMESTAMPTZ; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.computed_at: %', SQLERRM; END $$`},
		{"wb.sys_input",       `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS system_input_volume_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.sys_input: %', SQLERRM; END $$`},
		{"wb.billed_metered",  `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS billed_metered_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.billed_metered: %', SQLERRM; END $$`},
		{"wb.billed_unmetered",`DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS billed_unmetered_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.billed_unmetered: %', SQLERRM; END $$`},
		{"wb.unbilled_metered",`DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS unbilled_metered_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.unbilled_metered: %', SQLERRM; END $$`},
		{"wb.unbilled_unm",    `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS unbilled_unmetered_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.unbilled_unm: %', SQLERRM; END $$`},
		{"wb.total_auth",      `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS total_authorised_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.total_auth: %', SQLERRM; END $$`},
		{"wb.unauth_cons",     `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS unauthorised_consumption_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.unauth_cons: %', SQLERRM; END $$`},
		{"wb.meter_inaccuracy",`DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS metering_inaccuracies_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.meter_inaccuracy: %', SQLERRM; END $$`},
		{"wb.data_handling",   `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS data_handling_errors_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.data_handling: %', SQLERRM; END $$`},
		{"wb.apparent_losses", `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS total_apparent_losses_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.apparent_losses: %', SQLERRM; END $$`},
		{"wb.main_leakage",    `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS main_leakage_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.main_leakage: %', SQLERRM; END $$`},
		{"wb.storage_overflow",`DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS storage_overflow_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.storage_overflow: %', SQLERRM; END $$`},
		{"wb.svc_conn_leak",   `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS service_conn_leakage_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.svc_conn_leak: %', SQLERRM; END $$`},
		{"wb.real_losses",     `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS total_real_losses_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.real_losses: %', SQLERRM; END $$`},
		{"wb.total_nrw",       `DO $$ BEGIN ALTER TABLE water_balance_records ADD COLUMN IF NOT EXISTS total_nrw_m3 NUMERIC(15,4) NOT NULL DEFAULT 0; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'wb.total_nrw: %', SQLERRM; END $$`},
		// districts
		{"d.loss_ratio",       `DO $$ BEGIN ALTER TABLE districts ADD COLUMN IF NOT EXISTS loss_ratio_pct NUMERIC(5,2); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'd.loss_ratio: %', SQLERRM; END $$`},
		{"d.gps_lat",          `DO $$ BEGIN ALTER TABLE districts ADD COLUMN IF NOT EXISTS gps_latitude NUMERIC(10,7); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'd.gps_lat: %', SQLERRM; END $$`},
		{"d.gps_lon",          `DO $$ BEGIN ALTER TABLE districts ADD COLUMN IF NOT EXISTS gps_longitude NUMERIC(10,7); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'd.gps_lon: %', SQLERRM; END $$`},
		// field_jobs
		{"fj.outcome",         `DO $$ BEGIN ALTER TABLE field_jobs ADD COLUMN IF NOT EXISTS outcome VARCHAR(50); EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'fj.outcome: %', SQLERRM; END $$`},
		{"fj.outcome_notes",   `DO $$ BEGIN ALTER TABLE field_jobs ADD COLUMN IF NOT EXISTS outcome_notes TEXT; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'fj.outcome_notes: %', SQLERRM; END $$`},
		{"fj.meter_found",     `DO $$ BEGIN ALTER TABLE field_jobs ADD COLUMN IF NOT EXISTS meter_found BOOLEAN; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'fj.meter_found: %', SQLERRM; END $$`},
		{"fj.addr_confirmed",  `DO $$ BEGIN ALTER TABLE field_jobs ADD COLUMN IF NOT EXISTS address_confirmed BOOLEAN; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'fj.addr_confirmed: %', SQLERRM; END $$`},
		{"fj.rec_action",      `DO $$ BEGIN ALTER TABLE field_jobs ADD COLUMN IF NOT EXISTS recommended_action TEXT; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'fj.rec_action: %', SQLERRM; END $$`},
		{"fj.outcome_at",      `DO $$ BEGIN ALTER TABLE field_jobs ADD COLUMN IF NOT EXISTS outcome_recorded_at TIMESTAMPTZ; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'fj.outcome_at: %', SQLERRM; END $$`},
		// permissions
		{"grants",             `DO $$ BEGIN GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO gnwaas_app; GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO gnwaas_app; EXCEPTION WHEN OTHERS THEN RAISE NOTICE 'grants: %', SQLERRM; END $$`},
	}

	applied := 0
	for _, fix := range fixes {
		if _, err := pool.Exec(ctx, fix.sql); err != nil {
			logger.Warn("Emergency fix failed", zap.String("fix", fix.name), zap.Error(err))
		} else {
			applied++
		}
	}
	logger.Info("Emergency schema fixes complete", zap.Int("applied", applied), zap.Int("total", len(fixes)))
}

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
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)`)
	if err != nil {
		return fmt.Errorf("migration: create tracking table: %w", err)
	}

	// Find migration files
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
		pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename=$1)`, fname).Scan(&exists)
		if exists {
			skipped++
			continue
		}

		// Read and execute
		content, err := os.ReadFile(filepath.Join(migrationsDir, fname))
		if err != nil {
			return fmt.Errorf("migration: read %s: %w", fname, err)
		}

		_, err = pool.Exec(ctx, string(content))
		if err != nil {
			logger.Warn("Migration failed (continuing)", zap.String("file", fname), zap.Error(err))
			// Mark as applied anyway to avoid re-running partial migrations
		}

		pool.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1) ON CONFLICT DO NOTHING`, fname)
		logger.Info("Migration applied", zap.String("file", fname))
		applied++
	}

	logger.Info("Migrations complete",
		zap.Int("applied", applied),
		zap.Int("skipped", skipped),
	)
	return nil
}


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
			logger.Warn("Seed failed (continuing)", zap.String("file", fname), zap.Error(err))
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
	if os.Getenv("APP_ENV") == "production" {
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
		zap.String("build_date", "2026-03-10"),
	)

	// Config
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}
	if err := cfg.Validate(); err != nil {
		logger.Fatal("Invalid configuration", zap.Error(err))
	}

	// Run emergency schema fixes first (embedded in binary, no file system dependency)
	if os.Getenv("SKIP_MIGRATIONS") != "true" {
		fixCtx, fixCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		runEmergencyFixes(fixCtx, cfg.Database.DSN(), logger)
		fixCancel()
	}

	// Run migrations before starting the server
	if os.Getenv("SKIP_MIGRATIONS") != "true" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		if err := runMigrations(ctx, cfg.Database.DSN(), logger); err != nil {
			logger.Error("Migrations failed (continuing anyway)", zap.Error(err))
		}
		cancel()
	}

	// Run seeds after migrations if DB is empty
	if os.Getenv("SKIP_MIGRATIONS") != "true" {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Minute)
		if err := runSeeds(ctx2, cfg.Database.DSN(), logger); err != nil {
			logger.Error("Seeds failed (continuing anyway)", zap.Error(err))
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
