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

	// Check if this is a direct mention or command
	isMention := false
	for _, mention := range m.Mentions {
		if mention.ID == s.State.User.ID {
			isMention = true
			break
		}
	}
	isCommand := strings.HasPrefix(m.Content, h.discordClient.GetCommandPrefix())

	// Check if the bot was recently mentioned or commanded (within ~20 minutes)
	wasRecentlyActive := h.contextManager.WasRecentlyMentionedOrCommanded(m.ChannelID, 20)

	// Send the response if it's a direct mention/command, was recently active, or if the response seems appropriate
	if !isMention && !isCommand && !wasRecentlyActive {
		return
	}

	// Set typing indicator
	if isMention || isCommand {
		if err := h.discordClient.SetTyping(m.ChannelID); err != nil {
			logger.Warn("Failed to set typing indicator", zap.Error(err))
		}
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

	// Check for format action
	var formattedResponse string = cleanResponse
	var shouldPin bool = false

	// set to an empty response if the Silence action is in use.
	hasSilenceAction := false
	for _, action := range actions {
		if action.Type == ActionSilence {
			hasSilenceAction = true
			break
		}
	}

	if hasSilenceAction {
		formattedResponse = ""
	}

	for _, action := range actions {
		if action.Type == ActionFormat {
			// Parse format action: format|type:language|content
			parts := strings.SplitN(action.Parameters, "|", 2)
			if len(parts) >= 2 {
				formatType := parts[0]
				formatContent := parts[1]

				switch formatType {
				case "code":
					// Format as code block
					langParts := strings.SplitN(formatContent, "|", 2)
					if len(langParts) >= 2 {
						language := langParts[0]
						code := langParts[1]
						formattedResponse = "```" + language + "\n" + code + "\n```"
					}
				case "bold":
					formattedResponse = "**" + formatContent + "**"
				case "italic":
					formattedResponse = "*" + formatContent + "*"
				case "quote":
					lines := strings.Split(formatContent, "\n")
					var quotedLines []string
					for _, line := range lines {
						quotedLines = append(quotedLines, "> "+line)
					}
					formattedResponse = strings.Join(quotedLines, "\n")
				}

				logger.Debug("Applied formatting", zap.String("type", formatType))
			}
		} else if action.Type == ActionPin {
			shouldPin = true
		}
	}

	// Only send a response if there's actual content to send
	var sentMsg string
	if strings.TrimSpace(formattedResponse) != "" {
		// Send the response if it's a direct mention/command, was recently active, or if the response seems appropriate
		sentMsg, err = h.discordClient.SendMessage(m.ChannelID, formattedResponse)
		if err != nil {
			logger.Error("Failed to send response to Discord",
				zap.Error(err),
				zap.String("channel_id", m.ChannelID),
			)
		}
	} else {
		// Log that there's no response content
		logger.Info("No response content to send",
			zap.String("channel_id", m.ChannelID),
		)
	}

	// Handle pin action if needed
	if shouldPin && sentMsg != "" {
		err := s.ChannelMessagePin(m.ChannelID, sentMsg)
		if err != nil {
			logger.Warn("Failed to pin message", zap.Error(err), zap.String("message_id", sentMsg))
		} else {
			logger.Info("Pinned message", zap.String("message_id", sentMsg))
		}
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
