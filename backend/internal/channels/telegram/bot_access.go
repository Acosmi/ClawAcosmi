package telegram

import "strings"

// Telegram 访问控制 — 继承自 src/telegram/bot-access.ts (95L)

// NormalizedAllowFrom 归一化的允许列表
type NormalizedAllowFrom struct {
	Entries      []string
	EntriesLower []string
	HasWildcard  bool
	HasEntries   bool
}

// AllowFromMatch 允许匹配结果
type AllowFromMatch struct {
	Allowed     bool
	MatchKey    string
	MatchSource string // "wildcard", "id", "username"
}

// NormalizeAllowFrom 归一化允许列表
func NormalizeAllowFrom(list []string) NormalizedAllowFrom {
	entries := make([]string, 0, len(list))
	hasWildcard := false
	for _, v := range list {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if trimmed == "*" {
			hasWildcard = true
			continue
		}
		// 移除 telegram: / tg: 前缀
		cleaned := trimmed
		lower := strings.ToLower(cleaned)
		if strings.HasPrefix(lower, "telegram:") {
			cleaned = cleaned[9:]
		} else if strings.HasPrefix(lower, "tg:") {
			cleaned = cleaned[3:]
		}
		entries = append(entries, cleaned)
	}
	entriesLower := make([]string, len(entries))
	for i, e := range entries {
		entriesLower[i] = strings.ToLower(e)
	}
	return NormalizedAllowFrom{
		Entries: entries, EntriesLower: entriesLower,
		HasWildcard: hasWildcard, HasEntries: len(entries) > 0 || hasWildcard,
	}
}

// IsSenderAllowed 检查发送者是否被允许
func IsSenderAllowed(allow NormalizedAllowFrom, senderID, senderUsername string) bool {
	if !allow.HasEntries {
		return true
	}
	if allow.HasWildcard {
		return true
	}
	if senderID != "" {
		for _, e := range allow.Entries {
			if e == senderID {
				return true
			}
		}
	}
	username := strings.ToLower(strings.TrimSpace(senderUsername))
	if username == "" {
		return false
	}
	for _, e := range allow.EntriesLower {
		if e == username || e == "@"+username {
			return true
		}
	}
	return false
}

// ResolveSenderAllowMatch 解析发送者匹配详情
func ResolveSenderAllowMatch(allow NormalizedAllowFrom, senderID, senderUsername string) AllowFromMatch {
	if allow.HasWildcard {
		return AllowFromMatch{Allowed: true, MatchKey: "*", MatchSource: "wildcard"}
	}
	if !allow.HasEntries {
		return AllowFromMatch{}
	}
	if senderID != "" {
		for _, e := range allow.Entries {
			if e == senderID {
				return AllowFromMatch{Allowed: true, MatchKey: senderID, MatchSource: "id"}
			}
		}
	}
	username := strings.ToLower(strings.TrimSpace(senderUsername))
	if username == "" {
		return AllowFromMatch{}
	}
	for _, e := range allow.EntriesLower {
		if e == username || e == "@"+username {
			return AllowFromMatch{Allowed: true, MatchKey: e, MatchSource: "username"}
		}
	}
	return AllowFromMatch{}
}
