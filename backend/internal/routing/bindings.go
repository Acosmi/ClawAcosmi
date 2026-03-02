// 对齐 TS: src/routing/bindings.ts (121L)
// Agent 路由绑定：列举绑定的账户 ID、解析默认 agent 绑定的账户。

package routing

import (
	"sort"
	"strings"

	"github.com/openacosmi/claw-acismi/internal/agents/scope"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// normalizeBindingChannelID 归一化 channel ID（小写+trim）。
// 对齐 TS normalizeBindingChannelId — 简化版，不做完整别名解析。
func normalizeBindingChannelID(raw string) string {
	normalized := strings.TrimSpace(strings.ToLower(raw))
	if normalized == "" {
		return ""
	}
	return normalized
}

// listBindings 安全地获取 config 中的 bindings 列表。
func listBindings(cfg *types.OpenAcosmiConfig) []types.AgentBinding {
	if cfg == nil {
		return nil
	}
	return cfg.Bindings
}

// ListBoundAccountIds 列出指定 channel 中所有绑定的账户 ID（去重+排序）。
// 对齐 TS: listBoundAccountIds(cfg, channelId)
func ListBoundAccountIds(cfg *types.OpenAcosmiConfig, channelID string) []string {
	normalizedChannel := normalizeBindingChannelID(channelID)
	if normalizedChannel == "" {
		return nil
	}
	seen := make(map[string]bool)
	var ids []string
	for _, binding := range listBindings(cfg) {
		channel := normalizeBindingChannelID(binding.Match.Channel)
		if channel == "" || channel != normalizedChannel {
			continue
		}
		accountID := strings.TrimSpace(binding.Match.AccountID)
		if accountID == "" || accountID == "*" {
			continue
		}
		normalized := NormalizeAccountID(accountID)
		if !seen[normalized] {
			seen[normalized] = true
			ids = append(ids, normalized)
		}
	}
	sort.Strings(ids)
	return ids
}

// ResolveDefaultAgentBoundAccountId 解析默认 agent 在指定 channel 中绑定的账户 ID。
// 返回空字符串表示无绑定。
// 对齐 TS: resolveDefaultAgentBoundAccountId(cfg, channelId)
func ResolveDefaultAgentBoundAccountId(cfg *types.OpenAcosmiConfig, channelID string) string {
	normalizedChannel := normalizeBindingChannelID(channelID)
	if normalizedChannel == "" {
		return ""
	}
	defaultAgentID := scope.NormalizeAgentId(scope.ResolveDefaultAgentId(cfg))
	for _, binding := range listBindings(cfg) {
		if scope.NormalizeAgentId(binding.AgentID) != defaultAgentID {
			continue
		}
		channel := normalizeBindingChannelID(binding.Match.Channel)
		if channel == "" || channel != normalizedChannel {
			continue
		}
		accountID := strings.TrimSpace(binding.Match.AccountID)
		if accountID == "" || accountID == "*" {
			continue
		}
		return NormalizeAccountID(accountID)
	}
	return ""
}
