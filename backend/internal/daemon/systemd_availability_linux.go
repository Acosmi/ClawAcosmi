//go:build linux

package daemon

import (
	"os/exec"
	"strings"
)

// IsSystemdUserServiceAvailable 检查当前系统是否可用 systemd user 服务。
// 对应 TS: systemd.ts isSystemdUserServiceAvailable
//
// 逻辑：执行 systemctl --user status
//   - 退出码 0      → true
//   - stderr/stdout 含特定错误关键词 → false
//   - 其他情况（unit 不存在但 systemd 可用，退出码非 0）→ false
//     （保守返回 false，与 TS 原版一致：最后一行 return false）
func IsSystemdUserServiceAvailable() bool {
	cmd := exec.Command("systemctl", "--user", "status")
	out, err := cmd.CombinedOutput()
	if err == nil {
		// 退出码 0：systemd user daemon 正常运行
		return true
	}

	// 合并 stdout+stderr（CombinedOutput 已合并），转小写检查
	detail := strings.ToLower(strings.TrimSpace(string(out)))
	if detail == "" {
		return false
	}

	// 以下关键词表示 systemd user bus 根本不可用
	unavailablePatterns := []string{
		"not found",
		"failed to connect",
		"not been booted",
		"no such file or directory",
		"not supported",
	}
	for _, pat := range unavailablePatterns {
		if strings.Contains(detail, pat) {
			return false
		}
	}

	// 对应 TS 末尾的 return false：保守返回不可用
	return false
}
