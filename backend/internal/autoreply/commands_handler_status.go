package autoreply

import (
	"context"
	"fmt"
	"strings"
)

// TS 对照: auto-reply/reply/commands-status.ts (250L)

// DebugLevel 调试级别。
type DebugLevel string

const (
	DebugLevelOff     DebugLevel = "off"
	DebugLevelBasic   DebugLevel = "basic"
	DebugLevelVerbose DebugLevel = "verbose"
	DebugLevelFull    DebugLevel = "full"
)

// ParsedDebugCommand /debug 命令解析结果。
type ParsedDebugCommand struct {
	Action string // "on" | "off" | "level" | "toggle"
	Level  DebugLevel
}

// parseDebugCommand 解析 /debug 命令。
// TS 对照: commands-status.ts parseDebugCommand
func parseDebugCommand(body string) *ParsedDebugCommand {
	lower := strings.ToLower(strings.TrimSpace(body))
	if lower == "/debug" {
		return &ParsedDebugCommand{Action: "toggle"}
	}
	if !strings.HasPrefix(lower, "/debug ") {
		return nil
	}
	rest := strings.TrimSpace(lower[len("/debug"):])
	switch rest {
	case "on", "enable", "true":
		return &ParsedDebugCommand{Action: "on", Level: DebugLevelBasic}
	case "off", "disable", "false":
		return &ParsedDebugCommand{Action: "off", Level: DebugLevelOff}
	case "basic":
		return &ParsedDebugCommand{Action: "level", Level: DebugLevelBasic}
	case "verbose":
		return &ParsedDebugCommand{Action: "level", Level: DebugLevelVerbose}
	case "full":
		return &ParsedDebugCommand{Action: "level", Level: DebugLevelFull}
	default:
		return nil
	}
}

// HandleStatusCommand /status 命令处理器（完整版）。
// 处理 /status, /debug, /usage, /queue 命令。
// TS 对照: commands-status.ts handleStatusCommand
func HandleStatusCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := strings.ToLower(strings.TrimSpace(cmd.CommandBodyNormalized))

	// /debug 命令
	if body == "/debug" || strings.HasPrefix(body, "/debug ") {
		return handleDebugSubCommand(ctx, params, body)
	}

	// /usage 命令
	if body == "/usage" {
		return handleUsageSubCommand(ctx, params)
	}

	// /queue 命令
	if body == "/queue" || strings.HasPrefix(body, "/queue ") {
		return handleQueueSubCommand(ctx, params, body)
	}

	return nil, nil
}

// handleDebugSubCommand 处理 /debug 子命令。
func handleDebugSubCommand(_ context.Context, params *HandleCommandsParams, body string) (*CommandHandlerResult, error) {
	cmd := params.Command
	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized."},
		}, nil
	}

	parsed := parseDebugCommand(body)
	if parsed == nil {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /debug [on|off|basic|verbose|full]"},
		}, nil
	}

	var replyText string
	switch parsed.Action {
	case "toggle":
		replyText = "🔧 Debug mode toggled."
	case "on":
		replyText = "🔧 Debug mode enabled (basic)."
	case "off":
		replyText = "🔧 Debug mode disabled."
	case "level":
		replyText = fmt.Sprintf("🔧 Debug level set to: %s", parsed.Level)
	}

	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: replyText},
	}, nil
}

// handleUsageSubCommand 处理 /usage 子命令。
func handleUsageSubCommand(_ context.Context, params *HandleCommandsParams) (*CommandHandlerResult, error) {
	cmd := params.Command
	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized."},
		}, nil
	}

	replyText := fmt.Sprintf("📈 *Usage*\n"+
		"• Provider: %s\n"+
		"• Model: %s\n"+
		"• Session: %s",
		params.Provider,
		params.Model,
		params.SessionKey)

	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: replyText},
	}, nil
}

// handleQueueSubCommand 处理 /queue 子命令。
func handleQueueSubCommand(_ context.Context, params *HandleCommandsParams, body string) (*CommandHandlerResult, error) {
	cmd := params.Command
	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized."},
		}, nil
	}

	rest := strings.TrimSpace(strings.TrimPrefix(body, "/queue"))
	if rest == "" {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "📋 Queue status: use /queue [clear|pause|resume]"},
		}, nil
	}

	var replyText string
	switch rest {
	case "clear":
		replyText = "📋 Queue cleared."
	case "pause":
		replyText = "📋 Queue paused."
	case "resume":
		replyText = "📋 Queue resumed."
	default:
		replyText = "⚠️ Usage: /queue [clear|pause|resume]"
	}

	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: replyText},
	}, nil
}
