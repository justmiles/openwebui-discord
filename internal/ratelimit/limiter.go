package ratelimit

import (
	"sync"
	"time"

	"github.com/justmiles/openwebui-discord/internal/logger"
	"go.uber.org/zap"
)

// Limiter implements a token bucket rate limiter
type Limiter struct {
	tokens         int
	maxTokens      int
	refillRate     int
	refillInterval time.Duration
	lastRefill     time.Time
	mutex          sync.Mutex
}

// NewLimiter creates a new rate limiter
func NewLimiter(requestsPerMinute int) *Limiter {
	// Convert requests per minute to tokens and refill rate
	maxTokens := requestsPerMinute
	refillRate := requestsPerMinute
	refillInterval := time.Minute

	limiter := &Limiter{
		tokens:         maxTokens,
		maxTokens:      maxTokens,
		refillRate:     refillRate,
		refillInterval: refillInterval,
		lastRefill:     time.Now(),
	}

	return limiter
}

// Allow checks if a request is allowed and consumes a token if it is
func (l *Limiter) Allow() bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// Refill tokens based on time elapsed
	l.refill()

	// Check if we have tokens available
	if l.tokens > 0 {
		l.tokens--
		return true
	}

	return false
}

// refill adds tokens based on time elapsed since last refill
func (l *Limiter) refill() {
	now := time.Now()
	elapsed := now.Sub(l.lastRefill)

	// Calculate how many tokens to add based on elapsed time
	intervalsElapsed := float64(elapsed) / float64(l.refillInterval)
	tokensToAdd := int(intervalsElapsed * float64(l.refillRate))

	if tokensToAdd > 0 {
		l.tokens = min(l.maxTokens, l.tokens+tokensToAdd)
		l.lastRefill = now
	}
}

// Wait blocks until a token is available and then consumes it
func (l *Limiter) Wait() {
	for {
		l.mutex.Lock()
		l.refill()

		if l.tokens > 0 {
			l.tokens--
			l.mutex.Unlock()
			return
		}

		// Calculate time until next token is available
		timeToNextToken := l.refillInterval / time.Duration(l.refillRate)
		l.mutex.Unlock()

		logger.Debug("Rate limit reached, waiting",
			zap.Duration("wait_time", timeToNextToken),
		)

		// Wait a bit before trying again
		time.Sleep(timeToNextToken)
	}
}

// RemainingTokens returns the number of tokens currently available
func (l *Limiter) RemainingTokens() int {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.refill()
	return l.tokens
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ChannelLimiter manages rate limits for multiple channels
type ChannelLimiter struct {
	limiters      map[string]*Limiter
	globalLimiter *Limiter
	mutex         sync.RWMutex
}

// NewChannelLimiter creates a new channel-based rate limiter
func NewChannelLimiter(globalRequestsPerMinute, channelRequestsPerMinute int) *ChannelLimiter {
	return &ChannelLimiter{
		limiters:      make(map[string]*Limiter),
		globalLimiter: NewLimiter(globalRequestsPerMinute),
	}
}

// Allow checks if a request for a specific channel is allowed
func (cl *ChannelLimiter) Allow(channelID string) bool {
	// First check global rate limit
	if !cl.globalLimiter.Allow() {
		return false
	}

	// Then check channel-specific rate limit
	cl.mutex.RLock()
	limiter, exists := cl.limiters[channelID]
	cl.mutex.RUnlock()

	if !exists {
		cl.mutex.Lock()
		// Check again in case another goroutine created it while we were waiting for the lock
		limiter, exists = cl.limiters[channelID]
		if !exists {
			limiter = NewLimiter(10) // Default to 10 requests per minute per channel
			cl.limiters[channelID] = limiter
		}
		cl.mutex.Unlock()
	}

	return limiter.Allow()
}

// Wait blocks until a request for a specific channel is allowed
func (cl *ChannelLimiter) Wait(channelID string) {
	// First wait for global rate limit
	cl.globalLimiter.Wait()

	// Then wait for channel-specific rate limit
	cl.mutex.RLock()
	limiter, exists := cl.limiters[channelID]
	cl.mutex.RUnlock()

	if !exists {
		cl.mutex.Lock()
		// Check again in case another goroutine created it while we were waiting for the lock
		limiter, exists = cl.limiters[channelID]
		if !exists {
			limiter = NewLimiter(10) // Default to 10 requests per minute per channel
			cl.limiters[channelID] = limiter
		}
		cl.mutex.Unlock()
	}

	limiter.Wait()
}