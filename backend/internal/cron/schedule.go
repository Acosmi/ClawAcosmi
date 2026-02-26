package cron

import (
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// ============================================================================
// 调度计算 — 根据不同调度类型计算下次执行时间
// 对应 TS: cron/schedule.ts (66L)
// ============================================================================

// cronParser 支持秒级 cron 表达式（6 位）
// 对应 TS croner 的 6 位 cron 支持
var cronParser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// ComputeNextRunAtMs 计算下次执行时间（毫秒）
// fromMs: 从此时间点开始计算
// 返回 -1 表示不需要再次执行
func ComputeNextRunAtMs(schedule CronSchedule, fromMs int64) int64 {
	switch schedule.Kind {
	case ScheduleKindAt:
		return computeAtNextRun(schedule, fromMs)
	case ScheduleKindEvery:
		return computeEveryNextRun(schedule, fromMs)
	case ScheduleKindCron:
		return computeCronNextRun(schedule, fromMs)
	default:
		return -1
	}
}

// computeAtNextRun 处理 "at" 调度：一次性执行
func computeAtNextRun(schedule CronSchedule, fromMs int64) int64 {
	atMs := ParseAbsoluteTimeMs(schedule.At)
	if atMs < 0 {
		return -1
	}
	if atMs <= fromMs {
		return -1 // 已过期
	}
	return atMs
}

// computeEveryNextRun 处理 "every" 调度：固定间隔
func computeEveryNextRun(schedule CronSchedule, fromMs int64) int64 {
	if schedule.EveryMs <= 0 {
		return -1
	}
	// 有锚点时，对齐到锚点的整数倍
	if schedule.AnchorMs != nil {
		anchor := *schedule.AnchorMs
		if fromMs <= anchor {
			return anchor
		}
		elapsed := fromMs - anchor
		periods := elapsed / schedule.EveryMs
		next := anchor + (periods+1)*schedule.EveryMs
		return next
	}
	// 无锚点，简单加间隔
	return fromMs + schedule.EveryMs
}

// computeCronNextRun 处理 "cron" 调度：cron 表达式
func computeCronNextRun(schedule CronSchedule, fromMs int64) int64 {
	expr := strings.TrimSpace(schedule.Expr)
	if expr == "" {
		return -1
	}

	// 解析时区
	var loc *time.Location
	if tz := strings.TrimSpace(schedule.Tz); tz != "" {
		var err error
		loc, err = time.LoadLocation(tz)
		if err != nil {
			loc = time.UTC
		}
	} else {
		loc = time.UTC
	}

	sched, err := cronParser.Parse(expr)
	if err != nil {
		return -1
	}

	fromTime := time.UnixMilli(fromMs).In(loc)
	nextTime := sched.Next(fromTime)
	if nextTime.IsZero() {
		return -1
	}
	return nextTime.UnixMilli()
}
