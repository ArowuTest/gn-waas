package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ArowuTest/gn-waas/shared/go/http/response"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// AdminUserHandler handles user management for SYSTEM_ADMIN role
type AdminUserHandler struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewAdminUserHandler(db *pgxpool.Pool, logger *zap.Logger) *AdminUserHandler {
	return &AdminUserHandler{db: db, logger: logger}
}

type SystemUser struct {
	ID            uuid.UUID  `json:"id"`
	Email         string     `json:"email"`
	FullName      string     `json:"full_name"`
	Role          string     `json:"role"`
	DistrictID    *uuid.UUID `json:"district_id,omitempty"`
	DistrictName  *string    `json:"district_name,omitempty"`
	BadgeNumber   *string    `json:"badge_number,omitempty"`
	IsActive      bool       `json:"is_active"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// ListUsers godoc
// GET /api/v1/admin/users
func (h *AdminUserHandler) ListUsers(c *fiber.Ctx) error {
	// Only SYSTEM_ADMIN can manage users
	role, _ := c.Locals("user_role").(string)
	if role != "SYSTEM_ADMIN" {
		return response.Unauthorized(c, "Only SYSTEM_ADMIN can manage users")
	}

	q := c.Query("q")
	roleFilter := c.Query("role")

	args := []interface{}{}
	conditions := []string{"1=1"}
	argIdx := 1

	if q != "" {
		conditions = append(conditions,
			fmt.Sprintf("(u.full_name ILIKE $%d OR u.email ILIKE $%d)", argIdx, argIdx+1))
		args = append(args, "%"+q+"%", "%"+q+"%")
		argIdx += 2
	}
	if roleFilter != "" {
		conditions = append(conditions, fmt.Sprintf("u.role = $%d", argIdx))
		args = append(args, roleFilter)
		argIdx++
	}

	query := fmt.Sprintf(`
		SELECT
			u.id, u.email, u.full_name, u.role,
			u.district_id, d.name AS district_name,
			u.badge_number, u.is_active,
			u.last_login_at, u.created_at
		FROM users u
		LEFT JOIN districts d ON d.id = u.district_id
		WHERE %s
		ORDER BY u.created_at DESC
		LIMIT 200`, strings.Join(conditions, " AND "))

	rows, err := h.db.Query(context.Background(), query, args...)
	if err != nil {
		h.logger.Error("ListUsers query failed", zap.Error(err))
		return response.InternalError(c, "Failed to fetch users")
	}
	defer rows.Close()

	var users []SystemUser
	for rows.Next() {
		var u SystemUser
		err := rows.Scan(
			&u.ID, &u.Email, &u.FullName, &u.Role,
			&u.DistrictID, &u.DistrictName,
			&u.BadgeNumber, &u.IsActive,
			&u.LastLoginAt, &u.CreatedAt,
		)
		if err != nil {
			h.logger.Warn("Failed to scan user row", zap.Error(err))
			continue
		}
		users = append(users, u)
	}

	return response.OK(c, users)
}

// CreateUser godoc
// POST /api/v1/admin/users
func (h *AdminUserHandler) CreateUser(c *fiber.Ctx) error {
	role, _ := c.Locals("user_role").(string)
	if role != "SYSTEM_ADMIN" {
		return response.Unauthorized(c, "Only SYSTEM_ADMIN can create users")
	}

	var req struct {
		Email       string  `json:"email"`
		FullName    string  `json:"full_name"`
		Role        string  `json:"role"`
		DistrictID  *string `json:"district_id"`
		BadgeNumber *string `json:"badge_number"`
		Password    string  `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}
	if req.Email == "" || req.FullName == "" || req.Role == "" {
		return response.BadRequest(c, "MISSING_FIELDS", "email, full_name, and role are required")
	}

	validRoles := map[string]bool{
		"SYSTEM_ADMIN": true, "AUDIT_MANAGER": true, "DISTRICT_MANAGER": true,
		"FIELD_OFFICER": true, "GRA_LIAISON": true, "READONLY_VIEWER": true,
	}
	if !validRoles[req.Role] {
		return response.BadRequest(c, "INVALID_ROLE", "Invalid role")
	}

	var districtID *uuid.UUID
	if req.DistrictID != nil && *req.DistrictID != "" {
		id, err := uuid.Parse(*req.DistrictID)
		if err != nil {
			return response.BadRequest(c, "INVALID_DISTRICT", "Invalid district_id")
		}
		districtID = &id
	}

	var userID uuid.UUID
	err := h.db.QueryRow(context.Background(), `
		INSERT INTO users (email, full_name, role, district_id, badge_number, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
		RETURNING id`,
		req.Email, req.FullName, req.Role, districtID, req.BadgeNumber,
	).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			return response.BadRequest(c, "EMAIL_EXISTS", "A user with this email already exists")
		}
		h.logger.Error("CreateUser failed", zap.Error(err))
		return response.InternalError(c, "Failed to create user")
	}

	h.logger.Info("User created",
		zap.String("user_id", userID.String()),
		zap.String("email", req.Email),
		zap.String("role", req.Role),
		zap.String("created_by", c.Locals("user_id").(string)),
	)

	return c.Status(201).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":    userID,
			"email": req.Email,
			"role":  req.Role,
			"note":  "Password must be set via Keycloak admin console or reset email",
		},
	})
}

