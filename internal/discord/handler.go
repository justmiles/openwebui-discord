package discord

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	contextmgr "github.com/justmiles/openwebui-discord/internal/context"
	"github.com/justmiles/openwebui-discord/internal/logger"
	"github.com/justmiles/openwebui-discord/internal/openwebui"
	"go.uber.org/zap"
)

// OpenWebUIHandler handles Discord messages and processes them with OpenWebUI
type OpenWebUIHandler struct {
	discordClient  *Client
	openwebui      *openwebui.Client
	contextManager *contextmgr.Manager
	systemPrompt   string
}

// NewOpenWebUIHandler creates a new OpenWebUI message handler
func NewOpenWebUIHandler(
	discordClient *Client,
	openwebuiClient *openwebui.Client,
	contextManager *contextmgr.Manager,
	systemPrompt string,
) *OpenWebUIHandler {
	return &OpenWebUIHandler{
		discordClient:  discordClient,
		openwebui:      openwebuiClient,
		contextManager: contextManager,
		systemPrompt:   systemPrompt,
	}
}

// HandleMessage processes a Discord message with OpenWebUI
func (h *OpenWebUIHandler) HandleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Clean up the message content (remove mentions, etc.)
	content := cleanMessage(s, m.Content)

	// Skip empty messages
	if strings.TrimSpace(content) == "" {
		return
	}

	// Set typing indicator
	if err := h.discordClient.SetTyping(m.ChannelID); err != nil {
		logger.Warn("Failed to set typing indicator", zap.Error(err))
	}

	// Log the incoming message
	logger.Info("Received Discord message",
		zap.String("user", m.Author.Username),
		zap.String("channel_id", m.ChannelID),
		zap.Int("content_length", len(content)),
	)

	// Add user message to context with username
	h.contextManager.AddMessage(m.ChannelID, "user", content, m.Author.Username)

	// Prepare messages for OpenWebUI
	messages := h.prepareMessages(m.ChannelID)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get completion from OpenWebUI with retries
	response, err := h.openwebui.WithRetry(ctx, messages, 3)
	if err != nil {
		logger.Error("Failed to get completion from OpenWebUI",
			zap.Error(err),
			zap.String("channel_id", m.ChannelID),
		)
		h.discordClient.SendMessage(m.ChannelID, "Sorry, I encountered an error while processing your message. Please try again later.")
		return
	}

	// Parse actions from the response
	actions, cleanResponse := ParseActions(response)

	// Execute actions using the original message ID (m.ID)
	ExecuteActions(s, m.ChannelID, m.ID, actions)

	// Add assistant response to context (using the cleaned response)
	h.contextManager.AddMessage(m.ChannelID, "assistant", cleanResponse, "")

	// Send response to Discord
	_, err = h.discordClient.SendMessage(m.ChannelID, cleanResponse)
	if err != nil {
		logger.Error("Failed to send response to Discord",
			zap.Error(err),
			zap.String("channel_id", m.ChannelID),
		)
	}

	logger.Info("Sent response to Discord",
		zap.String("channel_id", m.ChannelID),
		zap.Int("response_length", len(cleanResponse)),
		zap.Int("context_size", h.contextManager.GetContextSize(m.ChannelID)),
	)
}

// prepareMessages prepares the messages for the OpenWebUI API
func (h *OpenWebUIHandler) prepareMessages(channelID string) []openwebui.Message {
	// Get messages from context
	contextMessages := h.contextManager.GetMessages(channelID)

	// Create messages array with system prompt
	messages := []openwebui.Message{
		{
			Role:    "system",
			Content: h.systemPrompt,
			// Content: h.systemPrompt,
		},
	}

	// Add context messages
	for _, msg := range contextMessages {
		messages = append(messages, openwebui.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return messages
}

// cleanMessage removes bot mentions and cleans up the message content
func cleanMessage(s *discordgo.Session, content string) string {
	// Remove mentions of the bot
	botID := s.State.User.ID
	content = strings.ReplaceAll(content, fmt.Sprintf("<@%s>", botID), "")
	content = strings.ReplaceAll(content, fmt.Sprintf("<@!%s>", botID), "")

	// Trim whitespace
	content = strings.TrimSpace(content)

	return content
}
