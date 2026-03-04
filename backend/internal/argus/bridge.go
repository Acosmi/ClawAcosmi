package argus

// bridge.go — Argus 视觉子智能体进程生命周期管理
//
// 状态机: init → starting → ready → degraded → stopped
//
// 功能:
//   - Start(): spawn argus-sensory -mcp 子进程，MCP 握手 + 工具发现
//   - 健康循环: 每 30s MCP ping，3次失败标记 degraded
//   - 进程监控: 子进程退出后指数退避重启（1s→2s→4s...→60s cap，最多 5 次）
//   - CallTool(): 转发工具调用到 MCP Client
//   - Stop(): 关闭 stdin → 等 3s → force kill
//   - IsAvailable(): 检查二进制是否存在

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/mcpclient"
)

// ---------- 结构化错误 ----------

// ArgusStartError 结构化启动失败错误。
type ArgusStartError struct {
	Phase    string `json:"phase"`    // "resolve" | "permission" | "codesign" | "handshake" | "crash"
	Reason   string `json:"reason"`   // 机器可读原因
	Recovery string `json:"recovery"` // 面向用户的恢复指引
	Err      error  `json:"-"`        // 底层错误（不序列化）
}

func (e *ArgusStartError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("argus [%s]: %s: %v", e.Phase, e.Reason, e.Err)
	}
	return fmt.Sprintf("argus [%s]: %s", e.Phase, e.Reason)
}

func (e *ArgusStartError) Unwrap() error { return e.Err }

// BinaryCheckResult 二进制检查结果。
type BinaryCheckResult struct {
	Status   string `json:"status"`   // "available" | "not_found" | "not_executable"
	Path     string `json:"path"`     // 检查的路径
	Recovery string `json:"recovery"` // 恢复指引（仅失败时非空）
}

// CheckBinary 检查指定路径的二进制状态，返回结构化结果。
func CheckBinary(binaryPath string) BinaryCheckResult {
	if binaryPath == "" {
		return BinaryCheckResult{Status: "not_found", Path: "", Recovery: "No Argus binary path configured. Install argus-sensory or set $ARGUS_BINARY_PATH."}
	}
	info, err := os.Stat(binaryPath)
	if err != nil {
		return BinaryCheckResult{Status: "not_found", Path: binaryPath, Recovery: fmt.Sprintf("Binary not found at %s. Install argus-sensory or check $ARGUS_BINARY_PATH.", binaryPath)}
	}
	if info.IsDir() || info.Mode().Perm()&0111 == 0 {
		return BinaryCheckResult{Status: "not_executable", Path: binaryPath, Recovery: fmt.Sprintf("Binary at %s is not executable. Run: chmod +x %s", binaryPath, binaryPath)}
	}
	return BinaryCheckResult{Status: "available", Path: binaryPath}
}

// ---------- 常量 ----------

const (
	defaultHealthInterval = 30 * time.Second
	maxPingFailures       = 3
	maxRestartAttempts    = 5
	initialBackoff        = 1 * time.Second
	maxBackoff            = 60 * time.Second
	gracefulShutdownWait  = 3 * time.Second
	rapidCrashWindow      = 60 * time.Second // 快速崩溃检测窗口
	rapidCrashMaxCount    = 3                // 窗口内最大崩溃次数
)

// ---------- 状态 ----------

// BridgeState Argus Bridge 状态。
type BridgeState string

const (
	BridgeStateInit     BridgeState = "init"
	BridgeStateStarting BridgeState = "starting"
	BridgeStateReady    BridgeState = "ready"
	BridgeStateDegraded BridgeState = "degraded"
	BridgeStateStopped  BridgeState = "stopped"
)

// ---------- 配置 ----------

// StateChangeCallback 状态变更回调（用于通知前端）。
type StateChangeCallback func(state BridgeState, reason string)

// BridgeConfig Argus Bridge 配置。
type BridgeConfig struct {
	BinaryPath     string              // argus-sensory 二进制路径
	Args           []string            // 额外命令行参数
	HealthInterval time.Duration       // 健康检查间隔
	OnStateChange  StateChangeCallback // 状态变更通知回调（可选）
}

// DefaultBridgeConfig 返回默认配置。
func DefaultBridgeConfig() BridgeConfig {
	return BridgeConfig{
		BinaryPath:     "argus-sensory",
		Args:           []string{"-mcp"},
		HealthInterval: defaultHealthInterval,
	}
}

// ---------- Bridge ----------

// Bridge Argus 视觉子智能体 MCP 桥接。
type Bridge struct {
	cfg BridgeConfig

	mu         sync.RWMutex
	state      BridgeState
	client     *mcpclient.Client
	cmd        *exec.Cmd
	tools      []mcpclient.MCPToolDef
	pid        int
	lastPing   time.Time
	lastRTT    time.Duration
	crashTimes []time.Time // 快速崩溃熔断: 记录崩溃时间戳

	cancel context.CancelFunc
	done   chan struct{}
}

// NewBridge 创建 Argus Bridge。
func NewBridge(cfg BridgeConfig) *Bridge {
	return &Bridge{
		cfg:   cfg,
		state: BridgeStateInit,
		done:  make(chan struct{}),
	}
}