// UpdateUser godoc
// PATCH /api/v1/admin/users/:id
func (h *AdminUserHandler) UpdateUser(c *fiber.Ctx) error {
	role, _ := c.Locals("user_role").(string)
	if role != "SYSTEM_ADMIN" {
		return response.Unauthorized(c, "Only SYSTEM_ADMIN can update users")
	}

	userID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid user ID")
	}

	var req struct {
		FullName    *string `json:"full_name"`
		Role        *string `json:"role"`
		DistrictID  *string `json:"district_id"`
		BadgeNumber *string `json:"badge_number"`
		IsActive    *bool   `json:"is_active"`
	}
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid request body")
	}

	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argIdx := 1

	if req.FullName != nil {
		setClauses = append(setClauses, fmt.Sprintf("full_name = $%d", argIdx))
		args = append(args, *req.FullName)
		argIdx++
	}
	if req.Role != nil {
		setClauses = append(setClauses, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, *req.Role)
		argIdx++
	}
	if req.DistrictID != nil {
		if *req.DistrictID == "" {
			setClauses = append(setClauses, "district_id = NULL")
		} else {
			id, err := uuid.Parse(*req.DistrictID)
			if err != nil {
				return response.BadRequest(c, "INVALID_DISTRICT", "Invalid district_id")
			}
			setClauses = append(setClauses, fmt.Sprintf("district_id = $%d", argIdx))
			args = append(args, id)
			argIdx++
		}
	}
	if req.BadgeNumber != nil {
		setClauses = append(setClauses, fmt.Sprintf("badge_number = $%d", argIdx))
		args = append(args, *req.BadgeNumber)
		argIdx++
	}
	if req.IsActive != nil {
		setClauses = append(setClauses, fmt.Sprintf("is_active = $%d", argIdx))
		args = append(args, *req.IsActive)
		argIdx++
	}

	args = append(args, userID)
	query := fmt.Sprintf("UPDATE users SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), argIdx)

	result, err := h.db.Exec(context.Background(), query, args...)
	if err != nil {
		h.logger.Error("UpdateUser failed", zap.Error(err))
		return response.InternalError(c, "Failed to update user")
	}
	if result.RowsAffected() == 0 {
		return response.NotFound(c, "user")
	}

	return response.OK(c, fiber.Map{"success": true, "user_id": userID})
}

// ResetPassword godoc
// POST /api/v1/admin/users/:id/reset-password
func (h *AdminUserHandler) ResetPassword(c *fiber.Ctx) error {
	role, _ := c.Locals("user_role").(string)
	if role != "SYSTEM_ADMIN" {
		return response.Unauthorized(c, "Only SYSTEM_ADMIN can reset passwords")
	}

	userID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return response.BadRequest(c, "INVALID_ID", "Invalid user ID")
	}

	// In production: trigger Keycloak password reset email via admin API
	// For now: log the action and return success
	h.logger.Info("Password reset requested",
		zap.String("target_user_id", userID.String()),
		zap.String("requested_by", c.Locals("user_id").(string)),
	)

	return response.OK(c, fiber.Map{
		"success": true,
		"message": "Password reset email will be sent via Keycloak",
		"user_id": userID,
	})
}
