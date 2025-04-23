package utils

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/justmiles/openwebui-discord/internal/logger"
	"go.uber.org/zap"
)

// Common errors
var (
	ErrNotFound      = errors.New("not found")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrRateLimited   = errors.New("rate limited")
	ErrTimeout       = errors.New("timeout")
	ErrInvalidInput  = errors.New("invalid input")
	ErrInternal      = errors.New("internal error")
)

// ErrorWithContext wraps an error with additional context
type ErrorWithContext struct {
	Err     error
	Context map[string]interface{}
	Stack   string
}

// Error implements the error interface
func (e *ErrorWithContext) Error() string {
	if e.Err == nil {
		return "nil error"
	}
	return e.Err.Error()
}

// Unwrap returns the wrapped error
func (e *ErrorWithContext) Unwrap() error {
	return e.Err
}

// WithContext adds context to an error
func WithContext(err error, context map[string]interface{}) error {
	if err == nil {
		return nil
	}

	// If the error already has context, add to it
	var errWithContext *ErrorWithContext
	if errors.As(err, &errWithContext) {
		for k, v := range context {
			errWithContext.Context[k] = v
		}
		return errWithContext
	}

	// Create a new error with context
	stack := captureStack(2) // Skip this function and the caller
	return &ErrorWithContext{
		Err:     err,
		Context: context,
		Stack:   stack,
	}
}

// LogError logs an error with its context
func LogError(err error, msg string, fields ...zap.Field) {
	if err == nil {
		return
	}

	// Extract context if available
	var errWithContext *ErrorWithContext
	if errors.As(err, &errWithContext) {
		// Add context fields
		for k, v := range errWithContext.Context {
			fields = append(fields, zap.Any(k, v))
		}

		// Add stack trace if available
		if errWithContext.Stack != "" {
			fields = append(fields, zap.String("stack", errWithContext.Stack))
		}

		// Use the wrapped error for the error field
		fields = append(fields, zap.Error(errWithContext.Err))
	} else {
		// Just log the error directly
		fields = append(fields, zap.Error(err))
	}

	logger.Error(msg, fields...)
}

// captureStack captures a stack trace
func captureStack(skip int) string {
	const maxStackDepth = 20
	stackTrace := make([]uintptr, maxStackDepth)
	length := runtime.Callers(skip+1, stackTrace)
	stack := stackTrace[:length]

	var sb strings.Builder
	frames := runtime.CallersFrames(stack)
	for {
		frame, more := frames.Next()
		fmt.Fprintf(&sb, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
	return sb.String()
}

// IsRetryable determines if an error should be retried
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types
	if errors.Is(err, ErrRateLimited) || errors.Is(err, ErrTimeout) {
		return true
	}

	// Check error string for common retryable patterns
	errStr := err.Error()
	retryablePatterns := []string{
		"connection reset",
		"connection refused",
		"temporary failure",
		"timeout",
		"too many requests",
		"try again",
		"503",
		"429",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}

	return false
}

// WithRetry attempts an operation with retries
func WithRetry(maxRetries int, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		if !IsRetryable(err) || attempt == maxRetries {
			break
		}

		// Exponential backoff
		backoff := 1 << uint(attempt) // 1, 2, 4, 8, 16, ...
		if backoff > 30 {
			backoff = 30 // Cap at 30 seconds
		}

		logger.Debug("Retrying after error",
			zap.Error(err),
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", maxRetries),
			zap.Int("backoff_seconds", backoff),
		)

		// Sleep with backoff
		runtime.Gosched() // Yield to other goroutines
		timer := time.NewTimer(time.Duration(backoff) * time.Second)
		<-timer.C
	}

	return lastErr
}