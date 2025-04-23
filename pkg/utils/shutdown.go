package utils

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/justmiles/openwebui-discord/internal/logger"
	"go.uber.org/zap"
)

// GracefulShutdown manages graceful shutdown of the application
type GracefulShutdown struct {
	timeout time.Duration
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewGracefulShutdown creates a new graceful shutdown manager
func NewGracefulShutdown(timeout time.Duration) *GracefulShutdown {
	ctx, cancel := context.WithCancel(context.Background())
	return &GracefulShutdown{
		timeout: timeout,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Context returns the shutdown context
func (gs *GracefulShutdown) Context() context.Context {
	return gs.ctx
}

// AddTask adds a task to the wait group
func (gs *GracefulShutdown) AddTask() {
	gs.wg.Add(1)
}

// TaskDone marks a task as done
func (gs *GracefulShutdown) TaskDone() {
	gs.wg.Done()
}

// WaitForSignal waits for termination signals and initiates shutdown
func (gs *GracefulShutdown) WaitForSignal() {
	// Create channel for signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal
	sig := <-sigChan
	logger.Info("Received shutdown signal", zap.String("signal", sig.String()))

	// Cancel context to notify all components
	gs.cancel()

	// Create a timeout context for the wait group
	timeoutCtx, cancel := context.WithTimeout(context.Background(), gs.timeout)
	defer cancel()

	// Create a channel to signal when the wait group is done
	done := make(chan struct{})
	go func() {
		gs.wg.Wait()
		close(done)
	}()

	// Wait for either the wait group to finish or the timeout to expire
	select {
	case <-done:
		logger.Info("Graceful shutdown completed")
	case <-timeoutCtx.Done():
		logger.Warn("Graceful shutdown timed out, forcing exit")
	}
}

// WithGracefulShutdown runs a function with graceful shutdown handling
func WithGracefulShutdown(timeout time.Duration, fn func(context.Context) error) error {
	gs := NewGracefulShutdown(timeout)
	
	// Run the function in a goroutine
	errChan := make(chan error, 1)
	go func() {
		gs.AddTask()
		defer gs.TaskDone()
		errChan <- fn(gs.Context())
	}()
	
	// Wait for signals
	go gs.WaitForSignal()
	
	// Wait for the function to complete or context to be canceled
	select {
	case err := <-errChan:
		return err
	case <-gs.Context().Done():
		// Wait for the function to finish cleanup
		select {
		case err := <-errChan:
			return err
		case <-time.After(timeout):
			return context.DeadlineExceeded
		}
	}
}