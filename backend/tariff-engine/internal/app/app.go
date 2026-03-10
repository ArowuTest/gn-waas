package app

import (
	"context"
	"time"
	"fmt"

	"github.com/ArowuTest/gn-waas/backend/tariff-engine/internal/config"
	"github.com/ArowuTest/gn-waas/backend/tariff-engine/internal/handler"
	"github.com/ArowuTest/gn-waas/backend/tariff-engine/internal/repository/postgres"
	"github.com/ArowuTest/gn-waas/backend/tariff-engine/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// App holds all application dependencies
type App struct {
	cfg    *config.Config
	db     *pgxpool.Pool
	server *fiber.App
	logger *zap.Logger
}

// New creates and wires all application dependencies
func New(cfg *config.Config, db *pgxpool.Pool, logger *zap.Logger) *App {
	// Repositories
	tariffRepo := postgres.NewTariffRateRepository(db, logger)
	vatRepo := postgres.NewVATConfigRepository(db, logger)
	shadowRepo := postgres.NewShadowBillRepository(db, logger)
	configRepo := postgres.NewSystemConfigRepository(db, logger)

	// Services
	tariffSvc := service.NewTariffService(tariffRepo, vatRepo, shadowRepo, configRepo, logger)

	// Handlers
	tariffHandler := handler.NewTariffHandler(tariffSvc, tariffRepo, vatRepo, shadowRepo, logger)

	// HTTP Server
	server := fiber.New(fiber.Config{
		AppName:      "GN-WAAS Tariff Engine v1.0",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "INTERNAL_ERROR", "message": err.Error()},
			})
		},
	})

	// Routes
	server.Get("/health", tariffHandler.HealthCheck)

	api := server.Group("/api/v1")
	tariff := api.Group("/tariff")
	tariff.Post("/calculate", tariffHandler.CalculateBill)
	tariff.Post("/calculate/batch", tariffHandler.CalculateBatch)
	tariff.Get("/rates", tariffHandler.GetTariffRates)
	tariff.Get("/rates/:category", tariffHandler.GetTariffRatesByCategory)
	tariff.Get("/vat", tariffHandler.GetVATConfig)
	tariff.Get("/variance/:district_id", tariffHandler.GetVarianceSummary)

	return &App{
		cfg:    cfg,
		db:     db,
		server: server,
		logger: logger,
	}
}

// Start starts the HTTP server
func (a *App) Start() error {
	addr := fmt.Sprintf(":%d", a.cfg.Server.Port)
	a.logger.Info("Starting tariff engine",
		zap.String("addr", addr),
		zap.String("env", a.cfg.App.Env),
	)
	return a.server.Listen(addr)
}

// Shutdown gracefully shuts down the application
func (a *App) Shutdown(ctx context.Context) error {
	a.logger.Info("Shutting down tariff engine")
	if err := a.server.ShutdownWithContext(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}
	a.db.Close()
	return nil
}
