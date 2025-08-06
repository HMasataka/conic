package logging

import (
	"log/slog"
	"os"
	"strings"

	"github.com/sytallax/prettylog"
)

type Config struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

type Logger struct {
	*slog.Logger
}

func New(cfg Config) *Logger {
	level := parseLevel(cfg.Level)

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = prettylog.NewHandler(opts)
	}

	return &Logger{
		Logger: slog.New(handler),
	}
}

func (l *Logger) WithFields(fields map[string]any) *Logger {
	attrs := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		attrs = append(attrs, k, v)
	}
	return &Logger{
		Logger: l.With(attrs...),
	}
}

// parseLevel parses a string log level
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
