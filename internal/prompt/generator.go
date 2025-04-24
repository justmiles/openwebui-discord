package prompt

import (
	"fmt"
	"strings"
)

// ActionType represents the type of action to perform
type ActionType string

const (
	ActionStatus    ActionType = "status"
	ActionReact     ActionType = "react"
	ActionFormat    ActionType = "format"
	ActionReactions ActionType = "reactions"
	ActionDelete    ActionType = "delete"
	ActionPin       ActionType = "pin"
	ActionFile      ActionType = "file"
)

// ActionDescription contains detailed information about an action
type ActionDescription struct {
	Type          ActionType
	Description   string
	Parameters    string
	Examples      []string
	Limitations   string
	BestPractices string
}

// GetActionDescriptions returns detailed descriptions for all available actions
func GetActionDescriptions() []ActionDescription {
	return []ActionDescription{
		{
			Type:        ActionStatus,
			Description: "Changes the bot's status message displayed in Discord.",
			Parameters:  "A single string representing the status text to display.",
			Examples: []string{
				"[ACTION:status|Playing chess]",
				"[ACTION:status|Listening to music]",
				"[ACTION:status|Watching tutorials]",
			},
			Limitations:   "Status changes may not be immediately visible to all users due to Discord's caching.",
			BestPractices: "Keep status messages concise and relevant to the current conversation or bot's purpose.",
		},
		{
			Type:        ActionReact,
			Description: "Adds a single emoji reaction to the user's message.",
			Parameters:  "A single emoji (Unicode emoji or Discord custom emoji ID).",
			Examples: []string{
				"[ACTION:react|👍]",
				"[ACTION:react|❤️]",
				"[ACTION:react|🎉]",
			},
			Limitations:   "Some custom emojis may only work if the bot has access to the server they're from.",
			BestPractices: "Use reactions to acknowledge user messages or provide quick feedback without sending a text response.",
		},
		{
			Type:        ActionFormat,
			Description: "Applies special formatting to the bot's message.",
			Parameters:  "Format type followed by content, separated by '|'. Format types include: code, bold, italic, quote.",
			Examples: []string{
				"[ACTION:format|code:python|print(\"Hello World\")]",
				"[ACTION:format|bold|Important information]",
				"[ACTION:format|italic|Emphasized text]",
				"[ACTION:format|quote|This is a quote]",
			},
			Limitations:   "Formatting may not be combined (e.g., can't have bold and italic together).",
			BestPractices: "Use code formatting when sharing code snippets to improve readability.",
		},
		{
			Type:        ActionReactions,
			Description: "Adds multiple emoji reactions in sequence to the user's message.",
			Parameters:  "Multiple emojis separated by '|'.",
			Examples: []string{
				"[ACTION:reactions|👍|❤️|🎉]",
				"[ACTION:reactions|1️⃣|2️⃣|3️⃣]",
			},
			Limitations:   "Limited to a reasonable number of reactions to avoid rate limiting.",
			BestPractices: "Use sequential reactions for creating simple polls or showing a sequence of emotions.",
		},
	}
}

// GenerateSystemPrompt creates a comprehensive system prompt with action descriptions
func GenerateSystemPrompt(basePrompt string) string {
	var sb strings.Builder

	// Add base prompt
	if basePrompt != "" {
		sb.WriteString(basePrompt)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("You are a helpful Discord bot assistant. You can respond to user queries and perform special actions.\n\n")
	}

	// Add action format description
	sb.WriteString("# SPECIAL ACTIONS\n\n")
	sb.WriteString("You can perform special actions by including action markup in your responses using this format:\n")
	sb.WriteString("```\n[ACTION:action_type|action_parameters]\n```\n\n")
	sb.WriteString("Always include a normal text response along with any actions to explain what you're doing.\n\n")

	// Add detailed action descriptions
	sb.WriteString("## Available Actions\n\n")

	for _, action := range GetActionDescriptions() {
		// Action header
		sb.WriteString(fmt.Sprintf("### %s\n", strings.ToUpper(string(action.Type))))

		// Description
		sb.WriteString(fmt.Sprintf("**Description:** %s\n\n", action.Description))

		// Parameters
		sb.WriteString(fmt.Sprintf("**Parameters:** %s\n\n", action.Parameters))

		// Examples
		sb.WriteString("**Examples:**\n")
		for _, example := range action.Examples {
			sb.WriteString(fmt.Sprintf("```\n%s\n```\n", example))
		}

		// Limitations
		sb.WriteString(fmt.Sprintf("**Limitations:** %s\n\n", action.Limitations))

		// Best practices
		sb.WriteString(fmt.Sprintf("**Best Practices:** %s\n\n", action.BestPractices))
	}

	// Add general usage guidelines
	sb.WriteString("## General Guidelines\n\n")
	sb.WriteString("1. **Combine actions with text responses** - Always include a normal text response explaining what actions you're performing.\n")
	sb.WriteString("2. **Rate limits** - Use actions judiciously to avoid hitting Discord's rate limits.\n")
	sb.WriteString("3. **Error handling** - If an action fails, the bot will log the error but continue processing other actions.\n")
	sb.WriteString("4. **Permissions** - Some actions require specific permissions in the Discord server.\n\n")

	// Add example of combined usage
	sb.WriteString("## Combined Usage Example\n\n")
	sb.WriteString("```\n[ACTION:status|Helping with code]\n[ACTION:react|💻]\nHere's the Python code you requested:\n\n[ACTION:format|code:python|def hello_world():\n    print(\"Hello, World!\")]\n\n[ACTION:pin|Important code example]\n```\n\n")
	sb.WriteString("This example changes the bot's status, adds a reaction, formats code, and pins the message.\n")

	return sb.String()
}
