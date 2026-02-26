package config

// 配置日志工具 — 对应 src/config/logging.ts (19 行)
//
// 提供配置路径格式化和配置更新日志。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FormatConfigPath 格式化配置文件路径（缩短 home 目录为 ~）。
// 对应 TS: formatConfigPath(path)
func FormatConfigPath(path string) string {
	if path == "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if strings.HasPrefix(abs, home) {
		return "~" + abs[len(home):]
	}
	return path
}

// ConfigLogger 配置日志接口
type ConfigLogger interface {
	Info(msg string, args ...interface{})
}

// LogConfigUpdated 记录配置更新日志。
// 对应 TS: logConfigUpdated(runtime, opts)
func LogConfigUpdated(logger ConfigLogger, path, suffix string) {
	formatted := FormatConfigPath(path)
	msg := fmt.Sprintf("Updated %s", formatted)
	if suffix = strings.TrimSpace(suffix); suffix != "" {
		msg = fmt.Sprintf("%s %s", msg, suffix)
	}
	logger.Info(msg)
}
