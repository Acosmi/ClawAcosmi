package runner

// plan_confirmation.go — 三级指挥体系 Phase 1: 方案确认门控
//
// task_write / task_delete / task_multimodal 意图下，
// 主智能体先生成方案 → 用户批准 → 才执行。
//
// 复用 CoderConfirmationManager 的阻塞 channel 模式。
// 行业对标:
//   - LangGraph: interrupt() + checkpoint（R4 TTL 清理借鉴 checkpoint 机制）
//   - Anthropic: Oversight Paradox（R5 GateMode 预留 smart/monitor 模式）
//   - OpenAI Agents SDK: Guardrails tripwire（紧急中止能力）

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ---------- 请求 / 决策类型 ----------

// PlanConfirmationRequest 方案确认请求。
type PlanConfirmationRequest struct {
	ID             string       `json:"id"`
	TaskBrief      string       `json:"taskBrief"`
	PlanSteps      []string     `json:"planSteps"`
	EstimatedScope []ScopeEntry `json:"estimatedScope,omitempty"`
	IntentTier     string       `json:"intentTier"`
	CreatedAtMs    int64        `json:"createdAtMs"`
	ExpiresAtMs    int64        `json:"expiresAtMs"`
}

// PlanDecision 用户对方案的决策。
type PlanDecision struct {
	Action     string `json:"action"`               // "approve" | "reject" | "edit"
	EditedPlan string `json:"editedPlan,omitempty"` // action=edit 时的修改方案
	Feedback   string `json:"feedback,omitempty"`   // action=reject 时的拒绝原因
}

// PlanDecisionRecord 决策记录（用于 VFS 持久化 [R9]）。
type PlanDecisionRecord struct {
	RequestID   string       `json:"requestId"`
	TaskBrief   string       `json:"taskBrief"`
	PlanSteps   []string     `json:"planSteps"`
	IntentTier  string       `json:"intentTier"`
	Decision    PlanDecision `json:"decision"`
	DecidedAtMs int64        `json:"decidedAtMs"`
}

// PlanDecisionLogger 决策持久化接口（[R9] 由 gateway 注入 VFS 实现）。
type PlanDecisionLogger interface {
	LogPlanDecision(record PlanDecisionRecord) error
}

// PlanConfirmRemoteNotifyFunc 方案确认远程通知回调（飞书/钉钉等非 Web 渠道）。
// sessionKey 用于确定目标渠道（如 "feishu:<chatID>"），空字符串表示广播到所有已配置渠道。
type PlanConfirmRemoteNotifyFunc func(req PlanConfirmationRequest, sessionKey string)

// ---------- GateMode [R5] ----------

const (
	// GateModeFull 全量门控：所有 task_write+ 意图都弹窗确认。
	GateModeFull = "full"
	// GateModeSmart 智能门控：低风险自动通过，高风险才弹窗（未来实现）。
	GateModeSmart = "smart"
	// GateModeMonitor 监控门控：全部自动通过，用户可随时干预（Anthropic 推荐终态）。
	GateModeMonitor = "monitor"
)

// ---------- Manager ----------

// planConfirmationEntry 内部 pending 条目（含过期时间和决策 channel）。
type planConfirmationEntry struct {
	ch        chan PlanDecision
	expiresAt time.Time
}

// PlanConfirmationManager 方案确认管理器。
// 当主智能体生成方案后:
//  1. 广播 "plan.confirm.requested" 给前端（WebSocket）
//  2. 阻塞等待用户决策（approve/reject/edit）或超时
//  3. 前端通过 "plan.confirm.resolve" RPC 回调
//
// 为 nil 时完全跳过方案确认（兼容现有行为）。
type PlanConfirmationManager struct {
	mu      sync.Mutex
	pending map[string]*planConfirmationEntry // id → pending entry (含过期时间)

	broadcast      CoderConfirmBroadcastFunc   // 复用 coder 广播类型（解耦 runner ↔ gateway）
	remoteNotify   PlanConfirmRemoteNotifyFunc // 远程通知回调（飞书卡片等），可为 nil
	decisionLogger PlanDecisionLogger          // [R9] 可选，nil = 不记录决策

	timeout  time.Duration // 默认 5min
	gateMode string        // [R5] "full" | "smart" | "monitor"

	// TTL 清理 [R4]
	closeOnce   sync.Once     // 防止 Close() 重复调用 panic
	cleanupDone chan struct{} // 关闭时停止 TTL 清理 goroutine
}

