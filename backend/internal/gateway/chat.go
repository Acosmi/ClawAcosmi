package gateway

import (
	"log/slog"
	"strings"
	"sync"
	"time"
)

// ---------- Verbose Level 规范化 (移植自 auto-reply/thinking.ts) ----------

// NormalizeVerboseLevel 将 verbose 配置规范化为 "off" | "on" | "full" | ""。
// 对齐 TS auto-reply/thinking.ts:normalizeVerboseLevel 的行为：
//   - "off"|"false"|"no"|"0" → "off"
//   - "on"|"minimal"|"true"|"yes"|"1" → "on"
//   - "full"|"all"|"everything" → "full"
//   - 其他/空值 → ""（未定义）
func NormalizeVerboseLevel(v string) string {
	key := strings.ToLower(strings.TrimSpace(v))
	switch key {
	case "off", "false", "no", "0":
		return "off"
	case "on", "minimal", "true", "yes", "1":
		return "on"
	case "full", "all", "everything":
		return "full"
	case "":
		return ""
	default:
		return ""
	}
}

// ---------- Agent 事件处理器 (移植自 server-chat.ts:createAgentEventHandler) ----------

const (
	deltaThrottleMs = 150 // 增量广播节流间隔
)

// AgentEventHandlerDeps 事件处理器的依赖注入接口。
type AgentEventHandlerDeps struct {
	Broadcaster    *Broadcaster
	ContextStore   *AgentRunContextStore
	ChatState      *ChatRunState
	ToolRecipients *ToolEventRecipientRegistry
	Logger         *slog.Logger
}

// AgentEventHandler 处理 agent 事件的广播和状态管理。
// 等价于 TS createAgentEventHandler 的完整逻辑。
type AgentEventHandler struct {
	deps         AgentEventHandlerDeps
	sessionKey   string
	runID        string
	lastSeq      int64
	verboseLevel string
	mu           sync.Mutex
}

// NewAgentEventHandler 创建事件处理器。
func NewAgentEventHandler(deps AgentEventHandlerDeps, sessionKey, runID, verboseLevel string) *AgentEventHandler {
	return &AgentEventHandler{
		deps:         deps,
		sessionKey:   sessionKey,
		runID:        runID,
		verboseLevel: NormalizeVerboseLevel(verboseLevel),
	}
}

// Handle 处理单个 agent 事件。
func (h *AgentEventHandler) Handle(evt AgentEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()

	logger := h.deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// seq gap 检测
	if evt.Seq > 0 && h.lastSeq > 0 && evt.Seq != h.lastSeq+1 {
		logger.Warn("seq gap detected",
			"runId", h.runID,
			"expected", h.lastSeq+1,
			"got", evt.Seq,
		)
	}
	if evt.Seq > 0 {
		h.lastSeq = evt.Seq
	}

	switch evt.Type {
	case "chat.delta":
		h.handleDelta(evt)
	case "chat.final":
		h.handleFinal(evt)
	case "chat.error":
		h.handleError(evt)
	case "chat.abort":
		h.handleAbort(evt)
	case "tool.start", "tool.result":
		h.handleToolEvent(evt)
	default:
		// 其他事件类型直接广播
		h.broadcastEvent(evt, nil)
	}
}

// handleDelta 处理增量消息（带节流）。
func (h *AgentEventHandler) handleDelta(evt AgentEvent) {
	// 累积到 buffer（以 runID 为 key，避免同一 session 并发 run 互相覆盖）
	if text, ok := evt.Data["text"].(string); ok {
		h.appendBuffer(text)
	}

	// 节流控制：每 deltaThrottleMs 才发一次
	if v, ok := h.deps.ChatState.DeltaSentAt.Load(h.runID); ok {
		lastSent := v.(int64)
		if time.Now().UnixMilli()-lastSent < deltaThrottleMs {
			return
		}
	}

	h.deps.ChatState.DeltaSentAt.Store(h.runID, time.Now().UnixMilli())

	// 心跳运行且 showOk=false 时，不向 webchat 广播
	if h.ShouldSuppressHeartbeat() {
		return
	}
	h.broadcastEvent(evt, nil)
}

// handleFinal 处理最终消息。
func (h *AgentEventHandler) handleFinal(evt AgentEvent) {
	// 清空 delta buffer（以 runID 为 key）
	h.deps.ChatState.Buffers.Delete(h.runID)
	h.deps.ChatState.DeltaSentAt.Delete(h.runID)
	// 标记工具事件接收者为 final
	if h.deps.ToolRecipients != nil {
		h.deps.ToolRecipients.MarkFinal(h.runID)
	}
	// 心跳运行且 showOk=false 时，不向 webchat 广播
	if h.ShouldSuppressHeartbeat() {
		return
	}
	h.broadcastEvent(evt, nil)
}

