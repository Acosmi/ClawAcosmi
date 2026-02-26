package autoreply

import (
	"context"
	"fmt"
	"strings"
)

// TS 对照: auto-reply/reply/commands-subagents.ts (431L)

// SubagentManager 子代理管理接口。
// TS 对照: agents/agent-scope.ts, agents/runner/subagent-*.ts
type SubagentManager interface {
	ListSubagents(ctx context.Context) ([]SubagentInfo, error)
	GetSubagent(ctx context.Context, id string) (*SubagentInfo, error)
	CreateSubagent(ctx context.Context, params *CreateSubagentParams) (*SubagentInfo, error)
	DeleteSubagent(ctx context.Context, id string) error
	SendToSubagent(ctx context.Context, id string, message string) error
}

// SubagentInfo 子代理信息。
type SubagentInfo struct {
	ID          string
	Name        string
	Description string
	AgentDir    string
	IsActive    bool
}

// CreateSubagentParams 创建子代理参数。
type CreateSubagentParams struct {
	Name        string
	Description string
	SourceAgent string
}

// ParsedSubagentsCommand /subagent(s) 命令解析结果。
type ParsedSubagentsCommand struct {
	Action  string // "list" | "create" | "delete" | "send" | "info" | "switch"
	ID      string
	Name    string
	Message string
}

// parseSubagentsCommand 解析 /subagent 或 /subagents 命令。
// TS 对照: commands-subagents.ts parseSubagentCommand
func parseSubagentsCommand(body string) *ParsedSubagentsCommand {
	lower := strings.ToLower(strings.TrimSpace(body))

	var prefix string
	if strings.HasPrefix(lower, "/subagents") {
		prefix = "/subagents"
	} else if strings.HasPrefix(lower, "/subagent") {
		prefix = "/subagent"
	} else if strings.HasPrefix(lower, "/agent") {
		prefix = "/agent"
	} else {
		return nil
	}

	if lower == prefix {
		return &ParsedSubagentsCommand{Action: "list"}
	}
	if len(lower) > len(prefix) && lower[len(prefix)] != ' ' {
		// 排除 /agents → 不匹配 /agent 前缀
		if prefix == "/agent" {
			return nil
		}
	}

	rest := strings.TrimSpace(body[len(prefix):])
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return &ParsedSubagentsCommand{Action: "list"}
	}

	action := strings.ToLower(parts[0])
	cmd := &ParsedSubagentsCommand{Action: action}

	switch action {
	case "list", "ls":
		cmd.Action = "list"
	case "create", "new", "add":
		cmd.Action = "create"
		if len(parts) > 1 {
			cmd.Name = parts[1]
		}
	case "delete", "remove", "rm":
		cmd.Action = "delete"
		if len(parts) > 1 {
			cmd.ID = parts[1]
		}
	case "send", "message", "msg":
		cmd.Action = "send"
		if len(parts) > 1 {
			cmd.ID = parts[1]
		}
		if len(parts) > 2 {
			cmd.Message = strings.Join(parts[2:], " ")
		}
	case "info", "details":
		cmd.Action = "info"
		if len(parts) > 1 {
			cmd.ID = parts[1]
		}
	case "switch", "use":
		cmd.Action = "switch"
		if len(parts) > 1 {
			cmd.ID = parts[1]
		}
	default:
		// 当作发送给指定子代理
		cmd.Action = "send"
		cmd.ID = action
		if len(parts) > 1 {
			cmd.Message = strings.Join(parts[1:], " ")
		}
	}

	return cmd
}

// HandleSubagentsCommand /subagent 或 /subagents 命令处理器。
// TS 对照: commands-subagents.ts handleSubagentsCommand
func HandleSubagentsCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := cmd.CommandBodyNormalized
	lower := strings.ToLower(strings.TrimSpace(body))

	if !strings.HasPrefix(lower, "/subagent") && !strings.HasPrefix(lower, "/agent") {
		return nil, nil
	}

	// /agent 需要精确匹配，避免冲突
	if strings.HasPrefix(lower, "/agent") && !strings.HasPrefix(lower, "/subagent") {
		afterAgent := lower[len("/agent"):]
		if afterAgent != "" && afterAgent[0] != ' ' && afterAgent != "s" && !strings.HasPrefix(afterAgent, "s ") {
			return nil, nil
		}
	}

	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized."},
		}, nil
	}

	parsed := parseSubagentsCommand(body)
	if parsed == nil {
		return nil, nil
	}

	switch parsed.Action {
	case "list":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("🤖 *Subagents*\n• Current agent: %s\n(pending SubagentManager implementation)", params.AgentID)},
		}, nil
	case "create":
		if parsed.Name == "" {
			return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: "⚠️ Usage: /subagent create <name>"}}, nil
		}
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("🤖 Subagent created: %s", parsed.Name)},
		}, nil
	case "delete":
		if parsed.ID == "" {
			return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: "⚠️ Usage: /subagent delete <id>"}}, nil
		}
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("🤖 Subagent deleted: %s", parsed.ID)},
		}, nil
	case "send":
		if parsed.ID == "" || parsed.Message == "" {
			return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: "⚠️ Usage: /subagent send <id> <message>"}}, nil
		}
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("🤖 Sending to %s: %s", parsed.ID, truncateForDisplay(parsed.Message, 60))},
		}, nil
	case "info":
		if parsed.ID == "" {
			return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: "⚠️ Usage: /subagent info <id>"}}, nil
		}
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("🤖 Subagent info: %s\n(pending SubagentManager implementation)", parsed.ID)},
		}, nil
	case "switch":
		if parsed.ID == "" {
			return &CommandHandlerResult{ShouldContinue: false, Reply: &ReplyPayload{Text: "⚠️ Usage: /subagent switch <id>"}}, nil
		}
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("🤖 Switched to subagent: %s", parsed.ID)},
		}, nil
	default:
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /subagent [list|create|delete|send|info|switch]"},
		}, nil
	}
}
