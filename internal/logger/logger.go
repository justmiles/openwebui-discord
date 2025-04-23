package logger

import (
	"fmt"
	"os"

	"github.com/justmiles/openwebui-discord/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Global logger instance
	log *zap.Logger
)

// Init initializes the logger with the provided configuration
func Init(cfg *config.Config) error {
	
	// Configure logging level
	level := zapcore.InfoLevel
	if err := level.UnmarshalText([]byte(cfg.Logging.Level)); err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}
	
	// Configure encoder based on format
	var encoder zapcore.Encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	
	if cfg.Logging.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}
	
	// Configure output
	var output zapcore.WriteSyncer
	if cfg.Logging.File != "" {
		file, err := os.OpenFile(cfg.Logging.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("could not open log file: %w", err)
		}
		output = zapcore.AddSync(file)
	} else {
		output = zapcore.AddSync(os.Stdout)
	}
	
	// Create core
	core := zapcore.NewCore(encoder, output, level)
	
	// Create logger
	log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	
	return nil
}

// With creates a child logger with additional fields
func With(fields ...zapcore.Field) *zap.Logger {
	return log.With(fields...)
}

// Debug logs a debug message
func Debug(msg string, fields ...zapcore.Field) {
	log.Debug(msg, fields...)
}

// Info logs an info message
func Info(msg string, fields ...zapcore.Field) {
	log.Info(msg, fields...)
}

// Warn logs a warning message
func Warn(msg string, fields ...zapcore.Field) {
	log.Warn(msg, fields...)
}

// Error logs an error message
func Error(msg string, fields ...zapcore.Field) {
	log.Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, fields ...zapcore.Field) {
	log.Fatal(msg, fields...)
}

// Sync flushes any buffered log entries
func Sync() error {
	return log.Sync()
}

// Field creates a field for structured logging
func Field(key string, value interface{}) zapcore.Field {
	return zap.Any(key, value)
}

// String creates a string field for structured logging
func String(key string, value string) zapcore.Field {
	return zap.String(key, value)
}

// Int creates an int field for structured logging
func Int(key string, value int) zapcore.Field {
	return zap.Int(key, value)
}

// Error creates an error field for structured logging
func ErrorField(key string, err error) zapcore.Field {
	return zap.Error(err)
}