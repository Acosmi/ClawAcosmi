package slack

// Slack 监控事件：固定消息 — 继承自 src/slack/monitor/events/pins.ts (88L)

import (
	"context"
	"fmt"
	"log"
)

// HandleSlackPinAddedEvent 处理消息固定事件。
func HandleSlackPinAddedEvent(ctx context.Context, monCtx *SlackMonitorContext, event SlackPinEvent) {
	channelID := event.Channel
	if channelID == "" && event.Item.Channel != "" {
		channelID = event.Item.Channel
	}
	if channelID == "" {
		return
	}

	// 构建系统事件文本
	var pinText string
	if event.Item.Message != nil && event.Item.Message.Text != "" {
		pinText = truncateText(event.Item.Message.Text, 80)
	}

	text := fmt.Sprintf("📌 Pin added by %s", event.User)
	if pinText != "" {
		text += fmt.Sprintf(": %s", pinText)
	}

	sessionKey := monCtx.ResolveSystemEventSessionKey(channelID, InferSlackChannelType(channelID))
	if monCtx.Deps != nil && monCtx.Deps.EnqueueSystemEvent != nil {
		_ = monCtx.Deps.EnqueueSystemEvent(text, sessionKey, "slack:pin")
	}

	log.Printf("[slack:%s] pin added in %s by %s", monCtx.AccountID, channelID, event.User)
}

// HandleSlackPinRemovedEvent 处理取消固定事件。
func HandleSlackPinRemovedEvent(ctx context.Context, monCtx *SlackMonitorContext, event SlackPinEvent) {
	channelID := event.Channel
	if channelID == "" && event.Item.Channel != "" {
		channelID = event.Item.Channel
	}
	if channelID == "" {
		return
	}

	text := fmt.Sprintf("📌 Pin removed by %s", event.User)
	sessionKey := monCtx.ResolveSystemEventSessionKey(channelID, InferSlackChannelType(channelID))
	if monCtx.Deps != nil && monCtx.Deps.EnqueueSystemEvent != nil {
		_ = monCtx.Deps.EnqueueSystemEvent(text, sessionKey, "slack:pin")
	}

	log.Printf("[slack:%s] pin removed in %s by %s", monCtx.AccountID, channelID, event.User)
}

// truncateText 截断文本用于日志/通知。
func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}