// State 返回当前状态。
func (b *Bridge) State() BridgeState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// notifyStateChange 触发状态变更回调（调用者需确保不持锁）。
func (b *Bridge) notifyStateChange(state BridgeState, reason string) {
	if cb := b.cfg.OnStateChange; cb != nil {
		cb(state, reason)
	}
}

// PID 返回子进程 PID（0 表示未运行）。
func (b *Bridge) PID() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.pid
}

// Tools 返回已发现的工具列表。
func (b *Bridge) Tools() []mcpclient.MCPToolDef {
	b.mu.RLock()
	defer b.mu.RUnlock()
	cp := make([]mcpclient.MCPToolDef, len(b.tools))
	copy(cp, b.tools)
	return cp
}

// LastPing 返回最近一次健康检查时间和 RTT。
func (b *Bridge) LastPing() (time.Time, time.Duration) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastPing, b.lastRTT
}

// Start 启动 Argus 子进程并完成 MCP 握手。
func (b *Bridge) Start() error {
	b.mu.Lock()
	if b.state != BridgeStateInit && b.state != BridgeStateStopped {
		b.mu.Unlock()
		return fmt.Errorf("argus: cannot start in state %s", b.state)
	}
	b.state = BridgeStateStarting
	b.crashTimes = nil // 重置熔断计数，允许新一轮重试
	b.mu.Unlock()

	if err := b.spawnAndHandshake(); err != nil {
		b.mu.Lock()
		b.state = BridgeStateStopped
		b.mu.Unlock()
		return err
	}

	// 启动后台健康循环和进程监控
	ctx, cancel := context.WithCancel(context.Background())
	b.mu.Lock()
	b.cancel = cancel
	b.done = make(chan struct{})
	b.mu.Unlock()

	go b.healthLoop(ctx)
	go b.processMonitor(ctx)

	return nil
}

// spawnAndHandshake 启动子进程并完成 MCP 初始化握手。
func (b *Bridge) spawnAndHandshake() error {
	cmd := exec.Command(b.cfg.BinaryPath, b.cfg.Args...)
	cmd.Stderr = &slogWriter{level: slog.LevelDebug, prefix: "argus"}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("argus: create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("argus: create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("argus: start process: %w", err)
	}

	slog.Info("argus: process started", "pid", cmd.Process.Pid, "binary", b.cfg.BinaryPath)

	// 创建 MCP 客户端
	client := mcpclient.NewClient(stdinPipe, stdoutPipe)

	// MCP 握手（5s 超时）
	initCtx, initCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer initCancel()

	initResult, err := client.Initialize(initCtx)
	if err != nil {
		client.Close()
		cmd.Process.Kill()
		cmd.Wait()
		return fmt.Errorf("argus: MCP initialize: %w", err)
	}
	slog.Info("argus: MCP initialized",
		"serverName", initResult.ServerInfo.Name,
		"serverVersion", initResult.ServerInfo.Version,
		"protocol", initResult.ProtocolVersion,
	)

	// 工具发现
	listCtx, listCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer listCancel()

	tools, err := client.ListTools(listCtx)
	if err != nil {
		client.Close()
		cmd.Process.Kill()
		cmd.Wait()
		return fmt.Errorf("argus: tools/list: %w", err)
	}

	b.mu.Lock()
	b.cmd = cmd
	b.client = client
	b.tools = tools
	b.pid = cmd.Process.Pid
	b.state = BridgeStateReady
	b.lastPing = time.Now()
	b.mu.Unlock()

	slog.Info("argus: ready", "tools", len(tools), "pid", cmd.Process.Pid)
	return nil
}

// healthLoop 定期 MCP ping 检查健康状态。
func (b *Bridge) healthLoop(ctx context.Context) {
	ticker := time.NewTicker(b.cfg.HealthInterval)
	defer ticker.Stop()
	failures := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.mu.RLock()
			client := b.client
			state := b.state
			b.mu.RUnlock()

			if client == nil || state == BridgeStateStopped {
				return
			}

			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			rtt, err := client.Ping(pingCtx)
			cancel()

			if err != nil {
				failures++
				slog.Warn("argus: ping failed", "failures", failures, "error", err)
				if failures >= maxPingFailures {
					b.mu.Lock()
					if b.state == BridgeStateReady {
						b.state = BridgeStateDegraded
						slog.Warn("argus: degraded — ping failures exceeded threshold")
					}
					b.mu.Unlock()
				}
			} else {
				failures = 0
				b.mu.Lock()
				b.lastPing = time.Now()
				b.lastRTT = rtt
				if b.state == BridgeStateDegraded {
					b.state = BridgeStateReady
					slog.Info("argus: recovered from degraded state", "rtt", rtt)
				}
				b.mu.Unlock()
			}
		}
	}
}

