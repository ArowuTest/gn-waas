package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode defines machine-readable error codes for GN-WAAS
type ErrorCode string

const (
	// General errors
	ErrCodeInternal          ErrorCode = "INTERNAL_ERROR"
	ErrCodeNotFound          ErrorCode = "NOT_FOUND"
	ErrCodeValidation        ErrorCode = "VALIDATION_ERROR"
	ErrCodeUnauthorised      ErrorCode = "UNAUTHORISED"
	ErrCodeForbidden         ErrorCode = "FORBIDDEN"
	ErrCodeConflict          ErrorCode = "CONFLICT"
	ErrCodeBadRequest        ErrorCode = "BAD_REQUEST"

	// Domain-specific errors
	ErrCodeAccountNotFound   ErrorCode = "ACCOUNT_NOT_FOUND"
	ErrCodeAuditLocked       ErrorCode = "AUDIT_LOCKED"         // Audit is GRA-signed, immutable
	ErrCodeGRAAPIFailure     ErrorCode = "GRA_API_FAILURE"
	ErrCodeGRATimeout        ErrorCode = "GRA_API_TIMEOUT"
	ErrCodeOCRFailed         ErrorCode = "OCR_FAILED"
	ErrCodeGPSOutOfRange     ErrorCode = "GPS_OUT_OF_RANGE"
	ErrCodeBiometricFailed   ErrorCode = "BIOMETRIC_FAILED"
	ErrCodeCDCConnectionLost ErrorCode = "CDC_CONNECTION_LOST"
	ErrCodeTariffNotFound    ErrorCode = "TARIFF_NOT_FOUND"
	ErrCodeInvalidCategory   ErrorCode = "INVALID_CATEGORY"
	ErrCodeThresholdViolation ErrorCode = "THRESHOLD_VIOLATION"
)

// AppError is the standard error type for GN-WAAS
type AppError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	Details    string    `json:"details,omitempty"`
	HTTPStatus int       `json:"-"`
	Err        error     `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// Constructor helpers
func New(code ErrorCode, message string) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: http.StatusInternalServerError}
}

func NotFound(resource string) *AppError {
	return &AppError{
		Code:       ErrCodeNotFound,
		Message:    fmt.Sprintf("%s not found", resource),
		HTTPStatus: http.StatusNotFound,
	}
}

func Validation(message string) *AppError {
	return &AppError{
		Code:       ErrCodeValidation,
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
	}
}

func Unauthorised(message string) *AppError {
	return &AppError{
		Code:       ErrCodeUnauthorised,
		Message:    message,
		HTTPStatus: http.StatusUnauthorized,
	}
}

func Forbidden(message string) *AppError {
	return &AppError{
		Code:       ErrCodeForbidden,
		Message:    message,
		HTTPStatus: http.StatusForbidden,
	}
}

func Internal(err error) *AppError {
	return &AppError{
		Code:       ErrCodeInternal,
		Message:    "An internal error occurred",
		HTTPStatus: http.StatusInternalServerError,
		Err:        err,
	}
}

func AuditLocked() *AppError {
	return &AppError{
		Code:       ErrCodeAuditLocked,
		Message:    "This audit record is GRA-signed and immutable",
		HTTPStatus: http.StatusForbidden,
	}
}

func GRAFailure(details string) *AppError {
	return &AppError{
		Code:       ErrCodeGRAAPIFailure,
		Message:    "GRA VSDC API signing failed",
		Details:    details,
		HTTPStatus: http.StatusBadGateway,
	}
}
