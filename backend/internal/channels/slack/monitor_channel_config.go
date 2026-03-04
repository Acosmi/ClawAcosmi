package slack

import (
	"path/filepath"
	"strings"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// Slack 频道配置解析 — 继承自 src/slack/monitor/channel-config.ts (139L)

// SlackChannelConfigResolved 解析后的频道配置
type SlackChannelConfigResolved struct {
	Allowed        bool
	RequireMention bool
	AllowBots      *bool
	Users          []interface{}
	Skills         []string
	SystemPrompt   string
	MatchKey       string
	MatchSource    string
}

// ShouldEmitSlackReactionNotification 判断是否应发送反应通知。
func ShouldEmitSlackReactionNotification(
	mode types.SlackReactionNotificationMode,
	botID, messageAuthorID, userID, userName string,
	allowlist []interface{},
) bool {
	effectiveMode := mode
	if effectiveMode == "" {
		effectiveMode = "own"
	}

	switch effectiveMode {
	case "off":
		return false
	case "own":
		if botID == "" || messageAuthorID == "" {
			return false
		}
		return messageAuthorID == botID
	case "allowlist":
		if len(allowlist) == 0 {
			return false
		}
		users := NormalizeAllowListLower(allowlist)
		return AllowListMatches(users, userID, userName)
	default: // "all"
		return true
	}
}

// ResolveSlackChannelLabel 生成频道标签。
func ResolveSlackChannelLabel(channelID, channelName string) string {
	name := strings.TrimSpace(channelName)
	if name != "" {
		slug := NormalizeSlackSlug(name)
		if slug != "" {
			return "#" + slug
		}
		return "#" + name
	}
	id := strings.TrimSpace(channelID)
	if id != "" {
		return "#" + id
	}
	return "unknown channel"
}

// BuildChannelKeyCandidates 构建频道 key 候选列表。
// 生成 ID、#name、name、slug、#slug 及其小写变体。
func BuildChannelKeyCandidates(channelID, channelName string) []string {
	seen := make(map[string]bool)
	var candidates []string
	add := func(key string) {
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		candidates = append(candidates, key)
	}

	id := strings.TrimSpace(channelID)
	name := strings.TrimSpace(channelName)
	slug := NormalizeSlackSlug(name)

	// 精确格式：ID → #name → name → #slug → slug
	add(id)
	if name != "" {
		add("#" + name)
		add(name)
	}
	if slug != "" && slug != name {
		add("#" + slug)
		add(slug)
	}
	// 小写变体
	if id != "" {
		add(strings.ToLower(id))
	}
	if name != "" {
		lower := strings.ToLower(name)
		add("#" + lower)
		add(lower)
	}

	return candidates
}

// ResolveChannelEntryMatchWithFallback 从 channels map 中按候选 key 匹配条目。
// 匹配优先级：精确 → 大小写不敏感 → glob → 通配符 *
func ResolveChannelEntryMatchWithFallback(
	candidates []string,
	channels map[string]*types.SlackChannelConfig,
) (entry *types.SlackChannelConfig, matchedKey, matchSource string) {
	// 1. 精确匹配
	for _, key := range candidates {
		if e, ok := channels[key]; ok && e != nil {
			return e, key, "exact"
		}
	}

	// 2. 大小写不敏感匹配
	for _, key := range candidates {
		lowerKey := strings.ToLower(key)
		for k, e := range channels {
			if k == "*" || e == nil {
				continue
			}
			if strings.ToLower(k) == lowerKey {
				return e, k, "case-insensitive"
			}
		}
	}

	// 3. glob 匹配（支持 * 和 ? 通配符模式）
	for _, key := range candidates {
		for k, e := range channels {
			if k == "*" || e == nil {
				continue
			}
			if strings.ContainsAny(k, "*?") {
				if matched, _ := filepath.Match(k, key); matched {
					return e, k, "glob"
				}
			}
		}
	}

	// 4. 通配符 fallback
	if w, ok := channels["*"]; ok && w != nil {
		return w, "*", "wildcard"
	}

	return nil, "", ""
}

// ResolveSlackChannelConfig 解析频道级配置。
// 使用 BuildChannelKeyCandidates + ResolveChannelEntryMatchWithFallback。
func ResolveSlackChannelConfig(
	channelID, channelName string,
	channels map[string]*types.SlackChannelConfig,
	defaultRequireMention bool,
) *SlackChannelConfigResolved {
	if len(channels) == 0 {
		return &SlackChannelConfigResolved{
			Allowed:        true,
			RequireMention: defaultRequireMention,
		}
	}

	candidates := BuildChannelKeyCandidates(channelID, channelName)
	entry, matchedKey, matchSource := ResolveChannelEntryMatchWithFallback(candidates, channels)

	if entry == nil {
		return &SlackChannelConfigResolved{
			Allowed:        false,
			RequireMention: defaultRequireMention,
		}
	}

	return resolveChannelEntry(entry, nil, defaultRequireMention, matchedKey, matchSource)
}

func resolveChannelEntry(
	entry, fallback *types.SlackChannelConfig,
	defaultRequireMention bool,
	matchKey, matchSource string,
) *SlackChannelConfigResolved {
	allowed := true
	if entry.Enabled != nil {
		allowed = *entry.Enabled
	} else if entry.Allow != nil {
		allowed = *entry.Allow
	}

	requireMention := defaultRequireMention
	if entry.RequireMention != nil {
		requireMention = *entry.RequireMention
	} else if fallback != nil && fallback.RequireMention != nil {
		requireMention = *fallback.RequireMention
	}

	result := &SlackChannelConfigResolved{
		Allowed:        allowed,
		RequireMention: requireMention,
		Users:          entry.Users,
		Skills:         entry.Skills,
		SystemPrompt:   entry.SystemPrompt,
		MatchKey:       matchKey,
		MatchSource:    matchSource,
	}

	if entry.AllowBots != nil {
		result.AllowBots = entry.AllowBots
	} else if fallback != nil && fallback.AllowBots != nil {
		result.AllowBots = fallback.AllowBots
	}

	return result
}
