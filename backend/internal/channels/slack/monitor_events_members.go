package slack

// Slack 监控事件：成员 — 继承自 src/slack/monitor/events/members.ts (90L)

import (
	"context"
	"log"
)

// HandleSlackMemberJoinedChannelEvent 处理成员加入频道事件。
func HandleSlackMemberJoinedChannelEvent(ctx context.Context, monCtx *SlackMonitorContext, event SlackMemberChannelEvent) {
	log.Printf("[slack:%s] member joined channel %s: user=%s",
		monCtx.AccountID, event.Channel, event.User)
}

// HandleSlackMemberLeftChannelEvent 处理成员离开频道事件。
func HandleSlackMemberLeftChannelEvent(ctx context.Context, monCtx *SlackMonitorContext, event SlackMemberChannelEvent) {
	log.Printf("[slack:%s] member left channel %s: user=%s",
		monCtx.AccountID, event.Channel, event.User)
}
