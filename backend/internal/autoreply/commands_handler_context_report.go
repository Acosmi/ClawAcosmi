package autoreply

import (
	"context"
	"fmt"
	"strings"
)

// TS 对照: auto-reply/reply/commands-context-report.ts (338L)

// ContextReportResolver 上下文报告解析器接口。
// TS 对照: commands-context-report.ts buildContextReply
type ContextReportResolver interface {
	BuildContextReport(ctx context.Context, params *ContextReportParams) (string, error)
}

// ContextReportParams 上下文报告参数。
type ContextReportParams struct {
	SessionKey string
	AgentID    string
	Channel    string
	Surface    string
	Verbose    bool
}

// ParsedContextReportCommand /context-report 命令解析结果。
type ParsedContextReportCommand struct {
	Action  string // "full" | "summary" | "tokens" | "history" | "agent"
	Verbose bool
}

// parseContextReportCommand 解析 /context-report 命令。
// TS 对照: commands-context-report.ts (top-level parse)
func parseContextReportCommand(body string) *ParsedContextReportCommand {
	lower := strings.ToLower(strings.TrimSpace(body))

	// 匹配 /context-report 或 /cr
	var prefix string
	if strings.HasPrefix(lower, "/context-report") {
		prefix = "/context-report"
	} else if strings.HasPrefix(lower, "/cr") {
		prefix = "/cr"
	} else {
		return nil
	}

	if lower == prefix {
		return &ParsedContextReportCommand{Action: "full"}
	}
	if len(lower) > len(prefix) && lower[len(prefix)] != ' ' {
		return nil
	}

	rest := strings.TrimSpace(lower[len(prefix):])
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return &ParsedContextReportCommand{Action: "full"}
	}

	cmd := &ParsedContextReportCommand{}
	action := parts[0]
	switch action {
	case "full", "all":
		cmd.Action = "full"
	case "summary", "short":
		cmd.Action = "summary"
	case "tokens", "usage":
		cmd.Action = "tokens"
	case "history", "messages":
		cmd.Action = "history"
	case "agent":
		cmd.Action = "agent"
	case "-v", "--verbose", "verbose":
		cmd.Action = "full"
		cmd.Verbose = true
	default:
		cmd.Action = "full"
	}

	// 检查 verbose 标志
	for _, p := range parts[1:] {
		if p == "-v" || p == "--verbose" {
			cmd.Verbose = true
		}
	}

	return cmd
}

// HandleContextReportCommand /context-report 或 /cr 命令处理器。
// TS 对照: commands-context-report.ts handleContextReportCommand
func HandleContextReportCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := cmd.CommandBodyNormalized
	lower := strings.ToLower(strings.TrimSpace(body))

	if !strings.HasPrefix(lower, "/context-report") && !strings.HasPrefix(lower, "/cr") {
		return nil, nil
	}

	// /cr 在非 /cr 前缀时额外检查不匹配其它 /c 命令
	if strings.HasPrefix(lower, "/cr") && !strings.HasPrefix(lower, "/context-report") {
		// 确保不是 /create 或其他 /cr* 命令
		afterCR := lower[len("/cr"):]
		if afterCR != "" && afterCR[0] != ' ' {
			return nil, nil
		}
	}

	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized."},
		}, nil
	}

	parsed := parseContextReportCommand(body)
	if parsed == nil {
		return nil, nil
	}

	switch parsed.Action {
	case "full":
		verboseFlag := ""
		if parsed.Verbose {
			verboseFlag = " (verbose)"
		}
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply: &ReplyPayload{Text: fmt.Sprintf("📊 *Context Report%s*\n"+
				"• Session: %s\n"+
				"• Agent: %s\n"+
				"• Channel: %s\n"+
				"(pending ContextReportResolver implementation)",
				verboseFlag, params.SessionKey, params.AgentID, cmd.Channel)},
		}, nil
	case "summary":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply: &ReplyPayload{Text: fmt.Sprintf("📊 *Context Summary*\n"+
				"• Session: %s\n"+
				"(pending ContextReportResolver implementation)",
				params.SessionKey)},
		}, nil
	case "tokens":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "📊 Token usage: (pending ContextReportResolver implementation)"},
		}, nil
	case "history":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "📊 Message history: (pending ContextReportResolver implementation)"},
		}, nil
	case "agent":
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("📊 Agent: %s\n(pending ContextReportResolver implementation)", params.AgentID)},
		}, nil
	default:
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Usage: /cr [full|summary|tokens|history|agent] [-v]"},
		}, nil
	}
}
