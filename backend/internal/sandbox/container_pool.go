// =============================================================================
// 文件: backend/internal/sandbox/container_pool.go | 模块: sandbox
// 职责: Code Interpreter Docker 容器池管理 — 预热/获取/归还/回收
// =============================================================================

package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// 容器池配置
// =============================================================================

// ContainerPoolConfig 容器池配置
type ContainerPoolConfig struct {
	// Image Docker 镜像名称
	Image string
	// MaxContainers 最大容器数量
	MaxContainers int
	// WarmPoolSize 预热池大小 (保持 N 个 idle 容器)
	WarmPoolSize int
	// IdleTimeout 空闲容器超时后销毁 (默认 20 分钟)
	IdleTimeout time.Duration
	// MemoryLimitMB 单容器内存限制 (MB)
	MemoryLimitMB int
	// CPUQuota CPU 配额 (1.0 = 1 core)
	CPUQuota float64
	// NetworkEnabled 是否开放网络 (默认 false)
	NetworkEnabled bool
	// PortStart 端口分配起始
	PortStart int
	// PortEnd 端口分配终止
	PortEnd int
}

// DefaultContainerPoolConfig 返回安全的默认配置
func DefaultContainerPoolConfig() ContainerPoolConfig {
	return ContainerPoolConfig{
		Image:          "acosmi-code-interpreter:latest",
		MaxContainers:  5,
		WarmPoolSize:   2,
		IdleTimeout:    20 * time.Minute,
		MemoryLimitMB:  512,
		CPUQuota:       1.0,
		NetworkEnabled: false,
		PortStart:      9200,
		PortEnd:        9299,
	}
}

// =============================================================================
// 池化容器信息
// =============================================================================

// PooledContainer 池化容器信息
type PooledContainer struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Endpoint  string    `json:"endpoint"` // http://localhost:PORT
	Port      int       `json:"port"`
	Status    string    `json:"status"` // idle, busy, stopped
	CreatedAt time.Time `json:"createdAt"`
	LastUsed  time.Time `json:"lastUsed"`
}

// =============================================================================
// 容器池管理器
// =============================================================================

// ContainerPool Code Interpreter 容器池
type ContainerPool struct {
	mu        sync.RWMutex
	config    ContainerPoolConfig
	idle      []*PooledContainer          // 空闲容器
	busy      map[string]*PooledContainer // 使用中的容器 (key: containerID)
	usedPorts map[int]bool                // 已占用端口
	stopCh    chan struct{}
}

// NewContainerPool 创建容器池
func NewContainerPool(config ContainerPoolConfig) *ContainerPool {
	if config.MaxContainers <= 0 {
		config.MaxContainers = 5
	}
	if config.WarmPoolSize <= 0 {
		config.WarmPoolSize = 2
	}
	if config.WarmPoolSize > config.MaxContainers {
		config.WarmPoolSize = config.MaxContainers
	}
	if config.PortStart <= 0 {
		config.PortStart = 9200
	}
	if config.PortEnd <= config.PortStart {
		config.PortEnd = config.PortStart + 99
	}

	return &ContainerPool{
		config:    config,
		idle:      make([]*PooledContainer, 0),
		busy:      make(map[string]*PooledContainer),
		usedPorts: make(map[int]bool),
		stopCh:    make(chan struct{}),
	}
}

// Start 启动容器池 (含后台回收 goroutine)
func (p *ContainerPool) Start(ctx context.Context) {
	// 后台空闲回收
	go p.cleanupLoop(ctx)

	slog.Info("code interpreter container pool started",
		"image", p.config.Image,
		"max_containers", p.config.MaxContainers,
		"warm_pool_size", p.config.WarmPoolSize,
	)
}

// Stop 停止容器池, 销毁所有容器
func (p *ContainerPool) Stop() {
	close(p.stopCh)

	p.mu.Lock()
	defer p.mu.Unlock()

	// 销毁所有 idle 容器
	for _, c := range p.idle {
		p.destroyContainerLocked(c.ID)
	}
	p.idle = nil

	// 销毁所有 busy 容器
	for id, c := range p.busy {
		p.destroyContainerLocked(c.ID)
		delete(p.busy, id)
	}

	p.usedPorts = make(map[int]bool)

	slog.Info("code interpreter container pool stopped")
}

// =============================================================================
// 核心操作: 获取 / 归还
// =============================================================================

// Acquire 从池中获取一个可用容器
// 优先复用 idle 容器, 不足时创建新容器
func (p *ContainerPool) Acquire(ctx context.Context) (*PooledContainer, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 1. 尝试从 idle 池获取
	if len(p.idle) > 0 {
		container := p.idle[0]
		p.idle = p.idle[1:]
		container.Status = "busy"
		container.LastUsed = time.Now()
		p.busy[container.ID] = container

		slog.Debug("container pool: reusing idle container",
			"container_id", container.ID[:12],
			"endpoint", container.Endpoint,
		)
		return container, nil
	}

	// 2. 检查是否达到上限
	totalContainers := len(p.busy)
	if totalContainers >= p.config.MaxContainers {
		return nil, fmt.Errorf("container pool exhausted: %d/%d in use", totalContainers, p.config.MaxContainers)
	}

	// 3. 创建新容器
	container, err := p.createContainerLocked(ctx)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}

	container.Status = "busy"
	p.busy[container.ID] = container

	return container, nil
}

// Release 归还容器到空闲池
func (p *ContainerPool) Release(containerID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	container, ok := p.busy[containerID]
	if !ok {
		return
	}

	delete(p.busy, containerID)
	container.Status = "idle"
	container.LastUsed = time.Now()
	p.idle = append(p.idle, container)

	slog.Debug("container pool: released container",
		"container_id", containerID[:12],
	)
}

