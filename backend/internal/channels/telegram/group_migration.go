package telegram

import (
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// Telegram 群组迁移 — 继承自 src/telegram/group-migration.ts (95L)

// GroupMigrationResult 群组迁移结果
type GroupMigrationResult struct {
	Migrated        bool
	SkippedExisting bool
	Scopes          []string // "account", "global"
}

// MigrateTelegramGroupsInPlace 原地将 oldChatId 的配置迁移到 newChatId。
func MigrateTelegramGroupsInPlace(groups map[string]*types.TelegramGroupConfig, oldChatID, newChatID string) (migrated, skippedExisting bool) {
	if groups == nil || oldChatID == newChatID {
		return false, false
	}
	if _, exists := groups[oldChatID]; !exists {
		return false, false
	}
	if _, exists := groups[newChatID]; exists {
		return false, true
	}
	groups[newChatID] = groups[oldChatID]
	delete(groups, oldChatID)
	return true, false
}

// MigrateTelegramGroupConfig 迁移 Telegram 群组配置（账户级 + 全局级）。
func MigrateTelegramGroupConfig(cfg *types.OpenAcosmiConfig, accountID, oldChatID, newChatID string) GroupMigrationResult {
	var result GroupMigrationResult

	// 账户级群组
	if accountID != "" && cfg.Channels != nil && cfg.Channels.Telegram != nil {
		normalized := NormalizeAccountID(accountID)
		acct := cfg.Channels.Telegram.Accounts[normalized]
		// 对齐 TS: 精确匹配失败时按 toLowerCase 回退查找
		if acct == nil {
			normalizedLower := strings.ToLower(normalized)
			for key, val := range cfg.Channels.Telegram.Accounts {
				if strings.ToLower(key) == normalizedLower {
					acct = val
					break
				}
			}
		}
		if acct != nil {
			m, s := MigrateTelegramGroupsInPlace(acct.Groups, oldChatID, newChatID)
			if m {
				result.Migrated = true
				result.Scopes = append(result.Scopes, "account")
			}
			if s {
				result.SkippedExisting = true
			}
		}
	}

	// 全局级群组
	if cfg.Channels != nil && cfg.Channels.Telegram != nil {
		m, s := MigrateTelegramGroupsInPlace(cfg.Channels.Telegram.Groups, oldChatID, newChatID)
		if m {
			result.Migrated = true
			result.Scopes = append(result.Scopes, "global")
		}
		if s {
			result.SkippedExisting = true
		}
	}

	return result
}
