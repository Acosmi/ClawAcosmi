package models

// TS 对照: src/agents/opencode-zen-models.ts (316L)
// OpenAcosmi Zen 模型目录 + 别名 + 缓存 + API 发现

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ---------- 常量 ----------

const (
	AcosmiZenBaseURL   = "https://api.openacosmi.com/v1"
	zenCacheTTL        = 1 * time.Hour
	zenDiscoverTimeout = 10 * time.Second
)

// ---------- 别名 ----------

// AcosmiZenModelAliases 模型别名映射。
var AcosmiZenModelAliases = map[string]string{
	"opus": "claude-opus-4-6", "opus-4.6": "claude-opus-4-6",
	"opus-4.5": "claude-opus-4-5", "opus-4": "claude-opus-4-6",
	"sonnet": "claude-sonnet-4-6", "sonnet-4.6": "claude-sonnet-4-6",
	"haiku": "claude-haiku-4-5", "haiku-4.5": "claude-haiku-4-5",
	"codex": "gpt-5.3-codex", "codex-mini": "gpt-5.1-codex-mini",
	"codex-max": "gpt-5.1-codex-max", "gpt-5.1": "gpt-5.1",
	"gpt-5.2": "gpt-5.2", "gpt-5.3": "gpt-5.3-codex",
	"gemini": "gemini-3.1-pro", "gemini-3": "gemini-3.1-pro",
	"gemini-pro": "gemini-3.1-pro", "gemini-3.1": "gemini-3.1-pro",
	"gemini-flash": "gemini-3-flash",
	"gemini-2.5":   "gemini-3.1-pro", "gemini-2.5-pro": "gemini-3.1-pro",
	"gemini-2.5-flash": "gemini-3-flash",
	"gemini-3-pro":     "gemini-3.1-pro",
	"glm":              "glm-4.7", "glm-free": "glm-4.7",
}

// ResolveAcosmiZenAlias 解析别名为完整模型 ID。
func ResolveAcosmiZenAlias(modelIDOrAlias string) string {
	if resolved, ok := AcosmiZenModelAliases[strings.ToLower(strings.TrimSpace(modelIDOrAlias))]; ok {
		return resolved
	}
	return modelIDOrAlias
}

// ---------- 成本 + 上下文 ----------

var zenModelCosts = map[string]ModelCost{
	"gpt-5.3-codex":      {Input: 1.07, Output: 8.5, CacheRead: 0.107},
	"gpt-5.1-codex":      {Input: 1.07, Output: 8.5, CacheRead: 0.107},
	"claude-opus-4-6":    {Input: 15.0, Output: 75.0, CacheRead: 1.5},
	"claude-sonnet-4-6":  {Input: 3.0, Output: 15.0, CacheRead: 0.3},
	"claude-haiku-4-5":   {Input: 0.8, Output: 4.0, CacheRead: 0.08},
	"claude-opus-4-5":    {Input: 15.0, Output: 75.0, CacheRead: 1.5},
	"gemini-3.1-pro":     {Input: 1.25, Output: 10.0, CacheRead: 0.3125},
	"gemini-3-pro":       {Input: 1.25, Output: 10.0, CacheRead: 0.3125},
	"gpt-5.1-codex-mini": {Input: 0.15, Output: 0.6, CacheRead: 0.015},
	"gpt-5.1":            {Input: 2.5, Output: 10.0, CacheRead: 0.25},
	"glm-4.7":            {Input: 0, Output: 0},
	"gemini-3-flash":     {Input: 0.15, Output: 0.6, CacheRead: 0.0375},
	"gpt-5.1-codex-max":  {Input: 1.07, Output: 8.5, CacheRead: 0.107},
	"gpt-5.2":            {Input: 2.5, Output: 10.0, CacheRead: 0.25},
}

// LookupZenModelCost 查找模型定价表。
// 未找到时尝试别名解析。返回 nil 表示未知模型。
func LookupZenModelCost(modelID string) *ModelCost {
	if c, ok := zenModelCosts[modelID]; ok {
		return &c
	}
	resolved := ResolveAcosmiZenAlias(modelID)
	if resolved != modelID {
		if c, ok := zenModelCosts[resolved]; ok {
			return &c
		}
	}
	return nil
}

var zenContextWindows = map[string]int{
	"gpt-5.3-codex": 400000, "gpt-5.1-codex": 400000,
	"claude-opus-4-6": 1000000, "claude-sonnet-4-6": 1000000,
	"claude-haiku-4-5": 200000, "claude-opus-4-5": 200000,
	"gemini-3.1-pro": 1048576, "gemini-3-pro": 1048576,
	"gpt-5.1-codex-mini": 400000, "gpt-5.1": 400000,
	"glm-4.7": 204800, "gemini-3-flash": 1048576,
	"gpt-5.1-codex-max": 400000, "gpt-5.2": 400000,
}

var zenMaxTokens = map[string]int{
	"gpt-5.3-codex": 128000, "gpt-5.1-codex": 128000,
	"claude-opus-4-6": 128000, "claude-sonnet-4-6": 128000,
	"claude-haiku-4-5": 128000, "claude-opus-4-5": 64000,
	"gemini-3.1-pro": 65536, "gemini-3-pro": 65536,
	"gpt-5.1-codex-mini": 128000, "gpt-5.1": 128000,
	"glm-4.7": 131072, "gemini-3-flash": 65536,
	"gpt-5.1-codex-max": 128000, "gpt-5.2": 128000,
}

