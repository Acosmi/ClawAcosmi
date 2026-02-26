package main

import (
	"testing"

	"github.com/anthropic/open-acosmi/pkg/types"
)

func TestApplyZaiConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplyZaiConfig(cfg)
	if cfg.Agents.Defaults.Models["zhipu/glm-4-plus"].Alias != "GLM" {
		t.Error("expected GLM alias")
	}
	if cfg.Agents.Defaults.Model.Primary != "zhipu/glm-4-plus" {
		t.Errorf("expected default model, got %s", cfg.Agents.Defaults.Model.Primary)
	}
}

func TestApplyOpenrouterConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplyOpenrouterConfig(cfg)
	entry := cfg.Agents.Defaults.Models["openrouter/auto"]
	if entry == nil || entry.Alias != "OpenRouter" {
		t.Error("expected OpenRouter alias")
	}
	if cfg.Agents.Defaults.Model.Primary != "openrouter/auto" {
		t.Error("default model not set")
	}
}

func TestApplyCloudflareAiGateway(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplyCloudflareAiGatewayProviderConfig(cfg, &CloudflareAiGatewayParams{
		AccountID: "acct123",
		GatewayID: "gw456",
	})
	p := cfg.Models.Providers["cloudflare-ai-gateway"]
	if p == nil {
		t.Fatal("provider not found")
	}
	if p.BaseURL == "" {
		t.Error("baseURL not set")
	}
	if len(p.Models) != 1 {
		t.Errorf("expected 1 model, got %d", len(p.Models))
	}
}

func TestApplyMoonshotConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplyMoonshotConfig(cfg)
	if cfg.Agents.Defaults.Model.Primary != moonshotModelRef {
		t.Error("default model not set")
	}
	p := cfg.Models.Providers["moonshot"]
	if p == nil || p.BaseURL == "" {
		t.Error("provider missing or baseURL empty")
	}
}

func TestApplyVeniceConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplyVeniceConfig(cfg)
	if cfg.Agents.Defaults.Model.Primary != veniceModelRef {
		t.Error("default model not set")
	}
}

func TestApplyXaiConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplyXaiConfig(cfg)
	if cfg.Agents.Defaults.Model.Primary != xaiModelRef {
		t.Error("default model not set")
	}
	p := cfg.Models.Providers["xai"]
	if p == nil || p.BaseURL != xaiBaseURL {
		t.Error("xai provider not configured")
	}
}

func TestApplyQianfanConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplyQianfanConfig(cfg)
	if cfg.Agents.Defaults.Model.Primary != qianfanModelRef {
		t.Error("default model not set")
	}
}

func TestMergeModelIfMissing_NoDuplicates(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	ApplySyntheticProviderConfig(cfg)
	ApplySyntheticProviderConfig(cfg) // call twice
	p := cfg.Models.Providers["synthetic"]
	if len(p.Models) != 1 {
		t.Errorf("expected no duplicates, got %d models", len(p.Models))
	}
}

func TestEmptyConfig_NoNilPanic(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	// 应该不 panic
	ApplyZaiConfig(cfg)
	ApplyOpenrouterConfig(cfg)
	ApplyXaiConfig(cfg)
	ApplyXiaomiConfig(cfg)
	ApplyVeniceConfig(cfg)
	ApplyQianfanConfig(cfg)
	if cfg.Agents == nil || cfg.Agents.Defaults == nil {
		t.Error("agents.defaults should be initialized")
	}
}
