// bash/shell_env.go — 登录 Shell 环境变量检测。
// TS 参考：src/infra/shell-env.ts (173L)
//
// 从登录 shell 获取 PATH 等环境变量，支持缓存、
// 超时控制和回退逻辑。
package bash

import (
	"bytes"
	"context"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- 常量 ----------

const (
	defaultShellEnvTimeoutMs = 15_000
	defaultMaxShellEnvBuffer = 2 * 1024 * 1024
)

// ---------- 缓存 ----------

var (
	cachedShellPath   *string // nil=未初始化, &""=无结果
	shellPathMu       sync.Mutex
	lastAppliedKeys   []string
	lastAppliedKeysMu sync.Mutex
)

// ---------- 内部函数 ----------

// resolveLoginShell 解析登录 shell。
// TS 参考: shell-env.ts resolveShell L9-12
func resolveLoginShell(env map[string]string) string {
	shell := ""
	if env != nil {
		shell = strings.TrimSpace(env["SHELL"])
	}
	if shell == "" {
		shell = strings.TrimSpace(os.Getenv("SHELL"))
	}
	if shell != "" {
		return shell
	}
	return "/bin/sh"
}

// parseShellEnvOutput 解析 env -0 的 null 分隔输出。
// TS 参考: shell-env.ts parseShellEnv L14-33
func parseShellEnvOutput(stdout []byte) map[string]string {
	result := make(map[string]string)
	parts := bytes.Split(stdout, []byte{0})
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		eq := bytes.IndexByte(part, '=')
		if eq <= 0 {
			continue
		}
		key := string(part[:eq])
		value := string(part[eq+1:])
		if key == "" {
			continue
		}
		result[key] = value
	}
	return result
}

// ---------- ShellEnvFallback ----------

// ShellEnvFallbackResult 回退加载结果。
// TS 参考: shell-env.ts ShellEnvFallbackResult L35-38
type ShellEnvFallbackResult struct {
	OK            bool
	Applied       []string
	SkippedReason string // "already-has-keys", "disabled", ""
	Error         string
}

// ShellEnvFallbackOptions 回退选项。
// TS 参考: shell-env.ts ShellEnvFallbackOptions L40-47
type ShellEnvFallbackOptions struct {
	Enabled      bool
	Env          map[string]string
	ExpectedKeys []string
	TimeoutMs    int
}

// LoadShellEnvFallback 从登录 shell 加载缺失的环境变量。
// TS 参考: shell-env.ts loadShellEnvFallback L49-104
func LoadShellEnvFallback(opts ShellEnvFallbackOptions) ShellEnvFallbackResult {
	lastAppliedKeysMu.Lock()
	defer lastAppliedKeysMu.Unlock()

	if !opts.Enabled {
		lastAppliedKeys = nil
		return ShellEnvFallbackResult{OK: true, SkippedReason: "disabled"}
	}

	// 检查是否已有目标键
	for _, key := range opts.ExpectedKeys {
		if v, ok := opts.Env[key]; ok && strings.TrimSpace(v) != "" {
			lastAppliedKeys = nil
			return ShellEnvFallbackResult{OK: true, SkippedReason: "already-has-keys"}
		}
	}

	timeoutMs := opts.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = defaultShellEnvTimeoutMs
	}

	shell := resolveLoginShell(opts.Env)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, "-l", "-c", "env -0")
	cmd.Env = buildEnvSliceFromMap(opts.Env)

	stdout, err := cmd.Output()
	if err != nil {
		lastAppliedKeys = nil
		return ShellEnvFallbackResult{OK: false, Error: err.Error()}
	}

	shellEnv := parseShellEnvOutput(stdout)

	var applied []string
	for _, key := range opts.ExpectedKeys {
		if existing, ok := opts.Env[key]; ok && strings.TrimSpace(existing) != "" {
			continue
		}
		value, ok := shellEnv[key]
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		opts.Env[key] = value
		applied = append(applied, key)
	}

	lastAppliedKeys = applied
	return ShellEnvFallbackResult{OK: true, Applied: applied}
}

