package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ---------- 模型注册类型 ----------

// ProviderConfig 供应商配置。
// TS 参考: models-config.providers.ts → ProviderConfig (from types.models.ts)
type ProviderConfig struct {
	Name    string                  `json:"name,omitempty"`
	BaseURL string                  `json:"baseUrl,omitempty"`
	APIKey  string                  `json:"apiKey,omitempty"`
	Models  []ModelDefinitionConfig `json:"models,omitempty"`
}

// ModelDefinitionConfig 模型定义配置。
// TS 参考: src/config/types.models.ts → ModelDefinitionConfig
type ModelDefinitionConfig struct {
	ID            string     `json:"id"`
	Name          string     `json:"name,omitempty"`
	ContextWindow *int       `json:"contextWindow,omitempty"`
	MaxTokens     *int       `json:"maxTokens,omitempty"`
	Reasoning     *bool      `json:"reasoning,omitempty"`
	Input         []string   `json:"input,omitempty"` // ["text", "image"]
	Cost          *ModelCost `json:"cost,omitempty"`
}

// ModelCost 模型计费配置。
type ModelCost struct {
	Input      float64 `json:"input,omitempty"`
	Output     float64 `json:"output,omitempty"`
	CacheRead  float64 `json:"cacheRead,omitempty"`
	CacheWrite float64 `json:"cacheWrite,omitempty"`
}

// ModelsJSON models.json 文件结构。
type ModelsJSON struct {
	Providers map[string]ProviderConfig `json:"providers"`
}

// ---------- 合并逻辑 ----------

// MergeProviderModels 合并供应商的隐式和显式模型列表。
// TS 参考: models-config.ts → mergeProviderModels()
// 规则: 显式列表优先，隐式模型中 ID 未出现过的追加到末尾。
func MergeProviderModels(implicit, explicit ProviderConfig) ProviderConfig {
	if len(implicit.Models) == 0 {
		result := implicit
		// 浅合并: explicit 覆盖 implicit 的 top-level 字段
		if explicit.Name != "" {
			result.Name = explicit.Name
		}
		if explicit.BaseURL != "" {
			result.BaseURL = explicit.BaseURL
		}
		if explicit.APIKey != "" {
			result.APIKey = explicit.APIKey
		}
		result.Models = explicit.Models
		return result
	}

	seen := make(map[string]bool)
	for _, m := range explicit.Models {
		id := strings.TrimSpace(m.ID)
		if id != "" {
			seen[id] = true
		}
	}

	merged := make([]ModelDefinitionConfig, 0, len(explicit.Models)+len(implicit.Models))
	merged = append(merged, explicit.Models...)
	for _, m := range implicit.Models {
		id := strings.TrimSpace(m.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		merged = append(merged, m)
	}

	result := implicit
	if explicit.Name != "" {
		result.Name = explicit.Name
	}
	if explicit.BaseURL != "" {
		result.BaseURL = explicit.BaseURL
	}
	if explicit.APIKey != "" {
		result.APIKey = explicit.APIKey
	}
	result.Models = merged
	return result
}

// MergeProviders 合并隐式和显式供应商配置。
// TS 参考: models-config.ts → mergeProviders()
func MergeProviders(implicit, explicit map[string]ProviderConfig) map[string]ProviderConfig {
	out := make(map[string]ProviderConfig)
	for k, v := range implicit {
		out[k] = v
	}
	for k, v := range explicit {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		if imp, ok := out[key]; ok {
			out[key] = MergeProviderModels(imp, v)
		} else {
			out[key] = v
		}
	}
	return out
}

// ---------- models.json 管理 ----------

// ModelsJSONMode 模型 JSON 合并模式。
type ModelsJSONMode string

const (
	ModelsJSONModeMerge   ModelsJSONMode = "merge"
	ModelsJSONModeReplace ModelsJSONMode = "replace"
)

// EnsureModelsJSON 确保 models.json 文件存在并包含最新的供应商+模型配置。
// TS 参考: models-config.ts → ensureOpenAcosmiModelsJson()
// 返回 agentDir 以及是否实际写入了文件。
func EnsureModelsJSON(params EnsureModelsJSONParams) (string, bool, error) {
	agentDir := strings.TrimSpace(params.AgentDir)
	if agentDir == "" {
		return "", false, fmt.Errorf("agentDir 不能为空")
	}

	// 合并显式 + 隐式供应商
	providers := MergeProviders(params.ImplicitProviders, params.ExplicitProviders)
	if len(providers) == 0 {
		return agentDir, false, nil
	}

	mode := params.Mode
	if mode == "" {
		mode = ModelsJSONModeMerge
	}

	targetPath := filepath.Join(agentDir, "models.json")

	mergedProviders := providers
	if mode == ModelsJSONModeMerge {
		existing, err := readModelsJSON(targetPath)
		if err == nil && existing != nil {
			for k, v := range providers {
				existing[k] = v
			}
			mergedProviders = existing
		}
	}

	next := ModelsJSON{Providers: mergedProviders}
	nextData, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return agentDir, false, fmt.Errorf("序列化 models.json 失败: %w", err)
	}
	nextData = append(nextData, '\n')

	// 检查内容是否已相同
	existingData, _ := os.ReadFile(targetPath)
	if string(existingData) == string(nextData) {
		return agentDir, false, nil
	}

	if err := os.MkdirAll(agentDir, 0o700); err != nil {
		return agentDir, false, fmt.Errorf("创建 agentDir 失败: %w", err)
	}
	if err := os.WriteFile(targetPath, nextData, 0o600); err != nil {
		return agentDir, false, fmt.Errorf("写入 models.json 失败: %w", err)
	}

	return agentDir, true, nil
}

// EnsureModelsJSONParams EnsureModelsJSON 的参数。
type EnsureModelsJSONParams struct {
	AgentDir          string
	Mode              ModelsJSONMode
	ExplicitProviders map[string]ProviderConfig
	ImplicitProviders map[string]ProviderConfig
}

func readModelsJSON(path string) (map[string]ProviderConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var mj ModelsJSON
	if err := json.Unmarshal(data, &mj); err != nil {
		return nil, err
	}
	return mj.Providers, nil
}

// ---------- 模型注册表 ----------

// ModelRegistry 全局模型注册表（provider→models 索引）。
// 使用 sync.RWMutex 保证并发安全。
type ModelRegistry struct {
	mu        sync.RWMutex
	providers map[string]ProviderConfig
}

// NewModelRegistry 创建空的模型注册表。
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		providers: make(map[string]ProviderConfig),
	}
}

// LoadFromFile 从 models.json 加载供应商配置。
func (r *ModelRegistry) LoadFromFile(modelsJSONPath string) error {
	providers, err := readModelsJSON(modelsJSONPath)
	if err != nil {
		return fmt.Errorf("加载 models.json 失败: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = providers
	return nil
}

// GetProvider 获取指定供应商。
func (r *ModelRegistry) GetProvider(name string) (ProviderConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// ListProviders 列出所有供应商名称。
func (r *ModelRegistry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for k := range r.providers {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// FindModel 按供应商和模型 ID 查找模型。
func (r *ModelRegistry) FindModel(provider, modelID string) *ModelDefinitionConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[provider]
	if !ok {
		return nil
	}
	normalizedID := strings.ToLower(strings.TrimSpace(modelID))
	for i := range p.Models {
		if strings.ToLower(strings.TrimSpace(p.Models[i].ID)) == normalizedID {
			m := p.Models[i]
			return &m
		}
	}
	return nil
}
