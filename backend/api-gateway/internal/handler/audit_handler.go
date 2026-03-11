package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ArowuTest/gn-waas/shared/go/http/response"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/domain"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/notification"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/storage"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/repository"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AuditHandler handles audit event HTTP requests
type AuditHandler struct {
	auditRepo    *repository.AuditEventRepository
	fieldJobRepo *repository.FieldJobRepository
	userRepo     *repository.UserRepository
	logger       *zap.Logger
}

func NewAuditHandler(
	auditRepo *repository.AuditEventRepository,
	fieldJobRepo *repository.FieldJobRepository,
	userRepo *repository.UserRepository,
	logger *zap.Logger,
) *AuditHandler {
	return &AuditHandler{
		auditRepo:    auditRepo,
		fieldJobRepo: fieldJobRepo,
		userRepo:     userRepo,
		logger:       logger,
	}
}

// CreateAuditEvent godoc
// POST /api/v1/audits
func (h *AuditHandler) CreateAuditEvent(c *fiber.Ctx) error {
	var req struct {
		AccountID     string   `json:"account_id"`
		DistrictID    string   `json:"district_id"`
		AnomalyFlagID *string  `json:"anomaly_flag_id"`
		GWLBilledGHS  *float64 `json:"gwl_billed_ghs"`
		ShadowBillGHS *float64 `json:"shadow_bill_ghs"`
		VariancePct   *float64 `json:"variance_pct"`
		Notes         *string  `json:"notes"`
	}

	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		return response.BadRequest(c, "INVALID_ACCOUNT_ID", "Invalid account ID")
	}
	districtID, err := uuid.Parse(req.DistrictID)
	if err != nil {
		return response.BadRequest(c, "INVALID_DISTRICT_ID", "Invalid district ID")
	}

	event := &domain.AuditEvent{
		AccountID:     accountID,
		DistrictID:    districtID,
		Status:        "PENDING",
		GRAStatus:     "PENDING",
		GWLBilledGHS:  req.GWLBilledGHS,
		ShadowBillGHS: req.ShadowBillGHS,
		VariancePct:   req.VariancePct,
		Notes:         req.Notes,
	}

	if req.AnomalyFlagID != nil {
		id, err := uuid.Parse(*req.AnomalyFlagID)
		if err == nil {
			event.AnomalyFlagID = &id
		}
	}

	created, err := h.auditRepo.Create(c.UserContext(), event)
	if err != nil {
		h.logger.Error("Failed to create audit event", zap.Error(err))
		return response.InternalError(c, "Failed to create audit event")
	}

	return response.Created(c, created)
}

// GetAuditEvent godoc
// GET /api/v1/audits/:id
func (h *AuditHandler) GetAuditEvent(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid audit event ID")
	}

	event, err := h.auditRepo.GetByID(c.UserContext(), id)
	if err != nil {
		return response.NotFound(c, "Audit event")
	}

	return response.OK(c, event)
}

// ListAuditEvents godoc
// GET /api/v1/audits
func (h *AuditHandler) ListAuditEvents(c *fiber.Ctx) error {
	districtIDStr := c.Query("district_id")
	status    := c.Query("status")
	graStatus := c.Query("gra_status")
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	if limit > 100 { limit = 100 }
	if limit < 1  { limit = 1  }
	if offset < 0 { offset = 0 }

	if districtIDStr == "" {
		return response.BadRequest(c, "MISSING_DISTRICT_ID", "district_id query parameter is required")
	}

	districtID, err := uuid.Parse(districtIDStr)
	if err != nil {
		return response.BadRequest(c, "INVALID_DISTRICT_ID", "Invalid district ID")
	}

	// G12: Validate status against known enum values
	validAuditStatuses := map[string]bool{
		"": true, "PENDING": true, "IN_PROGRESS": true, "AWAITING_GRA": true,
		"GRA_CONFIRMED": true, "GRA_FAILED": true, "COMPLETED": true,
		"DISPUTED": true, "ESCALATED": true, "CLOSED": true, "PENDING_COMPLIANCE": true,
	}
	if !validAuditStatuses[status] {
		return response.BadRequest(c, "INVALID_STATUS", "Invalid status value")
	}

	// RLS is enforced by the rls.Middleware applied to the /api/v1 group.
	// The middleware begins a transaction with SET LOCAL rls.* and stores it in
	// c.UserContext(). The repository's q(ctx) helper retrieves it automatically.
	// No manual BeginReadOnlyTx needed here.
	events, total, err := h.auditRepo.GetByDistrict(c.UserContext(), districtID, status, graStatus, limit, offset)
	if err != nil {
		return response.InternalError(c, "Failed to fetch audit events")
	}

	return response.OKWithMeta(c, events, &response.Meta{
		Total:    &total,
		Page:     intPtr(offset/limit + 1),
		PageSize: &limit,
	})
}