var zenModelNames = map[string]string{
	"gpt-5.3-codex": "GPT-5.3 Codex", "gpt-5.1-codex": "GPT-5.1 Codex",
	"claude-opus-4-6": "Claude Opus 4.6", "claude-sonnet-4-6": "Claude Sonnet 4.6",
	"claude-haiku-4-5": "Claude Haiku 4.5", "claude-opus-4-5": "Claude Opus 4.5",
	"gemini-3.1-pro": "Gemini 3.1 Pro", "gemini-3-pro": "Gemini 3 Pro",
	"gpt-5.1-codex-mini": "GPT-5.1 Codex Mini", "gpt-5.1": "GPT-5.1",
	"glm-4.7": "GLM-4.7", "gemini-3-flash": "Gemini 3 Flash",
	"gpt-5.1-codex-max": "GPT-5.1 Codex Max", "gpt-5.2": "GPT-5.2",
}

// 图片支持模型。
var zenVisionModels = map[string]bool{
	"claude-opus-4-6": true, "claude-sonnet-4-6": true,
	"claude-haiku-4-5": true, "claude-opus-4-5": true,
	"gemini-3.1-pro": true, "gemini-3-pro": true, "gemini-3-flash": true,
	"gpt-5.2": true, "gpt-5.3-codex": true,
}

// BuildZenModelDefinition 从模型 ID 构建定义。
func BuildZenModelDefinition(modelID string) ModelDefinitionConfig {
	name := zenModelNames[modelID]
	if name == "" {
		name = modelID
	}
	ctx := zenContextWindows[modelID]
	if ctx == 0 {
		ctx = 200000
	}
	maxTok := zenMaxTokens[modelID]
	if maxTok == 0 {
		maxTok = 8192
	}
	reasoning := modelID == "gpt-5.3-codex" || modelID == "gpt-5.1-codex" ||
		modelID == "claude-opus-4-6" || modelID == "claude-sonnet-4-6" ||
		modelID == "gemini-3.1-pro" || modelID == "gemini-3-pro" ||
		modelID == "gpt-5.1-codex-max" || modelID == "gpt-5.2"
	input := []string{"text"}
	if zenVisionModels[modelID] {
		input = append(input, "image")
	}
	def := ModelDefinitionConfig{
		ID: modelID, Name: name,
		Reasoning: modelBoolPtr(reasoning), Input: input,
		ContextWindow: modelIntPtr(ctx), MaxTokens: modelIntPtr(maxTok),
	}
	if cost, ok := zenModelCosts[modelID]; ok {
		def.Cost = &cost
	}
	return def
}

// GetAcosmiZenStaticFallbackModels 静态回退模型列表。
func GetAcosmiZenStaticFallbackModels() []ModelDefinitionConfig {
	ids := []string{
		"gpt-5.3-codex", "claude-opus-4-6", "claude-sonnet-4-6",
		"gemini-3.1-pro", "gpt-5.1-codex-mini", "gpt-5.1",
		"claude-haiku-4-5", "gemini-3-flash", "gpt-5.1-codex-max", "gpt-5.2",
	}
	result := make([]ModelDefinitionConfig, len(ids))
	for i, id := range ids {
		result[i] = BuildZenModelDefinition(id)
	}
	return result
}

// ---------- 缓存 + API ----------

var (
	zenCacheMu    sync.Mutex
	zenCachedAt   time.Time
	zenCachedData []ModelDefinitionConfig
)

// ClearAcosmiZenModelCache 清除缓存。
func ClearAcosmiZenModelCache() {
	zenCacheMu.Lock()
	defer zenCacheMu.Unlock()
	zenCachedData = nil
	zenCachedAt = time.Time{}
}

type zenModelsResp struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// FetchAcosmiZenModels 从 API 获取模型列表。
func FetchAcosmiZenModels(apiKey string) []ModelDefinitionConfig {
	zenCacheMu.Lock()
	if zenCachedData != nil && time.Since(zenCachedAt) < zenCacheTTL {
		cached := zenCachedData
		zenCacheMu.Unlock()
		return cached
	}
	zenCacheMu.Unlock()

	req, err := http.NewRequest("GET", AcosmiZenBaseURL+"/models", nil)
	if err != nil {
		return GetAcosmiZenStaticFallbackModels()
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := &http.Client{Timeout: zenDiscoverTimeout}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return GetAcosmiZenStaticFallbackModels()
	}
	defer resp.Body.Close()

	var body zenModelsResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || len(body.Data) == 0 {
		return GetAcosmiZenStaticFallbackModels()
	}

	result := make([]ModelDefinitionConfig, 0, len(body.Data))
	for _, m := range body.Data {
		if m.ID == "" {
			continue
		}
		result = append(result, BuildZenModelDefinition(m.ID))
	}

	zenCacheMu.Lock()
	zenCachedData = result
	zenCachedAt = time.Now()
	zenCacheMu.Unlock()
	return result
}
