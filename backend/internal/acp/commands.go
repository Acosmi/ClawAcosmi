package acp

// GetAvailableCommands 返回可用的 ACP 命令列表。
// 对应 TS: acp/commands.ts getAvailableCommands()
func GetAvailableCommands() []AvailableCommand {
	return []AvailableCommand{
		{Name: "/help", Description: "Show help and available commands"},
		{Name: "/commands", Description: "List all available commands"},
		{Name: "/status", Description: "Show current agent status"},
		{Name: "/context", Description: "Show context and token usage"},
		{Name: "/whoami", Description: "Show current user and session info"},
		{Name: "/subagents", Description: "List spawned subagents"},
		{Name: "/config", Description: "Show current configuration"},
		{Name: "/debug", Description: "Toggle debug mode"},
		{Name: "/usage", Description: "Show token usage summary"},
		{Name: "/stop", Description: "Stop the current agent run"},
		{Name: "/restart", Description: "Restart the agent"},
		{Name: "/dock-on", Description: "Enable dock mode"},
		{Name: "/dock-off", Description: "Disable dock mode"},
		{Name: "/dock-status", Description: "Show dock mode status"},
		{Name: "/activation", Description: "Show activation trigger status"},
		{Name: "/send", Description: "Send a message to a specific channel", Input: &CommandInput{Hint: "<channel> <message>"}},
		{Name: "/reset", Description: "Reset the current session"},
		{Name: "/new", Description: "Start a new session"},
		{Name: "/think", Description: "Set thinking level", Input: &CommandInput{Hint: "<off|low|medium|high>"}},
		{Name: "/verbose", Description: "Set verbose level", Input: &CommandInput{Hint: "<off|low|medium|high>"}},
		{Name: "/reasoning", Description: "Set reasoning level", Input: &CommandInput{Hint: "<off|low|medium|high>"}},
		{Name: "/elevated", Description: "Set elevated level", Input: &CommandInput{Hint: "<off|on>"}},
		{Name: "/model", Description: "Switch model", Input: &CommandInput{Hint: "<model-name>"}},
		{Name: "/queue", Description: "Queue a message for later delivery", Input: &CommandInput{Hint: "<message>"}},
		{Name: "/bash", Description: "Execute a bash command", Input: &CommandInput{Hint: "<command>"}},
		{Name: "/compact", Description: "Compact session history"},
	}
}
