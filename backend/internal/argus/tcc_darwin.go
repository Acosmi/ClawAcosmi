//go:build darwin

package argus

// tcc_darwin.go — macOS TCC (Transparency, Consent, Control) 权限预检
//
// macOS 要求屏幕录制和辅助功能权限，Sequoia (15+) 引入月度重新授权机制。
// 本模块在 Argus 启动前主动检测权限状态，提供精准诊断和恢复引导。

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PermissionState 权限状态。
type PermissionState string

const (
	PermGranted PermissionState = "granted"
	PermDenied  PermissionState = "denied"
	PermUnknown PermissionState = "unknown" // 无法检测（CI 环境等）
)

// TCCStatus macOS TCC 权限状态。
type TCCStatus struct {
	ScreenRecording         PermissionState `json:"screen_recording"`
	Accessibility           PermissionState `json:"accessibility"`
	ScreenRecordingDaysLeft int             `json:"screen_recording_days_left,omitempty"` // 距过期剩余天数（-1 = 未知）
	ScreenRecordingExpiring bool            `json:"screen_recording_expiring,omitempty"`  // 即将过期（<= 5 天）
}

// CheckTCCPermissions 检测 macOS TCC 权限状态。
// 注意: 屏幕录制检测在无窗口服务器环境可能不准确。
func CheckTCCPermissions() TCCStatus {
	status := TCCStatus{
		ScreenRecording:         checkScreenRecording(),
		Accessibility:           checkAccessibility(),
		ScreenRecordingDaysLeft: -1,
	}

	// Sequoia 月度过期检测
	if daysLeft, ok := checkSequoiaScreenRecordingExpiry(); ok {
		status.ScreenRecordingDaysLeft = daysLeft
		status.ScreenRecordingExpiring = daysLeft <= 5
		if status.ScreenRecordingExpiring {
			slog.Warn("argus: macOS screen recording permission expiring soon (Sequoia monthly re-authorization)",
				"days_left", daysLeft,
				"recovery", "Re-authorize Screen Recording in System Settings > Privacy & Security > Screen Recording")
		}
	}

	return status
}

// HasRequiredPermissions 检查是否具备所有必需权限。
func (s TCCStatus) HasRequiredPermissions() bool {
	return s.ScreenRecording == PermGranted && s.Accessibility == PermGranted
}

// Recovery 返回面向用户的恢复指引。
func (s TCCStatus) Recovery() string {
	var missing []string
	if s.ScreenRecording == PermDenied {
		missing = append(missing, "Screen Recording")
	}
	if s.Accessibility == PermDenied {
		missing = append(missing, "Accessibility")
	}
	if len(missing) == 0 {
		if s.ScreenRecordingExpiring {
			return fmt.Sprintf("Screen Recording permission expires in %d days. Re-authorize in System Settings > Privacy & Security > Screen Recording.", s.ScreenRecordingDaysLeft)
		}
		return ""
	}
	return fmt.Sprintf("Grant %s permission in System Settings > Privacy & Security > %s.",
		strings.Join(missing, " and "),
		strings.Join(missing, " / "))
}

// checkScreenRecording 检测屏幕录制权限。
// 使用 CGWindowListCopyWindowInfo 间接检测：无权限时返回空列表。
func checkScreenRecording() PermissionState {
	cmd := exec.Command("osascript", "-e",
		`use framework "CoreGraphics"
set wlist to current application's CGWindowListCopyWindowInfo(current application's kCGWindowListOptionOnScreenOnly, current application's kCGNullWindowID)
if (count of wlist) > 0 then
    return "granted"
else
    return "denied"
end if`)
	output, err := cmd.Output()
	if err != nil {
		slog.Debug("argus: TCC screen recording check failed", "error", err)
		return PermUnknown
	}
	result := strings.TrimSpace(string(output))
	if result == "granted" {
		return PermGranted
	}
	return PermDenied
}

