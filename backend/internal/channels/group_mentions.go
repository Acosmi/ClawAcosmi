package channels

import (
	"regexp"
	"strings"
)

// 群组提及与工具策略解析 — 继承自 src/channels/plugins/group-mentions.ts (409 行)
// 包含：Discord guild 解析、Slack slug 规范化、Telegram topic 解析

// GroupMentionParams 群组提及解析参数
type GroupMentionParams struct {
	GroupID        string
	GroupChannel   string
	GroupSpace     string
	AccountID      string
	SenderID       string
	SenderName     string
	SenderUsername string
	SenderE164     string
}

// ---------- Slug 规范化 ----------

var (
	discordLeadingChars = regexp.MustCompile(`^[@#]+`)
	multiDash           = regexp.MustCompile(`-{2,}`)
	nonAlphanumDash     = regexp.MustCompile(`[^a-z0-9-]+`)
	slackAllowed        = regexp.MustCompile(`[^a-z0-9#@._+\-]+`)
	telegramChatIDRe    = regexp.MustCompile(`^-?\d+$`)
	telegramTopicIDRe   = regexp.MustCompile(`^\d+$`)
)

// NormalizeDiscordSlug Discord slug 规范化
func NormalizeDiscordSlug(value string) string {
	s := strings.TrimSpace(strings.ToLower(value))
	if s == "" {
		return ""
	}
	s = discordLeadingChars.ReplaceAllString(s, "")
	s = regexp.MustCompile(`[\s_]+`).ReplaceAllString(s, "-")
	s = nonAlphanumDash.ReplaceAllString(s, "-")
	s = multiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// NormalizeSlackSlug Slack slug 规范化
func NormalizeSlackSlug(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return ""
	}
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, "-")
	s = slackAllowed.ReplaceAllString(s, "-")
	s = multiDash.ReplaceAllString(s, "-")
	s = strings.TrimLeft(s, "-.")
	s = strings.TrimRight(s, "-.")
	return s
}

// ---------- Telegram 解析 ----------

// TelegramGroupParsed Telegram 群组解析结果
type TelegramGroupParsed struct {
	ChatID  string
	TopicID string
}

// ParseTelegramGroupID 解析 Telegram 群组 ID（支持三种格式）
func ParseTelegramGroupID(value string) TelegramGroupParsed {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return TelegramGroupParsed{}
	}
	parts := splitNonEmpty(raw, ":")
	// chatId:topic:topicId
	if len(parts) >= 3 && parts[1] == "topic" &&
		telegramChatIDRe.MatchString(parts[0]) && telegramTopicIDRe.MatchString(parts[2]) {
		return TelegramGroupParsed{ChatID: parts[0], TopicID: parts[2]}
	}
	// chatId:topicId
	if len(parts) >= 2 &&
		telegramChatIDRe.MatchString(parts[0]) && telegramTopicIDRe.MatchString(parts[1]) {
		return TelegramGroupParsed{ChatID: parts[0], TopicID: parts[1]}
	}
	return TelegramGroupParsed{ChatID: raw}
}

func splitNonEmpty(s, sep string) []string {
	raw := strings.Split(s, sep)
	var result []string
	for _, p := range raw {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ---------- Discord Guild 解析 ----------

// DiscordGuildEntry Discord guild 配置条目（简化）
type DiscordGuildEntry struct {
	Slug           string                          `json:"slug,omitempty"`
	RequireMention *bool                           `json:"requireMention,omitempty"`
	Channels       map[string]*DiscordChannelEntry `json:"channels,omitempty"`
	Tools          interface{}                     `json:"tools,omitempty"`
	ToolsBySender  interface{}                     `json:"toolsBySender,omitempty"`
}

// DiscordChannelEntry Discord 频道配置条目
type DiscordChannelEntry struct {
	RequireMention *bool       `json:"requireMention,omitempty"`
	Tools          interface{} `json:"tools,omitempty"`
	ToolsBySender  interface{} `json:"toolsBySender,omitempty"`
}

// ResolveDiscordGuildEntry 解析 Discord guild 配置（四级回退）
func ResolveDiscordGuildEntry(guilds map[string]*DiscordGuildEntry, groupSpace string) *DiscordGuildEntry {
	if len(guilds) == 0 {
		return nil
	}
	space := strings.TrimSpace(groupSpace)
	// 1. 精确匹配
	if space != "" {
		if g, ok := guilds[space]; ok {
			return g
		}
	}
	// 2. 规范化匹配
	normalized := NormalizeDiscordSlug(space)
	if normalized != "" {
		if g, ok := guilds[normalized]; ok {
			return g
		}
	}
	// 3. slug 字段匹配
	if normalized != "" {
		for _, entry := range guilds {
			if entry != nil && NormalizeDiscordSlug(entry.Slug) == normalized {
				return entry
			}
		}
	}
	// 4. 通配符
	if w, ok := guilds["*"]; ok {
		return w
	}
	return nil
}

// ResolveDiscordChannelEntry 解析 Discord 频道条目
func ResolveDiscordChannelEntry(channels map[string]*DiscordChannelEntry, groupID, groupChannel string) *DiscordChannelEntry {
	if len(channels) == 0 {
		return nil
	}
	// 精确 ID
	if groupID != "" {
		if e, ok := channels[groupID]; ok {
			return e
		}
	}
	// slug 变体
	slug := NormalizeDiscordSlug(groupChannel)
	if slug != "" {
		if e, ok := channels[slug]; ok {
			return e
		}
		if e, ok := channels["#"+slug]; ok {
			return e
		}
	}
	// groupChannel 原始规范化
	if groupChannel != "" {
		ns := NormalizeDiscordSlug(groupChannel)
		if ns != "" {
			if e, ok := channels[ns]; ok {
				return e
			}
		}
	}
	return nil
}

// ---------- 各频道 RequireMention 解析 ----------

// ResolveDiscordGroupRequireMention 解析 Discord 群组的 requireMention
func ResolveDiscordGroupRequireMention(guilds map[string]*DiscordGuildEntry, p GroupMentionParams) bool {
	guild := ResolveDiscordGuildEntry(guilds, p.GroupSpace)
	if guild != nil && len(guild.Channels) > 0 {
		entry := ResolveDiscordChannelEntry(guild.Channels, p.GroupID, p.GroupChannel)
		if entry != nil && entry.RequireMention != nil {
			return *entry.RequireMention
		}
	}
	if guild != nil && guild.RequireMention != nil {
		return *guild.RequireMention
	}
	return true // Discord 默认需要提及
}

// ResolveSlackGroupRequireMention 解析 Slack 群组的 requireMention
func ResolveSlackGroupRequireMention(channels map[string]interface{}, p GroupMentionParams) bool {
	if len(channels) == 0 {
		return true
	}
	channelID := strings.TrimSpace(p.GroupID)
	channelName := strings.TrimPrefix(p.GroupChannel, "#")
	normalizedName := NormalizeSlackSlug(channelName)
	candidates := BuildChannelKeyCandidates(channelID, "#"+channelName, channelName, normalizedName)

	var matched interface{}
	for _, c := range candidates {
		if v, ok := channels[c]; ok {
			matched = v
			break
		}
	}
	if matched == nil {
		matched = channels["*"]
	}
	if m, ok := matched.(map[string]interface{}); ok {
		if r, ok := m["requireMention"].(bool); ok {
			return r
		}
	}
	return true
}
