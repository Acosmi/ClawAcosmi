//go:build darwin

package daemon

import (
	"os/exec"
	"strings"
)

func newPlatformService() GatewayService {
	return newLaunchdService()
}

// CheckAccessibilityPermission 检查当前进程是否拥有 macOS Accessibility 权限。
// 使用 osascript 探测 System Events，无需 cgo。
// 注：TS src/macos/ 中无直接对应文件（TS 侧为 daemon 入口），此为 Go 侧新增平台能力。
func CheckAccessibilityPermission() bool {
	// 尝试 osascript 方式检测：如果无权限则 osascript 会返回错误
	cmd := exec.Command("osascript", "-e",
		`tell application "System Events" to get name of first process`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// 检查输出中是否包含权限相关关键词
		s := strings.ToLower(string(out))
		if strings.Contains(s, "not allowed") || strings.Contains(s, "assistive") {
			return false
		}
		// 其他错误也视为无权限
		return false
	}
	return true
}
