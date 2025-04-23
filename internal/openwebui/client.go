package openwebui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/justmiles/openwebui-discord/internal/logger"
	"github.com/justmiles/openwebui-discord/internal/ratelimit"
	"go.uber.org/zap"
)

// Client represents an OpenWebUI API client
type Client struct {
	endpoint    string
	apiKey      string
	model       string
	timeout     time.Duration
	client      *http.Client
	rateLimiter *ratelimit.Limiter
}

// NewClient creates a new OpenWebUI API client
func NewClient(endpoint, apiKey, model string, timeoutSeconds, requestsPerMinute int) *Client {
	return &Client{
		endpoint:    endpoint,
		apiKey:      apiKey,
		model:       model,
		timeout:     time.Duration(timeoutSeconds) * time.Second,
		client:      &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
		rateLimiter: ratelimit.NewLimiter(requestsPerMinute),
	}
}

// ChatCompletion sends a chat completion request to the OpenWebUI API
func (c *Client) ChatCompletion(ctx context.Context, messages []Message) (*ChatCompletionResponse, error) {
	// Apply rate limiting
	c.rateLimiter.Wait()

	// Create request
	reqBody := ChatCompletionRequest{
		Model:    c.model,
		Messages: messages,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Create HTTP request
	url := fmt.Sprintf("%s/v1/chat/completions", c.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	// Log request (excluding sensitive data)
	logger.Debug("Sending request to OpenWebUI API",
		zap.String("url", url),
		zap.Int("message_count", len(messages)),
	)

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, fmt.Errorf("API error: %s (type: %s, code: %s)",
				errResp.Error.Message,
				errResp.Error.Type,
				errResp.Error.Code)
		}
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	// Log response (excluding sensitive data)
	logger.Debug("Received response from OpenWebUI API",
		zap.Int("choices", len(chatResp.Choices)),
		zap.Int("total_tokens", chatResp.Usage.TotalTokens),
	)

	return &chatResp, nil
}

// GetCompletion is a convenience method that returns just the completion text
func (c *Client) GetCompletion(ctx context.Context, messages []Message) (string, error) {
	resp, err := c.ChatCompletion(ctx, messages)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", errors.New("no completion choices returned")
	}

	return resp.Choices[0].Message.Content, nil
}

// WithRetry attempts to get a completion with retries and exponential backoff
func (c *Client) WithRetry(ctx context.Context, messages []Message, maxRetries int) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// If this isn't the first attempt, wait with exponential backoff
		if attempt > 0 {
			backoffDuration := time.Duration(1<<uint(attempt-1)) * time.Second
			if backoffDuration > 30*time.Second {
				backoffDuration = 30 * time.Second // Cap at 30 seconds
			}

			logger.Info("Retrying OpenWebUI API request",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoffDuration),
				zap.Error(lastErr),
			)

			select {
			case <-time.After(backoffDuration):
				// Continue after backoff
			case <-ctx.Done():
				return "", fmt.Errorf("context cancelled during backoff: %w", ctx.Err())
			}
		}

		// Attempt the request
		completion, err := c.GetCompletion(ctx, messages)
		if err == nil {
			// Success!
			if attempt > 0 {
				logger.Info("Successfully completed request after retries",
					zap.Int("attempts", attempt+1),
				)
			}
			return completion, nil
		}

		// Save the error for potential logging
		lastErr = err

		// Check if we should retry based on the error
		if !isRetryableError(err) {
			return "", fmt.Errorf("non-retryable error: %w", err)
		}
	}

	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// isRetryableError determines if an error should be retried
func isRetryableError(err error) bool {
	// Retry on network errors, timeouts, and certain HTTP status codes
	// This is a simplified implementation - in a real system you might want to
	// check for specific error types or status codes

	// Check for context deadline exceeded (timeout)
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Check for context canceled
	if errors.Is(err, context.Canceled) {
		return false // Don't retry if the context was explicitly canceled
	}

	// Check for network errors (simplified)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	// For HTTP errors, we might want to retry on 429 (Too Many Requests) or 5xx errors
	// This would require parsing the error string, which is not ideal but works for this example
	errStr := err.Error()
	if contains(errStr, "429") || contains(errStr, "status 5") {
		return true
	}

	return false
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
