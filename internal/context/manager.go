package context

import (
	"fmt"
	"sync"
	"time"

	"github.com/justmiles/openwebui-discord/internal/logger"
	"go.uber.org/zap"
)

// Message represents a single message in a conversation
type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Name      string    `json:"name"`
	Timestamp time.Time `json:"timestamp"`
}

// ChannelContext represents the conversation context for a specific channel
type ChannelContext struct {
	ChannelID  string    `json:"channel_id"`
	Messages   []Message `json:"messages"`
	LastActive time.Time `json:"last_active"`
}

// Manager handles conversation contexts for multiple channels
type Manager struct {
	contexts      map[string]*ChannelContext
	maxAgeMinutes int
	mutex         sync.RWMutex
}

// NewManager creates a new context manager
func NewManager(maxAgeMinutes int) *Manager {
	manager := &Manager{
		contexts:      make(map[string]*ChannelContext),
		maxAgeMinutes: maxAgeMinutes,
	}

	// Start a goroutine to periodically clean up old contexts
	go manager.cleanupLoop()

	return manager
}

// AddMessage adds a message to a channel's context
func (m *Manager) AddMessage(channelID, role, content, username string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get or create channel context
	ctx, exists := m.contexts[channelID]
	if !exists {
		ctx = &ChannelContext{
			ChannelID: channelID,
			Messages:  make([]Message, 0),
		}
		m.contexts[channelID] = ctx
	}

	// Add message
	message := Message{
		Role:      role,
		Content:   content,
		Name:      username,
		Timestamp: time.Now(),
	}
	ctx.Messages = append(ctx.Messages, message)
	ctx.LastActive = time.Now()

	// Prune old messages
	m.pruneChannelContext(ctx)

	logger.Debug("Added message to context",
		zap.String("channel_id", channelID),
		zap.String("role", role),
		zap.Int("context_size", len(ctx.Messages)),
	)
}

// GetMessages returns all messages for a channel within the time window
func (m *Manager) GetMessages(channelID string) []Message {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	ctx, exists := m.contexts[channelID]
	if !exists {
		return []Message{}
	}

	// Return a copy of the messages to prevent modification
	messages := make([]Message, len(ctx.Messages))
	copy(messages, ctx.Messages)

	return messages
}

// ClearChannel clears the context for a specific channel
func (m *Manager) ClearChannel(channelID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.contexts, channelID)
	logger.Debug("Cleared channel context", zap.String("channel_id", channelID))
}

// pruneChannelContext removes messages older than the max age
func (m *Manager) pruneChannelContext(ctx *ChannelContext) {
	if len(ctx.Messages) == 0 {
		return
	}

	cutoffTime := time.Now().Add(-time.Duration(m.maxAgeMinutes) * time.Minute)
	firstValidIndex := 0

	// Find the first message that's within the time window
	for i, msg := range ctx.Messages {
		if msg.Timestamp.After(cutoffTime) {
			firstValidIndex = i
			break
		}
	}

	// If all messages are too old, clear them
	if firstValidIndex >= len(ctx.Messages) {
		ctx.Messages = []Message{}
		return
	}

	// If some messages are too old, remove them
	if firstValidIndex > 0 {
		ctx.Messages = ctx.Messages[firstValidIndex:]
		logger.Debug("Pruned old messages from context",
			zap.String("channel_id", ctx.ChannelID),
			zap.Int("removed", firstValidIndex),
			zap.Int("remaining", len(ctx.Messages)),
		)
	}
}

// cleanupLoop periodically cleans up inactive contexts
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupInactiveContexts()
	}
}

// cleanupInactiveContexts removes contexts that have been inactive for too long
func (m *Manager) cleanupInactiveContexts() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	cutoffTime := time.Now().Add(-time.Duration(m.maxAgeMinutes*2) * time.Minute)
	var removedCount int

	for channelID, ctx := range m.contexts {
		if ctx.LastActive.Before(cutoffTime) {
			delete(m.contexts, channelID)
			removedCount++
		} else {
			// Also prune old messages from active contexts
			m.pruneChannelContext(ctx)
		}
	}

	if removedCount > 0 {
		logger.Debug("Cleaned up inactive contexts", zap.Int("removed", removedCount))
	}
}

// GetContextSize returns the number of messages in a channel's context
func (m *Manager) GetContextSize(channelID string) int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	ctx, exists := m.contexts[channelID]
	if !exists {
		return 0
	}

	return len(ctx.Messages)
}

// FormatForAPI formats the context messages for the OpenWebUI API
func (m *Manager) FormatForAPI(channelID string) []map[string]string {
	messages := m.GetMessages(channelID)
	formatted := make([]map[string]string, len(messages))

	for i, msg := range messages {
		content := msg.Content
		// Include username for user messages if available
		if msg.Role == "user" && msg.Name != "" {
			content = fmt.Sprintf("%s: %s", msg.Name, content)
		}

		formatted[i] = map[string]string{
			"role":    msg.Role,
			"name":    msg.Name,
			"content": content,
		}
	}

	return formatted
}
