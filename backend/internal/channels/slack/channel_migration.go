package slack

import (
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// Slack 频道 ID 迁移 — 继承自 src/slack/channel-migration.ts (102L)

// SlackChannelMigrationResult 频道迁移结果
type SlackChannelMigrationResult struct {
	Migrated        bool
	SkippedExisting bool
	Scopes          []string // "account" | "global"
}

// MigrateSlackChannelsInPlace 在 channels map 中将旧 ID 迁移为新 ID。
func MigrateSlackChannelsInPlace(channels map[string]*types.SlackChannelConfig, oldChannelID, newChannelID string) (migrated, skippedExisting bool) {
	if channels == nil {
		return false, false
	}
	if oldChannelID == newChannelID {
		return false, false
	}
	if _, ok := channels[oldChannelID]; !ok {
		return false, false
	}
	if _, ok := channels[newChannelID]; ok {
		return false, true
	}
	channels[newChannelID] = channels[oldChannelID]
	delete(channels, oldChannelID)
	return true, false
}

// MigrateSlackChannelConfig 迁移 Slack 频道配置（账户级 + 全局级）。
func MigrateSlackChannelConfig(cfg *types.OpenAcosmiConfig, accountID, oldChannelID, newChannelID string) SlackChannelMigrationResult {
	var scopes []string
	var migrated, skippedExisting bool

	// 账户级
	accountChannels := resolveAccountChannels(cfg, accountID)
	if accountChannels != nil {
		m, s := MigrateSlackChannelsInPlace(accountChannels, oldChannelID, newChannelID)
		if m {
			migrated = true
			scopes = append(scopes, "account")
		}
		if s {
			skippedExisting = true
		}
	}

	// 全局级
	if cfg.Channels != nil && cfg.Channels.Slack != nil && cfg.Channels.Slack.Channels != nil {
		m, s := MigrateSlackChannelsInPlace(cfg.Channels.Slack.Channels, oldChannelID, newChannelID)
		if m {
			migrated = true
			scopes = append(scopes, "global")
		}
		if s {
			skippedExisting = true
		}
	}

	return SlackChannelMigrationResult{
		Migrated:        migrated,
		SkippedExisting: skippedExisting,
		Scopes:          scopes,
	}
}

// resolveAccountChannels 获取指定账户的频道配置
func resolveAccountChannels(cfg *types.OpenAcosmiConfig, accountID string) map[string]*types.SlackChannelConfig {
	if accountID == "" {
		return nil
	}
	normalized := NormalizeAccountID(accountID)
	if cfg.Channels == nil || cfg.Channels.Slack == nil {
		return nil
	}
	accounts := cfg.Channels.Slack.Accounts
	if len(accounts) == 0 {
		return nil
	}
	// 精确匹配
	if acct, ok := accounts[normalized]; ok && acct != nil {
		return acct.Channels
	}
	// 大小写不敏感匹配
	for key, acct := range accounts {
		if strings.EqualFold(key, normalized) && acct != nil {
			return acct.Channels
		}
	}
	return nil
}
