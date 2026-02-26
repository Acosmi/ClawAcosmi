package bridge

// feishu_actions.go — 飞书 bridge action 路由
// 继承自 telegram_actions.go 相同模式

import (
	"context"
	"fmt"
)

// FeishuActionDeps 飞书 bridge action 依赖接口。
type FeishuActionDeps interface {
	// SendMessage 发送消息（文本）。
	SendMessage(ctx context.Context, to, text string) (messageID string, err error)
}

// HandleFeishuAction 飞书 action 分发路由
func HandleFeishuAction(ctx context.Context, params map[string]interface{}, actionGate ActionGate, deps FeishuActionDeps) (ToolResult, error) {
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
		msgID, sendErr := deps.SendMessage(ctx, to, text)
		if sendErr != nil {
			return ErrorResult(sendErr), nil
		}
		return OkResult(map[string]string{
			"messageId": msgID,
			"channel":   "feishu",
		}), nil

	default:
		return ErrorResult(fmt.Errorf("unknown feishu action: %s", action)), nil
	}
}