// handleError 处理错误事件。
func (h *AgentEventHandler) handleError(evt AgentEvent) {
	h.deps.ChatState.Buffers.Delete(h.runID)
	h.deps.ChatState.DeltaSentAt.Delete(h.runID)
	if h.deps.ToolRecipients != nil {
		h.deps.ToolRecipients.MarkFinal(h.runID)
	}
	h.broadcastEvent(evt, nil)
}

// handleAbort 处理 abort 事件。
func (h *AgentEventHandler) handleAbort(evt AgentEvent) {
	h.deps.ChatState.Buffers.Delete(h.runID)
	h.deps.ChatState.DeltaSentAt.Delete(h.runID)
	h.deps.ChatState.AbortedRuns.Store(h.runID, time.Now().UnixMilli())
	if h.deps.ToolRecipients != nil {
		h.deps.ToolRecipients.MarkFinal(h.runID)
	}
	h.broadcastEvent(evt, nil)
}

// handleToolEvent 处理工具开始/结果事件。
func (h *AgentEventHandler) handleToolEvent(evt AgentEvent) {
	if h.verboseLevel == "off" || h.verboseLevel == "" {
		return // 静默模式或未定义时不广播工具事件
	}
	// 指定接收者
	var targets map[string]struct{}
	if h.deps.ToolRecipients != nil {
		targets = h.deps.ToolRecipients.Get(h.runID)
	}

	// on（非 full）模式下移除 result/partialResult 字段
	if h.verboseLevel != "full" && evt.Type == "tool.result" {
		filtered := make(map[string]interface{})
		for k, v := range evt.Data {
			if k != "result" && k != "partialResult" {
				filtered[k] = v
			}
		}
		evt.Data = filtered
	}

	h.broadcastEvent(evt, targets)
}

// ShouldSuppressHeartbeat 判断是否应抑制心跳广播。
func (h *AgentEventHandler) ShouldSuppressHeartbeat() bool {
	ctx := h.deps.ContextStore.Get(h.runID)
	return ctx != nil && ctx.IsHeartbeat
}

// appendBuffer 追加文本到 run buffer（以 runID 为 key）。
func (h *AgentEventHandler) appendBuffer(text string) {
	if v, ok := h.deps.ChatState.Buffers.Load(h.runID); ok {
		h.deps.ChatState.Buffers.Store(h.runID, v.(string)+text)
	} else {
		h.deps.ChatState.Buffers.Store(h.runID, text)
	}
}

// broadcastEvent 通过 Broadcaster 广播事件。
func (h *AgentEventHandler) broadcastEvent(evt AgentEvent, targets map[string]struct{}) {
	if h.deps.Broadcaster == nil {
		return
	}
	opts := &BroadcastOptions{DropIfSlow: true}
	if len(targets) > 0 {
		h.deps.Broadcaster.BroadcastToConnIDs(evt.Type, evt.Data, targets, opts)
	} else {
		h.deps.Broadcaster.Broadcast(evt.Type, evt.Data, opts)
	}
}

// ---------- Chat 运行注册表 ----------

// ChatRunEntry 聊天运行条目。
type ChatRunEntry struct {
	SessionKey  string
	ClientRunID string
}

// ChatRunRegistry 管理 sessionId → ChatRunEntry 队列的映射。
type ChatRunRegistry struct {
	mu       sync.Mutex
	sessions map[string][]ChatRunEntry
}

// NewChatRunRegistry 创建聊天运行注册表。
func NewChatRunRegistry() *ChatRunRegistry {
	return &ChatRunRegistry{sessions: make(map[string][]ChatRunEntry)}
}

// Add 添加运行条目。
func (r *ChatRunRegistry) Add(sessionID string, entry ChatRunEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[sessionID] = append(r.sessions[sessionID], entry)
}

// Peek 查看队列头部条目（不移除）。
func (r *ChatRunRegistry) Peek(sessionID string) *ChatRunEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	q := r.sessions[sessionID]
	if len(q) == 0 {
		return nil
	}
	e := q[0]
	return &e
}

// Shift 从队列头部取出条目。
func (r *ChatRunRegistry) Shift(sessionID string) *ChatRunEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	q := r.sessions[sessionID]
	if len(q) == 0 {
		return nil
	}
	e := q[0]
	r.sessions[sessionID] = q[1:]
	if len(r.sessions[sessionID]) == 0 {
		delete(r.sessions, sessionID)
	}
	return &e
}

// Remove 按 clientRunId 移除特定条目。
func (r *ChatRunRegistry) Remove(sessionID, clientRunID string, sessionKey string) *ChatRunEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	q := r.sessions[sessionID]
	for i, entry := range q {
		if entry.ClientRunID == clientRunID && (sessionKey == "" || entry.SessionKey == sessionKey) {
			r.sessions[sessionID] = append(q[:i], q[i+1:]...)
			if len(r.sessions[sessionID]) == 0 {
				delete(r.sessions, sessionID)
			}
			return &entry
		}
	}
	return nil
}

