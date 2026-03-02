package config

// 频道群组策略模块 — 对应 src/config/group-policy.ts (214行)
//
// 解析频道群组的访问策略、@mention 要求、工具策略等。
// 支持 allowlist 模式、通配符 "*"、按 sender 的差异化工具策略。

import (
	"fmt"
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ChannelGroupConfig 频道群组配置
type ChannelGroupConfig struct {
	RequireMention *bool                               `json:"requireMention,omitempty"`
	Tools          *types.GroupToolPolicyConfig        `json:"tools,omitempty"`
	ToolsBySender  types.GroupToolPolicyBySenderConfig `json:"toolsBySender,omitempty"`
}

// ChannelGroupPolicy 群组策略解析结果
type ChannelGroupPolicy struct {
	AllowlistEnabled bool
	Allowed          bool
	GroupConfig      *ChannelGroupConfig
	DefaultConfig    *ChannelGroupConfig
}

// GroupToolPolicySender 发送者信息
type GroupToolPolicySender struct {
	SenderID       string
	SenderName     string
	SenderUsername string
	SenderE164     string
}

// normalizeSenderKey 规范化发送者键
// 对应 TS group-policy.ts:L30-L37
func normalizeSenderKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimPrefix(trimmed, "@")
	return strings.ToLower(trimmed)
}

// ResolveToolsBySender 按发送者查找工具策略
// 对应 TS group-policy.ts:L39-L96
func ResolveToolsBySender(toolsBySender types.GroupToolPolicyBySenderConfig, sender GroupToolPolicySender) *types.GroupToolPolicyConfig {
	if len(toolsBySender) == 0 {
		return nil
	}

	normalized := make(map[string]*types.GroupToolPolicyConfig)
	var wildcard *types.GroupToolPolicyConfig

	for rawKey, policy := range toolsBySender {
		if policy == nil {
			continue
		}
		key := normalizeSenderKey(rawKey)
		if key == "" {
			continue
		}
		if key == "*" {
			wildcard = policy
			continue
		}
		if _, exists := normalized[key]; !exists {
			normalized[key] = policy
		}
	}

	// 按优先级尝试匹配: ID → E164 → Username → Name
	candidates := []string{}
	if s := strings.TrimSpace(sender.SenderID); s != "" {
		candidates = append(candidates, s)
	}
	if s := strings.TrimSpace(sender.SenderE164); s != "" {
		candidates = append(candidates, s)
	}
	if s := strings.TrimSpace(sender.SenderUsername); s != "" {
		candidates = append(candidates, s)
	}
	if s := strings.TrimSpace(sender.SenderName); s != "" {
		candidates = append(candidates, s)
	}

	for _, candidate := range candidates {
		key := normalizeSenderKey(candidate)
		if key == "" {
			continue
		}
		if match, ok := normalized[key]; ok {
			return match
		}
	}

	return wildcard
}

// ChannelGroups 频道群组映射
type ChannelGroups map[string]*ChannelGroupConfig

// normalizeAccountId 规范化账号 ID
// 对应 TS routing/session-key.ts normalizeAccountId
func normalizeAccountId(accountID string) string {
	return strings.ToLower(strings.TrimSpace(accountID))
}

// telegramGroupsToChannelGroups 将 TelegramGroupConfig map 转为 ChannelGroups
func telegramGroupsToChannelGroups(groups map[string]*types.TelegramGroupConfig) ChannelGroups {
	if len(groups) == 0 {
		return nil
	}
	result := make(ChannelGroups, len(groups))
	for id, g := range groups {
		if g == nil {
			continue
		}
		result[id] = &ChannelGroupConfig{
			RequireMention: g.RequireMention,
			Tools:          g.Tools,
			ToolsBySender:  g.ToolsBySender,
		}
	}
	return result
}

