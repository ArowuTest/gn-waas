package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ArowuTest/gn-waas/shared/go/middleware"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/config"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/handler"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/notification"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/rls"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/storage"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/repository"
	"github.com/ansrivas/fiberprometheus/v2"
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

	// ── Database (with startup retry) ────────────────────────────────────────
	// In Docker/K8s, the DB container may not be ready immediately.
	// Retry up to 5 times with exponential backoff before giving up.
	var db *pgxpool.Pool
	var err error
	for attempt := 1; attempt <= 5; attempt++ {
		db, err = pgxpool.New(ctx, cfg.Database.DSN())
		if err == nil {
			if pingErr := db.Ping(ctx); pingErr == nil {
				break
			} else {
				db.Close()
				err = pingErr
			}
		}
		if attempt == 5 {
			return nil, fmt.Errorf("database not ready after 5 attempts: %w", err)
		}
		backoff := time.Duration(1<<uint(attempt-1)) * time.Second // 1s, 2s, 4s, 8s
		logger.Warn("Database not ready, retrying",
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)
		time.Sleep(backoff)
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

	// ── SOS Notifier ─────────────────────────────────────────────────────────
	sosNotifier := notification.NewSOSNotifier(logger)

	// ── Evidence Storage (MinIO presigned URL service) ────────────────────────
	evidenceStorage, evidenceErr := storage.NewEvidenceStorageService(
		cfg.MinIO.Endpoint,
		cfg.MinIO.AccessKey,
		cfg.MinIO.SecretKey,
		cfg.MinIO.Bucket,
		cfg.MinIO.UseSSL,
		logger,
	)
	if evidenceErr != nil {
		logger.Warn("Evidence storage init failed — photos will not be persisted to MinIO",
			zap.Error(evidenceErr))
	}

	// ── Handlers ─────────────────────────────────────────────────────────────
	auditHandler    := handler.NewAuditHandler(auditRepo, fieldJobRepo, userRepo, logger)
	fieldJobHandler := handler.NewFieldJobHandler(fieldJobRepo, auditRepo, sosNotifier, evidenceStorage, logger)
	districtHandler := handler.NewDistrictHandler(districtRepo, logger)
	userHandler     := handler.NewUserHandler(userRepo, logger)
	configHandler   := handler.NewSystemConfigHandler(configRepo, logger)
	accountHandler  := handler.NewAccountHandler(accountRepo, logger)
	nrwHandler      := handler.NewNRWHandler(nrwRepo, logger)
	dataHandler     := handler.NewDataHandler(db, logger)
	flagRepo        := repository.NewAnomalyFlagRepository(db, logger)
	flagHandler     := handler.NewAnomalyFlagHandler(flagRepo, logger)
	gwlCaseRepo     := repository.NewGWLCaseRepository(db, logger)
	gwlHandler       := handler.NewGWLHandler(gwlCaseRepo, logger)
	reportHandler    := handler.NewReportHandler(gwlCaseRepo, logger)
	adminUserHandler := handler.NewAdminUserHandler(db, logger)
	healthHandler   := handler.NewHealthHandler(db)

	evidenceHandler := handler.NewEvidenceHandler(evidenceStorage, logger)

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
	// ── Prometheus metrics (/metrics — no auth required for scraping) ─────────
	prom := fiberprometheus.New("gnwaas_api_gateway")
	prom.RegisterAt(app, "/metrics")
	app.Use(prom.Middleware)

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
	app.Get("/ready", healthHandler.ReadinessCheck)

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
		// Dev mode: return mock token in standard APIResponse envelope
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"access_token":  "dev-mock-token-" + body.Email,
				"token_type":    "Bearer",
				"expires_in":    3600,
				"refresh_token": "dev-refresh-" + body.Email,
				"user": fiber.Map{
					"id":          "a0000001-0000-0000-0000-000000000001",
					"email":       body.Email,
					"full_name":   "Dev User",
					"role":        "SUPER_ADMIN",
					"district_id": "d0000001-0000-0000-0000-000000000001",
				},
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

	// ── Auth: Dev Login (DEV_MODE only) ─────────────────────────────────────
	// POST /api/v1/auth/dev-login — quick role-based login for local development
	// Returns a mock token that DevAuthMiddleware will accept.
	// BLOCKED in production (DEV_MODE=false).
	authGroup.Post("/dev-login", func(c *fiber.Ctx) error {
		if !cfg.Server.DevMode {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "dev-login is disabled in production",
			})
		}
		var body struct {
			Role  string `json:"role"`
			Email string `json:"email"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		if body.Role == "" {
			body.Role = "SUPER_ADMIN"
		}
		if body.Email == "" {
			body.Email = "dev-" + strings.ToLower(body.Role) + "@gnwaas.gov.gh"
		}
		// Return in the standard APIResponse envelope so frontend can use response.data
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"access_token":  "dev-mock-token-" + body.Email,
				"token_type":    "Bearer",
				"expires_in":    3600,
				"refresh_token": "dev-refresh-" + body.Email,
				"user": fiber.Map{
					"id":          "a0000001-0000-0000-0000-000000000001",
					"email":       body.Email,
					"full_name":   "Dev " + body.Role,
					"role":        body.Role,
					"district_id": "d0000001-0000-0000-0000-000000000001",
				},
			},
		})
	})

	// ── RLS Middleware ───────────────────────────────────────────────────────
	// Applied to ALL authenticated endpoints. For each request it:
	//   1. Reads JWT claims (district_id, user_role, user_id) from Fiber locals
	//   2. Begins a pgx transaction with SET LOCAL rls.* session variables
	//   3. Stores the transaction in the Go request context
	//   4. Commits on success (HTTP < 400), rolls back on error
	// Repositories retrieve the transaction via rls.TxFromContext and use it
	// for all queries, ensuring PostgreSQL RLS policies are enforced.
	rlsMW := rls.Middleware(db, logger)
	api := app.Group("/api/v1", authMW, rlsMW)

	// ── Districts ─────────────────────────────────────────────────────────────
	districts := api.Group("/districts")
	districts.Get("/", districtHandler.ListDistricts)
	districts.Get("/:id", districtHandler.GetDistrict)

	// ── Users ─────────────────────────────────────────────────────────────────
	users := api.Group("/users")
	users.Get("/me", userHandler.GetMe)
	users.Get("/field-officers",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "GWL_MANAGER", "FIELD_SUPERVISOR"),
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
		middleware.RequireRoles("SYSTEM_ADMIN", "FIELD_SUPERVISOR", "GWL_MANAGER"),
		auditHandler.CreateAuditEvent,
	)
	audits.Get("/", auditHandler.ListAuditEvents)
	audits.Get("/:id", auditHandler.GetAuditEvent)
	audits.Patch("/:id/assign",
		middleware.RequireRoles("SYSTEM_ADMIN", "FIELD_SUPERVISOR", "GWL_MANAGER"),
		auditHandler.AssignAuditEvent,
	)

	// ── Field Jobs ────────────────────────────────────────────────────────────
	fieldJobs := api.Group("/field-jobs")
	// Admin/supervisor: list all field jobs with optional status/alert_level/district_id filters.
	// Used by admin portal FieldJobsPage to display the full job queue.
	fieldJobs.Get("/",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_SUPERVISOR", "GWL_MANAGER", "GWL_EXECUTIVE"),
		fieldJobHandler.ListAllJobs,
	)
	// Admin/supervisor: create a new field job dispatch.
	fieldJobs.Post("/",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_SUPERVISOR", "GWL_MANAGER"),
		fieldJobHandler.CreateFieldJob,
	)
	fieldJobs.Get("/my-jobs",
		middleware.RequireRoles("FIELD_OFFICER"),
		fieldJobHandler.GetMyJobs,
	)
	// Admin/supervisor: assign a field officer to a job.
	// Used by admin portal FieldJobsPage assign-officer modal.
	fieldJobs.Patch("/:id/assign",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_SUPERVISOR", "GWL_MANAGER"),
		fieldJobHandler.AssignOfficer,
	)
	fieldJobs.Patch("/:id/status",
		middleware.RequireRoles("SUPER_ADMIN", "FIELD_OFFICER", "FIELD_SUPERVISOR"),
		fieldJobHandler.UpdateJobStatus,
	)
	fieldJobs.Post("/:id/sos",
		middleware.RequireRoles("FIELD_OFFICER"),
		fieldJobHandler.TriggerSOS,
	)
	// Core field-officer workflow: submit completed meter reading evidence.
	// Flutter's MeterCaptureScreen calls POST /field-jobs/:id/submit after
	// capturing photos, computing SHA-256 hashes, and uploading to MinIO.
	// The handler verifies hashes, updates job status, and writes the audit record.
	fieldJobs.Post("/:id/submit",
		middleware.RequireRoles("FIELD_OFFICER"),
		fieldJobHandler.SubmitJobEvidence,
	)
	// FIO-004: Illegal connection reporting — field officers submit evidence
	// of unauthorised water connections with GPS-locked location and SHA-256
	// hashed photo evidence.  Route must appear BEFORE /:id/* to avoid
	// the "illegal-connections" segment being parsed as a job UUID.
	fieldJobs.Post("/illegal-connections",
		middleware.RequireRoles("FIELD_OFFICER"),
		fieldJobHandler.ReportIllegalConnection,
	)

	// ── NRW Reports ───────────────────────────────────────────────────────────
	// Anomaly flags
	anomalyFlags := api.Group("/anomaly-flags",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_SUPERVISOR", "GWL_MANAGER", "GWL_EXECUTIVE"),
	)
	anomalyFlags.Get("/", flagHandler.ListAnomalyFlags)

	// ── Sentinel routes (admin portal compatibility) ───────────────────────────
	// The admin portal hooks use /sentinel/* paths — proxy to anomaly-flags handler
	sentinel := api.Group("/sentinel",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_SUPERVISOR", "GWL_MANAGER", "GWL_EXECUTIVE"),
	)
	sentinel.Get("/anomalies", flagHandler.ListAnomalyFlags)
	sentinel.Post("/anomalies", flagHandler.CreateAnomalyFlag)
	sentinel.Get("/anomalies/:id", flagHandler.GetAnomalyFlag)
	sentinel.Get("/summary/:district_id", flagHandler.GetDistrictSummary)
	sentinel.Post("/scan/:district_id",
		middleware.RequireRoles("SYSTEM_ADMIN", "FIELD_SUPERVISOR"),
		flagHandler.TriggerScan,
	)

	// ── OCR proxy (Flutter calls /ocr/process via api-gateway) ────────────────
	// Proxies to the ocr-service on port 3005 (or OCR_SERVICE_URL env var)
	ocr := api.Group("/ocr",
		middleware.RequireRoles("FIELD_OFFICER", "SYSTEM_ADMIN", "FIELD_SUPERVISOR"),
	)
	ocr.Post("/process", fieldJobHandler.ProxyOCRProcess)

	// ── Evidence Upload (presigned MinIO URLs for meter photos) ─────────────
	// Flutter calls POST /evidence/upload-url to get a presigned PUT URL,
	// uploads photo directly to MinIO, then submits the object_key with the job.
	evidence := api.Group("/evidence",
		middleware.RequireRoles("FIELD_OFFICER", "FIELD_SUPERVISOR", "SYSTEM_ADMIN"),
	)
	evidence.Post("/upload-url", evidenceHandler.GetUploadURL)
	evidence.Post("/verify-hash", evidenceHandler.VerifyPhotoHash)
	// Wildcard route for nested object keys: GET /evidence/evidence/jobid/ts_file.jpg/url
	// M2 fix: restrict download URL generation to roles that legitimately need
	// to view evidence (supervisors, managers, admins).  Field officers upload
	// evidence but do not need to generate download links for arbitrary objects.
	// This enforces least-privilege and prevents cross-district evidence access.
	api.Get("/evidence/*",
		middleware.RequireRoles(
			"SYSTEM_ADMIN", "MOF_AUDITOR",
			"FIELD_SUPERVISOR", "GWL_MANAGER", "GWL_EXECUTIVE",
		),
		evidenceHandler.GetDownloadURL,
	)

	reports := api.Group("/reports")
	reports.Get("/nrw",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "MOF_AUDITOR", "GWL_MANAGER", "GWL_EXECUTIVE", "FIELD_SUPERVISOR"),
		nrwHandler.GetNRWSummary)
	reports.Get("/nrw/my-district",
		middleware.RequireRoles("FIELD_OFFICER", "GWL_MANAGER", "FIELD_SUPERVISOR"),
		nrwHandler.GetMyDistrictSummary,
	)
	reports.Get("/nrw/:district_id/trend", nrwHandler.GetDistrictNRWTrend)

	// ── Admin User Management (SYSTEM_ADMIN only) ───────────────────────────
	adminUsers := api.Group("/admin/users",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN"),
	)
	adminUsers.Get("/", adminUserHandler.ListUsers)
	adminUsers.Post("/", adminUserHandler.CreateUser)
	adminUsers.Patch("/:id", adminUserHandler.UpdateUser)
	adminUsers.Post("/:id/reset-password", adminUserHandler.ResetPassword)

	// ── Admin District Management (SYSTEM_ADMIN only) ───────────────────────
	adminDistricts := api.Group("/admin/districts",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN"),
	)
	adminDistricts.Post("/", districtHandler.CreateDistrict)
	adminDistricts.Patch("/:id", districtHandler.UpdateDistrict)

	// ── System Config (admin only) ────────────────────────────────────────────
	sysConfig := api.Group("/config",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN"),
	)
	sysConfig.Get("/:category", configHandler.GetConfigByCategory)
	sysConfig.Patch("/:key", configHandler.UpdateConfig)

	// ── GWL Case Management Portal ───────────────────────────────────────────
	// All GWL routes require GWL_SUPERVISOR, GWL_BILLING_OFFICER, or GWL_MANAGER role
	gwlRoles := middleware.RequireRoles("SUPER_ADMIN", "GWL_MANAGER", "GWL_SUPERVISOR", "GWL_ANALYST", "GWL_EXECUTIVE", "SYSTEM_ADMIN")
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

	// ── Core Data Endpoints (BE-7) ───────────────────────────────────────────
	// Read-only access to production, meter-reading, water-balance, billing data.
	// Accessible by SUPER_ADMIN, SYSTEM_ADMIN, MOF_AUDITOR, GWL roles.
	dataRoles := middleware.RequireRoles(
		"SUPER_ADMIN", "SYSTEM_ADMIN", "MOF_AUDITOR",
		"GWL_EXECUTIVE", "GWL_MANAGER", "GWL_SUPERVISOR", "GWL_ANALYST",
	)
	api.Get("/production-records", dataRoles, dataHandler.ListProductionRecords)
	api.Get("/meter-readings",     dataRoles, dataHandler.ListMeterReadings)
	api.Get("/water-balance",      dataRoles, dataHandler.ListWaterBalance)
	api.Get("/billing-records",    dataRoles, dataHandler.ListBillingRecords)

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
