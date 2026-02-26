package plugins

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
)

// 命令注册相关类型
// 对应 TS: commands.ts

// RegisteredPluginCommand 已注册的插件命令
type RegisteredPluginCommand struct {
	PluginCommandDefinition
	PluginID string
}

var (
	commandsMu     sync.RWMutex
	pluginCommands = make(map[string]*RegisteredPluginCommand)
	registryLocked bool
)

// MaxArgsLength 命令参数最大长度（防御性限制）
const MaxArgsLength = 4096

// commandNamePattern 命令名校验正则
var commandNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

// ReservedCommands 保留命令名（内置命令）
// 对应 TS: commands.ts RESERVED_COMMANDS
var ReservedCommands = map[string]bool{
	// Core commands
	"help": true, "commands": true, "status": true, "whoami": true, "context": true,
	// Session management
	"stop": true, "restart": true, "reset": true, "new": true, "compact": true,
	// Configuration
	"config": true, "debug": true, "allowlist": true, "activation": true,
	// Agent control
	"skill": true, "subagents": true, "model": true, "models": true, "queue": true,
	// Messaging
	"send": true,
	// Execution
	"bash": true, "exec": true,
	// Mode toggles
	"think": true, "verbose": true, "reasoning": true, "elevated": true,
	// Billing
	"usage": true,
}

// CommandRegistrationResult 命令注册结果
type CommandRegistrationResult struct {
	OK    bool
	Error string
}

// ValidateCommandName 校验命令名
// 对应 TS: commands.ts validateCommandName
func ValidateCommandName(name string) string {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed == "" {
		return "Command name cannot be empty"
	}
	if !commandNamePattern.MatchString(trimmed) {
		return "Command name must start with a letter and contain only letters, numbers, hyphens, and underscores"
	}
	if ReservedCommands[trimmed] {
		return fmt.Sprintf("Command name %q is reserved by a built-in command", trimmed)
	}
	return ""
}

// RegisterPluginCommand 注册插件命令
// 对应 TS: commands.ts registerPluginCommand
func RegisterPluginCommand(pluginID string, command PluginCommandDefinition) CommandRegistrationResult {
	commandsMu.Lock()
	defer commandsMu.Unlock()

	if registryLocked {
		return CommandRegistrationResult{OK: false, Error: "Cannot register commands while processing is in progress"}
	}

	if command.Handler == nil {
		return CommandRegistrationResult{OK: false, Error: "Command handler must be a function"}
	}

	if errMsg := ValidateCommandName(command.Name); errMsg != "" {
		return CommandRegistrationResult{OK: false, Error: errMsg}
	}

	key := "/" + strings.ToLower(command.Name)
	if existing, exists := pluginCommands[key]; exists {
		return CommandRegistrationResult{
			OK:    false,
			Error: fmt.Sprintf("Command %q already registered by plugin %q", command.Name, existing.PluginID),
		}
	}

	pluginCommands[key] = &RegisteredPluginCommand{
		PluginCommandDefinition: command,
		PluginID:                pluginID,
	}
	slog.Debug("Registered plugin command", "key", key, "plugin", pluginID)
	return CommandRegistrationResult{OK: true}
}

// ClearPluginCommands 清除所有已注册命令
// 对应 TS: commands.ts clearPluginCommands
func ClearPluginCommands() {
	commandsMu.Lock()
	defer commandsMu.Unlock()
	pluginCommands = make(map[string]*RegisteredPluginCommand)
}

// ClearPluginCommandsForPlugin 清除指定插件的命令
func ClearPluginCommandsForPlugin(pluginID string) {
	commandsMu.Lock()
	defer commandsMu.Unlock()
	for key, cmd := range pluginCommands {
		if cmd.PluginID == pluginID {
			delete(pluginCommands, key)
		}
	}
}

// PluginCommandMatch 命令匹配结果
type PluginCommandMatch struct {
	Command *RegisteredPluginCommand
	Args    string
}

