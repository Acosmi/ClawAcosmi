//go:build linux

package daemon

import (
	"fmt"
	"os/exec"
	"os/user"
	"strings"
)

// LingerStatus 描述 systemd user linger 的状态。
// 对应 TS: systemd-linger.ts SystemdUserLingerStatus
type LingerStatus struct {
	User   string // 用户名
	Linger bool   // true = "yes"，false = "no"
}

// resolveLingerUser 解析 linger 操作使用的用户名。
// 优先使用 env["USER"] 或 env["LOGNAME"]，回退到当前进程用户。
// 对应 TS: systemd-linger.ts resolveLoginctlUser
func resolveLingerUser(env map[string]string) (string, error) {
	if u := strings.TrimSpace(env["USER"]); u != "" {
		return u, nil
	}
	if u := strings.TrimSpace(env["LOGNAME"]); u != "" {
		return u, nil
	}
	// 回退到当前进程用户
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("无法确定当前用户: %w", err)
	}
	return u.Username, nil
}

// ReadSystemdUserLingerStatus 查询指定用户的 linger 状态。
// 对应 TS: systemd-linger.ts readSystemdUserLingerStatus
//
// 通过 loginctl show-user <user> -p Linger 读取，
// 若 loginctl 不可用则返回 nil, nil。
func ReadSystemdUserLingerStatus(env map[string]string) (*LingerStatus, error) {
	username, err := resolveLingerUser(env)
	if err != nil || username == "" {
		return nil, nil //nolint:nilerr // 对应 TS：用户未知时返回 null
	}

	cmd := exec.Command("loginctl", "show-user", username, "-p", "Linger")
	out, err := cmd.Output()
	if err != nil {
		// loginctl 不可用或用户未登录，对应 TS catch → return null
		return nil, nil //nolint:nilerr
	}

	for _, rawLine := range strings.Split(string(out), "\n") {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(line, "Linger=") {
			continue
		}
		val := strings.ToLower(strings.TrimPrefix(line, "Linger="))
		if val == "yes" || val == "no" {
			return &LingerStatus{
				User:   username,
				Linger: val == "yes",
			}, nil
		}
	}
	return nil, nil
}

// EnableLinger 为指定用户启用 systemd linger，使其服务在注销后继续运行。
// 对应 TS: systemd-linger.ts enableSystemdUserLinger
//
// 若 sudoMode 为非空字符串则在命令前加 sudo：
//   - "prompt"           → sudo loginctl enable-linger <user>
//   - "non-interactive"  → sudo -n loginctl enable-linger <user>
//   - ""                 → loginctl enable-linger <user>（假设已有权限）
func EnableLinger(env map[string]string, overrideUser, sudoMode string) error {
	username := overrideUser
	if username == "" {
		var err error
		username, err = resolveLingerUser(env)
		if err != nil || username == "" {
			return fmt.Errorf("无法确定用户名: %w", err)
		}
	}

	// 构建命令
	argv := []string{}
	if sudoMode != "" {
		if sudoMode == "non-interactive" {
			argv = append(argv, "sudo", "-n")
		} else {
			argv = append(argv, "sudo")
		}
	}
	argv = append(argv, "loginctl", "enable-linger", username)

	cmd := exec.Command(argv[0], argv[1:]...) //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("loginctl enable-linger %s: %s: %w",
			username, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// IsLingerEnabled 检查指定用户的 linger 是否已启用。
// 对应 TS: systemd-linger.ts readSystemdUserLingerStatus → linger === "yes"
func IsLingerEnabled(env map[string]string) (bool, error) {
	status, err := ReadSystemdUserLingerStatus(env)
	if err != nil {
		return false, err
	}
	if status == nil {
		return false, nil
	}
	return status.Linger, nil
}
