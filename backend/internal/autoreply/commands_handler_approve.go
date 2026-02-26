package autoreply

import (
	"context"
	"fmt"
	"strings"
)

// TS 对照: auto-reply/reply/commands-approve.ts (126L)

// ApproveDecision 审批决策类型。
type ApproveDecision string

const (
	ApproveAllowOnce   ApproveDecision = "allow-once"
	ApproveAllowAlways ApproveDecision = "allow-always"
	ApproveDeny        ApproveDecision = "deny"
)

// decisionAliases 审批决策别名映射。
// TS 对照: commands-approve.ts DECISION_ALIASES
var decisionAliases = map[string]ApproveDecision{
	"allow":        ApproveAllowOnce,
	"once":         ApproveAllowOnce,
	"allow-once":   ApproveAllowOnce,
	"allowonce":    ApproveAllowOnce,
	"always":       ApproveAllowAlways,
	"allow-always": ApproveAllowAlways,
	"allowalways":  ApproveAllowAlways,
	"deny":         ApproveDeny,
	"reject":       ApproveDeny,
	"block":        ApproveDeny,
	"no":           ApproveDeny,
}

// ParsedApproveCommand /approve 命令解析结果。
type ParsedApproveCommand struct {
	Decision  ApproveDecision
	RequestID string
}

// parseApproveCommand 解析 /approve 命令。
// 格式: /approve [decision] [requestId]
// TS 对照: commands-approve.ts L33-58
func parseApproveCommand(body string) *ParsedApproveCommand {
	lower := strings.ToLower(strings.TrimSpace(body))
	if !strings.HasPrefix(lower, "/approve") {
		return nil
	}

	rest := strings.TrimSpace(lower[len("/approve"):])
	if rest == "" {
		return &ParsedApproveCommand{Decision: ApproveAllowOnce}
	}

	parts := strings.Fields(rest)
	decision, ok := decisionAliases[parts[0]]
	if !ok {
		decision = ApproveAllowOnce
	}

	result := &ParsedApproveCommand{Decision: decision}
	if len(parts) > 1 {
		result.RequestID = parts[1]
	}
	return result
}

// HandleApproveCommand /approve 命令处理器。
// TS 对照: commands-approve.ts handleApproveCommand
func HandleApproveCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	parsed := parseApproveCommand(params.Command.CommandBodyNormalized)
	if parsed == nil {
		return nil, nil
	}

	cmd := params.Command
	if !cmd.IsAuthorizedSender {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⛔ Not authorized to approve commands."},
		}, nil
	}

	// 委托 GatewayCaller 执行审批
	if params.GatewayCaller == nil {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "⚠️ Gateway not available for approval."},
		}, nil
	}

	gwParams := map[string]any{
		"decision": string(parsed.Decision),
	}
	if parsed.RequestID != "" {
		gwParams["requestId"] = parsed.RequestID
	}

	result, err := params.GatewayCaller.CallGateway(ctx, "exec.approve", gwParams)
	if err != nil {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: fmt.Sprintf("❌ Approve error: %v", err)},
		}, nil
	}

	// 构建回复
	replyText := "✅ Approved."
	if msg, ok := result["message"].(string); ok && msg != "" {
		replyText = msg
	}

	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          &ReplyPayload{Text: replyText},
	}, nil
}
