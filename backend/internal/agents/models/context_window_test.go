package models

import (
	"testing"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

func intPtr(i int) *int { return &i }

func TestResolveContextWindowInfo_Default(t *testing.T) {
	info := ResolveContextWindowInfo(nil, "anthropic", "claude-3", 0, 200000)
	if info.Source != SourceDefault {
		t.Errorf("source = %q, want default", info.Source)
	}
	if info.Tokens != 200000 {
		t.Errorf("tokens = %d, want 200000", info.Tokens)
	}
}

func TestResolveContextWindowInfo_Model(t *testing.T) {
	info := ResolveContextWindowInfo(nil, "anthropic", "claude-3", 128000, 200000)
	if info.Source != SourceModel {
		t.Errorf("source = %q, want model", info.Source)
	}
	if info.Tokens != 128000 {
		t.Errorf("tokens = %d, want 128000", info.Tokens)
	}
}

func TestResolveContextWindowInfo_AgentCap(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Agents: &types.AgentsConfig{
			Defaults: &types.AgentDefaultsConfig{
				ContextTokens: intPtr(50000),
			},
		},
	}
	info := ResolveContextWindowInfo(cfg, "anthropic", "claude-3", 128000, 200000)
	if info.Source != SourceAgentContextTokens {
		t.Errorf("source = %q, want agentContextTokens", info.Source)
	}
	if info.Tokens != 50000 {
		t.Errorf("tokens = %d, want 50000 (capped)", info.Tokens)
	}
}

func TestEvaluateContextWindowGuard(t *testing.T) {
	// 正常值
	result := EvaluateContextWindowGuard(
		ContextWindowInfo{Tokens: 100000, Source: SourceModel},
		0, 0,
	)
	if result.ShouldWarn || result.ShouldBlock {
		t.Error("100k tokens should not warn or block")
	}

	// 低于警告阈值
	result = EvaluateContextWindowGuard(
		ContextWindowInfo{Tokens: 20000, Source: SourceDefault},
		0, 0,
	)
	if !result.ShouldWarn {
		t.Error("20k tokens should warn (< 32k)")
	}
	if result.ShouldBlock {
		t.Error("20k tokens should not block (> 16k)")
	}

	// 低于硬限
	result = EvaluateContextWindowGuard(
		ContextWindowInfo{Tokens: 10000, Source: SourceDefault},
		0, 0,
	)
	if !result.ShouldWarn {
		t.Error("10k should warn")
	}
	if !result.ShouldBlock {
		t.Error("10k should block (< 16k)")
	}
}

func TestLookupModelsConfigContextWindow(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Models: &types.ModelsConfig{
			Providers: map[string]*types.ModelProviderConfig{
				"anthropic": {
					BaseURL: "https://api.anthropic.com/v1",
					Models: []types.ModelDefinitionConfig{
						{ID: "claude-3-haiku", ContextWindow: 200000},
						{ID: "claude-3-sonnet", ContextWindow: 150000},
					},
				},
			},
		},
	}

	// 匹配模型
	cw := lookupModelsConfigContextWindow(cfg, "anthropic", "claude-3-haiku")
	if cw != 200000 {
		t.Errorf("cw = %d, want 200000", cw)
	}

	// 不存在的模型
	cw = lookupModelsConfigContextWindow(cfg, "anthropic", "missing-model")
	if cw != 0 {
		t.Errorf("cw = %d, want 0", cw)
	}

	// 不存在的供应商
	cw = lookupModelsConfigContextWindow(cfg, "openai", "gpt-4")
	if cw != 0 {
		t.Errorf("cw = %d, want 0", cw)
	}

	// nil 配置
	cw = lookupModelsConfigContextWindow(nil, "anthropic", "claude-3")
	if cw != 0 {
		t.Errorf("cw = %d, want 0", cw)
	}
}

func TestResolveContextWindowInfo_ModelsConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Models: &types.ModelsConfig{
			Providers: map[string]*types.ModelProviderConfig{
				"openai": {
					Models: []types.ModelDefinitionConfig{
						{ID: "gpt-4", ContextWindow: 128000},
					},
				},
			},
		},
	}
	info := ResolveContextWindowInfo(cfg, "openai", "gpt-4", 0, 200000)
	if info.Source != SourceModelsConfig {
		t.Errorf("source = %q, want modelsConfig", info.Source)
	}
	if info.Tokens != 128000 {
		t.Errorf("tokens = %d, want 128000", info.Tokens)
	}
}
