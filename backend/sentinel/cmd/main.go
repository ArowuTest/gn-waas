package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/handler"
	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/repository/postgres"
	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/service/night_flow"
	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/service/orchestrator"
	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/service/phantom_checker"
	"github.com/ArowuTest/gn-waas/backend/sentinel/internal/service/reconciler"
	"github.com/gofiber/fiber/v2"
	pgxpool "github.com/jackc/pgx/v5/pgxpool"
	natsgo "github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// ─── NATS event payloads (mirrors cdc-ingestor and meter-ingestor) ────────────

// CDCSyncCompletedEvent is published by the CDC ingestor after a successful sync.
// Sentinel subscribes to this to trigger automatic district scans.
type CDCSyncCompletedEvent struct {
	SyncID          string    `json:"sync_id"`
	SyncedAt        time.Time `json:"synced_at"`
	AccountsSynced  int       `json:"accounts_synced"`
	BillsSynced     int       `json:"bills_synced"`
	ReadingsSynced  int       `json:"readings_synced"`
	DistrictCodes   []string  `json:"district_codes"` // districts that had new data
}

// SentinelTriggerEvent is published by the meter-ingestor when an inline anomaly is detected.
type SentinelTriggerEvent struct {
	DistrictCode string    `json:"district_code"`
	Reason       string    `json:"reason"`
	TriggeredAt  time.Time `json:"triggered_at"`
}

// NATS subjects
const (
	subjectCDCSyncCompleted  = "gnwaas.cdc.sync.completed"
	subjectSentinelTrigger   = "gnwaas.sentinel.scan.trigger"
	subjectMeterReadingRcvd  = "gnwaas.meter.reading.received"
)

// loadConfigFloat reads a float64 value from the system_config table.
// Falls back to defaultVal if the key is missing or the DB query fails.
// This is called once at startup — sentinel does not hot-reload thresholds.
func loadConfigFloat(ctx context.Context, db *pgxpool.Pool, logger *zap.Logger, key string, defaultVal float64) float64 {
	var val float64
	err := db.QueryRow(ctx,
		`SELECT config_value::float8
		   FROM system_config
		  WHERE config_key = $1
		    AND is_active = true
		  LIMIT 1`,
		key,
	).Scan(&val)
	if err != nil {
		logger.Warn("system_config key not found — using default",
			zap.String("key", key),
			zap.Float64("default", defaultVal),
			zap.Error(err),
		)
		return defaultVal
	}
	return val
}