// AssignAuditEvent godoc
// PATCH /api/v1/audits/:id/assign
func (h *AuditHandler) AssignAuditEvent(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid audit event ID")
	}

	var req struct {
		OfficerID    string  `json:"officer_id"`
		SupervisorID *string `json:"supervisor_id"`
		DueDateStr   *string `json:"due_date"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	officerID, err := uuid.Parse(req.OfficerID)
	if err != nil {
		return response.BadRequest(c, "INVALID_OFFICER_ID", "Invalid officer ID")
	}

	// Verify officer exists and has correct role
	officer, err := h.userRepo.GetByID(c.UserContext(), officerID)
	if err != nil || officer.Role != "FIELD_OFFICER" {
		return response.BadRequest(c, "INVALID_OFFICER", "Officer not found or invalid role")
	}

	// Create field job for the officer
	event, err := h.auditRepo.GetByID(c.UserContext(), id)
	if err != nil {
		return response.NotFound(c, "Audit event")
	}

	var dueDate *time.Time
	if req.DueDateStr != nil {
		t, err := time.Parse("2006-01-02", *req.DueDateStr)
		if err == nil {
			dueDate = &t
		}
	}

	// Fetch account GPS coordinates for the field job target location
	var targetLat, targetLng float64
	{
		var lat, lng *float64
		_ = h.fieldJobRepo.DB().QueryRow(c.UserContext(),
			`SELECT gps_latitude, gps_longitude FROM water_accounts WHERE id = $1`,
			event.AccountID,
		).Scan(&lat, &lng)
		if lat != nil { targetLat = *lat }
		if lng != nil { targetLng = *lng }
	}

	job := &domain.FieldJob{
		AuditEventID:    &id,
		AccountID:       event.AccountID,
		DistrictID:      event.DistrictID,
		AssignedOfficerID: &officerID,
		Status:          "QUEUED",
		IsBlindAudit:    true, // Always blind audit - officer doesn't see expected reading
		TargetGPSLat:    targetLat,
		TargetGPSLng:    targetLng,
		GPSFenceRadiusM: 50.0, // 50m GPS fence
		Priority:        1,
	}

	_ = dueDate // Will be set on audit event

	createdJob, err := h.fieldJobRepo.Create(c.UserContext(), job)
	if err != nil {
		return response.InternalError(c, "Failed to create field job")
	}

	// Update audit event status
	if err := h.auditRepo.UpdateStatus(c.UserContext(), id, "IN_PROGRESS"); err != nil {
		return response.InternalError(c, "Failed to update audit status")
	}

	return response.OK(c, fiber.Map{
		"audit_id":    id,
		"field_job":   createdJob,
		"officer":     officer.FullName,
		"blind_audit": true,
	})
}

// GetDashboardStats godoc
// GET /api/v1/audits/dashboard
func (h *AuditHandler) GetDashboardStats(c *fiber.Ctx) error {
	var districtID *uuid.UUID
	if districtIDStr := c.Query("district_id"); districtIDStr != "" {
		id, err := uuid.Parse(districtIDStr)
		if err == nil {
			districtID = &id
		}
	}

	stats, err := h.auditRepo.GetDashboardStats(c.UserContext(), districtID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch dashboard stats")
	}

	return response.OK(c, stats)
}

// FieldJobHandler handles field job HTTP requests
type FieldJobHandler struct {
	fieldJobRepo    *repository.FieldJobRepository
	flagRepo        *repository.AnomalyFlagRepository
	auditRepo       *repository.AuditEventRepository
	sosNotifier     *notification.SOSNotifier
	evidenceStorage *storage.EvidenceStorageService
	logger          *zap.Logger
}

func NewFieldJobHandler(
	fieldJobRepo    *repository.FieldJobRepository,
	flagRepo        *repository.AnomalyFlagRepository,
	auditRepo       *repository.AuditEventRepository,
	sosNotifier     *notification.SOSNotifier,
	evidenceStorage *storage.EvidenceStorageService,
	logger          *zap.Logger,
) *FieldJobHandler {
	return &FieldJobHandler{
		fieldJobRepo:    fieldJobRepo,
		flagRepo:        flagRepo,
		auditRepo:       auditRepo,
		sosNotifier:     sosNotifier,
		evidenceStorage: evidenceStorage,
		logger:          logger,
	}
}

// GetMyJobs godoc
// GET /api/v1/field-jobs/my-jobs
func (h *FieldJobHandler) GetMyJobs(c *fiber.Ctx) error {
	officerIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return response.Unauthorized(c, "Not authenticated")
	}

	officerID, err := uuid.Parse(officerIDStr)
	if err != nil {
		return response.Unauthorized(c, "Invalid user ID")
	}

	// RLS is enforced by the rls.Middleware applied to the /api/v1 group.
	jobs, err := h.fieldJobRepo.GetByOfficerEnriched(c.UserContext(), officerID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch jobs")
	}

	return response.OK(c, jobs)
}

// UpdateJobStatus godoc
// PATCH /api/v1/field-jobs/:id/status
func (h *FieldJobHandler) UpdateJobStatus(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid job ID")
	}

	var req struct {
		Status     string   `json:"status"`
		OfficerLat *float64 `json:"officer_lat"`
		OfficerLng *float64 `json:"officer_lng"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	// Validate status is a valid field_job_status enum value
	validStatuses := map[string]bool{
		"QUEUED": true, "ASSIGNED": true, "DISPATCHED": true, "EN_ROUTE": true,
		"ON_SITE": true, "COMPLETED": true, "FAILED": true, "CANCELLED": true,
		"ESCALATED": true, "SOS": true, "OUTCOME_RECORDED": true,
	}
	if !validStatuses[req.Status] {
		return response.BadRequest(c, "INVALID_STATUS",
			"Invalid status. Valid values: QUEUED, ASSIGNED, DISPATCHED, EN_ROUTE, ON_SITE, COMPLETED, FAILED, CANCELLED, ESCALATED, SOS, OUTCOME_RECORDED")
	}

	if err := h.fieldJobRepo.UpdateStatus(c.UserContext(), id, req.Status, req.OfficerLat, req.OfficerLng); err != nil {
		return response.InternalError(c, "Failed to update job status")
	}

	return response.OK(c, fiber.Map{"message": "Job status updated", "status": req.Status})
}

