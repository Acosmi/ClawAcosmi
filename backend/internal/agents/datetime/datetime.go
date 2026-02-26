package datetime

import (
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- 日期时间工具 ----------

// TS 参考: src/agents/date-time.ts (192 行)

// TimeFormatPreference 时间格式偏好。
type TimeFormatPreference string

const (
	TimeFormatAuto TimeFormatPreference = "auto"
	TimeFormat12   TimeFormatPreference = "12"
	TimeFormat24   TimeFormatPreference = "24"
)

// ResolvedTimeFormat 解析后的时间格式。
type ResolvedTimeFormat string

const (
	Resolved12 ResolvedTimeFormat = "12"
	Resolved24 ResolvedTimeFormat = "24"
)

var (
	cachedTimeFormat     ResolvedTimeFormat
	cachedTimeFormatOnce sync.Once
)

// ResolveUserTimezone 解析用户时区。
// TS 参考: date-time.ts → resolveUserTimezone()
func ResolveUserTimezone(configured string) string {
	trimmed := strings.TrimSpace(configured)
	if trimmed != "" {
		loc, err := time.LoadLocation(trimmed)
		if err == nil && loc != nil {
			return trimmed
		}
	}
	// 回退到系统时区
	host := time.Now().Location().String()
	if host != "" {
		return host
	}
	return "UTC"
}

// ResolveUserTimeFormat 解析时间格式偏好。
// cachedTimeFormat 通过 sync.Once 初始化，保证并发安全。
func ResolveUserTimeFormat(preference TimeFormatPreference) ResolvedTimeFormat {
	if preference == TimeFormat12 {
		return Resolved12
	}
	if preference == TimeFormat24 {
		return Resolved24
	}
	// auto — 使用 sync.Once 保证只检测一次，无数据竞争
	cachedTimeFormatOnce.Do(func() {
		if detectSystemTimeFormat() {
			cachedTimeFormat = Resolved24
		} else {
			cachedTimeFormat = Resolved12
		}
	})
	return cachedTimeFormat
}

// NormalizedTimestamp 归一化后的时间戳。
type NormalizedTimestamp struct {
	TimestampMs  int64
	TimestampUtc string
}

// NormalizeTimestamp 将各种时间值归一化为毫秒时间戳 + UTC ISO 字符串。
// TS 参考: date-time.ts → normalizeTimestamp()
func NormalizeTimestamp(raw interface{}) *NormalizedTimestamp {
	if raw == nil {
		return nil
	}

	var timestampMs int64
	valid := false

	switch v := raw.(type) {
	case time.Time:
		timestampMs = v.UnixMilli()
		valid = true
	case float64:
		if !math.IsInf(v, 0) && !math.IsNaN(v) {
			if v < 1_000_000_000_000 {
				timestampMs = int64(math.Round(v * 1000))
			} else {
				timestampMs = int64(math.Round(v))
			}
			valid = true
		}
	case int64:
		if v < 1_000_000_000_000 {
			timestampMs = v * 1000
		} else {
			timestampMs = v
		}
		valid = true
	case int:
		i64 := int64(v)
		if i64 < 1_000_000_000_000 {
			timestampMs = i64 * 1000
		} else {
			timestampMs = i64
		}
		valid = true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil
		}
		// 纯数字
		if num, err := strconv.ParseFloat(trimmed, 64); err == nil {
			if strings.Contains(trimmed, ".") {
				timestampMs = int64(math.Round(num * 1000))
			} else if len(trimmed) >= 13 {
				timestampMs = int64(math.Round(num))
			} else {
				timestampMs = int64(math.Round(num * 1000))
			}
			valid = true
		} else {
			// ISO 格式
			for _, layout := range []string{
				time.RFC3339Nano,
				time.RFC3339,
				"2006-01-02T15:04:05",
				"2006-01-02",
			} {
				if t, err := time.Parse(layout, trimmed); err == nil {
					timestampMs = t.UnixMilli()
					valid = true
					break
				}
			}
		}
	}

	if !valid {
		return nil
	}

	utcTime := time.UnixMilli(timestampMs).UTC()
	return &NormalizedTimestamp{
		TimestampMs:  timestampMs,
		TimestampUtc: utcTime.Format(time.RFC3339Nano),
	}
}

// detectSystemTimeFormat 检测系统是否使用 24 小时制。
func detectSystemTimeFormat() bool {
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("defaults", "read", "-g", "AppleICUForce24HourTime").Output()
		if err == nil {
			result := strings.TrimSpace(string(out))
			if result == "1" {
				return true
			}
			if result == "0" {
				return false
			}
		}
	}
	// 回退: 使用 Go 的格式化检测
	sample := time.Date(2000, 1, 1, 13, 0, 0, 0, time.UTC)
	formatted := sample.Format("3 PM")
	return !strings.Contains(formatted, "PM")
}

// OrdinalSuffix 返回序数后缀。
func OrdinalSuffix(day int) string {
	if day >= 11 && day <= 13 {
		return "th"
	}
	switch day % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	default:
		return "th"
	}
}

// FormatUserTime 格式化用户友好的时间字符串。
// TS 参考: date-time.ts → formatUserTime()
func FormatUserTime(t time.Time, timezone string, format ResolvedTimeFormat) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return ""
	}
	t = t.In(loc)

	weekday := t.Weekday().String()
	month := t.Month().String()
	day := t.Day()
	year := t.Year()
	suffix := OrdinalSuffix(day)

	var timePart string
	if format == Resolved24 {
		timePart = t.Format("15:04")
	} else {
		timePart = t.Format("3:04 PM")
	}

	return fmt.Sprintf("%s, %s %d%s, %d — %s", weekday, month, day, suffix, year, timePart)
}
