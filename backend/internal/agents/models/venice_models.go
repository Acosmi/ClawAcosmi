package models

// TS 对照: src/agents/venice-models.ts (393L)
// Venice AI 模型目录 + API 动态发现

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ---------- 常量 ----------

const (
	VeniceBaseURL         = "https://api.venice.ai/api/v1"
	VeniceDefaultModelID  = "llama-3.3-70b"
	veniceDiscoverTimeout = 10 * time.Second
)

// ---------- 目录条目 ----------

// VeniceCatalogEntry 静态目录条目。
type VeniceCatalogEntry struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Reasoning     bool     `json:"reasoning"`
	Input         []string `json:"input"`
	ContextWindow int      `json:"contextWindow"`
	MaxTokens     int      `json:"maxTokens"`
	Privacy       string   `json:"privacy"`
}

// modelIntPtr / modelBoolPtr 辅助函数。
func modelIntPtr(v int) *int    { return &v }
func modelBoolPtr(v bool) *bool { return &v }

// VeniceModelCatalog 静态回退目录。
var VeniceModelCatalog = []VeniceCatalogEntry{
	{ID: "llama-3.3-70b", Name: "Llama 3.3 70B", Input: []string{"text"}, ContextWindow: 131072, MaxTokens: 8192, Privacy: "private"},
	{ID: "qwen3-235b-a22b", Name: "Qwen3 235B", Input: []string{"text"}, ContextWindow: 131072, MaxTokens: 8192, Privacy: "private"},
	{ID: "qwen3-coder-480b-a35b-instruct", Name: "Qwen3 Coder 480B", Input: []string{"text"}, ContextWindow: 262144, MaxTokens: 8192, Privacy: "private"},
	{ID: "qwen3-next-80b", Name: "Qwen3 Next 80B", Input: []string{"text"}, ContextWindow: 262144, MaxTokens: 8192, Privacy: "private"},
	{ID: "qwen3-vl-235b-a22b", Name: "Qwen3 VL 235B (Vision)", Reasoning: true, Input: []string{"text", "image"}, ContextWindow: 131072, MaxTokens: 8192, Privacy: "private"},
	{ID: "deepseek-r1-671b", Name: "DeepSeek R1 671B", Reasoning: true, Input: []string{"text"}, ContextWindow: 65536, MaxTokens: 8192, Privacy: "private"},
	{ID: "google-gemma-3-27b-it", Name: "Google Gemma 3 27B Instruct", Input: []string{"text", "image"}, ContextWindow: 202752, MaxTokens: 8192, Privacy: "private"},
	{ID: "openai-gpt-oss-120b", Name: "OpenAI GPT OSS 120B", Input: []string{"text"}, ContextWindow: 131072, MaxTokens: 8192, Privacy: "private"},
	{ID: "openai-gpt-52-codex", Name: "GPT-5.2 Codex (via Venice)", Reasoning: true, Input: []string{"text", "image"}, ContextWindow: 262144, MaxTokens: 8192, Privacy: "anonymized"},
	{ID: "gemini-3-pro-preview", Name: "Gemini 3 Pro (via Venice)", Reasoning: true, Input: []string{"text", "image"}, ContextWindow: 202752, MaxTokens: 8192, Privacy: "anonymized"},
	{ID: "gemini-3-flash-preview", Name: "Gemini 3 Flash (via Venice)", Input: []string{"text", "image"}, ContextWindow: 202752, MaxTokens: 8192, Privacy: "anonymized"},
	{ID: "minimax-m21", Name: "MiniMax M2.1 (via Venice)", Reasoning: true, Input: []string{"text"}, ContextWindow: 202752, MaxTokens: 8192, Privacy: "anonymized"},
}

// BuildVeniceModelDefinition 从目录条目构建 ModelDefinitionConfig。
func BuildVeniceModelDefinition(entry VeniceCatalogEntry) ModelDefinitionConfig {
	return ModelDefinitionConfig{
		ID:            entry.ID,
		Name:          entry.Name,
		Reasoning:     modelBoolPtr(entry.Reasoning),
		Input:         entry.Input,
		ContextWindow: modelIntPtr(entry.ContextWindow),
		MaxTokens:     modelIntPtr(entry.MaxTokens),
	}
}

// GetVeniceStaticFallbackModels 获取静态回退模型列表。
func GetVeniceStaticFallbackModels() []ModelDefinitionConfig {
	result := make([]ModelDefinitionConfig, len(VeniceModelCatalog))
	for i, entry := range VeniceModelCatalog {
		result[i] = BuildVeniceModelDefinition(entry)
	}
	return result
}

// ---------- API 发现 ----------

type veniceModelSpec struct {
	Name    string `json:"name"`
	Privacy string `json:"privacy"`
	Caps    struct {
		SupportsReasoning bool `json:"supportsReasoning"`
		SupportsVision    bool `json:"supportsVision"`
	} `json:"capabilities"`
	AvailableContextTokens int `json:"availableContextTokens"`
}

type veniceModel struct {
	ID   string          `json:"id"`
	Spec veniceModelSpec `json:"model_spec"`
}

type veniceModelsResponse struct {
	Data []veniceModel `json:"data"`
}

// DiscoverVeniceModels 从 Venice API 发现模型，失败时使用静态目录。
// TS 对照: venice-models.ts discoverVeniceModels
func DiscoverVeniceModels() ([]ModelDefinitionConfig, error) {
	client := &http.Client{Timeout: veniceDiscoverTimeout}
	resp, err := client.Get(VeniceBaseURL + "/models")
	if err != nil {
		return GetVeniceStaticFallbackModels(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return GetVeniceStaticFallbackModels(), nil
	}

	var body veniceModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return GetVeniceStaticFallbackModels(), nil
	}

	if len(body.Data) == 0 {
		return GetVeniceStaticFallbackModels(), nil
	}

	result := make([]ModelDefinitionConfig, 0, len(body.Data))
	for _, m := range body.Data {
		input := []string{"text"}
		if m.Spec.Caps.SupportsVision {
			input = append(input, "image")
		}
		ctxWindow := m.Spec.AvailableContextTokens
		if ctxWindow <= 0 {
			ctxWindow = 131072
		}
		reasoning := m.Spec.Caps.SupportsReasoning
		name := m.Spec.Name
		if name == "" {
			name = strings.ReplaceAll(m.ID, "-", " ")
			name = strings.Title(name)
		}
		def := ModelDefinitionConfig{
			ID:            m.ID,
			Name:          fmt.Sprintf("%s (Venice)", name),
			Reasoning:     modelBoolPtr(reasoning),
			Input:         input,
			ContextWindow: modelIntPtr(ctxWindow),
			MaxTokens:     modelIntPtr(8192),
		}
		result = append(result, def)
	}
	return result, nil
}