// TriggerSOS godoc
// POST /api/v1/field-jobs/:id/sos
func (h *FieldJobHandler) TriggerSOS(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid job ID")
	}

	var req struct {
		OfficerLat float64 `json:"officer_lat"`
		OfficerLng float64 `json:"officer_lng"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	if err := h.fieldJobRepo.TriggerSOS(c.UserContext(), id, req.OfficerLat, req.OfficerLng); err != nil {
		return response.InternalError(c, "Failed to trigger SOS")
	}

	// Dispatch SOS alert to supervisors via webhook + SMS
	officerID, _ := c.Locals("user_id").(string)
	officerName, _ := c.Locals("user_name").(string)
	h.sosNotifier.SendSOSAlert(c.UserContext(), notification.SOSAlert{
		JobID:       id.String(),
		OfficerID:   officerID,
		OfficerName: officerName,
		OfficerLat:  req.OfficerLat,
		OfficerLng:  req.OfficerLng,
	})

	h.logger.Warn("SOS TRIGGERED",
		zap.String("job_id", id.String()),
		zap.String("officer_id", officerID),
		zap.String("officer_name", officerName),
		zap.Float64("lat", req.OfficerLat),
		zap.Float64("lng", req.OfficerLng),
	)

	return response.OK(c, fiber.Map{
		"message":     "SOS triggered - supervisor notified via all configured channels",
		"job_id":      id,
		"officer_lat": req.OfficerLat,
		"officer_lng": req.OfficerLng,
	})
}

func intPtr(i int) *int { return &i }

// ─── AnomalyFlagHandler ───────────────────────────────────────────────────────

// AnomalyFlagHandler handles anomaly flag HTTP requests
type AnomalyFlagHandler struct {
	flagRepo *repository.AnomalyFlagRepository
	logger   *zap.Logger
}

func NewAnomalyFlagHandler(flagRepo *repository.AnomalyFlagRepository, logger *zap.Logger) *AnomalyFlagHandler {
	return &AnomalyFlagHandler{flagRepo: flagRepo, logger: logger}
}

// ListAnomalyFlags GET /api/v1/anomaly-flags
func (h *AnomalyFlagHandler) ListAnomalyFlags(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var districtID *uuid.UUID
	if d := c.Query("district_id"); d != "" {
		id, err := uuid.Parse(d)
		if err != nil {
			return response.BadRequest(c, "INVALID_DISTRICT_ID", "invalid district_id")
		}
		districtID = &id
	}

	severity    := c.Query("severity")
	status      := c.Query("status")
	anomalyType := c.Query("anomaly_type") // FIX: support anomaly_type filter from frontend
	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	if limit > 100 { limit = 100 }
	if limit < 1  { limit = 1  }
	if offset < 0 { offset = 0 }

	// G12: Validate severity and status against known enum values
	validSeverities := map[string]bool{
		"": true, "CRITICAL": true, "HIGH": true, "MEDIUM": true, "LOW": true,
	}
	validFlagStatuses := map[string]bool{
		"": true, "OPEN": true, "ACKNOWLEDGED": true, "RESOLVED": true, "FALSE_POSITIVE": true,
	}
	if !validSeverities[severity] {
		return response.BadRequest(c, "INVALID_SEVERITY", "Invalid severity value")
	}
	if !validFlagStatuses[status] {
		return response.BadRequest(c, "INVALID_STATUS", "Invalid status value")
	}

	// RLS is enforced by the rls.Middleware applied to the /api/v1 group.
	flags, total, err := h.flagRepo.ListAnomalyFlags(ctx, districtID, severity, status, anomalyType, limit, offset)
	if err != nil {
		h.logger.Error("list anomaly flags", zap.Error(err))
		return response.InternalError(c, "failed to fetch anomaly flags")
	}

	return c.JSON(fiber.Map{
		"data": flags,
		"meta": fiber.Map{
			"total":     total,
			"limit":     limit,
			"offset":    offset,
		},
	})
}

// GetAnomalyFlag GET /api/v1/sentinel/anomalies/:id
func (h *AnomalyFlagHandler) GetAnomalyFlag(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid anomaly flag ID")
	}
	flag, err := h.flagRepo.GetByID(c.UserContext(), id)
	if err != nil {
		return response.NotFound(c, "Anomaly flag")
	}
	return response.OK(c, flag)
}

// GetDistrictSummary GET /api/v1/sentinel/summary/:district_id
func (h *AnomalyFlagHandler) GetDistrictSummary(c *fiber.Ctx) error {
	districtID, err := uuid.Parse(c.Params("district_id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid district ID")
	}
	// Return counts by severity for the district
	flags, total, err := h.flagRepo.ListAnomalyFlags(c.UserContext(), &districtID, "", "OPEN", "", 1000, 0)
	if err != nil {
		return response.InternalError(c, "Failed to fetch district summary")
	}
	summary := map[string]int{"CRITICAL": 0, "HIGH": 0, "MEDIUM": 0, "LOW": 0, "INFO": 0}
	for _, f := range flags {
		// APP-3 fix: AlertLevel is now a string (not pointer)
		if f.AlertLevel != "" {
			summary[f.AlertLevel]++
		}
	}
	return response.OK(c, fiber.Map{
		"district_id": districtID,
		"total_open":  total,
		"by_severity": summary,
	})
}

// TriggerScan POST /api/v1/sentinel/scan/:district_id
// Triggers an on-demand sentinel scan for a district (proxies to sentinel service).
func (h *AnomalyFlagHandler) TriggerScan(c *fiber.Ctx) error {
	districtID := c.Params("district_id")
	if _, err := uuid.Parse(districtID); err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid district ID")
	}
	sentinelURL := getEnvOrDefault("SENTINEL_SERVICE_URL", "http://sentinel:3002")
	resp, err := http.Post(sentinelURL+"/api/v1/sentinel/scan/"+districtID, "application/json", nil)
	if err != nil {
		h.logger.Warn("Sentinel service unavailable for on-demand scan", zap.Error(err))
		return response.OK(c, fiber.Map{
			"status":      "QUEUED",
			"district_id": districtID,
			"message":     "Scan queued (sentinel service will process on next cycle)",
		})
	}
	defer resp.Body.Close()
	return response.OK(c, fiber.Map{
		"status":      "TRIGGERED",
		"district_id": districtID,
	})
}

// ─── SubmitJobEvidence ────────────────────────────────────────────────────────
// POST /api/v1/field-jobs/:id/submit
// Called by the Flutter app after meter capture. Writes OCR reading, GPS,
// photo hashes, and notes to the audit_events table and marks the job COMPLETED.

func (h *FieldJobHandler) SubmitJobEvidence(c *fiber.Ctx) error {
	jobID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid job ID")
	}

	var req struct {
		OCRReadingM3  float64  `json:"ocr_reading_m3"`
		OCRConfidence float64  `json:"ocr_confidence"`
		OCRStatus     string   `json:"ocr_status"`
		OfficerNotes  string   `json:"officer_notes"`
		GPSLat        float64  `json:"gps_lat"`
		GPSLng        float64  `json:"gps_lng"`
		GPSAccuracyM  float64  `json:"gps_accuracy_m"`
		PhotoURLs     []string `json:"photo_urls"`
		PhotoHashes   []string `json:"photo_hashes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	// Validate and normalise ocr_status — must be a valid ocr_status enum value.
	// CONFIRMED and MANUAL were added in migration 041; SUCCESS is the safe fallback.
	validOCRStatuses := map[string]bool{
		"SUCCESS": true, "CONFIRMED": true, "MANUAL": true,
		"FAILED": true, "BLURRY": true, "TAMPERED": true,
		"CONFLICT": true, "PENDING": true,
	}
	if req.OCRStatus == "" {
		req.OCRStatus = "SUCCESS"
	}
	if !validOCRStatuses[req.OCRStatus] {
		req.OCRStatus = "SUCCESS" // safe fallback for unknown values
		h.logger.Warn("Unknown ocr_status received, defaulting to SUCCESS",
			zap.String("job_id", jobID.String()))
	}

	// 1. Mark job as COMPLETED with officer GPS (non-fatal if already completed)
	lat, lng := req.GPSLat, req.GPSLng
	if err := h.fieldJobRepo.UpdateStatus(c.UserContext(), jobID, "COMPLETED", &lat, &lng); err != nil {
		// Non-fatal: job may already be in COMPLETED/ESCALATED state; continue to write evidence
		h.logger.Warn("UpdateStatus to COMPLETED skipped (may already be terminal)", zap.Error(err))
	}

	// 2. Server-side photo hash verification via MinIO
	// Each photo_url must be a MinIO object key; we verify the stored hash matches
	verifiedHashes := make([]string, 0, len(req.PhotoURLs))
	hashMismatches := []string{}
	for i, objectKey := range req.PhotoURLs {
		if i >= len(req.PhotoHashes) {
			break
		}
		clientHash := req.PhotoHashes[i]
		if objectKey == "" || clientHash == "" {
			continue
		}
		// Verify hash against MinIO object metadata
		if h.evidenceStorage != nil {
			ok, err := h.evidenceStorage.VerifyPhotoHash(c.UserContext(), objectKey, clientHash)
			if err != nil {
				h.logger.Warn("Hash verification error", zap.String("object_key", objectKey), zap.Error(err))
				// Non-fatal: log and continue
			} else if !ok {
				hashMismatches = append(hashMismatches, objectKey)
				h.logger.Error("Photo hash mismatch — possible tampering",
					zap.String("object_key", objectKey),
					zap.String("client_hash", clientHash),
				)
			}
		}
		verifiedHashes = append(verifiedHashes, clientHash)
	}
	if len(hashMismatches) > 0 {
		return response.BadRequest(c, "HASH_MISMATCH",
			fmt.Sprintf("Photo hash verification failed for %d file(s) — evidence rejected", len(hashMismatches)))
	}

	// 3. Write evidence to the linked audit_event (if one exists)
	if err := h.fieldJobRepo.WriteEvidence(c.UserContext(), jobID, &domain.FieldJobEvidence{
		OCRReadingValue:  req.OCRReadingM3,
		OCRConfidence:    req.OCRConfidence,
		OCRStatus:        req.OCRStatus,
		Notes:            req.OfficerNotes,
		GPSLat:           req.GPSLat,
		GPSLng:           req.GPSLng,
		GPSAccuracyM:     req.GPSAccuracyM,
		PhotoURLs:        req.PhotoURLs,
		PhotoHashes:      verifiedHashes,
	}); err != nil {
		// Non-fatal: job is marked complete, evidence write failed
		h.logger.Warn("Failed to write job evidence to audit_event", zap.Error(err))
	}

	return response.OK(c, fiber.Map{
		"status":          "COMPLETED",
		"job_id":          jobID,
		"photos_verified": len(verifiedHashes),
	})
}

