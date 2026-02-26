// =============================================================================
// 文件: backend/internal/sandbox/sandbox_test.go | 模块: sandbox | 职责: 沙箱配置、执行结果及网络集成的单元测试
// 审计: V12 2026-02-21 | 状态: ✅ 已审计
// =============================================================================

package sandbox

import (
	"encoding/json"
	"testing"
)

// ====== C-2.9: E2E 沙箱测试 ======

// --- Docker Runner 测试 ---

func TestDefaultDockerConfig(t *testing.T) {
	cfg := DefaultDockerConfig()
	if cfg.DockerImage != "alpine:3.19" {
		t.Errorf("expected alpine:3.19, got %s", cfg.DockerImage)
	}
	if cfg.MemoryLimitMB != 256 {
		t.Errorf("expected 256MB, got %d", cfg.MemoryLimitMB)
	}
	if cfg.NetworkEnabled {
		t.Error("network should be disabled by default")
	}
	if !cfg.ReadOnlyRoot {
		t.Error("root should be read-only by default")
	}
}

func TestDockerRunnerBuildArgs(t *testing.T) {
	runner := NewDockerRunner(DefaultDockerConfig())
	args := runner.buildDockerArgs("alpine:3.19", []string{"echo", "hello"})

	// Verify security constraints are present
	contains := func(s string) bool {
		for _, a := range args {
			if a == s {
				return true
			}
		}
		return false
	}

	checks := []string{
		"run",
		"--rm",
		"no-new-privileges:true",
		"--network=none",
		"--read-only",
		"--cap-drop=ALL",
	}

	for _, c := range checks {
		if !contains(c) {
			t.Errorf("missing security flag: %s", c)
		}
	}

	// Verify image and command are at the end
	if args[len(args)-2] != "echo" || args[len(args)-1] != "hello" {
		t.Error("command should be at end of args")
	}
}

func TestDockerExecutionResultJSON(t *testing.T) {
	result := &DockerExecutionResult{
		Stdout:     "hello world",
		ExitCode:   0,
		DurationMs: 150,
	}
	data := result.ToJSON()

	var parsed DockerExecutionResult
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Stdout != "hello world" {
		t.Errorf("expected 'hello world', got %s", parsed.Stdout)
	}
}

// --- Worker 测试 ---

func TestDefaultWorkerConfig(t *testing.T) {
	cfg := DefaultWorkerConfig()
	if cfg.WorkerCount != 2 {
		t.Errorf("expected 2 workers, got %d", cfg.WorkerCount)
	}
	if cfg.QueueSize != 100 {
		t.Errorf("expected queue size 100, got %d", cfg.QueueSize)
	}
}

func TestParseJSON(t *testing.T) {
	type testStruct struct {
		Name string `json:"name"`
	}

	var s testStruct
	err := parseJSON(`{"name": "test"}`, &s)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "test" {
		t.Errorf("expected 'test', got %s", s.Name)
	}

	// Empty string
	err = parseJSON("", &s)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

// --- WebSocket Hub 测试 ---

func TestProgressHubCreation(t *testing.T) {
	hub := NewProgressHub(nil)
	if hub.ClientCount() != 0 {
		t.Error("new hub should have 0 clients")
	}
}

func TestProgressHubBroadcastNoClients(t *testing.T) {
	hub := NewProgressHub(nil)
	// Should not panic when no clients
	hub.Broadcast(ProgressEvent{
		TaskID:   "test-task",
		Progress: 50,
		Message:  "processing",
	})
}

func TestProgressEventJSON(t *testing.T) {
	event := ProgressEvent{
		TaskID:   "task-123",
		Progress: 75,
		Message:  "nearly done",
		Type:     "progress",
		Output:   "",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	var parsed ProgressEvent
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.TaskID != "task-123" {
		t.Errorf("expected task-123, got %s", parsed.TaskID)
	}
	if parsed.Progress != 75 {
		t.Errorf("expected 75, got %d", parsed.Progress)
	}
	if parsed.Type != "progress" {
		t.Errorf("expected type 'progress', got %s", parsed.Type)
	}

	// Test stdout event with output
	stdoutEvent := ProgressEvent{
		TaskID: "task-456",
		Type:   "stdout",
		Output: "hello world\n",
	}
	data2, _ := json.Marshal(stdoutEvent)
	var parsed2 ProgressEvent
	if err := json.Unmarshal(data2, &parsed2); err != nil {
		t.Fatal(err)
	}
	if parsed2.Type != "stdout" {
		t.Errorf("expected stdout, got %s", parsed2.Type)
	}
	if parsed2.Output != "hello world\n" {
		t.Errorf("expected output 'hello world\\n', got %s", parsed2.Output)
	}
}

// --- Worker Status 测试 ---

func TestWorkerStatus(t *testing.T) {
	status := WorkerStatus{
		WorkerCount:  4,
		QueueSize:    100,
		QueuePending: 3,
	}
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}

	var parsed WorkerStatus
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.WorkerCount != 4 {
		t.Errorf("expected 4 workers, got %d", parsed.WorkerCount)
	}
	if parsed.QueueSize != 100 {
		t.Errorf("expected queue size 100, got %d", parsed.QueueSize)
	}
	if parsed.QueuePending != 3 {
		t.Errorf("expected 3 pending, got %d", parsed.QueuePending)
	}
}

// --- 集成: Wasm + Docker 选择策略测试 ---

func TestHybridExecutorCreation(t *testing.T) {
	executor := NewHybridExecutor()
	if executor.dockerRunner == nil {
		t.Error("docker runner should be initialized")
	}
}
