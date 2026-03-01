package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// OCRResult holds the result of a meter photo OCR analysis
type OCRResult struct {
	PhotoHash       string    `json:"photo_hash"`
	OCRReading      *float64  `json:"ocr_reading"`
	OCRConfidence   float64   `json:"ocr_confidence"`
	OCRStatus       string    `json:"ocr_status"`
	RawText         string    `json:"raw_text"`
	ProcessedAt     time.Time `json:"processed_at"`
	ProcessingMs    int64     `json:"processing_ms"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	ImageWidth      int       `json:"image_width"`
	ImageHeight     int       `json:"image_height"`
	ImageSizeBytes  int64     `json:"image_size_bytes"`
}

// PhotoMetadata holds GPS and device metadata from a field photo
type PhotoMetadata struct {
	DeviceID       string    `json:"device_id"`
	GPSLatitude    float64   `json:"gps_latitude"`
	GPSLongitude   float64   `json:"gps_longitude"`
	GPSPrecisionM  float64   `json:"gps_precision_m"`
	CapturedAt     time.Time `json:"captured_at"`
	PhotoHash      string    `json:"photo_hash"`
	IsWithinFence  bool      `json:"is_within_fence"`
	FenceRadiusM   float64   `json:"fence_radius_m"`
}

// OCRService handles meter photo processing using Tesseract
type OCRService struct {
	tesseractPath string
	tempDir       string
	logger        *zap.Logger
}

func NewOCRService(tesseractPath, tempDir string, logger *zap.Logger) *OCRService {
	if tesseractPath == "" {
		tesseractPath = "tesseract"
	}
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	return &OCRService{
		tesseractPath: tesseractPath,
		tempDir:       tempDir,
		logger:        logger,
	}
}

// ProcessMeterPhoto runs OCR on a meter face photo
// Returns the extracted reading and confidence score
func (s *OCRService) ProcessMeterPhoto(ctx context.Context, photoData []byte, mimeType string) (*OCRResult, error) {
	start := time.Now()

	result := &OCRResult{
		ProcessedAt: start,
		OCRStatus:   "PROCESSING",
	}

	// Compute photo hash for immutability
	hash := sha256.Sum256(photoData)
	result.PhotoHash = fmt.Sprintf("%x", hash)
	result.ImageSizeBytes = int64(len(photoData))

	// Get image dimensions
	if w, h, err := getImageDimensions(photoData, mimeType); err == nil {
		result.ImageWidth = w
		result.ImageHeight = h
	}

	// Write photo to temp file
	ext := ".jpg"
	if mimeType == "image/png" {
		ext = ".png"
	}
	tmpFile := filepath.Join(s.tempDir, fmt.Sprintf("meter_%d%s", time.Now().UnixNano(), ext))
	if err := os.WriteFile(tmpFile, photoData, 0600); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	// Run Tesseract OCR
	outputBase := strings.TrimSuffix(tmpFile, ext)
	cmd := exec.CommandContext(ctx, s.tesseractPath,
		tmpFile,
		outputBase,
		"--psm", "7",  // Single line mode - best for meter displays
		"-c", "tessedit_char_whitelist=0123456789.",
		"--oem", "3",  // LSTM + Legacy
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		s.logger.Warn("Tesseract OCR failed",
			zap.String("error", stderr.String()),
			zap.Error(err),
		)
		result.OCRStatus = "FAILED"
		result.ErrorMessage = fmt.Sprintf("OCR engine error: %s", stderr.String())
		result.ProcessingMs = time.Since(start).Milliseconds()
		return result, nil
	}

	// Read OCR output
	outputFile := outputBase + ".txt"
	defer os.Remove(outputFile)

	outputData, err := os.ReadFile(outputFile)
	if err != nil {
		result.OCRStatus = "FAILED"
		result.ErrorMessage = "Failed to read OCR output"
		result.ProcessingMs = time.Since(start).Milliseconds()
		return result, nil
	}

	rawText := strings.TrimSpace(string(outputData))
	result.RawText = rawText

	// Parse the meter reading from OCR text
	reading, confidence, err := s.parseMeterReading(rawText)
	if err != nil {
		result.OCRStatus = "UNREADABLE"
		result.ErrorMessage = fmt.Sprintf("Could not parse reading from: '%s'", rawText)
		result.ProcessingMs = time.Since(start).Milliseconds()
		return result, nil
	}

	result.OCRReading = &reading
	result.OCRConfidence = confidence
	result.OCRStatus = "SUCCESS"
	result.ProcessingMs = time.Since(start).Milliseconds()

	s.logger.Info("OCR processing complete",
		zap.Float64("reading", reading),
		zap.Float64("confidence", confidence),
		zap.Int64("processing_ms", result.ProcessingMs),
	)

	return result, nil
}

// ValidateGPSFence checks if a photo's GPS coordinates are within the meter's geofence
func (s *OCRService) ValidateGPSFence(
	photoLat, photoLng float64,
	meterLat, meterLng float64,
	fenceRadiusM float64,
) (bool, float64) {
	distanceM := haversineDistance(photoLat, photoLng, meterLat, meterLng)
	return distanceM <= fenceRadiusM, distanceM
}

// ValidatePhotoAge checks if a photo was taken within the allowed time window
func (s *OCRService) ValidatePhotoAge(capturedAt time.Time, maxAgeMinutes int) bool {
	age := time.Since(capturedAt)
	return age <= time.Duration(maxAgeMinutes)*time.Minute
}

// parseMeterReading extracts a numeric reading from OCR text
func (s *OCRService) parseMeterReading(text string) (float64, float64, error) {
	// Clean the text
	cleaned := strings.ReplaceAll(text, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "\n", "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")

	// Try to extract a number (with optional decimal point)
	re := regexp.MustCompile(`\d+\.?\d*`)
	matches := re.FindAllString(cleaned, -1)

	if len(matches) == 0 {
		return 0, 0, fmt.Errorf("no numeric value found in OCR text: '%s'", text)
	}

	// Take the longest numeric match (most likely the meter reading)
	bestMatch := ""
	for _, m := range matches {
		if len(m) > len(bestMatch) {
			bestMatch = m
		}
	}

	reading, err := strconv.ParseFloat(bestMatch, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse reading '%s': %w", bestMatch, err)
	}

	// Confidence based on how clean the extraction was
	confidence := 0.95
	if len(matches) > 1 {
		confidence = 0.75 // Multiple numbers found - less certain
	}
	if len(cleaned) != len(bestMatch) {
		confidence -= 0.1 // Extra characters present
	}

	return reading, confidence, nil
}

// haversineDistance calculates the distance between two GPS coordinates in metres
func haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6371000.0

	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusM * c
}

// getImageDimensions returns the width and height of an image
func getImageDimensions(data []byte, mimeType string) (int, int, error) {
	reader := bytes.NewReader(data)
	var img image.Image
	var err error

	switch mimeType {
	case "image/png":
		img, err = png.Decode(reader)
	default:
		img, err = jpeg.Decode(reader)
	}

	if err != nil {
		return 0, 0, err
	}

	bounds := img.Bounds()
	return bounds.Max.X, bounds.Max.Y, nil
}

// ReadAll reads all bytes from a reader
func ReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
