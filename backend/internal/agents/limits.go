package agents

import (
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- Agent 运行时限制常量 ----------

// TS 参考: src/config/agent-limits.ts
// Phase 1 F1 修复后从 config defaults 移出到 Agent Engine 层。

const (
	// DefaultAgentMaxConcurrent Agent 默认最大并发数。
	DefaultAgentMaxConcurrent = 4
	// DefaultSubagentMaxConcurrent 子 Agent 默认最大并发数。
	DefaultSubagentMaxConcurrent = 8
)

// ResolveAgentMaxConcurrent 解析 Agent 最大并发数。
// 从配置中读取，若无效则使用默认值。
func ResolveAgentMaxConcurrent(cfg *types.OpenAcosmiConfig) int {
	if cfg == nil || cfg.Agents == nil || cfg.Agents.Defaults == nil {
		return DefaultAgentMaxConcurrent
	}
	raw := cfg.Agents.Defaults.MaxConcurrent
	if raw != nil && *raw > 0 {
		return max(1, *raw)
	}
	return DefaultAgentMaxConcurrent
}

// ResolveSubagentMaxConcurrent 解析子 Agent 最大并发数。
// 从配置中读取，若无效则使用默认值。
func ResolveSubagentMaxConcurrent(cfg *types.OpenAcosmiConfig) int {
	if cfg == nil || cfg.Agents == nil || cfg.Agents.Defaults == nil {
		return DefaultSubagentMaxConcurrent
	}
	sub := cfg.Agents.Defaults.Subagents
	if sub == nil {
		return DefaultSubagentMaxConcurrent
	}
	if sub.MaxConcurrent != nil && *sub.MaxConcurrent > 0 {
		return max(1, *sub.MaxConcurrent)
	}
	return DefaultSubagentMaxConcurrent
}
