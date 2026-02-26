package bridge

import (
	"context"
	"fmt"
)

// Slack action 路由 — 继承自 src/agents/tools/slack-actions.ts (314L)

// SlackActionContext 管理 thread 自动注入状态
type SlackActionContext struct {
	CurrentChannelID string
	CurrentThreadTs  string
	ReplyToMode      string // "off" | "first" | "all"
	HasReplied       *bool  // mutable ref for "first" mode
}

// SlackReadOpts 消息读取选项
type SlackReadOpts struct {
	Limit    int
	Before   string
	After    string
	ThreadID string
}

// SlackActionDeps Slack bridge action 依赖接口。
type SlackActionDeps interface {
	// SendMessage 发送消息。返回消息 ts。
	SendMessage(ctx context.Context, channelID, text, threadTs string) (ts string, err error)
	// EditMessage 编辑消息。
	EditMessage(ctx context.Context, channelID, messageID, content string) error
	// DeleteMessage 删除消息。
	DeleteMessage(ctx context.Context, channelID, messageID string) error
	// ReadMessages 读取频道/线程历史。
	ReadMessages(ctx context.Context, channelID string, opts SlackReadOpts) (result interface{}, err error)
	// ReactMessage 添加 emoji 反应。
	ReactMessage(ctx context.Context, channelID, messageID, emoji string) error
	// RemoveReaction 移除 emoji 反应。
	RemoveReaction(ctx context.Context, channelID, messageID, emoji string) error
	// RemoveOwnReactions 移除 bot 的所有反应。
	RemoveOwnReactions(ctx context.Context, channelID, messageID string) ([]string, error)
	// ListReactions 列出消息反应。
	ListReactions(ctx context.Context, channelID, messageID string) (interface{}, error)
	// PinMessage 固定消息。
	PinMessage(ctx context.Context, channelID, messageID string) error
	// UnpinMessage 取消固定。
	UnpinMessage(ctx context.Context, channelID, messageID string) error
	// ListPins 列出固定消息。
	ListPins(ctx context.Context, channelID string) (interface{}, error)
	// GetMemberInfo 获取用户信息。
	GetMemberInfo(ctx context.Context, userID string) (interface{}, error)
	// ListEmojis 列出自定义 emoji。
	ListEmojis(ctx context.Context) (interface{}, error)
}

// messaging / reactions / pin action 集合
var slackMessagingActions = map[string]bool{
	"sendMessage":   true,
	"editMessage":   true,
	"deleteMessage": true,
	"readMessages":  true,
}

var slackReactionsActions = map[string]bool{
	"react":     true,
	"reactions": true,
}

var slackPinActions = map[string]bool{
	"pinMessage":   true,
	"unpinMessage": true,
	"listPins":     true,
}

// resolveThreadTsFromContext 根据上下文和 replyToMode 决定 threadTs
func resolveThreadTsFromContext(explicitThreadTs, targetChannel string, ctx *SlackActionContext) string {
	if explicitThreadTs != "" {
		return explicitThreadTs
	}
	if ctx == nil || ctx.CurrentThreadTs == "" || ctx.CurrentChannelID == "" {
		return ""
	}
	if targetChannel != ctx.CurrentChannelID {
		return ""
	}
	if ctx.ReplyToMode == "all" {
		return ctx.CurrentThreadTs
	}
	if ctx.ReplyToMode == "first" && ctx.HasReplied != nil && !*ctx.HasReplied {
		*ctx.HasReplied = true
		return ctx.CurrentThreadTs
	}
	return ""
}

