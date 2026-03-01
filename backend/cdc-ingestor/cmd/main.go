package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ArowuTest/gn-waas/services/cdc-ingestor/internal/service"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	logger.Info("GN-WAAS CDC Ingestor starting")

	schemaMapPath := getEnv("GWL_SCHEMA_MAP_PATH", "/app/config/gwl_schema_map.yaml")

	mapper, err := service.NewSchemaMapper(schemaMapPath, logger)
	if err != nil {
		logger.Fatal("Failed to load schema mapper", zap.Error(err))
	}

	cdcSvc := service.NewCDCSyncService(mapper, logger)

	app := fiber.New(fiber.Config{AppName: "GN-WAAS CDC Ingestor v1.0"})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"service": "cdc-ingestor", "status": "healthy"})
	})

	app.Post("/api/v1/cdc/sync/:type", func(c *fiber.Ctx) error {
		syncType := c.Params("type")
		ctx, cancel := context.WithTimeout(c.Context(), 5*time.Minute)
		defer cancel()

		status, err := cdcSvc.RunSync(ctx, syncType)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(status)
	})

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
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = app.ShutdownWithContext(shutdownCtx)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func init() {
	_ = fmt.Sprintf // suppress unused import
}
