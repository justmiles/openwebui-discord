package discord

import (
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/justmiles/openwebui-discord/internal/logger"
	"go.uber.org/zap"
)

// ActionType represents the type of action to perform
type ActionType string

const (
	ActionStatus ActionType = "status"
	ActionReact  ActionType = "react"
	// Add more action types as needed
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

			// Add more action types as needed
		default:
			logger.Warn("Unknown action type received", zap.String("type", string(action.Type)))
		}
	}
}
