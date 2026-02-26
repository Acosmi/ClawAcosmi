package signal

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// Signal daemon 进程管理 — 继承自 src/signal/daemon.ts (103L)

// SignalDaemonOpts 守护进程启动选项
type SignalDaemonOpts struct {
	CliPath           string
	Account           string
	HttpHost          string
	HttpPort          int
	ReceiveMode       string // "on-start"|"manual"
	IgnoreAttachments bool
	IgnoreStories     bool
	SendReadReceipts  bool
	ExtraArgs         []string
}

// SignalLogLevel daemon 日志级别分类
type SignalLogLevel string

const (
	LogLevelInfo  SignalLogLevel = "info"
	LogLevelWarn  SignalLogLevel = "warn"
	LogLevelError SignalLogLevel = "error"
)

// SignalDaemonHandle 守护进程句柄
type SignalDaemonHandle struct {
	Cmd     *exec.Cmd
	cancel  context.CancelFunc
	stopped bool
	mu      sync.Mutex
}

// Stop 终止守护进程
func (h *SignalDaemonHandle) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		return
	}
	h.stopped = true
	h.cancel()
	if h.Cmd.Process != nil {
		_ = h.Cmd.Process.Kill()
	}
}

// IsRunning 检查守护进程是否运行中
func (h *SignalDaemonHandle) IsRunning() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return !h.stopped
}

// ClassifySignalCliLogLine 分类 signal-cli 日志行。
// 对齐 TS classifySignalCliLogLine: 空行返回 nil，ERROR/WARN/WARNING → error，
// FAILED/SEVERE/EXCEPTION → error，其余 → log。
func ClassifySignalCliLogLine(line string) *SignalLogLevel {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil
	}
	upper := strings.ToUpper(trimmed)
	// 对齐 TS: /\b(ERROR|WARN|WARNING)\b/
	for _, kw := range []string{"ERROR", "WARN", "WARNING"} {
		if strings.Contains(upper, kw) {
			lv := LogLevelError
			return &lv
		}
	}
	// 对齐 TS: /\b(FAILED|SEVERE|EXCEPTION)\b/i
	for _, kw := range []string{"FAILED", "SEVERE", "EXCEPTION"} {
		if strings.Contains(upper, kw) {
			lv := LogLevelError
			return &lv
		}
	}
	lv := LogLevelInfo
	return &lv
}

// LogHandler daemon 日志处理器
type LogHandler func(level SignalLogLevel, line string)

// SpawnSignalDaemon 启动 signal-cli 守护进程
func SpawnSignalDaemon(ctx context.Context, opts SignalDaemonOpts, logHandler LogHandler) (*SignalDaemonHandle, error) {
	cliPath := opts.CliPath
	if cliPath == "" {
		cliPath = "signal-cli"
	}

	// 对齐 TS: -a account 必须在 daemon 子命令之前
	var args []string
	if opts.Account != "" {
		args = append(args, "-a", opts.Account)
	}
	args = append(args, "daemon")
	if opts.HttpHost != "" {
		args = append(args, "--http", fmt.Sprintf("%s:%d", opts.HttpHost, opts.HttpPort))
	} else if opts.HttpPort > 0 {
		args = append(args, "--http", fmt.Sprintf("127.0.0.1:%d", opts.HttpPort))
	}
	// 对齐 TS: 禁止 daemon 在 stdout 上输出 JSON-RPC 结果
	args = append(args, "--no-receive-stdout")
	// 对齐 TS: 传递 receiveMode/ignoreAttachments/ignoreStories/sendReadReceipts
	if opts.ReceiveMode != "" {
		args = append(args, "--receive-mode", opts.ReceiveMode)
	}
	if opts.IgnoreAttachments {
		args = append(args, "--ignore-attachments")
	}
	if opts.IgnoreStories {
		args = append(args, "--ignore-stories")
	}
	if opts.SendReadReceipts {
		args = append(args, "--send-read-receipts")
	}
	args = append(args, opts.ExtraArgs...)

	daemonCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(daemonCtx, cliPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start signal-cli daemon: %w", err)
	}

	handle := &SignalDaemonHandle{
		Cmd:    cmd,
		cancel: cancel,
	}

	// 对齐 TS: stdout 和 stderr 都通过 classifySignalCliLogLine 分类
	classifyAndLog := func(scanner *bufio.Scanner) {
		for scanner.Scan() {
			line := scanner.Text()
			level := ClassifySignalCliLogLine(line)
			if level == nil || logHandler == nil {
				continue
			}
			logHandler(*level, line)
		}
	}
	go classifyAndLog(bufio.NewScanner(stdout))
	go classifyAndLog(bufio.NewScanner(stderr))

	// 异步等待退出
	go func() {
		_ = cmd.Wait()
		handle.mu.Lock()
		handle.stopped = true
		handle.mu.Unlock()
	}()

	return handle, nil
}
