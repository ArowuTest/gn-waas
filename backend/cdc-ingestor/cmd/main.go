package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ArowuTest/gn-waas/backend/cdc-ingestor/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func main() {
	// ── Logger ────────────────────────────────────────────────────────────────
	env := getEnv("APP_ENV", "development")
	var logger *zap.Logger
	if env == "production" {
		logger, _ = zap.NewProduction()
	} else {
		logger, _ = zap.NewDevelopment()
	}
	defer logger.Sync()

	logger.Info("GN-WAAS CDC Ingestor starting", zap.String("env", env))

	// ── GN-WAAS Database (target) ─────────────────────────────────────────────
	gnwaasDSN := fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=%s pool_max_conns=5",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "gnwaas"),
		getEnv("DB_USER", "gnwaas"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_SSL_MODE", "disable"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	gnwaasDB, err := pgxpool.New(ctx, gnwaasDSN)
	cancel()
	if err != nil {
		logger.Fatal("Failed to connect to GN-WAAS database", zap.Error(err))
	}
	defer gnwaasDB.Close()

	if err := gnwaasDB.Ping(context.Background()); err != nil {
		logger.Fatal("GN-WAAS database ping failed", zap.Error(err))
	}
	logger.Info("Connected to GN-WAAS database", zap.String("host", getEnv("DB_HOST", "localhost")))

	// ── Schema Mapper ─────────────────────────────────────────────────────────
	schemaMapPath := getEnv("GWL_SCHEMA_MAP_PATH", "/app/config/gwl_schema_map.yaml")
	mapper, err := service.NewSchemaMapper(schemaMapPath, logger)
	if err != nil {
		logger.Fatal("Failed to load schema mapper", zap.Error(err))
	}

	// ── CDC Sync Service ──────────────────────────────────────────────────────
	cdcSvc := service.NewCDCSyncService(mapper, gnwaasDB, logger)

	// ── HTTP Server ───────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{AppName: "GN-WAAS CDC Ingestor v1.0"})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"service": "cdc-ingestor",
			"status":  "healthy",
			"gwl_configured": mapper.IsGWLConfigured(),
		})
	})

	// POST /api/v1/cdc/sync/:type — manual trigger (also called by K8s CronJob)
	app.Post("/api/v1/cdc/sync/:type", func(c *fiber.Ctx) error {
		syncType := c.Params("type")
		if syncType != "accounts" && syncType != "billing" && syncType != "meters" {
			return c.Status(400).JSON(fiber.Map{
				"error": "invalid sync type — must be: accounts, billing, meters",
			})
		}

		syncCtx, syncCancel := context.WithTimeout(c.Context(), 10*time.Minute)
		defer syncCancel()

		status, err := cdcSvc.RunSync(syncCtx, syncType)
		if err != nil {
			logger.Error("CDC sync error", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		httpStatus := 200
		if status.Status == "FAILED" {
			httpStatus = 500
		}
		return c.Status(httpStatus).JSON(status)
	})

	// GET /api/v1/cdc/status — last sync status per type
	app.Get("/api/v1/cdc/status", func(c *fiber.Ctx) error {
		rows, err := gnwaasDB.Query(c.Context(), `
			SELECT DISTINCT ON (sync_type)
				sync_type, status, records_synced, records_failed,
				started_at, completed_at, error_message
			FROM cdc_sync_log
			ORDER BY sync_type, completed_at DESC`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to fetch sync status"})
		}
		defer rows.Close()

		var statuses []map[string]interface{}
		for rows.Next() {
			var syncType, status, errMsg string
			var synced, failed int
			var startedAt, completedAt time.Time
			if err := rows.Scan(&syncType, &status, &synced, &failed, &startedAt, &completedAt, &errMsg); err != nil {
				continue
			}
			statuses = append(statuses, map[string]interface{}{
				"sync_type":      syncType,
				"status":         status,
				"records_synced": synced,
				"records_failed": failed,
				"started_at":     startedAt,
				"completed_at":   completedAt,
				"error_message":  errMsg,
			})
		}
		return c.JSON(statuses)
	})

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	port := getEnv("APP_PORT", "3006")
	go func() {
		logger.Info("CDC Ingestor listening", zap.String("port", port))
		if err := app.Listen(":" + port); err != nil {
			logger.Error("Server error", zap.Error(err))
		}
	}()

	<-quit
	logger.Info("Shutting down CDC ingestor")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = app.ShutdownWithContext(shutdownCtx)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
