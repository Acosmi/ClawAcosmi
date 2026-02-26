package autoreply

import (
	"context"
	"fmt"
	"strings"
)

// TS 对照: auto-reply/reply/commands-info.ts (205L)

// StatusReplyBuilder 状态回复构建器接口。
// TS 对照: auto-reply/status.ts buildStatusReply / buildCommandsMessage / buildHelpMessage
type StatusReplyBuilder interface {
	BuildStatusReply(ctx context.Context, params map[string]any) (string, error)
	BuildContextReply(ctx context.Context, params map[string]any) (string, error)
	BuildCommandsMessage(params map[string]any) string
	BuildCommandsMessagePaginated(page int, pageSize int) (string, bool)
	BuildHelpMessage() string
}

// HandleHelpCommand /help 命令处理器。
// TS 对照: commands-info.ts handleHelpCommand
func HandleHelpCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := strings.ToLower(strings.TrimSpace(cmd.CommandBodyNormalized))

	isHelp := body == "/help" || body == "/?" || body == "?"
	if !isHelp {
		return nil, nil
	}

	// 构建帮助消息
	helpText := buildDefaultHelpMessage()

	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: helpText},
	}, nil
}

// HandleCommandsListCommand /commands 命令处理器。
// TS 对照: commands-info.ts handleCommandsCommand
func HandleCommandsListCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := strings.ToLower(strings.TrimSpace(cmd.CommandBodyNormalized))

	if body != "/commands" && !strings.HasPrefix(body, "/commands ") {
		return nil, nil
	}

	// 解析分页参数
	page := 1
	rest := strings.TrimSpace(strings.TrimPrefix(body, "/commands"))
	if rest != "" {
		if _, err := fmt.Sscanf(rest, "%d", &page); err != nil {
			page = 1
		}
	}

	// 使用注册表构建命令列表
	cmds := ListChatCommands()
	lines := make([]string, 0, len(cmds)+2)
	lines = append(lines, "📋 *Available Commands*")
	lines = append(lines, "")

	for _, c := range cmds {
		desc := c.Description
		if desc == "" {
			desc = "(no description)"
		}
		lines = append(lines, fmt.Sprintf("  `/%s` — %s", c.Key, desc))
	}

	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: strings.Join(lines, "\n")},
	}, nil
}

// HandleStatusInfoCommand /status (info variant) 命令处理器。
// 简明版本，用于 /status 在 commands-info 中的快速路径。
// TS 对照: commands-info.ts handleStatusCommand (quick path)
func HandleStatusInfoCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := strings.ToLower(strings.TrimSpace(cmd.CommandBodyNormalized))

	if body != "/status" {
		return nil, nil
	}

	// 构建简明状态
	statusText := fmt.Sprintf("📊 *Status*\n"+
		"• Provider: %s\n"+
		"• Model: %s\n"+
		"• Session: %s\n"+
		"• Channel: %s",
		params.Provider,
		params.Model,
		params.SessionKey,
		cmd.Channel)

	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: statusText},
	}, nil
}

// HandleWhoamiCommand /whoami 命令处理器。
// TS 对照: commands-info.ts handleWhoamiCommand
func HandleWhoamiCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := strings.ToLower(strings.TrimSpace(cmd.CommandBodyNormalized))

	if body != "/whoami" {
		return nil, nil
	}

	lines := []string{
		"👤 *Who Am I*",
		fmt.Sprintf("• Sender: %s", cmd.SenderID),
		fmt.Sprintf("• From: %s", cmd.From),
		fmt.Sprintf("• Channel: %s (%s)", cmd.Channel, cmd.ChannelID),
		fmt.Sprintf("• Surface: %s", cmd.Surface),
		fmt.Sprintf("• Owner: %v", cmd.SenderIsOwner),
		fmt.Sprintf("• Authorized: %v", cmd.IsAuthorizedSender),
	}

	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: strings.Join(lines, "\n")},
	}, nil
}

// HandleContextInfoCommand /context 命令处理器。
// TS 对照: commands-info.ts handleContextCommand (simple path)
func HandleContextInfoCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := strings.ToLower(strings.TrimSpace(cmd.CommandBodyNormalized))

	if body != "/context" {
		return nil, nil
	}

	replyText := fmt.Sprintf("📝 *Context*\n"+
		"• Session: %s\n"+
		"• Agent: %s\n"+
		"• Channel: %s",
		params.SessionKey,
		params.AgentID,
		cmd.Channel)

	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: replyText},
	}, nil
}

// buildDefaultHelpMessage 构建默认帮助消息。
func buildDefaultHelpMessage() string {
	cmds := ListChatCommands()
	lines := []string{
		"🤖 *Help*",
		"",
		"Use slash commands to control the bot.",
		"Type `/commands` for a full list.",
		"",
		fmt.Sprintf("Registered commands: %d", len(cmds)),
	}
	return strings.Join(lines, "\n")
}
