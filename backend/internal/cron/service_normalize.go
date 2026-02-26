package cron

import (
	"fmt"
	"strings"
	"unicode/utf16"

	"github.com/anthropic/open-acosmi/internal/routing"
)

// ============================================================================
// 服务层规范化 — Job 名称、描述、agentId 等字段的服务层规范化
// 对应 TS: cron/service/normalize.ts (80L)
// ============================================================================

const (
	maxJobNameLen = 200
	maxJobDescLen = 500
)

// NormalizeRequiredName 规范化必填的 Job 名称
func NormalizeRequiredName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return truncateUtf16Safe(name, maxJobNameLen)
}

// NormalizeOptionalText 规范化可选文本（返回空字符串表示无值）
func NormalizeOptionalText(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return truncateUtf16Safe(text, maxLen)
}

// NormalizeOptionalAgentId 规范化可选的 agentId
// 使用 routing 包的 NormalizeAgentID
func NormalizeOptionalAgentId(agentId string) string {
	agentId = strings.TrimSpace(agentId)
	if agentId == "" {
		return ""
	}
	return routing.NormalizeAgentID(agentId)
}

// InferLegacyName 从调度和负载推断 Job 名称（遗留场景）
func InferLegacyName(schedule CronSchedule, payload CronPayload) string {
	var parts []string

	switch schedule.Kind {
	case ScheduleKindAt:
		parts = append(parts, fmt.Sprintf("at:%s", schedule.At))
	case ScheduleKindEvery:
		parts = append(parts, fmt.Sprintf("every:%dms", schedule.EveryMs))
	case ScheduleKindCron:
		parts = append(parts, fmt.Sprintf("cron:%s", schedule.Expr))
	}

	switch payload.Kind {
	case PayloadKindSystemEvent:
		if payload.Text != "" {
			text := NormalizeOptionalText(payload.Text, 50)
			parts = append(parts, text)
		}
	case PayloadKindAgentTurn:
		if payload.Message != "" {
			msg := NormalizeOptionalText(payload.Message, 50)
			parts = append(parts, msg)
		}
	}

	if len(parts) == 0 {
		return "unnamed-job"
	}
	return truncateUtf16Safe(strings.Join(parts, " / "), maxJobNameLen)
}

// truncateUtf16Safe 按 UTF-16 代码单元长度截断字符串
// 对应 TS: truncateUtf16Safe (utils.ts)
func truncateUtf16Safe(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	runes := []rune(s)
	utf16Len := 0
	lastSafeRune := 0

	for i, r := range runes {
		codeUnits := len(utf16.Encode([]rune{r}))
		if utf16Len+codeUnits > maxLen {
			break
		}
		utf16Len += codeUnits
		lastSafeRune = i + 1
	}

	if lastSafeRune >= len(runes) {
		return s
	}
	return string(runes[:lastSafeRune])
}
