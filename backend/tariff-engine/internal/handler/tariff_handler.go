package handler

import (
	"time"

	"github.com/ArowuTest/gn-waas/shared/go/http/response"
	"github.com/ArowuTest/gn-waas/backend/tariff-engine/internal/domain/entities"
	"github.com/ArowuTest/gn-waas/backend/tariff-engine/internal/repository/interfaces"
	"github.com/ArowuTest/gn-waas/backend/tariff-engine/internal/service"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TariffHandler handles HTTP requests for the tariff engine
type TariffHandler struct {
	tariffSvc  *service.TariffService
	tariffRepo interfaces.TariffRateRepository
	vatRepo    interfaces.VATConfigRepository
	shadowRepo interfaces.ShadowBillRepository
	logger     *zap.Logger
}

func NewTariffHandler(
	tariffSvc *service.TariffService,
	tariffRepo interfaces.TariffRateRepository,
	vatRepo interfaces.VATConfigRepository,
	shadowRepo interfaces.ShadowBillRepository,
	logger *zap.Logger,
) *TariffHandler {
	return &TariffHandler{
		tariffSvc:  tariffSvc,
		tariffRepo: tariffRepo,
		vatRepo:    vatRepo,
		shadowRepo: shadowRepo,
		logger:     logger,
	}
}

// CalculateBill godoc
// POST /api/v1/tariff/calculate
// Calculates the correct shadow bill for a given consumption
func (h *TariffHandler) CalculateBill(c *fiber.Ctx) error {
	var req entities.TariffCalculationRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body: "+err.Error())
	}

	if req.ConsumptionM3 < 0 {
		return response.BadRequest(c, "INVALID_CONSUMPTION", "Consumption cannot be negative")
	}
	if req.Category == "" {
		return response.BadRequest(c, "MISSING_CATEGORY", "Account category is required")
	}
	if req.BillingDate.IsZero() {
		req.BillingDate = time.Now()
	}

	calc, err := h.tariffSvc.CalculateShadowBill(c.Context(), &req)
	if err != nil {
		h.logger.Error("Shadow bill calculation failed", zap.Error(err))
		return response.InternalError(c, "Calculation failed: "+err.Error())
	}

	return response.OK(c, calc)
}

// CalculateBatch godoc
// POST /api/v1/tariff/calculate/batch
// Calculates shadow bills for multiple accounts
func (h *TariffHandler) CalculateBatch(c *fiber.Ctx) error {
	var requests []*entities.TariffCalculationRequest
	if err := c.BodyParser(&requests); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	if len(requests) == 0 {
		return response.BadRequest(c, "EMPTY_BATCH", "Batch must contain at least one request")
	}
	if len(requests) > 1000 {
		return response.BadRequest(c, "BATCH_TOO_LARGE", "Batch cannot exceed 1000 records")
	}

	results, err := h.tariffSvc.CalculateBatch(c.Context(), requests)
	if err != nil {
		return response.InternalError(c, "Batch calculation failed")
	}

	return response.OK(c, fiber.Map{
		"total":     len(requests),
		"processed": len(results),
		"results":   results,
	})
}

// GetTariffRates godoc
// GET /api/v1/tariff/rates
// Returns all active tariff rates
func (h *TariffHandler) GetTariffRates(c *fiber.Ctx) error {
	rates, err := h.tariffRepo.GetAll(c.Context())
	if err != nil {
		return response.InternalError(c, "Failed to fetch tariff rates")
	}
	return response.OK(c, rates)
}

// GetTariffRatesByCategory godoc
// GET /api/v1/tariff/rates/:category
// Returns active tariff rates for a specific category
func (h *TariffHandler) GetTariffRatesByCategory(c *fiber.Ctx) error {
	category := c.Params("category")
	if category == "" {
		return response.BadRequest(c, "MISSING_CATEGORY", "Category is required")
	}

	rates, err := h.tariffRepo.GetActiveRatesForCategory(c.Context(), category, time.Now())
	if err != nil {
		return response.InternalError(c, "Failed to fetch tariff rates")
	}

	return response.OK(c, rates)
}

// GetVATConfig godoc
// GET /api/v1/tariff/vat
// Returns the current active VAT configuration
func (h *TariffHandler) GetVATConfig(c *fiber.Ctx) error {
	cfg, err := h.vatRepo.GetActiveConfig(c.Context(), time.Now())
	if err != nil {
		return response.InternalError(c, "Failed to fetch VAT configuration")
	}
	return response.OK(c, cfg)
}

// GetVarianceSummary godoc
// GET /api/v1/tariff/variance/:district_id
// Returns variance summary for a district
func (h *TariffHandler) GetVarianceSummary(c *fiber.Ctx) error {
	districtIDStr := c.Params("district_id")
	districtID, err := uuid.Parse(districtIDStr)
	if err != nil {
		return response.BadRequest(c, "INVALID_DISTRICT_ID", "Invalid district ID format")
	}

	fromStr := c.Query("from", time.Now().AddDate(0, -1, 0).Format("2006-01-02"))
	toStr := c.Query("to", time.Now().Format("2006-01-02"))

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		return response.BadRequest(c, "INVALID_FROM_DATE", "Invalid from date format (YYYY-MM-DD)")
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		return response.BadRequest(c, "INVALID_TO_DATE", "Invalid to date format (YYYY-MM-DD)")
	}

	summary, err := h.shadowRepo.GetVarianceSummary(c.Context(), districtID, from, to)
	if err != nil {
		return response.InternalError(c, "Failed to fetch variance summary")
	}

	return response.OK(c, summary)
}

// HealthCheck godoc
// GET /health
func (h *TariffHandler) HealthCheck(c *fiber.Ctx) error {
	return response.OK(c, fiber.Map{
		"service": "tariff-engine",
		"status":  "healthy",
		"version": "1.0.0",
	})
}