// MatchPluginCommand 匹配插件命令
// 对应 TS: commands.ts matchPluginCommand
func MatchPluginCommand(commandBody string) *PluginCommandMatch {
	commandsMu.RLock()
	defer commandsMu.RUnlock()

	trimmed := strings.TrimSpace(commandBody)
	if !strings.HasPrefix(trimmed, "/") {
		return nil
	}

	spaceIdx := strings.Index(trimmed, " ")
	var commandName, args string
	if spaceIdx == -1 {
		commandName = trimmed
	} else {
		commandName = trimmed[:spaceIdx]
		args = strings.TrimSpace(trimmed[spaceIdx+1:])
	}

	key := strings.ToLower(commandName)
	cmd, exists := pluginCommands[key]
	if !exists {
		return nil
	}

	// If command doesn't accept args but args were provided, don't match
	if args != "" && !cmd.AcceptsArgs {
		return nil
	}

	return &PluginCommandMatch{Command: cmd, Args: args}
}

// SanitizeArgs 清理命令参数
func SanitizeArgs(args string) string {
	if args == "" {
		return ""
	}
	if len(args) > MaxArgsLength {
		args = args[:MaxArgsLength]
	}
	var b strings.Builder
	b.Grow(len(args))
	for _, ch := range args {
		code := int(ch)
		isControl := (code <= 0x1f && code != 0x09 && code != 0x0a) || code == 0x7f
		if !isControl {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// ExecutePluginCommand 执行插件命令
// 对应 TS: commands.ts executePluginCommand
func ExecutePluginCommand(command *RegisteredPluginCommand, ctx PluginCommandContext) (PluginCommandResult, error) {
	requireAuth := true
	if command.RequireAuth != nil {
		requireAuth = *command.RequireAuth
	}
	if requireAuth && !ctx.IsAuthorizedSender {
		slog.Debug("Plugin command blocked: unauthorized sender",
			"command", command.Name, "sender", ctx.SenderID)
		return PluginCommandResult{Text: "⚠️ This command requires authorization."}, nil
	}

	ctx.Args = SanitizeArgs(ctx.Args)

	commandsMu.Lock()
	registryLocked = true
	commandsMu.Unlock()

	defer func() {
		commandsMu.Lock()
		registryLocked = false
		commandsMu.Unlock()
	}()

	result, err := command.Handler(ctx)
	if err != nil {
		slog.Debug("Plugin command error", "command", command.Name, "error", err.Error())
		return PluginCommandResult{Text: "⚠️ Command failed. Please try again later."}, nil
	}

	slog.Debug("Plugin command executed", "command", command.Name, "sender", ctx.SenderID)
	return result, nil
}

// ListPluginCommands 列出所有插件命令
// 对应 TS: commands.ts listPluginCommands
func ListPluginCommands() []PluginCommandInfo {
	commandsMu.RLock()
	defer commandsMu.RUnlock()

	result := make([]PluginCommandInfo, 0, len(pluginCommands))
	for _, cmd := range pluginCommands {
		result = append(result, PluginCommandInfo{
			Name:        cmd.Name,
			Description: cmd.Description,
			PluginID:    cmd.PluginID,
		})
	}
	return result
}

// PluginCommandInfo 命令信息
type PluginCommandInfo struct {
	Name        string
	Description string
	PluginID    string
}

// GetPluginCommandSpecs 获取命令规格（用于 Telegram 等原生注册）
func GetPluginCommandSpecs() []PluginCommandSpec {
	commandsMu.RLock()
	defer commandsMu.RUnlock()

	result := make([]PluginCommandSpec, 0, len(pluginCommands))
	for _, cmd := range pluginCommands {
		result = append(result, PluginCommandSpec{
			Name:        cmd.Name,
			Description: cmd.Description,
		})
	}
	return result
}

// PluginCommandSpec 命令规格
type PluginCommandSpec struct {
	Name        string
	Description string
}
