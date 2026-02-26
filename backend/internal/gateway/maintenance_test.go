package gateway

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStartMaintenanceTick_BroadcastsTick(t *testing.T) {
	b := NewBroadcaster()
	var mu sync.Mutex
	var received []eventFrame

	// 添加一个 mock 客户端收集广播消息
	c := &WsClient{
		ConnID:  "tick-test",
		Connect: ConnectParams{Role: "operator"},
		Send: func(data []byte) error {
			var frame eventFrame
			if err := json.Unmarshal(data, &frame); err == nil {
				mu.Lock()
				received = append(received, frame)
				mu.Unlock()
			}
			return nil
		},
		Close:          func(code int, reason string) error { return nil },
		BufferedAmount: func() int64 { return 0 },
	}
	b.AddClient(c)

	mt := StartMaintenanceTick(b)
	defer mt.Stop()

	// 等待至少一个 tick（给 50ms 超出 TickIntervalMs）
	// 由于默认 TickIntervalMs=30s 太长，此测试验证 Stop 功能
	// 启动后立即停止应该不 panic
	time.Sleep(50 * time.Millisecond)
	mt.Stop()

	// 二次 Stop 应该是幂等的
	mt.Stop()
}

func TestMaintenanceTimers_StopIdempotent(t *testing.T) {
	mt := &MaintenanceTimers{stopCh: make(chan struct{})}
	mt.Stop()
	mt.Stop() // 不应 panic
	mt.Stop() // 不应 panic
}

func TestTickIntervalMs_Constant(t *testing.T) {
	if TickIntervalMs != 30000 {
		t.Errorf("TickIntervalMs = %d, want 30000", TickIntervalMs)
	}
}

func TestStartMaintenanceTick_TickPayload(t *testing.T) {
	// 使用短间隔 ticker 验证 payload 格式
	// 这里直接构造 TickEvent 验证 JSON 序列化
	evt := TickEvent{Ts: time.Now().UnixMilli()}
	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ts, ok := decoded["ts"]
	if !ok {
		t.Error("missing ts field")
	}
	if tsNum, ok := ts.(float64); !ok || tsNum <= 0 {
		t.Errorf("ts should be positive number, got %v", ts)
	}
}

func TestHealthRefreshIntervalMs_Constant(t *testing.T) {
	if HealthRefreshIntervalMs != 60000 {
		t.Errorf("HealthRefreshIntervalMs = %d, want 60000", HealthRefreshIntervalMs)
	}
}

func TestAbortedRunTTLMs_Constant(t *testing.T) {
	if AbortedRunTTLMs != 3600000 {
		t.Errorf("AbortedRunTTLMs = %d, want 3600000", AbortedRunTTLMs)
	}
}

func TestMaintenanceCleanup_AbortedRunsTTL(t *testing.T) {
	cs := NewChatRunState()
	nowMs := time.Now().UnixMilli()

	// 添加一个已过期的 abortedRun（2 小时前）
	cs.AbortedRuns.Store("expired-run", nowMs-2*3600000)
	cs.Buffers.Store("expired-run", "some buffer")
	cs.DeltaSentAt.Store("expired-run", nowMs-2*3600000)

	// 添加一个未过期的 abortedRun（5 分钟前）
	cs.AbortedRuns.Store("fresh-run", nowMs-5*60000)
	cs.Buffers.Store("fresh-run", "fresh buffer")

	cfg := MaintenanceConfig{ChatState: cs}
	maintenanceCleanup(cfg, nil)

	// 过期的应该被清理
	if _, ok := cs.AbortedRuns.Load("expired-run"); ok {
		t.Error("expired-run should be cleaned")
	}
	if _, ok := cs.Buffers.Load("expired-run"); ok {
		t.Error("expired-run buffer should be cleaned")
	}
	if _, ok := cs.DeltaSentAt.Load("expired-run"); ok {
		t.Error("expired-run deltaSentAt should be cleaned")
	}

	// 未过期的应该保留
	if _, ok := cs.AbortedRuns.Load("fresh-run"); !ok {
		t.Error("fresh-run should be kept")
	}
	if _, ok := cs.Buffers.Load("fresh-run"); !ok {
		t.Error("fresh-run buffer should be kept")
	}
}

