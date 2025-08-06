package logging

import (
	"context"
)

type contextKey string

const (
	loggerKey contextKey = "logger"
)

// FromContext extracts a logger from the context
func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(loggerKey).(*Logger); ok {
		return logger
	}
	// Return a default logger if none is found
	return New(Config{Level: "info", Format: "text"})
}

// WithLogger adds a logger to the context
func WithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// AddFields adds logging fields to the context
func AddFields(ctx context.Context, fields ...any) context.Context {
	logger := FromContext(ctx)
	newLogger := &Logger{
		Logger: logger.With(fields...),
	}
	return WithLogger(ctx, newLogger)
}