// processMonitor 监控子进程退出，触发指数退避重启。
func (b *Bridge) processMonitor(ctx context.Context) {
	defer close(b.done)
	backoff := initialBackoff

	for attempt := 0; attempt < maxRestartAttempts; attempt++ {
		b.mu.RLock()
		cmd := b.cmd
		b.mu.RUnlock()

		if cmd == nil || cmd.Process == nil {
			return
		}

		// 等待进程退出
		err := cmd.Wait()

		// 检查是否主动关闭
		select {
		case <-ctx.Done():
			slog.Info("argus: process monitor stopped (context cancelled)")
			return
		default:
		}

		slog.Warn("argus: process exited unexpectedly", "error", err, "attempt", attempt+1)

		// 快速崩溃熔断: 记录崩溃时间戳，检查窗口内崩溃频率
		now := time.Now()
		b.mu.Lock()
		b.crashTimes = append(b.crashTimes, now)
		// 只保留窗口内的时间戳
		cutoff := now.Add(-rapidCrashWindow)
		validIdx := 0
		for _, t := range b.crashTimes {
			if t.After(cutoff) {
				b.crashTimes[validIdx] = t
				validIdx++
			}
		}
		b.crashTimes = b.crashTimes[:validIdx]
		recentCrashes := len(b.crashTimes)

		if recentCrashes >= rapidCrashMaxCount {
			slog.Error("argus: rapid crash circuit breaker triggered",
				"crashes", recentCrashes,
				"window", rapidCrashWindow,
				"action", "stopping restarts — likely a persistent bug (check VLM provider config)",
			)
			b.state = BridgeStateStopped
			b.client = nil
			b.cmd = nil
			b.pid = 0
			b.mu.Unlock()
			b.notifyStateChange(BridgeStateStopped, fmt.Sprintf("rapid crash circuit breaker: %d crashes in %s", recentCrashes, rapidCrashWindow))
			return
		}

		b.state = BridgeStateDegraded
		b.client = nil
		b.cmd = nil
		b.pid = 0
		b.mu.Unlock()

		// 等待退避间隔
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		// 尝试重启
		slog.Info("argus: restarting", "attempt", attempt+1, "backoff", backoff)
		if err := b.spawnAndHandshake(); err != nil {
			slog.Error("argus: restart failed", "error", err, "attempt", attempt+1)
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		// 重启成功，重置退避（但不重置 crashTimes — 熔断器记忆跨重启）
		backoff = initialBackoff
		attempt = -1 // 重置计数（循环 ++ 后变为 0）
	}

	slog.Error("argus: max restart attempts exceeded, giving up")
	b.mu.Lock()
	b.state = BridgeStateStopped
	b.mu.Unlock()
	b.notifyStateChange(BridgeStateStopped, "max restart attempts exceeded")
}

// CallTool 转发工具调用到 MCP Client。
func (b *Bridge) CallTool(ctx context.Context, name string, arguments json.RawMessage, timeout time.Duration) (*mcpclient.MCPToolsCallResult, error) {
	b.mu.RLock()
	client := b.client
	state := b.state
	b.mu.RUnlock()

	if client == nil || (state != BridgeStateReady && state != BridgeStateDegraded) {
		return nil, fmt.Errorf("argus: bridge not available (state: %s)", state)
	}

	return client.CallTool(ctx, name, arguments, timeout)
}

// Stop 优雅关闭 Argus 子进程。
func (b *Bridge) Stop() {
	b.mu.Lock()
	if b.state == BridgeStateStopped {
		b.mu.Unlock()
		return
	}
	cancel := b.cancel
	client := b.client
	cmd := b.cmd
	b.state = BridgeStateStopped
	b.mu.Unlock()

	// 取消后台循环
	if cancel != nil {
		cancel()
	}

	// 关闭 stdin（通知子进程退出）
	if client != nil {
		client.Close()
	}

	if cmd != nil && cmd.Process != nil {
		// 等待优雅退出
		done := make(chan struct{})
		go func() {
			cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
			slog.Info("argus: process stopped gracefully")
		case <-time.After(gracefulShutdownWait):
			slog.Warn("argus: force killing process")
			cmd.Process.Kill()
			cmd.Wait()
		}
	}

	b.mu.Lock()
	b.client = nil
	b.cmd = nil
	b.pid = 0
	b.mu.Unlock()
}

// ResolveBinary 在 PATH 中搜索指定名称的可执行文件。
// 成功返回绝对路径，失败返回错误。
func ResolveBinary(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("argus: empty binary name")
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("argus: binary %q not found in PATH: %w", name, err)
	}
	return path, nil
}

// IsAvailable 检查 Argus 二进制是否存在于指定路径且具有可执行权限。
func IsAvailable(binaryPath string) bool {
	if binaryPath == "" {
		return false
	}
	info, err := os.Stat(binaryPath)
	if err != nil {
		return false
	}
	// 排除目录 + 检查至少一个执行位
	return !info.IsDir() && info.Mode().Perm()&0111 != 0
}

// ---------- 辅助 ----------

// slogWriter 将 io.Writer 输出转为 slog 日志行。
type slogWriter struct {
	level  slog.Level
	prefix string
}

func (w *slogWriter) Write(p []byte) (int, error) {
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	if msg == "" {
		return len(p), nil
	}
	slog.Log(context.Background(), w.level, w.prefix+": "+msg)
	return len(p), nil
}
