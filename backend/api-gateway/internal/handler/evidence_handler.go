package handler

// GN-WAAS Evidence Upload Handler
//
// Implements the presigned URL flow for meter photo uploads:
//
//   Step 1: Flutter calls POST /api/v1/evidence/upload-url
//           → Returns { object_key, upload_url, expires_in }
//
//   Step 2: Flutter PUTs photo bytes directly to MinIO via upload_url
//           (no API gateway involvement — keeps gateway lightweight)
//
//   Step 3: Flutter includes object_key in POST /field-jobs/:id/submit
//           → Backend verifies object exists in MinIO
//           → Backend verifies SHA-256 hash matches submitted hash
//           → Stores object_key as meter_photo_url in audit_events

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/storage"
	response "github.com/ArowuTest/gn-waas/shared/go/http/response"
)

// EvidenceHandler handles photo evidence upload coordination
type EvidenceHandler struct {
	storage *storage.EvidenceStorageService
	logger  *zap.Logger
}

// NewEvidenceHandler creates a new EvidenceHandler.
// If storage is nil (MinIO not configured), endpoints return graceful errors.
func NewEvidenceHandler(storage *storage.EvidenceStorageService, logger *zap.Logger) *EvidenceHandler {
	return &EvidenceHandler{storage: storage, logger: logger}
}

// GetUploadURL godoc
// POST /api/v1/evidence/upload-url
// Called by Flutter before capturing a photo. Returns a presigned PUT URL
// that Flutter uses to upload the photo directly to MinIO.
//
// Request body:
//
//	{
//	  "job_id":       "uuid",
//	  "filename":     "meter_photo.jpg",
//	  "content_type": "image/jpeg"
//	}
//
// Response:
//
//	{
//	  "object_key": "evidence/{job_id}/{ts}_{filename}",
//	  "upload_url": "https://minio.../presigned-put-url",
//	  "expires_in": 900
//	}
func (h *EvidenceHandler) GetUploadURL(c *fiber.Ctx) error {
	var req struct {
		JobID       string `json:"job_id"`
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if req.JobID == "" {
		return response.BadRequest(c, "MISSING_JOB_ID", "job_id is required")
	}
	if req.Filename == "" {
		req.Filename = "meter_photo.jpg"
	}
	if req.ContentType == "" {
		req.ContentType = "image/jpeg"
	}

	// Sanitise filename — strip path traversal
	req.Filename = filepath.Base(req.Filename)
	req.Filename = strings.ReplaceAll(req.Filename, " ", "_")

	if h.storage == nil {
		// MinIO not configured — return a graceful degraded response
		// Flutter will proceed without photo upload (offline mode)
		h.logger.Warn("Evidence upload requested but MinIO not configured",
			zap.String("job_id", req.JobID))
		return response.OK(c, fiber.Map{
			"object_key":  fmt.Sprintf("evidence/%s/offline_%d.jpg", req.JobID, time.Now().Unix()),
			"upload_url":  "",
			"expires_in":  0,
			"storage_mode": "offline",
			"warning":     "MinIO not configured — photo will be stored locally only",
		})
	}

	objectKey, uploadURL, err := h.storage.PresignedUploadURL(
		c.UserContext(), req.JobID, req.Filename, req.ContentType,
	)
	if err != nil {
		h.logger.Error("Failed to generate presigned upload URL",
			zap.String("job_id", req.JobID),
			zap.Error(err))
		return response.InternalError(c, "Failed to generate upload URL")
	}

	h.logger.Info("Presigned upload URL generated",
		zap.String("job_id", req.JobID),
		zap.String("object_key", objectKey),
	)

	return response.OK(c, fiber.Map{
		"object_key":   objectKey,
		"upload_url":   uploadURL,
		"expires_in":   900, // 15 minutes
		"storage_mode": "minio",
	})
}

// GetDownloadURL godoc
// GET /api/v1/evidence/:object_key/url
// Returns a presigned GET URL for viewing a stored evidence photo.
// Used by admin/authority portals to display photos in audit review.
func (h *EvidenceHandler) GetDownloadURL(c *fiber.Ctx) error {
	// object_key is URL-encoded in the path param
	objectKey := c.Params("*")
	if objectKey == "" {
		return response.BadRequest(c, "MISSING_KEY", "object_key is required")
	}

	if h.storage == nil {
		return response.ServiceUnavailable(c, "Evidence storage")
	}

	downloadURL, err := h.storage.PresignedDownloadURL(c.UserContext(), objectKey)
	if err != nil {
		h.logger.Error("Failed to generate presigned download URL",
			zap.String("object_key", objectKey),
			zap.Error(err))
		return response.InternalError(c, "Failed to generate download URL")
	}

	return response.OK(c, fiber.Map{
		"download_url": downloadURL,
		"expires_in":   3600, // 1 hour
	})
}

// VerifyPhotoHash godoc
// POST /api/v1/evidence/verify-hash
// Server-side tamper-evidence check: downloads the stored photo and verifies
// its SHA-256 hash matches the hash submitted by the field officer.
func (h *EvidenceHandler) VerifyPhotoHash(c *fiber.Ctx) error {
	var req struct {
		ObjectKey    string `json:"object_key"`
		ExpectedHash string `json:"expected_hash"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if req.ObjectKey == "" || req.ExpectedHash == "" {
		return response.BadRequest(c, "MISSING_FIELDS", "object_key and expected_hash are required")
	}

	if h.storage == nil {
		return response.ServiceUnavailable(c, "Evidence storage")
	}

	match, err := h.storage.VerifyPhotoHash(c.UserContext(), req.ObjectKey, req.ExpectedHash)
	if err != nil {
		h.logger.Error("Hash verification failed",
			zap.String("object_key", req.ObjectKey),
			zap.Error(err))
		return response.InternalError(c, "Hash verification failed")
	}

	if !match {
		h.logger.Warn("Photo hash mismatch — potential tampering detected",
			zap.String("object_key", req.ObjectKey),
			zap.String("expected_hash", req.ExpectedHash),
		)
	}

	return response.OK(c, fiber.Map{
		"hash_verified": match,
		"object_key":    req.ObjectKey,
	})
}
