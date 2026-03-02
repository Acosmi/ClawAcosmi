package main

import (
	"testing"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ---------- setup_auth_models.go tests ----------

func TestBuildMinimaxModelDef(t *testing.T) {
	m := buildMinimaxModelDef("MiniMax-M2.1", "", minimaxHostedCost, 200000, 8192)
	if m.Name != "MiniMax M2.1" {
		t.Errorf("expected catalog name 'MiniMax M2.1', got %q", m.Name)
	}
	if m.ID != "MiniMax-M2.1" {
		t.Errorf("expected id 'MiniMax-M2.1', got %q", m.ID)
	}
	if m.ContextWindow != 200000 {
		t.Errorf("expected contextWindow 200000, got %d", m.ContextWindow)
	}

	// 自定义名称
	m2 := buildMinimaxModelDef("custom-id", "Custom Name", minimaxAPICost, 100000, 4096)
	if m2.Name != "Custom Name" {
		t.Errorf("expected 'Custom Name', got %q", m2.Name)
	}

	// 未知 ID fallback
	m3 := buildMinimaxModelDef("unknown-id", "", minimaxAPICost, 100000, 4096)
	if m3.Name != "MiniMax unknown-id" {
		t.Errorf("expected 'MiniMax unknown-id', got %q", m3.Name)
	}
}

func TestBuildMinimaxAPIModelDef(t *testing.T) {
	m := buildMinimaxAPIModelDef("MiniMax-M2.1")
	if m.Cost.Input != 15 {
		t.Errorf("expected input cost 15, got %v", m.Cost.Input)
	}
	if m.Cost.Output != 60 {
		t.Errorf("expected output cost 60, got %v", m.Cost.Output)
	}
}

// ---------- setup_auth_config_minimax.go tests ----------

func TestApplyMinimaxProviderConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplyMinimaxProviderConfig(cfg)

	// lmstudio provider 应存在
	p := cfg.Models.Providers["lmstudio"]
	if p == nil {
		t.Fatal("lmstudio provider not created")
	}
	if p.BaseURL != "http://127.0.0.1:1234/v1" {
		t.Errorf("unexpected baseURL: %s", p.BaseURL)
	}
	if p.API != "openai-responses" {
		t.Errorf("unexpected api: %s", string(p.API))
	}
	if len(p.Models) == 0 {
		t.Fatal("no models in lmstudio provider")
	}
	if p.Models[0].ID != minimaxLMStudioModelID {
		t.Errorf("expected model id %q, got %q", minimaxLMStudioModelID, p.Models[0].ID)
	}
}

func TestApplyMinimaxHostedProviderConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplyMinimaxHostedProviderConfig(cfg, "")

	p := cfg.Models.Providers["minimax"]
	if p == nil {
		t.Fatal("minimax provider not created")
	}
	if p.BaseURL != minimaxDefaultBaseURL {
		t.Errorf("expected default base URL, got %s", p.BaseURL)
	}
	if p.API != "openai-completions" {
		t.Errorf("unexpected api: %s", string(p.API))
	}

	// 自定义 URL
	cfg2 := &types.OpenAcosmiConfig{}
	ApplyMinimaxHostedProviderConfig(cfg2, "https://custom.minimax.io/v1")
	p2 := cfg2.Models.Providers["minimax"]
	if p2.BaseURL != "https://custom.minimax.io/v1" {
		t.Errorf("custom URL not applied: %s", p2.BaseURL)
	}
}

func TestApplyMinimaxApiProviderConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplyMinimaxApiProviderConfig(cfg, "")

	p := cfg.Models.Providers["minimax"]
	if p == nil {
		t.Fatal("minimax provider not created")
	}
	if p.BaseURL != minimaxAnthropicBaseURL {
		t.Errorf("expected anthropic base URL, got %s", p.BaseURL)
	}
	if p.API != "anthropic-messages" {
		t.Errorf("unexpected api: %s", string(p.API))
	}
}

func TestApplyMinimaxConfig_SetsDefault(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplyMinimaxConfig(cfg)

	if cfg.Agents == nil || cfg.Agents.Defaults == nil || cfg.Agents.Defaults.Model == nil {
		t.Fatal("defaults.model not set")
	}
	if cfg.Agents.Defaults.Model.Primary != minimaxLMStudioModelRef {
		t.Errorf("expected primary %q, got %q", minimaxLMStudioModelRef, cfg.Agents.Defaults.Model.Primary)
	}
}

// ---------- setup_auth_config_openacosmi.go tests ----------

func TestApplyAcosmiZenConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplyAcosmiZenConfig(cfg)

	// 检查默认模型
	if cfg.Agents == nil || cfg.Agents.Defaults == nil || cfg.Agents.Defaults.Model == nil {
		t.Fatal("defaults.model not set")
	}
	if cfg.Agents.Defaults.Model.Primary != acosmiZenDefaultModelRef {
		t.Errorf("expected primary %q, got %q", acosmiZenDefaultModelRef, cfg.Agents.Defaults.Model.Primary)
	}

	// 检查 alias
	entry := cfg.Agents.Defaults.Models[acosmiZenDefaultModelRef]
	if entry == nil {
		t.Fatal("model alias entry not created")
	}
	if entry.Alias != "Opus" {
		t.Errorf("expected alias 'Opus', got %q", entry.Alias)
	}
}

func TestApplyAcosmiZenProviderConfig_NoAPIConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplyAcosmiZenProviderConfig(cfg)

	// OpenAcosmi Zen 不应创建 models.providers
	if cfg.Models != nil && cfg.Models.Providers != nil && len(cfg.Models.Providers) > 0 {
		t.Error("OpenAcosmi Zen should not create provider config")
	}
}