// resolveChannelGroups 从配置中解析频道群组
// 对应 TS group-policy.ts:L98-L121
// 优先从 accounts[accountId].groups 获取，回退到顶层 groups。
func resolveChannelGroups(cfg *types.OpenAcosmiConfig, channel, accountID string) ChannelGroups {
	if cfg.Channels == nil {
		return nil
	}
	normalizedAcct := normalizeAccountId(accountID)

	switch channel {
	case "telegram":
		tg := cfg.Channels.Telegram
		if tg == nil {
			return nil
		}
		// 优先从 accounts[accountId].groups 获取
		if normalizedAcct != "" && tg.Accounts != nil {
			// 精确匹配
			if acct := tg.Accounts[normalizedAcct]; acct != nil && len(acct.Groups) > 0 {
				return telegramGroupsToChannelGroups(acct.Groups)
			}
			// 大小写不敏感匹配
			for key, acct := range tg.Accounts {
				if strings.ToLower(key) == normalizedAcct && acct != nil && len(acct.Groups) > 0 {
					return telegramGroupsToChannelGroups(acct.Groups)
				}
			}
		}
		// 回退到顶层 groups（通过嵌入的 TelegramAccountConfig）
		return telegramGroupsToChannelGroups(tg.Groups)

	case "discord":
		dc := cfg.Channels.Discord
		if dc == nil {
			return nil
		}
		// Discord 使用 guilds → channels 结构
		// 将 guild.channels 映射到 ChannelGroups
		return resolveDiscordChannelGroups(dc, normalizedAcct)

	case "slack":
		sl := cfg.Channels.Slack
		if sl == nil {
			return nil
		}
		// Slack 使用 channels 结构
		return resolveSlackChannelGroups(sl, normalizedAcct)

	case "signal":
		sg := cfg.Channels.Signal
		if sg == nil {
			return nil
		}
		// Signal 使用简单的 groupAllowFrom 模式，无嵌套 group 配置
		// 但支持 groupPolicy 和 requireMention
		return resolveSignalChannelGroups(sg, normalizedAcct)

	default:
		return nil
	}
}

// ResolveChannelGroupPolicy 解析频道群组策略
// 对应 TS group-policy.ts:L123-L146
func ResolveChannelGroupPolicy(cfg *types.OpenAcosmiConfig, channel string, groupID string, accountID string) ChannelGroupPolicy {
	groups := resolveChannelGroups(cfg, channel, accountID)

	allowlistEnabled := len(groups) > 0
	normalizedID := strings.TrimSpace(groupID)

	var groupConfig *ChannelGroupConfig
	if normalizedID != "" && groups != nil {
		groupConfig = groups[normalizedID]
	}

	var defaultConfig *ChannelGroupConfig
	if groups != nil {
		defaultConfig = groups["*"]
	}

	_, hasWildcard := groups["*"]
	allowAll := allowlistEnabled && hasWildcard

	allowed := !allowlistEnabled || allowAll
	if !allowed && normalizedID != "" && groups != nil {
		_, allowed = groups[normalizedID]
	}

	return ChannelGroupPolicy{
		AllowlistEnabled: allowlistEnabled,
		Allowed:          allowed,
		GroupConfig:      groupConfig,
		DefaultConfig:    defaultConfig,
	}
}

// ResolveChannelGroupRequireMention 解析群组是否需要 @mention
// 对应 TS group-policy.ts:L148-L175
func ResolveChannelGroupRequireMention(
	cfg *types.OpenAcosmiConfig,
	channel string,
	groupID string,
	accountID string,
	requireMentionOverride *bool,
	overrideOrder string, // "before-config" or "after-config"(default)
) bool {
	if overrideOrder == "" {
		overrideOrder = "after-config"
	}

	policy := ResolveChannelGroupPolicy(cfg, channel, groupID, accountID)

	// 从配置中提取 requireMention
	var configMention *bool
	if policy.GroupConfig != nil && policy.GroupConfig.RequireMention != nil {
		configMention = policy.GroupConfig.RequireMention
	} else if policy.DefaultConfig != nil && policy.DefaultConfig.RequireMention != nil {
		configMention = policy.DefaultConfig.RequireMention
	}

	// before-config: override 优先
	if overrideOrder == "before-config" && requireMentionOverride != nil {
		return *requireMentionOverride
	}

	// 配置值存在则使用
	if configMention != nil {
		return *configMention
	}

	// after-config: override 兜底
	if overrideOrder != "before-config" && requireMentionOverride != nil {
		return *requireMentionOverride
	}

	// 最终默认需要 mention
	return true
}

