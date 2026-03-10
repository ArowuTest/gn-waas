package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ArowuTest/gn-waas/shared/go/middleware"
	"github.com/ArowuTest/gn-waas/shared/go/http/response"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/cache"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/config"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/handler"
	gwsvc "github.com/ArowuTest/gn-waas/backend/api-gateway/internal/service"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/notification"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/rls"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/storage"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/repository"
	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
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

	// ── Redis Cache ───────────────────────────────────────────────────────────
	redisAddr := cfg.Redis.Addr()
	cacheClient := cache.NewClient(redisAddr, cfg.Redis.Password, cfg.Redis.DB, logger)

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
	flagRepo        := repository.NewAnomalyFlagRepository(db, logger)
	auditHandler    := handler.NewAuditHandler(auditRepo, fieldJobRepo, userRepo, logger)
	fieldJobHandler := handler.NewFieldJobHandler(fieldJobRepo, flagRepo, auditRepo, sosNotifier, evidenceStorage, logger)
	districtHandler := handler.NewDistrictHandler(districtRepo, cacheClient, logger)
	userHandler     := handler.NewUserHandler(userRepo, logger)
	configHandler   := handler.NewSystemConfigHandler(configRepo, logger)
	accountHandler  := handler.NewAccountHandler(accountRepo, logger)
	nrwHandler      := handler.NewNRWHandler(nrwRepo, logger)
	dataHandler     := handler.NewDataHandler(db, logger)
	flagHandler     := handler.NewAnomalyFlagHandler(flagRepo, logger)
	gwlCaseRepo     := repository.NewGWLCaseRepository(db, logger)
	gwlHandler       := handler.NewGWLHandler(gwlCaseRepo, logger)
	reportHandler    := handler.NewReportHandler(gwlCaseRepo, auditRepo, fieldJobRepo, logger)
	adminUserHandler := handler.NewAdminUserHandler(db, logger)
	healthHandler   := handler.NewHealthHandler(db)

	evidenceHandler    := handler.NewEvidenceHandler(evidenceStorage, logger)
	revenueHandler    := handler.NewRevenueRecoveryHandler(db, logger)
	workforceHandler  := handler.NewWorkforceHandler(db, logger)

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
		// BUG-V30-04 fix: DevAuthMiddleware calls c.Next() internally. Wrapping it and
		// calling c.Next() again causes a double-advance of the Fiber middleware chain,
		// resulting in 404 for all authenticated routes. Fix: inline the dev auth setup
		// directly in a single middleware that calls c.Next() exactly once.
		authMW = func(c *fiber.Ctx) error {
			// BUG-RLS-01 fix: Parse the actual user email from the dev token so that
			// each user gets their real district_id and user_id from the database,
			// not a hardcoded first-district fallback that breaks RLS isolation.
			//
			// Token format: "dev-mock-token-{email}"
			// e.g. "dev-mock-token-officer.kwame@gnwaas.gov.gh"
			authHeader := c.Get("Authorization")
			tokenEmail := ""
			const devPrefix = "dev-mock-token-"
			if strings.HasPrefix(authHeader, "Bearer "+devPrefix) {
				tokenEmail = strings.TrimPrefix(authHeader, "Bearer "+devPrefix)
			}

			// Default role/identity for unauthenticated or generic dev requests
			devRole := c.Get("X-Dev-Role", "SUPER_ADMIN")
			devEmail := "dev-" + strings.ToLower(devRole) + "@gnwaas.gov.gh"
			devName := "Dev User (" + devRole + ")"
			devUserID := "a0000001-0000-0000-0000-000000000001"
			devDistrictID := "00000000-0000-0000-0000-000000000000"

			// If a real user email is embedded in the token, look them up
			if tokenEmail != "" {
				// BUG-RLS-05: The users table has RLS enabled. The gnwaas app user
				// is not a superuser, so a plain pool.QueryRow() returns 0 rows.
				// Fix: run the lookup inside a transaction with SET LOCAL ROLE gnwaas_app
				// and app.user_role = 'SUPER_ADMIN' so the RLS policy allows the read.
				var dbUserID, dbRole, dbDistrictID string
				authErr := func() error {
					tx, txErr := db.Begin(c.Context())
					if txErr != nil {
						return txErr
					}
					defer tx.Rollback(c.Context())
					if _, txErr = tx.Exec(c.Context(), "SET LOCAL ROLE gnwaas_app"); txErr != nil {
						return txErr
					}
					if _, txErr = tx.Exec(c.Context(), "SELECT set_config('app.user_role', 'SUPER_ADMIN', true)"); txErr != nil {
						return txErr
					}
					if _, txErr = tx.Exec(c.Context(), "SELECT set_config('app.district_id', '00000000-0000-0000-0000-000000000000', true)"); txErr != nil {
						return txErr
					}
					if _, txErr = tx.Exec(c.Context(), "SELECT set_config('app.user_id', '00000000-0000-0000-0000-000000000000', true)"); txErr != nil {
						return txErr
					}
					return tx.QueryRow(c.Context(),
						`SELECT id::text, role::text, COALESCE(district_id::text, '00000000-0000-0000-0000-000000000000')
						 FROM users WHERE email = $1 AND status = 'ACTIVE' LIMIT 1`,
						tokenEmail,
					).Scan(&dbUserID, &dbRole, &dbDistrictID)
				}()
				if authErr == nil {
					devEmail = tokenEmail
					devRole = dbRole
					devUserID = dbUserID
					devDistrictID = dbDistrictID
					devName = tokenEmail
				}
			}

			// Admin/global roles bypass district RLS (zero UUID = no district filter)
			adminRoles := map[string]bool{
				"SUPER_ADMIN":  true,
				"SYSTEM_ADMIN": true,
				"MOF_AUDITOR":  true,
			}
			if adminRoles[devRole] {
				devDistrictID = "00000000-0000-0000-0000-000000000000"
			}

			c.Locals("user_id", devUserID)
			c.Locals("user_email", devEmail)
			c.Locals("user_name", devName)
			c.Locals("user_roles", []string{devRole})
			c.Locals("claims", &middleware.Claims{
				Sub:   devUserID,
				Email: devEmail,
				Name:  devName,
				RealmAccess: middleware.RealmAccess{
					Roles: []string{devRole},
				},
			})
			c.Locals("rls_user_role", devRole)
			c.Locals("rls_user_id", devUserID)
			c.Locals("rls_district_id", devDistrictID)

			return c.Next()
		}
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
		// POST /api/v1/auth/login
		// Accepts email + password, validates against bcrypt hash in users table.
		// Works in both DEV_MODE and production (no Keycloak dependency).
		// In true production, Keycloak OIDC is the primary IdP; this endpoint
		// serves as a fallback for demo/staging environments.
		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"code": "BAD_REQUEST", "message": "invalid request body"}})
		}
		if body.Email == "" || body.Password == "" {
			return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"code": "BAD_REQUEST", "message": "email and password are required"}})
		}

		// Look up user by email — run in a SUPER_ADMIN RLS context so we can
		// read the password_hash regardless of the user's own district scope.
		var (
			userID         string
			fullName       string
			role           string
			districtID     string
			passwordHash   string
			status         string
		)
		err := func() error {
			tx, err := db.Begin(c.Context())
			if err != nil { return err }
			defer tx.Rollback(c.Context())
			if _, err = tx.Exec(c.Context(), "SET LOCAL ROLE gnwaas_app"); err != nil { return err }
			if _, err = tx.Exec(c.Context(), "SELECT set_config('app.user_role','SUPER_ADMIN',true)"); err != nil { return err }
			if _, err = tx.Exec(c.Context(), "SELECT set_config('app.district_id','00000000-0000-0000-0000-000000000000',true)"); err != nil { return err }
			if _, err = tx.Exec(c.Context(), "SELECT set_config('app.user_id','00000000-0000-0000-0000-000000000000',true)"); err != nil { return err }
			return tx.QueryRow(c.Context(),
				`SELECT id::text, full_name, role::text,
				        COALESCE(district_id::text,'00000000-0000-0000-0000-000000000000'),
				        COALESCE(password_hash,''),
				        status::text
				 FROM users WHERE email = $1 LIMIT 1`,
				body.Email,
			).Scan(&userID, &fullName, &role, &districtID, &passwordHash, &status)
		}()

		if err != nil {
			logger.Warn("login: user not found", zap.String("email", body.Email), zap.Error(err))
			return c.Status(401).JSON(fiber.Map{"success": false, "error": fiber.Map{"code": "UNAUTHORIZED", "message": "Invalid email or password"}})
		}

		if status != "ACTIVE" {
			return c.Status(403).JSON(fiber.Map{"success": false, "error": fiber.Map{"code": "FORBIDDEN", "message": "Account is not active"}})
		}

		// Validate password
		if passwordHash == "" {
			// No password set — only dev-login is available for this user
			return c.Status(401).JSON(fiber.Map{"success": false, "error": fiber.Map{"code": "UNAUTHORIZED", "message": "Password login not configured for this account. Use dev-login."}})
		}
		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(body.Password)); err != nil {
			logger.Warn("login: wrong password", zap.String("email", body.Email))
			return c.Status(401).JSON(fiber.Map{"success": false, "error": fiber.Map{"code": "UNAUTHORIZED", "message": "Invalid email or password"}})
		}

		// Update last_login_at
		_, _ = db.Exec(c.Context(),
			`UPDATE users SET last_login_at = NOW(), failed_login_count = 0 WHERE id = $1`,
			userID,
		)

		// Issue token (same format as dev-login so frontend works identically)
		token := "dev-mock-token-" + body.Email
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"access_token":  token,
				"token_type":    "Bearer",
				"expires_in":    86400,
				"refresh_token": "refresh-" + body.Email,
				"user": fiber.Map{
					"id":          userID,
					"email":       body.Email,
					"full_name":   fullName,
					"role":        role,
					"district_id": districtID,
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
		// BUG-V30-04 fix: look up the real user from DB so district_id is correct.
		// Run inside a transaction with SUPER_ADMIN context to bypass RLS.
		var realID, realName, realRole, realDistrictID string
		realID = "a0000001-0000-0000-0000-000000000001"
		realName = "Dev " + body.Role
		realRole = body.Role
		realDistrictID = "00000000-0000-0000-0000-000000000000"

		_ = func() error {
			tx, err := db.Begin(c.Context())
			if err != nil { return err }
			defer tx.Rollback(c.Context())
			if _, err = tx.Exec(c.Context(), "SET LOCAL ROLE gnwaas_app"); err != nil { return err }
			if _, err = tx.Exec(c.Context(), "SELECT set_config('app.user_role','SUPER_ADMIN',true)"); err != nil { return err }
			if _, err = tx.Exec(c.Context(), "SELECT set_config('app.district_id','00000000-0000-0000-0000-000000000000',true)"); err != nil { return err }
			if _, err = tx.Exec(c.Context(), "SELECT set_config('app.user_id','00000000-0000-0000-0000-000000000000',true)"); err != nil { return err }
			return tx.QueryRow(c.Context(),
				`SELECT id::text, full_name, role::text,
				        COALESCE(district_id::text,'00000000-0000-0000-0000-000000000000')
				 FROM users WHERE email=$1 AND status='ACTIVE' LIMIT 1`, body.Email,
			).Scan(&realID, &realName, &realRole, &realDistrictID)
		}()

		// Return in the standard APIResponse envelope so frontend can use response.data
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"access_token":  "dev-mock-token-" + body.Email,
				"token_type":    "Bearer",
				"expires_in":    3600,
				"refresh_token": "dev-refresh-" + body.Email,
				"user": fiber.Map{
					"id":          realID,
					"email":       body.Email,
					"full_name":   realName,
					"role":        realRole,
					"district_id": realDistrictID,
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
	// GET /districts/:id/heatmap — DMA anomaly heatmap data for a district
	// Returns district detail enriched with anomaly counts and NRW metrics
	// for rendering the DMA heatmap on the Operations Portal.
	districts.Get("/:id/heatmap", func(c *fiber.Ctx) error {
		districtID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return response.BadRequest(c, "INVALID_ID", "Invalid district ID")
		}
		tx, hasTx := rls.TxFromContext(c.UserContext())
		var q repository.Querier
		if hasTx {
			q = tx
		} else {
			q = db
		}
		var heatmap struct {
			DistrictID       string   `json:"district_id"`
			DistrictCode     string   `json:"district_code"`
			DistrictName     string   `json:"district_name"`
			Region           string   `json:"region"`
			ZoneType         string   `json:"zone_type"`
			LossRatioPct     *float64 `json:"loss_ratio_pct"`
			TotalConnections int      `json:"total_connections"`
			OpenAnomalies    int      `json:"open_anomalies"`
			CriticalFlags    int      `json:"critical_flags"`
			HighFlags        int      `json:"high_flags"`
			ActiveJobs       int      `json:"active_jobs"`
			NRWGrade         string   `json:"nrw_grade"`
		}
		err = q.QueryRow(c.UserContext(), `
			SELECT
				d.id::text,
				d.district_code,
				d.district_name,
				d.region,
				d.zone_type::text,
				d.loss_ratio_pct,
				d.total_connections,
				COALESCE((SELECT COUNT(*) FROM anomaly_flags af
				          WHERE af.district_id = d.id AND af.status = 'OPEN'), 0),
				COALESCE((SELECT COUNT(*) FROM anomaly_flags af
				          WHERE af.district_id = d.id AND af.status = 'OPEN'
				            AND af.alert_level = 'CRITICAL'), 0),
				COALESCE((SELECT COUNT(*) FROM anomaly_flags af
				          WHERE af.district_id = d.id AND af.status = 'OPEN'
				            AND af.alert_level = 'HIGH'), 0),
				COALESCE((SELECT COUNT(*) FROM field_jobs fj
				          WHERE fj.district_id = d.id
				            AND fj.status NOT IN ('COMPLETED','CANCELLED','FAILED')), 0),
				CASE
					WHEN d.loss_ratio_pct IS NULL THEN 'UNKNOWN'
					WHEN d.loss_ratio_pct <= 15   THEN 'A'
					WHEN d.loss_ratio_pct <= 25   THEN 'B'
					WHEN d.loss_ratio_pct <= 35   THEN 'C'
					WHEN d.loss_ratio_pct <= 50   THEN 'D'
					ELSE 'F'
				END
			FROM districts d
			WHERE d.id = $1
		`, districtID).Scan(
			&heatmap.DistrictID, &heatmap.DistrictCode, &heatmap.DistrictName,
			&heatmap.Region, &heatmap.ZoneType, &heatmap.LossRatioPct,
			&heatmap.TotalConnections, &heatmap.OpenAnomalies,
			&heatmap.CriticalFlags, &heatmap.HighFlags,
			&heatmap.ActiveJobs, &heatmap.NRWGrade,
		)
		if err != nil {
			logger.Error("District heatmap query failed", zap.Error(err), zap.String("district_id", districtID.String()))
			return response.NotFound(c, "District")
		}
		return response.OK(c, heatmap)
	})

	// ── Users ─────────────────────────────────────────────────────────────────
	users := api.Group("/users")
	users.Get("/me", userHandler.GetMe)
	users.Get("/field-officers",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "GWL_MANAGER", "FIELD_SUPERVISOR", "GRA_OFFICER"),
		userHandler.GetFieldOfficers,
	)

	// ── Water Accounts ────────────────────────────────────────────────────────
	accounts := api.Group("/accounts")
	accounts.Get("/search", accountHandler.SearchAccounts)
	accounts.Get("/", accountHandler.GetAccountsByDistrict)
	accounts.Get("/:id", accountHandler.GetAccount)
	// GET /accounts/:id/nrw — NRW summary for a specific water account
	// Returns meter readings, billing variance and loss ratio for the account.
	accounts.Get("/:id/nrw", func(c *fiber.Ctx) error {
		accountID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return response.BadRequest(c, "INVALID_ID", "Invalid account ID")
		}
		tx, hasTx := rls.TxFromContext(c.UserContext())
		var q repository.Querier
		if hasTx {
			q = tx
		} else {
			q = db
		}
		var result struct {
			AccountID        string   `json:"account_id"`
			GWLAccountNumber string   `json:"gwl_account_number"`
			AccountHolder    string   `json:"account_holder_name"`
			Category         string   `json:"category"`
			DistrictID       string   `json:"district_id"`
			LatestReadingM3  *float64 `json:"latest_reading_m3"`
			ShadowBillGHS    *float64 `json:"shadow_bill_ghs"`
			GWLBilledGHS     *float64 `json:"gwl_billed_ghs"`
			VarianceGHS      *float64 `json:"variance_ghs"`
			VariancePct      *float64 `json:"variance_pct"`
			AuditCount       int      `json:"audit_count"`
			OpenAnomalies    int      `json:"open_anomalies"`
		}
		err = q.QueryRow(c.UserContext(), `
			SELECT
				wa.id::text,
				COALESCE(wa.gwl_account_number, ''),
				COALESCE(wa.account_holder_name, ''),
				wa.category::text,
				wa.district_id::text,
				(SELECT mr.reading_m3 FROM meter_readings mr
				 WHERE mr.account_id = wa.id ORDER BY mr.reading_date DESC LIMIT 1),
				(SELECT ae.shadow_bill_ghs FROM audit_events ae
				 WHERE ae.account_id = wa.id ORDER BY ae.created_at DESC LIMIT 1),
				(SELECT ae.gwl_billed_ghs FROM audit_events ae
				 WHERE ae.account_id = wa.id ORDER BY ae.created_at DESC LIMIT 1),
				(SELECT ae.confirmed_loss_ghs FROM audit_events ae
				 WHERE ae.account_id = wa.id ORDER BY ae.created_at DESC LIMIT 1),
				(SELECT ae.variance_pct FROM audit_events ae
				 WHERE ae.account_id = wa.id ORDER BY ae.created_at DESC LIMIT 1),
				(SELECT COUNT(*) FROM audit_events ae WHERE ae.account_id = wa.id),
				(SELECT COUNT(*) FROM anomaly_flags af WHERE af.account_id = wa.id AND af.status = 'OPEN')
			FROM water_accounts wa
			WHERE wa.id = $1
		`, accountID).Scan(
			&result.AccountID, &result.GWLAccountNumber, &result.AccountHolder,
			&result.Category, &result.DistrictID,
			&result.LatestReadingM3, &result.ShadowBillGHS, &result.GWLBilledGHS,
			&result.VarianceGHS, &result.VariancePct,
			&result.AuditCount, &result.OpenAnomalies,
		)
		if err != nil {
			logger.Error("Account NRW query failed", zap.Error(err), zap.String("account_id", accountID.String()))
			return response.NotFound(c, "Account")
		}
		return response.OK(c, result)
	})

	// ── Audit Events ──────────────────────────────────────────────────────────
	audits := api.Group("/audits")
	audits.Get("/dashboard", auditHandler.GetDashboardStats)
	audits.Post("/",
		middleware.RequireRoles("SYSTEM_ADMIN", "FIELD_OFFICER", "FIELD_SUPERVISOR", "GWL_MANAGER"),
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
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_SUPERVISOR", "GWL_MANAGER", "GWL_EXECUTIVE", "GRA_OFFICER", "FIELD_OFFICER", "AUDIT_MANAGER"),
		fieldJobHandler.ListAllJobs,
	)
	// Admin/supervisor: create a new field job dispatch.
	fieldJobs.Post("/",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_SUPERVISOR", "GWL_MANAGER"),
		fieldJobHandler.CreateFieldJob,
	)
	// /my-jobs MUST be before /:id to prevent Fiber matching "my-jobs" as an ID param
	fieldJobs.Get("/my-jobs",
		middleware.RequireRoles("FIELD_OFFICER", "GRA_OFFICER"),
		fieldJobHandler.GetMyJobs,
	)
	fieldJobs.Get("/:id",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_SUPERVISOR", "GWL_MANAGER", "GWL_EXECUTIVE", "FIELD_OFFICER", "GRA_OFFICER", "MOF_AUDITOR"),
		fieldJobHandler.GetFieldJob,
	)
	// Admin/supervisor: assign a field officer to a job.
	// Used by admin portal FieldJobsPage assign-officer modal.
	fieldJobs.Patch("/:id/assign",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_SUPERVISOR", "GWL_MANAGER"),
		fieldJobHandler.AssignOfficer,
	)
	fieldJobs.Patch("/:id/status",
		middleware.RequireRoles("SUPER_ADMIN", "FIELD_OFFICER", "FIELD_SUPERVISOR", "GRA_OFFICER"),
		fieldJobHandler.UpdateJobStatus,
	)
	fieldJobs.Post("/:id/sos",
		middleware.RequireRoles("FIELD_OFFICER", "GRA_OFFICER"),
		fieldJobHandler.TriggerSOS,
	)
	// Core field-officer workflow: submit completed meter reading evidence.
	// Flutter's MeterCaptureScreen calls POST /field-jobs/:id/submit after
	// capturing photos, computing SHA-256 hashes, and uploading to MinIO.
	// The handler verifies hashes, updates job status, and writes the audit record.
	// PATCH /api/v1/field-jobs/:id/outcome
	// Records structured field officer outcome. Drives auto-escalation:
	//   METER_NOT_FOUND_INSTALL → UNMETERED_CONSUMPTION (revenue leakage)
	//   ADDRESS_INVALID         → FRAUDULENT_ACCOUNT (GWL internal fraud)
	//   METER_FOUND_OK          → dismiss ADDRESS_UNVERIFIED flag
	fieldJobs.Patch("/:id/outcome",
		middleware.RequireRoles("FIELD_OFFICER", "FIELD_SUPERVISOR", "SYSTEM_ADMIN", "GRA_OFFICER"),
		fieldJobHandler.RecordFieldJobOutcome,
	)
	fieldJobs.Post("/:id/submit",
		middleware.RequireRoles("FIELD_OFFICER", "GRA_OFFICER"),
		fieldJobHandler.SubmitJobEvidence,
	)
	// /evidence is the canonical REST name; /submit kept for backwards compat
	fieldJobs.Post("/:id/evidence",
		middleware.RequireRoles("FIELD_OFFICER", "GRA_OFFICER"),
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
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_SUPERVISOR", "GWL_MANAGER", "GWL_EXECUTIVE", "GRA_OFFICER"),
	)
	anomalyFlags.Get("/", flagHandler.ListAnomalyFlags)

	// ── Sentinel routes (admin portal compatibility) ───────────────────────────
	// The admin portal hooks use /sentinel/* paths — proxy to anomaly-flags handler
	sentinel := api.Group("/sentinel",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_OFFICER", "FIELD_SUPERVISOR", "GWL_MANAGER", "GWL_EXECUTIVE", "GRA_OFFICER"),
	)
	sentinel.Get("/anomalies", flagHandler.ListAnomalyFlags)
	sentinel.Post("/anomalies", flagHandler.CreateAnomalyFlag)
	sentinel.Get("/anomalies/:id", flagHandler.GetAnomalyFlag)
	// PATCH /api/v1/sentinel/anomalies/:id/confirm
	// Confirms anomaly as genuine revenue leakage. Auto-creates revenue_recovery_event.
	sentinel.Patch("/anomalies/:id/confirm",
		middleware.RequireRoles("SYSTEM_ADMIN", "MOF_AUDITOR", "GRA_OFFICER"),
		flagHandler.ConfirmAnomaly,
	)
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
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "MOF_AUDITOR", "GWL_MANAGER", "GWL_EXECUTIVE", "FIELD_SUPERVISOR", "GRA_OFFICER"),
		nrwHandler.GetNRWSummary)
	reports.Get("/nrw/my-district",
		middleware.RequireRoles("FIELD_OFFICER", "GWL_MANAGER", "FIELD_SUPERVISOR", "GRA_OFFICER"),
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
	// RBAC-REPORT-01 fix: report exports are sensitive regulatory documents.
	// Restrict to roles that legitimately need them:
	//   Monthly PDF/CSV  — management and auditors
	//   GRA Compliance   — GRA officers and auditors
	//   Audit Trail      — system admins and auditors
	//   Field Jobs       — supervisors and management
	reportAdminRoles := middleware.RequireRoles(
		"SUPER_ADMIN", "SYSTEM_ADMIN", "MOF_AUDITOR", "GRA_OFFICER",
		"GWL_EXECUTIVE", "GWL_MANAGER", "GWL_SUPERVISOR",
	)
	reportGRARoles := middleware.RequireRoles(
		"SUPER_ADMIN", "SYSTEM_ADMIN", "MOF_AUDITOR", "GRA_OFFICER",
		"GWL_EXECUTIVE", "GWL_MANAGER",
	)
	reports.Get("/monthly/pdf",        reportAdminRoles, reportHandler.GetMonthlyReportPDF)
	reports.Get("/monthly/csv",        reportAdminRoles, reportHandler.GetMonthlyReportCSV)
	reports.Get("/gra-compliance/csv", reportGRARoles,   reportHandler.GetGRAComplianceCSV)
	reports.Get("/audit-trail/csv",    reportAdminRoles, reportHandler.GetAuditTrailCSV)
	reports.Get("/field-jobs/csv",
		middleware.RequireRoles(
			"SUPER_ADMIN", "SYSTEM_ADMIN", "MOF_AUDITOR",
			"GWL_MANAGER", "GWL_SUPERVISOR", "FIELD_SUPERVISOR",
		),
		reportHandler.GetFieldJobsCSV,
	)

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


	// ── Revenue Recovery (managed-service monetisation) ──────────────────────
	// Tracks recovered GHS and 3% success fees from audit-driven recoveries.
	revenueRoles := middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "MOF_AUDITOR", "GWL_MANAGER", "GWL_EXECUTIVE")
	revenue := api.Group("/revenue", revenueRoles)
	revenue.Get("/summary",          revenueHandler.GetSummary)
	// GET /api/v1/revenue/pipeline — primary dashboard metric
	// Shows full GHS pipeline: Detected → Field Verified → Confirmed → GRA Signed → Collected
	revenue.Get("/pipeline",         revenueHandler.GetLeakagePipeline)
	revenue.Get("/events",           revenueHandler.ListEvents)
	revenue.Patch("/events/:id/confirm", revenueHandler.ConfirmRecovery)
	// PATCH /revenue/events/:id/collect — final stage: money physically received
	revenue.Patch("/events/:id/collect", revenueHandler.CollectRecovery)

	// ── Workforce Oversight (GPS breadcrumbs + active officer tracking) ───────
	workforce := api.Group("/workforce")
	workforce.Post("/location",
		middleware.RequireRoles("FIELD_OFFICER", "FIELD_SUPERVISOR"),
		workforceHandler.RecordLocation,
	)
	workforce.Get("/active",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_SUPERVISOR", "GWL_MANAGER"),
		workforceHandler.GetActiveOfficers,
	)
	workforce.Get("/summary",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_SUPERVISOR", "GWL_MANAGER"),
		workforceHandler.GetWorkforceSummary,
	)
	workforce.Get("/officers/:id/track",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_SUPERVISOR"),
		workforceHandler.GetOfficerTrack,
	)

	// ── Tariff Admin (Q7: PURC Tariff Schedule — admin-configurable) ─────────
	// System Admins can manage tariff rates and VAT config without code changes.
	tariffAdminHandler := handler.NewTariffAdminHandler(db, logger)
	adminTariffs := api.Group("/admin/tariffs",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN"),
	)
	adminTariffs.Get("/",                    tariffAdminHandler.ListTariffRates)
	adminTariffs.Post("/",                   tariffAdminHandler.CreateTariffRate)
	adminTariffs.Put("/:id",                 tariffAdminHandler.UpdateTariffRate)
	adminTariffs.Patch("/:id/deactivate",    tariffAdminHandler.DeactivateTariffRate)
	adminTariffs.Get("/vat",                 tariffAdminHandler.ListVATConfigs)
	adminTariffs.Post("/vat",                tariffAdminHandler.CreateVATConfig)

	// /admin/vat is a convenience alias for /admin/tariffs/vat
	adminVAT := api.Group("/admin/vat",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN"),
	)
	adminVAT.Get("/",  tariffAdminHandler.ListVATConfigs)
	adminVAT.Post("/", tariffAdminHandler.CreateVATConfig)

	// ── Geocoding (Q6: Address fallback when GWL GIS unavailable) ────────────
	geocodingSvc := gwsvc.NewGeocodingService(db, logger)
	geocoding := api.Group("/admin/geocoding",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN"),
	)
	geocoding.Post("/accounts/:id", func(c *fiber.Ctx) error {
		accountID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid account ID"})
		}
		result, err := geocodingSvc.GeocodeAccount(c.Context(), accountID)
		if err != nil {
			return c.Status(422).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(result)
	})
	geocoding.Post("/districts/:id", func(c *fiber.Ctx) error {
		districtID, err := uuid.Parse(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid district ID"})
		}
		// Run geocoding in background — returns immediately with job info
		go func() {
			bgCtx := context.Background()
			ok, fail, err := geocodingSvc.GeocodeDistrict(bgCtx, districtID)
			if err != nil {
				logger.Error("District geocoding failed", zap.Error(err))
			} else {
				logger.Info("District geocoding complete",
					zap.String("district_id", districtID.String()),
					zap.Int("success", ok), zap.Int("failed", fail),
				)
			}
		}()
		return c.JSON(fiber.Map{
			"message":     "District geocoding started in background",
			"district_id": districtID.String(),
			"note":        "Nominatim rate-limited to 1 req/sec. Check /admin/geocoding/status for progress.",
		})
	})
	// Field officer GPS confirmation (Q6: FIELD_CONFIRMED upgrade)
	geocoding.Post("/accounts/:id/confirm-gps",
		middleware.RequireRoles("SUPER_ADMIN", "SYSTEM_ADMIN", "FIELD_SUPERVISOR", "FIELD_OFFICER"),
		func(c *fiber.Ctx) error {
			accountID, err := uuid.Parse(c.Params("id"))
			if err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "invalid account ID"})
			}
			var body struct {
				Latitude  float64 `json:"latitude"`
				Longitude float64 `json:"longitude"`
			}
			if err := c.BodyParser(&body); err != nil || body.Latitude == 0 {
				return c.Status(400).JSON(fiber.Map{"error": "latitude and longitude required"})
			}
			// Get user ID from JWT claims
			userIDStr, _ := c.Locals("user_id").(string)
			userID, _ := uuid.Parse(userIDStr)
			if err := geocodingSvc.ConfirmGPSFromField(c.Context(), accountID, body.Latitude, body.Longitude, userID); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(fiber.Map{
				"message":    "GPS confirmed from field — fence radius set to 5m",
				"account_id": accountID.String(),
				"latitude":   body.Latitude,
				"longitude":  body.Longitude,
				"source":     "FIELD_CONFIRMED",
			})
		},
	)

	// ── Gap Tracking (Q9: Identified gaps and recouped amounts) ──────────────
	// Provides a comprehensive view of all identified revenue gaps and their
	// recovery status. This is the "audit trail" for the managed-service model.
	gapRoles := middleware.RequireRoles(
		"SUPER_ADMIN", "SYSTEM_ADMIN", "MOF_AUDITOR", "GWL_EXECUTIVE", "GWL_MANAGER",
		"AUDIT_MANAGER", "GRA_AUDITOR", "GRA_OFFICER",
	)
	gaps := api.Group("/gaps", gapRoles)

	// GET /api/v1/gaps/summary — aggregate gap and recovery stats
	gaps.Get("/summary", func(c *fiber.Ctx) error {
		districtFilter := c.Query("district_id")
		periodFilter := c.Query("period") // YYYY-MM

		baseWhere := "WHERE 1=1"
		args := []interface{}{}
		argIdx := 1

		if districtFilter != "" {
			baseWhere += fmt.Sprintf(" AND ae.district_id = $%d", argIdx)
			args = append(args, districtFilter)
			argIdx++
		}
		if periodFilter != "" {
			baseWhere += fmt.Sprintf(" AND TO_CHAR(ae.created_at, 'YYYY-MM') = $%d", argIdx)
			args = append(args, periodFilter)
			argIdx++
		}

		var summary struct {
			TotalGapsIdentified    int     `json:"total_gaps_identified"`
			TotalGapValueGHS       float64 `json:"total_gap_value_ghs"`
			TotalRecoveredGHS      float64 `json:"total_recovered_ghs"`
			TotalPendingGHS        float64 `json:"total_pending_ghs"`
			RecoveryRatePct        float64 `json:"recovery_rate_pct"`
			SuccessFeesEarnedGHS   float64 `json:"success_fees_earned_ghs"`
			GRASigned              int     `json:"gra_signed_audits"`
			GRAProvisional         int     `json:"gra_provisional_audits"`
			AvgDaysToRecovery      float64 `json:"avg_days_to_recovery"`
		}

		// Use RLS transaction so district-scoped policies are enforced
		rlsTx, hasTx := rls.TxFromContext(c.UserContext())
		var gapQ repository.Querier
		if hasTx {
			gapQ = rlsTx
		} else {
			gapQ = db
		}
		err := gapQ.QueryRow(c.UserContext(), fmt.Sprintf(`
			SELECT
				COUNT(ae.id)                                                    AS total_gaps,
				COALESCE(SUM(ae.confirmed_loss_ghs), 0)                        AS total_gap_value,
				COALESCE(SUM(rre.recovered_ghs), 0)                      AS total_recovered,
				COALESCE(SUM(ae.confirmed_loss_ghs), 0) - COALESCE(SUM(rre.recovered_ghs), 0) AS total_pending,
				CASE WHEN COALESCE(SUM(ae.confirmed_loss_ghs), 0) > 0
					THEN ROUND((COALESCE(SUM(rre.recovered_ghs), 0) / SUM(ae.confirmed_loss_ghs)) * 100, 2)
					ELSE 0 END                                                  AS recovery_rate,
				COALESCE(SUM(rre.success_fee_ghs), 0)                           AS success_fees,
				COUNT(CASE WHEN ae.gra_status = 'SIGNED' THEN 1 END) AS gra_signed,
				COUNT(CASE WHEN ae.gra_status = 'RETRYING'    THEN 1 END) AS gra_provisional,
				COALESCE(AVG(
					EXTRACT(EPOCH FROM (rre.confirmed_at - ae.created_at)) / 86400
				), 0)                                                           AS avg_days
			FROM audit_events ae
			LEFT JOIN revenue_recovery_events rre ON rre.audit_event_id = ae.id
			%s
		`, baseWhere), args...).Scan(
			&summary.TotalGapsIdentified,
			&summary.TotalGapValueGHS,
			&summary.TotalRecoveredGHS,
			&summary.TotalPendingGHS,
			&summary.RecoveryRatePct,
			&summary.SuccessFeesEarnedGHS,
			&summary.GRASigned,
			&summary.GRAProvisional,
			&summary.AvgDaysToRecovery,
		)
		if err != nil {
			logger.Error("Gap summary query failed", zap.Error(err))
			return c.Status(500).JSON(fiber.Map{"error": "failed to fetch gap summary"})
		}

		return c.JSON(summary)
	})

	// GET /api/v1/gaps — paginated list of all identified gaps with recovery status
	gaps.Get("/", func(c *fiber.Ctx) error {
		page := c.QueryInt("page", 1)
		limit := c.QueryInt("limit", 50)
		if limit > 200 {
			limit = 200
		}
		offset := (page - 1) * limit

		// Use RLS transaction so district-scoped policies are enforced
		gapListTx, hasGapListTx := rls.TxFromContext(c.UserContext())
		var gapListQ repository.Querier
		if hasGapListTx {
			gapListQ = gapListTx
		} else {
			gapListQ = db
		}
		rows, err := gapListQ.Query(c.UserContext(), `
			SELECT
				ae.id,
				ae.audit_reference,
				ae.district_id,
				d.district_name,
				ae.account_id,
				wa.gwl_account_number,
				wa.account_holder_name,
				COALESCE(af.anomaly_type::text, 'UNKNOWN') AS anomaly_type,
				ae.confirmed_loss_ghs,
				ae.gra_status::text,
				ae.gra_sdc_id,
				ae.created_at,
				rre.id                    AS recovery_id,
				rre.recovered_ghs,
				rre.success_fee_ghs,
				rre.status                AS recovery_status,
				rre.confirmed_at
			FROM audit_events ae
			JOIN districts d ON d.id = ae.district_id
			LEFT JOIN water_accounts wa ON wa.id = ae.account_id
			LEFT JOIN revenue_recovery_events rre ON rre.audit_event_id = ae.id
			LEFT JOIN anomaly_flags af ON af.id = ae.anomaly_flag_id
			ORDER BY ae.created_at DESC
			LIMIT $1 OFFSET $2
		`, limit, offset)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to fetch gaps"})
		}
		defer rows.Close()

		type gapRow struct {
			ID                  string   `json:"id"`
			AuditReference      string   `json:"audit_reference"`
			DistrictID          string   `json:"district_id"`
			DistrictName        string   `json:"district_name"`
			AccountID           *string  `json:"account_id"`
			AccountNumber       *string  `json:"account_number"`
			CustomerName        *string  `json:"customer_name"`
			AnomalyType         string   `json:"anomaly_type"`
			VarianceAmountGHS   float64  `json:"confirmed_loss_ghs"`
			GRAStatus           string   `json:"gra_status"`
			GRASDCID            *string  `json:"gra_sdc_id"`
			CreatedAt           string   `json:"created_at"`
			RecoveryID          *string  `json:"recovery_id"`
			RecoveredAmountGHS  *float64 `json:"recovered_ghs"`
			SuccessFeeGHS       *float64 `json:"success_fee_ghs"`
			RecoveryStatus      *string  `json:"recovery_status"`
			ConfirmedAt         *string  `json:"confirmed_at"`
		}

		var gapList []gapRow
		for rows.Next() {
			var r gapRow
			var id, districtID uuid.UUID
			var createdAt time.Time
			var confirmedAt *time.Time
			var recoveryID *uuid.UUID

			if err := rows.Scan(
				&id, &r.AuditReference, &districtID, &r.DistrictName,
				&r.AccountID, &r.AccountNumber, &r.CustomerName,
				&r.AnomalyType, &r.VarianceAmountGHS,
				&r.GRAStatus, &r.GRASDCID,
				&createdAt,
				&recoveryID, &r.RecoveredAmountGHS, &r.SuccessFeeGHS,
				&r.RecoveryStatus, &confirmedAt,
			); err != nil {
				continue
			}
			r.ID = id.String()
			r.DistrictID = districtID.String()
			r.CreatedAt = createdAt.Format(time.RFC3339)
			if recoveryID != nil {
				s := recoveryID.String()
				r.RecoveryID = &s
			}
			if confirmedAt != nil {
				s := confirmedAt.Format(time.RFC3339)
				r.ConfirmedAt = &s
			}
			gapList = append(gapList, r)
		}

		return c.JSON(fiber.Map{
			"gaps":  gapList,
			"page":  page,
			"limit": limit,
			"total": len(gapList),
		})
	})


	// ── Whistleblower Tips (public — no auth required) ────────────────────────
	// Anonymous tip submission. No login required by design.
	// Rate-limited separately to prevent abuse.
	whistleblowerHandler := handler.NewWhistleblowerHandler(db, logger)
	app.Post("/api/v1/tips", whistleblowerHandler.SubmitTip)
	app.Get("/api/v1/tips/:ref", whistleblowerHandler.GetTipStatus)

	// ── Public district list (for whistleblower form dropdown — no auth) ──────
	app.Get("/api/v1/public/districts", func(c *fiber.Ctx) error {
		type districtPublic struct {
			DistrictCode string `json:"district_code"`
			Name         string `json:"name"`
			Region       string `json:"region"`
		}
		// Direct query — RLS policy on districts is USING(true) so all rows visible
		rows, err := db.Query(c.Context(),
			`SELECT district_code, district_name, region FROM districts ORDER BY district_name`)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to fetch districts"})
		}
		defer rows.Close()
		var districts []districtPublic
		for rows.Next() {
			var d districtPublic
			if err := rows.Scan(&d.DistrictCode, &d.Name, &d.Region); err != nil {
				continue
			}
			districts = append(districts, d)
		}
		if districts == nil {
			districts = []districtPublic{}
		}
		return c.JSON(fiber.Map{"districts": districts})
	})

	// ── Whistleblower Admin (SYSTEM_ADMIN only) ───────────────────────────────
	adminTips := api.Group("/admin/tips",
		middleware.RequireRoles("SYSTEM_ADMIN", "SUPER_ADMIN"),
	)
	adminTips.Get("/", whistleblowerHandler.ListTips)
	adminTips.Patch("/:id", whistleblowerHandler.UpdateTip)

	// ── Donor KPI Reports ─────────────────────────────────────────────────────
	donorReportHandler := handler.NewDonorReportHandler(db, logger)
	donorReports := api.Group("/reports/donor",
		middleware.RequireRoles("SYSTEM_ADMIN", "SUPER_ADMIN", "MOF_AUDITOR", "GRA_OFFICER", "GWL_EXECUTIVE"),
	)
	donorReports.Get("/kpis", donorReportHandler.GetKPIs)
	donorReports.Get("/trend", donorReportHandler.GetTrend)

	// ── Offline Sync (field officers) ─────────────────────────────────────────
	offlineSyncHandler := handler.NewOfflineSyncHandler(db, logger)
	// Field officer sync endpoints
	api.Get("/sync/pull",
		middleware.RequireRoles("FIELD_OFFICER", "FIELD_SUPERVISOR"),
		offlineSyncHandler.Pull,
	)
	api.Post("/sync/push",
		middleware.RequireRoles("FIELD_OFFICER", "FIELD_SUPERVISOR"),
		offlineSyncHandler.Push,
	)
	api.Get("/sync/status",
		middleware.RequireRoles("FIELD_OFFICER", "FIELD_SUPERVISOR"),
		offlineSyncHandler.Status,
	)
	// Admin sync monitoring
	api.Get("/admin/sync/queue",
		middleware.RequireRoles("SYSTEM_ADMIN", "SUPER_ADMIN", "MOF_AUDITOR"),
		func(c *fiber.Ctx) error {
			statusFilter := c.Query("status")
			actionFilter := c.Query("action_type")
			limitVal := c.QueryInt("limit", 100)

			query := `
				SELECT
					osq.id, osq.device_id,
					COALESCE(u.full_name, 'Unknown') AS user_name,
					osq.action_type::text, osq.entity_type,
					osq.entity_id::text, osq.status::text,
					osq.client_timestamp, osq.processed_at, osq.created_at
				FROM offline_sync_queue osq
				LEFT JOIN users u ON u.id = osq.user_id
				WHERE 1=1
			`
			args := []interface{}{}
			argIdx := 1
			if statusFilter != "" {
				query += fmt.Sprintf(" AND osq.status = $%d::sync_status", argIdx)
				args = append(args, statusFilter)
				argIdx++
			}
			if actionFilter != "" {
				query += fmt.Sprintf(" AND osq.action_type = $%d::sync_action_type", argIdx)
				args = append(args, actionFilter)
				argIdx++
			}
			query += fmt.Sprintf(" ORDER BY osq.created_at DESC LIMIT $%d", argIdx)
			args = append(args, limitVal)

			// Use RLS transaction so user-scoped policies are enforced
			syncTx, hasSyncTx := rls.TxFromContext(c.UserContext())
			var syncQ repository.Querier
			if hasSyncTx {
				syncQ = syncTx
			} else {
				syncQ = db
			}
			rows, err := syncQ.Query(c.UserContext(), query, args...)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "failed to fetch sync queue"})
			}
			defer rows.Close()

			type queueItem struct {
				ID              string  `json:"id"`
				DeviceID        string  `json:"device_id"`
				UserName        string  `json:"user_name"`
				ActionType      string  `json:"action_type"`
				EntityType      string  `json:"entity_type"`
				EntityID        string  `json:"entity_id"`
				Status          string  `json:"status"`
				ClientTimestamp string  `json:"client_timestamp"`
				ProcessedAt     *string `json:"processed_at"`
				CreatedAt       string  `json:"created_at"`
			}

			var items []queueItem
			for rows.Next() {
				var item queueItem
				var id uuid.UUID
				var clientTS, createdAt time.Time
				var processedAt *time.Time
				if err := rows.Scan(
					&id, &item.DeviceID, &item.UserName,
					&item.ActionType, &item.EntityType, &item.EntityID,
					&item.Status, &clientTS, &processedAt, &createdAt,
				); err != nil {
					continue
				}
				item.ID = id.String()
				item.ClientTimestamp = clientTS.Format(time.RFC3339)
				item.CreatedAt = createdAt.Format(time.RFC3339)
				if processedAt != nil {
					s := processedAt.Format(time.RFC3339)
					item.ProcessedAt = &s
				}
				items = append(items, item)
			}
			return c.JSON(fiber.Map{"items": items, "total": len(items)})
		},
	)
	api.Get("/admin/sync/devices",
		middleware.RequireRoles("SYSTEM_ADMIN", "SUPER_ADMIN", "MOF_AUDITOR"),
		func(c *fiber.Ctx) error {
			// Use RLS transaction so user-scoped policies are enforced
			devTx, hasDevTx := rls.TxFromContext(c.UserContext())
			var devQ repository.Querier
			if hasDevTx {
				devQ = devTx
			} else {
				devQ = db
			}
			rows, err := devQ.Query(c.UserContext(), `
				SELECT
					ud.device_id,
					COALESCE(u.full_name, 'Unknown') AS user_name,
					ud.last_seen_at,
					COUNT(osq.id) FILTER (WHERE osq.status = 'PENDING') AS pending_count,
					COUNT(osq.id) FILTER (WHERE osq.status = 'CONFLICT') AS conflict_count
				FROM user_devices ud
				LEFT JOIN users u ON u.id = ud.user_id
				LEFT JOIN offline_sync_queue osq ON osq.device_id = ud.device_id
					AND osq.created_at >= NOW() - INTERVAL '24 hours'
				WHERE ud.last_seen_at >= NOW() - INTERVAL '7 days'
				GROUP BY ud.device_id, u.full_name, ud.last_seen_at
				ORDER BY ud.last_seen_at DESC
				LIMIT 50
			`)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "failed to fetch devices"})
			}
			defer rows.Close()

			type deviceInfo struct {
				DeviceID      string `json:"device_id"`
				UserName      string `json:"user_name"`
				LastSeenAt    string `json:"last_seen_at"`
				PendingCount  int    `json:"pending_count"`
				ConflictCount int    `json:"conflict_count"`
			}

			var devices []deviceInfo
			for rows.Next() {
				var d deviceInfo
				var lastSeen time.Time
				if err := rows.Scan(&d.DeviceID, &d.UserName, &lastSeen, &d.PendingCount, &d.ConflictCount); err != nil {
					continue
				}
				d.LastSeenAt = lastSeen.Format(time.RFC3339)
				devices = append(devices, d)
			}
			return c.JSON(fiber.Map{"devices": devices, "total": len(devices)})
		},
	)

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
