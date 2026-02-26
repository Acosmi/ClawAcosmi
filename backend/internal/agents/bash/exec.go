// bash/exec.go — Bash 命令执行主入口。
// TS 参考：src/agents/bash-tools.exec.ts (1630L)
//
// 包含 ExecuteBashCommand、命令预处理、超时控制、输出截断、
// 审批流程集成和工具定义。
package bash

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ---------- 配置常量 ----------

const (
	DefaultTimeout                  = 120 * time.Second
	MaxTimeout                      = 600 * time.Second
	MinTimeout                      = 5 * time.Second
	DefaultMaxOutputLen             = 200_000
	DefaultOutputLineMax            = 5000
	TruncationThreshold             = 0.8
	DefaultApprovalTimeoutMs        = 120_000
	DefaultApprovalRequestTimeoutMs = 130_000
	DefaultNotifyTailChars          = 400
)

// ResolveBashPendingMaxOutputChars 返回 bash 工具的最大 pending 输出字符数。
// 优先读取 OPENACOSMI_BASH_PENDING_MAX_OUTPUT_CHARS 环境变量，否则返回默认值。
// TS 参考: src/agents/bash-tools.exec.ts — BASH_PENDING_MAX_OUTPUT_CHARS
func ResolveBashPendingMaxOutputChars() int {
	if v := os.Getenv("OPENACOSMI_BASH_PENDING_MAX_OUTPUT_CHARS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return DefaultMaxOutputLen
}

// ---------- 执行选项 ----------

// ExecOptions Bash 命令执行选项。
// TS 参考: bash-tools.exec.ts L30-80
type ExecOptions struct {
	Command        string             `json:"command"`
	Cwd            string             `json:"cwd,omitempty"`
	Env            map[string]string  `json:"env,omitempty"`
	Timeout        *int               `json:"timeout,omitempty"`   // 秒
	MaxOutput      *int               `json:"maxOutput,omitempty"` // 最大输出字符数
	Background     bool               `json:"background,omitempty"`
	SessionID      string             `json:"sessionId,omitempty"`
	Description    string             `json:"description,omitempty"`
	ScopeKey       string             `json:"-"`
	SessionKey     string             `json:"-"`
	ApprovalPolicy ApprovalPolicy     `json:"-"`
	Sandbox        *BashSandboxConfig `json:"-"`
}

// ApprovalPolicy 审批策略接口。
// TS 参考: bash-tools.exec.ts L82-100
type ApprovalPolicy interface {
	// NeedsApproval 检查命令是否需要审批。
	NeedsApproval(command string) bool
	// RequestApproval 请求审批（同步等待）。
	RequestApproval(ctx context.Context, command, description string) (approved bool, err error)
}

// ExecResult 命令执行结果。
// TS 参考: bash-tools.exec.ts L102-130
type ExecResult struct {
	Stdout      string        `json:"stdout,omitempty"`
	Stderr      string        `json:"stderr,omitempty"`
	ExitCode    int           `json:"exitCode"`
	Signal      string        `json:"signal,omitempty"`
	Truncated   bool          `json:"truncated,omitempty"`
	Timed       bool          `json:"timedOut,omitempty"`
	SessionID   string        `json:"sessionId,omitempty"`
	Status      ProcessStatus `json:"status,omitempty"`
	Duration    int64         `json:"durationMs,omitempty"`
	Warnings    []string      `json:"warnings,omitempty"`
	Background  bool          `json:"background,omitempty"`
	SessionName string        `json:"sessionName,omitempty"`
}

// ---------- 主执行函数 ----------

// ExecuteBashCommand 执行 Bash 命令。
// TS 参考: bash-tools.exec.ts executeBashCommand L132-450
func ExecuteBashCommand(ctx context.Context, opts ExecOptions) (*ExecResult, error) {
	var warnings []string
	result := &ExecResult{}

	// 1. 命令预处理
	command := strings.TrimSpace(opts.Command)
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}
	command = preprocessCommand(command)
	result.SessionName = DeriveSessionName(command)

	// 2. 工作目录解析
	cwd := opts.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	if opts.Sandbox != nil {
		hostCwd, containerCwd := ResolveSandboxWorkdir(cwd, *opts.Sandbox, &warnings)
		cwd = hostCwd
		_ = containerCwd // 供 Docker 模式使用
	} else {
		cwd = ResolveWorkdir(cwd, &warnings)
	}

	// 3. 超时解析
	timeout := DefaultTimeout
	if opts.Timeout != nil {
		t := time.Duration(*opts.Timeout) * time.Second
		if t < MinTimeout {
			t = MinTimeout
		}
		if t > MaxTimeout {
			t = MaxTimeout
		}
		timeout = t
	}

	// 4. 审批检查
	if opts.ApprovalPolicy != nil && opts.ApprovalPolicy.NeedsApproval(command) {
		approved, err := opts.ApprovalPolicy.RequestApproval(ctx, command, opts.Description)
		if err != nil {
			return nil, fmt.Errorf("approval request failed: %w", err)
		}
		if !approved {
			return &ExecResult{
				ExitCode: -1,
				Stderr:   "Command was not approved.",
				Status:   StatusFailed,
				Warnings: warnings,
			}, nil
		}
	}

	// 5. cd 拦截
	if isCdCommand(command) {
		dir := extractCdTarget(command)
		return handleCdCommand(dir, cwd, warnings)
	}

	// 6. 最大输出限制
	maxOutput := DefaultMaxOutputLen
	if opts.MaxOutput != nil && *opts.MaxOutput > 0 {
		maxOutput = *opts.MaxOutput
	}

	// 7. 创建进程
	startTime := time.Now()
	handle, session, err := SpawnProcess(ProcessConfig{
		Command:    command,
		Shell:      resolveShell(),
		Cwd:        cwd,
		Env:        opts.Env,
		Timeout:    timeout,
		Sandbox:    opts.Sandbox,
		SessionKey: opts.SessionKey,
		ScopeKey:   opts.ScopeKey,
		MaxOutput:  maxOutput,
	})
	if err != nil {
		return nil, fmt.Errorf("spawn failed: %w", err)
	}

	result.SessionID = session.ID

	// 8. 后台模式
	if opts.Background {
		DefaultRegistry.MarkBackgrounded(session)
		result.Background = true
		result.Status = StatusRunning
		result.Warnings = warnings
		return result, nil
	}

	// 9. 等待完成
	completed := handle.WaitWithTimeout(timeout)
	duration := time.Since(startTime)
	result.Duration = duration.Milliseconds()

	if !completed {
		// 超时
		result.Timed = true
		handle.Kill()
		// 等待进程确实退出
		handle.WaitWithTimeout(5 * time.Second)
	}

	// 10. 收集输出
	stdout, stderr := DefaultRegistry.DrainSession(session)
	result.Stdout = truncateOutput(stdout, maxOutput)
	result.Stderr = truncateOutput(stderr, maxOutput)
	result.Truncated = session.Truncated

	if session.ExitCode != nil {
		result.ExitCode = *session.ExitCode
	}
	result.Signal = session.ExitSignal

	// 11. 推断状态
	if result.Timed {
		result.Status = StatusKilled
	} else if result.ExitCode == 0 {
		result.Status = StatusCompleted
	} else {
		result.Status = StatusFailed
	}

	result.Warnings = warnings
	return result, nil
}

