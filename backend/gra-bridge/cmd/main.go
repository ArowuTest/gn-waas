package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	gra "github.com/ArowuTest/gn-waas/backend/gra-bridge/internal/service"
	"go.uber.org/zap"
)

func main() {
	zapLogger, _ := zap.NewProduction()
	defer zapLogger.Sync()

	sandboxMode := os.Getenv("GRA_SANDBOX_MODE") != "false"

	// Q2 fix: consistent env var name GRA_VSDC_BASE_URL, correct default v8.1
	baseURL     := getEnv("GRA_VSDC_BASE_URL", "https://vsdc.gra.gov.gh/api/v8.1")
	apiKey      := getEnv("GRA_API_KEY", "")
	businessTIN := getEnv("GRA_BUSINESS_TIN", "C0000000000")
	port        := getEnv("PORT", "8085")

	if sandboxMode {
		zapLogger.Info("GRA Bridge starting in SANDBOX mode",
			zap.String("note", "GRA does not operate a public sandbox. "+
				"Set GRA_SANDBOX_MODE=false with live credentials from GRA Technical Division."),
		)
	} else {
		if apiKey == "" {
			log.Fatal("GRA_API_KEY must be set in production mode (obtain from GRA Technical Division)")
		}
		zapLogger.Info("GRA Bridge starting in PRODUCTION mode",
			zap.String("base_url", baseURL),
			zap.String("business_tin", businessTIN),
		)
	}

	svc := gra.NewGRABridgeService(baseURL, apiKey, businessTIN, sandboxMode, zapLogger)

	app := fiber.New(fiber.Config{
		AppName: "GN-WAAS GRA Bridge v1.0",
	})
	app.Use(recover.New())
	app.Use(logger.New())

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":       "ok",
			"service":      "gra-bridge",
			"sandbox_mode": sandboxMode,
			"gra_base_url": baseURL,
			"api_version":  "8.1",
		})
	})

	api := app.Group("/api/v1/gra")

	// POST /api/v1/gra/sign-audit — Sign an audit event invoice
	// Returns: InvoiceResponse + is_provisional flag
	// Q1: If GRA is down, returns provisional=true with internal QR; caller queues retry.
	api.Post("/sign-audit", func(c *fiber.Ctx) error {
		var req gra.AuditInvoiceRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}
		if req.AuditEventID == "" || req.AuditReference == "" {
			return c.Status(400).JSON(fiber.Map{"error": "audit_event_id and audit_reference are required"})
		}
		if req.TotalInvoiceGHS <= 0 {
			return c.Status(400).JSON(fiber.Map{"error": "total_invoice_ghs must be positive"})
		}

		resp, retryInfo, err := svc.SignAuditInvoice(c.Context(), &req)
		if err != nil {
			// Non-retryable error (GRA rejected or client error)
			zapLogger.Error("GRA signing failed (non-retryable)",
				zap.String("audit_event_id", req.AuditEventID),
				zap.Error(err),
			)
			return c.Status(502).JSON(fiber.Map{
				"error":        "GRA signing failed",
				"details":      err.Error(),
				"should_retry": false,
				"gra_status":   "FAILED",
			})
		}

		result := fiber.Map{
			"success":        resp.Success,
			"sdc_id":         resp.SDCID,
			"qr_code_url":    resp.QRCodeURL,
			"qr_code_string": resp.QRCodeString,
			"qr_code_base64": resp.QRCodeBase64,
			"invoice_number": resp.InvoiceNumber,
			"signed_at":      resp.SignedAt,
			"is_provisional": resp.IsProvisional,
			"gra_status":     "SIGNED",
		}

		if resp.IsProvisional && retryInfo != nil {
			result["gra_status"]   = "PROVISIONAL"
			result["retry_after"]  = retryInfo.RetryAfter
			result["should_retry"] = true
		}

		return c.JSON(result)
	})

	// POST /api/v1/gra/sign-invoice — Sign a raw invoice
	api.Post("/sign-invoice", func(c *fiber.Ctx) error {
		var req gra.InvoiceRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		resp, err := svc.SignInvoice(c.Context(), &req)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{
				"error":   "GRA signing failed",
				"details": err.Error(),
			})
		}

		return c.JSON(resp)
	})

	// GET /api/v1/gra/verify/:sdc_id — Verify a QR code / SDC-ID
	api.Get("/verify/:sdc_id", func(c *fiber.Ctx) error {
		sdcID := c.Params("sdc_id")
		if sdcID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "sdc_id is required"})
		}

		valid, status, err := svc.VerifyQRCode(c.Context(), sdcID)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{
				"error":   "GRA verification failed",
				"details": err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"valid":  valid,
			"status": status,
			"sdc_id": sdcID,
		})
	})

	// GET /api/v1/gra/qrcode — Generate a QR code PNG image
	api.Get("/qrcode", func(c *fiber.Ctx) error {
		content := c.Query("content")
		if content == "" {
			return c.Status(400).JSON(fiber.Map{"error": "content query param required"})
		}
		sizeStr := c.Query("size", "256")
		size, _ := strconv.Atoi(sizeStr)
		if size <= 0 || size > 1024 {
			size = 256
		}

		png, err := gra.GenerateQRCodePNG(content, size)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "QR code generation failed"})
		}

		format := c.Query("format", "png")
		if format == "base64" {
			return c.JSON(fiber.Map{
				"base64": base64.StdEncoding.EncodeToString(png),
				"size":   size,
			})
		}

		c.Set("Content-Type", "image/png")
		c.Set("Cache-Control", "public, max-age=3600")
		return c.Send(png)
	})

	addr := fmt.Sprintf(":%s", port)
	zapLogger.Info("GRA Bridge listening", zap.String("addr", addr))
	if err := app.Listen(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
