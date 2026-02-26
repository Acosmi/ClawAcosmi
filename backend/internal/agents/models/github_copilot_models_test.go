package models

import (
	"testing"

	"github.com/anthropic/open-acosmi/pkg/types"
)

func TestGetDefaultCopilotModelIDs(t *testing.T) {
	ids := GetDefaultCopilotModelIDs()

	if len(ids) != 7 {
		t.Errorf("GetDefaultCopilotModelIDs() returned %d models, want 7", len(ids))
	}

	// 验证不可变性 — 修改副本不影响源
	ids[0] = "modified"
	ids2 := GetDefaultCopilotModelIDs()
	if ids2[0] == "modified" {
		t.Error("GetDefaultCopilotModelIDs() returned mutable slice")
	}

	// 验证包含核心模型
	expected := map[string]bool{
		"gpt-4o":       true,
		"gpt-4.1":      true,
		"gpt-4.1-mini": true,
		"o1":           true,
		"o3-mini":      true,
	}
	for _, id := range ids2 {
		delete(expected, id)
	}
	if len(expected) > 0 {
		t.Errorf("missing models: %v", expected)
	}
}

func TestBuildCopilotModelDefinition(t *testing.T) {
	model := BuildCopilotModelDefinition("gpt-4o")

	if model.ID != "gpt-4o" {
		t.Errorf("ID = %q, want gpt-4o", model.ID)
	}
	if model.Name != "gpt-4o" {
		t.Errorf("Name = %q, want gpt-4o", model.Name)
	}
	if model.API != types.ModelAPIOpenAIResponses {
		t.Errorf("API = %q, want openai-responses", model.API)
	}
	if model.Reasoning {
		t.Error("Reasoning should be false")
	}
	if len(model.Input) != 2 {
		t.Errorf("Input length = %d, want 2", len(model.Input))
	}
	if model.Input[0] != types.ModelInputText || model.Input[1] != types.ModelInputImage {
		t.Errorf("Input = %v, want [text, image]", model.Input)
	}
	if model.Cost.Input != 0 || model.Cost.Output != 0 {
		t.Error("Cost should be zero")
	}
	if model.ContextWindow != 128_000 {
		t.Errorf("ContextWindow = %d, want 128000", model.ContextWindow)
	}
	if model.MaxTokens != 8192 {
		t.Errorf("MaxTokens = %d, want 8192", model.MaxTokens)
	}
}

func TestBuildDefaultCopilotModels(t *testing.T) {
	models := BuildDefaultCopilotModels()

	if len(models) != 7 {
		t.Errorf("BuildDefaultCopilotModels() returned %d models, want 7", len(models))
	}

	// 每个模型应有正确的 API 类型
	for _, m := range models {
		if m.API != types.ModelAPIOpenAIResponses {
			t.Errorf("model %q has API=%q, want openai-responses", m.ID, m.API)
		}
		if m.ContextWindow != 128_000 {
			t.Errorf("model %q has ContextWindow=%d, want 128000", m.ID, m.ContextWindow)
		}
	}
}
