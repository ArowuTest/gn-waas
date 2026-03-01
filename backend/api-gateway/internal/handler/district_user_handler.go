package handler

import (
	"fmt"
	"strings"

	"github.com/ArowuTest/gn-waas/shared/go/http/response"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/repository"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// DistrictHandler handles district HTTP requests
type DistrictHandler struct {
	districtRepo *repository.DistrictRepository
	logger       *zap.Logger
}

func NewDistrictHandler(districtRepo *repository.DistrictRepository, logger *zap.Logger) *DistrictHandler {
	return &DistrictHandler{districtRepo: districtRepo, logger: logger}
}

// ListDistricts godoc
// GET /api/v1/districts
func (h *DistrictHandler) ListDistricts(c *fiber.Ctx) error {
	districts, err := h.districtRepo.GetAll(c.Context())
	if err != nil {
		return response.InternalError(c, "Failed to fetch districts")
	}
	return response.OK(c, districts)
}

// GetDistrict godoc
// GET /api/v1/districts/:id
func (h *DistrictHandler) GetDistrict(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid district ID")
	}

	district, err := h.districtRepo.GetByID(c.Context(), id)
	if err != nil {
		return response.NotFound(c, "District")
	}

	return response.OK(c, district)
}

// UserHandler handles user management HTTP requests
type UserHandler struct {
	userRepo *repository.UserRepository
	logger   *zap.Logger
}

func NewUserHandler(userRepo *repository.UserRepository, logger *zap.Logger) *UserHandler {
	return &UserHandler{userRepo: userRepo, logger: logger}
}

// GetMe godoc
// GET /api/v1/users/me
func (h *UserHandler) GetMe(c *fiber.Ctx) error {
	userIDStr, ok := c.Locals("user_id").(string)
	if !ok {
		return response.Unauthorized(c, "Not authenticated")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return response.Unauthorized(c, "Invalid user ID")
	}

	user, err := h.userRepo.GetByID(c.Context(), userID)
	if err != nil {
		return response.NotFound(c, "User")
	}

	// Update last login
	_ = h.userRepo.UpdateLastLogin(c.Context(), userID)

	return response.OK(c, user)
}

// GetFieldOfficers godoc
// GET /api/v1/users/field-officers
func (h *UserHandler) GetFieldOfficers(c *fiber.Ctx) error {
	var districtID *uuid.UUID
	if districtIDStr := c.Query("district_id"); districtIDStr != "" {
		id, err := uuid.Parse(districtIDStr)
		if err == nil {
			districtID = &id
		}
	}

	officers, err := h.userRepo.GetFieldOfficers(c.Context(), districtID)
	if err != nil {
		return response.InternalError(c, "Failed to fetch field officers")
	}

	return response.OK(c, officers)
}

// SystemConfigHandler handles system configuration HTTP requests
type SystemConfigHandler struct {
	configRepo *repository.SystemConfigRepository
	logger     *zap.Logger
}

func NewSystemConfigHandler(configRepo *repository.SystemConfigRepository, logger *zap.Logger) *SystemConfigHandler {
	return &SystemConfigHandler{configRepo: configRepo, logger: logger}
}

// GetConfigByCategory godoc
// GET /api/v1/config/:category
func (h *SystemConfigHandler) GetConfigByCategory(c *fiber.Ctx) error {
	category := c.Params("category")
	configs, err := h.configRepo.GetByCategory(c.Context(), category)
	if err != nil {
		return response.InternalError(c, "Failed to fetch config")
	}
	return response.OK(c, configs)
}

