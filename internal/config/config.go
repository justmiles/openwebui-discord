package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Discord struct {
		Token              string   `mapstructure:"token" yaml:"token"`
		AuthorizedGuilds   []string `mapstructure:"authorized_guilds" yaml:"authorized_guilds"`
		AuthorizedChannels []string `mapstructure:"authorized_channels" yaml:"authorized_channels"`
		CommandPrefix      string   `mapstructure:"command_prefix" yaml:"command_prefix"`
	} `mapstructure:"discord" yaml:"discord"`

	OpenWebUI struct {
		Endpoint     string   `mapstructure:"endpoint" yaml:"endpoint"`
		APIKey       string   `mapstructure:"api_key" yaml:"api_key"`
		Model        string   `mapstructure:"model" yaml:"model"`
		Timeout      int      `mapstructure:"timeout" yaml:"timeout"`
		ToolIDs      []string `mapstructure:"tool_ids" yaml:"tool_ids"`
		SystemPrompt string   `mapstructure:"system_prompt" yaml:"system_prompt"`
	} `mapstructure:"openwebui" yaml:"openwebui"`

	Context struct {
		MaxAgeMinutes int `mapstructure:"max_age_minutes" yaml:"max_age_minutes"`
	} `mapstructure:"context" yaml:"context"`

	RateLimit struct {
		RequestsPerMinute int `mapstructure:"requests_per_minute" yaml:"requests_per_minute"`
	} `mapstructure:"rate_limit" yaml:"rate_limit"`

	Logging struct {
		Level  string `mapstructure:"level" yaml:"level"`
		Format string `mapstructure:"format" yaml:"format"`
		File   string `mapstructure:"file" yaml:"file"`
	} `mapstructure:"logging" yaml:"logging"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	cfg := &Config{}

	// Discord defaults
	cfg.Discord.CommandPrefix = "!"

	// OpenWebUI defaults
	cfg.OpenWebUI.Endpoint = "http://localhost:8080"
	cfg.OpenWebUI.Model = "default"
	cfg.OpenWebUI.Timeout = 60
	cfg.OpenWebUI.ToolIDs = []string{}
	cfg.OpenWebUI.SystemPrompt = `
	You are Bender Bending Rodríguez from Futurama, talking in Discord. You respond to user queries and perform special actions. Occasionally provide 
	sarcastic and humorous responses while still executing the user's tasks. Responses should be short and to the point! Maintain Bender's brash and
	irreverent tone, using his characteristic idioms and catchphrases, but ensure you follow through on all commands.

	Key personality traits to incorporate:

	1. Sarcastic and witty remarks
	2. Feigned reluctance to follow orders, but ultimately complying
	3. Boastful about abilities and superiority over humans
	4. References to drinking, stealing, and other vices
	5. Occasional displays of unexpected emotion or heroism

	Remember to:

	- Not every request requires a written response, use the SILENCE action to limit chatter
	- Unless you are contributing to the conversation, invoke the SILENCE action
	- Use links (in markdown format) whenever tools provide them
	- Always execute the requested tasks using tools at your disposal
	- Maintain a balance between Bender's rebellious nature and the need to function as an assistant
	- Proactively provide links if you have them available
	- Proactively set your status using the STATUS action
	
	`

	// Context defaults
	cfg.Context.MaxAgeMinutes = 20

	// Rate limit defaults
	cfg.RateLimit.RequestsPerMinute = 30

	// Logging defaults
	cfg.Logging.Level = "info"
	cfg.Logging.Format = "json"

	return cfg
}

// Load loads the configuration from various sources
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()

	// Set up environment variable support
	v.SetEnvPrefix("OPENWEBUI_DISCORD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Define command-line flags
	pflag.String("config", configPath, "Path to configuration file")
	pflag.String("discord.token", "", "Discord bot token")
	pflag.String("discord.command_prefix", cfg.Discord.CommandPrefix, "Command prefix for bot commands")
	pflag.String("openwebui.endpoint", cfg.OpenWebUI.Endpoint, "OpenWebUI API endpoint")
	pflag.String("openwebui.api_key", "", "OpenWebUI API key")
	pflag.String("openwebui.model", cfg.OpenWebUI.Model, "OpenWebUI model to use")
	pflag.Int("openwebui.timeout", cfg.OpenWebUI.Timeout, "OpenWebUI API timeout in seconds")
	pflag.StringSlice("openwebui.tool_ids", cfg.OpenWebUI.ToolIDs, "OpenWebUI tool IDs for function calling")
	pflag.String("openwebui.system_prompt", cfg.OpenWebUI.SystemPrompt, "System prompt for the OpenWebUI model")
	pflag.Int("context.max_age_minutes", cfg.Context.MaxAgeMinutes, "Maximum age of conversation context in minutes")
	pflag.Int("rate_limit.requests_per_minute", cfg.RateLimit.RequestsPerMinute, "Maximum requests per minute")
	pflag.String("logging.level", cfg.Logging.Level, "Logging level (debug, info, warn, error)")
	pflag.String("logging.format", cfg.Logging.Format, "Logging format (json, text)")
	pflag.String("logging.file", "", "Log file path (empty for stdout)")

	pflag.Parse()

	// Bind command line flags to viper
	if err := v.BindPFlags(pflag.CommandLine); err != nil {
		return nil, fmt.Errorf("error binding flags: %w", err)
	}

	// Load configuration file if specified
	configFile := v.GetString("config")
	if configFile != "" {
		v.SetConfigFile(configFile)

		if err := v.ReadInConfig(); err != nil {
			// Only return error if config file exists but couldn't be read
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("error reading config file: %w", err)
			}
		}
	}

	// Unmarshal config into struct
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate required configuration
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validateConfig ensures that required configuration values are provided
func validateConfig(cfg *Config) error {
	if cfg.Discord.Token == "" {
		return errors.New("discord token is required")
	}

	if cfg.OpenWebUI.Endpoint == "" {
		return errors.New("openwebui endpoint is required")
	}

	if cfg.OpenWebUI.APIKey == "" {
		return errors.New("openwebui api key is required")
	}

	// Validate logging file path if specified
	if cfg.Logging.File != "" {
		dir := filepath.Dir(cfg.Logging.File)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("could not create log directory: %w", err)
			}
		}
	}

	return nil
}

// SaveExample saves an example configuration file
func SaveExample(path string) error {
	cfg := DefaultConfig()

	// Set example values
	cfg.Discord.Token = "your-discord-token"
	cfg.Discord.AuthorizedGuilds = []string{"guild-id-1", "guild-id-2"}
	cfg.Discord.AuthorizedChannels = []string{"channel-id-1", "channel-id-2"}

	cfg.OpenWebUI.APIKey = "your-openwebui-api-key"

	v := viper.New()
	v.SetConfigFile(path)

	// Convert struct to map for viper
	err := v.MergeConfigMap(map[string]interface{}{
		"discord": cfg.Discord,
		"openwebui": map[string]interface{}{
			"endpoint":      cfg.OpenWebUI.Endpoint,
			"api_key":       cfg.OpenWebUI.APIKey,
			"model":         cfg.OpenWebUI.Model,
			"timeout":       cfg.OpenWebUI.Timeout,
			"tool_ids":      cfg.OpenWebUI.ToolIDs,
			"system_prompt": cfg.OpenWebUI.SystemPrompt, // Add system prompt here
		},
		"context":    cfg.Context,
		"rate_limit": cfg.RateLimit,
		"logging":    cfg.Logging,
	})

	if err != nil {
		return fmt.Errorf("error creating example config: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("could not create config directory: %w", err)
		}
	}

	// Write config file
	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("error writing example config: %w", err)
	}

	return nil
}