// Destroy 销毁指定容器 (异常时使用, 不归还到池)
func (p *ContainerPool) Destroy(containerID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 从 busy 移除
	delete(p.busy, containerID)

	// 从 idle 移除
	for i, c := range p.idle {
		if c.ID == containerID {
			p.idle = append(p.idle[:i], p.idle[i+1:]...)
			break
		}
	}

	p.destroyContainerLocked(containerID)
}

// =============================================================================
// 容器生命周期 (内部方法)
// =============================================================================

// createContainerLocked 创建新容器 (已持锁)
func (p *ContainerPool) createContainerLocked(ctx context.Context) (*PooledContainer, error) {
	// 分配端口
	port, err := p.allocatePortLocked()
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("acosmi-ci-%d", port)

	// 构建 docker run 命令
	args := p.buildRunArgs(name, port)

	slog.Info("container pool: creating container",
		"name", name,
		"image", p.config.Image,
		"port", port,
	)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		p.usedPorts[port] = false
		return nil, fmt.Errorf("docker run failed: %w, output: %s", err, string(output))
	}

	containerID := strings.TrimSpace(string(output))
	if len(containerID) > 64 {
		containerID = containerID[:64]
	}

	container := &PooledContainer{
		ID:        containerID,
		Name:      name,
		Endpoint:  fmt.Sprintf("http://localhost:%d", port),
		Port:      port,
		Status:    "idle",
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}

	// 等待容器健康
	if err := p.waitForHealthy(ctx, container, 30*time.Second); err != nil {
		p.destroyContainerLocked(containerID)
		return nil, fmt.Errorf("container unhealthy: %w", err)
	}

	return container, nil
}

// destroyContainerLocked 销毁容器 (已持锁)
func (p *ContainerPool) destroyContainerLocked(containerID string) {
	// 释放端口 — 遍历所有容器找到对应端口
	findPort := func(containers []*PooledContainer) int {
		for _, c := range containers {
			if c.ID == containerID {
				return c.Port
			}
		}
		return 0
	}

	if port := findPort(p.idle); port > 0 {
		delete(p.usedPorts, port)
	}
	for _, c := range p.busy {
		if c.ID == containerID {
			delete(p.usedPorts, c.Port)
			break
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerID)
	if err := cmd.Run(); err != nil {
		slog.Debug("container pool: destroy container error (may already be gone)",
			"container_id", containerID[:12],
			"error", err,
		)
	} else {
		slog.Info("container pool: destroyed container",
			"container_id", containerID[:12],
		)
	}
}

// allocatePortLocked 分配可用端口 (已持锁)
func (p *ContainerPool) allocatePortLocked() (int, error) {
	for port := p.config.PortStart; port <= p.config.PortEnd; port++ {
		if !p.usedPorts[port] {
			p.usedPorts[port] = true
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports in range %d-%d", p.config.PortStart, p.config.PortEnd)
}

// buildRunArgs 构建 docker run 参数
func (p *ContainerPool) buildRunArgs(name string, hostPort int) []string {
	args := []string{
		"run", "-d",
		"--name", name,
		"--rm",
		"--security-opt", "no-new-privileges:true",
		"--pids-limit", "256",
		"--cap-drop=ALL",
		fmt.Sprintf("--memory=%dm", p.config.MemoryLimitMB),
		fmt.Sprintf("--cpus=%.1f", p.config.CPUQuota),
		"-p", fmt.Sprintf("127.0.0.1:%d:8080", hostPort),
	}

	if !p.config.NetworkEnabled {
		// 注意: Code Interpreter 需要端口映射, 使用 bridge 网络
		// 但在内部隔离网络中运行
		// 用 --network 自定义隔离网络或依赖 iptables 限制出站
	}

	args = append(args, p.config.Image)
	return args
}

// waitForHealthy 等待容器健康就绪
func (p *ContainerPool) waitForHealthy(_ context.Context, container *PooledContainer, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// TCP 探活
		cmd := exec.Command("docker", "inspect", "--format={{.State.Running}}", container.ID)
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) == "true" {
			// 额外等待一下 FastAPI 启动
			time.Sleep(1 * time.Second)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("container %s did not become healthy within %v", container.ID[:12], timeout)
}

// =============================================================================
// 后台维护
// =============================================================================

// cleanupLoop 定期清理空闲超时容器
func (p *ContainerPool) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.cleanupIdleContainers()
		}
	}
}

// cleanupIdleContainers 清理超时的空闲容器
func (p *ContainerPool) cleanupIdleContainers() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	keepIdle := make([]*PooledContainer, 0, len(p.idle))

	for _, c := range p.idle {
		if now.Sub(c.LastUsed) > p.config.IdleTimeout {
			slog.Info("container pool: cleaning up idle container",
				"container_id", c.ID[:12],
				"idle_duration", now.Sub(c.LastUsed).String(),
			)
			p.destroyContainerLocked(c.ID)
		} else {
			keepIdle = append(keepIdle, c)
		}
	}

	p.idle = keepIdle
}

// =============================================================================
// 状态查询
// =============================================================================

// PoolStatus 容器池状态
type PoolStatus struct {
	TotalContainers int `json:"totalContainers"`
	IdleContainers  int `json:"idleContainers"`
	BusyContainers  int `json:"busyContainers"`
	MaxContainers   int `json:"maxContainers"`
}

// Status 返回池状态
func (p *ContainerPool) Status() PoolStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return PoolStatus{
		TotalContainers: len(p.idle) + len(p.busy),
		IdleContainers:  len(p.idle),
		BusyContainers:  len(p.busy),
		MaxContainers:   p.config.MaxContainers,
	}
}
