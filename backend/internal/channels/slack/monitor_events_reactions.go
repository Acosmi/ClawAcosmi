package slack

// Slack 监控事件：反应 — 继承自 src/slack/monitor/events/reactions.ts (73L)

import (
	"context"
	"fmt"
	"log"
)

// HandleSlackReactionAddedEvent 处理反应添加事件。
func HandleSlackReactionAddedEvent(ctx context.Context, monCtx *SlackMonitorContext, event SlackReactionEvent) {
	channelID := event.Item.Channel
	if channelID == "" {
		return
	}

	// 检查是否应发送通知
	if !ShouldEmitSlackReactionNotification(
		monCtx.Account.ReactionNotifications,
		monCtx.BotUserID,
		event.ItemUser,
		event.User,
		monCtx.ResolveUserName(event.User),
		monCtx.ReactionAllowlist,
	) {
		return
	}

	userName := monCtx.ResolveUserName(event.User)
	text := fmt.Sprintf(":%s: reaction from %s", event.Reaction, userName)

	sessionKey := monCtx.ResolveSystemEventSessionKey(channelID, InferSlackChannelType(channelID))
	if monCtx.Deps != nil && monCtx.Deps.EnqueueSystemEvent != nil {
		_ = monCtx.Deps.EnqueueSystemEvent(text, sessionKey, "slack:reaction")
	}

	log.Printf("[slack:%s] reaction added: :%s: by %s in %s",
		monCtx.AccountID, event.Reaction, event.User, channelID)
}

// HandleSlackReactionRemovedEvent 处理反应移除事件。
func HandleSlackReactionRemovedEvent(ctx context.Context, monCtx *SlackMonitorContext, event SlackReactionEvent) {
	channelID := event.Item.Channel
	if channelID == "" {
		return
	}

	log.Printf("[slack:%s] reaction removed: :%s: by %s in %s",
		monCtx.AccountID, event.Reaction, event.User, channelID)
}
