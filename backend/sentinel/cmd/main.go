package main

import (
	"context"
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
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	logger.Info("GN-WAAS Sentinel Service starting")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Database connection
	dsn := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=%s",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "gnwaas"),
		getEnv("DB_USER", "gnwaas_user"),
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

	// Repositories
	anomalyRepo := postgres.NewAnomalyFlagRepository(db, logger)
	billingRepo := postgres.NewGWLBillingRepository(db, logger)
	accountRepo := postgres.NewWaterAccountRepository(db, logger)
	districtRepo := postgres.NewDistrictRepository(db, logger)

	// Services (thresholds from system_config - using defaults here)
	reconcilerSvc := reconciler.NewReconcilerService(logger, 15.0)
	phantomSvc := phantom_checker.NewPhantomCheckerService(logger, 6)
	nightFlowSvc := night_flow.NewNightFlowAnalyser(logger, 30.0)

	// Orchestrator
	orch := orchestrator.NewSentinelOrchestrator(
		anomalyRepo, billingRepo, accountRepo, districtRepo, nil,
		reconcilerSvc, phantomSvc, nightFlowSvc, logger,
	)

	// Handler
	sentinelHandler := handler.NewSentinelHandler(orch, anomalyRepo, districtRepo, logger)

	// HTTP Server
	app := fiber.New(fiber.Config{AppName: "GN-WAAS Sentinel v1.0"})

	app.Get("/health", sentinelHandler.HealthCheck)

	api := app.Group("/api/v1/sentinel")
	api.Post("/scan/:district_id", sentinelHandler.TriggerScan)
	api.Get("/anomalies", sentinelHandler.GetAnomalies)
	api.Get("/anomalies/:id", sentinelHandler.GetAnomaly)
	api.Get("/summary/:district_id", sentinelHandler.GetDistrictSummary)
	api.Patch("/anomalies/:id/resolve", sentinelHandler.ResolveAnomaly)
	api.Patch("/anomalies/:id/false-positive", sentinelHandler.MarkFalsePositive)

	// Graceful shutdown
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

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
