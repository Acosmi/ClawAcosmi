// bash/process.go — 进程生命周期管理。
// TS 参考：src/agents/bash-tools.process.ts (665L)
//
// 管理 PTY 进程的创建、输入/输出流、Docker 包装、信号处理。
package bash

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ---------- 配置 ----------

// ProcessConfig 进程启动配置。
type ProcessConfig struct {
	Command    string
	Shell      string
	Cwd        string
	Env        map[string]string
	UsePTY     bool
	Timeout    time.Duration
	Sandbox    *BashSandboxConfig
	SessionKey string
	ScopeKey   string
	MaxOutput  int
}

// ProcessHandle 进程句柄。
type ProcessHandle struct {
	Cmd    *exec.Cmd
	PID    int
	Stdin  io.WriteCloser
	cancel context.CancelFunc
	ctx    context.Context
	exited chan struct{}
	mu     sync.Mutex
	killed bool
}

// ---------- 进程创建 ----------

// SpawnProcess 创建新进程。
// TS 参考: bash-tools.process.ts L61-180
func SpawnProcess(cfg ProcessConfig) (*ProcessHandle, *ProcessSession, error) {
	shell := cfg.Shell
	if shell == "" {
		shell = DefaultShell
	}

	ctx, cancel := context.WithCancel(context.Background())
	if cfg.Timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), cfg.Timeout)
	}

	var cmd *exec.Cmd
	if cfg.Sandbox != nil {
		// Docker 模式
		dockerArgs := BuildDockerExecArgs(
			cfg.Sandbox.ContainerName,
			cfg.Command,
			cfg.Sandbox.ContainerWorkdir,
			BuildSandboxEnv(DefaultPath, cfg.Env, cfg.Sandbox.Env, cfg.Sandbox.ContainerWorkdir),
			cfg.UsePTY,
		)
		cmd = exec.CommandContext(ctx, "docker", dockerArgs...)
	} else {
		// 本地模式
		cmd = exec.CommandContext(ctx, shell, "-c", cfg.Command)
		if cfg.Cwd != "" {
			cmd.Dir = cfg.Cwd
		}
		cmd.Env = buildEnvSlice(cfg.Env)
	}

	// 设置进程组，以便杀整个树
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, fmt.Errorf("start process: %w", err)
	}

	maxOutput := cfg.MaxOutput
	if maxOutput <= 0 {
		maxOutput = 200_000
	}

	session := &ProcessSession{
		ID:             createSessionSlug(DefaultRegistry),
		Command:        cfg.Command,
		ScopeKey:       cfg.ScopeKey,
		SessionKey:     cfg.SessionKey,
		PID:            cmd.Process.Pid,
		StartedAt:      time.Now().UnixMilli(),
		Cwd:            cfg.Cwd,
		MaxOutputChars: maxOutput,
		PendingStdout:  make([]string, 0),
		PendingStderr:  make([]string, 0),
	}

	handle := &ProcessHandle{
		Cmd:    cmd,
		PID:    cmd.Process.Pid,
		Stdin:  stdin,
		cancel: cancel,
		ctx:    ctx,
		exited: make(chan struct{}),
	}

	// 注册到全局注册表
	DefaultRegistry.AddSession(session)

	// 启动输出读取 goroutine
	go captureStream(stdout, session, "stdout")
	go captureStream(stderr, session, "stderr")

	// 启动等待 goroutine
	go func() {
		defer close(handle.exited)
		err := cmd.Wait()
		code := 0
		signal := ""
		status := StatusCompleted

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
				if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					if ws.Signaled() {
						signal = ws.Signal().String()
						status = StatusKilled
					}
				}
				if code != 0 {
					status = StatusFailed
				}
			} else {
				status = StatusFailed
			}
		}

		exitCode := code
		DefaultRegistry.MarkExited(session, &exitCode, signal, status)
	}()

	return handle, session, nil
}

// ---------- 进程操作 ----------

// SendInput 向进程发送输入。
// TS 参考: bash-tools.process.ts L240-260
func (h *ProcessHandle) SendInput(data string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.Stdin == nil {
		return fmt.Errorf("stdin not available")
	}
	_, err := h.Stdin.Write([]byte(data))
	return err
}

// SendSignal 向进程发送信号。
// TS 参考: bash-tools.process.ts L262-290
func (h *ProcessHandle) SendSignal(sig os.Signal) error {
	if h.Cmd.Process == nil {
		return fmt.Errorf("process not started")
	}
	return h.Cmd.Process.Signal(sig)
}

// Kill 终止进程。
// TS 参考: bash-tools.process.ts L292-330
func (h *ProcessHandle) Kill() {
	h.mu.Lock()
	if h.killed {
		h.mu.Unlock()
		return
	}
	h.killed = true
	h.mu.Unlock()

	// 先发 SIGTERM
	if h.Cmd.Process != nil {
		_ = syscall.Kill(-h.PID, syscall.SIGTERM)
	}

	// 等待 2 秒后发 SIGKILL
	go func() {
		select {
		case <-h.exited:
			return
		case <-time.After(2 * time.Second):
			if h.Cmd.Process != nil {
				_ = syscall.Kill(-h.PID, syscall.SIGKILL)
			}
			h.cancel()
		}
	}()
}

// WaitWithTimeout 在超时内等待进程结束。
// TS 参考: bash-tools.process.ts L332-370
func (h *ProcessHandle) WaitWithTimeout(timeout time.Duration) bool {
	if timeout <= 0 {
		<-h.exited
		return true
	}
	select {
	case <-h.exited:
		return true
	case <-time.After(timeout):
		return false
	}
}

// IsRunning 检查进程是否还在运行。
func (h *ProcessHandle) IsRunning() bool {
	select {
	case <-h.exited:
		return false
	default:
		return true
	}
}

// ExitedChan 返回进程退出通道。
func (h *ProcessHandle) ExitedChan() <-chan struct{} {
	return h.exited
}

// ---------- 内部函数 ----------

func captureStream(reader io.Reader, session *ProcessSession, stream string) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text() + "\n"
		DefaultRegistry.AppendOutput(session, stream, line)
	}
	if err := scanner.Err(); err != nil {
		if err != io.EOF && !strings.Contains(err.Error(), "file already closed") {
			slog.Debug("stream capture error", "stream", stream, "err", err)
		}
	}
}

func buildEnvSlice(env map[string]string) []string {
	if env == nil {
		return os.Environ()
	}
	// 合并到当前环境
	current := os.Environ()
	merged := make(map[string]string)
	for _, kv := range current {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			merged[parts[0]] = parts[1]
		}
	}
	for k, v := range env {
		merged[k] = v
	}
	result := make([]string, 0, len(merged))
	for k, v := range merged {
		result = append(result, k+"="+v)
	}
	return result
}

// createSessionSlug 生成不重复的会话 slug。
func createSessionSlug(reg *ProcessRegistry) string {
	for i := 0; i < 100; i++ {
		slug := generateSlug()
		if !reg.IsSessionIDTaken(slug) {
			return slug
		}
	}
	return fmt.Sprintf("sess-%d", time.Now().UnixNano())
}

// generateSlug 生成短 slug。
func generateSlug() string {
	// 使用时间戳 + 随机后缀
	now := time.Now()
	return fmt.Sprintf("%s-%04d", now.Format("150405"), now.Nanosecond()/100000)
}
