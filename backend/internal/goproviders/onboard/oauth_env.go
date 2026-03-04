// onboard/oauth_env.go — OAuth 环境检测
// 对应 TS 文件: src/commands/oauth-env.ts
// 检测是否在远程环境（SSH、Codespaces、无显示的 Linux）中运行。
package onboard

import (
	"os"
	"runtime"
)

// IsRemoteEnvironment 检查是否在远程环境中运行。
// 对应 TS: isRemoteEnvironment()
func IsRemoteEnvironment() bool {
	// SSH 连接
	if os.Getenv("SSH_CLIENT") != "" || os.Getenv("SSH_TTY") != "" || os.Getenv("SSH_CONNECTION") != "" {
		return true
	}

	// 容器/Codespaces 环境
	if os.Getenv("REMOTE_CONTAINERS") != "" || os.Getenv("CODESPACES") != "" {
		return true
	}

	// Linux 无图形显示（排除 WSL）
	if runtime.GOOS == "linux" &&
		os.Getenv("DISPLAY") == "" &&
		os.Getenv("WAYLAND_DISPLAY") == "" &&
		!isWSLEnv() {
		return true
	}

	return false
}

// isWSLEnv 检测是否在 WSL 环境中。
func isWSLEnv() bool {
	// 检查 WSL 特有的环境变量
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSLENV") != "" {
		return true
	}
	// 检查 /proc/version 中是否包含 WSL 标志
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	content := string(data)
	return contains(content, "microsoft") || contains(content, "WSL")
}

// contains 检查字符串是否包含子串（不区分大小写用不到，此处精确匹配）。
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
