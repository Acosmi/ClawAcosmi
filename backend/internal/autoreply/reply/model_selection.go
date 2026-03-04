package reply

import (
	"log/slog"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/agents/models"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// TS 对照: auto-reply/reply/model-selection.ts (585L)
// AutoReply 侧的模型选择集成层。
// 核心解析逻辑在 agents/models/selection.go (408L, 91%覆盖)。
// 本文件提供 autoreply 特有的会话覆盖解析、指令选择、context token 计算。

// ---------- 类型 ----------

// ModelDirectiveSelection 模型指令选择结果。
// TS 对照: model-selection.ts ModelDirectiveSelection
type ModelDirectiveSelection struct {
	Provider      string
	Model         string
	ContextTokens int
	Source        string // "directive" | "session" | "config" | "default"
	AckMessage    string // 用户可见的确认消息
}

// ModelSelectionState 模型选择综合状态。
// TS 对照: model-selection.ts ModelSelectionState
type ModelSelectionState struct {
	Provider        string
	Model           string
	DefaultProvider string
	DefaultModel    string
	ContextTokens   int
	AliasIndex      models.ModelAliasIndex
	AllowedSet      models.AllowedModelSet
	HasOverride     bool
	OverrideSource  string
}

// ---------- 核心函数 ----------

// CreateModelSelectionState 创建模型选择综合状态。
// TS 对照: model-selection.ts createModelSelectionState (L45-100)
// 组合: session override + config default + alias index + allowed set
func CreateModelSelectionState(params CreateModelSelectionStateParams) ModelSelectionState {
	defaultProvider := params.DefaultProvider
	if defaultProvider == "" {
		defaultProvider = models.DefaultProvider
	}
	defaultModel := params.DefaultModel
	if defaultModel == "" {
		defaultModel = models.DefaultModel
	}

	// 构建别名索引
	aliasIndex := models.BuildModelAliasIndex(params.Cfg, defaultProvider)

	// 构建允许集合
	allowedSet := models.BuildAllowedModelSet(params.Cfg, params.Catalog, defaultProvider, defaultModel)

	state := ModelSelectionState{
		Provider:        defaultProvider,
		Model:           defaultModel,
		DefaultProvider: defaultProvider,
		DefaultModel:    defaultModel,
		AliasIndex:      aliasIndex,
		AllowedSet:      allowedSet,
	}

	// 应用会话覆盖
	if params.SessionEntry != nil {
		override := ResolveStoredModelOverride(params.SessionEntry, defaultProvider)
		if override != nil {
			state.Provider = override.Provider
			state.Model = override.Model
			state.HasOverride = true
			state.OverrideSource = "session"
			slog.Debug("model_selection: session override applied",
				"provider", override.Provider,
				"model", override.Model,
			)
		}
		if params.SessionEntry.ContextTokens != nil {
			state.ContextTokens = *params.SessionEntry.ContextTokens
		}
	}

	return state
}

// CreateModelSelectionStateParams 创建模型选择状态参数。
type CreateModelSelectionStateParams struct {
	Cfg             *types.OpenAcosmiConfig
	SessionEntry    *SessionEntry
	DefaultProvider string
	DefaultModel    string
	Catalog         []models.ModelCatalogEntry
}

// ResolveStoredModelOverride 从会话条目读取存储的模型覆盖。
// TS 对照: model-selection.ts resolveStoredModelOverride (L102-140)
func ResolveStoredModelOverride(entry *SessionEntry, defaultProvider string) *models.ModelRef {
	if entry == nil {
		return nil
	}
	if entry.ModelOverride == "" && entry.ProviderOverride == "" {
		return nil
	}

	provider := entry.ProviderOverride
	if provider == "" {
		provider = defaultProvider
	}
	model := entry.ModelOverride
	if model == "" {
		return nil
	}

	return &models.ModelRef{
		Provider: provider,
		Model:    model,
	}
}

// ResolveModelDirectiveSelection 解析模型指令选择。
// TS 对照: model-selection.ts resolveModelDirectiveSelection (L142-260)
// 处理用户通过文本指令设置模型的请求（例如 /model sonnet-4.5）。
func ResolveModelDirectiveSelection(params ResolveModelDirectiveParams) ModelDirectiveSelection {
	raw := strings.TrimSpace(params.ModelArg)
	if raw == "" {
		return ModelDirectiveSelection{
			Provider: params.State.Provider,
			Model:    params.State.Model,
			Source:   "default",
		}
	}

	// 尝试解析
	ref := models.ResolveModelRefFromString(raw, params.State.DefaultProvider, &params.State.AliasIndex)
	if ref == nil {
		// fuzzy match 失败，返回原始值
		slog.Warn("model_selection: could not resolve model",
			"arg", raw,
		)
		return ModelDirectiveSelection{
			Provider:   params.State.Provider,
			Model:      params.State.Model,
			Source:     "default",
			AckMessage: "Could not resolve model: " + raw,
		}
	}

	// 检查是否在允许列表中
	if !params.State.AllowedSet.AllowAny {
		key := models.ModelKey(ref.Provider, ref.Model)
		if !params.State.AllowedSet.AllowedKeys[key] {
			return ModelDirectiveSelection{
				Provider:   params.State.Provider,
				Model:      params.State.Model,
				Source:     "default",
				AckMessage: "Model not allowed: " + ref.Provider + "/" + ref.Model,
			}
		}
	}

	return ModelDirectiveSelection{
		Provider:   ref.Provider,
		Model:      ref.Model,
		Source:     "directive",
		AckMessage: "Model set to " + ref.Provider + "/" + ref.Model,
	}
}

// ResolveModelDirectiveParams 模型指令解析参数。
type ResolveModelDirectiveParams struct {
	ModelArg string
	State    ModelSelectionState
}

// ResolveContextTokens 解析上下文 token 数。
// TS 对照: model-selection.ts resolveContextTokens (L350-380)
func ResolveContextTokens(sessionEntry *SessionEntry, configDefault int, provider, modelID string, catalog []models.ModelCatalogEntry) int {
	// 1. 会话覆盖优先
	if sessionEntry != nil && sessionEntry.ContextTokens != nil && *sessionEntry.ContextTokens > 0 {
		return *sessionEntry.ContextTokens
	}

	// 2. 查找 catalog 中模型的默认 context window
	for _, entry := range catalog {
		if entry.Provider == provider && entry.ID == modelID {
			if entry.ContextWindow != nil && *entry.ContextWindow > 0 {
				return *entry.ContextWindow
			}
		}
	}

	// 3. 配置默认值
	if configDefault > 0 {
		return configDefault
	}

	// 4. 兜底值
	return 200_000 // Claude 默认 200K
}
