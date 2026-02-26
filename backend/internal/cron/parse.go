package cron

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// 时间戳解析 — 将字符串解析为毫秒时间戳
// 对应 TS: cron/parse.ts (32L)
// ============================================================================

var (
	isoTzRe       = regexp.MustCompile(`(?i)(Z|[+-]\d{2}:?\d{2})$`)
	isoDateRe     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	isoDateTimeRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T`)
	pureDigitsRe  = regexp.MustCompile(`^\d+$`)
)

// normalizeUtcISO 规范化 ISO 时间字符串，确保包含时区信息
func normalizeUtcISO(raw string) string {
	if isoTzRe.MatchString(raw) {
		return raw
	}
	if isoDateRe.MatchString(raw) {
		return raw + "T00:00:00Z"
	}
	if isoDateTimeRe.MatchString(raw) {
		return raw + "Z"
	}
	return raw
}

// ParseAbsoluteTimeMs 解析时间字符串为毫秒时间戳
// 支持格式：纯数字（毫秒）、ISO-8601 日期/时间
// 返回 -1 表示解析失败
func ParseAbsoluteTimeMs(input string) int64 {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return -1
	}

	// 纯数字 → 视为毫秒时间戳
	if pureDigitsRe.MatchString(raw) {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err == nil && n > 0 && !math.IsInf(float64(n), 0) {
			return n
		}
	}

	// ISO 日期/时间 → time.Parse
	normalized := normalizeUtcISO(raw)
	// 尝试多种 ISO-8601 格式
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, normalized); err == nil {
			return t.UnixMilli()
		}
	}

	return -1
}

// ParseAbsoluteTimeMsPtr 同 ParseAbsoluteTimeMs，但返回指针（nil 表示解析失败）
func ParseAbsoluteTimeMsPtr(input string) *int64 {
	ms := ParseAbsoluteTimeMs(input)
	if ms < 0 {
		return nil
	}
	return &ms
}
