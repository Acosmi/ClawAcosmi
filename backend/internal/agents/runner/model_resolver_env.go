package runner

// ============================================================================
// EnvModelResolver — 基于环境变量和供应商默认值的简单 ModelResolver 实现
// 用于开发阶段快速启动，不依赖完整的配置向导。
// ============================================================================

import (
	"github.com/Acosmi/ClawAcosmi/internal/agents/models"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// EnvModelResolver 基于环境变量的简单模型解析器。
// 开发阶段使用，通过供应商默认配置 + 环境变量 API Key 解析模型。
type EnvModelResolver struct {
	Catalog *models.ModelCatalog
}

// ResolveModel 解析模型。
// 优先从 ModelCatalog 查找，找不到则使用供应商默认值创建。
func (r *EnvModelResolver) ResolveModel(provider, modelID, agentDir string, cfg *types.OpenAcosmiConfig) ModelResolveResult {
	// 1. 尝试从目录查找
	if r.Catalog != nil {
		if entry := r.Catalog.FindModel(provider, modelID); entry != nil {
			ctxWindow := 0
			if entry.ContextWindow != nil {
				ctxWindow = *entry.ContextWindow
			}
			return ModelResolveResult{
				Model: &ResolvedModel{
					ID:            entry.ID,
					Provider:      entry.Provider,
					ContextWindow: ctxWindow,
				},
			}
		}
	}

	// 2. 使用供应商默认值创建
	defaults := models.GetProviderDefaults(provider)
	ctxWindow := DefaultContextTokens // 128k fallback
	if defaults != nil && defaults.ContextWindow > 0 {
		ctxWindow = defaults.ContextWindow
	}

	return ModelResolveResult{
		Model: &ResolvedModel{
			ID:            modelID,
			Provider:      provider,
			ContextWindow: ctxWindow,
		},
	}
}

// ResolveContextWindowInfo 解析上下文窗口信息。
func (r *EnvModelResolver) ResolveContextWindowInfo(cfg *types.OpenAcosmiConfig, provider, modelID string, contextWindow int) ContextWindowInfo {
	tokens := contextWindow
	source := "model"
	if tokens <= 0 {
		defaults := models.GetProviderDefaults(provider)
		if defaults != nil && defaults.ContextWindow > 0 {
			tokens = defaults.ContextWindow
			source = "provider-defaults"
		} else {
			tokens = DefaultContextTokens
			source = "global-default"
		}
	}
	return ContextWindowInfo{
		Tokens: tokens,
		Source: source,
	}
}