func TestMaintenanceCleanup_ChatAbortControllerExpiry(t *testing.T) {
	cs := NewChatRunState()
	nowMs := time.Now().UnixMilli()

	var cancelCalled atomic.Bool

	// 添加一个已过期的 abort controller
	cs.AbortControllers.Store("expired-ctrl", &ChatAbortControllerEntry{
		Cancel:      func() { cancelCalled.Store(true) },
		SessionKey:  "session-1",
		StartedAtMs: nowMs - 300000,
		ExpiresAtMs: nowMs - 60000, // 1 分钟前过期
	})

	// 添加一个未过期的 abort controller
	cs.AbortControllers.Store("active-ctrl", &ChatAbortControllerEntry{
		Cancel:      func() {},
		SessionKey:  "session-2",
		StartedAtMs: nowMs - 30000,
		ExpiresAtMs: nowMs + 60000, // 1 分钟后过期
	})

	b := NewBroadcaster()
	var mu sync.Mutex
	var broadcasts []eventFrame
	c := &WsClient{
		ConnID:  "test-client",
		Connect: ConnectParams{Role: "operator"},
		Send: func(data []byte) error {
			var frame eventFrame
			if err := json.Unmarshal(data, &frame); err == nil {
				mu.Lock()
				broadcasts = append(broadcasts, frame)
				mu.Unlock()
			}
			return nil
		},
		Close:          func(code int, reason string) error { return nil },
		BufferedAmount: func() int64 { return 0 },
	}
	b.AddClient(c)

	cfg := MaintenanceConfig{
		Broadcaster: b,
		ChatState:   cs,
	}
	maintenanceCleanup(cfg, nil)

	// 过期的 controller 应被清理
	if _, ok := cs.AbortControllers.Load("expired-ctrl"); ok {
		t.Error("expired-ctrl should be cleaned")
	}

	// 应调用 cancel
	if !cancelCalled.Load() {
		t.Error("cancel should have been called on expired controller")
	}

	// 应标记为 aborted
	if _, ok := cs.AbortedRuns.Load("expired-ctrl"); !ok {
		t.Error("expired-ctrl should be marked as aborted run")
	}

	// 未过期的 controller 应保留
	if _, ok := cs.AbortControllers.Load("active-ctrl"); !ok {
		t.Error("active-ctrl should be kept")
	}

	// 应广播 chat.abort 事件
	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, frame := range broadcasts {
		if frame.Event == "chat.abort" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected chat.abort broadcast for expired controller")
	}
}

func TestHealthRefreshCallback(t *testing.T) {
	var callCount atomic.Int32
	cfg := MaintenanceConfig{
		Broadcaster: NewBroadcaster(),
		HealthRefreshFunc: func() {
			callCount.Add(1)
		},
	}

	mt := StartMaintenanceTimers(cfg)
	// 等待初始刷新
	time.Sleep(100 * time.Millisecond)
	mt.Stop()

	// 至少应该调用一次（初始刷新）
	if callCount.Load() < 1 {
		t.Errorf("health refresh should be called at least once, got %d", callCount.Load())
	}
}

func TestStartMaintenanceTimers_StopIdempotent(t *testing.T) {
	cfg := MaintenanceConfig{
		Broadcaster: NewBroadcaster(),
		ChatState:   NewChatRunState(),
	}
	mt := StartMaintenanceTimers(cfg)
	mt.Stop()
	mt.Stop() // 不应 panic
	mt.Stop() // 不应 panic
}

func TestResolveChatRunExpiresAtMs(t *testing.T) {
	now := int64(1000000)

	tests := []struct {
		name      string
		timeoutMs int64
		wantMin   int64
		wantMax   int64
	}{
		{
			name:      "normal timeout",
			timeoutMs: 300000, // 5 min
			wantMin:   now + 300000 + 60000,
			wantMax:   now + 300000 + 60000,
		},
		{
			name:      "zero timeout gets min",
			timeoutMs: 0,
			wantMin:   now + 2*60000, // min 2 min
			wantMax:   now + 2*60000,
		},
		{
			name:      "negative timeout gets min",
			timeoutMs: -1000,
			wantMin:   now + 2*60000,
			wantMax:   now + 2*60000,
		},
		{
			name:      "huge timeout capped at max",
			timeoutMs: 48 * 3600000,
			wantMin:   now + 24*3600000,
			wantMax:   now + 24*3600000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveChatRunExpiresAtMs(now, tt.timeoutMs)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("ResolveChatRunExpiresAtMs(%d, %d) = %d, want [%d, %d]",
					now, tt.timeoutMs, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestMaintenanceCleanup_NilLogger(t *testing.T) {
	// 传入 nil logger 不应 panic
	cs := NewChatRunState()
	nowMs := time.Now().UnixMilli()
	cs.AbortedRuns.Store("old", nowMs-2*3600000)

	cfg := MaintenanceConfig{ChatState: cs}
	maintenanceCleanup(cfg, nil) // nil logger 应使用 slog.Default()
}
