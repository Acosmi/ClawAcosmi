package reply

import (
	"github.com/anthropic/open-acosmi/internal/autoreply"
)

// TS 对照: auto-reply/reply/get-reply-directives.ts (489L)

// ---------- 结果类型 ----------

// ReplyDirectiveContinuation 指令解析继续结果。
type ReplyDirectiveContinuation struct {
	CommandSource          string
	Command                CommandContext
	AllowTextCommands      bool
	Directives             InlineDirectives
	CleanedBody            string
	ElevatedEnabled        bool
	ElevatedAllowed        bool
	ElevatedFailures       []ElevatedFailure
	DefaultActivation      GroupActivation
	ResolvedThinkLevel     autoreply.ThinkLevel
	ResolvedVerboseLevel   autoreply.VerboseLevel
	ResolvedReasoningLevel autoreply.ReasoningLevel
	ResolvedElevatedLevel  autoreply.ElevatedLevel
	ExecOverrides          *ExecOverrides
	BlockStreamingEnabled  bool
	BlockReplyChunking     *BlockReplyChunking
	BlockStreamingBreak    string // "text_end" | "message_end"
	Provider               string
	Model                  string
	ContextTokens          int
	InlineStatusRequested  bool
	DirectiveAck           *autoreply.ReplyPayload
	PerMessageQueueMode    QueueMode
	PerMessageQueueOptions *QueueOptions
}

// ExecOverrides exec 覆盖配置。
type ExecOverrides struct {
	Host     ExecHost
	Security ExecSecurity
	Ask      ExecAsk
	Node     string
}

// ElevatedFailure 提权失败记录。
type ElevatedFailure struct {
	Gate string
	Key  string
}

// QueueOptions 队列选项。
type QueueOptions struct {
	DebounceMs *int
	Cap        *int
	DropPolicy QueueDropPolicy
}

// GroupActivation 群组激活模式。
type GroupActivation string

// CommandContext 命令上下文。
// TS 对照: commands-context.ts buildCommandContext
type CommandContext struct {
	IsAuthorizedSender    bool
	SenderIsOwner         bool
	CommandBodyNormalized string
	RawBodyNormalized     string
	Surface               string
	Channel               string // TS: channel（provider 或 surface 的小写规范化）
	ChannelID             string
	AbortKey              string
	OwnerList             []string
	From                  string
	To                    string
	SenderID              string
}

// ReplyDirectiveResult 指令解析结果。
type ReplyDirectiveResult struct {
	Kind         string // "reply" | "continue"
	Reply        []autoreply.ReplyPayload
	Continuation *ReplyDirectiveContinuation
}

// ResolveDirectivesParams 指令解析参数。
type ResolveDirectivesParams struct {
	Ctx                   *autoreply.MsgContext
	AgentID               string
	AgentDir              string
	WorkspaceDir          string
	SessionEntry          *SessionEntry
	SessionKey            string
	StorePath             string
	IsGroup               bool
	TriggerBodyNormalized string
	CommandAuthorized     bool
	DefaultProvider       string
	DefaultModel          string
	Provider              string
	Model                 string
	Typing                *TypingController
	Opts                  *autoreply.GetReplyOptions
	SkillFilter           []string
}

