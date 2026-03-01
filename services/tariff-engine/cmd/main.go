package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ArowuTest/gn-waas/services/tariff-engine/internal/app"
	"github.com/ArowuTest/gn-waas/services/tariff-engine/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Bootstrap logger
	logger := buildLogger()
	defer logger.Sync()

	logger.Info("GN-WAAS Tariff Engine starting",
		zap.String("version", "1.0.0"),
	)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Connect to database
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	poolCfg, err := pgxpool.ParseConfig(cfg.Database.DSN())
	if err != nil {
		logger.Fatal("Failed to parse database config", zap.Error(err))
	}
	poolCfg.MaxConns = int32(cfg.Database.MaxConns)
	poolCfg.MinConns = int32(cfg.Database.MinConns)

	db, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}

	if err := db.Ping(ctx); err != nil {
		logger.Fatal("Database ping failed", zap.Error(err))
	}

	logger.Info("Database connected",
		zap.String("host", cfg.Database.Host),
		zap.String("database", cfg.Database.Name),
	)

	// Create and start application
	application := app.New(cfg, db, logger)

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

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		time.Duration(cfg.Server.GracefulStopSec)*time.Second,
	)
	defer shutdownCancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		logger.Error("Graceful shutdown failed", zap.Error(err))
	}

	logger.Info("Tariff engine stopped")
}

func buildLogger() *zap.Logger {
	env := os.Getenv("APP_ENV")

	var cfg zap.Config
	if env == "production" {
		cfg = zap.NewProductionConfig()
		cfg.EncoderConfig.TimeKey = "timestamp"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	logger, err := cfg.Build()
	if err != nil {
		panic("failed to build logger: " + err.Error())
	}
	return logger
}
