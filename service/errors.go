package service

import "fmt"

// ErrorType represents the type of error that occurred
type ErrorType string

const (
	ErrorTypeNotFound   ErrorType = "not_found"
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeConflict   ErrorType = "conflict"
	ErrorTypeInternal   ErrorType = "internal"
	ErrorTypeUnknown    ErrorType = "unknown"
)

// ServiceError represents a service layer error with additional context
type ServiceError struct {
	Type    ErrorType
	Message string
	Err     error
}

// Error implements the error interface
func (e *ServiceError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the wrapped error
func (e *ServiceError) Unwrap() error {
	return e.Err
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(message string) *ServiceError {
	return &ServiceError{
		Type:    ErrorTypeNotFound,
		Message: message,
	}
}

// NewValidationError creates a new validation error
func NewValidationError(message string, err error) *ServiceError {
	return &ServiceError{
		Type:    ErrorTypeValidation,
		Message: message,
		Err:     err,
	}
}

// NewConflictError creates a new conflict error
func NewConflictError(message string) *ServiceError {
	return &ServiceError{
		Type:    ErrorTypeConflict,
		Message: message,
	}
}

// NewInternalError creates a new internal error
func NewInternalError(message string, err error) *ServiceError {
	return &ServiceError{
		Type:    ErrorTypeInternal,
		Message: message,
		Err:     err,
	}
}
