package bridge

// dingtalk_actions.go — 钉钉 bridge action 路由

import (
	"context"
	"fmt"
)

// DingTalkActionDeps 钉钉 bridge action 依赖接口。
type DingTalkActionDeps interface {
	// SendMessage 发送消息（文本或 Markdown）。
	SendMessage(ctx context.Context, to, text string) error
	// SendGroupMessage 发送群消息。
	SendGroupMessage(ctx context.Context, openConversationID, text string) error
}

// HandleDingTalkAction 钉钉 action 分发路由
func HandleDingTalkAction(ctx context.Context, params map[string]interface{}, actionGate ActionGate, deps DingTalkActionDeps) (ToolResult, error) {
	action, err := ReadStringParam(params, "action", true)
	if err != nil {
		return ErrorResult(err), nil
	}

	if !actionGate(action) {
		return ErrorResult(fmt.Errorf("action %q is not allowed", action)), nil
	}

	switch action {
	case "send_message":
		to, err := ReadStringParam(params, "to", true)
		if err != nil {
			return ErrorResult(err), nil
		}
		text, err := ReadStringParam(params, "text", true)
		if err != nil {
			return ErrorResult(err), nil
		}
		if sendErr := deps.SendMessage(ctx, to, text); sendErr != nil {
			return ErrorResult(sendErr), nil
		}
		return OkResult(map[string]string{
			"status":  "sent",
			"channel": "dingtalk",
		}), nil

	case "send_group_message":
		conversationID, err := ReadStringParam(params, "openConversationId", true)
		if err != nil {
			return ErrorResult(err), nil
		}
		text, err := ReadStringParam(params, "text", true)
		if err != nil {
			return ErrorResult(err), nil
		}
		if sendErr := deps.SendGroupMessage(ctx, conversationID, text); sendErr != nil {
			return ErrorResult(sendErr), nil
		}
		return OkResult(map[string]string{
			"status":         "sent",
			"channel":        "dingtalk",
			"conversationId": conversationID,
		}), nil

	default:
		return ErrorResult(fmt.Errorf("unknown dingtalk action: %s", action)), nil
	}
}
