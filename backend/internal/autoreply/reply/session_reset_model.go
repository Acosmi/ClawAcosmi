package reply

import (
	"log/slog"
	"strings"

	"github.com/anthropic/open-acosmi/internal/agents/models"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// TS 对照: auto-reply/reply/session-reset-model.ts (203L)
// 会话模型重置/覆盖逻辑。

// ---------- 类型 ----------

// ResetModelResult 模型重置结果。
type ResetModelResult struct {
	Applied  bool
	Provider string
	Model    string
	Message  string
}

// ResetModelParams 模型重置参数。
type ResetModelParams struct {
	Entry           *SessionEntry
	Store           SessionStoreAccessor
	Cfg             *types.OpenAcosmiConfig
	ResetArg        string // 用户提供的重置参数（模型名/别名/空）
	DefaultProvider string
	DefaultModel    string
}

// ---------- 核心函数 ----------

// ApplyResetModelOverride 应用模型重置覆盖。
// TS 对照: session-reset-model.ts applyResetModelOverride (L18-120)
//
// 逻辑：
// 1. 空参数 → 清除覆盖，恢复默认
// 2. "default" → 同上
// 3. 明确模型参数 → 解析并验证 → 应用
func ApplyResetModelOverride(params ResetModelParams) ResetModelResult {
	if params.Entry == nil {
		return ResetModelResult{Message: "no session entry"}
	}

	arg := strings.TrimSpace(params.ResetArg)

	// 清除覆盖
	if arg == "" || strings.EqualFold(arg, "default") {
		params.Entry.ModelOverride = ""
		params.Entry.ProviderOverride = ""
		params.Entry.ContextTokens = nil

		slog.Debug("session_reset_model: cleared override",
			"sessionKey", params.Entry.SessionKey,
		)

		if params.Store != nil {
			params.Store.Save(params.Entry)
		}

		return ResetModelResult{
			Applied:  true,
			Provider: params.DefaultProvider,
			Model:    params.DefaultModel,
			Message:  "Model reset to default",
		}
	}

	// 解析模型参数
	ref := models.ParseModelRef(arg, params.DefaultProvider)
	if ref == nil {
		return ResetModelResult{
			Message: "Could not parse model: " + arg,
		}
	}

	// 应用覆盖
	return ApplySelectionToSession(ApplySelectionParams{
		Entry:    params.Entry,
		Store:    params.Store,
		Provider: ref.Provider,
		Model:    ref.Model,
	})
}

// ApplySelectionParams 模型选择应用参数。
type ApplySelectionParams struct {
	Entry         *SessionEntry
	Store         SessionStoreAccessor
	Provider      string
	Model         string
	ContextTokens *int
}

// ApplySelectionToSession 将模型选择应用到会话条目。
// TS 对照: session-reset-model.ts applySelectionToSession (L122-202)
func ApplySelectionToSession(params ApplySelectionParams) ResetModelResult {
	if params.Entry == nil {
		return ResetModelResult{Message: "no session entry"}
	}

	params.Entry.ProviderOverride = params.Provider
	params.Entry.ModelOverride = params.Model
	if params.ContextTokens != nil {
		params.Entry.ContextTokens = params.ContextTokens
	}

	slog.Debug("session_reset_model: applied override",
		"sessionKey", params.Entry.SessionKey,
		"provider", params.Provider,
		"model", params.Model,
	)

	if params.Store != nil {
		params.Store.Save(params.Entry)
	}

	return ResetModelResult{
		Applied:  true,
		Provider: params.Provider,
		Model:    params.Model,
		Message:  "Model set to " + params.Provider + "/" + params.Model,
	}
}
