package errors

import (
	"context"
	"log/slog"
)

// Handler handles errors in a consistent way
type Handler interface {
	// Handle processes an error
	Handle(ctx context.Context, err error)

	// HandleWithLogger processes an error with a specific logger
	HandleWithLogger(ctx context.Context, err error, logger *slog.Logger)
}

// DefaultHandler is the default error handler
type DefaultHandler struct {
	logger *slog.Logger
}

// NewDefaultHandler creates a new default error handler
func NewDefaultHandler(logger *slog.Logger) *DefaultHandler {
	return &DefaultHandler{
		logger: logger,
	}
}

// Handle implements the Handler interface
func (h *DefaultHandler) Handle(ctx context.Context, err error) {
	h.HandleWithLogger(ctx, err, h.logger)
}

// HandleWithLogger implements the Handler interface
func (h *DefaultHandler) HandleWithLogger(ctx context.Context, err error, logger *slog.Logger) {
	if err == nil {
		return
	}

	// Type assert to our custom error type
	if e, ok := err.(*Error); ok {
		attrs := []any{
			slog.String("error_code", e.Code),
			slog.String("error_type", errorTypeToString(e.Type)),
			slog.Time("timestamp", e.Timestamp),
		}

		if e.Details != "" {
			attrs = append(attrs, slog.String("details", e.Details))
		}

		if e.Cause != nil {
			attrs = append(attrs, slog.String("cause", e.Cause.Error()))
		}

		switch e.Type {
		case ErrorTypeInternal:
			logger.ErrorContext(ctx, e.Message, attrs...)
		case ErrorTypeTimeout, ErrorTypeNotFound:
			logger.WarnContext(ctx, e.Message, attrs...)
		default:
			logger.InfoContext(ctx, e.Message, attrs...)
		}
	} else {
		// Handle standard errors
		logger.ErrorContext(ctx, "unhandled error", slog.String("error", err.Error()))
	}
}

// errorTypeToString converts ErrorType to string
func errorTypeToString(t ErrorType) string {
	switch t {
	case ErrorTypeTransport:
		return "transport"
	case ErrorTypeProtocol:
		return "protocol"
	case ErrorTypeWebRTC:
		return "webrtc"
	case ErrorTypeNotFound:
		return "not_found"
	case ErrorTypeUnauthorized:
		return "unauthorized"
	case ErrorTypeInternal:
		return "internal"
	case ErrorTypeTimeout:
		return "timeout"
	case ErrorTypeValidation:
		return "validation"
	default:
		return "unknown"
	}
}
