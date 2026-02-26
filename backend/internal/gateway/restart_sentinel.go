package gateway

// TS 对照: 无直接等价文件，但 TS 端通过 server.impl.ts 写入哨兵文件
// 重启哨兵文件 — 用于标记 gateway 需要重启。
// daemon / update 流程写入哨兵文件后，主进程检测到后自启动。

import (
	"os"
	"path/filepath"
	"time"
)

// RestartSentinelFileName 重启哨兵文件名。
const RestartSentinelFileName = ".openacosmi-restart"

// WriteRestartSentinel 写入重启哨兵文件。
// 写入当前时间戳作为内容。
func WriteRestartSentinel(dir string) error {
	sentinelPath := filepath.Join(dir, RestartSentinelFileName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	content := []byte(time.Now().UTC().Format(time.RFC3339))
	return os.WriteFile(sentinelPath, content, 0o600)
}

// ClearRestartSentinel 清除重启哨兵文件。
func ClearRestartSentinel(dir string) error {
	sentinelPath := filepath.Join(dir, RestartSentinelFileName)
	err := os.Remove(sentinelPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// CheckRestartSentinel 检查重启哨兵文件是否存在。
func CheckRestartSentinel(dir string) bool {
	sentinelPath := filepath.Join(dir, RestartSentinelFileName)
	_, err := os.Stat(sentinelPath)
	return err == nil
}
