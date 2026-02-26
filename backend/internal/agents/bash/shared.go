// Package bash 实现 Bash 命令执行链。
// TS 参考：src/agents/bash-tools.shared.ts (252L)
//
// 包含常量、工具函数、类型定义。
package bash

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ---------- 常量 ----------

const (
	// ChunkLimit 单次输出块字节限制。
	ChunkLimit = 8 * 1024

	// DefaultShell 默认 shell 路径。
	DefaultShell = "/bin/sh"

	// DefaultPath 沙盒默认 PATH。
	DefaultPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
)

// ---------- 类型 ----------

// BashSandboxConfig Docker 沙盒配置。
type BashSandboxConfig struct {
	ContainerName    string            `json:"containerName"`
	WorkspaceDir     string            `json:"workspaceDir"`
	ContainerWorkdir string            `json:"containerWorkdir"`
	Env              map[string]string `json:"env,omitempty"`
}

// ---------- 沙盒环境 ----------

// BuildSandboxEnv 构建沙盒环境变量。
// TS 参考: bash-tools.shared.ts L19-36
func BuildSandboxEnv(defaultPath string, paramsEnv, sandboxEnv map[string]string, containerWorkdir string) map[string]string {
	env := map[string]string{
		"PATH": defaultPath,
		"HOME": containerWorkdir,
	}
	for k, v := range sandboxEnv {
		env[k] = v
	}
	for k, v := range paramsEnv {
		env[k] = v
	}
	return env
}

// CoerceEnv 将可能含 nil 值的环境映射规范化为纯字符串映射。
// TS 参考: bash-tools.shared.ts L38-49
func CoerceEnv(env map[string]string) map[string]string {
	if env == nil {
		return make(map[string]string)
	}
	record := make(map[string]string, len(env))
	for k, v := range env {
		record[k] = v
	}
	return record
}

// ---------- Docker 参数构建 ----------

// BuildDockerExecArgs 构建 docker exec 命令参数。
// TS 参考: bash-tools.shared.ts L51-82
func BuildDockerExecArgs(containerName, command, workdir string, env map[string]string, tty bool) []string {
	args := []string{"exec", "-i"}
	if tty {
		args = append(args, "-t")
	}
	if workdir != "" {
		args = append(args, "-w", workdir)
	}
	for k, v := range env {
		args = append(args, "-e", k+"="+v)
	}

	customPath, hasCustomPath := env["PATH"]
	if hasCustomPath && customPath != "" {
		args = append(args, "-e", "OPENACOSMI_PREPEND_PATH="+customPath)
	}

	// Login shell (-l) sources /etc/profile which resets PATH. Prepend custom
	// PATH after profile sourcing to ensure custom tools are accessible.
	pathExport := ""
	if hasCustomPath && customPath != "" {
		pathExport = `export PATH="${OPENACOSMI_PREPEND_PATH}:$PATH"; unset OPENACOSMI_PREPEND_PATH; `
	}
	args = append(args, containerName, "sh", "-lc", pathExport+command)
	return args
}

// ---------- 工作目录解析 ----------

// ResolveWorkdir 解析工作目录（非沙盒模式）。
// TS 参考: bash-tools.shared.ts L125-138
func ResolveWorkdir(workdir string, warnings *[]string) string {
	fallback := safeCwd()
	if fallback == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fallback = "/"
		} else {
			fallback = home
		}
	}
	fi, err := os.Stat(workdir)
	if err == nil && fi.IsDir() {
		return workdir
	}
	*warnings = append(*warnings, `Warning: workdir "`+workdir+`" is unavailable; using "`+fallback+`".`)
	return fallback
}

func safeCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	_, err = os.Stat(cwd)
	if err != nil {
		return ""
	}
	return cwd
}

// ---------- 数字工具 ----------

