// =============================================================================
// 文件: backend/internal/sandbox/container_pool_test.go | 模块: sandbox
// 职责: Code Interpreter 容器池管理器单元测试
// Phase C: CI-C14
// =============================================================================

package sandbox

import (
	"encoding/json"
	"testing"
	"time"
)

// --- ContainerPoolConfig 默认值测试 ---

func TestDefaultContainerPoolConfig(t *testing.T) {
	cfg := DefaultContainerPoolConfig()

	if cfg.Image != "acosmi-code-interpreter:latest" {
		t.Errorf("expected acosmi-code-interpreter:latest, got %s", cfg.Image)
	}
	if cfg.MaxContainers != 5 {
		t.Errorf("expected 5 max containers, got %d", cfg.MaxContainers)
	}
	if cfg.WarmPoolSize != 2 {
		t.Errorf("expected 2 warm pool, got %d", cfg.WarmPoolSize)
	}
	if cfg.IdleTimeout != 20*time.Minute {
		t.Errorf("expected 20m idle timeout, got %v", cfg.IdleTimeout)
	}
	if cfg.MemoryLimitMB != 512 {
		t.Errorf("expected 512MB memory, got %d", cfg.MemoryLimitMB)
	}
	if cfg.NetworkEnabled {
		t.Error("network should be disabled by default")
	}
	if cfg.PortStart != 9200 {
		t.Errorf("expected port start 9200, got %d", cfg.PortStart)
	}
}

// --- ContainerPool 创建测试 ---

func TestNewContainerPool(t *testing.T) {
	cfg := DefaultContainerPoolConfig()
	pool := NewContainerPool(cfg)

	if pool == nil {
		t.Fatal("pool should not be nil")
	}
	if pool.config.MaxContainers != 5 {
		t.Errorf("expected max 5, got %d", pool.config.MaxContainers)
	}
}

func TestNewContainerPoolInvalidConfig(t *testing.T) {
	// 测试无效配置自动修正
	cfg := ContainerPoolConfig{
		MaxContainers: -1,
		WarmPoolSize:  -1,
		PortStart:     0,
		PortEnd:       0,
	}
	pool := NewContainerPool(cfg)

	if pool.config.MaxContainers != 5 {
		t.Errorf("expected auto-corrected max 5, got %d", pool.config.MaxContainers)
	}
	if pool.config.WarmPoolSize != 2 {
		t.Errorf("expected auto-corrected warm 2, got %d", pool.config.WarmPoolSize)
	}
	if pool.config.PortStart != 9200 {
		t.Errorf("expected auto-corrected port start 9200, got %d", pool.config.PortStart)
	}
}

func TestNewContainerPoolWarmExceedsMax(t *testing.T) {
	cfg := ContainerPoolConfig{
		MaxContainers: 3,
		WarmPoolSize:  10,
		PortStart:     9200,
		PortEnd:       9299,
	}
	pool := NewContainerPool(cfg)

	if pool.config.WarmPoolSize != 3 {
		t.Errorf("warm should be capped to max, got %d", pool.config.WarmPoolSize)
	}
}

// --- PoolStatus 测试 ---

func TestPoolStatus(t *testing.T) {
	pool := NewContainerPool(DefaultContainerPoolConfig())

	status := pool.Status()
	if status.TotalContainers != 0 {
		t.Errorf("expected 0 total, got %d", status.TotalContainers)
	}
	if status.IdleContainers != 0 {
		t.Errorf("expected 0 idle, got %d", status.IdleContainers)
	}
	if status.BusyContainers != 0 {
		t.Errorf("expected 0 busy, got %d", status.BusyContainers)
	}
	if status.MaxContainers != 5 {
		t.Errorf("expected max 5, got %d", status.MaxContainers)
	}
}

