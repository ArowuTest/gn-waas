package handler

import (
	"time"

	"github.com/ArowuTest/gn-waas/pkg/shared/http/response"
	"github.com/ArowuTest/gn-waas/services/sentinel/internal/repository/interfaces"
	"github.com/ArowuTest/gn-waas/services/sentinel/internal/service/orchestrator"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SentinelHandler handles HTTP requests for the sentinel service
type SentinelHandler struct {
	orchestrator *orchestrator.SentinelOrchestrator
	anomalyRepo  interfaces.AnomalyFlagRepository
	districtRepo interfaces.DistrictRepository
	logger       *zap.Logger
}

func NewSentinelHandler(
	orch *orchestrator.SentinelOrchestrator,
	anomalyRepo interfaces.AnomalyFlagRepository,
	districtRepo interfaces.DistrictRepository,
	logger *zap.Logger,
) *SentinelHandler {
	return &SentinelHandler{
		orchestrator: orch,
		anomalyRepo:  anomalyRepo,
		districtRepo: districtRepo,
		logger:       logger,
	}
}

// TriggerScan godoc
// POST /api/v1/sentinel/scan/:district_id
// Triggers a full sentinel scan for a district
func (h *SentinelHandler) TriggerScan(c *fiber.Ctx) error {
	districtID, err := uuid.Parse(c.Params("district_id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_DISTRICT_ID", "Invalid district ID")
	}

	result, err := h.orchestrator.RunDistrictScan(c.Context(), districtID)
	if err != nil {
		h.logger.Error("Sentinel scan failed", zap.Error(err))
		return response.InternalError(c, "Scan failed: "+err.Error())
	}

	return response.OK(c, result)
}

// GetAnomalies godoc
// GET /api/v1/sentinel/anomalies
// Returns anomaly flags with filtering
func (h *SentinelHandler) GetAnomalies(c *fiber.Ctx) error {
	filter := interfaces.AnomalyFilter{
		Limit:  c.QueryInt("limit", 20),
		Offset: c.QueryInt("offset", 0),
	}

	if districtIDStr := c.Query("district_id"); districtIDStr != "" {
		id, err := uuid.Parse(districtIDStr)
		if err != nil {
			return response.BadRequest(c, "INVALID_DISTRICT_ID", "Invalid district ID")
		}
		filter.DistrictID = &id
	}

	if anomalyType := c.Query("type"); anomalyType != "" {
		filter.AnomalyType = &anomalyType
	}
	if alertLevel := c.Query("level"); alertLevel != "" {
		filter.AlertLevel = &alertLevel
	}
	if status := c.Query("status"); status != "" {
		filter.Status = &status
	}

	if fromStr := c.Query("from"); fromStr != "" {
		from, err := time.Parse("2006-01-02", fromStr)
		if err == nil {
			filter.From = &from
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		to, err := time.Parse("2006-01-02", toStr)
		if err == nil {
			filter.To = &to
		}
	}

	flags, total, err := h.anomalyRepo.GetByCriteria(c.Context(), filter)
	if err != nil {
		return response.InternalError(c, "Failed to fetch anomalies")
	}

	return response.OKWithMeta(c, flags, &response.Meta{
		Total:    &total,
		Page:     intPtr(filter.Offset/filter.Limit + 1),
		PageSize: &filter.Limit,
	})
}

// GetAnomaly godoc
// GET /api/v1/sentinel/anomalies/:id
func (h *SentinelHandler) GetAnomaly(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid anomaly ID")
	}

	flag, err := h.anomalyRepo.GetByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, "Anomaly flag")
	}

	return response.OK(c, flag)
}

// GetDistrictSummary godoc
// GET /api/v1/sentinel/summary/:district_id
func (h *SentinelHandler) GetDistrictSummary(c *fiber.Ctx) error {
	districtID, err := uuid.Parse(c.Params("district_id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_DISTRICT_ID", "Invalid district ID")
	}

	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()

	if fromStr := c.Query("from"); fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t
		}
	}

	summary, err := h.anomalyRepo.GetSummaryByDistrict(c.Context(), districtID, from, to)
	if err != nil {
		return response.InternalError(c, "Failed to fetch summary")
	}

	return response.OK(c, summary)
}

// ResolveAnomaly godoc
// PATCH /api/v1/sentinel/anomalies/:id/resolve
func (h *SentinelHandler) ResolveAnomaly(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid anomaly ID")
	}

	var body struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	userID, _ := uuid.Parse(c.Locals("user_id").(string))

	if err := h.anomalyRepo.UpdateStatus(c.Context(), id, body.Status, userID, body.Notes); err != nil {
		return response.InternalError(c, "Failed to update anomaly status")
	}

	return response.OK(c, fiber.Map{"message": "Anomaly status updated"})
}

// MarkFalsePositive godoc
// PATCH /api/v1/sentinel/anomalies/:id/false-positive
func (h *SentinelHandler) MarkFalsePositive(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid anomaly ID")
	}

	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.BodyParser(&body); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	userID, _ := uuid.Parse(c.Locals("user_id").(string))

	if err := h.anomalyRepo.MarkFalsePositive(c.Context(), id, userID, body.Reason); err != nil {
		return response.InternalError(c, "Failed to mark as false positive")
	}

	return response.OK(c, fiber.Map{"message": "Anomaly marked as false positive"})
}

// HealthCheck godoc
// GET /health
func (h *SentinelHandler) HealthCheck(c *fiber.Ctx) error {
	return response.OK(c, fiber.Map{
		"service": "sentinel",
		"status":  "healthy",
		"version": "1.0.0",
	})
}

func intPtr(i int) *int { return &i }
