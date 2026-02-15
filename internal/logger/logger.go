// Package logger provides structured logging via zap.
package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	envProduction = "production"
	fmtConsole    = "console"
	fmtJSON       = "json"
)

var globalLogger *zap.Logger //nolint:gochecknoglobals // singleton logger pattern

// Config holds logger configuration.
type Config struct {
	Level       string // debug, info, warn, error.
	Environment string // development, production.
	Format      string // json, console.
}

// Init initialises the global logger.
func Init(config Config) error {
	level := parseLevel(config.Level)
	zapConfig := buildZapConfig(config, level)

	var err error

	globalLogger, err = zapConfig.Build(
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return fmt.Errorf("failed to build logger: %w", err)
	}

	return nil
}

func parseLevel(lvl string) zapcore.Level {
	switch lvl {
	case "debug":
		return zap.DebugLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

func buildZapConfig(config Config, level zapcore.Level) zap.Config {
	if config.Environment == envProduction {
		cfg := zap.NewProductionConfig()

		cfg.Level = zap.NewAtomicLevelAt(level)

		if config.Format == fmtConsole {
			cfg.Encoding = fmtConsole
		} else {
			cfg.Encoding = fmtJSON
		}

		return cfg
	}

	cfg := zap.NewDevelopmentConfig()

	cfg.Level = zap.NewAtomicLevelAt(level)
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	if config.Format == fmtJSON {
		cfg.Encoding = fmtJSON
	} else {
		cfg.Encoding = fmtConsole
	}

	return cfg
}

// GetLogger returns the global logger instance.
func GetLogger() *zap.Logger {
	if globalLogger == nil {
		globalLogger, _ = zap.NewDevelopment()
	}

	return globalLogger
}

// Sync flushes any buffered log entries.
func Sync() {
	if globalLogger != nil {
		_ = globalLogger.Sync()
	}
}

// ComponentLogger provides component-scoped logging.
type ComponentLogger struct {
	logger *zap.Logger
}

// NewComponentLogger creates a ComponentLogger.
func NewComponentLogger(component string) *ComponentLogger {
	return &ComponentLogger{
		logger: GetLogger().With(zap.String("component", component)),
	}
}

// Info logs an informational message.
func (l *ComponentLogger) Info(msg string, fields ...zap.Field) {
	l.logger.Info(msg, fields...)
}

// Error logs an error message.
func (l *ComponentLogger) Error(msg string, err error, fields ...zap.Field) {
	allFields := make([]zap.Field, 0, len(fields)+1)
	allFields = append(allFields, fields...)
	allFields = append(allFields, zap.Error(err))
	l.logger.Error(msg, allFields...)
}

// Debug logs a debug message.
func (l *ComponentLogger) Debug(msg string, fields ...zap.Field) {
	l.logger.Debug(msg, fields...)
}

// Warn logs a warning message.
func (l *ComponentLogger) Warn(msg string, fields ...zap.Field) {
	l.logger.Warn(msg, fields...)
}

// DefaultConfig returns logger Config based on environment variables.
func DefaultConfig() Config {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		if env == envProduction {
			logLevel = "info"
		} else {
			logLevel = "debug"
		}
	}

	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat == "" {
		if env == envProduction {
			logFormat = fmtJSON
		} else {
			logFormat = fmtConsole
		}
	}

	return Config{
		Level:       logLevel,
		Environment: env,
		Format:      logFormat,
	}
}