func TestPoolStatusJSON(t *testing.T) {
	status := PoolStatus{
		TotalContainers: 3,
		IdleContainers:  1,
		BusyContainers:  2,
		MaxContainers:   5,
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}

	var parsed PoolStatus
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.TotalContainers != 3 {
		t.Errorf("expected 3 total, got %d", parsed.TotalContainers)
	}
}

// --- PooledContainer JSON 测试 ---

func TestPooledContainerJSON(t *testing.T) {
	now := time.Now()
	container := PooledContainer{
		ID:        "abc123def456",
		Name:      "acosmi-ci-9200",
		Endpoint:  "http://localhost:9200",
		Port:      9200,
		Status:    "idle",
		CreatedAt: now,
		LastUsed:  now,
	}

	data, err := json.Marshal(container)
	if err != nil {
		t.Fatal(err)
	}

	var parsed PooledContainer
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.ID != "abc123def456" {
		t.Errorf("expected abc123def456, got %s", parsed.ID)
	}
	if parsed.Endpoint != "http://localhost:9200" {
		t.Errorf("expected http://localhost:9200, got %s", parsed.Endpoint)
	}
	if parsed.Port != 9200 {
		t.Errorf("expected 9200, got %d", parsed.Port)
	}
}

// --- buildRunArgs 安全约束测试 ---

func TestContainerPoolBuildRunArgs(t *testing.T) {
	cfg := DefaultContainerPoolConfig()
	pool := NewContainerPool(cfg)

	args := pool.buildRunArgs("acosmi-ci-9200", 9200)

	contains := func(s string) bool {
		for _, a := range args {
			if a == s {
				return true
			}
		}
		return false
	}

	// 安全标志检查
	checks := []string{
		"run",
		"-d",
		"--rm",
		"no-new-privileges:true",
		"--cap-drop=ALL",
	}

	for _, c := range checks {
		if !contains(c) {
			t.Errorf("missing security flag: %s", c)
		}
	}

	// 验证名称和端口映射
	if !contains("acosmi-ci-9200") {
		t.Error("missing container name")
	}

	// 验证镜像在最后
	if args[len(args)-1] != cfg.Image {
		t.Errorf("image should be last arg, got %s", args[len(args)-1])
	}
}

// --- Port 分配测试 ---

func TestAllocatePort(t *testing.T) {
	cfg := ContainerPoolConfig{
		PortStart:     9200,
		PortEnd:       9202,
		MaxContainers: 5,
		WarmPoolSize:  1,
	}
	pool := NewContainerPool(cfg)

	// 分配 3 个端口
	port1, err := pool.allocatePortLocked()
	if err != nil {
		t.Fatal(err)
	}
	if port1 != 9200 {
		t.Errorf("expected 9200, got %d", port1)
	}

	port2, err := pool.allocatePortLocked()
	if err != nil {
		t.Fatal(err)
	}
	if port2 != 9201 {
		t.Errorf("expected 9201, got %d", port2)
	}

	port3, err := pool.allocatePortLocked()
	if err != nil {
		t.Fatal(err)
	}
	if port3 != 9202 {
		t.Errorf("expected 9202, got %d", port3)
	}

	// 第 4 个应该失败
	_, err = pool.allocatePortLocked()
	if err == nil {
		t.Error("expected port exhaustion error")
	}
}

// --- Release 和 Destroy 测试 ---

func TestReleaseNonExistentContainer(t *testing.T) {
	pool := NewContainerPool(DefaultContainerPoolConfig())

	// 不应 panic
	pool.Release("non-existent-id")
	pool.Destroy("non-existent-id")
}

// --- Code Interpreter Input JSON 测试 ---

func TestCodeInterpreterInputJSON(t *testing.T) {
	input := codeInterpreterInput{
		Code:    "print('hello')",
		Timeout: 30,
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	var parsed codeInterpreterInput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Code != "print('hello')" {
		t.Errorf("expected print('hello'), got %s", parsed.Code)
	}
	if parsed.Timeout != 30 {
		t.Errorf("expected 30, got %d", parsed.Timeout)
	}
}
