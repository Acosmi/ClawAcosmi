//go:build darwin

package daemon

import (
	"fmt"
	"os/exec"
	"strings"
)

// SendNotification 通过 osascript 发送 macOS 原生通知。
// 新增 macOS 平台功能，补全 TS src/macos/ 中未覆盖的通知能力。
func SendNotification(title, body string) error {
	// AppleScript 字符串使用双引号包裹，内部双引号和反斜杠需转义
	safeTitle := escapeAppleScript(title)
	safeBody := escapeAppleScript(body)

	script := fmt.Sprintf(
		`display notification "%s" with title "%s"`,
		safeBody, safeTitle,
	)

	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript notification failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// escapeAppleScript 转义 AppleScript 字符串中的双引号和反斜杠。
func escapeAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
