package errors

import (
	"fmt"
	"runtime"
)

// AppError represents an application-specific error
type AppError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Details   string `json:"details,omitempty"`
	Cause     error  `json:"-"`
	File      string `json:"file,omitempty"`
	Line      int    `json:"line,omitempty"`
	Operation string `json:"operation,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewAppError creates a new application error
func NewAppError(code, message string, cause error) *AppError {
	_, file, line, _ := runtime.Caller(1)
	return &AppError{
		Code:    code,
		Message: message,
		Cause:   cause,
		File:    file,
		Line:    line,
	}
}

// WithOperation adds operation context to the error
func (e *AppError) WithOperation(operation string) *AppError {
	e.Operation = operation
	return e
}

// WithDetails adds additional details to the error
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// Common error codes
const (
	ErrCodeNotFound         = "NOT_FOUND"
	ErrCodeInvalidInput     = "INVALID_INPUT"
	ErrCodeUnauthorized     = "UNAUTHORIZED"
	ErrCodeForbidden        = "FORBIDDEN"
	ErrCodeInternalError    = "INTERNAL_ERROR"
	ErrCodeDatabaseError    = "DATABASE_ERROR"
	ErrCodeValidationError  = "VALIDATION_ERROR"
	ErrCodeConflict         = "CONFLICT"
	ErrCodeServiceError     = "SERVICE_ERROR"
)

// Common error constructors
func NotFound(message string, cause error) *AppError {
	return NewAppError(ErrCodeNotFound, message, cause)
}

func InvalidInput(message string, cause error) *AppError {
	return NewAppError(ErrCodeInvalidInput, message, cause)
}

func Unauthorized(message string, cause error) *AppError {
	return NewAppError(ErrCodeUnauthorized, message, cause)
}

func Forbidden(message string, cause error) *AppError {
	return NewAppError(ErrCodeForbidden, message, cause)
}

func InternalError(message string, cause error) *AppError {
	return NewAppError(ErrCodeInternalError, message, cause)
}

func DatabaseError(message string, cause error) *AppError {
	return NewAppError(ErrCodeDatabaseError, message, cause)
}

func ValidationError(message string, cause error) *AppError {
	return NewAppError(ErrCodeValidationError, message, cause)
}

func Conflict(message string, cause error) *AppError {
	return NewAppError(ErrCodeConflict, message, cause)
}

func ServiceError(message string, cause error) *AppError {
	return NewAppError(ErrCodeServiceError, message, cause)
}