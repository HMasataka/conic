package domain

import (
	"errors"
	"fmt"
)

// Common domain errors
var (
	// ErrClientNotFound is returned when a client is not found
	ErrClientNotFound = errors.New("client not found")

	// ErrClientAlreadyExists is returned when trying to register a client that already exists
	ErrClientAlreadyExists = errors.New("client already exists")

	// ErrInvalidMessage is returned when a message is invalid
	ErrInvalidMessage = errors.New("invalid message")

	// ErrHubNotStarted is returned when trying to use a hub that hasn't been started
	ErrHubNotStarted = errors.New("hub not started")

	// ErrHubStopped is returned when trying to use a hub that has been stopped
	ErrHubStopped = errors.New("hub stopped")

	// ErrConnectionClosed is returned when trying to use a closed connection
	ErrConnectionClosed = errors.New("connection closed")

	// ErrTimeout is returned when an operation times out
	ErrTimeout = errors.New("operation timed out")
)

// DomainError represents a domain-specific error
type DomainError struct {
	Code    string
	Message string
	Cause   error
}

// Error implements the error interface
func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *DomainError) Unwrap() error {
	return e.Cause
}

// NewDomainError creates a new domain error
func NewDomainError(code, message string, cause error) *DomainError {
	return &DomainError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// Error codes
const (
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeAlreadyExists = "ALREADY_EXISTS"
	ErrCodeInvalid       = "INVALID"
	ErrCodeInternal      = "INTERNAL"
	ErrCodeUnauthorized  = "UNAUTHORIZED"
	ErrCodeTimeout       = "TIMEOUT"
)
