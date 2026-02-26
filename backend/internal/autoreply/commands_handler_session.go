package autoreply

import (
	"context"
	"fmt"
	"strings"
)

// TS 对照: auto-reply/reply/commands-session.ts (380L)

// SessionManager 会话管理接口。
// TS 对照: config/sessions.ts
type SessionManager interface {
	ListSessions(ctx context.Context) ([]SessionInfo, error)
	GetSession(ctx context.Context, key string) (*SessionInfo, error)
	ActivateSession(ctx context.Context, key string) error
	ResetSession(ctx context.Context, key string) error
	DeleteSession(ctx context.Context, key string) error
}

// SessionInfo 会话信息。
type SessionInfo struct {
	Key        string
	Label      string
	IsActive   bool
	TokenUsage int64
}

// ParsedSessionCommand /session 命令解析结果。
type ParsedSessionCommand struct {
	Action     string // "list" | "switch" | "new" | "delete" | "rename" | "info" | "export" | "activation" | "send-policy" | "usage" | "stop" | "restart"
	SessionKey string
	Label      string
	Mode       string // activation mode 或 send-policy mode
}

// parseSessionCommand 解析 /session 命令。
// TS 对照: commands-session.ts parseSessionCommand
func parseSessionCommand(body string) *ParsedSessionCommand {
	lower := strings.ToLower(strings.TrimSpace(body))
	if lower == "/session" || lower == "/sessions" {
		return &ParsedSessionCommand{Action: "list"}
	}

	var prefix string
	if strings.HasPrefix(lower, "/sessions ") {
		prefix = "/sessions"
	} else if strings.HasPrefix(lower, "/session ") {
		prefix = "/session"
	} else {
		return nil
	}

	rest := strings.TrimSpace(body[len(prefix):])
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return &ParsedSessionCommand{Action: "list"}
	}

	action := strings.ToLower(parts[0])
	cmd := &ParsedSessionCommand{Action: action}

	switch action {
	case "list", "ls":
		cmd.Action = "list"
	case "switch", "use", "activate":
		cmd.Action = "switch"
		if len(parts) > 1 {
			cmd.SessionKey = parts[1]
		}
	case "new", "create":
		cmd.Action = "new"
		if len(parts) > 1 {
			cmd.Label = strings.Join(parts[1:], " ")
		}
	case "delete", "remove", "rm":
		cmd.Action = "delete"
		if len(parts) > 1 {
			cmd.SessionKey = parts[1]
		}
	case "rename":
		cmd.Action = "rename"
		if len(parts) > 1 {
			cmd.SessionKey = parts[1]
		}
		if len(parts) > 2 {
			cmd.Label = strings.Join(parts[2:], " ")
		}
	case "info", "details":
		cmd.Action = "info"
		if len(parts) > 1 {
			cmd.SessionKey = parts[1]
		}
	case "export":
		cmd.Action = "export"
		if len(parts) > 1 {
			cmd.SessionKey = parts[1]
		}
	case "usage":
		cmd.Action = "usage"
	case "stop":
		cmd.Action = "stop"
	case "restart":
		cmd.Action = "restart"
	default:
		// 无子命令 → 当作 switch <key>
		cmd.Action = "switch"
		cmd.SessionKey = action
	}

	return cmd
}

// HandleSessionCommand /session 命令处理器。
// TS 对照: commands-session.ts handleSessionCommand
func HandleSessionCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := cmd.CommandBodyNormalized
	lower := strings.ToLower(strings.TrimSpace(body))

	// 匹配 /session 或 /sessions
	if !strings.HasPrefix(lower, "/session") {
		return nil, nil
	}

	// /activation 独立处理（TS 中 session handler 也处理 /activation）
	if strings.HasPrefix(lower, "/activation") {
		return handleActivationSubCommand(ctx, params, body)
	}

	// /send-policy 独立处理
	if strings.HasPrefix(lower, "/send-policy") || strings.HasPrefix(lower, "/sendpolicy") {
		return handleSendPolicySubCommand(ctx, params, body)
	}

	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized."},
		}, nil
	}

	parsed := parseSessionCommand(body)
	if parsed == nil {
		return nil, nil
	}

	switch parsed.Action {
	case "list":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("📂 *Sessions*\n• Current: %s\n(pending SessionManager implementation)", params.SessionKey)},
		}, nil
	case "switch":
		if parsed.SessionKey == "" {
			return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: "⚠️ Usage: /session switch <key>"}}, nil
		}
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("📂 Switching to session: %s", parsed.SessionKey)},
		}, nil
	case "new":
		label := parsed.Label
		if label == "" {
			label = "(unnamed)"
		}
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("📂 New session created: %s", label)},
		}, nil
	case "delete":
		if parsed.SessionKey == "" {
			return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: "⚠️ Usage: /session delete <key>"}}, nil
		}
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("📂 Session deleted: %s", parsed.SessionKey)},
		}, nil
	case "rename":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("📂 Session renamed: %s → %s", parsed.SessionKey, parsed.Label)},
		}, nil
	case "info":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "📂 Session info: (pending SessionManager implementation)"},
		}, nil
	case "export":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "📂 Session export: (pending SessionManager implementation)"},
		}, nil
	case "usage":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "📈 Session usage: (pending SessionManager implementation)"},
		}, nil
	case "stop":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⏹️ Session stopped."},
		}, nil
	case "restart":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "🔄 Session restarting..."},
		}, nil
	default:
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Unknown session sub-command. Usage: /session [list|switch|new|delete|rename|info|export|usage|stop|restart]"},
		}, nil
	}
}

// handleActivationSubCommand 处理 /activation 命令。
// 委托给现有的 ParseActivationCommand。
func handleActivationSubCommand(_ context.Context, params *HandleCommandsParams, body string) (*CommandHandlerResult, error) {
	cmd := params.Command
	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized."},
		}, nil
	}

	hasActivation, mode := ParseActivationCommand(body)
	if !hasActivation {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /activation [mention|always]"},
		}, nil
	}

	if mode == "" {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "📡 Current activation mode: (query from config)"},
		}, nil
	}

	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: fmt.Sprintf("📡 Activation mode set to: %s", mode)},
	}, nil
}

// handleSendPolicySubCommand 处理 /send-policy 命令。
// 委托给现有的 ParseSendPolicyCommand。
func handleSendPolicySubCommand(_ context.Context, params *HandleCommandsParams, body string) (*CommandHandlerResult, error) {
	cmd := params.Command
	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized."},
		}, nil
	}

	hasPolicy, policy := ParseSendPolicyCommand(body)
	if !hasPolicy {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /send-policy [allow|deny|filter]"},
		}, nil
	}

	if policy == "" {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "📡 Current send policy: (query from config)"},
		}, nil
	}

	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: fmt.Sprintf("📡 Send policy set to: %s", policy)},
	}, nil
}
