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
	"github.com/gofiber/fiber/v2/middleware/limiter"
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

	// ── Database ──────────────────────────────────────────────────────────────
	db, err := pgxpool.New(ctx, cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to create db pool: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}
	logger.Info("Database connected", zap.String("host", cfg.Database.Host))

	// ── Repositories ─────────────────────────────────────────────────────────
	auditRepo    := repository.NewAuditEventRepository(db, logger)
	fieldJobRepo := repository.NewFieldJobRepository(db, logger)
	userRepo     := repository.NewUserRepository(db, logger)
	districtRepo := repository.NewDistrictRepository(db, logger)
	configRepo   := repository.NewSystemConfigRepository(db, logger)
	accountRepo  := repository.NewAccountRepository(db, logger)
	nrwRepo      := repository.NewNRWReportRepository(db, logger)

	// ── Handlers ─────────────────────────────────────────────────────────────
	auditHandler    := handler.NewAuditHandler(auditRepo, fieldJobRepo, userRepo, logger)
	fieldJobHandler := handler.NewFieldJobHandler(fieldJobRepo, auditRepo, logger)
	districtHandler := handler.NewDistrictHandler(districtRepo, logger)
	userHandler     := handler.NewUserHandler(userRepo, logger)
	configHandler   := handler.NewSystemConfigHandler(configRepo, logger)
	accountHandler  := handler.NewAccountHandler(accountRepo, logger)
	nrwHandler      := handler.NewNRWHandler(nrwRepo, logger)
	flagRepo        := repository.NewAnomalyFlagRepository(db, logger)
	flagHandler     := handler.NewAnomalyFlagHandler(flagRepo, logger)
	gwlCaseRepo     := repository.NewGWLCaseRepository(db, logger)
	gwlHandler       := handler.NewGWLHandler(gwlCaseRepo, logger)
	reportHandler    := handler.NewReportHandler(gwlCaseRepo, logger)
	adminUserHandler := handler.NewAdminUserHandler(db, logger)
	healthHandler   := handler.NewHealthHandler()

	// ── Fiber app ─────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName:      "GN-WAAS API Gateway v1.0",
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSec) * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger.Error("Unhandled error", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "Internal server error"})
		},
	})

	// ── Global middleware ─────────────────────────────────────────────────────
	app.Use(middleware.RecoverMiddleware(logger))
	app.Use(middleware.RequestLogger(logger))
	app.Use(middleware.CORS())
	app.Use(middleware.SecurityHeaders())

	// ── Rate limiting ─────────────────────────────────────────────────────────
	// Global rate limit: 300 requests/minute per IP (protects all endpoints)
	app.Use(limiter.New(limiter.Config{
		Max:        300,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Rate limit exceeded. Please slow down.",
				"retry_after": "60s",
			})
		},
	}))

	// ── Health check (no auth) ────────────────────────────────────────────────
	app.Get("/health", healthHandler.HealthCheck)

	// ── Mobile app config (no auth — needed before login) ────────────────────
	// Returns field.* system_config values that control mobile app behaviour.
	// Admin changes these via the admin portal Settings → Mobile App page.
	// Values are read live from system_config table so admin changes take effect immediately.
	app.Get("/api/v1/config/mobile", func(c *fiber.Ctx) error {
		ctx := c.Context()

		// Helper: read float with default
		getFloat := func(key string, def float64) float64 {
			v, _ := configRepo.GetFloat64(ctx, key, def)
			return v
		}
		getBool := func(key string, def bool) bool {
			s, _ := configRepo.GetString(ctx, key, fmt.Sprintf("%v", def))
			return s == "true" || s == "1"
		}
		getString := func(key string, def string) string {
			s, _ := configRepo.GetString(ctx, key, def)
			return s
		}
		getInt := func(key string, def int) int {
			v, _ := configRepo.GetFloat64(ctx, key, float64(def))
			return int(v)
		}

		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"geofence_radius_m":          getFloat("field.gps_fence_radius_m", 100.0),
				"require_biometric":          getBool("field.require_biometric", true),
				"blind_audit_default":        getBool("field.blind_audit_default", true),
				"require_surroundings_photo": getBool("field.require_surroundings_photo", true),
				"max_photo_age_minutes":      getInt("field.max_photo_age_minutes", 5),
				"ocr_conflict_tolerance_pct": getFloat("field.ocr_conflict_tolerance_pct", 2.0),
				"sync_interval_seconds":      getInt("field.sync_interval_seconds", 30),
				"max_jobs_per_officer":       getInt("audit.max_jobs_per_officer", 5),
				"app_min_version":            getString("mobile.app_min_version", "1.0.0"),
				"app_latest_version":         getString("mobile.app_latest_version", "1.0.0"),
				"force_update":               getBool("mobile.force_update", false),
				"maintenance_mode":           getBool("mobile.maintenance_mode", false),
				"maintenance_message":        getString("mobile.maintenance_message", ""),
			},
		})
	})

	// ── Auth middleware config ────────────────────────────────────────────────
	authCfg := middleware.AuthConfig{
		KeycloakURL: cfg.Keycloak.URL,
		Realm:       cfg.Keycloak.Realm,
		ClientID:    cfg.Keycloak.ClientID,
	}

	// In development mode, use DevAuthMiddleware to bypass JWT validation.
	// In production, AuthMiddleware fetches JWKS from Keycloak and validates RS256 tokens.
	var authMW fiber.Handler
	if cfg.Server.DevMode {
		logger.Warn("DEVELOPMENT MODE: JWT validation bypassed — DO NOT USE IN PRODUCTION")
		authMW = middleware.DevAuthMiddleware(logger)
	} else {
		authMW = middleware.AuthMiddleware(authCfg, logger)
	}

	// ── API v1 routes ─────────────────────────────────────────────────────────
	// ── Auth endpoints (no JWT required) ─────────────────────────────────────
	authGroup := app.Group("/api/v1/auth")
	// Strict rate limit on login: 10 attempts/minute per IP (brute-force protection)
	authGroup.Use(limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many login attempts. Please wait 60 seconds before trying again.",
				"retry_after": "60s",
			})
		},
	}))
	authGroup.Post("/login", func(c *fiber.Ctx) error {
		// In production: redirect to Keycloak OIDC
		// In DEV_MODE: return a mock token for testing
		if !cfg.Server.DevMode {
			return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
				"error": "Use Keycloak OIDC for authentication in production",
				"keycloak_url": cfg.Keycloak.URL,
			})
		}
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		if body.Email == "" || body.Password == "" {
			return c.Status(400).JSON(fiber.Map{"error": "email and password required"})
		}
		// Dev mode: return mock token
		return c.JSON(fiber.Map{
			"access_token": "dev-mock-token-" + body.Email,
			"token_type":   "Bearer",
			"expires_in":   3600,
			"user": fiber.Map{
				"email": body.Email,
				"role":  "AUDIT_SUPERVISOR",
				"name":  "Dev User",
			},
		})
	})

	// ── Auth: Refresh Token ───────────────────────────────────────────────────
	// POST /api/v1/auth/refresh — exchange refresh token for new access token
	// In production: proxied to Keycloak token endpoint
	// In dev mode: returns a new mock token
	authGroup.Post("/refresh", func(c *fiber.Ctx) error {
		if !cfg.Server.DevMode {
			// Production: proxy to Keycloak token endpoint
			return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
				"error": "Use Keycloak OIDC token refresh in production",
				"keycloak_token_url": cfg.Keycloak.URL + "/realms/" + cfg.Keycloak.Realm + "/protocol/openid-connect/token",
			})
		}
		var body struct {
			RefreshToken string `json:"refresh_token"`
			GrantType    string `json:"grant_type"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		if body.RefreshToken == "" {
			return c.Status(400).JSON(fiber.Map{"error": "refresh_token required"})
		}
		// Dev mode: validate mock refresh token and return new access token
		if len(body.RefreshToken) < 10 {
			return c.Status(401).JSON(fiber.Map{"error": "invalid or expired refresh token"})
		}
		return c.JSON(fiber.Map{
			"access_token":  "dev-refreshed-token-" + body.RefreshToken[:8],
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "dev-refresh-" + body.RefreshToken[:8] + "-new",
			"user": fiber.Map{
				"id":         "a0000001-0000-0000-0000-000000000001",
				"email":      "officer@gnwaas.gov.gh",
				"full_name":  "Dev Field Officer",
				"role":       "FIELD_OFFICER",
				"district_id": "d0000001-0000-0000-0000-000000000001",
			},
		})
	})

	api := app.Group("/api/v1", authMW)

	// ── Districts ─────────────────────────────────────────────────────────────
	districts := api.Group("/districts")
	districts.Get("/", districtHandler.ListDistricts)
	districts.Get("/:id", districtHandler.GetDistrict)

	// ── Users ─────────────────────────────────────────────────────────────────
	users := api.Group("/users")
	users.Get("/me", userHandler.GetMe)
	users.Get("/field-officers",
		middleware.RequireRoles("SYSTEM_ADMIN", "DISTRICT_MANAGER", "AUDIT_SUPERVISOR"),
		userHandler.GetFieldOfficers,
	)

	// ── Water Accounts ────────────────────────────────────────────────────────
	accounts := api.Group("/accounts")
	accounts.Get("/search", accountHandler.SearchAccounts)
	accounts.Get("/", accountHandler.GetAccountsByDistrict)
	accounts.Get("/:id", accountHandler.GetAccount)

	// ── Audit Events ──────────────────────────────────────────────────────────
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

	// ── Field Jobs ────────────────────────────────────────────────────────────
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

	// ── NRW Reports ───────────────────────────────────────────────────────────
	// Anomaly flags
	anomalyFlags := api.Group("/anomaly-flags")
	anomalyFlags.Get("/", flagHandler.ListAnomalyFlags)

	reports := api.Group("/reports")
	reports.Get("/nrw", nrwHandler.GetNRWSummary)
	reports.Get("/nrw/my-district",
		middleware.RequireRoles("FIELD_OFFICER", "DISTRICT_MANAGER", "AUDIT_SUPERVISOR"),
		nrwHandler.GetMyDistrictSummary,
	)
	reports.Get("/nrw/:district_id/trend", nrwHandler.GetDistrictNRWTrend)

	// ── Admin User Management (SYSTEM_ADMIN only) ───────────────────────────
	adminUsers := api.Group("/admin/users",
		middleware.RequireRoles("SYSTEM_ADMIN"),
	)
	adminUsers.Get("/", adminUserHandler.ListUsers)
	adminUsers.Post("/", adminUserHandler.CreateUser)
	adminUsers.Patch("/:id", adminUserHandler.UpdateUser)
	adminUsers.Post("/:id/reset-password", adminUserHandler.ResetPassword)

	// ── Admin District Management (SYSTEM_ADMIN only) ───────────────────────
	adminDistricts := api.Group("/admin/districts",
		middleware.RequireRoles("SYSTEM_ADMIN"),
	)
	adminDistricts.Post("/", districtHandler.CreateDistrict)
	adminDistricts.Patch("/:id", districtHandler.UpdateDistrict)

	// ── System Config (admin only) ────────────────────────────────────────────
	sysConfig := api.Group("/config",
		middleware.RequireRoles("SYSTEM_ADMIN"),
	)
	sysConfig.Get("/:category", configHandler.GetConfigByCategory)
	sysConfig.Patch("/:key", configHandler.UpdateConfig)

	// ── GWL Case Management Portal ───────────────────────────────────────────
	// All GWL routes require GWL_SUPERVISOR, GWL_BILLING_OFFICER, or GWL_MANAGER role
	gwlRoles := middleware.RequireRoles("GWL_SUPERVISOR", "GWL_BILLING_OFFICER", "GWL_MANAGER", "SYSTEM_ADMIN")
	gwl := api.Group("/gwl", gwlRoles)

	// Case queue and summary
	gwl.Get("/cases/summary", gwlHandler.GetCaseSummary)
	gwl.Get("/cases", gwlHandler.ListCases)
	gwl.Get("/cases/:id", gwlHandler.GetCase)
	gwl.Get("/cases/:id/actions", gwlHandler.GetCaseActions)

	// Case workflow actions
	gwl.Post("/cases/:id/assign", gwlHandler.AssignToFieldOfficer)
	gwl.Patch("/cases/:id/status", gwlHandler.UpdateCaseStatus)
	gwl.Post("/cases/:id/reclassify", gwlHandler.RequestReclassification)
	gwl.Post("/cases/:id/credit", gwlHandler.RequestCredit)

	// Reclassification and credit management
	gwl.Get("/reclassifications", gwlHandler.ListReclassifications)
	gwl.Get("/credits", gwlHandler.ListCredits)

	// Monthly reports
	gwl.Get("/reports/monthly", gwlHandler.GetMonthlyReport)

	// ── Report export endpoints (server-generated PDF + CSV) ──────────────────
	reports.Get("/monthly/pdf", reportHandler.GetMonthlyReportPDF)
	reports.Get("/monthly/csv", reportHandler.GetMonthlyReportCSV)

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
