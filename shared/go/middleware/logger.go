package middleware

import (
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RequestLogger logs all incoming HTTP requests with timing
func RequestLogger(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		requestID := uuid.New().String()

		// Inject request ID into context
		c.Locals("request_id", requestID)
		c.Set("X-Request-ID", requestID)

		// Process request
		err := c.Next()

		// Log after response
		duration := time.Since(start)
		statusCode := c.Response().StatusCode()

		logFn := logger.Info
		if statusCode >= 500 {
			logFn = logger.Error
		} else if statusCode >= 400 {
			logFn = logger.Warn
		}

		logFn("HTTP request",
			zap.String("request_id", requestID),
			zap.String("method", c.Method()),
			zap.String("path", c.Path()),
			zap.Int("status", statusCode),
			zap.Duration("duration", duration),
			zap.String("ip", c.IP()),
			zap.String("user_agent", c.Get("User-Agent")),
			zap.String("user_id", getStringLocal(c, "user_id")),
		)

		return err
	}
}

// buildAllowedOrigins constructs the set of allowed CORS origins.
// Hard-coded production origins are always included.
// Additional origins can be injected at runtime via CORS_ALLOWED_ORIGINS
// (comma-separated), which lets Render/Vercel preview URLs work without
// a code change.
func buildAllowedOrigins() map[string]bool {
	origins := map[string]bool{
		// Production sovereign domains
		"https://admin.gnwaas.gov.gh":     true,
		"https://authority.gnwaas.gov.gh": true,
		"https://gwl.gnwaas.gov.gh":       true,
		"https://gnwaas.gov.gh":           true,
		"https://www.gnwaas.gov.gh":       true,
		// Staging
		"https://admin-staging.gnwaas.gov.gh":     true,
		"https://authority-staging.gnwaas.gov.gh": true,
	}

	// Runtime-injectable origins (Vercel preview URLs, Render custom domains, etc.)
	// Set CORS_ALLOWED_ORIGINS=https://gn-waas-admin.vercel.app,https://gn-waas-gwl.vercel.app
	if extra := os.Getenv("CORS_ALLOWED_ORIGINS"); extra != "" {
		for _, o := range strings.Split(extra, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				origins[o] = true
			}
		}
	}

	return origins
}

// CORS configures Cross-Origin Resource Sharing.
// Development (APP_ENV=development or unset): permits all origins.
// Production: restricts to allowedOrigins (hard-coded + CORS_ALLOWED_ORIGINS env var).
func CORS() fiber.Handler {
	isDev := os.Getenv("APP_ENV") == "development" || os.Getenv("APP_ENV") == ""
	allowedOrigins := buildAllowedOrigins()

	return func(c *fiber.Ctx) error {
		origin := c.Get("Origin")

		if isDev {
			// Development: allow all origins
			c.Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" && allowedOrigins[origin] {
			// Production: only allow known portal origins
			c.Set("Access-Control-Allow-Origin", origin)
			c.Set("Vary", "Origin")
		} else if origin != "" {
			// Unknown origin in production — reject preflight
			if c.Method() == fiber.MethodOptions {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "CORS: origin not allowed",
				})
			}
		}

		c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Request-ID")
		c.Set("Access-Control-Expose-Headers", "X-Request-ID")
		c.Set("Access-Control-Max-Age", "86400")

		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}

// SecurityHeaders adds security headers to all responses.
func SecurityHeaders() fiber.Handler {
	isProd := os.Getenv("APP_ENV") == "production"

	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "geolocation=(self), microphone=(), camera=(self)")

		if isProd {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		csp := "default-src 'self'; " +
			"script-src 'self'; " +
			"style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data: blob:; " +
			"font-src 'self'; " +
			"connect-src 'self'; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self'; " +
			"form-action 'self'"
		c.Set("Content-Security-Policy", csp)

		return c.Next()
	}
}

// RecoverMiddleware recovers from panics and returns a 500 response
func RecoverMiddleware(logger *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Panic recovered",
					zap.Any("panic", r),
					zap.String("path", c.Path()),
					zap.String("method", c.Method()),
				)
				_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"success": false,
					"error": fiber.Map{
						"code":    "INTERNAL_ERROR",
						"message": "An unexpected error occurred",
					},
				})
			}
		}()
		return c.Next()
	}
}

func getStringLocal(c *fiber.Ctx, key string) string {
	if val, ok := c.Locals(key).(string); ok {
		return val
	}
	return ""
}
