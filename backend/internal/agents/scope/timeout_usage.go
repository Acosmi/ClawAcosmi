package scope

import (
	"math"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- Agent 超时 ----------

// TS 参考: src/agents/timeout.ts (49 行)

const (
	DefaultAgentTimeoutSeconds = 600
	MaxSafeTimeoutMs           = 2_147_000_000
)

// ResolveAgentTimeoutSeconds 解析 Agent 超时秒数。
func ResolveAgentTimeoutSeconds(cfg *types.OpenAcosmiConfig) int {
	if cfg != nil && cfg.Agents != nil && cfg.Agents.Defaults != nil {
		if cfg.Agents.Defaults.TimeoutSeconds != nil {
			raw := *cfg.Agents.Defaults.TimeoutSeconds
			if raw > 0 {
				return raw
			}
		}
	}
	return DefaultAgentTimeoutSeconds
}

// TimeoutOptions 超时解析选项。
type TimeoutOptions struct {
	Cfg             *types.OpenAcosmiConfig
	OverrideMs      *int
	OverrideSeconds *int
	MinMs           int
}

// ResolveAgentTimeoutMs 解析 Agent 超时毫秒数。
// 优先级: overrideMs → overrideSeconds → cfg → default。
// 0 表示 "无超时"，负数回退默认值。
func ResolveAgentTimeoutMs(opts TimeoutOptions) int {
	minMs := max(1, opts.MinMs)
	clamp := func(valueMs int) int {
		return min(max(valueMs, minMs), MaxSafeTimeoutMs)
	}
	defaultMs := clamp(ResolveAgentTimeoutSeconds(opts.Cfg) * 1000)

	if opts.OverrideMs != nil {
		v := *opts.OverrideMs
		if v == 0 {
			return MaxSafeTimeoutMs
		}
		if v < 0 {
			return defaultMs
		}
		return clamp(v)
	}

	if opts.OverrideSeconds != nil {
		v := *opts.OverrideSeconds
		if v == 0 {
			return MaxSafeTimeoutMs
		}
		if v < 0 {
			return defaultMs
		}
		return clamp(v * 1000)
	}

	return defaultMs
}

// ---------- Token 用量 ----------

// TS 参考: src/agents/usage.ts (137 行)

// NormalizedUsage 归一化 token 用量。
type NormalizedUsage struct {
	Input      int `json:"input,omitempty"`
	Output     int `json:"output,omitempty"`
	CacheRead  int `json:"cacheRead,omitempty"`
	CacheWrite int `json:"cacheWrite,omitempty"`
	Total      int `json:"total,omitempty"`
}

// UsageRaw 多供应商原始用量（兼容各种字段名）。
type UsageRaw struct {
	Input                    *int `json:"input,omitempty"`
	Output                   *int `json:"output,omitempty"`
	CacheRead                *int `json:"cacheRead,omitempty"`
	CacheWrite               *int `json:"cacheWrite,omitempty"`
	Total                    *int `json:"total,omitempty"`
	InputTokens              *int `json:"inputTokens,omitempty"`
	OutputTokens             *int `json:"outputTokens,omitempty"`
	PromptTokens             *int `json:"promptTokens,omitempty"`
	CompletionTokens         *int `json:"completionTokens,omitempty"`
	InputTokensSnake         *int `json:"input_tokens,omitempty"`
	OutputTokensSnake        *int `json:"output_tokens,omitempty"`
	PromptTokensSnake        *int `json:"prompt_tokens,omitempty"`
	CompletionTokensSnake    *int `json:"completion_tokens,omitempty"`
	CacheReadInputTokens     *int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens *int `json:"cache_creation_input_tokens,omitempty"`
	TotalTokens              *int `json:"totalTokens,omitempty"`
	TotalTokensSnake         *int `json:"total_tokens,omitempty"`
	CacheReadSnake           *int `json:"cache_read,omitempty"`
	CacheWriteSnake          *int `json:"cache_write,omitempty"`
}

// NormalizeUsage 将原始用量归一化为统一结构。
func NormalizeUsage(raw *UsageRaw) *NormalizedUsage {
	if raw == nil {
		return nil
	}

	input := firstNonNil(raw.Input, raw.InputTokens, raw.InputTokensSnake, raw.PromptTokens, raw.PromptTokensSnake)
	output := firstNonNil(raw.Output, raw.OutputTokens, raw.OutputTokensSnake, raw.CompletionTokens, raw.CompletionTokensSnake)
	cacheRead := firstNonNil(raw.CacheRead, raw.CacheReadSnake, raw.CacheReadInputTokens)
	cacheWrite := firstNonNil(raw.CacheWrite, raw.CacheWriteSnake, raw.CacheCreationInputTokens)
	total := firstNonNil(raw.Total, raw.TotalTokens, raw.TotalTokensSnake)

	if input == 0 && output == 0 && cacheRead == 0 && cacheWrite == 0 && total == 0 {
		return nil
	}

	return &NormalizedUsage{
		Input:      input,
		Output:     output,
		CacheRead:  cacheRead,
		CacheWrite: cacheWrite,
		Total:      total,
	}
}

// HasNonzeroUsage 检查是否有非零用量。
func HasNonzeroUsage(u *NormalizedUsage) bool {
	if u == nil {
		return false
	}
	return u.Input > 0 || u.Output > 0 || u.CacheRead > 0 || u.CacheWrite > 0 || u.Total > 0
}

// DerivePromptTokens 计算提示 token 总量 (input + cache)。
func DerivePromptTokens(input, cacheRead, cacheWrite int) int {
	sum := input + cacheRead + cacheWrite
	if sum > 0 {
		return sum
	}
	return 0
}

// DeriveSessionTotalTokens 推导会话总 token 数（被上下文窗口限制）。
func DeriveSessionTotalTokens(usage *NormalizedUsage, contextTokens int) int {
	if usage == nil {
		return 0
	}
	promptTokens := DerivePromptTokens(usage.Input, usage.CacheRead, usage.CacheWrite)
	total := promptTokens
	if total <= 0 {
		total = usage.Total
	}
	if total <= 0 {
		total = usage.Input
	}
	if total <= 0 {
		return 0
	}
	if contextTokens > 0 {
		total = int(math.Min(float64(total), float64(contextTokens)))
	}
	return total
}

// firstNonNil 返回第一个非空指针的值。
func firstNonNil(ptrs ...*int) int {
	for _, p := range ptrs {
		if p != nil && *p >= 0 {
			return *p
		}
	}
	return 0
}
