package discord

import (
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/justmiles/openwebui-discord/internal/logger"
	"github.com/justmiles/openwebui-discord/internal/prompt"
	"go.uber.org/zap"
)

// For convenience, alias the ActionType from prompt package
type ActionType = prompt.ActionType

// Action constants from prompt package
const (
	ActionStatus    = prompt.ActionStatus
	ActionReact     = prompt.ActionReact
	ActionSilence   = prompt.ActionSilence
	ActionFormat    = prompt.ActionFormat
	ActionReactions = prompt.ActionReactions
	ActionDelete    = prompt.ActionDelete
	ActionPin       = prompt.ActionPin
	ActionFile      = prompt.ActionFile
)

// Action represents a parsed action from the LLM response
type Action struct {
	Type       ActionType
	Parameters string
}

// ParseActions extracts actions from the LLM response
func ParseActions(content string) ([]Action, string) {
	// Regex to match action markup: [ACTION:type|parameters]
	// It captures the type (letters only) and parameters (anything until ']')
	actionRegex := regexp.MustCompile(`\[ACTION:([a-zA-Z]+)\|([^\]]+)\]`)

	// Find all matches
	matches := actionRegex.FindAllStringSubmatch(content, -1)

	// Extract actions
	actions := make([]Action, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 3 {
			actionType := ActionType(strings.ToLower(match[1])) // Normalize type to lowercase
			parameters := match[2]
			actions = append(actions, Action{
				Type:       actionType,
				Parameters: parameters,
			})
			logger.Debug("Parsed action", zap.String("type", string(actionType)), zap.String("params", parameters))
		}
	}

	// Remove action markup from content
	cleanContent := actionRegex.ReplaceAllString(content, "")
	// Clean up any extra whitespace or newlines resulting from removal
	cleanContent = strings.TrimSpace(cleanContent)

	return actions, cleanContent
}

// ExecuteActions performs the specified actions
func ExecuteActions(s *discordgo.Session, channelID string, messageID string, actions []Action) {
	for _, action := range actions {
		logger.Info("Executing action", zap.String("type", string(action.Type)), zap.String("params", action.Parameters))
		switch action.Type {
		case ActionStatus:
			// Update bot status
			err := s.UpdateCustomStatus(action.Parameters)
			if err != nil {
				logger.Warn("Failed to update status via action", zap.Error(err), zap.String("status", action.Parameters))
			}

		case ActionReact:
			// Add reaction to the original user message
			err := s.MessageReactionAdd(channelID, messageID, action.Parameters)
			if err != nil {
				// Log error but don't stop processing other actions
				logger.Warn("Failed to add reaction via action", zap.Error(err), zap.String("emoji", action.Parameters), zap.String("channel_id", channelID), zap.String("message_id", messageID))
			}

		case ActionSilence:
			// This is handled during message sending in handler.go
			logger.Debug("Silence action engaged - LLM decided not to respond to this message", zap.String("params", action.Parameters))

		case ActionFormat:
			// This is handled during message sending in handler.go
			// The format action is parsed and applied to the message content
			logger.Debug("Format action detected", zap.String("params", action.Parameters))
			// No direct action needed here as formatting will be applied when sending the message

		case ActionReactions:
			// Add multiple reactions in sequence
			emojis := strings.Split(action.Parameters, "|")
			for _, emoji := range emojis {
				emoji = strings.TrimSpace(emoji)
				if emoji == "" {
					continue
				}

				err := s.MessageReactionAdd(channelID, messageID, emoji)
				if err != nil {
					logger.Warn("Failed to add reaction in sequence", zap.Error(err), zap.String("emoji", emoji))
				}
				// Small delay between reactions to avoid rate limiting
				time.Sleep(300 * time.Millisecond)
			}

		case ActionDelete:
			// Delete the bot's previous message
			// We need to find the bot's previous message
			if action.Parameters == "previous" {
				messages, err := s.ChannelMessages(channelID, 10, "", "", "")
				if err != nil {
					logger.Warn("Failed to fetch messages for delete action", zap.Error(err))
					break
				}

				// Find the most recent message from the bot
				for _, msg := range messages {
					if msg.Author.ID == s.State.User.ID && msg.ID != messageID {
						err := s.ChannelMessageDelete(channelID, msg.ID)
						if err != nil {
							logger.Warn("Failed to delete previous message", zap.Error(err), zap.String("message_id", msg.ID))
						} else {
							logger.Info("Deleted previous message", zap.String("message_id", msg.ID))
						}
						break
					}
				}
			}

		case ActionPin:
			// Pin the message that will be sent
			// We'll need to handle this after the message is sent
			// Store the action for processing after message is sent
			logger.Debug("Pin action detected, will be processed after message is sent", zap.String("params", action.Parameters))
			// This will be handled in handler.go after sending the message

		case ActionFile:
			// Generate and upload a file
			parts := strings.SplitN(action.Parameters, "|", 2)
			if len(parts) != 2 {
				logger.Warn("Invalid file action format", zap.String("params", action.Parameters))
				break
			}

			filename := strings.TrimSpace(parts[0])
			content := parts[1]

			reader := strings.NewReader(content)
			_, err := s.ChannelFileSend(channelID, filename, reader)
			if err != nil {
				logger.Warn("Failed to upload file", zap.Error(err), zap.String("filename", filename))
			}

		default:
			logger.Warn("Unknown action type received", zap.String("type", string(action.Type)))
		}
	}
}
