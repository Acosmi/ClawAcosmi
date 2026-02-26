package discord

import (
	"regexp"
	"strings"
)

// Discord 允许列表 — 继承自 src/discord/monitor/allow-list.ts (454L)
// 仅移植纯逻辑型函数，gateway 依赖的部分延迟到 Phase 7

// DiscordAllowList 允许列表
type DiscordAllowList struct {
	AllowAll bool
	IDs      map[string]bool
	Names    map[string]bool
}

// DiscordAllowListMatch 匹配结果
type DiscordAllowListMatch struct {
	Allowed bool
	ByID    bool
	ByName  bool
}

// DiscordGuildEntryResolved 服务器配置（解析后）
type DiscordGuildEntryResolved struct {
	ID                   string                                 `json:"id,omitempty"`
	Slug                 string                                 `json:"slug,omitempty"`
	RequireMention       *bool                                  `json:"requireMention,omitempty"`
	ReactionNotification string                                 `json:"reactionNotifications,omitempty"`
	Users                []string                               `json:"users,omitempty"`
	Channels             map[string]DiscordChannelEntryResolved `json:"channels,omitempty"`
	Allow                *bool                                  `json:"allow,omitempty"`
	Skills               []string                               `json:"skills,omitempty"`
	Enabled              *bool                                  `json:"enabled,omitempty"`
	SystemPrompt         string                                 `json:"systemPrompt,omitempty"`
	IncludeThreadStarter *bool                                  `json:"includeThreadStarter,omitempty"`
	AutoThread           *bool                                  `json:"autoThread,omitempty"`
}

// DiscordChannelEntryResolved 频道配置
type DiscordChannelEntryResolved struct {
	Allow                *bool    `json:"allow,omitempty"`
	RequireMention       *bool    `json:"requireMention,omitempty"`
	Skills               []string `json:"skills,omitempty"`
	Enabled              *bool    `json:"enabled,omitempty"`
	Users                []string `json:"users,omitempty"`
	SystemPrompt         string   `json:"systemPrompt,omitempty"`
	IncludeThreadStarter *bool    `json:"includeThreadStarter,omitempty"`
	AutoThread           *bool    `json:"autoThread,omitempty"`
}

// DiscordChannelConfigResolved 频道配置（解析后最终结果）
type DiscordChannelConfigResolved struct {
	Allowed              bool     `json:"allowed"`
	RequireMention       *bool    `json:"requireMention,omitempty"`
	Skills               []string `json:"skills,omitempty"`
	Enabled              *bool    `json:"enabled,omitempty"`
	Users                []string `json:"users,omitempty"`
	SystemPrompt         string   `json:"systemPrompt,omitempty"`
	IncludeThreadStarter *bool    `json:"includeThreadStarter,omitempty"`
	AutoThread           *bool    `json:"autoThread,omitempty"`
	MatchKey             string   `json:"matchKey,omitempty"`
	MatchSource          string   `json:"matchSource,omitempty"`
}

// discordMentionStripRe strips the Discord mention prefix <@ or <@!
// TS ref: text.replace(/^<@!?/, "")
var discordMentionStripRe = regexp.MustCompile(`^<@!?`)

// NormalizeDiscordAllowList 从原始列表构建允许列表
// W-042 fix: When raw is empty/nil, return nil (unconfigured) instead of AllowAll:true.
// TS ref: if (!raw || raw.length === 0) return null;
// Callers must treat nil as "unconfigured / allow all".
func NormalizeDiscordAllowList(raw []string, prefixes []string) *DiscordAllowList {
	if len(raw) == 0 {
		return nil
	}
	list := DiscordAllowList{
		IDs:   make(map[string]bool),
		Names: make(map[string]bool),
	}
	for _, entry := range raw {
		val := strings.TrimSpace(entry)
		if val == "" {
			continue
		}
		if val == "*" {
			list.AllowAll = true
			continue
		}
		// W-043: Strip Discord mention format <@!?ID> before further processing.
		// TS ref: const maybeId = text.replace(/^<@!?/, "").replace(/>$/, "");
		stripped := discordMentionStripRe.ReplaceAllString(val, "")
		stripped = strings.TrimSuffix(stripped, ">")
		if discordNumericRe.MatchString(stripped) {
			list.IDs[stripped] = true
			continue
		}
		// 剥离前缀 (e.g. "user:", "discord:", "pk:")
		stripped = val
		for _, prefix := range prefixes {
			if strings.HasPrefix(strings.ToLower(stripped), strings.ToLower(prefix)) {
				stripped = strings.TrimSpace(stripped[len(prefix):])
				break
			}
		}
		if stripped != "" && discordNumericRe.MatchString(stripped) {
			list.IDs[stripped] = true
		} else if stripped != "" {
			list.Names[NormalizeDiscordSlug(stripped)] = true
		}
	}
	return &list
}

