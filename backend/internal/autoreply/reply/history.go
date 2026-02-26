package reply

import (
	"strings"
	"sync"
)

// TS 对照: auto-reply/reply/history.ts (194L)

// HistoryContextMarker 历史上下文标记。
const HistoryContextMarker = "[Chat messages since your last reply - for context]"

// DefaultGroupHistoryLimit 默认群组历史限制。
const DefaultGroupHistoryLimit = 50

// MaxHistoryKeys 最大历史 key 数量（LRU 驱逐阈值）。
const MaxHistoryKeys = 1000

// HistoryEntry 历史条目。
type HistoryEntry struct {
	Sender    string
	Body      string
	Timestamp int64
	MessageID string
}

// HistoryMap 线程安全的历史记录表。
type HistoryMap struct {
	mu      sync.RWMutex
	entries map[string][]HistoryEntry
	// 记录 key 插入顺序（用于 LRU 驱逐）
	order []string
}

// NewHistoryMap 创建历史记录表。
func NewHistoryMap() *HistoryMap {
	return &HistoryMap{
		entries: make(map[string][]HistoryEntry),
	}
}

// EvictOldHistoryKeys 驱逐超出上限的历史 key。
// TS 对照: history.ts L13-28
func (m *HistoryMap) EvictOldHistoryKeys(maxKeys int) {
	if maxKeys <= 0 {
		maxKeys = MaxHistoryKeys
	}
	if len(m.order) <= maxKeys {
		return
	}
	toDelete := len(m.order) - maxKeys
	for i := 0; i < toDelete; i++ {
		delete(m.entries, m.order[i])
	}
	m.order = m.order[toDelete:]
}

// AppendHistoryEntry 追加历史条目。
// TS 对照: history.ts L52-75
func (m *HistoryMap) AppendHistoryEntry(key string, entry HistoryEntry, limit int) []HistoryEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limit <= 0 {
		return nil
	}
	history := m.entries[key]
	history = append(history, entry)
	for len(history) > limit {
		history = history[1:]
	}
	// 刷新插入顺序
	m.removeFromOrder(key)
	m.order = append(m.order, key)
	m.entries[key] = history

	m.EvictOldHistoryKeys(MaxHistoryKeys)
	return history
}

// GetEntries 获取指定 key 的历史条目。
func (m *HistoryMap) GetEntries(key string) []HistoryEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.entries[key]
}

// ClearEntries 清除指定 key 的历史。
// TS 对照: history.ts L157-162
func (m *HistoryMap) ClearEntries(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[key] = nil
}

func (m *HistoryMap) removeFromOrder(key string) {
	for i, k := range m.order {
		if k == key {
			m.order = append(m.order[:i], m.order[i+1:]...)
			return
		}
	}
}

// ---------- 历史上下文构建 ----------

// BuildHistoryContext 构建带历史的消息上下文。
// TS 对照: history.ts L37-50
func BuildHistoryContext(historyText, currentMessage, lineBreak string) string {
	if lineBreak == "" {
		lineBreak = "\n"
	}
	if strings.TrimSpace(historyText) == "" {
		return currentMessage
	}
	return strings.Join([]string{
		HistoryContextMarker,
		historyText,
		"",
		CurrentMessageMarker,
		currentMessage,
	}, lineBreak)
}

// BuildHistoryContextFromEntries 从条目列表构建历史上下文。
// TS 对照: history.ts L175-193
func BuildHistoryContextFromEntries(entries []HistoryEntry, currentMessage string, formatEntry func(HistoryEntry) string, lineBreak string, excludeLast bool) string {
	if lineBreak == "" {
		lineBreak = "\n"
	}
	selected := entries
	if excludeLast && len(selected) > 0 {
		selected = selected[:len(selected)-1]
	}
	if len(selected) == 0 {
		return currentMessage
	}
	parts := make([]string, 0, len(selected))
	for _, e := range selected {
		parts = append(parts, formatEntry(e))
	}
	historyText := strings.Join(parts, lineBreak)
	return BuildHistoryContext(historyText, currentMessage, lineBreak)
}

// BuildHistoryContextFromMap 从历史表构建上下文（可选追加新条目）。
// TS 对照: history.ts L127-155
func BuildHistoryContextFromMap(m *HistoryMap, key string, limit int, entry *HistoryEntry, currentMessage string, formatEntry func(HistoryEntry) string, lineBreak string, excludeLast bool) string {
	if limit <= 0 {
		return currentMessage
	}
	var entries []HistoryEntry
	if entry != nil {
		entries = m.AppendHistoryEntry(key, *entry, limit)
	} else {
		entries = m.GetEntries(key)
	}
	return BuildHistoryContextFromEntries(entries, currentMessage, formatEntry, lineBreak, excludeLast)
}
