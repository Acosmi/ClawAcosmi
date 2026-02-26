// bash/shell_utils.go — Shell 配置与工具函数。
// TS 参考：src/agents/shell-utils.ts (173L)
//
// 包含 Shell 检测、PowerShell 解析、二进制输出净化、进程树终止。
package bash

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// ShellConfig Shell 启动配置。
type ShellConfig struct {
	Shell string
	Args  []string
}

// ---------- PowerShell 解析（Windows）----------

// resolvePowerShellPath 查找 PowerShell 可执行文件路径。
// TS 参考: shell-utils.ts resolvePowerShellPath L5-20
func resolvePowerShellPath() string {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = os.Getenv("WINDIR")
	}
	if systemRoot != "" {
		candidate := filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "powershell.exe"
}

// ---------- Shell 配置检测 ----------

// GetShellConfig 检测当前系统的 Shell 配置。
// TS 参考: shell-utils.ts getShellConfig L22-50
func GetShellConfig() ShellConfig {
	if runtime.GOOS == "windows" {
		return ShellConfig{
			Shell: resolvePowerShellPath(),
			Args:  []string{"-NoProfile", "-NonInteractive", "-Command"},
		}
	}

	envShell := strings.TrimSpace(os.Getenv("SHELL"))
	shellName := ""
	if envShell != "" {
		shellName = filepath.Base(envShell)
	}

	// Fish 拒绝常见的 bashisms，优先使用 bash。
	if shellName == "fish" {
		if bash := resolveShellFromPath("bash"); bash != "" {
			return ShellConfig{Shell: bash, Args: []string{"-c"}}
		}
		if sh := resolveShellFromPath("sh"); sh != "" {
			return ShellConfig{Shell: sh, Args: []string{"-c"}}
		}
	}

	shell := envShell
	if shell == "" {
		shell = "sh"
	}
	return ShellConfig{Shell: shell, Args: []string{"-c"}}
}

// resolveShellFromPath 在 PATH 中查找可执行文件。
// TS 参考: shell-utils.ts resolveShellFromPath L52-68
func resolveShellFromPath(name string) string {
	envPath := os.Getenv("PATH")
	if envPath == "" {
		return ""
	}
	entries := strings.Split(envPath, string(os.PathListSeparator))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		candidate := filepath.Join(entry, name)
		fi, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		// 检查是否可执行
		if fi.Mode()&0111 != 0 {
			return candidate
		}
	}
	return ""
}

// NormalizeShellName 规范化 Shell 名称。
// TS 参考: shell-utils.ts normalizeShellName L70-79
func NormalizeShellName(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	name := filepath.Base(trimmed)
	// 移除 Windows 扩展名
	for _, ext := range []string{".exe", ".cmd", ".bat", ".EXE", ".CMD", ".BAT"} {
		if strings.HasSuffix(name, ext) {
			name = name[:len(name)-len(ext)]
			break
		}
	}
	// 只保留字母数字、下划线和连字符
	var buf strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// DetectRuntimeShell 检测当前运行时的 Shell 类型。
// TS 参考: shell-utils.ts detectRuntimeShell L81-125
func DetectRuntimeShell() string {
	override := strings.TrimSpace(os.Getenv("CLAWDBOT_SHELL"))
	if override != "" {
		name := NormalizeShellName(override)
		if name != "" {
			return name
		}
	}

	if runtime.GOOS == "windows" {
		if os.Getenv("POWERSHELL_DISTRIBUTION_CHANNEL") != "" {
			return "pwsh"
		}
		return "powershell"
	}

	envShell := strings.TrimSpace(os.Getenv("SHELL"))
	if envShell != "" {
		name := NormalizeShellName(envShell)
		if name != "" {
			return name
		}
	}

	switch {
	case os.Getenv("POWERSHELL_DISTRIBUTION_CHANNEL") != "":
		return "pwsh"
	case os.Getenv("BASH_VERSION") != "":
		return "bash"
	case os.Getenv("ZSH_VERSION") != "":
		return "zsh"
	case os.Getenv("FISH_VERSION") != "":
		return "fish"
	case os.Getenv("KSH_VERSION") != "":
		return "ksh"
	case os.Getenv("NU_VERSION") != "" || os.Getenv("NUSHELL_VERSION") != "":
		return "nu"
	}

	return ""
}

// ---------- 二进制输出净化 ----------

// SanitizeBinaryOutput 清理二进制输出中的不可见字符。
// TS 参考: shell-utils.ts sanitizeBinaryOutput L127-148
func SanitizeBinaryOutput(text string) string {
	if text == "" {
		return ""
	}

	var buf strings.Builder
	buf.Grow(len(text))

	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		i += size

		if r == utf8.RuneError {
			continue
		}

		// 跳过 Unicode Format 和 Surrogate 类别
		if unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Cs, r) {
			continue
		}

		cp := int(r)

		// 保留 Tab、换行、回车
		if cp == 0x09 || cp == 0x0a || cp == 0x0d {
			buf.WriteRune(r)
			continue
		}

		// 跳过其他控制字符
		if cp < 0x20 {
			continue
		}

		buf.WriteRune(r)
	}

	return buf.String()
}

// ---------- 进程树终止 ----------

// KillProcessTree 终止进程树。
// TS 参考: shell-utils.ts killProcessTree L150-172
func KillProcessTree(pid int) {
	if runtime.GOOS == "windows" {
		// Windows: taskkill /F /T /PID
		cmd := exec.Command("taskkill", "/F", "/T", "/PID", strings.TrimSpace(string(rune(pid+'0'))))
		cmd.Stdout = nil
		cmd.Stderr = nil
		_ = cmd.Start()
		return
	}

	// Unix: 先尝试进程组 -pid，失败再尝试单进程
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}