// AllowListMatches 检查候选者是否匹配允许列表
func AllowListMatches(list *DiscordAllowList, candidateID, candidateName, candidateTag string) DiscordAllowListMatch {
	// W-042: nil list means "unconfigured → allow all" (TS returns true when null)
	if list == nil {
		return DiscordAllowListMatch{Allowed: true}
	}
	if list.AllowAll {
		return DiscordAllowListMatch{Allowed: true}
	}
	if candidateID != "" && list.IDs[candidateID] {
		return DiscordAllowListMatch{Allowed: true, ByID: true}
	}
	if candidateName != "" {
		slug := NormalizeDiscordSlug(candidateName)
		if list.Names[slug] {
			return DiscordAllowListMatch{Allowed: true, ByName: true}
		}
	}
	if candidateTag != "" {
		slug := NormalizeDiscordSlug(candidateTag)
		if list.Names[slug] {
			return DiscordAllowListMatch{Allowed: true, ByName: true}
		}
	}
	return DiscordAllowListMatch{Allowed: false}
}

// ResolveDiscordUserAllowed 检查用户是否被允许
func ResolveDiscordUserAllowed(allowList []string, userID, userName, userTag string) bool {
	if len(allowList) == 0 {
		return true
	}
	list := NormalizeDiscordAllowList(allowList, []string{"user:", "discord:"})
	return AllowListMatches(list, userID, userName, userTag).Allowed
}

// ResolveDiscordGuildEntry 解析服务器条目
// TS ref: resolveDiscordGuildEntry (allow-list.ts) — supports ID key lookup,
// slug lookup, and wildcard "*" fallback.
func ResolveDiscordGuildEntry(guildID, guildName string, entries map[string]DiscordGuildEntryResolved) *DiscordGuildEntryResolved {
	if len(entries) == 0 {
		return nil
	}

	// 1. Direct key lookup by guild ID (TS: entries[guild.id])
	if guildID != "" {
		if entry, ok := entries[guildID]; ok {
			entry.ID = guildID
			return &entry
		}
		// Also check entries where the nested ID field matches
		for _, entry := range entries {
			if entry.ID == guildID {
				e := entry
				e.ID = guildID
				return &e
			}
		}
	}

	// 2. 用名称/slug 查找
	slug := ""
	if guildName != "" {
		slug = NormalizeDiscordSlug(guildName)
		if bySlug, ok := entries[slug]; ok {
			e := bySlug
			if guildID != "" {
				e.ID = guildID
			}
			if e.Slug == "" {
				e.Slug = slug
			}
			return &e
		}
		for key, entry := range entries {
			if NormalizeDiscordSlug(key) == slug || entry.Slug == slug {
				e := entry
				if guildID != "" {
					e.ID = guildID
				}
				if e.Slug == "" {
					e.Slug = slug
				}
				return &e
			}
		}
	}

	// 3. Wildcard "*" fallback (TS: entries["*"])
	if wildcard, ok := entries["*"]; ok {
		e := wildcard
		if guildID != "" {
			e.ID = guildID
		}
		if e.Slug == "" && slug != "" {
			e.Slug = slug
		}
		return &e
	}

	return nil
}

// ResolveDiscordChannelConfig 解析频道配置
// TS ref: resolveDiscordChannelConfig (allow-list.ts) — supports ID, slug,
// name matching, and wildcard "*" fallback.
func ResolveDiscordChannelConfig(guildInfo *DiscordGuildEntryResolved, channelID, channelName, channelSlug string) *DiscordChannelConfigResolved {
	if guildInfo == nil || len(guildInfo.Channels) == 0 {
		return nil
	}
	// 精确 ID 匹配
	if channelID != "" {
		if entry, ok := guildInfo.Channels[channelID]; ok {
			return resolveChannelEntry(entry, channelID)
		}
	}
	// slug 匹配
	if channelSlug != "" {
		for key, entry := range guildInfo.Channels {
			if key != "*" && NormalizeDiscordSlug(key) == channelSlug {
				return resolveChannelEntry(entry, key)
			}
		}
	}
	// 名称匹配
	if channelName != "" {
		nameSlug := NormalizeDiscordSlug(channelName)
		for key, entry := range guildInfo.Channels {
			if key != "*" && NormalizeDiscordSlug(key) == nameSlug {
				return resolveChannelEntry(entry, key)
			}
		}
	}
	// Wildcard "*" fallback
	if wildcard, ok := guildInfo.Channels["*"]; ok {
		return resolveChannelEntry(wildcard, "*")
	}
	return nil
}