// ResolveReplyDirectives 解析回复指令。
// TS 对照: get-reply-directives.ts resolveReplyDirectives (L87-488)
// 编排：命令上下文 → 指令解析 → mention 剥离 → elevated 检查 → 模型解析 → 流式配置。
func ResolveReplyDirectives(params ResolveDirectivesParams) (*ReplyDirectiveResult, error) {
	ctx := params.Ctx
	if ctx == nil {
		return &ReplyDirectiveResult{Kind: "reply"}, nil
	}

	// 1. 解析命令源
	commandSource := resolveCommandSource(ctx)
	promptSource := resolvePromptSource(ctx)
	commandText := commandSource
	if commandText == "" {
		commandText = promptSource
	}

	// 2. 构建命令上下文
	command := CommandContext{
		IsAuthorizedSender:    params.CommandAuthorized,
		CommandBodyNormalized: commandText,
		RawBodyNormalized:     commandText,
	}

	// 3. 解析内联指令
	directives := ParseInlineDirectives(commandText, nil)

	// 4. 群组指令过滤
	if params.IsGroup && ctx.WasMentioned != "true" {
		if directives.HasElevatedDirective && directives.ElevatedLevel != autoreply.ElevatedOff {
			directives.HasElevatedDirective = false
			directives.ElevatedLevel = ""
			directives.RawElevatedLevel = ""
		}
		if directives.HasExecDirective && directives.ExecSecurity != ExecSecurityDeny {
			directives.HasExecDirective = false
			directives.ExecHost = ""
			directives.ExecSecurity = ""
			directives.ExecAsk = ""
			directives.ExecNode = ""
		}
	}

	// 5. 未授权发送者指令过滤
	if !params.CommandAuthorized {
		directives.HasThinkDirective = false
		directives.HasVerboseDirective = false
		directives.HasReasoningDirective = false
		directives.HasStatusDirective = false
		directives.HasModelDirective = false
		directives.HasQueueDirective = false
		directives.QueueReset = false
	}

	// 6. 清理消息体
	cleanedBody := directives.Cleaned

	// 7. 解析指令级别
	resolvedThinkLevel := directives.ThinkLevel
	if resolvedThinkLevel == "" && params.SessionEntry != nil {
		resolvedThinkLevel = autoreply.ThinkLevel(params.SessionEntry.ThinkingLevel)
	}

	resolvedVerboseLevel := directives.VerboseLevel
	if resolvedVerboseLevel == "" && params.SessionEntry != nil {
		resolvedVerboseLevel = autoreply.VerboseLevel(params.SessionEntry.VerboseLevel)
	}

	resolvedReasoningLevel := directives.ReasoningLevel
	if resolvedReasoningLevel == "" {
		if params.SessionEntry != nil && params.SessionEntry.ReasoningLevel != "" {
			resolvedReasoningLevel = autoreply.ReasoningLevel(params.SessionEntry.ReasoningLevel)
		} else {
			resolvedReasoningLevel = autoreply.ReasoningOff
		}
	}

	resolvedElevatedLevel := directives.ElevatedLevel
	if resolvedElevatedLevel == "" {
		if params.SessionEntry != nil && params.SessionEntry.ElevatedLevel != "" {
			resolvedElevatedLevel = autoreply.ElevatedLevel(params.SessionEntry.ElevatedLevel)
		} else {
			resolvedElevatedLevel = autoreply.ElevatedOn
		}
	}

	// 8. 块流式配置
	blockStreamingEnabled := false
	blockStreamingBreak := "text_end"

	// 9. 解析 exec overrides
	var execOverrides *ExecOverrides
	if directives.HasExecDirective || hasSessionExecOverrides(params.SessionEntry) {
		execOverrides = resolveExecOverrides(directives, params.SessionEntry)
	}

	return &ReplyDirectiveResult{
		Kind: "continue",
		Continuation: &ReplyDirectiveContinuation{
			CommandSource:          commandText,
			Command:                command,
			AllowTextCommands:      true,
			Directives:             directives,
			CleanedBody:            cleanedBody,
			ElevatedEnabled:        true,
			ElevatedAllowed:        true,
			DefaultActivation:      "require",
			ResolvedThinkLevel:     resolvedThinkLevel,
			ResolvedVerboseLevel:   resolvedVerboseLevel,
			ResolvedReasoningLevel: resolvedReasoningLevel,
			ResolvedElevatedLevel:  resolvedElevatedLevel,
			ExecOverrides:          execOverrides,
			BlockStreamingEnabled:  blockStreamingEnabled,
			BlockStreamingBreak:    blockStreamingBreak,
			Provider:               params.Provider,
			Model:                  params.Model,
		},
	}, nil
}

// ---------- 内部辅助 ----------

func resolveCommandSource(ctx *autoreply.MsgContext) string {
	if ctx.BodyForCommands != "" {
		return ctx.BodyForCommands
	}
	if ctx.CommandBody != "" {
		return ctx.CommandBody
	}
	if ctx.RawBody != "" {
		return ctx.RawBody
	}
	return ctx.Body
}

func resolvePromptSource(ctx *autoreply.MsgContext) string {
	if ctx.BodyForAgent != "" {
		return ctx.BodyForAgent
	}
	if ctx.BodyStripped != "" {
		return ctx.BodyStripped
	}
	return ctx.Body
}

func hasSessionExecOverrides(entry *SessionEntry) bool {
	if entry == nil {
		return false
	}
	return entry.ExecHost != "" || entry.ExecSecurity != "" || entry.ExecAsk != "" || entry.ExecNode != ""
}

func resolveExecOverrides(directives InlineDirectives, entry *SessionEntry) *ExecOverrides {
	host := directives.ExecHost
	if host == "" && entry != nil {
		host = ExecHost(entry.ExecHost)
	}
	security := directives.ExecSecurity
	if security == "" && entry != nil {
		security = ExecSecurity(entry.ExecSecurity)
	}
	ask := directives.ExecAsk
	if ask == "" && entry != nil {
		ask = ExecAsk(entry.ExecAsk)
	}
	node := directives.ExecNode
	if node == "" && entry != nil {
		node = entry.ExecNode
	}
	if host == "" && security == "" && ask == "" && node == "" {
		return nil
	}
	return &ExecOverrides{Host: host, Security: security, Ask: ask, Node: node}
}