// ClampNumber 将数值限制在 [min, max] 范围内。
// TS 参考: bash-tools.shared.ts L149-159
func ClampNumber(value *int, defaultValue, min, max int) int {
	if value == nil {
		return defaultValue
	}
	v := *value
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ReadEnvInt 从环境变量读取整数。
// TS 参考: bash-tools.shared.ts L161-168
func ReadEnvInt(key string) (int, bool) {
	raw := os.Getenv(key)
	if raw == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

// ---------- 字符串处理 ----------

// ChunkString 将字符串按 limit 字节分块。
// TS 参考: bash-tools.shared.ts L170-176
func ChunkString(input string, limit int) []string {
	if limit <= 0 {
		limit = ChunkLimit
	}
	var chunks []string
	for i := 0; i < len(input); i += limit {
		end := i + limit
		if end > len(input) {
			end = len(input)
		}
		chunks = append(chunks, input[i:end])
	}
	if len(chunks) == 0 {
		chunks = append(chunks, "")
	}
	return chunks
}

// TruncateMiddle 截断字符串中间部分，保留首尾。
// TS 参考: bash-tools.shared.ts L178-184
func TruncateMiddle(str string, maxLen int) string {
	if utf8.RuneCountInString(str) <= maxLen {
		return str
	}
	runes := []rune(str)
	half := (maxLen - 3) / 2
	if half < 0 {
		half = 0
	}
	return string(runes[:half]) + "..." + string(runes[len(runes)-half:])
}

// SliceLogLines 切片日志行。
// TS 参考: bash-tools.shared.ts L186-212
func SliceLogLines(text string, offset, limit *int) (slice string, totalLines, totalChars int) {
	if text == "" {
		return "", 0, 0
	}
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	totalLines = len(lines)
	totalChars = len(text)

	start := 0
	if offset != nil && *offset >= 0 {
		start = *offset
	}
	if limit != nil && offset == nil {
		tailCount := *limit
		if tailCount < 0 {
			tailCount = 0
		}
		start = totalLines - tailCount
		if start < 0 {
			start = 0
		}
	}

	end := totalLines
	if limit != nil {
		e := start + *limit
		if e < end {
			end = e
		}
	}
	if start > totalLines {
		start = totalLines
	}
	if end > totalLines {
		end = totalLines
	}
	return strings.Join(lines[start:end], "\n"), totalLines, totalChars
}

// ---------- 命令解析 ----------

var tokenizeRe = regexp.MustCompile(`(?:[^\s"']+|"(?:\\.|[^"])*"|'(?:\\.|[^'])*')+`)

// DeriveSessionName 从命令推断会话名。
// TS 参考: bash-tools.shared.ts L214-229
func DeriveSessionName(command string) string {
	tokens := tokenizeCommand(command)
	if len(tokens) == 0 {
		return ""
	}
	verb := tokens[0]
	var target string
	for _, t := range tokens[1:] {
		if !strings.HasPrefix(t, "-") {
			target = t
			break
		}
	}
	if target == "" && len(tokens) > 1 {
		target = tokens[1]
	}
	if target == "" {
		return verb
	}
	cleaned := TruncateMiddle(stripQuotes(target), 48)
	return stripQuotes(verb) + " " + cleaned
}

func tokenizeCommand(command string) []string {
	matches := tokenizeRe.FindAllString(command, -1)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		s := stripQuotes(m)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

func stripQuotes(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 2 {
		if (trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') ||
			(trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'') {
			return trimmed[1 : len(trimmed)-1]
		}
	}
	return trimmed
}

// Pad 右填充空格。
// TS 参考: bash-tools.shared.ts L247-252
func Pad(str string, width int) string {
	if len(str) >= width {
		return str
	}
	return str + strings.Repeat(" ", width-len(str))
}

// ---------- 隐藏依赖: KillSession ----------

// KillSession 向进程树发送 SIGKILL。
// TS 参考: bash-tools.shared.ts L118-123
// Go 实现使用 syscall.Kill(-pid, SIGKILL)（进程组）。
func KillSession(pid int) {
	if pid <= 0 {
		return
	}
	// 尝试发送 SIGKILL 给整个进程组
	p, err := os.FindProcess(-pid)
	if err != nil {
		return
	}
	_ = p.Signal(os.Kill)
	// 也直接 kill 进程本身
	p2, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = p2.Signal(os.Kill)
}

// ---------- 沙盒路径解析 ----------

// ResolveSandboxWorkdir 解析沙盒工作目录。
// TS 参考: bash-tools.shared.ts L84-116
func ResolveSandboxWorkdir(workdir string, sandbox BashSandboxConfig, warnings *[]string) (hostWorkdir, containerWorkdir string) {
	fallback := sandbox.WorkspaceDir
	fi, err := os.Stat(workdir)
	if err != nil || !fi.IsDir() {
		*warnings = append(*warnings, `Warning: workdir "`+workdir+`" is unavailable; using "`+fallback+`".`)
		return fallback, sandbox.ContainerWorkdir
	}

	// 检查 workdir 是否在 workspace 目录下
	rel, err := relPath(sandbox.WorkspaceDir, workdir)
	if err != nil || strings.HasPrefix(rel, "..") {
		*warnings = append(*warnings, `Warning: workdir "`+workdir+`" is outside sandbox; using "`+fallback+`".`)
		return fallback, sandbox.ContainerWorkdir
	}

	if rel == "" || rel == "." {
		return workdir, sandbox.ContainerWorkdir
	}
	// 构建容器工作目录
	containerWd := strings.TrimRight(sandbox.ContainerWorkdir, "/") + "/" + strings.ReplaceAll(rel, string(os.PathSeparator), "/")
	return workdir, containerWd
}

func relPath(base, target string) (string, error) {
	// 简单实现：去除公共前缀
	base = strings.TrimRight(base, string(os.PathSeparator)) + string(os.PathSeparator)
	if strings.HasPrefix(target, base) {
		return target[len(base):], nil
	}
	return ".." + string(os.PathSeparator), nil
}

// ---------- 隐藏依赖: sliceUtf16Safe ----------

// sliceUtf16Safe 安全切片，与 TS 的 sliceUtf16Safe 对应。
// Go 中使用 rune 切片代替 UTF-16 代理对处理。
func sliceUtf16Safe(str string, start, end int) string {
	runes := []rune(str)
	n := len(runes)
	if start < 0 {
		start = n + start
		if start < 0 {
			start = 0
		}
	}
	if end < 0 {
		end = n + end
		if end < 0 {
			end = 0
		}
	}
	if start > n {
		start = n
	}
	if end > n {
		end = n
	}
	if start > end {
		return ""
	}
	return string(runes[start:end])
}

// ---------- 浮点数 ClampFloat ----------

// ClampFloat 将浮点数值限制在 [min, max] 范围内。
func ClampFloat(value *float64, defaultValue, min, max float64) float64 {
	if value == nil || math.IsNaN(*value) {
		return defaultValue
	}
	v := *value
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
