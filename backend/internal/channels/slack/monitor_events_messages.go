package slack

// Slack 监控事件：消息 — 继承自 src/slack/monitor/events/messages.ts (135L)
// Phase 9 实现：Socket Mode / HTTP 消息事件路由。

import (
	"context"
	"log"
	"strings"
)

// HandleSlackMessageEvent 处理 Slack message 事件。
// 过滤子类型 → 去重 → PrepareSlackInboundMessage → DispatchSlackMessage。
func HandleSlackMessageEvent(ctx context.Context, monCtx *SlackMonitorContext, event SlackMessageEvent) {
	// 过滤非标准消息子类型
	if shouldDropSlackMessageSubtype(event.Subtype) {
		return
	}

	// 去重
	if monCtx.MarkMessageSeen(event.Channel, event.Ts) {
		return
	}

	// 预处理
	msg, skipReason := PrepareSlackInboundMessage(ctx, monCtx, event)
	if skipReason != "" {
		log.Printf("[slack:%s] message skipped: %s", monCtx.AccountID, skipReason)
		return
	}
	if msg == nil {
		return
	}

	// 分发
	DispatchSlackMessage(ctx, monCtx, msg)
}

// HandleSlackAppMentionEvent 处理 Slack app_mention 事件。
// 转换为 SlackMessageEvent 统一处理。
func HandleSlackAppMentionEvent(ctx context.Context, monCtx *SlackMonitorContext, event SlackAppMentionEvent) {
	// 转为标准消息事件处理
	msgEvent := SlackMessageEvent{
		Type:        "message",
		User:        event.User,
		BotID:       event.BotID,
		Username:    event.Username,
		Text:        event.Text,
		Ts:          event.Ts,
		ThreadTs:    event.ThreadTs,
		EventTs:     event.EventTs,
		Channel:     event.Channel,
		ChannelType: event.ChannelType,
	}
	HandleSlackMessageEvent(ctx, monCtx, msgEvent)
}

// shouldDropSlackMessageSubtype 判断是否应丢弃消息子类型。
func shouldDropSlackMessageSubtype(subtype string) bool {
	if subtype == "" {
		return false // 标准消息
	}
	// 保留的子类型
	switch subtype {
	case "file_share", "thread_broadcast", "me_message":
		return false
	}
	// 丢弃的子类型
	switch subtype {
	case "message_changed", "message_deleted", "message_replied",
		"channel_join", "channel_leave", "channel_topic",
		"channel_purpose", "channel_name", "bot_message",
		"ekm_access_denied", "group_join", "group_leave",
		"group_topic", "group_purpose", "group_name":
		return true
	}
	// 未知子类型：保留（保守策略）
	return !strings.HasPrefix(subtype, "file_")
}
