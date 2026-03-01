package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ArowuTest/gn-waas/services/api-gateway/internal/app"
	"github.com/ArowuTest/gn-waas/services/api-gateway/internal/config"
	"go.uber.org/zap"
)

func main() {
	// Logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic("failed to create logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("GN-WAAS API Gateway starting",
		zap.String("version", "1.0.0"),
		zap.String("build_date", "2026-03-01"),
	)

	// Config
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
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
