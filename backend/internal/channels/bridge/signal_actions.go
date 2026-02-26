package bridge

import (
	"context"
	"fmt"
	"strconv"

	"github.com/anthropic/open-acosmi/internal/channels/signal"
)

// Signal action 路由 — 继承自 src/channels/plugins/actions/signal.ts (147L)

// SignalActionContext Signal action 执行上下文
type SignalActionContext struct {
	Ctx       context.Context
	BaseURL   string
	Account   string
	AccountID string
}

// HandleSignalAction Signal action 分发路由
func HandleSignalAction(params map[string]interface{}, actionGate ActionGate, actx *SignalActionContext) (ToolResult, error) {
	action, err := ReadStringParam(params, "action", true)
	if err != nil {
		return ErrorResult(err), err
	}

	switch action {
	case "sendMessage":
		recipient, err := ReadStringParam(params, "recipient", true)
		if err != nil {
			return ErrorResult(err), err
		}
		text, err := ReadStringParam(params, "text", true)
		if err != nil {
			return ErrorResult(err), err
		}
		mediaURL, _ := ReadStringParam(params, "mediaUrl", false)

		opts := signal.SignalSendOpts{
			BaseURL:   actx.BaseURL,
			Account:   actx.Account,
			AccountID: actx.AccountID,
			MediaURL:  mediaURL,
		}

		result, err := signal.SendMessageSignal(actx.Ctx, recipient, text, opts)
		if err != nil {
			return ErrorResult(err), err
		}
		out := map[string]interface{}{"ok": true}
		if result != nil {
			out["messageId"] = result.MessageID
			out["timestamp"] = result.Timestamp
		}
		return OkResult(out), nil

	case "react":
		if !actionGate("reactions") {
			return ToolResult{}, fmt.Errorf("Signal reactions are disabled")
		}
		recipient, err := ReadStringParam(params, "recipient", true)
		if err != nil {
			return ErrorResult(err), err
		}
		messageIdStr, err := ReadStringParam(params, "messageId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		timestamp, err := strconv.ParseInt(messageIdStr, 10, 64)
		if err != nil {
			return ErrorResult(fmt.Errorf("invalid messageId (must be numeric timestamp): %s", messageIdStr)), err
		}

		reaction := ReadReactionParams(params)
		targetAuthor, _ := ReadStringParam(params, "targetAuthor", false)

		reactionOpts := signal.SignalReactionOpts{
			BaseURL:   actx.BaseURL,
			Account:   actx.Account,
			AccountID: actx.AccountID,
		}

		if reaction.Remove {
			if err := signal.RemoveReactionSignal(actx.Ctx, recipient, reaction.Emoji, timestamp, reactionOpts); err != nil {
				return ErrorResult(err), err
			}
			return OkResult(map[string]interface{}{"ok": true, "removed": true}), nil
		}

		if reaction.IsEmpty {
			return ErrorResult(fmt.Errorf("emoji is required for adding a reaction")), fmt.Errorf("missing emoji")
		}

		if targetAuthor != "" {
			if err := signal.SendReactionToTargetSignal(actx.Ctx, recipient, reaction.Emoji, timestamp, targetAuthor, reactionOpts); err != nil {
				return ErrorResult(err), err
			}
		} else {
			if err := signal.SendReactionSignal(actx.Ctx, recipient, reaction.Emoji, timestamp, reactionOpts); err != nil {
				return ErrorResult(err), err
			}
		}
		return OkResult(map[string]interface{}{"ok": true, "added": reaction.Emoji}), nil

	default:
		return ToolResult{}, fmt.Errorf("unsupported Signal action: %s", action)
	}
}
