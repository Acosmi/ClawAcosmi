package reply

import (
	"time"

	"github.com/anthropic/open-acosmi/internal/autoreply"
)

// TS 对照: auto-reply/reply/directive-handling.persist.ts (247L)
// 指令持久化：将内联指令覆盖写入 SessionEntry。

// PersistDirectiveResult 指令持久化结果。
type PersistDirectiveResult struct {
	Provider      string
	Model         string
	ContextTokens int
}

// PersistDirectiveParams 指令持久化参数。
type PersistDirectiveParams struct {
	Directives              InlineDirectives
	EffectiveModelDirective string
	SessionEntry            *SessionEntry
	SessionKey              string
	StorePath               string
	ElevatedEnabled         bool
	ElevatedAllowed         bool
	DefaultProvider         string
	DefaultModel            string
	Provider                string
	Model                   string
	ContextTokens           int

	// DI 回调：保存 session store。
	SaveSessionFn func(entry *SessionEntry)
}

// PersistInlineDirectives 持久化内联指令到 SessionEntry。
// TS 对照: directive-handling.persist.ts persistInlineDirectives (L25-228)
// Phase 9 D5: 实现指令持久化逻辑。
func PersistInlineDirectives(params PersistDirectiveParams) PersistDirectiveResult {
	directives := params.Directives
	provider := params.Provider
	model := params.Model
	contextTokens := params.ContextTokens
	entry := params.SessionEntry

	if entry == nil || params.SessionKey == "" {
		return PersistDirectiveResult{
			Provider:      provider,
			Model:         model,
			ContextTokens: contextTokens,
		}
	}

	updated := false

	// 1. /think 指令持久化
	if directives.HasThinkDirective && directives.ThinkLevel != "" {
		if directives.ThinkLevel == autoreply.ThinkOff {
			entry.ThinkingLevel = ""
		} else {
			entry.ThinkingLevel = string(directives.ThinkLevel)
		}
		updated = true
	}

	// 2. /verbose 指令持久化
	if directives.HasVerboseDirective && directives.VerboseLevel != "" {
		if directives.VerboseLevel == autoreply.VerboseOff {
			entry.VerboseLevel = ""
		} else {
			entry.VerboseLevel = string(directives.VerboseLevel)
		}
		updated = true
	}

	// 3. /reasoning 指令持久化
	if directives.HasReasoningDirective && directives.ReasoningLevel != "" {
		if directives.ReasoningLevel == autoreply.ReasoningOff {
			entry.ReasoningLevel = ""
		} else {
			entry.ReasoningLevel = string(directives.ReasoningLevel)
		}
		updated = true
	}

	// 4. /elevated 指令持久化
	if directives.HasElevatedDirective &&
		directives.ElevatedLevel != "" &&
		params.ElevatedEnabled &&
		params.ElevatedAllowed {
		// Persist "off" explicitly so inline `/elevated off` overrides defaults.
		entry.ElevatedLevel = string(directives.ElevatedLevel)
		updated = true
	}

	// 5. /exec 指令持久化
	if directives.HasExecDirective && directives.HasExecOptions {
		if directives.ExecHost != "" {
			entry.ExecHost = string(directives.ExecHost)
			updated = true
		}
		if directives.ExecSecurity != "" {
			entry.ExecSecurity = string(directives.ExecSecurity)
			updated = true
		}
		if directives.ExecAsk != "" {
			entry.ExecAsk = string(directives.ExecAsk)
			updated = true
		}
		if directives.ExecNode != "" {
			entry.ExecNode = directives.ExecNode
			updated = true
		}
	}

	// 6. /queue reset 持久化
	if directives.HasQueueDirective && directives.QueueReset {
		entry.QueueMode = ""
		entry.QueueDebounceMs = 0
		entry.QueueCap = 0
		entry.QueueDrop = ""
		updated = true
	}

	// 7. 保存
	if updated {
		entry.UpdatedAt = time.Now().UnixMilli()
		if params.SaveSessionFn != nil {
			params.SaveSessionFn(entry)
		}
	}

	return PersistDirectiveResult{
		Provider:      provider,
		Model:         model,
		ContextTokens: contextTokens,
	}
}
