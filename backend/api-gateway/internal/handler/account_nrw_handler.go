package handler

import (
	"github.com/ArowuTest/gn-waas/shared/go/http/response"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/repository"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AccountHandler handles water account HTTP requests
type AccountHandler struct {
	accountRepo *repository.AccountRepository
	logger      *zap.Logger
}

func NewAccountHandler(accountRepo *repository.AccountRepository, logger *zap.Logger) *AccountHandler {
	return &AccountHandler{accountRepo: accountRepo, logger: logger}
}

// SearchAccounts godoc
// GET /api/v1/accounts/search?q=&district_id=&limit=&offset=
func (h *AccountHandler) SearchAccounts(c *fiber.Ctx) error {
	query := c.Query("q")
	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)

	if limit > 100 {
		limit = 100
	}

	var districtID *uuid.UUID
	if districtIDStr := c.Query("district_id"); districtIDStr != "" {
		id, err := uuid.Parse(districtIDStr)
		if err != nil {
			return response.BadRequest(c, "INVALID_DISTRICT_ID", "Invalid district ID")
		}
		districtID = &id
	}

	accounts, total, err := h.accountRepo.Search(c.Context(), query, districtID, limit, offset)
	if err != nil {
		h.logger.Error("Account search failed", zap.Error(err), zap.String("query", query))
		return response.InternalError(c, "Account search failed")
	}

	return response.OKWithMeta(c, accounts, &response.Meta{
		Total:    &total,
		Page:     intPtr(offset/limit + 1),
		PageSize: &limit,
	})
}

// GetAccount godoc
// GET /api/v1/accounts/:id
func (h *AccountHandler) GetAccount(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid account ID")
	}

	account, err := h.accountRepo.GetByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, "Account")
	}

	return response.OK(c, account)
}

// GetAccountsByDistrict godoc
// GET /api/v1/accounts?district_id=&limit=&offset=
func (h *AccountHandler) GetAccountsByDistrict(c *fiber.Ctx) error {
	districtIDStr := c.Query("district_id")
	if districtIDStr == "" {
		return response.BadRequest(c, "MISSING_DISTRICT_ID", "district_id is required")
	}

	districtID, err := uuid.Parse(districtIDStr)
	if err != nil {
		return response.BadRequest(c, "INVALID_DISTRICT_ID", "Invalid district ID")
	}

	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)

	accounts, total, err := h.accountRepo.GetByDistrict(c.Context(), districtID, limit, offset)
	if err != nil {
		return response.InternalError(c, "Failed to fetch accounts")
	}

	return response.OKWithMeta(c, accounts, &response.Meta{
		Total:    &total,
		Page:     intPtr(offset/limit + 1),
		PageSize: &limit,
	})
}

// NRWHandler handles Non-Revenue Water reporting HTTP requests
type NRWHandler struct {
	nrwRepo *repository.NRWReportRepository
	logger  *zap.Logger
}

func NewNRWHandler(nrwRepo *repository.NRWReportRepository, logger *zap.Logger) *NRWHandler {
	return &NRWHandler{nrwRepo: nrwRepo, logger: logger}
}

// GetNRWSummary godoc
// GET /api/v1/reports/nrw?district_id=&from=&to=
func (h *NRWHandler) GetNRWSummary(c *fiber.Ctx) error {
	var districtID *uuid.UUID
	if districtIDStr := c.Query("district_id"); districtIDStr != "" {
		id, err := uuid.Parse(districtIDStr)
		if err != nil {
			return response.BadRequest(c, "INVALID_DISTRICT_ID", "Invalid district ID")
		}
		districtID = &id
	}

	summaries, err := h.nrwRepo.GetNRWSummary(c.Context(), districtID, nil, nil)
	if err != nil {
		h.logger.Error("NRW summary failed", zap.Error(err))
		return response.InternalError(c, "Failed to fetch NRW summary")
	}

	return response.OK(c, summaries)
}

// GetDistrictNRWTrend godoc
// GET /api/v1/reports/nrw/:district_id/trend
func (h *NRWHandler) GetDistrictNRWTrend(c *fiber.Ctx) error {
	districtID, err := uuid.Parse(c.Params("district_id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_DISTRICT_ID", "Invalid district ID")
	}

	trend, err := h.nrwRepo.GetDistrictNRWTrend(c.Context(), districtID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch NRW trend")
	}

	return response.OK(c, trend)
}

// GetMyDistrictSummary godoc
// GET /api/v1/reports/nrw/my-district
// Used by GWL staff portal — returns summary for the authenticated user's district
func (h *NRWHandler) GetMyDistrictSummary(c *fiber.Ctx) error {
	// Get district from JWT claims (set by AuthMiddleware)
	// The user's district_id is stored in the users table and fetched via /users/me
	// For district-scoped roles, we look up the user's district from the DB
	userID, ok := c.Locals("user_id").(string)
	if !ok {
		return response.Unauthorized(c, "Not authenticated")
	}

	// We need to look up the user's district from the DB
	// This is done via a join — pass userID to the repo
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return response.Unauthorized(c, "Invalid user ID")
	}

	// Get district_id from users table
	var districtID uuid.UUID
	err = h.nrwRepo.DB().QueryRow(c.Context(),
		"SELECT district_id FROM users WHERE id = $1 AND district_id IS NOT NULL", userUUID,
	).Scan(&districtID)
	if err != nil {
		return response.BadRequest(c, "NO_DISTRICT", "User has no assigned district")
	}

	district, summary, err := h.nrwRepo.GetMyDistrictSummary(c.Context(), districtID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch district summary")
	}

	return response.OK(c, fiber.Map{
		"district": district,
		"summary":  summary,
	})
}
