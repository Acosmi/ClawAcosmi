package autoreply

import (
	"context"
	"fmt"
	"strings"
)

// TS 对照: auto-reply/reply/commands-ptt.ts (209L)

// NodeResolver 节点解析接口。
// TS 对照: commands-ptt.ts 中的 gateway 节点发现逻辑
type NodeResolver interface {
	ListNodes(ctx context.Context) ([]NodeSummary, error)
	ResolveNode(ctx context.Context, hint string) (*NodeSummary, error)
}

// NodeSummary 节点摘要。
// TS 对照: commands-ptt.ts NodeSummary
type NodeSummary struct {
	NodeID       string
	DisplayName  string
	Platform     string
	DeviceFamily string
	RemoteIP     string
	Connected    bool
}

// pttCommands PTT 子命令映射。
// TS 对照: commands-ptt.ts PTT_COMMANDS
var pttCommands = map[string]string{
	"start":  "talk.ptt.start",
	"stop":   "talk.ptt.stop",
	"once":   "talk.ptt.once",
	"toggle": "talk.ptt.toggle",
	"status": "talk.ptt.status",
	"mute":   "talk.ptt.mute",
	"unmute": "talk.ptt.unmute",
}

// pttAliases PTT 子命令别名。
// TS 对照: commands-ptt.ts PTT_ALIASES
var pttAliases = map[string]string{
	"on":      "start",
	"off":     "stop",
	"single":  "once",
	"one":     "once",
	"flip":    "toggle",
	"switch":  "toggle",
	"state":   "status",
	"info":    "status",
	"quiet":   "mute",
	"silence": "mute",
	"talk":    "unmute",
	"speak":   "unmute",
}

// parsePTTArgs 解析 PTT 命令参数。
// 格式: /ptt [sub] [node-hint]
// TS 对照: commands-ptt.ts parsePttArgs
func parsePTTArgs(body string) (sub string, nodeHint string) {
	lower := strings.ToLower(strings.TrimSpace(body))
	if !strings.HasPrefix(lower, "/ptt") {
		return "", ""
	}

	rest := strings.TrimSpace(lower[len("/ptt"):])
	if rest == "" {
		return "status", ""
	}

	parts := strings.Fields(rest)
	rawSub := parts[0]

	// 解析别名
	if aliased, ok := pttAliases[rawSub]; ok {
		rawSub = aliased
	}

	// 验证子命令
	if _, ok := pttCommands[rawSub]; !ok {
		return "", ""
	}

	hint := ""
	if len(parts) > 1 {
		hint = strings.Join(parts[1:], " ")
	}

	return rawSub, hint
}

// HandlePTTCommand /ptt 命令处理器。
// TS 对照: commands-ptt.ts handlePttCommand
func HandlePTTCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	cmd := params.Command
	body := strings.ToLower(strings.TrimSpace(cmd.CommandBodyNormalized))

	if !strings.HasPrefix(body, "/ptt") {
		return nil, nil
	}

	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized."},
		}, nil
	}

	sub, nodeHint := parsePTTArgs(cmd.CommandBodyNormalized)
	if sub == "" {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply: &ReplyPayload{
				Text: "⚠️ Unknown PTT sub-command.\n" +
					"Usage: `/ptt [start|stop|once|toggle|status|mute|unmute] [node]`",
			},
		}, nil
	}

	gatewayMethod, ok := pttCommands[sub]
	if !ok {
		// 不应到达此处
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Invalid PTT command."},
		}, nil
	}

	// 委托 GatewayCaller 执行
	if params.GatewayCaller == nil {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Gateway not available for PTT."},
		}, nil
	}

	gwParams := map[string]any{
		"action": gatewayMethod,
	}
	if nodeHint != "" {
		gwParams["nodeHint"] = nodeHint
	}

	result, err := params.GatewayCaller.CallGateway(ctx, gatewayMethod, gwParams)
	if err != nil {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("❌ PTT error: %v", err)},
		}, nil
	}

	// 构建回复
	replyText := fmt.Sprintf("🎙️ PTT %s", sub)
	if msg, ok := result["message"].(string); ok && msg != "" {
		replyText = msg
	}

	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: replyText},
	}, nil
}
