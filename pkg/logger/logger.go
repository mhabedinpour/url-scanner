// Package logger provides a structured logging facility using zap logger.
// It offers context-aware logging capabilities, environment-specific configuration,
// and helper functions for different log levels.
package logger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// DevelopmentEnvironment represents the development environment setting.
	// In this environment, the logger is configured with development settings (more verbose, human-readable).
	DevelopmentEnvironment = "development"

	// ProductionEnvironment represents the production environment setting.
	// In this environment, the logger is configured with production settings (less verbose, JSON format).
	ProductionEnvironment = "production"
)

// defaultLogger is the package-level logger instance used when no logger is found in context.
var defaultLogger *zap.Logger //nolint: gochecknoglobals

// Setup initializes the default logger based on the environment.
// It configures the logger with appropriate settings for either development or production use.
//
// Parameters:
//   - environment: A string indicating the environment ("development" or "production").
func Setup(environment string) {
	if environment == ProductionEnvironment {
		defaultLogger, _ = zap.NewProduction()

		return
	}

	defaultLogger, _ = zap.NewDevelopment()
}

// key is a custom type used as a context key for storing and retrieving logger instances.
type key struct{}

// Get retrieves a logger from the provided context.
// If no logger is found in the context, it returns the default logger.
func Get(ctx context.Context) *zap.Logger {
	if logger, _ := ctx.Value(key{}).(*zap.Logger); logger != nil {
		return logger
	}

	return defaultLogger
}

// WithLogger creates a new context with the provided logger attached.
// This allows for context-specific logging with custom logger instances.
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, key{}, logger)
}

// WithFields creates a new context with a logger that includes the specified fields.
// This is useful for adding structured data to all log messages within a context.
func WithFields(ctx context.Context, fields ...zapcore.Field) context.Context {
	return WithLogger(ctx, Get(ctx).With(fields...))
}

// IsDebug checks if the logger in the context is configured at debug level.
func IsDebug(ctx context.Context) bool {
	return Get(ctx).Level() == zap.DebugLevel
}

// Debug logs a message at debug level with the given fields.
func Debug(ctx context.Context, msg string, fields ...zapcore.Field) {
	Get(ctx).Debug(msg, fields...)
}

// Info logs a message at info level with the given fields.
func Info(ctx context.Context, msg string, fields ...zapcore.Field) {
	Get(ctx).Info(msg, fields...)
}

// Warn logs a message at warn level with the given fields.
func Warn(ctx context.Context, msg string, fields ...zapcore.Field) {
	Get(ctx).Warn(msg, fields...)
}

// Error logs a message at error level with the given fields.
func Error(ctx context.Context, msg string, fields ...zapcore.Field) {
	Get(ctx).Error(msg, fields...)
}

// Fatal logs a message at fatal level with the given fields.
func Fatal(ctx context.Context, msg string, fields ...zapcore.Field) {
	Get(ctx).Fatal(msg, fields...)
}