// ---------- 命令预处理 ----------

var (
	cdOnlyRe       = regexp.MustCompile(`^\s*cd\s`)
	ansiRe         = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	multiNewlineRe = regexp.MustCompile(`\n{3,}`)
)

// preprocessCommand 预处理命令字符串。
// TS 参考: bash-tools.exec.ts L452-500
func preprocessCommand(command string) string {
	// 移除 ANSI 转义序列
	command = ansiRe.ReplaceAllString(command, "")
	// 规范化多余换行
	command = multiNewlineRe.ReplaceAllString(command, "\n\n")
	return strings.TrimSpace(command)
}

func isCdCommand(command string) bool {
	trimmed := strings.TrimSpace(command)
	return trimmed == "cd" || cdOnlyRe.MatchString(trimmed)
}

func extractCdTarget(command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "cd" || trimmed == "cd " {
		home, _ := os.UserHomeDir()
		return home
	}
	parts := strings.SplitN(trimmed, " ", 2)
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[1])
	}
	home, _ := os.UserHomeDir()
	return home
}

func handleCdCommand(dir, cwd string, warnings []string) (*ExecResult, error) {
	target := dir
	if target == "~" || target == "" {
		home, _ := os.UserHomeDir()
		target = home
	}
	if target == "-" {
		return &ExecResult{
			Stderr:   "cd - is not supported in non-interactive mode",
			ExitCode: 1,
			Status:   StatusFailed,
			Warnings: warnings,
		}, nil
	}

	// 检查目标目录
	fi, err := os.Stat(target)
	if err != nil || !fi.IsDir() {
		return &ExecResult{
			Stderr:   fmt.Sprintf("cd: no such file or directory: %s", target),
			ExitCode: 1,
			Status:   StatusFailed,
			Warnings: warnings,
		}, nil
	}

	return &ExecResult{
		Stdout:   fmt.Sprintf("Changed directory to %s", target),
		ExitCode: 0,
		Status:   StatusCompleted,
		Warnings: warnings,
	}, nil
}

func resolveShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return DefaultShell
}

// truncateOutput 截断输出文本。
// TS 参考: bash-tools.exec.ts L502-540
func truncateOutput(text string, maxChars int) string {
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	// 保留首尾
	headLen := int(float64(maxChars) * 0.4)
	tailLen := maxChars - headLen - 50 // 留空间给省略标记
	if tailLen < 0 {
		tailLen = 0
	}

	head := text[:headLen]
	tail := text[len(text)-tailLen:]
	omitted := len(text) - headLen - tailLen
	return head + fmt.Sprintf("\n\n... (%d characters omitted) ...\n\n", omitted) + tail
}

// ---------- 会话操作 ----------

// SendInputToSession 向指定会话发送输入。
// TS 参考: bash-tools.exec.ts L600-640
func SendInputToSession(sessionID string, input KeyEncodingRequest) (*ExecResult, error) {
	session := DefaultRegistry.GetSession(sessionID)
	if session == nil {
		// 检查已完成的会话
		finished := DefaultRegistry.GetFinishedSession(sessionID)
		if finished != nil {
			return &ExecResult{
				ExitCode:  derefIntPtr(finished.ExitCode),
				Status:    finished.Status,
				Stdout:    finished.Aggregated,
				Truncated: finished.Truncated,
			}, nil
		}
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	encoded := EncodeKeySequence(input)
	if session.Stdin != nil {
		if err := session.Stdin.Write(encoded.Data); err != nil {
			slog.Warn("send input failed", "session", sessionID, "err", err)
		}
	}

	// 等待一小段时间让输出到达
	time.Sleep(100 * time.Millisecond)

	stdout, stderr := DefaultRegistry.DrainSession(session)
	result := &ExecResult{
		SessionID: sessionID,
		Stdout:    stdout,
		Stderr:    stderr,
		Status:    StatusRunning,
		Warnings:  encoded.Warnings,
	}

	if session.Exited {
		result.ExitCode = derefIntPtr(session.ExitCode)
		result.Signal = session.ExitSignal
		result.Status = StatusCompleted
		if result.ExitCode != 0 {
			result.Status = StatusFailed
		}
	}

	return result, nil
}

// ViewSessionOutput 查看会话输出。
// TS 参考: bash-tools.exec.ts L642-700
func ViewSessionOutput(sessionID string, offset, limit *int) (*ExecResult, error) {
	session := DefaultRegistry.GetSession(sessionID)
	if session == nil {
		finished := DefaultRegistry.GetFinishedSession(sessionID)
		if finished != nil {
			slice, totalLines, _ := SliceLogLines(finished.Aggregated, offset, limit)
			return &ExecResult{
				ExitCode:  derefIntPtr(finished.ExitCode),
				Status:    finished.Status,
				Stdout:    slice,
				Truncated: finished.Truncated,
				Warnings:  []string{fmt.Sprintf("total_lines: %d", totalLines)},
			}, nil
		}
		return nil, fmt.Errorf("session %q not found", sessionID)
	}

	slice, totalLines, _ := SliceLogLines(session.Aggregated, offset, limit)
	result := &ExecResult{
		SessionID: sessionID,
		Stdout:    slice,
		Truncated: session.Truncated,
		Status:    StatusRunning,
		Warnings:  []string{fmt.Sprintf("total_lines: %d", totalLines)},
	}

	if session.Exited {
		result.ExitCode = derefIntPtr(session.ExitCode)
		result.Status = StatusCompleted
		if result.ExitCode != 0 {
			result.Status = StatusFailed
		}
	}

	return result, nil
}

// KillProcessSession 终止指定会话。
// TS 参考: bash-tools.exec.ts L702-730
func KillProcessSession(sessionID string) (*ExecResult, error) {
	session := DefaultRegistry.GetSession(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session %q not found or already exited", sessionID)
	}
	KillSession(session.PID)
	return &ExecResult{
		SessionID: sessionID,
		Status:    StatusKilled,
	}, nil
}

// ListSessions 列出所有会话。
// TS 参考: bash-tools.exec.ts L732-780
func ListSessions() []SessionSummary {
	running := DefaultRegistry.ListRunningSessions()
	finished := DefaultRegistry.ListFinishedSessions()

	var result []SessionSummary
	for _, s := range running {
		result = append(result, SessionSummary{
			ID:        s.ID,
			Command:   TruncateMiddle(s.Command, 80),
			Status:    StatusRunning,
			StartedAt: s.StartedAt,
			PID:       s.PID,
		})
	}
	for _, s := range finished {
		result = append(result, SessionSummary{
			ID:        s.ID,
			Command:   TruncateMiddle(s.Command, 80),
			Status:    s.Status,
			StartedAt: s.StartedAt,
			EndedAt:   s.EndedAt,
			ExitCode:  derefIntPtr(s.ExitCode),
		})
	}
	return result
}

// SessionSummary 会话摘要。
type SessionSummary struct {
	ID        string        `json:"id"`
	Command   string        `json:"command"`
	Status    ProcessStatus `json:"status"`
	StartedAt int64         `json:"startedAt"`
	EndedAt   int64         `json:"endedAt,omitempty"`
	PID       int           `json:"pid,omitempty"`
	ExitCode  int           `json:"exitCode,omitempty"`
}

// ---------- 工具定义 ----------

// BashToolSchema 返回 bash 工具 JSON schema。
// TS 参考: bash-tools.exec.ts L800-900
func BashToolSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"execute", "sendInput", "view", "kill", "list"},
				"description": "The action to perform.",
			},
			"command": map[string]any{
				"type":        "string",
				"description": "Shell command to execute (for 'execute' action).",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Brief description of what this command does.",
			},
			"cwd": map[string]any{
				"type":        "string",
				"description": "Working directory for the command.",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Timeout in seconds.",
			},
			"background": map[string]any{
				"type":        "boolean",
				"description": "Run command in background.",
			},
			"sessionId": map[string]any{
				"type":        "string",
				"description": "Session ID for interactive commands.",
			},
			"keys": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Named keys to send (for 'sendInput' action).",
			},
			"literal": map[string]any{
				"type":        "string",
				"description": "Literal text to send (for 'sendInput' action).",
			},
			"hex": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Hex bytes to send (for 'sendInput' action).",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Line offset for 'view' action.",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Line limit for 'view' action.",
			},
		},
		"required": []string{"action"},
	}
}

