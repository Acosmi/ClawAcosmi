package reply

import (
	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

// TS 对照: auto-reply/reply/directive-handling.fast-lane.ts (139L)

// FastLaneResult fast-lane 处理结果。
// TS 对照: directive-handling.fast-lane.ts applyInlineDirectivesFastLane 返回值
type FastLaneResult struct {
	DirectiveAck *autoreply.ReplyPayload
	Provider     string
	Model        string
}

// FastLaneParams fast-lane 处理参数。
// TS 对照: directive-handling.fast-lane.ts applyInlineDirectivesFastLane params (L11-47)
type FastLaneParams struct {
	Directives        InlineDirectives
	CommandAuthorized bool
	IsGroup           bool
	SessionEntry      *SessionEntry
	SessionKey        string
	StorePath         string
	ElevatedEnabled   bool
	ElevatedAllowed   bool
	ElevatedFailures  []ElevatedGateFailure
	DefaultProvider   string
	DefaultModel      string
	Provider          string
	Model             string
	InitialModelLabel string

	// 当前 level 状态（从 session/agentCfg 解析）
	CurrentThinkLevel     autoreply.ThinkLevel
	CurrentVerboseLevel   autoreply.VerboseLevel
	CurrentReasoningLevel autoreply.ReasoningLevel
	CurrentElevatedLevel  autoreply.ElevatedLevel

	// 运行时沙箱状态
	RuntimeIsSandboxed bool

	// exec 默认值
	ExecDefaults ExecDefaults

	// HandleDirectiveOnly 的模型相关结果（调用方预先解析）
	ModelErrorText    string
	ResolvedProvider  string
	ResolvedModel     string
	ModelSwitchLabel  string
	ModelSwitchAlias  string
	ModelIsDefault    bool
	ModelHasSelection bool
	ProfileOverride   string

	// 队列 ack 文本（调用方预先构建）
	QueueAckText string

	// xhigh 降级
	ShouldDowngradeXHigh bool

	// isDirectiveOnly 检查回调（用于决定是否应走 fast-lane）
	// TS 对照: isDirectiveOnly(directives, cleanedBody, ctx, cfg, agentId, isGroup)
	IsDirectiveOnlyFn func(directives InlineDirectives, cleanedBody string, isGroup bool) bool
}

// ApplyInlineDirectivesFastLane 执行 fast-lane 指令处理。
// TS 对照: directive-handling.fast-lane.ts applyInlineDirectivesFastLane (L11-138)
//
// 逻辑：
//  1. 若命令未授权，或消息是纯指令消息（isDirectiveOnly），则跳过 fast-lane。
//  2. 否则调用 HandleDirectiveOnly 获取确认 ack。
//  3. 若 session 中有 provider/model 覆盖，更新返回的 provider/model。
func ApplyInlineDirectivesFastLane(params FastLaneParams) FastLaneResult {
	provider := params.Provider
	model := params.Model

	// 1. 未授权，或纯指令消息 → 不走 fast-lane
	if !params.CommandAuthorized {
		return FastLaneResult{Provider: provider, Model: model}
	}
	if params.IsDirectiveOnlyFn != nil {
		if params.IsDirectiveOnlyFn(params.Directives, params.Directives.Cleaned, params.IsGroup) {
			return FastLaneResult{Provider: provider, Model: model}
		}
	}

	// 2. 解析当前 level 状态（session > agentCfg）
	currentThinkLevel := params.CurrentThinkLevel
	if currentThinkLevel == "" && params.SessionEntry != nil {
		currentThinkLevel = autoreply.ThinkLevel(params.SessionEntry.ThinkingLevel)
	}

	currentVerboseLevel := params.CurrentVerboseLevel
	if currentVerboseLevel == "" && params.SessionEntry != nil {
		currentVerboseLevel = autoreply.VerboseLevel(params.SessionEntry.VerboseLevel)
	}

	currentReasoningLevel := params.CurrentReasoningLevel
	if currentReasoningLevel == "" {
		if params.SessionEntry != nil && params.SessionEntry.ReasoningLevel != "" {
			currentReasoningLevel = autoreply.ReasoningLevel(params.SessionEntry.ReasoningLevel)
		} else {
			currentReasoningLevel = autoreply.ReasoningOff
		}
	}

	currentElevatedLevel := params.CurrentElevatedLevel
	if currentElevatedLevel == "" && params.SessionEntry != nil {
		currentElevatedLevel = autoreply.ElevatedLevel(params.SessionEntry.ElevatedLevel)
	}

	// 3. 处理纯指令
	ack := HandleDirectiveOnly(HandleDirectiveOnlyParams{
		Directives:            params.Directives,
		SessionEntry:          params.SessionEntry,
		SessionKey:            params.SessionKey,
		ElevatedEnabled:       params.ElevatedEnabled,
		ElevatedAllowed:       params.ElevatedAllowed,
		ElevatedFailures:      params.ElevatedFailures,
		Provider:              provider,
		Model:                 model,
		DefaultProvider:       params.DefaultProvider,
		DefaultModel:          params.DefaultModel,
		CurrentThinkLevel:     currentThinkLevel,
		CurrentVerboseLevel:   currentVerboseLevel,
		CurrentReasoningLevel: currentReasoningLevel,
		CurrentElevatedLevel:  currentElevatedLevel,
		ResolvedProvider:      params.ResolvedProvider,
		ResolvedModel:         params.ResolvedModel,
		ExecDefaults:          params.ExecDefaults,
		ModelErrorText:        params.ModelErrorText,
		ModelSwitchLabel:      params.ModelSwitchLabel,
		ModelSwitchAlias:      params.ModelSwitchAlias,
		ModelIsDefault:        params.ModelIsDefault,
		ModelHasSelection:     params.ModelHasSelection,
		ProfileOverride:       params.ProfileOverride,
		QueueAckText:          params.QueueAckText,
		ShouldDowngradeXHigh:  params.ShouldDowngradeXHigh,
		RuntimeIsSandboxed:    params.RuntimeIsSandboxed,
	})

	// 4. 更新 provider/model（session 覆盖）
	if params.SessionEntry != nil {
		if params.SessionEntry.ProviderOverride != "" {
			provider = params.SessionEntry.ProviderOverride
		}
		if params.SessionEntry.ModelOverride != "" {
			model = params.SessionEntry.ModelOverride
		}
	}

	return FastLaneResult{
		DirectiveAck: ack,
		Provider:     provider,
		Model:        model,
	}
}
