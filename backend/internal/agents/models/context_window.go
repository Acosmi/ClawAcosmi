package models

import (
	"math"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 上下文窗口守卫 ----------

// TS 参考: src/agents/context-window-guard.ts (75 行)

const (
	ContextWindowHardMinTokens   = 16_000
	ContextWindowWarnBelowTokens = 32_000
)

// ContextWindowSource 上下文窗口值的来源。
type ContextWindowSource string

const (
	SourceModel              ContextWindowSource = "model"
	SourceModelsConfig       ContextWindowSource = "modelsConfig"
	SourceAgentContextTokens ContextWindowSource = "agentContextTokens"
	SourceDefault            ContextWindowSource = "default"
)

// ContextWindowInfo 上下文窗口信息。
type ContextWindowInfo struct {
	Tokens int                 `json:"tokens"`
	Source ContextWindowSource `json:"source"`
}

// ResolveContextWindowInfo 解析上下文窗口大小。
// 优先级: modelsConfig > model metadata > default, 然后被 agentContextTokens 限制。
func ResolveContextWindowInfo(cfg *types.OpenAcosmiConfig, provider, modelId string, modelContextWindow int, defaultTokens int) ContextWindowInfo {
	// 尝试从 models config 获取
	fromModelsConfig := lookupModelsConfigContextWindow(cfg, provider, modelId)

	var baseInfo ContextWindowInfo
	switch {
	case fromModelsConfig > 0:
		baseInfo = ContextWindowInfo{Tokens: fromModelsConfig, Source: SourceModelsConfig}
	case normalizePositiveInt(modelContextWindow) > 0:
		baseInfo = ContextWindowInfo{Tokens: normalizePositiveInt(modelContextWindow), Source: SourceModel}
	default:
		baseInfo = ContextWindowInfo{Tokens: int(math.Floor(float64(defaultTokens))), Source: SourceDefault}
	}

	// 检查 agentContextTokens 上限
	if cfg != nil && cfg.Agents != nil && cfg.Agents.Defaults != nil {
		if cfg.Agents.Defaults.ContextTokens != nil {
			cap := *cfg.Agents.Defaults.ContextTokens
			if cap > 0 && cap < baseInfo.Tokens {
				return ContextWindowInfo{Tokens: cap, Source: SourceAgentContextTokens}
			}
		}
	}

	return baseInfo
}

// ContextWindowGuardResult 上下文窗口安全检查结果。
type ContextWindowGuardResult struct {
	ContextWindowInfo
	ShouldWarn  bool `json:"shouldWarn"`
	ShouldBlock bool `json:"shouldBlock"`
}

// EvaluateContextWindowGuard 评估上下文窗口安全性。
func EvaluateContextWindowGuard(info ContextWindowInfo, warnBelowTokens, hardMinTokens int) ContextWindowGuardResult {
	if warnBelowTokens <= 0 {
		warnBelowTokens = ContextWindowWarnBelowTokens
	}
	if hardMinTokens <= 0 {
		hardMinTokens = ContextWindowHardMinTokens
	}
	warnBelow := max(1, warnBelowTokens)
	hardMin := max(1, hardMinTokens)
	tokens := max(0, info.Tokens)

	return ContextWindowGuardResult{
		ContextWindowInfo: ContextWindowInfo{Tokens: tokens, Source: info.Source},
		ShouldWarn:        tokens > 0 && tokens < warnBelow,
		ShouldBlock:       tokens > 0 && tokens < hardMin,
	}
}

// normalizePositiveInt 正整数归一化。
func normalizePositiveInt(v int) int {
	if v <= 0 {
		return 0
	}
	return v
}

// lookupModelsConfigContextWindow 从 models config 查找模型上下文窗口。
// TS 参考: context-window-guard.ts L28-37
func lookupModelsConfigContextWindow(cfg *types.OpenAcosmiConfig, provider, modelId string) int {
	if cfg == nil || cfg.Models == nil || cfg.Models.Providers == nil {
		return 0
	}
	providerCfg, ok := cfg.Models.Providers[provider]
	if !ok || providerCfg == nil {
		return 0
	}
	for _, m := range providerCfg.Models {
		if m.ID == modelId {
			return normalizePositiveInt(m.ContextWindow)
		}
	}
	return 0
}
