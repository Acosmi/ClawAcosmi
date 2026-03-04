package runner

import (
	"context"
	"sync"
	"testing"
	"time"
)

// ---------- GateMode 测试 ----------

func TestPlanConfirmation_GateMode(t *testing.T) {
	mgr := NewPlanConfirmationManager(nil, nil, 5*time.Second)
	defer mgr.Close()

	// 默认 mode = full
	if m := mgr.GateMode(); m != GateModeFull {
		t.Errorf("default GateMode() = %q, want %q", m, GateModeFull)
	}

	// 合法切换
	mgr.SetGateMode(GateModeMonitor)
	if m := mgr.GateMode(); m != GateModeMonitor {
		t.Errorf("after SetGateMode(monitor), GateMode() = %q, want %q", m, GateModeMonitor)
	}

	// 非法模式保持不变
	mgr.SetGateMode("invalid_mode")
	if m := mgr.GateMode(); m != GateModeMonitor {
		t.Errorf("after SetGateMode(invalid), GateMode() = %q, want %q", m, GateModeMonitor)
	}
}

func TestPlanConfirmation_ShouldGate(t *testing.T) {
	tests := []struct {
		mode     string
		expected bool
	}{
		{GateModeFull, true},
		{GateModeSmart, true},
		{GateModeMonitor, false},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			mgr := NewPlanConfirmationManager(nil, nil, 5*time.Second)
			defer mgr.Close()
			mgr.SetGateMode(tt.mode)
			if got := mgr.ShouldGate(); got != tt.expected {
				t.Errorf("ShouldGate() with mode %q = %v, want %v", tt.mode, got, tt.expected)
			}
		})
	}
}

// ---------- RequestPlanConfirmation + ResolvePlanConfirmation 测试 ----------

func TestPlanConfirmation_Approve(t *testing.T) {
	var broadcastCalls []string
	broadcast := func(event string, payload interface{}) {
		broadcastCalls = append(broadcastCalls, event)
	}

	mgr := NewPlanConfirmationManager(broadcast, nil, 5*time.Second)
	defer mgr.Close()

	var decision PlanDecision
	var reqErr error
	done := make(chan struct{})

	go func() {
		decision, reqErr = mgr.RequestPlanConfirmation(context.Background(), PlanConfirmationRequest{
			TaskBrief:  "test task",
			IntentTier: "task_write",
		})
		close(done)
	}()

	// 等待 pending 出现
	time.Sleep(50 * time.Millisecond)
	if mgr.PendingCount() != 1 {
		t.Fatalf("PendingCount() = %d, want 1", mgr.PendingCount())
	}

	// 找到 pending ID 并 approve
	mgr.mu.Lock()
	var pendingID string
	for id := range mgr.pending {
		pendingID = id
	}
	mgr.mu.Unlock()

	if err := mgr.ResolvePlanConfirmation(pendingID, PlanDecision{Action: "approve"}); err != nil {
		t.Fatalf("ResolvePlanConfirmation error: %v", err)
	}

	<-done

	if reqErr != nil {
		t.Errorf("RequestPlanConfirmation error: %v", reqErr)
	}
	if decision.Action != "approve" {
		t.Errorf("decision.Action = %q, want %q", decision.Action, "approve")
	}
	if mgr.PendingCount() != 0 {
		t.Errorf("PendingCount after resolve = %d, want 0", mgr.PendingCount())
	}

	// 验证广播事件
	if len(broadcastCalls) < 2 {
		t.Fatalf("broadcast called %d times, want >= 2", len(broadcastCalls))
	}
	if broadcastCalls[0] != "plan.confirm.requested" {
		t.Errorf("first broadcast = %q, want %q", broadcastCalls[0], "plan.confirm.requested")
	}
	if broadcastCalls[1] != "plan.confirm.resolved" {
		t.Errorf("second broadcast = %q, want %q", broadcastCalls[1], "plan.confirm.resolved")
	}
}

