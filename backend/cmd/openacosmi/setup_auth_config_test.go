package main

import (
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/bridge"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

func TestBridgeApplyZhipu(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	bridge.ApplyProviderByID("zhipu", cfg, &bridge.ApplyOpts{SetDefaultModel: true})
	if cfg.Agents == nil || cfg.Agents.Defaults == nil || cfg.Agents.Defaults.Model == nil {
		t.Fatal("agents.defaults.model not set")
	}
	// zhipu → zai (normalized)
	ref := bridge.GetDefaultModelRef("zhipu")
	if cfg.Agents.Defaults.Model.Primary != ref {
		t.Errorf("expected default model %s, got %s", ref, cfg.Agents.Defaults.Model.Primary)
	}
}

func TestBridgeApplyMoonshot(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	bridge.ApplyProviderByID("moonshot", cfg, &bridge.ApplyOpts{SetDefaultModel: true})
	if cfg.Agents.Defaults.Model.Primary == "" {
		t.Error("default model not set")
	}
	p := cfg.Models.Providers["moonshot"]
	if p == nil || p.BaseURL == "" {
		t.Error("provider missing or baseURL empty")
	}
}

func TestBridgeApplyXai(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	bridge.ApplyProviderByID("xai", cfg, &bridge.ApplyOpts{SetDefaultModel: true})
	if cfg.Agents.Defaults.Model.Primary == "" {
		t.Error("default model not set")
	}
	p := cfg.Models.Providers["xai"]
	if p == nil || p.BaseURL == "" {
		t.Error("xai provider not configured")
	}
}

func TestBridgeApplyNoDuplicateModels(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	bridge.ApplyProviderByID("moonshot", cfg, nil)
	bridge.ApplyProviderByID("moonshot", cfg, nil)
	p := cfg.Models.Providers["moonshot"]
	if p == nil {
		t.Fatal("moonshot provider not created")
	}
	// Models should not be duplicated
	ids := make(map[string]int)
	for _, m := range p.Models {
		ids[m.ID]++
	}
	for id, count := range ids {
		if count > 1 {
			t.Errorf("model %s duplicated %d times", id, count)
		}
	}
}

func TestBridgeApplyMultiProviders_NoNilPanic(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	// 连续 Apply 多个 provider，不应 panic
	bridge.ApplyProviderByID("zhipu", cfg, &bridge.ApplyOpts{SetDefaultModel: true})
	bridge.ApplyProviderByID("xai", cfg, &bridge.ApplyOpts{SetDefaultModel: true})
	bridge.ApplyProviderByID("moonshot", cfg, &bridge.ApplyOpts{SetDefaultModel: true})
	if cfg.Agents == nil || cfg.Agents.Defaults == nil {
		t.Error("agents.defaults should be initialized")
	}
}