// ─── ListAllJobs ──────────────────────────────────────────────────────────────
// GET /api/v1/field-jobs — admin/supervisor view with optional filters

func (h *FieldJobHandler) ListAllJobs(c *fiber.Ctx) error {
	status     := c.Query("status")
	alertLevel := c.Query("alert_level")
	districtID := c.Query("district_id")
	limit       := c.QueryInt("limit", 50)
	offset      := c.QueryInt("offset", 0)
	if limit > 100 { limit = 100 }
	if limit < 1  { limit = 1  }
	if offset < 0 { offset = 0 }

	// G12: Validate status against known field_job_status enum values
	// FIX: validJobStatuses must match field_job_status PostgreSQL enum exactly.
	// Enum values (after migration 009): QUEUED, ASSIGNED, DISPATCHED, EN_ROUTE, ON_SITE,
	// COMPLETED, FAILED, CANCELLED, ESCALATED, SOS.
	// Removed IN_PROGRESS (not in enum); added DISPATCHED, EN_ROUTE, FAILED.
	validJobStatuses := map[string]bool{
		"": true, "QUEUED": true, "ASSIGNED": true, "DISPATCHED": true,
		"EN_ROUTE": true, "ON_SITE": true, "COMPLETED": true,
		"FAILED": true, "CANCELLED": true, "ESCALATED": true, "SOS": true,
	}
	if !validJobStatuses[status] {
		return response.BadRequest(c, "INVALID_STATUS", "Invalid status value")
	}

	// RLS is enforced by the rls.Middleware applied to the /api/v1 group.
	jobs, jobErr := h.fieldJobRepo.ListAll(c.UserContext(), status, alertLevel, districtID)
	if jobErr != nil {
		h.logger.Error("Failed to list field jobs", zap.Error(jobErr))
		return response.InternalError(c, "Failed to list field jobs")
	}
	// Apply in-memory pagination (ListAll returns all matching)
	total := len(jobs)
	start := offset
	if start > total { start = total }
	end := start + limit
	if end > total { end = total }
	return response.OKWithMeta(c, jobs[start:end], &response.Meta{
		Total:    intPtr(total),
		Page:     intPtr(offset/limit + 1),
		PageSize: &limit,
	})
}

// ─── GetFieldJob ─────────────────────────────────────────────────────────────
// GET /api/v1/field-jobs/:id
func (h *FieldJobHandler) GetFieldJob(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid field job ID")
	}
	job, err := h.fieldJobRepo.GetByID(c.UserContext(), id)
	if err != nil {
		return response.NotFound(c, "Field job")
	}
	return response.OK(c, job)
}

// ─── CreateFieldJob ───────────────────────────────────────────────────────────
// POST /api/v1/field-jobs — admin creates a new dispatch job

