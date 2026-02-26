// =============================================================================
// 文件: backend/internal/sandbox/native_bridge.go
// 模块: sandbox | 职责: 原生沙箱 Worker 进程桥接 (JSON-Lines IPC)
// =============================================================================

// NativeSandboxBridge 管理一个持久 Rust Worker 子进程。
// Worker 在 Seatbelt/Landlock 沙箱中运行，通过 stdin/stdout JSON-Lines IPC
// 接收并执行命令。后续调用延迟 <1ms（相比 Docker ~215ms）。
//
// 生命周期仿 argus/bridge.go:
//   - Start(): 启动 Worker 子进程，验证 ping
//   - Execute(): 发送 JSON-Lines 请求，等待响应
//   - healthMonitor(): goroutine，30s 间隔 ping
//   - processMonitor(): goroutine，crash 时 exponential backoff 重启
//   - Stop(): close stdin → 3s grace → kill → wait

package sandbox

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// ---------- 常量 ----------

const (
	nativeHealthInterval     = 30 * time.Second
	nativeMaxPingFailures    = 3
	nativeMaxRestartAttempts = 5
	nativeInitialBackoff     = 1 * time.Second
	nativeMaxBackoff         = 60 * time.Second
	nativeGracefulWait       = 3 * time.Second
)

// ---------- 状态 ----------

// NativeBridgeState 原生沙箱 Bridge 状态。
type NativeBridgeState string

const (
	NativeBridgeInit     NativeBridgeState = "init"
	NativeBridgeStarting NativeBridgeState = "starting"
	NativeBridgeReady    NativeBridgeState = "ready"
	NativeBridgeDegraded NativeBridgeState = "degraded"
	NativeBridgeStopped  NativeBridgeState = "stopped"
)

// ---------- IPC 协议（对齐 Rust worker/protocol.rs） ----------

