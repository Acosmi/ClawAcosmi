package gateway

// system_presence.go — 系统在线状态存储
// 对齐 TS src/infra/system-presence.ts
//
// PresenceEntry 类型定义在 protocol.go 中（P-R1: 16 字段），此处复用。

import (
	"strings"
	"sync"
	"time"
)

// PresenceUpdateResult 更新 presence 后的返回值。
type PresenceUpdateResult struct {
	Key         string
	Next        *PresenceEntry
	ChangedKeys []string
}

// SystemPresenceStore 线程安全的系统在线状态存储。
type SystemPresenceStore struct {
	mu      sync.RWMutex
	entries map[string]*PresenceEntry
}

// NewSystemPresenceStore 创建系统 presence 存储。
func NewSystemPresenceStore() *SystemPresenceStore {
	return &SystemPresenceStore{
		entries: make(map[string]*PresenceEntry),
	}
}

// resolvePresenceKey 生成 presence 条目的唯一键。
func resolvePresenceKey(entry *PresenceEntry) string {
	parts := []string{entry.Text}
	if entry.DeviceID != "" {
		parts = append(parts, entry.DeviceID)
	}
	if entry.InstanceID != "" {
		parts = append(parts, entry.InstanceID)
	}
	return strings.Join(parts, "::")
}

// Update 更新/插入 presence 条目，返回变更信息。
func (s *SystemPresenceStore) Update(entry *PresenceEntry) PresenceUpdateResult {
	key := resolvePresenceKey(entry)
	entry.Ts = time.Now().UnixMilli()

	s.mu.Lock()
	defer s.mu.Unlock()

	prev := s.entries[key]
	var changed []string

	if prev == nil {
		changed = []string{"text", "host", "ip", "version", "mode", "reason"}
	} else {
		if prev.Host != entry.Host {
			changed = append(changed, "host")
		}
		if prev.IP != entry.IP {
			changed = append(changed, "ip")
		}
		if prev.Version != entry.Version {
			changed = append(changed, "version")
		}
		if prev.Mode != entry.Mode {
			changed = append(changed, "mode")
		}
		if prev.Reason != entry.Reason {
			changed = append(changed, "reason")
		}
	}

	s.entries[key] = entry
	return PresenceUpdateResult{
		Key:         key,
		Next:        entry,
		ChangedKeys: changed,
	}
}

// List 返回所有 presence 条目。
func (s *SystemPresenceStore) List() []*PresenceEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*PresenceEntry, 0, len(s.entries))
	for _, e := range s.entries {
		result = append(result, e)
	}
	return result
}

// ---------- 心跳状态 ----------

// HeartbeatState 心跳启用状态。
type HeartbeatState struct {
	mu      sync.RWMutex
	enabled bool
	last    map[string]interface{}
}

// NewHeartbeatState 创建心跳状态。
func NewHeartbeatState() *HeartbeatState {
	return &HeartbeatState{enabled: true}
}

// IsEnabled 返回是否启用。
func (h *HeartbeatState) IsEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.enabled
}

// SetEnabled 设置启用状态。
func (h *HeartbeatState) SetEnabled(enabled bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.enabled = enabled
}

// GetLast 返回最后一次心跳事件。
func (h *HeartbeatState) GetLast() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.last
}

// SetLast 设置最后一次心跳事件。
func (h *HeartbeatState) SetLast(evt map[string]interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.last = evt
}

// ---------- System Event 队列 ----------

// SystemEventEntry 系统事件条目。
type SystemEventEntry struct {
	Text       string `json:"text"`
	SessionKey string `json:"sessionKey,omitempty"`
	ContextKey string `json:"contextKey,omitempty"`
	Timestamp  int64  `json:"timestamp"`
}

// SystemEventQueue 系统事件队列。
type SystemEventQueue struct {
	mu             sync.Mutex
	entries        []SystemEventEntry
	lastContextKey map[string]string
}

// NewSystemEventQueue 创建系统事件队列。
func NewSystemEventQueue() *SystemEventQueue {
	return &SystemEventQueue{
		entries:        make([]SystemEventEntry, 0),
		lastContextKey: make(map[string]string),
	}
}

// Enqueue 入队系统事件。
func (q *SystemEventQueue) Enqueue(text string, sessionKey, contextKey string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.entries = append(q.entries, SystemEventEntry{
		Text:       text,
		SessionKey: sessionKey,
		ContextKey: contextKey,
		Timestamp:  time.Now().UnixMilli(),
	})
	if contextKey != "" && sessionKey != "" {
		q.lastContextKey[sessionKey] = contextKey
	}
}

// IsContextChanged 检查 contextKey 是否变更。
func (q *SystemEventQueue) IsContextChanged(sessionKey, contextKey string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	last, ok := q.lastContextKey[sessionKey]
	if !ok {
		return true
	}
	return last != contextKey
}
