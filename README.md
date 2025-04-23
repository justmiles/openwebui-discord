# OpenWebUI Discord Bot

A production-grade Golang daemon that bridges Discord and OpenWebUI, allowing users to interact with OpenWebUI through Discord.

## Features

- Bidirectional communication between Discord and OpenWebUI
- Conversation context management (limited to ~20 minutes)
- Robust authentication and authorization
- Structured logging with configurable verbosity levels
- Graceful shutdown handling with proper resource cleanup
- Context propagation throughout the application lifecycle
- Proper error handling with appropriate recovery mechanisms
- Concurrent request processing with synchronization primitives
- Rate limiting for both Discord and OpenWebUI APIs
- Automatic reconnection logic with exponential backoff
- Comprehensive configuration system supporting environment variables, config files, and CLI flags
- Memory-efficient conversation history management
- Secure credential handling and storage

## Requirements

- Go 1.20+
- Discord Bot Token
- OpenWebUI API access

## Installation

### From Source

1. Clone the repository:
   ```
   git clone https://github.com/justmiles/openwebui-discord.git
   cd openwebui-discord
   ```

2. Build the application:
   ```
   go build -o openwebui-discord ./cmd/openwebui-discord
   ```

3. Generate an example configuration:
   ```
   ./openwebui-discord --generate-config
   ```

4. Edit the configuration file:
   ```
   cp configs/config.yaml.example configs/config.yaml
   # Edit configs/config.yaml with your preferred editor
   ```

5. Run the application:
   ```
   ./openwebui-discord
   ```

## Configuration

The application can be configured using a YAML configuration file, environment variables, or command-line flags. The priority order is:

1. Command-line flags
2. Environment variables
3. Configuration file

### Configuration File

See [configs/config.yaml.example](configs/config.yaml.example) for a complete example with comments.

### Environment Variables

Environment variables are prefixed with `OPENWEBUI_DISCORD_` and use underscores instead of dots. For example:

```
OPENWEBUI_DISCORD_DISCORD_TOKEN=your-discord-token
OPENWEBUI_DISCORD_OPENWEBUI_ENDPOINT=http://localhost:8080
OPENWEBUI_DISCORD_OPENWEBUI_API_KEY=your-openwebui-api-key
```

### Command-Line Flags

Run `./openwebui-discord --help` to see all available command-line flags.

## Discord Bot Setup

1. Create a new Discord application at https://discord.com/developers/applications
2. Add a bot to your application
3. Enable the necessary intents (at minimum, you need the Message Content intent)
4. Copy the bot token and add it to your configuration
5. Invite the bot to your server using the OAuth2 URL generator with the following scopes:
   - bot
   - applications.commands
   
   And the following permissions:
   - Read Messages/View Channels
   - Send Messages
   - Read Message History

## Usage

Users can interact with the bot in two ways:

1. Mentioning the bot: `@BotName How does photosynthesis work?`
2. Using the command prefix: `!How does photosynthesis work?`

The bot will process the message through OpenWebUI and respond with the generated text.

## Architecture

The application follows a modular architecture with clear separation of concerns:

- `cmd/openwebui-discord`: Main application entry point
- `internal/config`: Configuration handling
- `internal/discord`: Discord client and message handling
- `internal/openwebui`: OpenWebUI API client
- `internal/context`: Conversation context management
- `internal/ratelimit`: Rate limiting implementation
- `internal/logger`: Structured logging
- `pkg/utils`: Utility functions for error handling and graceful shutdown

For more details, see [openwebui-discord-architecture.md](openwebui-discord-architecture.md).

## License

MIT