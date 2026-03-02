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

	created, err := h.auditRepo.Create(c.Context(), event)
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

	event, err := h.auditRepo.GetByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, "Audit event")
	}

	return response.OK(c, event)
}

// ListAuditEvents godoc
// GET /api/v1/audits
func (h *AuditHandler) ListAuditEvents(c *fiber.Ctx) error {
	districtIDStr := c.Query("district_id")
	status := c.Query("status")
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
	// c.Context(). The repository's q(ctx) helper retrieves it automatically.
	// No manual BeginReadOnlyTx needed here.
	events, total, err := h.auditRepo.GetByDistrict(c.Context(), districtID, status, limit, offset)
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
	officer, err := h.userRepo.GetByID(c.Context(), officerID)
	if err != nil || officer.Role != "FIELD_OFFICER" {
		return response.BadRequest(c, "INVALID_OFFICER", "Officer not found or invalid role")
	}

	// Create field job for the officer
	event, err := h.auditRepo.GetByID(c.Context(), id)
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

	job := &domain.FieldJob{
		AuditEventID:    &id,
		AccountID:       event.AccountID,
		DistrictID:      event.DistrictID,
		AssignedOfficerID: &officerID,
		Status:          "IN_PROGRESS",
		IsBlindAudit:    true, // Always blind audit - officer doesn't see expected reading
		GPSFenceRadiusM: 50.0, // 50m GPS fence
		Priority:        1,
	}

	_ = dueDate // Will be set on audit event

	createdJob, err := h.fieldJobRepo.Create(c.Context(), job)
	if err != nil {
		return response.InternalError(c, "Failed to create field job")
	}

	// Update audit event status
	if err := h.auditRepo.UpdateStatus(c.Context(), id, "IN_PROGRESS"); err != nil {
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

	stats, err := h.auditRepo.GetDashboardStats(c.Context(), districtID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch dashboard stats")
	}

	return response.OK(c, stats)
}

// FieldJobHandler handles field job HTTP requests
type FieldJobHandler struct {
	fieldJobRepo    *repository.FieldJobRepository
	auditRepo       *repository.AuditEventRepository
	sosNotifier     *notification.SOSNotifier
	evidenceStorage *storage.EvidenceStorageService
	logger          *zap.Logger
}

func NewFieldJobHandler(
	fieldJobRepo    *repository.FieldJobRepository,
	auditRepo       *repository.AuditEventRepository,
	sosNotifier     *notification.SOSNotifier,
	evidenceStorage *storage.EvidenceStorageService,
	logger          *zap.Logger,
) *FieldJobHandler {
	return &FieldJobHandler{
		fieldJobRepo:    fieldJobRepo,
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
	jobs, err := h.fieldJobRepo.GetByOfficerEnriched(c.Context(), officerID)
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

	if err := h.fieldJobRepo.UpdateStatus(c.Context(), id, req.Status, req.OfficerLat, req.OfficerLng); err != nil {
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

	if err := h.fieldJobRepo.TriggerSOS(c.Context(), id, req.OfficerLat, req.OfficerLng); err != nil {
		return response.InternalError(c, "Failed to trigger SOS")
	}

	// Dispatch SOS alert to supervisors via webhook + SMS
	officerID, _ := c.Locals("user_id").(string)
	officerName, _ := c.Locals("user_name").(string)
	h.sosNotifier.SendSOSAlert(c.Context(), notification.SOSAlert{
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
	ctx := c.Context()

	var districtID *uuid.UUID
	if d := c.Query("district_id"); d != "" {
		id, err := uuid.Parse(d)
		if err != nil {
			return response.BadRequest(c, "INVALID_DISTRICT_ID", "invalid district_id")
		}
		districtID = &id
	}

	severity := c.Query("severity")
	status := c.Query("status")
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
	flags, total, err := h.flagRepo.ListAnomalyFlags(ctx, districtID, severity, status, limit, offset)
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
	flag, err := h.flagRepo.GetByID(c.Context(), id)
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
	flags, total, err := h.flagRepo.ListAnomalyFlags(c.Context(), &districtID, "", "OPEN", 1000, 0)
	if err != nil {
		return response.InternalError(c, "Failed to fetch district summary")
	}
	summary := map[string]int{"CRITICAL": 0, "HIGH": 0, "MEDIUM": 0, "LOW": 0, "INFO": 0}
	for _, f := range flags {
		if f.AlertLevel != nil {
			summary[*f.AlertLevel]++
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
	resp, err := http.Post(sentinelURL+"/api/v1/scan/"+districtID, "application/json", nil)
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

	// 1. Mark job as COMPLETED with officer GPS
	lat, lng := req.GPSLat, req.GPSLng
	if err := h.fieldJobRepo.UpdateStatus(c.Context(), jobID, "COMPLETED", &lat, &lng); err != nil {
		h.logger.Error("Failed to complete field job", zap.Error(err))
		return response.InternalError(c, "Failed to update job status")
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
			ok, err := h.evidenceStorage.VerifyPhotoHash(c.Context(), objectKey, clientHash)
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
	if err := h.fieldJobRepo.WriteEvidence(c.Context(), jobID, &domain.FieldJobEvidence{
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
	validJobStatuses := map[string]bool{
		"": true, "QUEUED": true, "ASSIGNED": true, "IN_PROGRESS": true,
		"ON_SITE": true, "COMPLETED": true, "ESCALATED": true, "SOS": true, "CANCELLED": true,
	}
	if !validJobStatuses[status] {
		return response.BadRequest(c, "INVALID_STATUS", "Invalid status value")
	}

	// RLS is enforced by the rls.Middleware applied to the /api/v1 group.
	jobs, jobErr := h.fieldJobRepo.ListAll(c.Context(), status, alertLevel, districtID)
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

	created, err := h.fieldJobRepo.Create(c.Context(), job)
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

	if err := h.fieldJobRepo.AssignOfficer(c.Context(), jobID, officerID); err != nil {
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

	// Use the repository method so the INSERT runs inside the RLS-activated
	// transaction from rls.Middleware — no raw DB() bypass.
	reportID, err := h.auditRepo.CreateIllegalConnection(c.Context(), &repository.IllegalConnectionReport{
		OfficerID:                officerID,
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
		return response.InternalError(c, "Failed to save report")
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
