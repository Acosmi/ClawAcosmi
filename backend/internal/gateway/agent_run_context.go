package gateway

import (
	"sync"
	"sync/atomic"
)

// ---------- Agent 运行上下文存储 (移植自 agent-events.ts) ----------

// AgentRunContext 单次 agent 运行的上下文信息。
type AgentRunContext struct {
	RunID        string `json:"runId"`
	SessionKey   string `json:"sessionKey"`
	IsHeartbeat  bool   `json:"isHeartbeat"`
	VerboseLevel string `json:"verboseLevel"` // "off" | "compact" | "full"
}

// AgentRunContextStore 线程安全的 agent 运行上下文管理器。
// 移植自 TS agent-events.ts 的 runContextById Map 和 seqByRun Map。
type AgentRunContextStore struct {
	contexts  sync.Map     // runID → *AgentRunContext
	seqs      sync.Map     // runID → *atomic.Int64
	mu        sync.RWMutex // 保护 listeners
	listeners []AgentEventListener
}

// AgentEventListener agent 事件监听器。
type AgentEventListener func(evt AgentEvent)

// AgentEvent 包含 seq 和 ts 的 agent 事件。
type AgentEvent struct {
	RunID string                 `json:"runId"`
	Seq   int64                  `json:"seq"`
	Ts    int64                  `json:"ts"`
	Type  string                 `json:"type"`
	Data  map[string]interface{} `json:"data,omitempty"`
}

// NewAgentRunContextStore 创建新的上下文存储。
func NewAgentRunContextStore() *AgentRunContextStore {
	return &AgentRunContextStore{}
}

// Register 注册一个 agent 运行上下文。
func (s *AgentRunContextStore) Register(ctx *AgentRunContext) {
	s.contexts.Store(ctx.RunID, ctx)
	var seq atomic.Int64
	s.seqs.Store(ctx.RunID, &seq)
}

// Get 获取指定 runID 的上下文。
func (s *AgentRunContextStore) Get(runID string) *AgentRunContext {
	v, ok := s.contexts.Load(runID)
	if !ok {
		return nil
	}
	return v.(*AgentRunContext)
}

// NextSeq 获取并递增指定 runID 的序列号。
func (s *AgentRunContextStore) NextSeq(runID string) int64 {
	v, ok := s.seqs.Load(runID)
	if !ok {
		return 0
	}
	return v.(*atomic.Int64).Add(1)
}

// Clear 清除指定 runID 的上下文。
func (s *AgentRunContextStore) Clear(runID string) {
	s.contexts.Delete(runID)
	s.seqs.Delete(runID)
}

// Reset 清除所有上下文（用于测试）。
func (s *AgentRunContextStore) Reset() {
	s.contexts.Range(func(key, _ interface{}) bool {
		s.contexts.Delete(key)
		return true
	})
	s.seqs.Range(func(key, _ interface{}) bool {
		s.seqs.Delete(key)
		return true
	})
	s.mu.Lock()
	s.listeners = nil
	s.mu.Unlock()
}

// AddListener 添加事件监听器。
func (s *AgentRunContextStore) AddListener(fn AgentEventListener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listeners = append(s.listeners, fn)
}

// Emit 发送事件到所有监听器。
func (s *AgentRunContextStore) Emit(evt AgentEvent) {
	s.mu.RLock()
	listeners := make([]AgentEventListener, len(s.listeners))
	copy(listeners, s.listeners)
	s.mu.RUnlock()
	for _, fn := range listeners {
		fn(evt)
	}
}
