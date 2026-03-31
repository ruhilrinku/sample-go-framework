package domain

import "fmt"

// ErrorType classifies domain errors for proper handling at adapter boundaries.
type ErrorType int

const (
	ErrorTypeUnknown ErrorType = iota
	ErrorTypeValidation
	ErrorTypeNotFound
	ErrorTypeInternal
)

// DomainError is a typed error that carries classification and context.
type DomainError struct {
	Type    ErrorType
	Message string
	Err     error
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// NewValidationError creates a validation error (bad input).
func NewValidationError(message string) *DomainError {
	return &DomainError{Type: ErrorTypeValidation, Message: message}
}

// NewNotFoundError creates a not-found error.
func NewNotFoundError(message string) *DomainError {
	return &DomainError{Type: ErrorTypeNotFound, Message: message}
}

// NewInternalError creates an internal error wrapping an underlying cause.
func NewInternalError(message string, err error) *DomainError {
	return &DomainError{Type: ErrorTypeInternal, Message: message, Err: err}
}

// GetErrorType extracts the ErrorType from an error if it is a DomainError,
// otherwise returns ErrorTypeUnknown.
func GetErrorType(err error) ErrorType {
	if err == nil {
		return ErrorTypeUnknown
	}
	if de, ok := err.(*DomainError); ok {
		return de.Type
	}
	return ErrorTypeUnknown
}
