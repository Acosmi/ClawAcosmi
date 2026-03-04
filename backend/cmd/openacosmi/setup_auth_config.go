package main

// setup_auth_config.go — Provider 配置应用函数
// TS 对照: src/commands/onboard-auth.config-core.ts (793L)
//
// 26 个 provider config 变换函数，统一使用 helper 模式减少重复。

import (
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 内部 helper (逐步迁移到 goproviders/bridge) ----------

// setModelAlias 在 agents.defaults.models 中注册模型别名。
func setModelAlias(cfg *types.OpenAcosmiConfig, ref, alias string) {
	ensureAgentDefaults(cfg)
	if cfg.Agents.Defaults.Models == nil {
		cfg.Agents.Defaults.Models = make(map[string]*types.AgentModelEntryConfig)
	}
	entry := cfg.Agents.Defaults.Models[ref]
	if entry == nil {
		entry = &types.AgentModelEntryConfig{}
		cfg.Agents.Defaults.Models[ref] = entry
	}
	if entry.Alias == "" {
		entry.Alias = alias
	}
}

// setDefaultModel 设置 agents.defaults.model.primary（保留 fallbacks）。
func setDefaultModel(cfg *types.OpenAcosmiConfig, primary string) {
	ensureAgentDefaults(cfg)
	existing := cfg.Agents.Defaults.Model
	if existing == nil {
		cfg.Agents.Defaults.Model = &types.AgentModelListConfig{Primary: primary}
		return
	}
	existing.Primary = primary
}

// ensureProvider 确保 models.providers 中有指定的 provider。
func ensureProvider(cfg *types.OpenAcosmiConfig, name string) *types.ModelProviderConfig {
	if cfg.Models == nil {
		cfg.Models = &types.ModelsConfig{Mode: "merge"}
	}
	if cfg.Models.Providers == nil {
		cfg.Models.Providers = make(map[string]*types.ModelProviderConfig)
	}
	p := cfg.Models.Providers[name]
	if p == nil {
		p = &types.ModelProviderConfig{}
		cfg.Models.Providers[name] = p
	}
	return p
}

// mergeModelIfMissing 向 provider 添加模型（如不存在）。
func mergeModelIfMissing(p *types.ModelProviderConfig, model types.ModelDefinitionConfig) {
	for _, m := range p.Models {
		if m.ID == model.ID {
			return
		}
	}
	p.Models = append(p.Models, model)
}

// ensureAgentDefaults 确保 agents.defaults 存在。
func ensureAgentDefaults(cfg *types.OpenAcosmiConfig) {
	if cfg.Agents == nil {
		cfg.Agents = &types.AgentsConfig{}
	}
	if cfg.Agents.Defaults == nil {
		cfg.Agents.Defaults = &types.AgentDefaultsConfig{}
	}
}

// ---------- Auth Profile ----------

// AuthProfileParams 多 profile 凭证参数。
type AuthProfileParams struct {
	ProfileID          string
	Provider           string
	Mode               string // "api_key" | "oauth" | "token"
	Email              string
	PreferProfileFirst bool
}

// ApplyAuthProfileConfig 在 auth.profiles 中注册/更新 profile + 维护 auth.order。
func ApplyAuthProfileConfig(cfg *types.OpenAcosmiConfig, params AuthProfileParams) {
	if cfg.Auth == nil {
		cfg.Auth = &types.AuthConfig{}
	}
	if cfg.Auth.Profiles == nil {
		cfg.Auth.Profiles = make(map[string]*types.AuthProfileConfig)
	}

	profile := &types.AuthProfileConfig{
		Provider: params.Provider,
		Mode:     types.AuthProfileMode(params.Mode),
		Email:    params.Email,
	}
	cfg.Auth.Profiles[params.ProfileID] = profile

	// 维护 auth.order
	if cfg.Auth.Order == nil {
		return // 无显式 order 时不创建
	}
	existingOrder, exists := cfg.Auth.Order[params.Provider]
	if !exists {
		return
	}
	preferFirst := params.PreferProfileFirst
	// 确保 profileID 在 order 中
	found := false
	for _, id := range existingOrder {
		if id == params.ProfileID {
			found = true
			break
		}
	}
	if !found {
		existingOrder = append(existingOrder, params.ProfileID)
	}
	if preferFirst {
		// 移到最前
		reordered := []string{params.ProfileID}
		for _, id := range existingOrder {
			if id != params.ProfileID {
				reordered = append(reordered, id)
			}
		}
		existingOrder = reordered
	}
	cfg.Auth.Order[params.Provider] = existingOrder
}

// ---------- 辅助 ----------

func boolPtr(v bool) *bool {
	return &v
}
