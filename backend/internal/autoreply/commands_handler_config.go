package autoreply

import (
	"context"
	"fmt"
	"strings"
)

// TS 对照: auto-reply/reply/commands-config.ts (274L)

// ConfigManager 配置管理接口。
// TS 对照: config/config.ts readConfigFileSnapshot / writeConfigFile / validateConfigObjectWithPlugins
type ConfigManager interface {
	ReadConfigSnapshot() (map[string]any, error)
	WriteConfig(cfg map[string]any) error
	ValidateConfig(cfg map[string]any) ([]string, error)
	GetConfigValueAtPath(cfg map[string]any, path []string) (any, bool)
	SetConfigValueAtPath(cfg map[string]any, path []string, value any) error
	UnsetConfigValueAtPath(cfg map[string]any, path []string) error
}

// ConfigOverrideManager 配置覆盖管理接口。
// TS 对照: config/config-overrides.ts
type ConfigOverrideManager interface {
	GetOverrides(sessionKey string) map[string]any
	SetOverride(sessionKey string, key string, value any) error
	ResetOverrides(sessionKey string) error
}

// ParsedConfigCommand /config 命令解析结果。
type ParsedConfigCommand struct {
	Action string // "get" | "set" | "unset" | "list" | "reset" | "override" | "validate"
	Path   string
	Value  string
	IsTemp bool // /config temp ... → 临时覆盖
}

// parseConfigCommand 解析 /config 命令。
// 格式: /config [get|set|unset|list|reset|validate] [path] [value]
// TS 对照: commands-config.ts parseConfigCommand
func parseConfigCommand(body string) *ParsedConfigCommand {
	lower := strings.ToLower(strings.TrimSpace(body))
	if lower == "/config" {
		return &ParsedConfigCommand{Action: "list"}
	}
	if !strings.HasPrefix(lower, "/config ") {
		return nil
	}

	rest := strings.TrimSpace(body[len("/config"):])
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return &ParsedConfigCommand{Action: "list"}
	}

	action := strings.ToLower(parts[0])
	cmd := &ParsedConfigCommand{Action: action}

	// 处理 /config temp [set|unset] [path] [value]
	if action == "temp" || action == "override" {
		cmd.IsTemp = true
		if len(parts) < 2 {
			cmd.Action = "list"
			return cmd
		}
		action = strings.ToLower(parts[1])
		cmd.Action = action
		parts = parts[2:]
	} else {
		parts = parts[1:]
	}

	switch action {
	case "get":
		if len(parts) > 0 {
			cmd.Path = parts[0]
		}
	case "set":
		if len(parts) > 0 {
			cmd.Path = parts[0]
		}
		if len(parts) > 1 {
			cmd.Value = strings.Join(parts[1:], " ")
		}
	case "unset", "delete", "remove":
		cmd.Action = "unset"
		if len(parts) > 0 {
			cmd.Path = parts[0]
		}
	case "list", "show", "dump":
		cmd.Action = "list"
	case "reset":
		cmd.Action = "reset"
	case "validate", "check":
		cmd.Action = "validate"
	default:
		// 无子命令 → 当作 get path
		cmd.Action = "get"
		cmd.Path = action
		if len(parts) > 0 {
			cmd.Value = strings.Join(parts, " ")
		}
	}

	return cmd
}

// HandleConfigCommand /config 命令处理器。
// TS 对照: commands-config.ts handleConfigCommand
func HandleConfigCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := cmd.CommandBodyNormalized
	lower := strings.ToLower(strings.TrimSpace(body))

	if !strings.HasPrefix(lower, "/config") {
		return nil, nil
	}

	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized to manage config."},
		}, nil
	}

	parsed := parseConfigCommand(body)
	if parsed == nil {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Invalid config command."},
		}, nil
	}

	switch parsed.Action {
	case "get":
		return handleConfigGet(ctx, params, parsed)
	case "set":
		return handleConfigSet(ctx, params, parsed)
	case "unset":
		return handleConfigUnset(ctx, params, parsed)
	case "list":
		return handleConfigList(ctx, params, parsed)
	case "reset":
		return handleConfigReset(ctx, params, parsed)
	case "validate":
		return handleConfigValidate(ctx, params)
	default:
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /config [get|set|unset|list|reset|validate] [path] [value]"},
		}, nil
	}
}

func handleConfigGet(_ context.Context, _ *HandleCommandsParams, parsed *ParsedConfigCommand) (*CommandHandlerResult, error) {
	if parsed.Path == "" {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /config get <path>"},
		}, nil
	}
	// 实际读取配置需要 ConfigManager DI
	replyText := fmt.Sprintf("⚙️ Config `%s`: (pending ConfigManager implementation)", parsed.Path)
	return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: replyText}}, nil
}

func handleConfigSet(_ context.Context, _ *HandleCommandsParams, parsed *ParsedConfigCommand) (*CommandHandlerResult, error) {
	if parsed.Path == "" || parsed.Value == "" {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /config set <path> <value>"},
		}, nil
	}
	qualifier := ""
	if parsed.IsTemp {
		qualifier = " (temporary override)"
	}
	replyText := fmt.Sprintf("⚙️ Config `%s` = `%s`%s", parsed.Path, parsed.Value, qualifier)
	return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: replyText}}, nil
}

func handleConfigUnset(_ context.Context, _ *HandleCommandsParams, parsed *ParsedConfigCommand) (*CommandHandlerResult, error) {
	if parsed.Path == "" {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /config unset <path>"},
		}, nil
	}
	replyText := fmt.Sprintf("⚙️ Config `%s` removed.", parsed.Path)
	return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: replyText}}, nil
}

func handleConfigList(_ context.Context, _ *HandleCommandsParams, parsed *ParsedConfigCommand) (*CommandHandlerResult, error) {
	prefix := "⚙️ *Config*"
	if parsed.IsTemp {
		prefix = "⚙️ *Temp Overrides*"
	}
	replyText := fmt.Sprintf("%s\n(pending ConfigManager implementation)", prefix)
	return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: replyText}}, nil
}

func handleConfigReset(_ context.Context, _ *HandleCommandsParams, parsed *ParsedConfigCommand) (*CommandHandlerResult, error) {
	target := "all"
	if parsed.IsTemp {
		target = "temp overrides"
	}
	replyText := fmt.Sprintf("⚙️ Config %s reset.", target)
	return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: replyText}}, nil
}

func handleConfigValidate(_ context.Context, _ *HandleCommandsParams) (*CommandHandlerResult, error) {
	replyText := "⚙️ Config validation: (pending ConfigManager implementation)"
	return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: replyText}}, nil
}
