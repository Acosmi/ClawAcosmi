package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeProviderModels(t *testing.T) {
	implicit := ProviderConfig{
		Name:    "openai",
		BaseURL: "https://api.openai.com/v1",
		Models: []ModelDefinitionConfig{
			{ID: "gpt-4"},
			{ID: "gpt-3.5"},
		},
	}
	explicit := ProviderConfig{
		Models: []ModelDefinitionConfig{
			{ID: "gpt-4", Name: "GPT-4 Custom"},
			{ID: "custom-model"},
		},
	}

	result := MergeProviderModels(implicit, explicit)
	if len(result.Models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(result.Models))
	}
	// gpt-4 (explicit), custom-model (explicit), gpt-3.5 (implicit, not seen)
	if result.Models[0].ID != "gpt-4" || result.Models[0].Name != "GPT-4 Custom" {
		t.Errorf("first model should be explicit gpt-4: %+v", result.Models[0])
	}
	if result.Models[2].ID != "gpt-3.5" {
		t.Errorf("third model should be implicit gpt-3.5: %+v", result.Models[2])
	}
	if result.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("BaseURL should come from implicit: %q", result.BaseURL)
	}
}

func TestMergeProviders(t *testing.T) {
	implicit := map[string]ProviderConfig{
		"openai": {Name: "OpenAI", Models: []ModelDefinitionConfig{{ID: "gpt-4"}}},
	}
	explicit := map[string]ProviderConfig{
		"openai":    {Models: []ModelDefinitionConfig{{ID: "gpt-5"}}},
		"anthropic": {Name: "Anthropic"},
	}

	result := MergeProviders(implicit, explicit)
	if len(result) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(result))
	}
	if _, ok := result["anthropic"]; !ok {
		t.Error("missing 'anthropic' provider")
	}
	openai := result["openai"]
	if len(openai.Models) != 2 {
		t.Errorf("openai should have 2 merged models, got %d", len(openai.Models))
	}
}

func TestModelRegistry(t *testing.T) {
	dir := t.TempDir()
	modelsPath := filepath.Join(dir, "models.json")

	// 写入测试 models.json
	content := `{
  "providers": {
    "anthropic": {
      "name": "Anthropic",
      "models": [
        {"id": "claude-3", "name": "Claude 3"},
        {"id": "claude-3.5", "name": "Claude 3.5", "reasoning": true}
      ]
    }
  }
}`
	if err := os.WriteFile(modelsPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	registry := NewModelRegistry()
	if err := registry.LoadFromFile(modelsPath); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	providers := registry.ListProviders()
	if len(providers) != 1 || providers[0] != "anthropic" {
		t.Errorf("ListProviders: %v", providers)
	}

	p, ok := registry.GetProvider("anthropic")
	if !ok {
		t.Fatal("anthropic provider not found")
	}
	if len(p.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(p.Models))
	}

	m := registry.FindModel("anthropic", "Claude-3")
	if m == nil || m.ID != "claude-3" {
		t.Errorf("FindModel(anthropic, Claude-3) = %+v", m)
	}

	missing := registry.FindModel("anthropic", "nonexistent")
	if missing != nil {
		t.Errorf("FindModel should return nil for missing: %+v", missing)
	}
}

func TestEnsureModelsJSON(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, "agent")

	result, wrote, err := EnsureModelsJSON(EnsureModelsJSONParams{
		AgentDir: agentDir,
		ExplicitProviders: map[string]ProviderConfig{
			"test": {Name: "Test", Models: []ModelDefinitionConfig{{ID: "m1"}}},
		},
	})
	if err != nil {
		t.Fatalf("EnsureModelsJSON: %v", err)
	}
	if !wrote {
		t.Error("expected wrote=true on first call")
	}
	if result != agentDir {
		t.Errorf("agentDir mismatch: %q", result)
	}

	// 验证文件存在
	data, err := os.ReadFile(filepath.Join(agentDir, "models.json"))
	if err != nil {
		t.Fatalf("read models.json: %v", err)
	}
	if len(data) == 0 {
		t.Error("models.json is empty")
	}

	// 第二次调用 — 内容相同应返回 wrote=false
	_, wrote2, _ := EnsureModelsJSON(EnsureModelsJSONParams{
		AgentDir: agentDir,
		ExplicitProviders: map[string]ProviderConfig{
			"test": {Name: "Test", Models: []ModelDefinitionConfig{{ID: "m1"}}},
		},
	})
	if wrote2 {
		t.Error("expected wrote=false on second call with same content")
	}
}
