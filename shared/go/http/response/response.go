package response

import (
	"github.com/gofiber/fiber/v2"
)

// APIResponse is the standard GN-WAAS API response envelope
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

// APIError represents a structured API error
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// Meta holds response metadata (pagination, timing, etc.)
type Meta struct {
	Total      *int    `json:"total,omitempty"`
	Page       *int    `json:"page,omitempty"`
	PageSize   *int    `json:"page_size,omitempty"`
	TotalPages *int    `json:"total_pages,omitempty"`
	HasNext    *bool   `json:"has_next,omitempty"`
	HasPrev    *bool   `json:"has_prev,omitempty"`
	RequestID  string  `json:"request_id,omitempty"`
}

// OK sends a 200 success response
func OK(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(APIResponse{
		Success: true,
		Data:    data,
	})
}

// Created sends a 201 created response
func Created(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(APIResponse{
		Success: true,
		Data:    data,
	})
}

// OKWithMeta sends a 200 response with pagination metadata
func OKWithMeta(c *fiber.Ctx, data interface{}, meta *Meta) error {
	return c.Status(fiber.StatusOK).JSON(APIResponse{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}

// BadRequest sends a 400 error response
func BadRequest(c *fiber.Ctx, code, message string) error {
	return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}

// Unauthorized sends a 401 error response
func Unauthorized(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusUnauthorized).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "UNAUTHORIZED",
			Message: message,
		},
	})
}

// Forbidden sends a 403 error response
func Forbidden(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusForbidden).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "FORBIDDEN",
			Message: message,
		},
	})
}

// NotFound sends a 404 error response
func NotFound(c *fiber.Ctx, resource string) error {
	return c.Status(fiber.StatusNotFound).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "NOT_FOUND",
			Message: resource + " not found",
		},
	})
}

// Conflict sends a 409 error response
func Conflict(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusConflict).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "CONFLICT",
			Message: message,
		},
	})
}

// UnprocessableEntity sends a 422 error response with field details
func UnprocessableEntity(c *fiber.Ctx, details map[string]string) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "VALIDATION_ERROR",
			Message: "Request validation failed",
			Details: details,
		},
	})
}

// InternalError sends a 500 error response
func InternalError(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "INTERNAL_ERROR",
			Message: message,
		},
	})
}

// ServiceUnavailable sends a 503 error response
func ServiceUnavailable(c *fiber.Ctx, service string) error {
	return c.Status(fiber.StatusServiceUnavailable).JSON(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "SERVICE_UNAVAILABLE",
			Message: service + " is temporarily unavailable",
		},
	})
}
