package line

// TS 对照: src/line/bot-access.ts
// LINE 访问控制 — DM/群组策略、allowFrom 归一化

import "strings"

// NormalizedAllowFrom 归一化的允许列表。
type NormalizedAllowFrom struct {
	Entries      []string
	EntriesLower []string
	HasWildcard  bool
	HasEntries   bool
}

// StoreAllowFromResult 从 store 读取的允许列表结果。
type StoreAllowFromResult struct {
	Entries []string
}

// NormalizeAllowFromWithStore 合并 config + store 来源的 allowFrom 列表。
// TS: normalizeAllowFromWithStore({ allowFrom, storeAllowFrom })
func NormalizeAllowFromWithStore(allowFrom []string, storeAllowFrom []string) NormalizedAllowFrom {
	merged := make([]string, 0, len(allowFrom)+len(storeAllowFrom))
	merged = append(merged, allowFrom...)
	merged = append(merged, storeAllowFrom...)
	return normalizeAllowFrom(merged)
}

func normalizeAllowFrom(list []string) NormalizedAllowFrom {
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
		// 移除 line: 前缀
		cleaned := trimmed
		lower := strings.ToLower(cleaned)
		if strings.HasPrefix(lower, "line:") {
			cleaned = cleaned[5:]
		}
		entries = append(entries, cleaned)
	}
	entriesLower := make([]string, len(entries))
	for i, e := range entries {
		entriesLower[i] = strings.ToLower(e)
	}
	return NormalizedAllowFrom{
		Entries:      entries,
		EntriesLower: entriesLower,
		HasWildcard:  hasWildcard,
		HasEntries:   len(entries) > 0 || hasWildcard,
	}
}

// IsSenderAllowedLine 检查 LINE 发送者是否被允许。
// TS: isSenderAllowed({ allow, senderId })
func IsSenderAllowedLine(allow NormalizedAllowFrom, senderID string) bool {
	if allow.HasWildcard {
		return true
	}
	if !allow.HasEntries {
		return false
	}
	senderLower := strings.ToLower(strings.TrimSpace(senderID))
	for _, e := range allow.EntriesLower {
		if e == senderLower {
			return true
		}
	}
	return false
}

// FirstDefined 返回切片中第一个非 nil 的 []string。
// TS: firstDefined(...args)
func FirstDefined(values ...[]string) []string {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}
