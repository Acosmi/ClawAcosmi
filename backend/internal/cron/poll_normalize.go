package cron

import "github.com/anthropic/open-acosmi/pkg/polls"

// Poll 输入验证 — 委托到 pkg/polls 共享包。
// 保留类型别名供 cron 包内部使用，避免修改调用方签名。

// PollInput 投票输入（委托到 polls.PollInput）
type PollInput = polls.PollInput

// NormalizedPollInput 规范化后的投票输入（委托到 polls.NormalizedPollInput）
type NormalizedPollInput = polls.NormalizedPollInput

// NormalizePollInput 委托到 polls.NormalizePollInput。
func NormalizePollInput(input PollInput, maxOptions int) (NormalizedPollInput, error) {
	return polls.NormalizePollInput(input, maxOptions)
}

// NormalizePollDurationHours 委托到 polls.NormalizePollDurationHours。
func NormalizePollDurationHours(value float64, defaultHours, maxHours float64) float64 {
	return polls.NormalizePollDurationHours(value, defaultHours, maxHours)
}
