package middleware

import (
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

// CORS configures Cross-Origin Resource Sharing for the admin portal
func CORS() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*") // Restrict in production
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

// SecurityHeaders adds security headers to all responses
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
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
