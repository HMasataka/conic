package logging

import (
	"context"
)

type contextKey string

const (
	loggerKey contextKey = "logger"
)

func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(loggerKey).(*Logger); ok {
		return logger
	}

	return New(Config{Level: "info", Format: "text"})
}

func WithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}