// workerRequest JSON-Lines 请求。
type workerRequest struct {
	ID         uint64            `json:"id"`
	Command    string            `json:"command"`
	Args       []string          `json:"args,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Cwd        string            `json:"cwd,omitempty"`
	TimeoutSec *uint64           `json:"timeout_secs,omitempty"`
}

// workerResponse JSON-Lines 响应。
type workerResponse struct {
	ID         uint64  `json:"id"`
	Stdout     string  `json:"stdout"`
	Stderr     string  `json:"stderr"`
	ExitCode   int     `json:"exit_code"`
	DurationMs uint64  `json:"duration_ms"`
	Error      *string `json:"error,omitempty"`
}

// ---------- 配置 ----------

// NativeSandboxConfig 原生沙箱 Bridge 配置。
type NativeSandboxConfig struct {
	// BinaryPath CLI 二进制路径 (e.g. "openacosmi")。
	BinaryPath string
	// Workspace 工作目录（挂载 R/W）。
	Workspace string
	// SecurityLevel "deny" / "sandbox" / "full"。
	SecurityLevel string
	// IdleTimeout Worker 空闲超时（0 = 不超时）。
	IdleTimeout time.Duration
	// HealthInterval 健康检查间隔。
	HealthInterval time.Duration
	// MaxRetries 最大崩溃重启次数。
	MaxRetries int
}

// DefaultNativeSandboxConfig 返回默认配置。
func DefaultNativeSandboxConfig() NativeSandboxConfig {
	return NativeSandboxConfig{
		BinaryPath:     "openacosmi",
		SecurityLevel:  "sandbox",
		HealthInterval: nativeHealthInterval,
		MaxRetries:     nativeMaxRestartAttempts,
	}
}

// ---------- Bridge ----------

// NativeSandboxBridge 原生沙箱 Worker 进程桥接。
type NativeSandboxBridge struct {
	cfg NativeSandboxConfig

	mu      sync.Mutex
	state   NativeBridgeState
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Scanner
	pid     int
	nextID  atomic.Uint64
	stopped bool // 标记是否已请求停止

	cancel context.CancelFunc
	done   chan struct{}
}

// NewNativeSandboxBridge 创建原生沙箱 Bridge（未启动）。
func NewNativeSandboxBridge(cfg NativeSandboxConfig) *NativeSandboxBridge {
	return &NativeSandboxBridge{
		cfg:   cfg,
		state: NativeBridgeInit,
		done:  make(chan struct{}),
	}
}

// State 返回当前状态。
func (b *NativeSandboxBridge) State() NativeBridgeState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// PID 返回 Worker 子进程 PID（0 = 未运行）。
func (b *NativeSandboxBridge) PID() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.pid
}

// Start 启动 Worker 子进程并验证初始 ping。
func (b *NativeSandboxBridge) Start() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state == NativeBridgeReady || b.state == NativeBridgeStarting {
		return nil // 已启动
	}

	b.state = NativeBridgeStarting

	if err := b.spawnLocked(); err != nil {
		b.state = NativeBridgeStopped
		return fmt.Errorf("native sandbox bridge start: %w", err)
	}

	b.state = NativeBridgeReady

	// 启动监控 goroutine
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	go b.healthMonitor(ctx)
	go b.processMonitor(ctx)

	slog.Info("native sandbox bridge started",
		"pid", b.pid,
		"workspace", b.cfg.Workspace,
		"security", b.cfg.SecurityLevel,
	)
	return nil
}

// spawnLocked 启动 Worker 子进程（需持有 mu）。
func (b *NativeSandboxBridge) spawnLocked() error {
	args := []string{
		"sandbox", "worker-start",
		"--workspace", b.cfg.Workspace,
		"--timeout", "120",
		"--security-level", b.cfg.SecurityLevel,
	}
	if b.cfg.IdleTimeout > 0 {
		args = append(args, "--idle-timeout", fmt.Sprintf("%d", int(b.cfg.IdleTimeout.Seconds())))
	} else {
		args = append(args, "--idle-timeout", "0")
	}

	cmd := exec.Command(b.cfg.BinaryPath, args...)
	// Worker 的 tracing 日志通过 stderr 流向父进程，不混入 stdout IPC 通道。
	// Rust 端 init_tracing() 使用 .with_writer(std::io::stderr) 确保 log 不污染 IPC。
	// Go 端 stderr 不走 slog 结构化日志 — 这是设计决策（见审计 F-11）。
	cmd.Stderr = os.Stderr

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn worker: %w", err)
	}

	b.cmd = cmd
	b.stdin = stdinPipe
	b.stdout = bufio.NewScanner(stdoutPipe)
	// 设置最大行长度 (10MB) 防止 OOM
	b.stdout.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	b.pid = cmd.Process.Pid

	slog.Debug("native sandbox worker spawned", "pid", b.pid)

	// 验证初始 ping
	if err := b.pingLocked(); err != nil {
		// Ping 失败 — 终止进程
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("initial ping failed: %w", err)
	}

	return nil
}

// pingLocked 发送 __ping__ 并验证响应（需持有 mu）。
func (b *NativeSandboxBridge) pingLocked() error {
	id := b.nextID.Add(1)
	req := workerRequest{
		ID:      id,
		Command: "__ping__",
	}
	if err := b.writeLocked(&req); err != nil {
		return fmt.Errorf("ping write: %w", err)
	}
	resp, err := b.readLocked()
	if err != nil {
		return fmt.Errorf("ping read: %w", err)
	}
	if resp.ID != id || resp.ExitCode != 0 {
		return fmt.Errorf("ping response mismatch: id=%d exitCode=%d", resp.ID, resp.ExitCode)
	}
	return nil
}

// writeLocked 写入 JSON-Lines 请求（需持有 mu）。
func (b *NativeSandboxBridge) writeLocked(req *workerRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')
	if _, err := b.stdin.Write(data); err != nil {
		return fmt.Errorf("write stdin: %w", err)
	}
	return nil
}

// readLocked 读取 JSON-Lines 响应（需持有 mu）。
func (b *NativeSandboxBridge) readLocked() (*workerResponse, error) {
	if !b.stdout.Scan() {
		if err := b.stdout.Err(); err != nil {
			return nil, fmt.Errorf("read stdout: %w", err)
		}
		return nil, fmt.Errorf("worker stdout closed (EOF)")
	}
	line := b.stdout.Bytes()
	var resp workerResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (line: %s)", err, string(line))
	}
	return &resp, nil
}

// Execute 在 Worker 沙箱中执行命令并返回结果。
// 这是提供给 Go 后端的主要 API。
//
// 设计: Execute 全程持有 mu 锁。JSON-Lines IPC 协议是串行的（单 stdin/stdout），
// 不支持请求复用。锁确保写请求→读响应的原子性，防止 healthMonitor ping
// 抢占响应。长命令执行期间 Ping 和 Stop 会等待，这是预期行为。
// ctx 超时由 Worker 端 per-command TimeoutSec 保证（不使用 goroutine+select，
// 避免锁释放后遗留 goroutine 破坏 IPC 状态 — 见审计 F-01）。
func (b *NativeSandboxBridge) Execute(ctx context.Context, cmd string, args []string, env map[string]string, timeoutMs int64) (stdout, stderr string, exitCode int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.state != NativeBridgeReady {
		return "", "", -1, fmt.Errorf("native sandbox not ready (state=%s)", b.state)
	}

	id := b.nextID.Add(1)
	req := workerRequest{
		ID:      id,
		Command: cmd,
		Args:    args,
		Env:     env,
	}
	if timeoutMs > 0 {
		secs := uint64(timeoutMs / 1000)
		if secs == 0 {
			secs = 1
		}
		req.TimeoutSec = &secs
	}

	if err := b.writeLocked(&req); err != nil {
		b.state = NativeBridgeDegraded
		return "", "", -1, fmt.Errorf("execute write: %w", err)
	}

	// 同步读取响应。ctx 超时由 Worker 端 per-command TimeoutSec 保证。
	// 不使用 goroutine+select 模式：锁释放后遗留 goroutine 会破坏 IPC 状态
	// （读到下一次请求的响应或被 healthMonitor ping 抢占）。
	resp, readErr := b.readLocked()
	if readErr != nil {
		b.state = NativeBridgeDegraded
		return "", "", -1, fmt.Errorf("execute read: %w", readErr)
	}
	if resp.Error != nil {
		return resp.Stdout, resp.Stderr, resp.ExitCode, fmt.Errorf("worker error: %s", *resp.Error)
	}
	return resp.Stdout, resp.Stderr, resp.ExitCode, nil
}

// Ping 发送健康检查。
func (b *NativeSandboxBridge) Ping() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == NativeBridgeStopped {
		return fmt.Errorf("bridge stopped")
	}
	return b.pingLocked()
}

// Stop 优雅关闭 Worker 进程。
func (b *NativeSandboxBridge) Stop() {
	b.mu.Lock()
	if b.stopped {
		b.mu.Unlock()
		return
	}
	b.stopped = true

	// 取消监控 goroutine
	if b.cancel != nil {
		b.cancel()
	}

	// 发送 __shutdown__ 命令（best-effort）
	if b.stdin != nil && b.state == NativeBridgeReady {
		id := b.nextID.Add(1)
		shutdownReq := workerRequest{
			ID:      id,
			Command: "__shutdown__",
		}
		_ = b.writeLocked(&shutdownReq)
	}

	// 关闭 stdin → Worker 收到 EOF 退出
	if b.stdin != nil {
		_ = b.stdin.Close()
	}

	cmd := b.cmd
	b.state = NativeBridgeStopped
	b.mu.Unlock()

	// 等待进程退出
	if cmd != nil && cmd.Process != nil {
		done := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
			slog.Info("native sandbox worker exited gracefully")
		case <-time.After(nativeGracefulWait):
			slog.Warn("native sandbox worker did not exit in time, killing")
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}

	// 等待监控 goroutine 退出
	select {
	case <-b.done:
	case <-time.After(5 * time.Second):
		slog.Warn("native sandbox monitor goroutines did not exit in time")
	}

	slog.Info("native sandbox bridge stopped")
}

// healthMonitor 定期 ping Worker，连续失败标记 degraded。
func (b *NativeSandboxBridge) healthMonitor(ctx context.Context) {
	if b.cfg.HealthInterval <= 0 {
		// No health check configured — wait for ctx cancellation.
		<-ctx.Done()
		return
	}
	ticker := time.NewTicker(b.cfg.HealthInterval)
	defer ticker.Stop()

	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := b.Ping(); err != nil {
				failures++
				slog.Warn("native sandbox health ping failed",
					"failures", failures,
					"error", err,
				)
				if failures >= nativeMaxPingFailures {
					b.mu.Lock()
					if b.state == NativeBridgeReady {
						b.state = NativeBridgeDegraded
						slog.Warn("native sandbox bridge degraded", "failures", failures)
					}
					b.mu.Unlock()
				}
			} else {
				if failures > 0 {
					slog.Info("native sandbox health recovered", "prevFailures", failures)
				}
				failures = 0
				b.mu.Lock()
				if b.state == NativeBridgeDegraded {
					b.state = NativeBridgeReady
				}
				b.mu.Unlock()
			}
		}
	}
}

// processMonitor 等待 Worker 退出，crash 时 exponential backoff 重启。
func (b *NativeSandboxBridge) processMonitor(ctx context.Context) {
	defer close(b.done)

	backoff := nativeInitialBackoff
	restarts := 0

	for {
		// 等待当前进程退出
		b.mu.Lock()
		cmd := b.cmd
		b.mu.Unlock()

		if cmd == nil || cmd.Process == nil {
			return
		}

		// Wait for process in a goroutine
		waitCh := make(chan error, 1)
		go func() {
			waitCh <- cmd.Wait()
		}()

		select {
		case <-ctx.Done():
			// ctx cancelled (Stop called). Kill the process so the Wait goroutine
			// can exit promptly instead of leaking until the process happens to die.
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			// Drain waitCh to allow the goroutine to complete.
			<-waitCh
			return
		case err := <-waitCh:
			b.mu.Lock()
			if b.stopped {
				b.mu.Unlock()
				return
			}

			slog.Warn("native sandbox worker crashed",
				"pid", b.pid,
				"error", err,
				"restarts", restarts,
			)
			b.state = NativeBridgeDegraded
			b.mu.Unlock()

			// 检查重启次数
			restarts++
			maxRetries := b.cfg.MaxRetries
			if maxRetries <= 0 {
				maxRetries = nativeMaxRestartAttempts
			}
			if restarts > maxRetries {
				slog.Error("native sandbox bridge max restarts exceeded, giving up",
					"restarts", restarts,
				)
				b.mu.Lock()
				b.state = NativeBridgeStopped
				b.mu.Unlock()
				return
			}

			// Exponential backoff
			slog.Info("native sandbox bridge restarting",
				"backoff", backoff,
				"attempt", restarts,
			)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}

			backoff *= 2
			if backoff > nativeMaxBackoff {
				backoff = nativeMaxBackoff
			}

			// 尝试重启
			b.mu.Lock()
			if b.stopped {
				b.mu.Unlock()
				return
			}
			if err := b.spawnLocked(); err != nil {
				slog.Error("native sandbox bridge restart failed",
					"error", err,
					"attempt", restarts,
				)
				b.state = NativeBridgeDegraded
				b.mu.Unlock()
				continue
			}
			b.state = NativeBridgeReady
			slog.Info("native sandbox bridge restarted",
				"pid", b.pid,
				"attempt", restarts,
			)
			b.mu.Unlock()

			// 重置 backoff on successful restart
			backoff = nativeInitialBackoff
		}
	}
}

// IsNativeSandboxAvailable 检查原生沙箱 CLI 二进制是否可用。
func IsNativeSandboxAvailable(binaryPath string) bool {
	if binaryPath == "" {
		binaryPath = "openacosmi"
	}
	// 检查二进制是否存在且可执行
	path, err := exec.LookPath(binaryPath)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
