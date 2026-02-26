// bash/spawn_utils.go — 进程启动工具（含回退重试）。
// TS 参考：src/process/spawn-utils.ts (142L)
//
// 提供带回退策略的进程启动、错误格式化、stdio 解析。
package bash

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// ---------- 类型定义 ----------

// SpawnFallback 定义一个回退策略。
// TS 参考: spawn-utils.ts SpawnFallback L4-7
type SpawnFallback struct {
	Label    string
	Detached bool     // 对应 TS options.detached
	Env      []string // 可选覆盖环境变量
}

// SpawnWithFallbackResult 启动结果。
// TS 参考: spawn-utils.ts SpawnWithFallbackResult L9-13
type SpawnWithFallbackResult struct {
	Cmd           *exec.Cmd
	UsedFallback  bool
	FallbackLabel string
}

// SpawnWithFallbackParams 启动参数。
// TS 参考: spawn-utils.ts SpawnWithFallbackParams L15-22
type SpawnWithFallbackParams struct {
	Argv       []string
	Dir        string
	Env        []string
	Detached   bool
	Fallbacks  []SpawnFallback
	RetryCodes []string // 默认 ["EBADF"]
	OnFallback func(err error, fallback SpawnFallback)
}

// 默认重试错误码。
var defaultRetryCodes = []string{"EBADF"}

// ---------- Stdio 解析 ----------

// StdioMode stdio 模式。
type StdioMode string

const (
	StdioPipe    StdioMode = "pipe"
	StdioInherit StdioMode = "inherit"
	StdioIgnore  StdioMode = "ignore"
)

// ResolveCommandStdio 决定 stdin 模式。
// TS 参考: spawn-utils.ts resolveCommandStdio L26-32
func ResolveCommandStdio(hasInput, preferInherit bool) StdioMode {
	if hasInput {
		return StdioPipe
	}
	if preferInherit {
		return StdioInherit
	}
	return StdioPipe
}

// ---------- 错误格式化 ----------

// FormatSpawnError 格式化进程启动错误。
// TS 参考: spawn-utils.ts formatSpawnError L34-54
func FormatSpawnError(err error) string {
	if err == nil {
		return ""
	}

	parts := []string{}
	msg := strings.TrimSpace(err.Error())
	if msg != "" {
		parts = append(parts, msg)
	}

	// 提取 errno 和 syscall 信息
	if exitErr, ok := err.(*exec.ExitError); ok {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if ws.Signaled() {
				parts = append(parts, fmt.Sprintf("signal=%s", ws.Signal()))
			}
		}
	}

	if pathErr, ok := err.(*exec.Error); ok {
		if pathErr.Err != nil {
			errStr := pathErr.Err.Error()
			if !strings.Contains(msg, errStr) {
				parts = append(parts, errStr)
			}
		}
	}

	return strings.Join(parts, " ")
}

// ---------- 重试判断 ----------

// shouldRetrySpawn 检查错误是否应该重试。
// TS 参考: spawn-utils.ts shouldRetry L56-60
func shouldRetrySpawn(err error, codes []string) bool {
	if err == nil || len(codes) == 0 {
		return false
	}
	errStr := err.Error()
	for _, code := range codes {
		if strings.Contains(errStr, code) {
			return true
		}
	}
	return false
}

// ---------- 启动与等待 ----------

// spawnAndWait 启动命令并等待其就绪（进程成功启动或返回错误）。
// TS 参考: spawn-utils.ts spawnAndWaitForSpawn L62-103
func spawnAndWait(ctx context.Context, argv []string, dir string, env []string, detached bool) (*exec.Cmd, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("argv is empty")
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = env
	}

	if detached {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	// 设置管道
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	_ = stdin // 调用者可以通过 cmd.Process 获取

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return cmd, nil
}

// SpawnWithFallback 启动进程，失败时按顺序尝试回退策略。
// TS 参考: spawn-utils.ts spawnWithFallback L105-141
func SpawnWithFallback(ctx context.Context, params SpawnWithFallbackParams) (*SpawnWithFallbackResult, error) {
	retryCodes := params.RetryCodes
	if len(retryCodes) == 0 {
		retryCodes = defaultRetryCodes
	}

	type attempt struct {
		label    string
		detached bool
		env      []string
	}

	attempts := []attempt{
		{label: "", detached: params.Detached, env: params.Env},
	}
	for _, fb := range params.Fallbacks {
		env := params.Env
		if len(fb.Env) > 0 {
			env = fb.Env
		}
		attempts = append(attempts, attempt{
			label:    fb.Label,
			detached: fb.Detached,
			env:      env,
		})
	}

	var lastErr error
	for i, att := range attempts {
		cmd, err := spawnAndWait(ctx, params.Argv, params.Dir, att.env, att.detached)
		if err == nil {
			return &SpawnWithFallbackResult{
				Cmd:           cmd,
				UsedFallback:  i > 0,
				FallbackLabel: att.label,
			}, nil
		}

		lastErr = err

		// 检查是否有下一个回退，且错误是否可重试
		if i < len(params.Fallbacks) {
			nextFb := params.Fallbacks[i]
			if !shouldRetrySpawn(err, retryCodes) {
				return nil, err
			}
			if params.OnFallback != nil {
				params.OnFallback(err, nextFb)
			}
		} else {
			return nil, err
		}
	}

	return nil, lastErr
}
