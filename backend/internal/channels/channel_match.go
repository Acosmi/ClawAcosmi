package channels

import (
	"strings"
)

// 频道配置匹配 — 继承自 src/channels/channel-config.ts (183 行)
// 实现五级回退匹配：direct → normalized → parent → normalizedParent → wildcard

// ChannelMatchSource 匹配来源
type ChannelMatchSource string

const (
	MatchSourceDirect   ChannelMatchSource = "direct"
	MatchSourceParent   ChannelMatchSource = "parent"
	MatchSourceWildcard ChannelMatchSource = "wildcard"
)

// ChannelEntryMatch 频道条目匹配结果（泛型用 interface{}）
type ChannelEntryMatch struct {
	Entry         interface{}
	Key           string
	WildcardEntry interface{}
	WildcardKey   string
	ParentEntry   interface{}
	ParentKey     string
	MatchKey      string
	MatchSource   ChannelMatchSource
}

// NormalizeChannelSlug 规范化频道 slug
// 继承自 TS: trim → lowercase → 去 # 前缀 → 非字母数字替换为 - → 去首尾 -
func NormalizeChannelSlug(value string) string {
	s := strings.TrimSpace(strings.ToLower(value))
	if strings.HasPrefix(s, "#") {
		s = s[1:]
	}
	// 非 a-z0-9 替换为 -
	var buf strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			buf.WriteRune(c)
		} else {
			buf.WriteByte('-')
		}
	}
	// 去首尾 -
	result := strings.Trim(buf.String(), "-")
	return result
}

// BuildChannelKeyCandidates 构建去重的候选键列表
func BuildChannelKeyCandidates(keys ...string) []string {
	seen := make(map[string]bool)
	var candidates []string
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		candidates = append(candidates, trimmed)
	}
	return candidates
}

// ResolveChannelEntryMatch 基础匹配：精确键 + 通配符
func ResolveChannelEntryMatch(entries map[string]interface{}, keys []string, wildcardKey string) ChannelEntryMatch {
	match := ChannelEntryMatch{}
	for _, key := range keys {
		if entry, ok := entries[key]; ok {
			match.Entry = entry
			match.Key = key
			break
		}
	}
	if wildcardKey != "" {
		if entry, ok := entries[wildcardKey]; ok {
			match.WildcardEntry = entry
			match.WildcardKey = wildcardKey
		}
	}
	return match
}

// ResolveChannelEntryMatchWithFallback 五级回退匹配
// 1. 精确键匹配 → 2. 规范化键匹配 → 3. 父级键匹配 → 4. 规范化父级匹配 → 5. 通配符
func ResolveChannelEntryMatchWithFallback(
	entries map[string]interface{},
	keys []string,
	parentKeys []string,
	wildcardKey string,
	normalizeKey func(string) string,
) ChannelEntryMatch {
	// 第 1 步：精确匹配
	direct := ResolveChannelEntryMatch(entries, keys, wildcardKey)
	if direct.Entry != nil && direct.Key != "" {
		direct.MatchKey = direct.Key
		direct.MatchSource = MatchSourceDirect
		return direct
	}

	// 第 2 步：规范化键匹配
	if normalizeKey != nil {
		var normalizedKeys []string
		for _, k := range keys {
			if nk := normalizeKey(k); nk != "" {
				normalizedKeys = append(normalizedKeys, nk)
			}
		}
		if len(normalizedKeys) > 0 {
			for entryKey, entry := range entries {
				ne := normalizeKey(entryKey)
				if ne == "" {
					continue
				}
				for _, nk := range normalizedKeys {
					if ne == nk {
						direct.Entry = entry
						direct.Key = entryKey
						direct.MatchKey = entryKey
						direct.MatchSource = MatchSourceDirect
						return direct
					}
				}
			}
		}
	}

	// 第 3 步：父级键匹配
	if len(parentKeys) > 0 {
		parent := ResolveChannelEntryMatch(entries, parentKeys, "")
		if parent.Entry != nil && parent.Key != "" {
			direct.Entry = parent.Entry
			direct.Key = parent.Key
			direct.ParentEntry = parent.Entry
			direct.ParentKey = parent.Key
			direct.MatchKey = parent.Key
			direct.MatchSource = MatchSourceParent
			return direct
		}

		// 第 4 步：规范化父级匹配
		if normalizeKey != nil {
			var normalizedParentKeys []string
			for _, k := range parentKeys {
				if nk := normalizeKey(k); nk != "" {
					normalizedParentKeys = append(normalizedParentKeys, nk)
				}
			}
			if len(normalizedParentKeys) > 0 {
				for entryKey, entry := range entries {
					ne := normalizeKey(entryKey)
					if ne == "" {
						continue
					}
					for _, nk := range normalizedParentKeys {
						if ne == nk {
							direct.Entry = entry
							direct.Key = entryKey
							direct.ParentEntry = entry
							direct.ParentKey = entryKey
							direct.MatchKey = entryKey
							direct.MatchSource = MatchSourceParent
							return direct
						}
					}
				}
			}
		}
	}

	// 第 5 步：通配符回退
	if direct.WildcardEntry != nil && direct.WildcardKey != "" {
		direct.Entry = direct.WildcardEntry
		direct.Key = direct.WildcardKey
		direct.MatchKey = direct.WildcardKey
		direct.MatchSource = MatchSourceWildcard
		return direct
	}

	return direct
}

// ResolveNestedAllowlistDecision 嵌套白名单决策
// 外→内两级过滤：外层未配置=放行，外层不匹配=拒绝，内层未配置=放行
func ResolveNestedAllowlistDecision(outerConfigured, outerMatched, innerConfigured, innerMatched bool) bool {
	if !outerConfigured {
		return true
	}
	if !outerMatched {
		return false
	}
	if !innerConfigured {
		return true
	}
	return innerMatched
}
