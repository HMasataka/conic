package errors

import (
	"fmt"
	"time"
)

// ErrorType represents the type of error
type ErrorType int

const (
	// ErrorTypeTransport indicates a transport layer error
	ErrorTypeTransport ErrorType = iota
	// ErrorTypeProtocol indicates a protocol error
	ErrorTypeProtocol
	// ErrorTypeWebRTC indicates a WebRTC error
	ErrorTypeWebRTC
	// ErrorTypeNotFound indicates a not found error
	ErrorTypeNotFound
	// ErrorTypeUnauthorized indicates an authorization error
	ErrorTypeUnauthorized
	// ErrorTypeInternal indicates an internal error
	ErrorTypeInternal
	// ErrorTypeTimeout indicates a timeout error
	ErrorTypeTimeout
	// ErrorTypeValidation indicates a validation error
	ErrorTypeValidation
)

// Error represents a structured error with metadata
type Error struct {
	Type      ErrorType  `json:"type"`
	Code      string     `json:"code"`
	Message   string     `json:"message"`
	Details   string     `json:"details,omitempty"`
	Cause     error      `json:"-"`
	Timestamp time.Time  `json:"timestamp"`
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %s (caused by: %v)", e.Code, e.Message, e.Details, e.Cause)
	}
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is checks if the error is of a specific type
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Type == t.Type && e.Code == t.Code
}

// New creates a new error
func New(errorType ErrorType, code, message string) *Error {
	return &Error{
		Type:      errorType,
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// Wrap wraps an error with additional context
func Wrap(err error, errorType ErrorType, code, message string) *Error {
	return &Error{
		Type:      errorType,
		Code:      code,
		Message:   message,
		Cause:     err,
		Timestamp: time.Now(),
	}
}

// WithDetails adds details to an error
func (e *Error) WithDetails(details string) *Error {
	e.Details = details
	return e
}