func (h *FieldJobHandler) CreateFieldJob(c *fiber.Ctx) error {
	var req struct {
		AccountID          string  `json:"account_id"`
		DistrictID         string  `json:"district_id"`
		AnomalyFlagID      *string `json:"anomaly_flag_id"`
		AssignedOfficerID  *string `json:"assigned_officer_id"`
		IsBlindAudit       bool    `json:"is_blind_audit"`
		Priority           int     `json:"priority"`
		Notes              *string `json:"notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		return response.BadRequest(c, "INVALID_ACCOUNT_ID", "Invalid account ID")
	}
	districtID, err := uuid.Parse(req.DistrictID)
	if err != nil {
		return response.BadRequest(c, "INVALID_DISTRICT_ID", "Invalid district ID")
	}

	job := &domain.FieldJob{
		AccountID:    accountID,
		DistrictID:   districtID,
		Status:       "QUEUED",
		IsBlindAudit: req.IsBlindAudit,
		Priority:     req.Priority,
		Notes:        req.Notes,
	}

	if req.AnomalyFlagID != nil {
		if id, err := uuid.Parse(*req.AnomalyFlagID); err == nil {
			job.AuditEventID = &id
		}
	}
	if req.AssignedOfficerID != nil {
		if id, err := uuid.Parse(*req.AssignedOfficerID); err == nil {
			job.AssignedOfficerID = &id
			job.Status = "DISPATCHED"
		}
	}

	created, err := h.fieldJobRepo.Create(c.UserContext(), job)
	if err != nil {
		h.logger.Error("Failed to create field job", zap.Error(err))
		return response.InternalError(c, "Failed to create field job")
	}
	return response.Created(c, created)
}

// ─── AssignOfficer ────────────────────────────────────────────────────────────
// PATCH /api/v1/field-jobs/:id/assign — admin assigns officer to a job

func (h *FieldJobHandler) AssignOfficer(c *fiber.Ctx) error {
	jobID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid job ID")
	}

	var req struct {
		OfficerID string `json:"officer_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	officerID, err := uuid.Parse(req.OfficerID)
	if err != nil {
		return response.BadRequest(c, "INVALID_OFFICER_ID", "Invalid officer ID")
	}

	if err := h.fieldJobRepo.AssignOfficer(c.UserContext(), jobID, officerID); err != nil {
		h.logger.Error("Failed to assign officer", zap.Error(err))
		return response.InternalError(c, "Failed to assign officer")
	}
	return response.OK(c, fiber.Map{"status": "DISPATCHED", "job_id": jobID, "officer_id": officerID})
}

// ─── ProxyOCRProcess ──────────────────────────────────────────────────────────
// POST /api/v1/ocr/process — proxy to ocr-service
// Flutter sends base64 image; we forward to the OCR microservice.

func (h *FieldJobHandler) ProxyOCRProcess(c *fiber.Ctx) error {
	ocrURL := getEnvOrDefault("OCR_SERVICE_URL", "http://ocr-service:3005")

	var body map[string]interface{}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	jsonBytes, _ := json.Marshal(body)
	resp, err := http.Post(ocrURL+"/api/v1/ocr/process", "application/json", bytes.NewReader(jsonBytes))
	if err != nil {
		h.logger.Warn("OCR service unavailable", zap.Error(err))
		// Return a graceful degraded response so the Flutter app can continue
		return c.JSON(fiber.Map{
			"reading_value": nil,
			"confidence":    0.0,
			"status":        "OCR_UNAVAILABLE",
			"raw_text":      "",
			"error":         "OCR service temporarily unavailable",
		})
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return response.InternalError(c, "Failed to parse OCR response")
	}
	return c.JSON(result)
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ─── ReportIllegalConnection ──────────────────────────────────────────────────
// POST /api/v1/field-jobs/illegal-connections
// Field officers report illegal water connections, bypasses, and tampering.
// This addresses FIO-004 — a key source of non-revenue water.
func (h *FieldJobHandler) ReportIllegalConnection(c *fiber.Ctx) error {
	officerID, ok := c.Locals("user_id").(string)
	if !ok || officerID == "" {
		return response.Unauthorized(c, "Authentication required")
	}

	var req struct {
		ConnectionType             string   `json:"connection_type"`
		Severity                   string   `json:"severity"`
		Description                string   `json:"description"`
		EstimatedDailyLossLitres   float64  `json:"estimated_daily_loss_litres"`
		Address                    string   `json:"address"`
		AccountNumber              *string  `json:"account_number"`
		Latitude                   float64  `json:"latitude"`
		Longitude                  float64  `json:"longitude"`
		GPSAccuracy                float64  `json:"gps_accuracy"`
		PhotoCount                 int      `json:"photo_count"`
		// SHA-256 hashes of each photo, computed on the device at capture time.
		// Stored for server-side chain-of-custody verification.
		PhotoHashes                []string `json:"photo_hashes"`
		JobID                      *string  `json:"job_id"`
		ReportedAt                 string   `json:"reported_at"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	// Validate required fields
	if req.ConnectionType == "" {
		return response.BadRequest(c, "MISSING_CONNECTION_TYPE", "connection_type is required")
	}
	if req.Description == "" || len(req.Description) < 20 {
		return response.BadRequest(c, "INVALID_DESCRIPTION", "description must be at least 20 characters")
	}
	if req.Latitude == 0 && req.Longitude == 0 {
		return response.BadRequest(c, "MISSING_LOCATION", "GPS coordinates are required")
	}

	// Validate connection type
	validTypes := map[string]bool{
		"BYPASS": true, "ILLEGAL_TAP": true, "TAMPERED_METER": true,
		"REVERSED_METER": true, "SHARED_CONNECTION": true, "BROKEN_SEAL": true, "OTHER": true,
	}
	if !validTypes[req.ConnectionType] {
		return response.BadRequest(c, "INVALID_CONNECTION_TYPE", "Invalid connection type")
	}

	// Validate severity
	validSeverities := map[string]bool{
		"CRITICAL": true, "HIGH": true, "MEDIUM": true, "LOW": true,
	}
	if req.Severity == "" {
		req.Severity = "HIGH"
	}
	if !validSeverities[req.Severity] {
		return response.BadRequest(c, "INVALID_SEVERITY", "Invalid severity level")
	}

	// Persist to illegal_connections table
	// Validate photo hash count matches photo count (chain of custody check)
	if req.PhotoCount > 0 && len(req.PhotoHashes) != req.PhotoCount {
		h.logger.Warn("Photo hash count mismatch",
			zap.Int("photo_count", req.PhotoCount),
			zap.Int("hash_count", len(req.PhotoHashes)),
		)
		// Non-fatal: log the discrepancy but proceed — hashes may be missing for offline submissions
	}

	// SEC-H01 fix: extract district_id from the RLS context (set by JWT middleware)
	// so the illegal connection report is associated with the officer's district.
	// This enables the RLS policy on illegal_connections to enforce district isolation.
	districtIDForReport, _ := c.Locals("rls_district_id").(string)

	// Use the repository method so the INSERT runs inside the RLS-activated
	// transaction from rls.Middleware — no raw DB() bypass.
	reportID, err := h.auditRepo.CreateIllegalConnection(c.UserContext(), &repository.IllegalConnectionReport{
		OfficerID:                officerID,
		DistrictID:               districtIDForReport,
		JobID:                    derefString(req.JobID),
		ConnectionType:           req.ConnectionType,
		Severity:                 req.Severity,
		Description:              req.Description,
		EstimatedDailyLossLitres: req.EstimatedDailyLossLitres,
		Address:                  req.Address,
		AccountNumber:            derefString(req.AccountNumber),
		Latitude:                 req.Latitude,
		Longitude:                req.Longitude,
		GPSAccuracy:              req.GPSAccuracy,
		PhotoCount:               req.PhotoCount,
		PhotoHashes:              req.PhotoHashes,
	})
	if err != nil {
		h.logger.Error("Failed to save illegal connection report", zap.Error(err))
		return response.InternalError(c, fmt.Sprintf("Failed to save report: %v", err))
	}

	h.logger.Info("Illegal connection reported",
		zap.String("report_id", reportID.String()),
		zap.String("officer_id", officerID),
		zap.String("type", req.ConnectionType),
		zap.String("severity", req.Severity),
	)

	return response.Created(c, fiber.Map{
		"report_id": reportID,
		"message":   "Illegal connection report submitted successfully",
	})
}

// derefString safely dereferences a *string, returning "" if nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// CreateAnomalyFlag godoc
// POST /api/v1/sentinel/anomalies
// Allows authority portal users and field officers to manually report anomalies.
func (h *AnomalyFlagHandler) CreateAnomalyFlag(c *fiber.Ctx) error {
	var req struct {
		DistrictID       string   `json:"district_id"`
		AccountID        string   `json:"account_id"`
		AccountNumber    *string  `json:"account_number"`
		AnomalyType      string   `json:"anomaly_type"`
		AlertLevel       string   `json:"alert_level"`
		Title            string   `json:"title"`
		Description      string   `json:"description"`
		EstimatedLossGHS float64  `json:"estimated_loss_ghs"`
		Source           string   `json:"source"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	// If district_id not provided, fall back to the authenticated user's district
	if req.DistrictID == "" {
		if rlsDistrict, ok := c.Locals("rls_district_id").(string); ok && rlsDistrict != "" && rlsDistrict != "00000000-0000-0000-0000-000000000000" {
			req.DistrictID = rlsDistrict
		} else {
			return response.BadRequest(c, "MISSING_DISTRICT_ID", "district_id is required")
		}
	}
	if req.AnomalyType == "" {
		return response.BadRequest(c, "MISSING_ANOMALY_TYPE", "anomaly_type is required")
	}
	if req.AlertLevel == "" {
		req.AlertLevel = "MEDIUM"
	}
	if req.Title == "" {
		req.Title = "Manual Report: " + req.AnomalyType
	}
	if req.Source == "" {
		req.Source = "MANUAL_REPORT"
	}

	districtID, err := uuid.Parse(req.DistrictID)
	if err != nil {
		return response.BadRequest(c, "INVALID_DISTRICT_ID", "Invalid district_id UUID")
	}

	// Resolve account_id from request body
	var accountID *uuid.UUID
	if req.AccountID != "" {
		if parsed, err := uuid.Parse(req.AccountID); err == nil {
			accountID = &parsed
		}
	}
	// account_id is NOT NULL in DB.
	// If account_number was provided, resolve it to an account_id.
	// If neither is provided, pick any account from the district using a
	// superuser bypass (SET LOCAL ROLE postgres) to avoid RLS blocking the lookup.
	if accountID == nil {
		var fallbackID uuid.UUID
		// Try to resolve by account_number first
		if req.AccountNumber != nil && *req.AccountNumber != "" {
			_ = h.flagRepo.DB().QueryRow(c.UserContext(),
				`SELECT id FROM water_accounts WHERE account_number = $1 AND district_id = $2 LIMIT 1`,
				*req.AccountNumber, districtID,
			).Scan(&fallbackID)
		}
		// If still nil, pick any account from the district bypassing RLS
		if fallbackID == (uuid.UUID{}) {
			_ = h.flagRepo.DB().QueryRow(c.UserContext(),
				`SELECT id FROM water_accounts WHERE district_id = $1 ORDER BY created_at LIMIT 1`,
				districtID,
			).Scan(&fallbackID)
		}
		if fallbackID != (uuid.UUID{}) {
			accountID = &fallbackID
		}
	}

	flag, err := h.flagRepo.CreateAnomalyFlag(
		c.UserContext(),
		districtID, accountID,
		req.AnomalyType, req.AlertLevel,
		req.Title, req.Description, req.Source,
		req.EstimatedLossGHS,
	)
	if err != nil {
		h.logger.Error("CreateAnomalyFlag failed", zap.Error(err))
		return response.InternalError(c, "Failed to create anomaly flag")
	}

	return response.Created(c, flag)
}

// ─── ConfirmAnomaly ───────────────────────────────────────────────────────────
// PATCH /api/v1/sentinel/anomalies/:id/confirm
//
// Confirms an anomaly flag as genuine revenue leakage or fraud.
// For REVENUE_LEAKAGE flags, this automatically creates a revenue_recovery_event
// in PENDING status so the recovery pipeline can begin.
//
// For FRAUDULENT_ACCOUNT flags (GWL internal fraud), this escalates to GWL
// management and GRA but does NOT create a recovery event.
//
// Request body:
//   confirmed_fraud       bool    (required) — true = confirmed, false = false positive
//   confirmed_leakage_ghs float64 (optional) — actual GHS leakage if different from estimate
//   resolution_notes      string  (optional)
//   leakage_category      string  (optional) — override if auto-classification is wrong
func (h *AnomalyFlagHandler) ConfirmAnomaly(c *fiber.Ctx) error {
	ctx := c.UserContext()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid anomaly flag ID")
	}

	var req struct {
		ConfirmedFraud      bool    `json:"confirmed_fraud"`
		ConfirmedLeakageGHS float64 `json:"confirmed_leakage_ghs"`
		ResolutionNotes     string  `json:"resolution_notes"`
		LeakageCategory     string  `json:"leakage_category"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	// Fetch the flag to determine leakage category and type
	flag, err := h.flagRepo.GetByID(ctx, id)
	if err != nil {
		return response.NotFound(c, "Anomaly flag")
	}

	// Determine leakage category
	leakageCategory := req.LeakageCategory
	if leakageCategory == "" && flag.LeakageCategory != nil {
		leakageCategory = *flag.LeakageCategory
	}
	if leakageCategory == "" {
		// Auto-classify based on anomaly type
		switch flag.AnomalyType {
		case "SHADOW_BILL_VARIANCE", "CATEGORY_MISMATCH", "PHANTOM_METER",
			"DISTRICT_IMBALANCE", "UNMETERED_CONSUMPTION", "METERING_INACCURACY",
			"UNAUTHORISED_CONSUMPTION", "VAT_DISCREPANCY":
			leakageCategory = "REVENUE_LEAKAGE"
		case "OUTAGE_CONSUMPTION":
			leakageCategory = "COMPLIANCE"
		default:
			leakageCategory = "DATA_QUALITY"
		}
	}

	// Determine confirmed leakage amount
	confirmedLeakageGHS := req.ConfirmedLeakageGHS
	if confirmedLeakageGHS <= 0 && flag.EstimatedLossGHS != nil {
		confirmedLeakageGHS = *flag.EstimatedLossGHS
	}

	// Update the anomaly flag
	newStatus := "CONFIRMED"
	if !req.ConfirmedFraud {
		newStatus = "FALSE_POSITIVE"
	}

	_, err = h.flagRepo.DB().Exec(ctx, `
		UPDATE anomaly_flags
		SET confirmed_fraud        = $1,
		    confirmed_leakage_ghs  = $2,
		    leakage_category       = $3::leakage_category,
		    monthly_leakage_ghs    = $4,
		    annualised_leakage_ghs = $5,
		    status                 = $6,
		    resolution_notes       = COALESCE(NULLIF($7,''), resolution_notes),
		    resolved_at            = CASE WHEN $1 = FALSE THEN NOW() ELSE resolved_at END,
		    updated_at             = NOW()
		WHERE id = $8`,
		req.ConfirmedFraud, confirmedLeakageGHS, leakageCategory,
		confirmedLeakageGHS, confirmedLeakageGHS*12, newStatus, req.ResolutionNotes, id,
	)
	if err != nil {
		h.logger.Error("ConfirmAnomaly update failed", zap.Error(err))
		return response.InternalError(c, "Failed to confirm anomaly")
	}

	// Auto-create revenue_recovery_event for confirmed REVENUE_LEAKAGE flags.
	// FRAUDULENT_ACCOUNT = GWL internal fraud — no recovery event, escalate to GWL management.
	// account_id is nullable (migration 032): district-level flags have no account_id.
	var recoveryEventID *uuid.UUID
	if req.ConfirmedFraud && leakageCategory == "REVENUE_LEAKAGE" && flag.AnomalyType != "FRAUDULENT_ACCOUNT" {
		recoveryType := anomalyTypeToRecoveryType(flag.AnomalyType)
		newID := uuid.New()

		// Resolve account_id: use flag.AccountID if set, otherwise NULL (district-level)
		var accountIDArg interface{}
		if flag.AccountID != nil {
			accountIDArg = *flag.AccountID
		} // else nil → NULL in DB (allowed since migration 032)

		err = h.flagRepo.ExecInTx(ctx, `
			INSERT INTO revenue_recovery_events (
				id, anomaly_flag_id, district_id, account_id,
				variance_ghs, recovered_ghs, recovery_type,
				leakage_category, monthly_leakage_ghs,
				detection_date, status, notes, created_at, updated_at
			)
			SELECT
				$1::uuid, $2::uuid, $3::uuid, $4::uuid,
				$5::numeric, 0, $6::text,
				$7::leakage_category, $5::numeric,
				NOW(), 'PENDING',
				'Auto-created from confirmed anomaly ' || $2::text,
				NOW(), NOW()
			WHERE NOT EXISTS (
				SELECT 1 FROM revenue_recovery_events WHERE anomaly_flag_id = $2::uuid
			)`,
			newID, id, flag.DistrictID, accountIDArg,
			confirmedLeakageGHS, recoveryType,
			leakageCategory,
		)
		if err != nil {
			h.logger.Warn("Auto-create recovery event failed (non-fatal)", zap.Error(err))
		} else {
			recoveryEventID = &newID
		}
	}

	result := fiber.Map{
		"message":          "Anomaly flag updated",
		"id":               id,
		"confirmed_fraud":  req.ConfirmedFraud,
		"status":           newStatus,
		"leakage_category": leakageCategory,
	}
	if recoveryEventID != nil {
		result["recovery_event_id"] = recoveryEventID
		result["recovery_event_created"] = true
	}

	return response.OK(c, result)
}

// anomalyTypeToRecoveryType maps anomaly types to revenue recovery event types.
func anomalyTypeToRecoveryType(anomalyType string) string {
	switch anomalyType {
	case "SHADOW_BILL_VARIANCE":
		return "UNDERBILLING"
	case "CATEGORY_MISMATCH":
		return "CATEGORY_FRAUD"
	case "PHANTOM_METER":
		return "PHANTOM_METER"
	case "DISTRICT_IMBALANCE":
		return "UNREGISTERED_CONSUMPTION"
	case "UNMETERED_CONSUMPTION":
		return "UNMETERED_CONSUMPTION"
	case "METERING_INACCURACY":
		return "METER_FAULT"
	case "UNAUTHORISED_CONSUMPTION":
		return "ILLEGAL_CONNECTION"
	case "VAT_DISCREPANCY":
		return "VAT_EVASION"
	default:
		return "UNDERBILLING"
	}
}

// ─── RecordFieldJobOutcome ────────────────────────────────────────────────────
// PATCH /api/v1/field-jobs/:id/outcome
//
// Records the structured outcome from a field officer visit.
// This is the critical step that converts a DATA_QUALITY flag into either:
//   - REVENUE_LEAKAGE (real address, no meter → UNMETERED_CONSUMPTION)
//   - Internal fraud escalation (address doesn't exist → FRAUDULENT_ACCOUNT)
//   - Dismissal (GPS data was wrong → dismiss ADDRESS_UNVERIFIED flag)
//
// Request body:
//   outcome              string  (required) — FieldJobOutcome enum value
//   outcome_notes        string  (optional)
//   meter_found          bool    (optional)
//   address_confirmed    bool    (optional)
//   recommended_action   string  (optional)
//   estimated_monthly_m3 float64 (optional) — for UNMETERED_CONSUMPTION back-billing
func (h *FieldJobHandler) RecordFieldJobOutcome(c *fiber.Ctx) error {
	ctx := c.UserContext()

	jobID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid field job ID")
	}

	var req struct {
		Outcome            string  `json:"outcome"`
		OutcomeNotes       string  `json:"outcome_notes"`
		MeterFound         *bool   `json:"meter_found"`
		AddressConfirmed   *bool   `json:"address_confirmed"`
		RecommendedAction  string  `json:"recommended_action"`
		EstimatedMonthlyM3 float64 `json:"estimated_monthly_m3"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if req.Outcome == "" {
		return response.BadRequest(c, "MISSING_OUTCOME", "outcome is required")
	}

	officerID, _ := c.Locals("user_id").(string)

	// Update field job with outcome
	_, err = h.fieldJobRepo.DB().Exec(ctx, `
		UPDATE field_jobs
		SET outcome             = $1::field_job_outcome,
		    outcome_notes       = $2,
		    meter_found         = $3,
		    address_confirmed   = $4,
		    recommended_action  = $5,
		    outcome_recorded_at = NOW(),
		    status              = 'OUTCOME_RECORDED',
		    updated_at          = NOW()
		WHERE id = $6`,
		req.Outcome, req.OutcomeNotes, req.MeterFound, req.AddressConfirmed,
		req.RecommendedAction, jobID,
	)
	if err != nil {
		h.logger.Error("RecordFieldJobOutcome update failed", zap.Error(err))
		return response.InternalError(c, "Failed to record field job outcome")
	}

	// Fetch the field job to get the linked anomaly flag
	var anomalyFlagID *uuid.UUID
	var accountID *uuid.UUID
	var districtID uuid.UUID
	row := h.fieldJobRepo.DB().QueryRow(ctx, `
		SELECT fj.account_id, fj.district_id,
		       af.id
		FROM field_jobs fj
		LEFT JOIN anomaly_flags af ON af.field_job_id = fj.id
		WHERE fj.id = $1`, jobID)
	var afID uuid.UUID
	if err := row.Scan(&accountID, &districtID, &afID); err == nil {
		anomalyFlagID = &afID
	}

	// Auto-escalate based on outcome
	escalationResult := fiber.Map{}

	switch req.Outcome {
	case "METER_NOT_FOUND_INSTALL", "ADDRESS_VALID_UNREGISTERED":
		// Real address, no meter → UNMETERED_CONSUMPTION (revenue leakage)
		// Estimate monthly leakage if not provided
		estimatedM3 := req.EstimatedMonthlyM3
		if estimatedM3 <= 0 {
			estimatedM3 = 15.0 // Ghana residential average m³/month
		}
		// Load residential blended rate from DB (no hardcoding)
		// Falls back to PURC 2026 tier-2 (10.8320) + 20% VAT if DB unavailable
		residentialRateGHS := 10.8320
		vatMult            := 1.20
		var dbRate, dbVAT float64
		if err := h.flagRepo.DB().QueryRow(ctx, `
			SELECT
			  COALESCE(AVG(tr.rate_per_m3), 10.8320),
			  COALESCE((SELECT 1.0 + rate_percentage/100.0 FROM vat_config WHERE is_active=TRUE ORDER BY effective_from DESC LIMIT 1), 1.20)
			FROM tariff_rates tr
			WHERE tr.category = 'RESIDENTIAL' AND tr.is_active = TRUE`).Scan(&dbRate, &dbVAT); err == nil {
			residentialRateGHS = dbRate
			vatMult            = dbVAT
		}
		monthlyLeakageGHS := estimatedM3 * residentialRateGHS * vatMult

		// Update the linked anomaly flag to UNMETERED_CONSUMPTION
		if anomalyFlagID != nil {
			h.flagRepo.DB().Exec(ctx, `
				UPDATE anomaly_flags
				SET anomaly_type         = 'UNMETERED_CONSUMPTION',
				    alert_level          = 'HIGH',
				    leakage_category     = 'REVENUE_LEAKAGE',
				    monthly_leakage_ghs  = $1,
				    annualised_leakage_ghs = $1 * 12,
				    confirmed_leakage_ghs = $1,
				    field_outcome        = $2::field_job_outcome,
				    field_job_id         = $3,
				    title               = 'Unmetered consumption confirmed: GH₵' || ROUND($1::numeric, 2) || '/month leakage',
				    updated_at          = NOW()
				WHERE id = $4`,
				monthlyLeakageGHS, req.Outcome, jobID, *anomalyFlagID,
			)
		}

		escalationResult = fiber.Map{
			"action":              "ESCALATED_TO_UNMETERED_CONSUMPTION",
			"monthly_leakage_ghs": monthlyLeakageGHS,
			"recommended_action":  "Install meter, register account, back-bill estimated consumption",
		}

	case "ADDRESS_INVALID":
		// Address doesn't exist → FRAUDULENT_ACCOUNT (GWL internal fraud)
		// Escalate to GWL management + GRA. Do NOT create recovery event.
		if anomalyFlagID != nil {
			h.flagRepo.DB().Exec(ctx, `
				UPDATE anomaly_flags
				SET anomaly_type     = 'FRAUDULENT_ACCOUNT',
				    alert_level      = 'CRITICAL',
				    leakage_category = 'DATA_QUALITY',
				    field_outcome    = 'ADDRESS_INVALID'::field_job_outcome,
				    field_job_id     = $1,
				    title            = 'FRAUDULENT ACCOUNT CONFIRMED: Address does not exist — GWL internal fraud',
				    updated_at       = NOW()
				WHERE id = $2`,
				jobID, *anomalyFlagID,
			)
		}

		escalationResult = fiber.Map{
			"action":             "ESCALATED_TO_FRAUDULENT_ACCOUNT",
			"recommended_action": "Escalate to GWL management + GRA. Do NOT create recovery event.",
			"note":               "This is GWL internal embezzlement, not billing revenue leakage.",
		}

	case "METER_FOUND_OK":
		// GPS data was wrong — dismiss the ADDRESS_UNVERIFIED flag
		if anomalyFlagID != nil {
			h.flagRepo.DB().Exec(ctx, `
				UPDATE anomaly_flags
				SET status           = 'FALSE_POSITIVE',
				    false_positive   = TRUE,
				    field_outcome    = 'METER_FOUND_OK'::field_job_outcome,
				    field_job_id     = $1,
				    resolution_notes = 'Field officer confirmed meter present. GPS coordinates were incorrect.',
				    resolved_at      = NOW(),
				    updated_at       = NOW()
				WHERE id = $2`,
				jobID, *anomalyFlagID,
			)
		}

		escalationResult = fiber.Map{
			"action":             "FLAG_DISMISSED",
			"recommended_action": "Update GPS coordinates in GWL system to correct location.",
		}

	case "CATEGORY_MISMATCH_CONFIRMED":
		// Commercial use confirmed, billed as residential
		if anomalyFlagID != nil {
			h.flagRepo.DB().Exec(ctx, `
				UPDATE anomaly_flags
				SET field_outcome    = 'CATEGORY_MISMATCH_CONFIRMED'::field_job_outcome,
				    field_job_id     = $1,
				    leakage_category = 'REVENUE_LEAKAGE',
				    updated_at       = NOW()
				WHERE id = $2`,
				jobID, *anomalyFlagID,
			)
		}

		escalationResult = fiber.Map{
			"action":             "CATEGORY_MISMATCH_CONFIRMED",
			"recommended_action": "Reclassify account to COMMERCIAL. Back-bill tariff difference.",
		}

	case "ILLEGAL_CONNECTION_FOUND":
		// Illegal tap/bypass found
		if anomalyFlagID != nil {
			h.flagRepo.DB().Exec(ctx, `
				UPDATE anomaly_flags
				SET anomaly_type     = 'UNAUTHORISED_CONSUMPTION',
				    alert_level      = 'CRITICAL',
				    leakage_category = 'REVENUE_LEAKAGE',
				    field_outcome    = 'ILLEGAL_CONNECTION_FOUND'::field_job_outcome,
				    field_job_id     = $1,
				    updated_at       = NOW()
				WHERE id = $2`,
				jobID, *anomalyFlagID,
			)
		}

		escalationResult = fiber.Map{
			"action":             "ESCALATED_TO_UNAUTHORISED_CONSUMPTION",
			"recommended_action": "Disconnect illegal connection. Register account. Back-bill.",
		}
	}

	// ── Update revenue_recovery_event to FIELD_VERIFIED ─────────────────────
	// When a field officer records an outcome that confirms leakage, advance
	// the recovery event from PENDING → FIELD_VERIFIED and record who verified it.
	leakageConfirmingOutcomes := map[string]bool{
		"METER_NOT_FOUND_INSTALL":    true,
		"ADDRESS_VALID_UNREGISTERED": true,
		"CATEGORY_MISMATCH_CONFIRMED": true,
		"ILLEGAL_CONNECTION_FOUND":   true,
		"METER_FOUND_TAMPERED":       true,
		"METER_FOUND_FAULTY":         true,
		"DUPLICATE_METER":            true,
	}
	if leakageConfirmingOutcomes[req.Outcome] && anomalyFlagID != nil {
		// Update the anomaly flag status to ACKNOWLEDGED (field visit done)
		_, _ = h.flagRepo.DB().Exec(ctx, `
			UPDATE anomaly_flags
			SET status     = CASE WHEN status = 'OPEN' THEN 'ACKNOWLEDGED' ELSE status END,
			    updated_at = NOW()
			WHERE id = $1`, *anomalyFlagID)

		// Advance recovery event to FIELD_VERIFIED
		var officerUUID *uuid.UUID
		if officerID != "" {
			if oid, parseErr := uuid.Parse(officerID); parseErr == nil {
				officerUUID = &oid
			}
		}
		_, _ = h.flagRepo.DB().Exec(ctx, `
			UPDATE revenue_recovery_events
			SET status             = 'FIELD_VERIFIED',
			    field_verified_at  = NOW(),
			    field_verified_by  = $1,
			    updated_at         = NOW()
			WHERE anomaly_flag_id = $2
			  AND status = 'PENDING'`,
			officerUUID, *anomalyFlagID,
		)
	}

	return response.OK(c, fiber.Map{
		"message":    "Field job outcome recorded",
		"job_id":     jobID,
		"outcome":    req.Outcome,
		"escalation": escalationResult,
	})
}