// HandleSlackAction Slack action 分发路由
func HandleSlackAction(ctx context.Context, params map[string]interface{}, actionGate ActionGate, sctx *SlackActionContext, deps SlackActionDeps) (ToolResult, error) {
	action, err := ReadStringParam(params, "action", true)
	if err != nil {
		return ErrorResult(err), err
	}

	// ── Reactions ──────────────────────────────────────────────────────
	if slackReactionsActions[action] {
		if !actionGate("reactions") {
			return ToolResult{}, fmt.Errorf("Slack reactions are disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		messageID, err := ReadStringParam(params, "messageId", true)
		if err != nil {
			return ErrorResult(err), err
		}

		if action == "react" {
			reaction := ReadReactionParams(params)
			if reaction.Remove {
				if reaction.Emoji == "" {
					return ToolResult{}, fmt.Errorf("Emoji is required to remove a Slack reaction")
				}
				if err := deps.RemoveReaction(ctx, channelID, messageID, reaction.Emoji); err != nil {
					return ErrorResult(err), err
				}
				return OkResult(map[string]interface{}{"ok": true, "removed": reaction.Emoji}), nil
			}
			if reaction.IsEmpty {
				removed, err := deps.RemoveOwnReactions(ctx, channelID, messageID)
				if err != nil {
					return ErrorResult(err), err
				}
				return OkResult(map[string]interface{}{"ok": true, "removed": removed}), nil
			}
			if err := deps.ReactMessage(ctx, channelID, messageID, reaction.Emoji); err != nil {
				return ErrorResult(err), err
			}
			return OkResult(map[string]interface{}{"ok": true, "added": reaction.Emoji}), nil
		}

		// action == "reactions": list reactions
		reactions, err := deps.ListReactions(ctx, channelID, messageID)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "reactions": reactions}), nil
	}

	// ── Messaging ──────────────────────────────────────────────────────
	if slackMessagingActions[action] {
		if !actionGate("messages") {
			return ToolResult{}, fmt.Errorf("Slack messages are disabled")
		}

		switch action {
		case "sendMessage":
			to, err := ReadStringParam(params, "to", true)
			if err != nil {
				return ErrorResult(err), err
			}
			content, err := ReadStringParam(params, "content", true)
			if err != nil {
				return ErrorResult(err), err
			}
			explicitThreadTs, _ := ReadStringParam(params, "threadTs", false)
			threadTs := resolveThreadTsFromContext(explicitThreadTs, to, sctx)

			ts, err := deps.SendMessage(ctx, to, content, threadTs)
			if err != nil {
				return ErrorResult(err), err
			}

			// 更新 first-mode ref
			if sctx != nil && sctx.HasReplied != nil && sctx.CurrentChannelID == to {
				*sctx.HasReplied = true
			}

			return OkResult(map[string]interface{}{"ok": true, "result": map[string]interface{}{"ts": ts}}), nil

		case "editMessage":
			channelID, err := ReadStringParam(params, "channelId", true)
			if err != nil {
				return ErrorResult(err), err
			}
			messageID, err := ReadStringParam(params, "messageId", true)
			if err != nil {
				return ErrorResult(err), err
			}
			content, err := ReadStringParam(params, "content", true)
			if err != nil {
				return ErrorResult(err), err
			}
			if err := deps.EditMessage(ctx, channelID, messageID, content); err != nil {
				return ErrorResult(err), err
			}
			return OkResult(map[string]interface{}{"ok": true}), nil

		case "deleteMessage":
			channelID, err := ReadStringParam(params, "channelId", true)
			if err != nil {
				return ErrorResult(err), err
			}
			messageID, err := ReadStringParam(params, "messageId", true)
			if err != nil {
				return ErrorResult(err), err
			}
			if err := deps.DeleteMessage(ctx, channelID, messageID); err != nil {
				return ErrorResult(err), err
			}
			return OkResult(map[string]interface{}{"ok": true}), nil

		case "readMessages":
			channelID, err := ReadStringParam(params, "channelId", true)
			if err != nil {
				return ErrorResult(err), err
			}
			var limit int
			if v, ok := ReadIntParam(params, "limit"); ok {
				limit = v
			}
			before, _ := ReadStringParam(params, "before", false)
			after, _ := ReadStringParam(params, "after", false)
			threadID, _ := ReadStringParam(params, "threadId", false)

			result, err := deps.ReadMessages(ctx, channelID, SlackReadOpts{
				Limit:    limit,
				Before:   before,
				After:    after,
				ThreadID: threadID,
			})
			if err != nil {
				return ErrorResult(err), err
			}
			return OkResult(map[string]interface{}{"ok": true, "result": result}), nil
		}
	}

	// ── Pins ───────────────────────────────────────────────────────────
	if slackPinActions[action] {
		if !actionGate("pins") {
			return ToolResult{}, fmt.Errorf("Slack pins are disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}

		switch action {
		case "pinMessage":
			messageID, err := ReadStringParam(params, "messageId", true)
			if err != nil {
				return ErrorResult(err), err
			}
			if err := deps.PinMessage(ctx, channelID, messageID); err != nil {
				return ErrorResult(err), err
			}
			return OkResult(map[string]interface{}{"ok": true}), nil

		case "unpinMessage":
			messageID, err := ReadStringParam(params, "messageId", true)
			if err != nil {
				return ErrorResult(err), err
			}
			if err := deps.UnpinMessage(ctx, channelID, messageID); err != nil {
				return ErrorResult(err), err
			}
			return OkResult(map[string]interface{}{"ok": true}), nil

		case "listPins":
			pins, err := deps.ListPins(ctx, channelID)
			if err != nil {
				return ErrorResult(err), err
			}
			return OkResult(map[string]interface{}{"ok": true, "pins": pins}), nil
		}
	}

	// ── Member Info ────────────────────────────────────────────────────
	if action == "memberInfo" {
		if !actionGate("memberInfo") {
			return ToolResult{}, fmt.Errorf("Slack member info is disabled")
		}
		userID, err := ReadStringParam(params, "userId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		info, err := deps.GetMemberInfo(ctx, userID)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "info": info}), nil
	}

	// ── Emoji List ─────────────────────────────────────────────────────
	if action == "emojiList" {
		if !actionGate("emojiList") {
			return ToolResult{}, fmt.Errorf("Slack emoji list is disabled")
		}
		emojis, err := deps.ListEmojis(ctx)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "emojis": emojis}), nil
	}

	return ToolResult{}, fmt.Errorf("unsupported slack action: %s", action)
}
