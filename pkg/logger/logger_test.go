package logger_test

import (
	"context"
	"scanner/pkg/logger"
	"testing"

	"github.com/stretchr/testify/require"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestSetup(t *testing.T) {
	tests := []struct {
		name        string
		environment string
	}{
		{
			name:        "Development Environment",
			environment: logger.DevelopmentEnvironment,
		},
		{
			name:        "Production Environment",
			environment: logger.ProductionEnvironment,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// setup should not panic
			require.NotPanics(t, func() {
				logger.Setup(tt.environment)
			})

			// get a logger from context to verify setup worked
			ctx := context.Background()
			l := logger.Get(ctx)
			require.NotNil(t, l)
		})
	}
}

func TestGet(t *testing.T) {
	// setup logger
	logger.Setup(logger.DevelopmentEnvironment)

	// test with empty context
	ctx := context.Background()
	l := logger.Get(ctx)
	require.NotNil(t, l, "Should return default logger when context has no logger")

	// test with logger in context
	customLogger, _ := zap.NewDevelopment()
	ctxWithLogger := logger.WithLogger(ctx, customLogger)
	l = logger.Get(ctxWithLogger)
	require.Equal(t, customLogger, l, "Should return logger from context")
}

func TestWithLogger(t *testing.T) {
	ctx := context.Background()
	customLogger, _ := zap.NewDevelopment()

	// add logger to context
	ctxWithLogger := logger.WithLogger(ctx, customLogger)

	// verify logger is in context
	l := logger.Get(ctxWithLogger)
	require.Equal(t, customLogger, l, "Logger in context should match the one we added")
}

func TestWithFields(t *testing.T) {
	// setup
	logger.Setup(logger.DevelopmentEnvironment)
	ctx := context.Background()

	// add fields to logger in context
	fields := []zapcore.Field{
		zap.String("key1", "value1"),
		zap.Int("key2", 42),
	}

	ctxWithFields := logger.WithFields(ctx, fields...)

	// we can't directly test the fields are added since zap.Logger doesn't expose its fields
	// but we can verify the context has a logger
	l := logger.Get(ctxWithFields)
	require.NotNil(t, l, "Context should have a logger with fields")
}

func TestIsDebug(t *testing.T) {
	// setup development logger (which should be at debug level)
	logger.Setup(logger.DevelopmentEnvironment)
	ctx := context.Background()

	// test debug level detection
	require.True(t, logger.IsDebug(ctx), "Development logger should be at debug level")

	// create a custom logger at info level
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	infoLogger, _ := cfg.Build()

	// add to context and test
	ctxWithInfoLogger := logger.WithLogger(ctx, infoLogger)
	require.False(t, logger.IsDebug(ctxWithInfoLogger), "Info level logger should not be at debug level")
}

func TestLoggingFunctions(t *testing.T) {
	// setup
	logger.Setup(logger.DevelopmentEnvironment)
	ctx := context.Background()

	// test that logging functions don't panic
	require.NotPanics(t, func() {
		logger.Debug(ctx, "debug message", zap.String("key", "value"))
	})

	require.NotPanics(t, func() {
		logger.Info(ctx, "info message", zap.String("key", "value"))
	})

	require.NotPanics(t, func() {
		logger.Warn(ctx, "warn message", zap.String("key", "value"))
	})

	require.NotPanics(t, func() {
		logger.Error(ctx, "error message", zap.String("key", "value"))
	})
}