// ResolveChannelGroupToolsPolicy 解析群组工具策略
// 对应 TS group-policy.ts:L177-L213
// 查找优先级: 群组 sender 策略 → 群组工具策略 → 默认 sender 策略 → 默认工具策略
func ResolveChannelGroupToolsPolicy(
	cfg *types.OpenAcosmiConfig,
	channel string,
	groupID string,
	accountID string,
	sender GroupToolPolicySender,
) *types.GroupToolPolicyConfig {
	policy := ResolveChannelGroupPolicy(cfg, channel, groupID, accountID)

	// 1. 群组特定 sender 策略
	if policy.GroupConfig != nil {
		if senderPolicy := ResolveToolsBySender(policy.GroupConfig.ToolsBySender, sender); senderPolicy != nil {
			return senderPolicy
		}
	}

	// 2. 群组工具策略
	if policy.GroupConfig != nil && policy.GroupConfig.Tools != nil {
		return policy.GroupConfig.Tools
	}

	// 3. 默认 sender 策略
	if policy.DefaultConfig != nil {
		if senderPolicy := ResolveToolsBySender(policy.DefaultConfig.ToolsBySender, sender); senderPolicy != nil {
			return senderPolicy
		}
	}

	// 4. 默认工具策略
	if policy.DefaultConfig != nil && policy.DefaultConfig.Tools != nil {
		return policy.DefaultConfig.Tools
	}

	return nil
}

// ----- Discord 群组解析 (C1) -----

// discordGuildToChannelGroupConfig 将 Discord Guild 配置转换为通用 ChannelGroupConfig
func discordGuildToChannelGroupConfig(guild *types.DiscordGuildEntry) *ChannelGroupConfig {
	if guild == nil {
		return nil
	}
	return &ChannelGroupConfig{
		RequireMention: guild.RequireMention,
		Tools:          guild.Tools,
		ToolsBySender:  guild.ToolsBySender,
	}
}

// resolveDiscordChannelGroups 从 Discord 配置中解析频道群组
// Discord 使用 guilds → channels 两级结构:
//
//	guilds: { "guild-id": { channels: { "channel-id": { requireMention, tools... } } } }
//
// 将 guild channels 扁平化为 ChannelGroups
func resolveDiscordChannelGroups(dc *types.DiscordConfig, normalizedAcct string) ChannelGroups {
	// 优先从 accounts[accountId] 获取
	acct := resolveDiscordAccount(dc, normalizedAcct)
	if acct == nil {
		return nil
	}

	if len(acct.Guilds) == 0 {
		return nil
	}

	result := make(ChannelGroups)
	for guildID, guild := range acct.Guilds {
		if guild == nil {
			continue
		}
		// Guild 本身作为一个 group entry
		result[guildID] = discordGuildToChannelGroupConfig(guild)

		// Guild 的 channels 子级也展开
		for channelID, channelCfg := range guild.Channels {
			if channelCfg == nil {
				continue
			}
			key := guildID + ":" + channelID
			result[key] = &ChannelGroupConfig{
				RequireMention: channelCfg.RequireMention,
				Tools:          channelCfg.Tools,
				ToolsBySender:  channelCfg.ToolsBySender,
			}
		}
	}

	return result
}

