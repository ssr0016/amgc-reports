package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// ContextKey is a custom type for context keys to avoid collisions.
type ContextKey string

const (
	// RequestIDKey is the context key for the request ID.
	RequestIDKey ContextKey = "request_id"

	// LoggerKey is the context key for the logger.
	LoggerKey ContextKey = "logger"
)

// Config holds the logger configuration.
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, text
}

// New creates a new slog.Logger based on the config.
func New(cfg Config) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}

	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// WithRequestID returns a logger with the request ID from context.
func WithRequestID(ctx context.Context, log *slog.Logger) *slog.Logger {
	if ctx == nil {
		return log
	}
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
		return log.With("request_id", reqID)
	}
	return log
}

// FromContext returns a logger from context, or the default logger.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return slog.Default()
	}
	if log, ok := ctx.Value(LoggerKey).(*slog.Logger); ok {
		return log
	}
	return slog.Default()
}

// WithContext returns a new context with the logger attached.
func WithContext(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, LoggerKey, log)
}

// WithRequestIDContext returns a new context with the request ID attached.
func WithRequestIDContext(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, reqID)
}
