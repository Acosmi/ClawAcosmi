package argus

// integration_test.go — Argus Bridge 端到端集成测试
//
// 需要 ARGUS_BINARY_PATH 环境变量指向构建好的 argus-sensory 二进制。
// 跳过条件：二进制不存在时自动跳过（CI 友好）。

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func resolveTestBinaryPath() string {
	if v := os.Getenv("ARGUS_BINARY_PATH"); v != "" {
		return v
	}
	return ""
}

func TestIntegration_BridgeLifecycle(t *testing.T) {
	binPath := resolveTestBinaryPath()
	if binPath == "" || !IsAvailable(binPath) {
		t.Skip("ARGUS_BINARY_PATH not set or binary not available, skipping integration test")
	}

	cfg := DefaultBridgeConfig()
	cfg.BinaryPath = binPath
	cfg.HealthInterval = 2 * time.Second // 加快测试节奏

	bridge := NewBridge(cfg)

	// 1. Start
	if err := bridge.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer bridge.Stop()

	// 2. 验证 ready 状态
	if bridge.State() != BridgeStateReady {
		t.Fatalf("expected ready state, got %s", bridge.State())
	}

	// 3. 验证 PID > 0
	if bridge.PID() <= 0 {
		t.Fatalf("expected positive PID, got %d", bridge.PID())
	}

	// 4. 验证工具发现
	tools := bridge.Tools()
	if len(tools) == 0 {
		t.Fatal("expected at least 1 tool from tools/list")
	}
	t.Logf("discovered %d tools", len(tools))

	// 验证已知工具存在
	foundCapture := false
	for _, tool := range tools {
		if tool.Name == "capture_screen" {
			foundCapture = true
			break
		}
	}
	if !foundCapture {
		t.Error("expected to find 'capture_screen' in tool list")
	}

	// 5. CallTool — capture_screen
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := json.RawMessage(`{"quality":"vlm"}`)
	result, err := bridge.CallTool(ctx, "capture_screen", args, 10*time.Second)
	if err != nil {
		t.Fatalf("CallTool capture_screen failed: %v", err)
	}
	if result.IsError {
		errMsg := "unknown"
		if len(result.Content) > 0 {
			errMsg = result.Content[0].Text
		}
		t.Fatalf("capture_screen returned error: %s", errMsg)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content from capture_screen")
	}
	t.Logf("capture_screen returned %d content blocks, first type=%s", len(result.Content), result.Content[0].Type)

	// 6. CallTool — mouse_position（轻量、无副作用）
	posResult, err := bridge.CallTool(ctx, "mouse_position", nil, 5*time.Second)
	if err != nil {
		t.Fatalf("CallTool mouse_position failed: %v", err)
	}
	if posResult.IsError {
		t.Fatalf("mouse_position returned error")
	}
	t.Logf("mouse_position: %s", posResult.Content[0].Text)

	// 7. Skills 转换
	entries := BuildArgusSkillEntries(tools)
	if len(entries) != len(tools) {
		t.Errorf("expected %d skill entries, got %d", len(tools), len(entries))
	}

	// 8. 优雅停止
	bridge.Stop()
	if bridge.State() != BridgeStateStopped {
		t.Errorf("expected stopped state after Stop, got %s", bridge.State())
	}
	if bridge.PID() != 0 {
		t.Errorf("expected PID 0 after Stop, got %d", bridge.PID())
	}
}

func TestIntegration_SkillsCoverage(t *testing.T) {
	binPath := resolveTestBinaryPath()
	if binPath == "" || !IsAvailable(binPath) {
		t.Skip("ARGUS_BINARY_PATH not set or binary not available, skipping integration test")
	}

	cfg := DefaultBridgeConfig()
	cfg.BinaryPath = binPath

	bridge := NewBridge(cfg)
	if err := bridge.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer bridge.Stop()

	tools := bridge.Tools()
	entries := BuildArgusSkillEntries(tools)

	// 验证每个 entry 的关键字段
	for i, e := range entries {
		if e.Source != "argus" {
			t.Errorf("[%d] %s: source should be 'argus', got %q", i, e.Name, e.Source)
		}
		if e.Category == "" {
			t.Errorf("[%d] %s: category should not be empty", i, e.Name)
		}
		if e.Risk == "" {
			t.Errorf("[%d] %s: risk should not be empty", i, e.Name)
		}
		if !e.Eligible {
			t.Errorf("[%d] %s: should be eligible", i, e.Name)
		}
		t.Logf("  %s → category=%s, risk=%s, emoji=%s", e.Name, e.Category, e.Risk, e.Emoji)
	}
}
