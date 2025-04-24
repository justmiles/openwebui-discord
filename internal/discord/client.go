package discord

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/justmiles/openwebui-discord/internal/logger"
	"github.com/justmiles/openwebui-discord/internal/ratelimit"
	"go.uber.org/zap"
)

// Client represents a Discord client
type Client struct {
	session            *discordgo.Session
	token              string
	commandPrefix      string
	authorizedGuilds   []string
	authorizedChannels []string
	rateLimiter        *ratelimit.Limiter
	handlers           []Handler
	handlersMutex      sync.RWMutex
}

// Handler is an interface for message handlers
type Handler interface {
	HandleMessage(s *discordgo.Session, m *discordgo.MessageCreate)
}

// NewClient creates a new Discord client
func NewClient(token, commandPrefix string, authorizedGuilds, authorizedChannels []string, requestsPerMinute int) (*Client, error) {
	// Create Discord session
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("error creating Discord session: %w", err)
	}

	client := &Client{
		session:            session,
		token:              token,
		commandPrefix:      commandPrefix,
		authorizedGuilds:   authorizedGuilds,
		authorizedChannels: authorizedChannels,
		rateLimiter:        ratelimit.NewLimiter(requestsPerMinute),
		handlers:           make([]Handler, 0),
	}

	// Add message handler
	session.AddHandler(client.messageHandler)

	return client, nil
}

// Start connects to Discord and starts listening for events
func (c *Client) Start(ctx context.Context) error {
	// Open connection to Discord
	if err := c.session.Open(); err != nil {
		return fmt.Errorf("error opening Discord connection: %w", err)
	}

	logger.Info("Connected to Discord",
		zap.String("username", c.session.State.User.Username),
		zap.String("discriminator", c.session.State.User.Discriminator),
		zap.String("id", c.session.State.User.ID),
	)

	// Set status
	err := c.session.UpdateCustomStatus("Chatting with OpenWebUI")
	if err != nil {
		logger.Warn("Failed to update status", zap.Error(err))
	}

	// Wait for context to be done
	<-ctx.Done()

	// Close connection when context is done
	return c.session.Close()
}

// AddHandler adds a message handler
func (c *Client) AddHandler(handler Handler) {
	c.handlersMutex.Lock()
	defer c.handlersMutex.Unlock()
	c.handlers = append(c.handlers, handler)
}

// messageHandler handles incoming Discord messages
func (c *Client) messageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check if the message is from an authorized guild/channel
	if !c.isAuthorized(m.GuildID, m.ChannelID) {
		return
	}

	// Check if the message mentions the bot or starts with the command prefix
	isMention := false
	for _, mention := range m.Mentions {
		if mention.ID == s.State.User.ID {
			isMention = true
			break
		}
	}

	isCommand := strings.HasPrefix(m.Content, c.commandPrefix)

	// Always process the message, but log if it's not a direct mention or command
	if !isMention && !isCommand {
		logger.Debug("Processing message without direct mention or command",
			zap.String("channel_id", m.ChannelID),
			zap.String("user_id", m.Author.ID),
		)
	}

	// Apply rate limiting
	if !c.rateLimiter.Allow() {
		logger.Warn("Rate limit exceeded for Discord message",
			zap.String("channel_id", m.ChannelID),
			zap.String("user_id", m.Author.ID),
		)
		c.sendMessage(m.ChannelID, "I'm receiving too many messages right now. Please try again later.")
		return
	}

	// Process message with all registered handlers
	c.handlersMutex.RLock()
	handlers := c.handlers
	c.handlersMutex.RUnlock()

	for _, handler := range handlers {
		handler.HandleMessage(s, m)
	}
}

// isAuthorized checks if a message is from an authorized guild/channel
func (c *Client) isAuthorized(guildID, channelID string) bool {
	// If no authorized guilds/channels are specified, allow all
	if len(c.authorizedGuilds) == 0 && len(c.authorizedChannels) == 0 {
		return true
	}

	// Check if the guild is authorized
	for _, authorizedGuild := range c.authorizedGuilds {
		if guildID == authorizedGuild {
			return true
		}
	}

	// Check if the channel is authorized
	for _, authorizedChannel := range c.authorizedChannels {
		if channelID == authorizedChannel {
			return true
		}
	}

	return false
}

// SendMessage sends a message to a Discord channel
func (c *Client) SendMessage(channelID, content string) (string, error) {
	// Apply rate limiting
	c.rateLimiter.Wait()

	return c.sendMessage(channelID, content)
}

// sendMessage sends a message to a Discord channel without rate limiting
func (c *Client) sendMessage(channelID, content string) (string, error) {
	// Split message if it's too long
	if len(content) > 2000 {
		messages := splitMessage(content, 1900)
		var lastMessageID string
		var err error

		for i, msg := range messages {
			if i == len(messages)-1 {
				lastMessageID, err = c.sendMessage(channelID, msg)
			} else {
				_, err = c.sendMessage(channelID, msg)
			}

			if err != nil {
				return "", err
			}
		}

		return lastMessageID, nil
	}

	// Send message
	msg, err := c.session.ChannelMessageSend(channelID, content)
	if err != nil {
		logger.Error("Failed to send Discord message",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return "", fmt.Errorf("error sending message: %w", err)
	}

	return msg.ID, nil
}

// SetTyping sets the typing indicator in a Discord channel
func (c *Client) SetTyping(channelID string) error {
	return c.session.ChannelTyping(channelID)
}

// splitMessage splits a message into multiple parts if it's too long
func splitMessage(message string, maxLength int) []string {
	if len(message) <= maxLength {
		return []string{message}
	}

	var parts []string
	for len(message) > 0 {
		// Find a good place to split (preferably at a newline or space)
		splitIndex := maxLength
		if splitIndex > len(message) {
			splitIndex = len(message)
		}

		// Try to find a newline to split at
		newlineIndex := strings.LastIndex(message[:splitIndex], "\n")
		if newlineIndex > maxLength/2 {
			splitIndex = newlineIndex + 1 // Include the newline
		} else {
			// Try to find a space to split at
			spaceIndex := strings.LastIndex(message[:splitIndex], " ")
			if spaceIndex > maxLength/2 {
				splitIndex = spaceIndex + 1 // Include the space
			}
		}

		// Add the part
		parts = append(parts, message[:splitIndex])
		message = message[splitIndex:]
	}

	return parts
}

// GetSession returns the underlying Discord session
func (c *Client) GetSession() *discordgo.Session {
	return c.session
}

// GetCommandPrefix returns the command prefix
func (c *Client) GetCommandPrefix() string {
	return c.commandPrefix
}