func main() {
	var logger *zap.Logger
	if os.Getenv("APP_ENV") == "production" {
		logger, _ = zap.NewProduction()
	} else {
		logger, _ = zap.NewDevelopment()
	}
	defer logger.Sync()

	logger.Info("GN-WAAS Sentinel Service starting")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// ── Database ──────────────────────────────────────────────────────────────
	dsn := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "gnwaas"),
		getEnv("DB_USER", "gnwaas_app"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_SSL_MODE", "disable"),
	)

	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		logger.Fatal("Database ping failed", zap.Error(err))
	}
	logger.Info("Connected to GN-WAAS database")

	// ── Repositories ──────────────────────────────────────────────────────────
	anomalyRepo  := postgres.NewAnomalyFlagRepository(db, logger)
	billingRepo  := postgres.NewGWLBillingRepository(db, logger)
	accountRepo  := postgres.NewWaterAccountRepository(db, logger)
	districtRepo := postgres.NewDistrictRepository(db, logger)

	// ── Load thresholds from system_config (with safe defaults) ─────────────
	// These values are seeded in database/seeds/001_system_config.sql and can
	// be updated at runtime via the admin portal without redeploying.
	shadowBillVariancePct := loadConfigFloat(ctx, db, logger,
		"sentinel.shadow_bill_variance_pct", 15.0)
	nightFlowPctOfDaily := loadConfigFloat(ctx, db, logger,
		"sentinel.night_flow_pct_of_daily", 30.0)
	phantomMeterMonths := int(loadConfigFloat(ctx, db, logger,
		"sentinel.phantom_meter_months", 6.0))

	logger.Info("Sentinel thresholds loaded from system_config",
		zap.Float64("shadow_bill_variance_pct", shadowBillVariancePct),
		zap.Float64("night_flow_pct_of_daily", nightFlowPctOfDaily),
		zap.Int("phantom_meter_months", phantomMeterMonths),
	)

	// ── Services ──────────────────────────────────────────────────────────────
	reconcilerSvc := reconciler.NewReconcilerService(logger, shadowBillVariancePct)
	phantomSvc    := phantom_checker.NewPhantomCheckerService(logger, phantomMeterMonths)
	nightFlowSvc  := night_flow.NewNightFlowAnalyser(logger, nightFlowPctOfDaily)

	// ── Orchestrator ──────────────────────────────────────────────────────────
	orch := orchestrator.NewSentinelOrchestrator(
		anomalyRepo, billingRepo, accountRepo, districtRepo, nil,
		reconcilerSvc, phantomSvc, nightFlowSvc, logger,
	)

	// ── HTTP Handler ──────────────────────────────────────────────────────────
	sentinelHandler := handler.NewSentinelHandler(orch, anomalyRepo, districtRepo, logger)

	// ── NATS Subscriber ───────────────────────────────────────────────────────
	// Sentinel subscribes to two subjects:
	//   1. gnwaas.cdc.sync.completed  → scan all districts that had new data
	//   2. gnwaas.sentinel.scan.trigger → scan a specific district immediately
	//      (published by meter-ingestor when inline pre-check flags an anomaly)
	natsURL := getEnv("NATS_URL", "")
	if natsURL != "" {
		nc, natsErr := natsgo.Connect(natsURL,
			natsgo.Name("gnwaas-sentinel"),
			natsgo.ReconnectWait(3*time.Second),
			natsgo.MaxReconnects(20),
			natsgo.DisconnectErrHandler(func(_ *natsgo.Conn, err error) {
				logger.Warn("NATS disconnected", zap.Error(err))
			}),
			natsgo.ReconnectHandler(func(nc *natsgo.Conn) {
				logger.Info("NATS reconnected", zap.String("url", nc.ConnectedUrl()))
			}),
		)
		if natsErr != nil {
			logger.Warn("NATS connection failed — sentinel will not auto-scan on CDC events",
				zap.Error(natsErr))
		} else {
			defer nc.Close()
			logger.Info("Connected to NATS", zap.String("url", natsURL))

			// ── Subscribe: CDC sync completed → scan affected districts ──────
			_, subErr := nc.Subscribe(subjectCDCSyncCompleted, func(msg *natsgo.Msg) {
				var evt CDCSyncCompletedEvent
				if err := json.Unmarshal(msg.Data, &evt); err != nil {
					logger.Error("Failed to parse CDCSyncCompletedEvent", zap.Error(err))
					return
				}

				logger.Info("CDC sync completed — triggering sentinel scans",
					zap.String("sync_id", evt.SyncID),
					zap.Int("accounts_synced", evt.AccountsSynced),
					zap.Int("bills_synced", evt.BillsSynced),
					zap.Int("readings_synced", evt.ReadingsSynced),
					zap.Strings("districts", evt.DistrictCodes),
				)

				// If specific districts are listed, scan only those.
				// Otherwise scan all active districts.
				if len(evt.DistrictCodes) > 0 {
					for _, districtCode := range evt.DistrictCodes {
						go runDistrictScan(orch, districtCode, "cdc_sync_completed", logger)
					}
				} else {
					go runAllDistrictScans(db, orch, "cdc_sync_completed", logger)
				}
			})
			if subErr != nil {
				logger.Error("Failed to subscribe to CDC sync events", zap.Error(subErr))
			} else {
				logger.Info("Subscribed to CDC sync events", zap.String("subject", subjectCDCSyncCompleted))
			}

			// ── Subscribe: Meter anomaly trigger → scan specific district ────
			_, subErr2 := nc.Subscribe(subjectSentinelTrigger, func(msg *natsgo.Msg) {
				var evt SentinelTriggerEvent
				if err := json.Unmarshal(msg.Data, &evt); err != nil {
					logger.Error("Failed to parse SentinelTriggerEvent", zap.Error(err))
					return
				}

				logger.Info("Sentinel scan triggered by meter anomaly",
					zap.String("district_code", evt.DistrictCode),
					zap.String("reason", evt.Reason),
				)

				go runDistrictScan(orch, evt.DistrictCode, "meter_anomaly_trigger", logger)
			})
			if subErr2 != nil {
				logger.Error("Failed to subscribe to sentinel trigger events", zap.Error(subErr2))
			} else {
				logger.Info("Subscribed to sentinel trigger events", zap.String("subject", subjectSentinelTrigger))
			}
		}
	} else {
		logger.Info("NATS_URL not set — sentinel will only scan via HTTP API calls")
	}

	// ── Scheduled Night-Flow Scan (2–4 AM UTC daily) ────────────────────────
	// The spec requires automated night-flow analysis during the minimum-demand
	// window (2–4 AM) when legitimate consumption is near zero.
	// We trigger at 02:05 UTC to ensure all meter readings for the night window
	// have been ingested before analysis begins.
	go func() {
		logger.Info("Night-flow scheduler started — will scan all districts at 02:05 UTC daily")
		for {
			now := time.Now().UTC()
			// Calculate next 02:05 UTC
			next := time.Date(now.Year(), now.Month(), now.Day(), 2, 5, 0, 0, time.UTC)
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}
			waitDuration := next.Sub(now)
			logger.Info("Next night-flow scan scheduled",
				zap.Time("at", next),
				zap.Duration("in", waitDuration),
			)
			time.Sleep(waitDuration)

			logger.Info("Night-flow scan triggered by scheduler",
				zap.Time("scan_time", time.Now().UTC()),
			)
			go runAllDistrictScans(db, orch, "scheduled_night_flow", logger)
		}
	}()

	// ── HTTP Server ───────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{AppName: "GN-WAAS Sentinel v1.0"})

	app.Get("/health", sentinelHandler.HealthCheck)

	api := app.Group("/api/v1/sentinel")
	api.Post("/scan/:district_id", sentinelHandler.TriggerScan)
	api.Get("/anomalies", sentinelHandler.GetAnomalies)
	api.Get("/anomalies/:id", sentinelHandler.GetAnomaly)
	api.Get("/summary/:district_id", sentinelHandler.GetDistrictSummary)
	api.Patch("/anomalies/:id/resolve", sentinelHandler.ResolveAnomaly)
	api.Patch("/anomalies/:id/false-positive", sentinelHandler.MarkFalsePositive)

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	port := getEnv("APP_PORT", "3002")
	go func() {
		logger.Info("Sentinel listening", zap.String("port", port))
		if err := app.Listen(":" + port); err != nil {
			logger.Error("Server error", zap.Error(err))
		}
	}()

	<-quit
	logger.Info("Shutting down sentinel")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = app.ShutdownWithContext(shutdownCtx)
}

