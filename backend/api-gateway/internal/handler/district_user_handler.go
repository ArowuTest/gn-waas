package handler

import (
	"context"
	"time"

	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/domain"
	"github.com/ArowuTest/gn-waas/backend/api-gateway/internal/repository"
	"github.com/ArowuTest/gn-waas/shared/go/http/response"
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

// logAdminAction writes an entry to the audit_trail table for compliance.
// Non-fatal: if the write fails, the action is still completed but the
// failure is logged for investigation.
func (h *DistrictHandler) logAdminAction(ctx context.Context, actorID, entityType, entityID, action string, oldVal, newVal interface{}) {
	// Use the repository method so the INSERT runs inside the RLS-activated
	// transaction from rls.Middleware — no raw DB() bypass.
	if err := h.districtRepo.LogAdminAction(ctx, entityType, entityID, action, actorID, oldVal, newVal); err != nil {
		h.logger.Warn("Failed to write audit trail",
			zap.String("entity_type", entityType),
			zap.String("entity_id", entityID),
			zap.String("action", action),
			zap.Error(err),
		)
	}
}

// ListDistricts godoc
// GET /api/v1/districts
func (h *DistrictHandler) ListDistricts(c *fiber.Ctx) error {
	districts, err := h.districtRepo.GetAll(c.UserContext())
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

	district, err := h.districtRepo.GetByID(c.UserContext(), id)
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

	user, err := h.userRepo.GetByID(c.UserContext(), userID)
	if err != nil {
		return response.NotFound(c, "User")
	}

	// Update last login
	_ = h.userRepo.UpdateLastLogin(c.UserContext(), userID)

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

	officers, err := h.userRepo.GetFieldOfficers(c.UserContext(), districtID)
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
	configs, err := h.configRepo.GetByCategory(c.UserContext(), category)
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

	if err := h.configRepo.Update(c.UserContext(), key, req.Value, userID); err != nil {
		return response.InternalError(c, "Failed to update config")
	}

	return response.OK(c, fiber.Map{"message": "Config updated", "key": key, "value": req.Value})
}

// HealthHandler handles health check and readiness requests
type HealthHandler struct {
	db interface{ Ping(context.Context) error }
}

func NewHealthHandler(db interface{ Ping(context.Context) error }) *HealthHandler {
	return &HealthHandler{db: db}
}

// HealthCheck is the liveness probe — returns 200 if the process is alive.
// K8s liveness probe: if this fails, the pod is restarted.
// GET /health
func (h *HealthHandler) HealthCheck(c *fiber.Ctx) error {
	return response.OK(c, fiber.Map{
		"service": "api-gateway",
		"status":  "alive",
		"version": "1.0.0",
	})
}

// ReadinessCheck is the readiness probe — returns 200 only if the service
// can handle traffic (DB is reachable). K8s readiness probe: if this fails,
// the pod is removed from the load balancer until it recovers.
// GET /ready
func (h *HealthHandler) ReadinessCheck(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), 3*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"service": "api-gateway",
			"status":  "not_ready",
			"reason":  "database_unreachable",
		})
	}

	return response.OK(c, fiber.Map{
		"service": "api-gateway",
		"status":  "ready",
		"version": "1.0.0",
	})
}

// CreateDistrict godoc
// POST /api/v1/admin/districts
func (h *DistrictHandler) CreateDistrict(c *fiber.Ctx) error {
	role, _ := c.Locals("rls_user_role").(string)
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
		GeographicZone     string  `json:"geographic_zone"`
		IsPilotDistrict    bool    `json:"is_pilot_district"`
		IsActive           bool    `json:"is_active"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if req.DistrictCode == "" || req.DistrictName == "" {
		return response.BadRequest(c, "MISSING_FIELDS", "district_code and district_name are required")
	}

	// Use the repository method so the INSERT runs inside the RLS-activated
	// transaction from rls.Middleware — no raw DB() bypass.
	newDistrict := &domain.District{
		DistrictCode:       req.DistrictCode,
		DistrictName:       req.DistrictName,
		Region:             req.Region,
		PopulationEstimate: req.PopulationEstimate,
		TotalConnections:   req.TotalConnections,
		SupplyStatus:       req.SupplyStatus,
		ZoneType:           req.ZoneType,
		IsPilotDistrict:    req.IsPilotDistrict,
		IsActive:           req.IsActive,
	}
	id, err := h.districtRepo.Create(c.UserContext(), newDistrict)
	if err != nil {
		h.logger.Error("CreateDistrict failed", zap.Error(err))
		return response.InternalError(c, "Failed to create district")
	}

	return c.Status(201).JSON(fiber.Map{"success": true, "id": id})
}

// UpdateDistrict godoc
// PATCH /api/v1/admin/districts/:id
func (h *DistrictHandler) UpdateDistrict(c *fiber.Ctx) error {
	role, _ := c.Locals("rls_user_role").(string)
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
		GeographicZone     *string `json:"geographic_zone"`
		IsPilotDistrict    *bool   `json:"is_pilot_district"`
		IsActive           *bool   `json:"is_active"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	// Build the fields map for the repository method.
	// Using repo.UpdateFields() ensures the UPDATE runs inside the RLS-activated
	// transaction from rls.Middleware — no raw DB() bypass.
	fields := map[string]interface{}{}
	if req.DistrictName != nil    { fields["district_name"]       = *req.DistrictName }
	if req.Region != nil          { fields["region"]              = *req.Region }
	if req.PopulationEstimate != nil { fields["population_estimate"] = *req.PopulationEstimate }
	if req.TotalConnections != nil { fields["total_connections"]   = *req.TotalConnections }
	if req.SupplyStatus != nil    { fields["supply_status"]        = *req.SupplyStatus }
	if req.ZoneType != nil        { fields["zone_type"]            = *req.ZoneType }
	if req.GeographicZone != nil  { fields["geographic_zone"]      = *req.GeographicZone }
	if req.IsPilotDistrict != nil { fields["is_pilot_district"]    = *req.IsPilotDistrict }
	if req.IsActive != nil        { fields["is_active"]            = *req.IsActive }

	rowsAffected, err := h.districtRepo.UpdateFields(c.UserContext(), districtID, fields)
	if err != nil {
		h.logger.Error("UpdateDistrict failed", zap.Error(err))
		return response.InternalError(c, "Failed to update district")
	}
	if rowsAffected == 0 {
		return response.NotFound(c, "district")
	}

	return response.OK(c, fiber.Map{"success": true, "id": districtID})
}