// buildEnvSliceFromMap 将环境 map 转为 []string 格式。
func buildEnvSliceFromMap(env map[string]string) []string {
	if env == nil {
		return os.Environ()
	}
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}

// ---------- 配置判断 ----------

// ShouldEnableShellEnvFallback 检查是否启用 shell env 回退。
// TS 参考: shell-env.ts shouldEnableShellEnvFallback L106-108
func ShouldEnableShellEnvFallback() bool {
	return isTruthyEnvValue(os.Getenv("OPENACOSMI_LOAD_SHELL_ENV"))
}

// ShouldDeferShellEnvFallback 检查是否延迟执行回退。
// TS 参考: shell-env.ts shouldDeferShellEnvFallback L110-112
func ShouldDeferShellEnvFallback() bool {
	return isTruthyEnvValue(os.Getenv("OPENACOSMI_DEFER_SHELL_ENV_FALLBACK"))
}

// ResolveShellEnvFallbackTimeoutMs 解析回退超时。
// TS 参考: shell-env.ts resolveShellEnvFallbackTimeoutMs L114-124
func ResolveShellEnvFallbackTimeoutMs() int {
	raw := strings.TrimSpace(os.Getenv("OPENACOSMI_SHELL_ENV_TIMEOUT_MS"))
	if raw == "" {
		return defaultShellEnvTimeoutMs
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || !isFinite(float64(parsed)) {
		return defaultShellEnvTimeoutMs
	}
	if parsed < 0 {
		return 0
	}
	return parsed
}

// ---------- 登录 Shell PATH ----------

// GetShellPathFromLoginShell 从登录 shell 获取 PATH。
// TS 参考: shell-env.ts getShellPathFromLoginShell L126-164
func GetShellPathFromLoginShell(env map[string]string, timeoutMs int) string {
	shellPathMu.Lock()
	defer shellPathMu.Unlock()

	if cachedShellPath != nil {
		return *cachedShellPath
	}

	// Windows 不支持
	if runtime.GOOS == "windows" {
		empty := ""
		cachedShellPath = &empty
		return ""
	}

	if timeoutMs <= 0 {
		timeoutMs = defaultShellEnvTimeoutMs
	}

	shell := resolveLoginShell(env)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, "-l", "-c", "env -0")
	if env != nil {
		cmd.Env = buildEnvSliceFromMap(env)
	}

	stdout, err := cmd.Output()
	if err != nil {
		empty := ""
		cachedShellPath = &empty
		return ""
	}

	shellEnv := parseShellEnvOutput(stdout)
	shellPath := strings.TrimSpace(shellEnv["PATH"])
	if shellPath == "" {
		empty := ""
		cachedShellPath = &empty
		return ""
	}

	cachedShellPath = &shellPath
	return shellPath
}

// ResetShellPathCacheForTests 清除缓存（仅测试用）。
// TS 参考: shell-env.ts resetShellPathCacheForTests L166-168
func ResetShellPathCacheForTests() {
	shellPathMu.Lock()
	defer shellPathMu.Unlock()
	cachedShellPath = nil
}

// GetShellEnvAppliedKeys 返回上次应用的键列表。
// TS 参考: shell-env.ts getShellEnvAppliedKeys L170-172
func GetShellEnvAppliedKeys() []string {
	lastAppliedKeysMu.Lock()
	defer lastAppliedKeysMu.Unlock()
	if lastAppliedKeys == nil {
		return nil
	}
	result := make([]string, len(lastAppliedKeys))
	copy(result, lastAppliedKeys)
	return result
}

// ---------- 工具函数 ----------

func isTruthyEnvValue(value string) bool {
	v := strings.TrimSpace(strings.ToLower(value))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func isFinite(v float64) bool {
	return !math.IsInf(v, 0) && !math.IsNaN(v)
}
