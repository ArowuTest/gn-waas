package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bytes"
	"strings"

	"github.com/ArowuTest/gn-waas/backend/cdc-ingestor/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	natsgo "github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// NATS subject constants (mirrors shared/go/messaging)
const (
	subjectCDCSyncCompleted = "gnwaas.cdc.sync.completed"
	subjectSentinelTrigger  = "gnwaas.sentinel.scan.trigger"
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

	// ── Startup Validation ────────────────────────────────────────────────────
	validateConfig(logger)

	// ── GN-WAAS Database (target) ─────────────────────────────────────────────
	gnwaasDSN := fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=%s pool_max_conns=5",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "gnwaas"),
		getEnv("DB_USER", "gnwaas_app"),
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

	// ── Demo Mode: synthetic data generator ───────────────────────────────────
	// When GWL_DB_HOST is not set, the CDC ingestor runs in DEMO mode.
	// DemoSyncService generates realistic meter readings and bills using
	// real account data from the GN-WAAS database, applying PURC 2026 tariffs.
	// This replicates exactly what the live CDC sync would write.
	demoSvc := service.NewDemoSyncService(gnwaasDB, logger)
	fileImportSvc := service.NewFileImportService(gnwaasDB, logger)
	isLiveMode := mapper.IsGWLConfigured()
	if !isLiveMode {
		logger.Info("CDC DEMO MODE ACTIVE — synthetic GWL data will be generated on each sync trigger")
	}

	// ── NATS Connection (optional — graceful degradation if unavailable) ──────
	var nc *natsgo.Conn
	natsURL := getEnv("NATS_URL", "")
	if natsURL != "" {
		nc, err = natsgo.Connect(
			natsURL,
			natsgo.Name("cdc-ingestor"),
			natsgo.MaxReconnects(10),
			natsgo.ReconnectWait(2*time.Second),
		)
		if err != nil {
			logger.Warn("NATS unavailable — running without async messaging", zap.Error(err))
			nc = nil
		} else {
			logger.Info("NATS connected", zap.String("url", natsURL))
			defer nc.Drain()
		}
	} else {
		logger.Info("NATS_URL not set — async messaging disabled")
	}

	// publishSyncCompleted publishes a CDC sync completed event to NATS
	publishSyncCompleted := func(status *service.SyncStatus) {
		if nc == nil {
			return
		}
		event := map[string]interface{}{
			"sync_type":       status.SyncType,
			"records_synced":  status.RecordsSynced,
			"completed_at":    status.CompletedAt,
			"status":          status.Status,
		}
		data, _ := json.Marshal(event)
		if err := nc.Publish(subjectCDCSyncCompleted, data); err != nil {
			logger.Warn("Failed to publish CDC sync event", zap.Error(err))
		} else {
			logger.Debug("Published CDC sync completed event",
				zap.String("sync_type", status.SyncType),
				zap.Int("records", status.RecordsSynced),
			)
		}
	}

	// ── HTTP Server ───────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{AppName: "GN-WAAS CDC Ingestor v1.0"})

	app.Get("/health", func(c *fiber.Ctx) error {
		natsStatus := "disabled"
		if nc != nil && nc.IsConnected() {
			natsStatus = "connected"
		} else if natsURL != "" {
			natsStatus = "disconnected"
		}
		return c.JSON(fiber.Map{
			"service":         "cdc-ingestor",
			"status":          "healthy",
			"gwl_configured":  mapper.IsGWLConfigured(),
			"nats_status":     natsStatus,
		})
	})

	// POST /api/v1/cdc/sync/:type — manual trigger (also called by K8s CronJob)
	// In DEMO mode (no GWL_DB_HOST), uses DemoSyncService to generate synthetic data.
	// In LIVE mode, uses CDCSyncService to sync from the real GWL database.
	app.Post("/api/v1/cdc/sync/:type", func(c *fiber.Ctx) error {
		syncType := c.Params("type")
		if syncType != "accounts" && syncType != "billing" && syncType != "meters" && syncType != "demo" {
			return c.Status(400).JSON(fiber.Map{
				"error": "invalid sync type — must be: accounts, billing, meters, demo",
			})
		}

		syncCtx, syncCancel := context.WithTimeout(c.Context(), 10*time.Minute)
		defer syncCancel()

		// DEMO MODE: generate synthetic data
		if !isLiveMode || syncType == "demo" {
			result, err := demoSvc.RunDemoSync(syncCtx)
			if err != nil {
				logger.Error("Demo sync error", zap.Error(err))
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			// Publish NATS event so Sentinel auto-triggers scans
			if nc != nil {
				event := map[string]interface{}{
					"sync_id":          "demo-" + result.SyncedAt.Format("20060102-150405"),
					"synced_at":        result.SyncedAt,
					"accounts_synced":  result.AccountsFound,
					"bills_synced":     result.BillsSynced,
					"readings_synced":  result.ReadingsSynced,
					"district_codes":   result.DistrictCodes,
				}
				data, _ := json.Marshal(event)
				if err := nc.Publish(subjectCDCSyncCompleted, data); err != nil {
					logger.Warn("Failed to publish demo sync event", zap.Error(err))
				} else {
					logger.Info("Published demo sync completed event to NATS",
						zap.Int("districts", len(result.DistrictCodes)),
					)
				}
			}

			return c.JSON(fiber.Map{
				"mode":             "DEMO",
				"status":           "SUCCESS",
				"readings_synced":  result.ReadingsSynced,
				"bills_synced":     result.BillsSynced,
				"accounts_found":   result.AccountsFound,
				"districts":        result.DistrictCodes,
				"synced_at":        result.SyncedAt,
				"message":          "Demo sync completed — synthetic GWL data written to GN-WAAS tables",
			})
		}

		// LIVE MODE: sync from real GWL database
		status, err := cdcSvc.RunSync(syncCtx, syncType)
		if err != nil {
			logger.Error("CDC sync error", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		// Publish async event so Sentinel can trigger a scan
		if status.Status == "SUCCESS" {
			publishSyncCompleted(status)
		}

		httpStatus := 200
		if status.Status == "FAILED" {
			httpStatus = 500
		}
		return c.Status(httpStatus).JSON(status)
	})

	// POST /api/v1/cdc/sync-all — sync all types sequentially
	app.Post("/api/v1/cdc/sync-all", func(c *fiber.Ctx) error {
		results := make(map[string]*service.SyncStatus)
		for _, syncType := range []string{"accounts", "billing", "meters"} {
			syncCtx, syncCancel := context.WithTimeout(c.Context(), 10*time.Minute)
			status, err := cdcSvc.RunSync(syncCtx, syncType)
			syncCancel()
			if err != nil {
				logger.Error("CDC sync-all error", zap.String("type", syncType), zap.Error(err))
				results[syncType] = &service.SyncStatus{
					SyncType: syncType,
					Status:   "FAILED",
					ErrorMessage: err.Error(),
				}
				continue
			}
			results[syncType] = status
			if status.Status == "SUCCESS" {
				publishSyncCompleted(status)
			}
		}
		return c.JSON(results)
	})

	// ── File Upload Routes (Q4/Q5/Q8) ──────────────────────────────────────────
	// POST /api/v1/cdc/upload/:type — upload a GWL CSV file
	// type: accounts | billing | meter_readings | production_records
	//
	// Used when GWL cannot provide a live DB replica or API.
	// Accepts multipart/form-data with field "file" (CSV).
	// Returns ImportResult with per-row error details.
	app.Post("/api/v1/cdc/upload/:type", func(c *fiber.Ctx) error {
		importType := strings.ToUpper(c.Params("type"))
		validTypes := map[string]bool{
			"ACCOUNTS": true, "BILLING": true,
			"METER_READINGS": true, "PRODUCTION_RECORDS": true,
		}
		if !validTypes[importType] {
			return c.Status(400).JSON(fiber.Map{
				"error": fmt.Sprintf("invalid type %q; valid: accounts, billing, meter_readings, production_records", importType),
			})
		}

		fileHeader, err := c.FormFile("file")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "field 'file' is required (multipart/form-data)"})
		}

		f, err := fileHeader.Open()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to open uploaded file"})
		}
		defer f.Close()

		// Read into buffer so we can pass an io.Reader
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(f); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to read file"})
		}

		importID := uuid.New()
		filename := fileHeader.Filename

		// Record import start in gwl_file_imports
		gnwaasDB.Exec(c.Context(), `
			INSERT INTO gwl_file_imports (id, import_type, filename, file_size_bytes, status, started_at)
			VALUES ($1, $2, $3, $4, 'PROCESSING', NOW())
		`, importID, importType, filename, fileHeader.Size)

		var result *service.ImportResult
		switch importType {
		case "ACCOUNTS":
			result, err = fileImportSvc.ImportAccounts(c.Context(), importID, filename, &buf)
		case "BILLING":
			result, err = fileImportSvc.ImportBilling(c.Context(), importID, filename, &buf)
		case "PRODUCTION_RECORDS":
			result, err = fileImportSvc.ImportProductionRecords(c.Context(), importID, filename, &buf)
		default:
			return c.Status(400).JSON(fiber.Map{"error": "import type not yet implemented: " + importType})
		}

		if err != nil {
			gnwaasDB.Exec(c.Context(), `
				UPDATE gwl_file_imports SET status='FAILED', completed_at=NOW(),
				error_summary=$1 WHERE id=$2
			`, fmt.Sprintf(`[{"error":"%s"}]`, err.Error()), importID)
			return c.Status(422).JSON(fiber.Map{"error": err.Error()})
		}

		// Update import record
		gnwaasDB.Exec(c.Context(), `
			UPDATE gwl_file_imports SET
				status=$1, records_total=$2, records_success=$3, records_failed=$4,
				completed_at=NOW()
			WHERE id=$5
		`, result.Status, result.RecordsTotal, result.RecordsOK, result.RecordsFailed, importID)

		statusCode := 200
		if result.Status == "FAILED" {
			statusCode = 422
		}
		return c.Status(statusCode).JSON(result)
	})

	// GET /api/v1/cdc/imports — list recent file imports
	app.Get("/api/v1/cdc/imports", func(c *fiber.Ctx) error {
		rows, err := gnwaasDB.Query(c.Context(), `
			SELECT id, import_type, filename, file_size_bytes,
			       status, records_total, records_success, records_failed,
			       started_at, completed_at, created_at
			FROM gwl_file_imports
			ORDER BY created_at DESC
			LIMIT 50
		`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to fetch imports"})
		}
		defer rows.Close()

		type importRow struct {
			ID            string  `json:"id"`
			ImportType    string  `json:"import_type"`
			Filename      string  `json:"filename"`
			FileSizeBytes *int64  `json:"file_size_bytes"`
			Status        string  `json:"status"`
			RecordsTotal  int     `json:"records_total"`
			RecordsOK     int     `json:"records_success"`
			RecordsFailed int     `json:"records_failed"`
			StartedAt     *string `json:"started_at"`
			CompletedAt   *string `json:"completed_at"`
			CreatedAt     string  `json:"created_at"`
		}

		var imports []importRow
		for rows.Next() {
			var r importRow
			if err := rows.Scan(
				&r.ID, &r.ImportType, &r.Filename, &r.FileSizeBytes,
				&r.Status, &r.RecordsTotal, &r.RecordsOK, &r.RecordsFailed,
				&r.StartedAt, &r.CompletedAt, &r.CreatedAt,
			); err == nil {
				imports = append(imports, r)
			}
		}
		return c.JSON(fiber.Map{"imports": imports, "total": len(imports)})
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

// validateConfig checks required environment variables and logs warnings for missing optional ones.
// It calls logger.Fatal if any required variable is missing in production mode.
func validateConfig(logger *zap.Logger) {
	env := getEnv("APP_ENV", "development")
	isProd := env == "production"

	required := []struct{ key, desc string }{
		{"DB_HOST",     "GN-WAAS PostgreSQL host"},
		{"DB_NAME",     "GN-WAAS database name"},
		{"DB_USER",     "GN-WAAS database user"},
		{"DB_PASSWORD", "GN-WAAS database password"},
	}

	optional := []struct{ key, desc, defaultVal string }{
		{"GWL_DB_HOST",          "GWL source database host (leave blank for demo mode)", ""},
		{"NATS_URL",             "NATS messaging URL",                                   "nats://localhost:4222"},
		{"SYNC_INTERVAL_MINUTES","Sync interval in minutes",                             "15"},
		{"SYNC_BATCH_SIZE",      "Records per sync batch",                               "500"},
	}

	allOK := true
	for _, r := range required {
		if os.Getenv(r.key) == "" {
			if isProd {
				logger.Error("REQUIRED environment variable not set",
					zap.String("variable", r.key),
					zap.String("description", r.desc),
				)
				allOK = false
			} else {
				logger.Warn("Environment variable not set (using default)",
					zap.String("variable", r.key),
					zap.String("description", r.desc),
				)
			}
		}
	}

	if !allOK {
		logger.Fatal("Startup aborted: missing required environment variables. See .env.example for reference.")
	}

	gwlConfigured := os.Getenv("GWL_DB_HOST") != ""
	if !gwlConfigured {
		logger.Warn("GWL_DB_HOST not set — running in DEMO mode (no live GWL sync)")
	} else {
		logger.Info("GWL source database configured", zap.String("host", os.Getenv("GWL_DB_HOST")))
	}

	for _, o := range optional {
		val := os.Getenv(o.key)
		if val == "" {
			logger.Info("Optional config using default",
				zap.String("variable", o.key),
				zap.String("default", o.defaultVal),
			)
		}
	}

	logger.Info("Configuration validation passed",
		zap.String("env", env),
		zap.Bool("gwl_configured", gwlConfigured),
		zap.Bool("nats_configured", os.Getenv("NATS_URL") != ""),
	)
}