// resolveDiscordAccount 查找 Discord 账号配置
func resolveDiscordAccount(dc *types.DiscordConfig, normalizedAcct string) *types.DiscordAccountConfig {
	if normalizedAcct != "" && dc.Accounts != nil {
		if acct := dc.Accounts[normalizedAcct]; acct != nil {
			return acct
		}
		for key, acct := range dc.Accounts {
			if strings.ToLower(key) == normalizedAcct && acct != nil {
				return acct
			}
		}
	}
	// 回退到嵌入的顶层 DiscordAccountConfig
	return &dc.DiscordAccountConfig
}

// ----- Slack 群组解析 (C1) -----

// resolveSlackChannelGroups 从 Slack 配置中解析频道群组
// Slack 使用 channels 扁平结构:
//
//	channels: { "channel-id": { requireMention, tools... } }
func resolveSlackChannelGroups(sl *types.SlackConfig, normalizedAcct string) ChannelGroups {
	acct := resolveSlackAccount(sl, normalizedAcct)
	if acct == nil {
		return nil
	}

	if len(acct.Channels) == 0 {
		return nil
	}

	result := make(ChannelGroups, len(acct.Channels))
	for channelID, channelCfg := range acct.Channels {
		if channelCfg == nil {
			continue
		}
		result[channelID] = &ChannelGroupConfig{
			RequireMention: channelCfg.RequireMention,
			Tools:          channelCfg.Tools,
			ToolsBySender:  channelCfg.ToolsBySender,
		}
	}

	return result
}

// resolveSlackAccount 查找 Slack 账号配置
func resolveSlackAccount(sl *types.SlackConfig, normalizedAcct string) *types.SlackAccountConfig {
	if normalizedAcct != "" && sl.Accounts != nil {
		if acct := sl.Accounts[normalizedAcct]; acct != nil {
			return acct
		}
		for key, acct := range sl.Accounts {
			if strings.ToLower(key) == normalizedAcct && acct != nil {
				return acct
			}
		}
	}
	return &sl.SlackAccountConfig
}

// ----- Signal 群组解析 (C1) -----

// resolveSignalChannelGroups 从 Signal 配置中解析频道群组
// Signal 没有嵌套的 groups 配置，使用 groupAllowFrom 作为允许列表。
// 如果设置了 groupAllowFrom，则作为 allowlist 模式的占位群组返回。
func resolveSignalChannelGroups(sg *types.SignalConfig, normalizedAcct string) ChannelGroups {
	acct := resolveSignalAccount(sg, normalizedAcct)
	if acct == nil {
		return nil
	}

	// Signal 没有 groups map，但有 groupAllowFrom 列表
	if len(acct.GroupAllowFrom) == 0 {
		return nil
	}

	result := make(ChannelGroups)
	// 将 groupAllowFrom 中的每个 ID 作为允许的群组
	for _, groupIDRaw := range acct.GroupAllowFrom {
		var groupID string
		switch v := groupIDRaw.(type) {
		case string:
			groupID = v
		case float64:
			groupID = strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", v), "0"), ".")
		default:
			continue
		}
		if groupID == "" {
			continue
		}
		// 每个允许的群组获得一个空的 ChannelGroupConfig
		result[groupID] = &ChannelGroupConfig{}
	}

	// 添加通配符如果 groupPolicy 是 "all"
	if acct.GroupPolicy == "all" {
		result["*"] = &ChannelGroupConfig{}
	}

	return result
}

// resolveSignalAccount 查找 Signal 账号配置
func resolveSignalAccount(sg *types.SignalConfig, normalizedAcct string) *types.SignalAccountConfig {
	if normalizedAcct != "" && sg.Accounts != nil {
		if acct := sg.Accounts[normalizedAcct]; acct != nil {
			return acct
		}
		for key, acct := range sg.Accounts {
			if strings.ToLower(key) == normalizedAcct && acct != nil {
				return acct
			}
		}
	}
	return &sg.SignalAccountConfig
}
