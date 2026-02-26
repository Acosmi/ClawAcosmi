package slack

// Slack 监控事件：频道 — 继承自 src/slack/monitor/events/channels.ts (161L)
// Phase 9 实现：缓存刷新 + channel_id_changed 迁移。

import (
	"context"
	"log"
)

// HandleSlackChannelRenameEvent 处理频道重命名事件。
func HandleSlackChannelRenameEvent(ctx context.Context, monCtx *SlackMonitorContext, event SlackChannelEvent) {
	chID := event.Channel.ID
	chName := event.Channel.Name
	if chID == "" {
		return
	}
	monCtx.CacheChannelName(chID, chName)
	log.Printf("[slack:%s] channel renamed: %s → %s", monCtx.AccountID, chID, chName)
}

// HandleSlackChannelArchiveEvent 处理频道归档事件。
func HandleSlackChannelArchiveEvent(ctx context.Context, monCtx *SlackMonitorContext, event SlackChannelEvent) {
	chID := event.Channel.ID
	if chID == "" {
		return
	}
	log.Printf("[slack:%s] channel archived: %s", monCtx.AccountID, chID)
}

// HandleSlackChannelUnarchiveEvent 处理频道解归档事件。
func HandleSlackChannelUnarchiveEvent(ctx context.Context, monCtx *SlackMonitorContext, event SlackChannelEvent) {
	chID := event.Channel.ID
	if chID == "" {
		return
	}
	log.Printf("[slack:%s] channel unarchived: %s", monCtx.AccountID, chID)
}

// HandleSlackChannelDeletedEvent 处理频道删除事件。
func HandleSlackChannelDeletedEvent(ctx context.Context, monCtx *SlackMonitorContext, event SlackChannelEvent) {
	chID := event.Channel.ID
	if chID == "" {
		return
	}
	monCtx.RemoveChannelFromCache(chID)
	log.Printf("[slack:%s] channel deleted: %s", monCtx.AccountID, chID)
}

// HandleSlackChannelCreatedEvent 处理频道创建事件。
func HandleSlackChannelCreatedEvent(ctx context.Context, monCtx *SlackMonitorContext, event SlackChannelEvent) {
	chID := event.Channel.ID
	chName := event.Channel.Name
	if chID == "" {
		return
	}
	if chName != "" {
		monCtx.CacheChannelName(chID, chName)
	}
	log.Printf("[slack:%s] channel created: %s (%s)", monCtx.AccountID, chID, chName)
}

// HandleSlackChannelIDChangedEvent 处理频道 ID 变更事件。
func HandleSlackChannelIDChangedEvent(ctx context.Context, monCtx *SlackMonitorContext, event SlackChannelEvent) {
	oldID := event.OldChannelID
	newID := event.NewChannelID
	if oldID == "" || newID == "" {
		return
	}

	// 迁移缓存
	oldName := monCtx.ResolveChannelName(oldID)
	monCtx.RemoveChannelFromCache(oldID)
	if oldName != oldID {
		monCtx.CacheChannelName(newID, oldName)
	}

	// 迁移频道配置
	result := MigrateSlackChannelConfig(monCtx.CFG, monCtx.AccountID, oldID, newID)
	log.Printf("[slack:%s] channel ID changed: %s → %s (migrated=%v)",
		monCtx.AccountID, oldID, newID, result.Migrated)
}