// checkAccessibility 检测辅助功能权限。
// 使用 osascript 调用 AXIsProcessTrusted 检测。
func checkAccessibility() PermissionState {
	cmd := exec.Command("osascript", "-e",
		`use framework "ApplicationServices"
if current application's AXIsProcessTrusted() then
    return "granted"
else
    return "denied"
end if`)
	output, err := cmd.Output()
	if err != nil {
		slog.Debug("argus: TCC accessibility check failed", "error", err)
		return PermUnknown
	}
	result := strings.TrimSpace(string(output))
	if result == "granted" {
		return PermGranted
	}
	return PermDenied
}

// ---------- Sequoia 月度过期检测 ----------

// screenCaptureApprovalsPlistPath 返回 ScreenCaptureApprovals.plist 路径。
func screenCaptureApprovalsPlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Group Containers",
		"group.com.apple.replayd", "ScreenCaptureApprovals.plist")
}

// checkSequoiaScreenRecordingExpiry 检测 Sequoia 屏幕录制权限月度过期状态。
// 返回 (距离 30 天过期的剩余天数, 是否成功检测)。
// macOS Sequoia 每 30 天强制重新授权屏幕录制权限。
func checkSequoiaScreenRecordingExpiry() (daysLeft int, ok bool) {
	plistPath := screenCaptureApprovalsPlistPath()
	if plistPath == "" {
		return -1, false
	}

	// 使用 stat 获取文件最后修改时间作为近似授权时间
	// （plist 在授权变更时会被系统更新）
	info, err := os.Stat(plistPath)
	if err != nil {
		slog.Debug("argus: ScreenCaptureApprovals.plist not found (non-Sequoia or not authorized)", "error", err)
		return -1, false
	}
	modTime := info.ModTime()

	// 尝试用 defaults read 获取更精确的授权时间
	if approvalTime, readOK := readApprovalDateFromPlist(plistPath); readOK {
		modTime = approvalTime
	}

	// Sequoia 月度过期: 30 天
	expiryDate := modTime.Add(30 * 24 * time.Hour)
	remaining := int(time.Until(expiryDate).Hours() / 24)
	if remaining < 0 {
		remaining = 0
	}

	slog.Debug("argus: Sequoia screen recording approval",
		"approval_date", modTime.Format("2006-01-02"),
		"expiry_date", expiryDate.Format("2006-01-02"),
		"days_left", remaining)

	return remaining, true
}

// readApprovalDateFromPlist 尝试从 plist 中读取授权日期。
// 使用 /usr/libexec/PlistBuddy 或 defaults read 命令，避免引入外部依赖。
func readApprovalDateFromPlist(plistPath string) (time.Time, bool) {
	// 尝试用 PlistBuddy 列出所有键
	cmd := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print", plistPath)
	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, false
	}

	// 查找 kScreenCaptureApprovedDate 行
	// 格式: kScreenCaptureApprovedDate = 2026-01-15 10:30:00 +0000
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "kScreenCaptureApprovedDate") {
			// 提取日期部分
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			dateStr := strings.TrimSpace(parts[1])
			// macOS plist 日期格式多种，尝试常见格式
			for _, layout := range []string{
				"2006-01-02 15:04:05 -0700",
				time.RFC3339,
				"Mon Jan 2 15:04:05 MST 2006",
			} {
				if t, err := time.Parse(layout, dateStr); err == nil {
					return t, true
				}
			}
		}
	}

	return time.Time{}, false
}

// SequoiaExpiryDaysFromModTime 从文件修改时间计算到期天数（用于测试）。
func SequoiaExpiryDaysFromModTime(modTime time.Time) int {
	expiryDate := modTime.Add(30 * 24 * time.Hour)
	remaining := int(time.Until(expiryDate).Hours() / 24)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ParseSequoiaExpiryFromStat 从 stat -f 输出解析修改时间戳（用于测试）。
func ParseSequoiaExpiryFromStat(statOutput string) (time.Time, bool) {
	statOutput = strings.TrimSpace(statOutput)
	ts, err := strconv.ParseInt(statOutput, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(ts, 0), true
}
