package cron

import (
	"fmt"
	"math"
	"time"
)

// ============================================================================
// 时间戳校验 — 验证 cron 调度中的 at 时间戳合理性
// 对应 TS: cron/validate-timestamp.ts (67L)
// ============================================================================

const (
	oneMinuteMs = 60 * 1000
	tenYearsMs  = int64(10 * 365.25 * 24 * 60 * 60 * 1000)
)

// TimestampValidationResult 时间戳校验结果
type TimestampValidationResult struct {
	OK      bool
	Message string
}

// ValidateScheduleTimestamp 校验 cron 调度中的 at 时间戳
// 拒绝：超过 1 分钟过去 / 超过 10 年未来
func ValidateScheduleTimestamp(schedule CronSchedule, nowMs int64) TimestampValidationResult {
	if schedule.Kind != ScheduleKindAt {
		return TimestampValidationResult{OK: true}
	}

	atRaw := schedule.At
	if atRaw == "" {
		return TimestampValidationResult{
			OK:      false,
			Message: fmt.Sprintf("Invalid schedule.at: expected ISO-8601 timestamp (got %s)", schedule.At),
		}
	}

	atMs := ParseAbsoluteTimeMs(atRaw)
	if atMs < 0 || math.IsInf(float64(atMs), 0) {
		return TimestampValidationResult{
			OK:      false,
			Message: fmt.Sprintf("Invalid schedule.at: expected ISO-8601 timestamp (got %s)", schedule.At),
		}
	}

	diffMs := atMs - nowMs

	// 检查是否在过去（允许 1 分钟宽限）
	if diffMs < -oneMinuteMs {
		nowDate := time.UnixMilli(nowMs).UTC().Format(time.RFC3339)
		atDate := time.UnixMilli(atMs).UTC().Format(time.RFC3339)
		minutesAgo := (-diffMs) / oneMinuteMs
		return TimestampValidationResult{
			OK:      false,
			Message: fmt.Sprintf("schedule.at is in the past: %s (%d minutes ago). Current time: %s", atDate, minutesAgo, nowDate),
		}
	}

	// 检查是否过于遥远
	if diffMs > tenYearsMs {
		atDate := time.UnixMilli(atMs).UTC().Format(time.RFC3339)
		yearsAhead := diffMs / int64(365.25*24*60*60*1000)
		return TimestampValidationResult{
			OK:      false,
			Message: fmt.Sprintf("schedule.at is too far in the future: %s (%d years ahead). Maximum allowed: 10 years", atDate, yearsAhead),
		}
	}

	return TimestampValidationResult{OK: true}
}
