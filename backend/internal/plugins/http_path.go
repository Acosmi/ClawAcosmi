package plugins

import "strings"

// NormalizePluginHttpPath 规范化插件 HTTP 路径
// 对应 TS: http-path.ts normalizePluginHttpPath
func NormalizePluginHttpPath(path, fallback string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		fallbackTrimmed := strings.TrimSpace(fallback)
		if fallbackTrimmed == "" {
			return ""
		}
		if strings.HasPrefix(fallbackTrimmed, "/") {
			return fallbackTrimmed
		}
		return "/" + fallbackTrimmed
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	return "/" + trimmed
}