// runDistrictScan triggers a full sentinel scan for a single district by its code.
// It resolves the district UUID from the code, then calls the orchestrator.
func runDistrictScan(
	orch *orchestrator.SentinelOrchestrator,
	districtCode string,
	trigger string,
	logger *zap.Logger,
) {
	scanCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger.Info("Running sentinel scan",
		zap.String("district_code", districtCode),
		zap.String("trigger", trigger),
	)

	result, err := orch.RunScanByCode(scanCtx, districtCode)
	if err != nil {
		logger.Error("Sentinel scan failed",
			zap.String("district_code", districtCode),
			zap.String("trigger", trigger),
			zap.Error(err),
		)
		return
	}

	logger.Info("Sentinel scan completed",
		zap.String("district_code", districtCode),
		zap.String("trigger", trigger),
		zap.Int("anomalies_found", result.AnomaliesFound),
		zap.Int("critical", result.CriticalCount),
		zap.Duration("duration", result.Duration),
	)
}

// runAllDistrictScans fetches all active districts and scans each one.
func runAllDistrictScans(
	db *pgxpool.Pool,
	orch *orchestrator.SentinelOrchestrator,
	trigger string,
	logger *zap.Logger,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	rows, err := db.Query(ctx,
		`SELECT district_code FROM districts WHERE is_active = true ORDER BY district_name`)
	if err != nil {
		logger.Error("Failed to fetch districts for scan", zap.Error(err))
		return
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err == nil {
			codes = append(codes, code)
		}
	}

	logger.Info("Running sentinel scans for all active districts",
		zap.Int("district_count", len(codes)),
		zap.String("trigger", trigger),
	)

	for _, code := range codes {
		go runDistrictScan(orch, code, trigger, logger)
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
