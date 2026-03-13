package main

import (
	"encoding/base64"
	"os"
	"strings"
	"os/signal"
	"syscall"
	"time"
	"context"

	"github.com/ArowuTest/gn-waas/backend/ocr-service/internal/service"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func main() {
	var logger *zap.Logger
	if os.Getenv("APP_ENV") == "production" {
		logger, _ = zap.NewProduction()
	} else {
		logger, _ = zap.NewDevelopment()
	}
	defer logger.Sync()

	logger.Info("GN-WAAS OCR Service starting")

	ocrSvc := service.NewOCRService(
		getEnv("TESSERACT_PATH", "tesseract"),
		getEnv("TEMP_DIR", "/tmp"),
		logger,
	)

	app := fiber.New(fiber.Config{
		AppName:   "GN-WAAS OCR Service v1.0",
		BodyLimit: 10 * 1024 * 1024, // 10MB max photo size
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"service": "ocr-service", "status": "healthy"})
	})

	// Process meter photo
	// Accepts EITHER:
	//   - multipart/form-data with "photo" file field (direct upload)
	//   - application/json with "image_base64" field (from API gateway proxy)
	app.Post("/api/v1/ocr/process", func(c *fiber.Ctx) error {
		var data []byte
		var err error

		contentType := string(c.Request().Header.ContentType())
		if strings.Contains(contentType, "application/json") {
			// JSON path: decode base64 image
			var body struct {
				ImageBase64 string `json:"image_base64"`
				JobID       string `json:"job_id"`
			}
			if err = c.BodyParser(&body); err != nil || body.ImageBase64 == "" {
				return c.Status(400).JSON(fiber.Map{"error": "image_base64 required in JSON body"})
			}
			data, err = base64.StdEncoding.DecodeString(body.ImageBase64)
			if err != nil {
				// Try URL-safe base64
				data, err = base64.URLEncoding.DecodeString(body.ImageBase64)
				if err != nil {
					return c.Status(400).JSON(fiber.Map{"error": "invalid base64 image data"})
				}
			}
		} else {
			// Multipart path: read file field
			file, ferr := c.FormFile("photo")
			if ferr != nil {
				return c.Status(400).JSON(fiber.Map{"error": "photo file required (multipart) or image_base64 (JSON)"})
			}
			f, ferr := file.Open()
			if ferr != nil {
				return c.Status(500).JSON(fiber.Map{"error": "failed to open file"})
			}
			defer f.Close()
			data, err = service.ReadAll(f)
		}

		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to read file"})
		}

		// Detect MIME type from data bytes (works for both paths)
		mimeType := "image/jpeg"
		if len(data) > 3 && data[0] == 0x89 && data[1] == 0x50 {
			mimeType = "image/png"
		}

		result, err := ocrSvc.ProcessMeterPhoto(c.Context(), data, mimeType)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(result)
	})

	// Validate GPS fence
	app.Post("/api/v1/ocr/validate-gps", func(c *fiber.Ctx) error {
		var body struct {
			PhotoLat    float64 `json:"photo_lat"`
			PhotoLng    float64 `json:"photo_lng"`
			MeterLat    float64 `json:"meter_lat"`
			MeterLng    float64 `json:"meter_lng"`
			FenceRadius float64 `json:"fence_radius_m"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid body"})
		}

		withinFence, distanceM := ocrSvc.ValidateGPSFence(
			body.PhotoLat, body.PhotoLng,
			body.MeterLat, body.MeterLng,
			body.FenceRadius,
		)

		return c.JSON(fiber.Map{
			"within_fence": withinFence,
			"distance_m":   distanceM,
			"fence_radius_m": body.FenceRadius,
		})
	})

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Render injects PORT at runtime; fall back to APP_PORT then 3005 for local dev.
	port := getEnv("PORT", getEnv("APP_PORT", "3005"))
	go func() {
		logger.Info("OCR Service listening", zap.String("port", port))
		if err := app.Listen(":" + port); err != nil {
			logger.Error("Server error", zap.Error(err))
		}
	}()

	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = app.ShutdownWithContext(ctx)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
