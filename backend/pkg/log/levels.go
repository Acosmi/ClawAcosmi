// Package log — 日志级别定义与工具函数。
//
// 对应原版 src/logging/levels.ts。提供日志级别的规范化和优先级排序。
// 级别类型使用 types.LogLevel，本文件仅提供操作函数。
package log

import (
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// AllowedLogLevels 所有合法日志级别（从高优先级到低优先级）
var AllowedLogLevels = []types.LogLevel{
	types.LogSilent,
	types.LogFatal,
	types.LogError,
	types.LogWarn,
	types.LogInfo,
	types.LogDebug,
	types.LogTrace,
}

// levelPriority 日志级别 → 数值映射（数越小优先级越高）。
// 与原版 tslog 兼容：fatal=0, error=1, warn=2, info=3, debug=4, trace=5, silent=∞
var levelPriority = map[types.LogLevel]int{
	types.LogFatal:  0,
	types.LogError:  1,
	types.LogWarn:   2,
	types.LogInfo:   3,
	types.LogDebug:  4,
	types.LogTrace:  5,
	types.LogSilent: 999, // 等价于 Infinity，永远不会被 ≤ 比较匹配
}

// LevelPriority 返回日志级别的数值优先级。
// 数值越小优先级越高。未知级别返回 info 的优先级 (3)。
func LevelPriority(level types.LogLevel) int {
	if p, ok := levelPriority[level]; ok {
		return p
	}
	return levelPriority[types.LogInfo]
}

// IsLevelEnabled 判断 msgLevel 在 minLevel 级别下是否应该输出。
// 例如 IsLevelEnabled("debug", "info") == false（debug 优先级低于 info）
func IsLevelEnabled(msgLevel, minLevel types.LogLevel) bool {
	if minLevel == types.LogSilent {
		return false
	}
	return LevelPriority(msgLevel) <= LevelPriority(minLevel)
}

// NormalizeLogLevel 将任意字符串规范化为合法的 LogLevel。
// 如果 level 无效或为空，则返回 fallback。
func NormalizeLogLevel(level string, fallback types.LogLevel) types.LogLevel {
	candidate := types.LogLevel(strings.TrimSpace(level))
	if candidate == "" {
		return fallback
	}
	for _, allowed := range AllowedLogLevels {
		if candidate == allowed {
			return candidate
		}
	}
	return fallback
}