// Clear 清空所有条目。
func (r *ChatRunRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions = make(map[string][]ChatRunEntry)
}

// ---------- Chat Abort Controller ----------
// 对齐 TS chat-abort.ts: ChatAbortControllerEntry

// ChatAbortControllerEntry 聊天运行的 abort 控制器条目。
// 对齐 TS: { controller: AbortController; sessionId; sessionKey; startedAtMs; expiresAtMs }
// Go 使用 context.CancelFunc 替代 AbortController。
type ChatAbortControllerEntry struct {
	Cancel      func()
	SessionKey  string
	StartedAtMs int64
	ExpiresAtMs int64
}

// ResolveChatRunExpiresAtMs 计算聊天运行的过期时间。
// 对齐 TS chat-abort.ts: resolveChatRunExpiresAtMs()
func ResolveChatRunExpiresAtMs(nowMs, timeoutMs int64) int64 {
	const (
		graceMs = 60_000           // 60s grace period
		minMs   = 2 * 60_000       // 最少 2 分钟
		maxMs   = 24 * 60 * 60_000 // 最多 24 小时
	)
	bounded := timeoutMs
	if bounded < 0 {
		bounded = 0
	}
	target := nowMs + bounded + graceMs
	min := nowMs + minMs
	max := nowMs + maxMs
	if target < min {
		target = min
	}
	if target > max {
		target = max
	}
	return target
}

// ---------- Chat 运行状态 ----------

// ChatRunState 聊天运行全局状态。
type ChatRunState struct {
	Registry         *ChatRunRegistry
	Buffers          sync.Map // runId → string
	DeltaSentAt      sync.Map // runId → int64 (时间戳 ms)
	AbortedRuns      sync.Map // runId → int64 (时间戳 ms)
	AbortControllers sync.Map // runId → *ChatAbortControllerEntry
}

// NewChatRunState 创建聊天运行状态。
func NewChatRunState() *ChatRunState {
	return &ChatRunState{
		Registry: NewChatRunRegistry(),
	}
}

// Clear 清空所有状态。
func (s *ChatRunState) Clear() {
	s.Registry.Clear()
	s.Buffers = sync.Map{}
	s.DeltaSentAt = sync.Map{}
	s.AbortedRuns = sync.Map{}
	s.AbortControllers = sync.Map{}
}

// ---------- 工具事件接收者注册表 ----------

const (
	toolRecipientTTL        = 10 * time.Minute
	toolRecipientFinalGrace = 30 * time.Second
)

type toolRecipientEntry struct {
	connIDs     map[string]struct{}
	updatedAt   time.Time
	finalizedAt *time.Time
}

// ToolEventRecipientRegistry 管理 runId → 连接 ID 集合的映射（带 TTL 自动清理）。
type ToolEventRecipientRegistry struct {
	mu         sync.Mutex
	recipients map[string]*toolRecipientEntry
}

// NewToolEventRecipientRegistry 创建工具事件接收者注册表。
func NewToolEventRecipientRegistry() *ToolEventRecipientRegistry {
	return &ToolEventRecipientRegistry{
		recipients: make(map[string]*toolRecipientEntry),
	}
}

// Add 注册 runId 对应的连接 ID。
func (r *ToolEventRecipientRegistry) Add(runID, connID string) {
	if runID == "" || connID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	entry, exists := r.recipients[runID]
	if exists {
		entry.connIDs[connID] = struct{}{}
		entry.updatedAt = now
	} else {
		r.recipients[runID] = &toolRecipientEntry{
			connIDs:   map[string]struct{}{connID: {}},
			updatedAt: now,
		}
	}
	r.pruneLocked(now)
}

// Get 获取 runId 对应的连接 ID 集合。
func (r *ToolEventRecipientRegistry) Get(runID string) map[string]struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, exists := r.recipients[runID]
	if !exists {
		return nil
	}
	entry.updatedAt = time.Now()
	r.pruneLocked(entry.updatedAt)
	// 返回副本
	result := make(map[string]struct{}, len(entry.connIDs))
	for k := range entry.connIDs {
		result[k] = struct{}{}
	}
	return result
}

// MarkFinal 标记 runId 已完成（开始 grace period 倒计时）。
func (r *ToolEventRecipientRegistry) MarkFinal(runID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, exists := r.recipients[runID]
	if !exists {
		return
	}
	now := time.Now()
	entry.finalizedAt = &now
	r.pruneLocked(now)
}

func (r *ToolEventRecipientRegistry) pruneLocked(now time.Time) {
	for runID, entry := range r.recipients {
		var cutoff time.Time
		if entry.finalizedAt != nil {
			cutoff = entry.finalizedAt.Add(toolRecipientFinalGrace)
		} else {
			cutoff = entry.updatedAt.Add(toolRecipientTTL)
		}
		if now.After(cutoff) || now.Equal(cutoff) {
			delete(r.recipients, runID)
		}
	}
}