// HandleBashToolCall 统一处理 bash 工具调用。
// TS 参考: bash-tools.exec.ts L900-1000
func HandleBashToolCall(ctx context.Context, args map[string]any, policy ApprovalPolicy, scopeKey, sessionKey string, sandbox *BashSandboxConfig) (json.RawMessage, error) {
	action, _ := args["action"].(string)

	switch action {
	case "execute":
		return handleExecuteAction(ctx, args, policy, scopeKey, sessionKey, sandbox)
	case "sendInput":
		return handleSendInputAction(args)
	case "view":
		return handleViewAction(args)
	case "kill":
		return handleKillAction(args)
	case "list":
		return handleListAction()
	default:
		return nil, fmt.Errorf("unknown bash action: %s", action)
	}
}

func handleExecuteAction(ctx context.Context, args map[string]any, policy ApprovalPolicy, scopeKey, sessionKey string, sandbox *BashSandboxConfig) (json.RawMessage, error) {
	command, _ := args["command"].(string)
	cwd, _ := args["cwd"].(string)
	desc, _ := args["description"].(string)
	bg, _ := args["background"].(bool)

	var timeout *int
	if t, ok := args["timeout"].(float64); ok {
		v := int(t)
		timeout = &v
	}
	var maxOutput *int
	if m, ok := args["maxOutput"].(float64); ok {
		v := int(m)
		maxOutput = &v
	}

	env := make(map[string]string)
	if e, ok := args["env"].(map[string]any); ok {
		for k, v := range e {
			if s, ok := v.(string); ok {
				env[k] = s
			}
		}
	}

	result, err := ExecuteBashCommand(ctx, ExecOptions{
		Command:        command,
		Cwd:            cwd,
		Env:            env,
		Timeout:        timeout,
		MaxOutput:      maxOutput,
		Background:     bg,
		Description:    desc,
		ScopeKey:       scopeKey,
		SessionKey:     sessionKey,
		ApprovalPolicy: policy,
		Sandbox:        sandbox,
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func handleSendInputAction(args map[string]any) (json.RawMessage, error) {
	sessionID, _ := args["sessionId"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("sessionId is required for sendInput")
	}

	req := KeyEncodingRequest{}
	if lit, ok := args["literal"].(string); ok {
		req.Literal = lit
	}
	if keys, ok := args["keys"].([]any); ok {
		for _, k := range keys {
			if s, ok := k.(string); ok {
				req.Keys = append(req.Keys, s)
			}
		}
	}
	if hexes, ok := args["hex"].([]any); ok {
		for _, h := range hexes {
			if s, ok := h.(string); ok {
				req.Hex = append(req.Hex, s)
			}
		}
	}

	result, err := SendInputToSession(sessionID, req)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func handleViewAction(args map[string]any) (json.RawMessage, error) {
	sessionID, _ := args["sessionId"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("sessionId is required for view")
	}

	var offset, limit *int
	if o, ok := args["offset"].(float64); ok {
		v := int(o)
		offset = &v
	}
	if l, ok := args["limit"].(float64); ok {
		v := int(l)
		limit = &v
	}

	result, err := ViewSessionOutput(sessionID, offset, limit)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func handleKillAction(args map[string]any) (json.RawMessage, error) {
	sessionID, _ := args["sessionId"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("sessionId is required for kill")
	}
	result, err := KillProcessSession(sessionID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func handleListAction() (json.RawMessage, error) {
	sessions := ListSessions()
	return json.Marshal(sessions)
}

// ---------- 工具函数 ----------

func derefIntPtr(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// readIntArg 从参数中读取整数。
func readIntArg(args map[string]any, key string) (int, bool) {
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i, true
		}
	}
	return 0, false
}