// ResolveDiscordChannelConfigWithFallback 解析频道配置（含线程 parent 回退）
// W-044 fix: TS ref: resolveDiscordChannelConfigWithFallback (allow-list.ts)
// When the current channel (thread) has no config, fall back to the parent channel config.
func ResolveDiscordChannelConfigWithFallback(
	guildInfo *DiscordGuildEntryResolved,
	channelID, channelName, channelSlug string,
	parentID, parentName, parentSlug string,
	scope string, // "channel" | "thread"
) *DiscordChannelConfigResolved {
	if guildInfo == nil || len(guildInfo.Channels) == 0 {
		return nil
	}

	// TS ref: allowNameMatch = scope !== "thread"
	allowNameMatch := scope != "thread"

	// 1. Try direct match on the channel itself
	result := resolveChannelFromMap(guildInfo.Channels, channelID, channelName, channelSlug, allowNameMatch)
	if result != nil {
		result.MatchSource = "direct"
		return result
	}

	// 2. Thread parent fallback — if parent info is provided, try the parent channel
	if parentID != "" || parentName != "" || parentSlug != "" {
		resolvedParentSlug := parentSlug
		if resolvedParentSlug == "" && parentName != "" {
			resolvedParentSlug = NormalizeDiscordSlug(parentName)
		}
		parentResult := resolveChannelFromMap(guildInfo.Channels, parentID, parentName, resolvedParentSlug, true)
		if parentResult != nil {
			parentResult.MatchSource = "parent"
			return parentResult
		}
	}

	// 3. Wildcard "*" fallback
	if wildcard, ok := guildInfo.Channels["*"]; ok {
		cfg := resolveChannelEntry(wildcard, "*")
		cfg.MatchSource = "wildcard"
		return cfg
	}

	// TS returns { allowed: false } when no match at all
	return &DiscordChannelConfigResolved{Allowed: false}
}

// resolveChannelFromMap attempts to resolve a channel config from the channels map.
func resolveChannelFromMap(
	channels map[string]DiscordChannelEntryResolved,
	channelID, channelName, channelSlug string,
	allowNameMatch bool,
) *DiscordChannelConfigResolved {
	// Exact ID match
	if channelID != "" {
		if entry, ok := channels[channelID]; ok {
			return resolveChannelEntry(entry, channelID)
		}
	}
	if !allowNameMatch {
		return nil
	}
	// Slug match
	if channelSlug != "" {
		for key, entry := range channels {
			if key != "*" && NormalizeDiscordSlug(key) == channelSlug {
				return resolveChannelEntry(entry, key)
			}
		}
	}
	// Name match
	if channelName != "" {
		nameSlug := NormalizeDiscordSlug(channelName)
		for key, entry := range channels {
			if key != "*" && NormalizeDiscordSlug(key) == nameSlug {
				return resolveChannelEntry(entry, key)
			}
		}
	}
	return nil
}

func resolveChannelEntry(entry DiscordChannelEntryResolved, key string) *DiscordChannelConfigResolved {
	allowed := true
	if entry.Allow != nil {
		allowed = *entry.Allow
	}
	return &DiscordChannelConfigResolved{
		Allowed:              allowed,
		RequireMention:       entry.RequireMention,
		Skills:               entry.Skills,
		Enabled:              entry.Enabled,
		Users:                entry.Users,
		SystemPrompt:         entry.SystemPrompt,
		IncludeThreadStarter: entry.IncludeThreadStarter,
		AutoThread:           entry.AutoThread,
		MatchKey:             key,
	}
}

// ResolveDiscordShouldRequireMention 是否要求提及
// contextDefault 为 monitor context 级别的 RequireMention 设置，
// 当频道级/服务器级配置都未覆盖时作为兜底值。
func ResolveDiscordShouldRequireMention(isGuildMessage, isThread bool, botID, threadOwnerID string, channelConfig *DiscordChannelConfigResolved, guildInfo *DiscordGuildEntryResolved, isAutoThreadOwned bool, contextDefault bool) bool {
	if !isGuildMessage {
		return false
	}
	if isAutoThreadOwned {
		return false
	}
	if channelConfig != nil && channelConfig.RequireMention != nil {
		return *channelConfig.RequireMention
	}
	if guildInfo != nil && guildInfo.RequireMention != nil {
		return *guildInfo.RequireMention
	}
	return contextDefault
}

// IsDiscordAutoThreadOwnedByBot 是否是 bot 拥有的自动线程
func IsDiscordAutoThreadOwnedByBot(isThread bool, channelConfig *DiscordChannelConfigResolved, botID, threadOwnerID string) bool {
	if !isThread {
		return false
	}
	if channelConfig == nil || channelConfig.AutoThread == nil || !*channelConfig.AutoThread {
		return false
	}
	if botID == "" || threadOwnerID == "" {
		return false
	}
	return botID == threadOwnerID
}

// IsDiscordGroupAllowedByPolicy 群组策略是否允许
// W-045 fix: 对齐 TS — allowlist 模式下必须先检查 guild 是否在白名单中
// TS ref: if (!guildAllowlisted) return false; if (!channelAllowlistConfigured) return true; return channelAllowed;
func IsDiscordGroupAllowedByPolicy(groupPolicy string, guildAllowlisted, channelAllowlistConfigured, channelAllowed bool) bool {
	switch groupPolicy {
	case "disabled":
		return false
	case "open":
		return true
	case "allowlist":
		if !guildAllowlisted {
			return false
		}
		if !channelAllowlistConfigured {
			return true
		}
		return channelAllowed
	default:
		return false
	}
}
