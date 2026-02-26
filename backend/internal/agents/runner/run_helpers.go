package runner

// ============================================================================
// Embedded PI Runner 辅助函数
// 对应 TS: pi-embedded-runner/run.ts L61-135
// ============================================================================

import (
	"math"
	"strings"
)

// ANTHROPIC_MAGIC_STRING_TRIGGER_REFUSAL 避免 Anthropic 拒绝测试 token 污染。
const ANTHROPIC_MAGIC_STRING_TRIGGER_REFUSAL = "ANTHROPIC_MAGIC_STRING_TRIGGER_REFUSAL"
const ANTHROPIC_MAGIC_STRING_REPLACEMENT = "ANTHROPIC MAGIC STRING TRIGGER REFUSAL (redacted)"

// ScrubAnthropicRefusalMagic 替换 Anthropic 拒绝魔术字符串。
func ScrubAnthropicRefusalMagic(prompt string) string {
	if !strings.Contains(prompt, ANTHROPIC_MAGIC_STRING_TRIGGER_REFUSAL) {
		return prompt
	}
	return strings.ReplaceAll(prompt, ANTHROPIC_MAGIC_STRING_TRIGGER_REFUSAL, ANTHROPIC_MAGIC_STRING_REPLACEMENT)
}

// UsageAccumulator token 使用量累加器。
type UsageAccumulator struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cacheRead"`
	CacheWrite int `json:"cacheWrite"`
	Total      int `json:"total"`
}

// NewUsageAccumulator 创建新的累加器。
func NewUsageAccumulator() *UsageAccumulator {
	return &UsageAccumulator{}
}

// NormalizedUsage 标准化后的使用量。
type NormalizedUsage struct {
	Input      *int `json:"input,omitempty"`
	Output     *int `json:"output,omitempty"`
	CacheRead  *int `json:"cacheRead,omitempty"`
	CacheWrite *int `json:"cacheWrite,omitempty"`
	Total      *int `json:"total,omitempty"`
}

// MergeUsage 合并使用量数据。
func (acc *UsageAccumulator) MergeUsage(usage *NormalizedUsage) {
	if usage == nil {
		return
	}
	if usage.Input != nil {
		acc.Input += *usage.Input
	}
	if usage.Output != nil {
		acc.Output += *usage.Output
	}
	if usage.CacheRead != nil {
		acc.CacheRead += *usage.CacheRead
	}
	if usage.CacheWrite != nil {
		acc.CacheWrite += *usage.CacheWrite
	}
	if usage.Total != nil {
		acc.Total += *usage.Total
	}
}

// ToNormalizedUsage 转换为标准化格式。
func (acc *UsageAccumulator) ToNormalizedUsage() *NormalizedUsage {
	if acc.Input == 0 && acc.Output == 0 && acc.Total == 0 && acc.CacheRead == 0 && acc.CacheWrite == 0 {
		return nil
	}
	intPtr := func(v int) *int { return &v }
	result := &NormalizedUsage{}
	if acc.Input > 0 {
		result.Input = intPtr(acc.Input)
	}
	if acc.Output > 0 {
		result.Output = intPtr(acc.Output)
	}
	if acc.CacheRead > 0 {
		result.CacheRead = intPtr(acc.CacheRead)
	}
	if acc.CacheWrite > 0 {
		result.CacheWrite = intPtr(acc.CacheWrite)
	}
	total := acc.Total
	if total == 0 {
		total = acc.Input + acc.Output
	}
	if total > 0 {
		result.Total = intPtr(total)
	}
	return result
}

// --- Context Window ---

// ContextWindowInfo 上下文窗口信息。
type ContextWindowInfo struct {
	Tokens int    `json:"tokens"`
	Source string `json:"source"`
}

// ContextWindowGuard 上下文窗口守卫。
type ContextWindowGuard struct {
	Tokens      int    `json:"tokens"`
	Source      string `json:"source"`
	ShouldWarn  bool   `json:"shouldWarn"`
	ShouldBlock bool   `json:"shouldBlock"`
}

const (
	DefaultContextTokens         = 128_000
	ContextWindowWarnBelowTokens = 8_000
	ContextWindowHardMinTokens   = 2_000
)

// EvaluateContextWindowGuard 评估上下文窗口限制。
func EvaluateContextWindowGuard(info ContextWindowInfo) ContextWindowGuard {
	return ContextWindowGuard{
		Tokens:      info.Tokens,
		Source:      info.Source,
		ShouldWarn:  info.Tokens < ContextWindowWarnBelowTokens,
		ShouldBlock: info.Tokens < ContextWindowHardMinTokens,
	}
}

// --- Error helpers ---

// ParseImageSizeError 从错误消息解析图片大小错误。
func ParseImageSizeError(msg string) *ImageSizeError {
	lower := strings.ToLower(msg)
	if !strings.Contains(lower, "image") || !strings.Contains(lower, "size") {
		return nil
	}
	if !strings.Contains(lower, "too large") && !strings.Contains(lower, "exceeds") {
		return nil
	}
	return &ImageSizeError{Message: msg}
}

// ImageSizeError 图片大小错误。
type ImageSizeError struct {
	Message string
	MaxMb   float64
}

// PickFallbackThinkingLevel 选择降级的思考级别。
func PickFallbackThinkingLevel(msg string, attempted map[string]bool) string {
	lower := strings.ToLower(msg)
	isThinkingError := strings.Contains(lower, "thinking") ||
		strings.Contains(lower, "extended_thinking") ||
		strings.Contains(lower, "budget_tokens")
	if !isThinkingError {
		return ""
	}
	levels := []string{"high", "medium", "low", "off"}
	for _, level := range levels {
		if !attempted[level] {
			return level
		}
	}
	return ""
}

// RedactRunIdentifier 对运行标识符进行脱敏。
func RedactRunIdentifier(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:4] + "…" + s[len(s)-4:]
}

// DescribeUnknownError 描述未知错误。
func DescribeUnknownError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// intMax 返回两个整数中较大的。
func intMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// intMin 返回两个整数中较小的。
func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// clamp 将值限制在范围内。
func clamp(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

var _ = math.MaxInt // 确保 math 被使用
