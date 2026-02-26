package reply

import (
	"log/slog"
	"time"
)

// TS 对照: auto-reply/reply/session-usage.ts (104L)
// 会话使用量持久化：token 用量跟踪、模型标记、CLI session ID 更新。

// UsageUpdateParams 用量更新参数。
type UsageUpdateParams struct {
	Entry         *SessionEntry
	Store         SessionStoreAccessor
	InputTokens   int64
	OutputTokens  int64
	Provider      string
	Model         string
	CliSessionID  string
	CliSessionKey string
}

// PersistSessionUsageUpdate 持久化会话使用量更新。
// TS 对照: session-usage.ts persistSessionUsageUpdate (L10-103)
//
// 更新内容：
// - inputTokens / outputTokens / totalTokens
// - modelProvider / modelOverride（记录最近使用的模型）
// - cliSessionIds（如果有 CLI 会话 ID）
// - updatedAt 时间戳
func PersistSessionUsageUpdate(params UsageUpdateParams) {
	if params.Entry == nil {
		return
	}

	now := time.Now().UnixMilli()
	entry := params.Entry

	// 累加 token 用量
	if params.InputTokens > 0 {
		entry.InputTokens += params.InputTokens
	}
	if params.OutputTokens > 0 {
		entry.OutputTokens += params.OutputTokens
	}
	entry.TotalTokens = entry.InputTokens + entry.OutputTokens

	// 记录最近使用的 provider + model
	if params.Provider != "" {
		entry.ModelProvider = params.Provider
	}
	if params.Model != "" {
		// 注意: 这里更新的是 ModelProvider/Model 用于统计，
		// 不覆盖用户通过指令设置的 ModelOverride。
		// TS 对照: session-usage.ts L62-68
	}

	// CLI session ID 追踪
	if params.CliSessionID != "" && params.CliSessionKey != "" {
		if entry.CliSessionIds == nil {
			entry.CliSessionIds = make(map[string]string)
		}
		entry.CliSessionIds[params.CliSessionKey] = params.CliSessionID
	}

	entry.UpdatedAt = now

	slog.Debug("session_usage: updated",
		"sessionKey", entry.SessionKey,
		"inputTokens", entry.InputTokens,
		"outputTokens", entry.OutputTokens,
		"totalTokens", entry.TotalTokens,
		"provider", params.Provider,
	)

	if params.Store != nil {
		params.Store.Save(entry)
	}
}
