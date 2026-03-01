package handler

import (
	"time"

	"github.com/ArowuTest/gn-waas/pkg/shared/http/response"
	"github.com/ArowuTest/gn-waas/services/api-gateway/internal/domain"
	"github.com/ArowuTest/gn-waas/services/api-gateway/internal/repository"
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

	if districtIDStr == "" {
		return response.BadRequest(c, "MISSING_DISTRICT_ID", "district_id query parameter is required")
	}

	districtID, err := uuid.Parse(districtIDStr)
	if err != nil {
		return response.BadRequest(c, "INVALID_DISTRICT_ID", "Invalid district ID")
	}

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
		AssignedOfficerID: officerID,
		Status:          "ASSIGNED",
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
	if err := h.auditRepo.UpdateStatus(c.Context(), id, "ASSIGNED"); err != nil {
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
	fieldJobRepo *repository.FieldJobRepository
	auditRepo    *repository.AuditEventRepository
	logger       *zap.Logger
}

func NewFieldJobHandler(
	fieldJobRepo *repository.FieldJobRepository,
	auditRepo *repository.AuditEventRepository,
	logger *zap.Logger,
) *FieldJobHandler {
	return &FieldJobHandler{
		fieldJobRepo: fieldJobRepo,
		auditRepo:    auditRepo,
		logger:       logger,
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

	status := c.Query("status")
	jobs, err := h.fieldJobRepo.GetByOfficer(c.Context(), officerID, status)
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

	h.logger.Warn("SOS TRIGGERED",
		zap.String("job_id", id.String()),
		zap.Float64("lat", req.OfficerLat),
		zap.Float64("lng", req.OfficerLng),
	)

	return response.OK(c, fiber.Map{
		"message":    "SOS triggered - supervisor notified",
		"job_id":     id,
		"officer_lat": req.OfficerLat,
		"officer_lng": req.OfficerLng,
	})
}

func intPtr(i int) *int { return &i }
