//go:build darwin

package imessage

import (
	"fmt"
	"strings"
)

// 群组消息历史管理 — 对标 TS auto-reply/reply/history.ts

// DefaultGroupHistoryLimit 默认群组历史记录数量限制
const DefaultGroupHistoryLimit = 10

// HistoryEntry 历史消息条目
type HistoryEntry struct {
	Sender    string
	Body      string
	Timestamp *int64
	MessageID string
}

// GroupHistories 群组历史管理器
type GroupHistories struct {
	histories map[string][]HistoryEntry
}

// NewGroupHistories 创建群组历史管理器
func NewGroupHistories() *GroupHistories {
	return &GroupHistories{
		histories: make(map[string][]HistoryEntry),
	}
}

// RecordPendingHistoryEntry 记录待处理的消息到历史记录中。
// 仅在 historyLimit > 0 时记录。
// TS 对照: auto-reply/reply/history.ts recordPendingHistoryEntryIfEnabled()
func (h *GroupHistories) RecordPendingHistoryEntry(historyKey string, limit int, entry *HistoryEntry) {
	if limit <= 0 || historyKey == "" || entry == nil {
		return
	}
	entries := h.histories[historyKey]
	entries = append(entries, *entry)
	// 保持在 limit 内
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	h.histories[historyKey] = entries
}

// BuildPendingHistoryContext 将历史记录拼接到当前消息前。
// TS 对照: auto-reply/reply/history.ts buildPendingHistoryContextFromMap()
func (h *GroupHistories) BuildPendingHistoryContext(historyKey string, limit int, currentMessage string, formatEntry func(entry HistoryEntry) string) string {
	if limit <= 0 || historyKey == "" {
		return currentMessage
	}
	entries := h.histories[historyKey]
	if len(entries) == 0 {
		return currentMessage
	}

	var parts []string
	for _, entry := range entries {
		formatted := formatEntry(entry)
		if formatted != "" {
			parts = append(parts, formatted)
		}
	}
	if len(parts) == 0 {
		return currentMessage
	}

	parts = append(parts, currentMessage)
	return strings.Join(parts, "\n\n")
}

// ClearHistoryEntries 清除指定群组的历史记录。
// TS 对照: auto-reply/reply/history.ts clearHistoryEntriesIfEnabled()
func (h *GroupHistories) ClearHistoryEntries(historyKey string, limit int) {
	if limit <= 0 || historyKey == "" {
		return
	}
	delete(h.histories, historyKey)
}

// TruncateUtf16Safe 安全截断字符串以避免截断 UTF-16 代理对。
// TS 对照: utils.ts truncateUtf16Safe()
func TruncateUtf16Safe(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	truncated := string(runes[:maxLen])
	if maxLen > 3 {
		return truncated[:len(truncated)-3] + "..."
	}
	return truncated
}

// FormatMediaPlaceholder 格式化媒体占位符
func FormatMediaPlaceholder(kind string, attachmentCount int) string {
	if kind != "" {
		return fmt.Sprintf("<media:%s>", kind)
	}
	if attachmentCount > 0 {
		return "<media:attachment>"
	}
	return ""
}
