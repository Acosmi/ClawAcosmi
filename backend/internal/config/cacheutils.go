package config

// 配置缓存工具 — 对应 src/config/cache-utils.ts (28 行)
//
// 提供缓存 TTL 解析、缓存启用检查和文件 mtime 查询。
//
// 依赖: os (文件 stat)

import (
	"os"
	"strconv"
	"strings"
)

// ResolveCacheTTLMs 解析缓存 TTL（毫秒），支持从环境变量字符串覆盖默认值。
// 对应 TS: resolveCacheTtlMs(params)
func ResolveCacheTTLMs(envValue string, defaultTTLMs int) int {
	trimmed := strings.TrimSpace(envValue)
	if trimmed != "" {
		parsed, err := strconv.Atoi(trimmed)
		if err == nil && parsed >= 0 {
			return parsed
		}
	}
	return defaultTTLMs
}

// IsCacheEnabled 检查缓存是否启用（TTL > 0）。
// 对应 TS: isCacheEnabled(ttlMs)
func IsCacheEnabled(ttlMs int) bool {
	return ttlMs > 0
}

// GetFileMtimeMs 返回文件的修改时间（毫秒），若文件不存在或出错则返回 0, false。
// 对应 TS: getFileMtimeMs(filePath)
func GetFileMtimeMs(filePath string) (int64, bool) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, false
	}
	return info.ModTime().UnixMilli(), true
}
