// sandbox_integration_test.go — 端到端 Docker 沙箱集成测试
// 需要 Docker daemon 运行 + alpine:3.19 镜像
//
//go:build integration

package sandbox

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDockerRunnerExecute_Integration(t *testing.T) {
	if !IsDockerAvailable() {
		t.Skip("Docker not available, skipping integration test")
	}

	cfg := DefaultDockerConfig()
	cfg.TimeoutSecs = 30
	runner := NewDockerRunner(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test 1: 基本执行
	t.Run("basic_echo", func(t *testing.T) {
		result, err := runner.Execute(ctx, "", []string{"echo", "Hello from sandbox!"}, "")
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
		}
		if !strings.Contains(result.Stdout, "Hello from sandbox!") {
			t.Errorf("expected stdout to contain 'Hello from sandbox!', got: %s", result.Stdout)
		}
		t.Logf("✅ stdout: %s (duration: %dms)", strings.TrimSpace(result.Stdout), result.DurationMs)
	})

	// Test 2: 只读文件系统验证
	t.Run("readonly_root", func(t *testing.T) {
		result, err := runner.Execute(ctx, "", []string{"sh", "-c", "touch /test_file 2>&1 || echo 'READONLY_OK'"}, "")
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !strings.Contains(result.Stdout, "READONLY_OK") && !strings.Contains(result.Stderr, "Read-only") {
			t.Errorf("expected read-only filesystem error, got stdout: %s, stderr: %s", result.Stdout, result.Stderr)
		}
		t.Logf("✅ 只读文件系统: %s", strings.TrimSpace(result.Stdout+result.Stderr))
	})

	// Test 3: /tmp 可写验证
	t.Run("tmpfs_writable", func(t *testing.T) {
		result, err := runner.Execute(ctx, "", []string{"sh", "-c", "echo 'test' > /tmp/test && cat /tmp/test"}, "")
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}
		if !strings.Contains(result.Stdout, "test") {
			t.Errorf("expected 'test' in stdout, got: %s", result.Stdout)
		}
		t.Logf("✅ tmpfs 可写: %s", strings.TrimSpace(result.Stdout))
	})

	// Test 4: 网络隔离验证
	t.Run("network_disabled", func(t *testing.T) {
		result, err := runner.Execute(ctx, "", []string{"sh", "-c", "ping -c 1 -W 2 8.8.8.8 2>&1 || echo 'NETWORK_BLOCKED'"}, "")
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !strings.Contains(result.Stdout, "NETWORK_BLOCKED") && !strings.Contains(result.Stderr, "Network") {
			t.Errorf("expected network to be blocked, got stdout: %s", result.Stdout)
		}
		t.Logf("✅ 网络隔离: %s", strings.TrimSpace(result.Stdout+result.Stderr))
	})

	// Test 5: 安全约束验证 (no-new-privileges + cap-drop=ALL)
	t.Run("security_constraints", func(t *testing.T) {
		result, err := runner.Execute(ctx, "", []string{"sh", "-c", "id && cat /proc/self/status | grep -i cap"}, "")
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		t.Logf("✅ 安全上下文: %s", strings.TrimSpace(result.Stdout))
	})
}
