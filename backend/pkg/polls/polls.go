package polls

import (
	"fmt"
	"math"
	"strings"
)

// Poll 输入验证工具 — 移植自 TS src/polls.ts
// 提供跨渠道（Discord / WhatsApp / Cron）共享的投票规范化逻辑。

// PollInput 投票输入（对应 TS PollInput）
type PollInput struct {
	Question      string
	Options       []string
	MaxSelections int     // 0 表示默认 1
	DurationHours float64 // 0 表示未指定
}

// NormalizedPollInput 规范化后的投票输入（对应 TS NormalizedPollInput）
type NormalizedPollInput struct {
	Question      string
	Options       []string
	MaxSelections int
	DurationHours float64 // 0 表示未指定
}

// NormalizePollInput 规范化投票输入：验证 question、trim 选项、校验 maxSelections。
// maxOptions <= 0 表示不限制选项数量。
// 对齐 TS normalizePollInput()：trim + 过滤空值（不做 case-insensitive 去重）。
func NormalizePollInput(input PollInput, maxOptions int) (NormalizedPollInput, error) {
	question := strings.TrimSpace(input.Question)
	if question == "" {
		return NormalizedPollInput{}, fmt.Errorf("poll question is required")
	}

	// trim + 过滤空值（对齐 TS: map(trim).filter(Boolean)）
	var cleaned []string
	for _, opt := range input.Options {
		trimmed := strings.TrimSpace(opt)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}

	if len(cleaned) < 2 {
		return NormalizedPollInput{}, fmt.Errorf("poll requires at least 2 options")
	}
	if maxOptions > 0 && len(cleaned) > maxOptions {
		return NormalizedPollInput{}, fmt.Errorf("poll supports at most %d options", maxOptions)
	}

	// maxSelections：对齐 TS Math.floor + 范围校验
	maxSelections := input.MaxSelections
	if maxSelections <= 0 {
		maxSelections = 1
	}
	if maxSelections < 1 {
		return NormalizedPollInput{}, fmt.Errorf("maxSelections must be at least 1")
	}
	if maxSelections > len(cleaned) {
		return NormalizedPollInput{}, fmt.Errorf("maxSelections cannot exceed option count")
	}

	// durationHours：对齐 TS Math.floor + >= 1 校验
	durationHours := float64(0)
	if input.DurationHours > 0 {
		durationHours = math.Floor(input.DurationHours)
		if durationHours < 1 {
			return NormalizedPollInput{}, fmt.Errorf("durationHours must be at least 1")
		}
	}

	return NormalizedPollInput{
		Question:      question,
		Options:       cleaned,
		MaxSelections: maxSelections,
		DurationHours: durationHours,
	}, nil
}

// NormalizePollDurationHours 将投票时长钳位到 [1, maxHours] 范围。
// value <= 0 时使用 defaultHours。对齐 TS normalizePollDurationHours()。
func NormalizePollDurationHours(value float64, defaultHours, maxHours float64) float64 {
	base := defaultHours
	if value > 0 {
		base = math.Floor(value)
	}
	if base < 1 {
		base = 1
	}
	if base > maxHours {
		base = maxHours
	}
	return base
}
