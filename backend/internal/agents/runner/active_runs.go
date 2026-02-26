package runner

import (
	"log/slog"
	"sync"
	"time"
)

// ---------- activeRuns 全局追踪 ----------
// TS 参考: pi-embedded-runner/runs.ts (141L)
// 防止同一 session 并发运行冲突，支持 waiter 等待机制。

// RunHandle 活跃 run 的控制句柄。
type RunHandle interface {
	QueueMessage(text string) error
	IsStreaming() bool
	IsCompacting() bool
	Abort()
}

// ActiveRunsManager 管理全局活跃 run 注册。
type ActiveRunsManager struct {
	mu      sync.RWMutex
	runs    map[string]RunHandle
	waiters map[string][]chan bool
}

// NewActiveRunsManager 创建新的 ActiveRunsManager。
func NewActiveRunsManager() *ActiveRunsManager {
	return &ActiveRunsManager{
		runs:    make(map[string]RunHandle),
		waiters: make(map[string][]chan bool),
	}
}

// DefaultActiveRuns 全局单例。
var DefaultActiveRuns = NewActiveRunsManager()

// RegisterRun 注册活跃 run。
func (m *ActiveRunsManager) RegisterRun(sessionID string, handle RunHandle) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs[sessionID] = handle
	slog.Debug("run registered", "sessionId", sessionID, "totalActive", len(m.runs))
}

// DeregisterRun 去注册活跃 run（仅 handle 匹配时）。
func (m *ActiveRunsManager) DeregisterRun(sessionID string, handle RunHandle) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if current, ok := m.runs[sessionID]; ok && current == handle {
		delete(m.runs, sessionID)
		slog.Debug("run cleared", "sessionId", sessionID, "totalActive", len(m.runs))
		// 通知所有 waiters
		if chs, ok := m.waiters[sessionID]; ok {
			for _, ch := range chs {
				ch <- true
				close(ch)
			}
			delete(m.waiters, sessionID)
		}
	}
}

// IsRunning 检查 session 是否有活跃 run。
func (m *ActiveRunsManager) IsRunning(sessionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.runs[sessionID]
	return ok
}

// IsStreaming 检查 session 的活跃 run 是否正在流式传输。
// TS 对应: runs.ts → isEmbeddedPiRunStreaming()
func (m *ActiveRunsManager) IsStreaming(sessionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	handle, ok := m.runs[sessionID]
	if !ok || handle == nil {
		return false
	}
	return handle.IsStreaming()
}

// QueueMessage 向活跃 run 发送消息。
func (m *ActiveRunsManager) QueueMessage(sessionID, text string) bool {
	m.mu.RLock()
	handle, ok := m.runs[sessionID]
	m.mu.RUnlock()
	if !ok || handle == nil || !handle.IsStreaming() || handle.IsCompacting() {
		return false
	}
	_ = handle.QueueMessage(text)
	return true
}

// AbortRun 中止活跃 run。
func (m *ActiveRunsManager) AbortRun(sessionID string) bool {
	m.mu.RLock()
	handle, ok := m.runs[sessionID]
	m.mu.RUnlock()
	if !ok || handle == nil {
		return false
	}
	handle.Abort()
	return true
}

// WaitForRunEnd 等待 run 结束，返回 true=正常结束, false=超时。
func (m *ActiveRunsManager) WaitForRunEnd(sessionID string, timeout time.Duration) bool {
	m.mu.Lock()
	if _, ok := m.runs[sessionID]; !ok {
		m.mu.Unlock()
		return true // 没有活跃 run
	}
	ch := make(chan bool, 1)
	m.waiters[sessionID] = append(m.waiters[sessionID], ch)
	m.mu.Unlock()

	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

// stubRunHandle 空 handle，用于 RunEmbeddedPiAgent 注册占位。
type stubRunHandle struct{}

func (s *stubRunHandle) QueueMessage(_ string) error { return nil }
func (s *stubRunHandle) IsStreaming() bool           { return false }
func (s *stubRunHandle) IsCompacting() bool          { return false }
func (s *stubRunHandle) Abort()                      {}