// NewPlanConfirmationManager 创建方案确认管理器。
func NewPlanConfirmationManager(
	broadcastFn CoderConfirmBroadcastFunc,
	remoteNotifyFn PlanConfirmRemoteNotifyFunc,
	timeout time.Duration,
) *PlanConfirmationManager {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	m := &PlanConfirmationManager{
		pending:      make(map[string]*planConfirmationEntry),
		broadcast:    broadcastFn,
		remoteNotify: remoteNotifyFn,
		timeout:      timeout,
		gateMode:     GateModeFull, // Phase 1 硬编码 full
		cleanupDone:  make(chan struct{}),
	}
	// [R4] 启动 TTL 清理 goroutine
	go m.ttlCleanupLoop()
	return m
}

// SetDecisionLogger 设置决策持久化日志器 [R9]。
func (m *PlanConfirmationManager) SetDecisionLogger(logger PlanDecisionLogger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.decisionLogger = logger
}

// SetGateMode 设置门控模式 [R5]。
func (m *PlanConfirmationManager) SetGateMode(mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch mode {
	case GateModeFull, GateModeSmart, GateModeMonitor:
		m.gateMode = mode
	default:
		slog.Warn("invalid gate mode, keeping current", "mode", mode, "current", m.gateMode)
	}
}

// GateMode 返回当前门控模式。
func (m *PlanConfirmationManager) GateMode() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.gateMode
}

// ShouldGate 判断当前模式是否需要门控（full 需要，monitor 不需要，smart 未来按风险判断）。
func (m *PlanConfirmationManager) ShouldGate() bool {
	mode := m.GateMode()
	switch mode {
	case GateModeMonitor:
		return false
	case GateModeSmart:
		// TODO: Phase L3 — 按 intentTier + 任务风险动态判断
		return true // 暂时等同 full
	default:
		return true
	}
}

// RequestPlanConfirmation 请求用户确认执行方案。
// 阻塞直到用户决策、超时或 ctx 取消。
// [R1] ctx 应为独立 context（不复用 RunAttempt timeout）。
func (m *PlanConfirmationManager) RequestPlanConfirmation(ctx context.Context, req PlanConfirmationRequest) (PlanDecision, error) {
	// 填充默认字段
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	now := time.Now()
	if req.CreatedAtMs == 0 {
		req.CreatedAtMs = now.UnixMilli()
	}
	if req.ExpiresAtMs == 0 {
		req.ExpiresAtMs = now.Add(m.timeout).UnixMilli()
	}

	ch := make(chan PlanDecision, 1)

	m.mu.Lock()
	m.pending[req.ID] = &planConfirmationEntry{
		ch:        ch,
		expiresAt: time.UnixMilli(req.ExpiresAtMs),
	}
	m.mu.Unlock()

	// 广播方案确认请求到前端（WebSocket）
	if m.broadcast != nil {
		m.broadcast("plan.confirm.requested", req)
	}

	// 推送远程通知到非 Web 渠道（飞书卡片等）
	if m.remoteNotify != nil {
		m.remoteNotify(req, "")
	}

	slog.Debug("plan confirmation requested",
		"id", req.ID,
		"tier", req.IntentTier,
		"steps", len(req.PlanSteps),
	)

	// 等待用户决策、超时或 ctx 取消
	timer := time.NewTimer(m.timeout)
	defer timer.Stop()

	var decision PlanDecision
	select {
	case decision = <-ch:
		// 用户已决策
	case <-timer.C:
		decision = PlanDecision{Action: "reject", Feedback: "timeout"}
		slog.Info("plan confirmation timed out, auto-rejecting",
			"id", req.ID,
		)
	case <-ctx.Done():
		decision = PlanDecision{Action: "reject", Feedback: "context cancelled"}
		slog.Debug("plan confirmation cancelled by context",
			"id", req.ID,
		)
	}

	// 清理 pending
	m.mu.Lock()
	delete(m.pending, req.ID)
	logger := m.decisionLogger
	m.mu.Unlock()

	// 广播决策结果
	if m.broadcast != nil {
		m.broadcast("plan.confirm.resolved", map[string]interface{}{
			"id":       req.ID,
			"decision": decision,
			"ts":       time.Now().UnixMilli(),
		})
	}

	// [R9] 决策记录
	if logger != nil {
		record := PlanDecisionRecord{
			RequestID:   req.ID,
			TaskBrief:   req.TaskBrief,
			PlanSteps:   req.PlanSteps,
			IntentTier:  req.IntentTier,
			Decision:    decision,
			DecidedAtMs: time.Now().UnixMilli(),
		}
		if logErr := logger.LogPlanDecision(record); logErr != nil {
			slog.Warn("failed to log plan decision", "error", logErr, "id", req.ID)
		}
	}

	return decision, nil
}

