package discord

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// Discord 审计 — 继承自 src/discord/audit.ts (140L)

// DiscordChannelPermissionsAuditEntry 单个频道权限审计结果
type DiscordChannelPermissionsAuditEntry struct {
	ChannelID   string   `json:"channelId"`
	OK          bool     `json:"ok"`
	Missing     []string `json:"missing,omitempty"`
	Error       string   `json:"error,omitempty"`
	MatchKey    string   `json:"matchKey,omitempty"`
	MatchSource string   `json:"matchSource,omitempty"`
}

// DiscordChannelPermissionsAudit 频道权限审计报告
type DiscordChannelPermissionsAudit struct {
	OK                 bool                                  `json:"ok"`
	CheckedChannels    int                                   `json:"checkedChannels"`
	UnresolvedChannels int                                   `json:"unresolvedChannels"`
	Channels           []DiscordChannelPermissionsAuditEntry `json:"channels"`
	ElapsedMs          int64                                 `json:"elapsedMs"`
}

// requiredChannelPermissions 需要检查的频道权限
var requiredChannelPermissions = []string{"ViewChannel", "SendMessages"}

// shouldAuditChannelConfig 判断频道配置是否需要审计
func shouldAuditChannelConfig(config *types.DiscordGuildChannelConfig) bool {
	if config == nil {
		return true
	}
	if config.Allow != nil && !*config.Allow {
		return false
	}
	if config.Enabled != nil && !*config.Enabled {
		return false
	}
	return true
}

// listConfiguredGuildChannelKeys 从 guilds 配置中提取频道 key
func listConfiguredGuildChannelKeys(guilds map[string]*types.DiscordGuildEntry) []string {
	if len(guilds) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	for _, entry := range guilds {
		if entry == nil || len(entry.Channels) == 0 {
			continue
		}
		for key, value := range entry.Channels {
			channelID := strings.TrimSpace(key)
			if channelID == "" {
				continue
			}
			if !shouldAuditChannelConfig(value) {
				continue
			}
			seen[channelID] = struct{}{}
		}
	}
	var ids []string
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// isNumeric 判断字符串是否为纯数字
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// CollectDiscordAuditChannelIds 收集需要审计的频道 ID
func CollectDiscordAuditChannelIds(cfg *types.OpenAcosmiConfig, accountID string) (channelIds []string, unresolvedChannels int) {
	account := ResolveDiscordAccount(cfg, accountID)
	keys := listConfiguredGuildChannelKeys(account.Config.Guilds)
	for _, key := range keys {
		if isNumeric(key) {
			channelIds = append(channelIds, key)
		} else {
			unresolvedChannels++
		}
	}
	return
}

// AuditDiscordChannelPermissions 审计频道权限。
// fetchPermsFn 用于获取频道权限（注入以解耦对 send 包的依赖）。
// Phase 9: fetchPermsFn 通过 DI 参数注入，调用方传入 fetchChannelPermissionsDiscord 实现。
func AuditDiscordChannelPermissions(
	ctx context.Context,
	token string,
	channelIds []string,
	accountID string,
	fetchPermsFn func(ctx context.Context, channelID string, token string, accountID string) ([]string, error),
) DiscordChannelPermissionsAudit {
	started := time.Now()
	trimmedToken := strings.TrimSpace(token)

	if trimmedToken == "" || len(channelIds) == 0 {
		return DiscordChannelPermissionsAudit{
			OK:              true,
			CheckedChannels: 0,
			Channels:        []DiscordChannelPermissionsAuditEntry{},
			ElapsedMs:       time.Since(started).Milliseconds(),
		}
	}

	channels := []DiscordChannelPermissionsAuditEntry{}
	for _, channelID := range channelIds {
		entry := DiscordChannelPermissionsAuditEntry{
			ChannelID:   channelID,
			MatchKey:    channelID,
			MatchSource: "id",
		}

		if fetchPermsFn == nil {
			entry.OK = false
			entry.Error = "permission check function not provided"
			channels = append(channels, entry)
			continue
		}

		perms, err := fetchPermsFn(ctx, channelID, trimmedToken, accountID)
		if err != nil {
			entry.OK = false
			entry.Error = err.Error()
			channels = append(channels, entry)
			continue
		}

		permSet := make(map[string]struct{}, len(perms))
		for _, p := range perms {
			permSet[p] = struct{}{}
		}
		var missing []string
		for _, req := range requiredChannelPermissions {
			if _, ok := permSet[req]; !ok {
				missing = append(missing, req)
			}
		}
		entry.OK = len(missing) == 0
		if len(missing) > 0 {
			entry.Missing = missing
		}
		channels = append(channels, entry)
	}

	allOK := true
	for _, c := range channels {
		if !c.OK {
			allOK = false
			break
		}
	}

	return DiscordChannelPermissionsAudit{
		OK:              allOK,
		CheckedChannels: len(channels),
		Channels:        channels,
		ElapsedMs:       time.Since(started).Milliseconds(),
	}
}
