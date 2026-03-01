package app

import (
	"context"
	"fmt"
	"time"

	"github.com/ArowuTest/gn-waas/shared/go/middleware"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/config"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/handler"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/repository"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// App holds all application dependencies
type App struct {
	fiber  *fiber.App
	db     *pgxpool.Pool
	cfg    *config.Config
	logger *zap.Logger
}

// New creates and wires the entire application
func New(cfg *config.Config, logger *zap.Logger) (*App, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Database
	db, err := pgxpool.New(ctx, cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to create db pool: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}
	logger.Info("Database connected", zap.String("host", cfg.Database.Host))

	// Repositories
	auditRepo := repository.NewAuditEventRepository(db, logger)
	fieldJobRepo := repository.NewFieldJobRepository(db, logger)
	userRepo := repository.NewUserRepository(db, logger)
	districtRepo := repository.NewDistrictRepository(db, logger)
	configRepo := repository.NewSystemConfigRepository(db, logger)

	// Handlers
	auditHandler := handler.NewAuditHandler(auditRepo, fieldJobRepo, userRepo, logger)
	fieldJobHandler := handler.NewFieldJobHandler(fieldJobRepo, auditRepo, logger)
	districtHandler := handler.NewDistrictHandler(districtRepo, logger)
	userHandler := handler.NewUserHandler(userRepo, logger)
	configHandler := handler.NewSystemConfigHandler(configRepo, logger)
	healthHandler := handler.NewHealthHandler()

	// Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "GN-WAAS API Gateway v1.0",
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSec) * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger.Error("Unhandled error", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
		},
	})

	// Global middleware
	app.Use(middleware.RecoverMiddleware(logger))
	app.Use(middleware.RequestLogger(logger))
	app.Use(middleware.CORS())
	app.Use(middleware.SecurityHeaders())

	// Health check (no auth)
	app.Get("/health", healthHandler.HealthCheck)

	// Auth middleware config
	authCfg := middleware.AuthConfig{
		KeycloakURL:   cfg.Keycloak.URL,
		Realm:         cfg.Keycloak.Realm,
		ClientID:      cfg.Keycloak.ClientID,
		
	}

	// API v1 routes
	api := app.Group("/api/v1", middleware.AuthMiddleware(authCfg, logger))

	// Districts
	districts := api.Group("/districts")
	districts.Get("/", districtHandler.ListDistricts)
	districts.Get("/:id", districtHandler.GetDistrict)

	// Users
	users := api.Group("/users")
	users.Get("/me", userHandler.GetMe)
	users.Get("/field-officers",
		middleware.RequireRoles("SYSTEM_ADMIN", "DISTRICT_MANAGER", "AUDIT_SUPERVISOR"),
		userHandler.GetFieldOfficers,
	)

	// Audit Events
	audits := api.Group("/audits")
	audits.Get("/dashboard", auditHandler.GetDashboardStats)
	audits.Post("/",
		middleware.RequireRoles("SYSTEM_ADMIN", "AUDIT_SUPERVISOR", "DISTRICT_MANAGER"),
		auditHandler.CreateAuditEvent,
	)
	audits.Get("/", auditHandler.ListAuditEvents)
	audits.Get("/:id", auditHandler.GetAuditEvent)
	audits.Patch("/:id/assign",
		middleware.RequireRoles("SYSTEM_ADMIN", "AUDIT_SUPERVISOR", "DISTRICT_MANAGER"),
		auditHandler.AssignAuditEvent,
	)

	// Field Jobs
	fieldJobs := api.Group("/field-jobs")
	fieldJobs.Get("/my-jobs",
		middleware.RequireRoles("FIELD_OFFICER"),
		fieldJobHandler.GetMyJobs,
	)
	fieldJobs.Patch("/:id/status",
		middleware.RequireRoles("FIELD_OFFICER", "AUDIT_SUPERVISOR"),
		fieldJobHandler.UpdateJobStatus,
	)
	fieldJobs.Post("/:id/sos",
		middleware.RequireRoles("FIELD_OFFICER"),
		fieldJobHandler.TriggerSOS,
	)

	// System Config (admin only)
	sysConfig := api.Group("/config",
		middleware.RequireRoles("SYSTEM_ADMIN"),
	)
	sysConfig.Get("/:category", configHandler.GetConfigByCategory)
	sysConfig.Patch("/:key", configHandler.UpdateConfig)

	return &App{
		fiber:  app,
		db:     db,
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Start starts the HTTP server
func (a *App) Start() error {
	port := fmt.Sprintf(":%d", a.cfg.Server.Port)
	a.logger.Info("API Gateway starting", zap.String("port", port))
	return a.fiber.Listen(port)
}

// Shutdown gracefully shuts down the application
func (a *App) Shutdown(ctx context.Context) error {
	a.logger.Info("Shutting down API Gateway")
	if err := a.fiber.ShutdownWithContext(ctx); err != nil {
		return err
	}
	a.db.Close()
	return nil
}
