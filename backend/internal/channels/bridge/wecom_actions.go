package bridge

// wecom_actions.go — 企业微信 bridge action 路由

import (
	"context"
	"fmt"
)

// WeComActionDeps 企业微信 bridge action 依赖接口。
type WeComActionDeps interface {
	// SendText 发送文本消息。
	SendText(ctx context.Context, toUser, text string) error
	// SendMarkdown 发送 Markdown 消息。
	SendMarkdown(ctx context.Context, toUser, content string) error
}

// HandleWeComAction 企业微信 action 分发路由
func HandleWeComAction(ctx context.Context, params map[string]interface{}, actionGate ActionGate, deps WeComActionDeps) (ToolResult, error) {
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
		if sendErr := deps.SendText(ctx, to, text); sendErr != nil {
			return ErrorResult(sendErr), nil
		}
		return OkResult(map[string]string{
			"status":  "sent",
			"channel": "wecom",
		}), nil

	case "send_markdown":
		to, err := ReadStringParam(params, "to", true)
		if err != nil {
			return ErrorResult(err), nil
		}
		content, err := ReadStringParam(params, "content", true)
		if err != nil {
			return ErrorResult(err), nil
		}
		if sendErr := deps.SendMarkdown(ctx, to, content); sendErr != nil {
			return ErrorResult(sendErr), nil
		}
		return OkResult(map[string]string{
			"status":  "sent",
			"channel": "wecom",
		}), nil

	default:
		return ErrorResult(fmt.Errorf("unknown wecom action: %s", action)), nil
	}
}