// ResolvePlanConfirmation 处理前端的方案确认决策回调。
// 由 WebSocket RPC "plan.confirm.resolve" 调用。
func (m *PlanConfirmationManager) ResolvePlanConfirmation(id string, decision PlanDecision) error {
	if decision.Action != "approve" && decision.Action != "reject" && decision.Action != "edit" {
		return fmt.Errorf("invalid decision action: %q (expected approve/reject/edit)", decision.Action)
	}

	m.mu.Lock()
	entry, ok := m.pending[id]
	m.mu.Unlock()

	if !ok {
		return fmt.Errorf("no pending plan confirmation with id: %s", id)
	}

	// 非阻塞写入（channel 有 1 缓冲）
	select {
	case entry.ch <- decision:
		slog.Debug("plan confirmation resolved",
			"id", id,
			"action", decision.Action,
		)
	default:
		// channel 已被写入（超时或重复调用），忽略
	}

	return nil
}

// Close 关闭管理器，停止 TTL 清理 goroutine。安全支持重复调用。
func (m *PlanConfirmationManager) Close() {
	m.closeOnce.Do(func() {
		close(m.cleanupDone)
	})
}

// ---------- TTL 清理 [R4] ----------

// ttlCleanupLoop 后台 goroutine 每 1min 扫描 pending map，
// 删除超过 TTL 的条目并关闭 channel（防止 goroutine 泄漏）。
func (m *PlanConfirmationManager) ttlCleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.cleanupDone:
			return
		case <-ticker.C:
			m.cleanupExpired()
		}
	}
}

// cleanupExpired 清理超过 TTL 的 pending 条目。
// 仅清理已过期条目，未过期条目保留（修复: 之前未检查过期时间导致提前清理）。
func (m *PlanConfirmationManager) cleanupExpired() {
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, entry := range m.pending {
		if now.Before(entry.expiresAt) {
			continue // 未过期，跳过
		}
		// 过期 — auto-reject
		select {
		case entry.ch <- PlanDecision{Action: "reject", Feedback: "ttl_expired"}:
			slog.Debug("plan confirmation TTL expired, auto-rejected", "id", id)
		default:
			// channel 已有决策（timer 先触发），只清理 map
		}
		delete(m.pending, id)
	}
}

// Timeout 返回确认超时时间。
func (m *PlanConfirmationManager) Timeout() time.Duration {
	return m.timeout
}

// PendingCount 返回当前等待确认的请求数（用于监控）。
func (m *PlanConfirmationManager) PendingCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pending)
}