// UpdateConfig godoc
// PATCH /api/v1/config/:key
func (h *SystemConfigHandler) UpdateConfig(c *fiber.Ctx) error {
	key := c.Params("key")

	var req struct {
		Value string `json:"value"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	userIDStr, _ := c.Locals("user_id").(string)
	userID, _ := uuid.Parse(userIDStr)

	if err := h.configRepo.Update(c.Context(), key, req.Value, userID); err != nil {
		return response.InternalError(c, "Failed to update config")
	}

	return response.OK(c, fiber.Map{"message": "Config updated", "key": key, "value": req.Value})
}

// HealthHandler handles health check requests
type HealthHandler struct{}

func NewHealthHandler() *HealthHandler { return &HealthHandler{} }

// HealthCheck godoc
// GET /health
func (h *HealthHandler) HealthCheck(c *fiber.Ctx) error {
	return response.OK(c, fiber.Map{
		"service": "api-gateway",
		"status":  "healthy",
		"version": "1.0.0",
	})
}

// CreateDistrict godoc
// POST /api/v1/admin/districts
func (h *DistrictHandler) CreateDistrict(c *fiber.Ctx) error {
	role, _ := c.Locals("user_role").(string)
	if role != "SYSTEM_ADMIN" {
		return response.Unauthorized(c, "Only SYSTEM_ADMIN can create districts")
	}

	var req struct {
		DistrictCode       string  `json:"district_code"`
		DistrictName       string  `json:"district_name"`
		Region             string  `json:"region"`
		PopulationEstimate int     `json:"population_estimate"`
		TotalConnections   int     `json:"total_connections"`
		SupplyStatus       string  `json:"supply_status"`
		ZoneType           string  `json:"zone_type"`
		IsPilotDistrict    bool    `json:"is_pilot_district"`
		IsActive           bool    `json:"is_active"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if req.DistrictCode == "" || req.DistrictName == "" {
		return response.BadRequest(c, "MISSING_FIELDS", "district_code and district_name are required")
	}

	var id uuid.UUID
	err := h.districtRepo.DB().QueryRow(c.Context(), `
		INSERT INTO districts
			(district_code, district_name, region, population_estimate,
			 total_connections, supply_status, zone_type, is_pilot_district, is_active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id`,
		req.DistrictCode, req.DistrictName, req.Region,
		req.PopulationEstimate, req.TotalConnections,
		req.SupplyStatus, req.ZoneType,
		req.IsPilotDistrict, req.IsActive,
	).Scan(&id)
	if err != nil {
		h.logger.Error("CreateDistrict failed", zap.Error(err))
		return response.InternalError(c, "Failed to create district")
	}

	return c.Status(201).JSON(fiber.Map{"success": true, "id": id})
}

// UpdateDistrict godoc
// PATCH /api/v1/admin/districts/:id
func (h *DistrictHandler) UpdateDistrict(c *fiber.Ctx) error {
	role, _ := c.Locals("user_role").(string)
	if role != "SYSTEM_ADMIN" {
		return response.Unauthorized(c, "Only SYSTEM_ADMIN can update districts")
	}

	districtID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid district ID")
	}

	var req struct {
		DistrictName       *string `json:"district_name"`
		Region             *string `json:"region"`
		PopulationEstimate *int    `json:"population_estimate"`
		TotalConnections   *int    `json:"total_connections"`
		SupplyStatus       *string `json:"supply_status"`
		ZoneType           *string `json:"zone_type"`
		IsPilotDistrict    *bool   `json:"is_pilot_district"`
		IsActive           *bool   `json:"is_active"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	idx := 1

	if req.DistrictName != nil    { setClauses = append(setClauses, fmt.Sprintf("district_name=$%d", idx)); args = append(args, *req.DistrictName); idx++ }
	if req.Region != nil          { setClauses = append(setClauses, fmt.Sprintf("region=$%d", idx)); args = append(args, *req.Region); idx++ }
	if req.PopulationEstimate != nil { setClauses = append(setClauses, fmt.Sprintf("population_estimate=$%d", idx)); args = append(args, *req.PopulationEstimate); idx++ }
	if req.TotalConnections != nil { setClauses = append(setClauses, fmt.Sprintf("total_connections=$%d", idx)); args = append(args, *req.TotalConnections); idx++ }
	if req.SupplyStatus != nil    { setClauses = append(setClauses, fmt.Sprintf("supply_status=$%d", idx)); args = append(args, *req.SupplyStatus); idx++ }
	if req.ZoneType != nil        { setClauses = append(setClauses, fmt.Sprintf("zone_type=$%d", idx)); args = append(args, *req.ZoneType); idx++ }
	if req.IsPilotDistrict != nil { setClauses = append(setClauses, fmt.Sprintf("is_pilot_district=$%d", idx)); args = append(args, *req.IsPilotDistrict); idx++ }
	if req.IsActive != nil        { setClauses = append(setClauses, fmt.Sprintf("is_active=$%d", idx)); args = append(args, *req.IsActive); idx++ }

	args = append(args, districtID)
	query := fmt.Sprintf("UPDATE districts SET %s WHERE id=$%d",
		strings.Join(setClauses, ", "), idx)

	result, err := h.districtRepo.DB().Exec(c.Context(), query, args...)
	if err != nil {
		h.logger.Error("UpdateDistrict failed", zap.Error(err))
		return response.InternalError(c, "Failed to update district")
	}
	if result.RowsAffected() == 0 {
		return response.NotFound(c, "district")
	}

	return response.OK(c, fiber.Map{"success": true, "id": districtID})
}