func TestPlanConfirmation_Reject(t *testing.T) {
	mgr := NewPlanConfirmationManager(nil, nil, 5*time.Second)
	defer mgr.Close()

	var decision PlanDecision
	done := make(chan struct{})

	go func() {
		decision, _ = mgr.RequestPlanConfirmation(context.Background(), PlanConfirmationRequest{
			TaskBrief: "delete all files",
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	mgr.mu.Lock()
	var pendingID string
	for id := range mgr.pending {
		pendingID = id
	}
	mgr.mu.Unlock()

	_ = mgr.ResolvePlanConfirmation(pendingID, PlanDecision{
		Action:   "reject",
		Feedback: "too dangerous",
	})

	<-done

	if decision.Action != "reject" {
		t.Errorf("decision.Action = %q, want %q", decision.Action, "reject")
	}
	if decision.Feedback != "too dangerous" {
		t.Errorf("decision.Feedback = %q, want %q", decision.Feedback, "too dangerous")
	}
}

func TestPlanConfirmation_Edit(t *testing.T) {
	mgr := NewPlanConfirmationManager(nil, nil, 5*time.Second)
	defer mgr.Close()

	var decision PlanDecision
	done := make(chan struct{})

	go func() {
		decision, _ = mgr.RequestPlanConfirmation(context.Background(), PlanConfirmationRequest{
			TaskBrief: "write code",
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	mgr.mu.Lock()
	var pendingID string
	for id := range mgr.pending {
		pendingID = id
	}
	mgr.mu.Unlock()

	_ = mgr.ResolvePlanConfirmation(pendingID, PlanDecision{
		Action:     "edit",
		EditedPlan: "write code with tests",
	})

	<-done

	if decision.Action != "edit" {
		t.Errorf("decision.Action = %q, want %q", decision.Action, "edit")
	}
	if decision.EditedPlan != "write code with tests" {
		t.Errorf("decision.EditedPlan = %q, want %q", decision.EditedPlan, "write code with tests")
	}
}

// ---------- Timeout 测试 ----------

func TestPlanConfirmation_Timeout(t *testing.T) {
	mgr := NewPlanConfirmationManager(nil, nil, 200*time.Millisecond)
	defer mgr.Close()

	decision, err := mgr.RequestPlanConfirmation(context.Background(), PlanConfirmationRequest{
		TaskBrief: "slow task",
	})

	if err != nil {
		t.Fatalf("RequestPlanConfirmation error: %v", err)
	}
	if decision.Action != "reject" {
		t.Errorf("timeout decision.Action = %q, want %q", decision.Action, "reject")
	}
	if decision.Feedback != "timeout" {
		t.Errorf("timeout decision.Feedback = %q, want %q", decision.Feedback, "timeout")
	}
}

// ---------- Context Cancellation 测试 ----------

func TestPlanConfirmation_ContextCancel(t *testing.T) {
	mgr := NewPlanConfirmationManager(nil, nil, 5*time.Second)
	defer mgr.Close()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan PlanDecision)
	go func() {
		d, _ := mgr.RequestPlanConfirmation(ctx, PlanConfirmationRequest{
			TaskBrief: "cancelled task",
		})
		done <- d
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	decision := <-done
	if decision.Action != "reject" {
		t.Errorf("cancel decision.Action = %q, want %q", decision.Action, "reject")
	}
}

// ---------- ResolvePlanConfirmation 错误路径 ----------

func TestPlanConfirmation_ResolveUnknownID(t *testing.T) {
	mgr := NewPlanConfirmationManager(nil, nil, 5*time.Second)
	defer mgr.Close()

	err := mgr.ResolvePlanConfirmation("nonexistent-id", PlanDecision{Action: "approve"})
	if err == nil {
		t.Error("ResolvePlanConfirmation with unknown ID should return error")
	}
}

func TestPlanConfirmation_ResolveInvalidAction(t *testing.T) {
	mgr := NewPlanConfirmationManager(nil, nil, 5*time.Second)
	defer mgr.Close()

	err := mgr.ResolvePlanConfirmation("any-id", PlanDecision{Action: "invalid"})
	if err == nil {
		t.Error("ResolvePlanConfirmation with invalid action should return error")
	}
}

// ---------- Default Timeout 测试 ----------

func TestPlanConfirmation_DefaultTimeout(t *testing.T) {
	mgr := NewPlanConfirmationManager(nil, nil, 0)
	defer mgr.Close()

	if mgr.Timeout() != 5*time.Minute {
		t.Errorf("default Timeout() = %v, want %v", mgr.Timeout(), 5*time.Minute)
	}
}

// ---------- Close 幂等性 ----------

func TestPlanConfirmation_CloseIdempotent(t *testing.T) {
	mgr := NewPlanConfirmationManager(nil, nil, 5*time.Second)
	mgr.Close()
	mgr.Close() // 不应 panic
}

// ---------- Auto-fill ID 测试 ----------

func TestPlanConfirmation_AutoFillID(t *testing.T) {
	mgr := NewPlanConfirmationManager(nil, nil, 5*time.Second)
	defer mgr.Close()

	done := make(chan struct{})
	go func() {
		mgr.RequestPlanConfirmation(context.Background(), PlanConfirmationRequest{
			TaskBrief: "id test",
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// 收集 ID（持锁），释放后再 resolve
	mgr.mu.Lock()
	var ids []string
	for id := range mgr.pending {
		if id == "" {
			t.Error("auto-generated ID is empty")
		}
		ids = append(ids, id)
	}
	mgr.mu.Unlock()

	for _, id := range ids {
		_ = mgr.ResolvePlanConfirmation(id, PlanDecision{Action: "approve"})
	}
	<-done
}

// ---------- DecisionLogger 测试 ----------

func TestPlanConfirmation_DecisionLogger(t *testing.T) {
	var logged []PlanDecisionRecord
	var mu sync.Mutex
	logger := &mockDecisionLogger{
		logFn: func(r PlanDecisionRecord) error {
			mu.Lock()
			logged = append(logged, r)
			mu.Unlock()
			return nil
		},
	}

	mgr := NewPlanConfirmationManager(nil, nil, 5*time.Second)
	defer mgr.Close()
	mgr.SetDecisionLogger(logger)

	done := make(chan struct{})
	go func() {
		mgr.RequestPlanConfirmation(context.Background(), PlanConfirmationRequest{
			TaskBrief: "logged task",
		})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	mgr.mu.Lock()
	var pendingID string
	for id := range mgr.pending {
		pendingID = id
	}
	mgr.mu.Unlock()

	_ = mgr.ResolvePlanConfirmation(pendingID, PlanDecision{Action: "approve"})
	<-done

	mu.Lock()
	defer mu.Unlock()
	if len(logged) != 1 {
		t.Fatalf("logged %d records, want 1", len(logged))
	}
	if logged[0].Decision.Action != "approve" {
		t.Errorf("logged action = %q, want %q", logged[0].Decision.Action, "approve")
	}
}

type mockDecisionLogger struct {
	logFn func(PlanDecisionRecord) error
}

func (m *mockDecisionLogger) LogPlanDecision(r PlanDecisionRecord) error {
	if m.logFn != nil {
		return m.logFn(r)
	}
	return nil
}
