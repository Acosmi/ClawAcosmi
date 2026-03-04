package reply

import (
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

// TS 对照: auto-reply/reply/get-reply-inline-actions.ts (385L)

// InlineActionResult 内联动作结果。
type InlineActionResult struct {
	Kind           string // "reply" | "continue"
	Reply          []autoreply.ReplyPayload
	Directives     InlineDirectives
	AbortedLastRun bool
}

// InlineActionParams 内联动作参数。
type InlineActionParams struct {
	Ctx                    *autoreply.MsgContext
	AgentID                string
	AgentDir               string
	SessionEntry           *SessionEntry
	PreviousSessionEntry   *SessionEntry
	SessionKey             string
	StorePath              string
	WorkspaceDir           string
	IsGroup                bool
	Opts                   *autoreply.GetReplyOptions
	Typing                 *TypingController
	AllowTextCommands      bool
	InlineStatusRequested  bool
	Command                CommandContext
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
	Provider               string
	Model                  string
	ContextTokens          int
	DirectiveAck           *autoreply.ReplyPayload
	AbortedLastRun         bool
	SkillFilter            []string
	SkillCommands          []autoreply.SkillCommandSpec

	// DI 回调：构建状态回复。
	BuildStatusReplyFn func() *autoreply.ReplyPayload

	// DI 回调：处理命令（主命令处理入口）。
	// 返回 (reply, shouldContinue)。
	HandleCommandsFn func(command CommandContext) (*autoreply.ReplyPayload, bool)

	// DI 回调：发送内联回复。
	SendInlineReplyFn func(reply *autoreply.ReplyPayload)
}

// HandleInlineActions 处理内联动作。
// TS 对照: get-reply-inline-actions.ts handleInlineActions (L57-384)
// 处理：技能命令、内联命令、状态查询、通用命令。
func HandleInlineActions(params InlineActionParams) (*InlineActionResult, error) {
	directives := params.Directives
	abortedLastRun := params.AbortedLastRun
	cleanedBody := params.CleanedBody

	// 1. 技能命令处理
	if params.AllowTextCommands && strings.HasPrefix(params.Command.CommandBodyNormalized, "/") {
		// 加载技能命令列表（传入或按需发现）。
		skillCommands := params.SkillCommands

		if len(skillCommands) > 0 {
			invocation := autoreply.ResolveSkillCommandInvocation(
				skillCommands, params.Command.CommandBodyNormalized,
			)
			if invocation != nil {
				if !params.Command.IsAuthorizedSender {
					// 未授权发送者 → 静默忽略。
					if params.Typing != nil {
						params.Typing.Cleanup()
					}
					return &InlineActionResult{Kind: "reply"}, nil
				}
				// 技能命令调用 → 改写消息体为技能提示。
				promptParts := []string{
					`Use the "` + invocation.Spec.Name + `" skill for this request.`,
				}
				if invocation.Args != "" {
					promptParts = append(promptParts, "User input:\n"+invocation.Args)
				}
				rewrittenBody := strings.Join(promptParts, "\n\n")
				params.Ctx.Body = rewrittenBody
				cleanedBody = rewrittenBody
			}
		}
	}

	// 辅助函数：发送内联回复。
	sendInlineReply := func(reply *autoreply.ReplyPayload) {
		if reply == nil {
			return
		}
		if params.SendInlineReplyFn != nil {
			params.SendInlineReplyFn(reply)
		}
	}

	// 2. 内联命令提取（/help, /commands, /whoami, /id）
	var inlineCommand *InlineSimpleCommandResult
	if params.AllowTextCommands && params.Command.IsAuthorizedSender {
		inlineCommand = ExtractInlineSimpleCommand(cleanedBody)
	}
	if inlineCommand != nil {
		cleanedBody = inlineCommand.Cleaned
	}

	// 3. 内联状态查询
	handleInlineStatus := !IsDirectiveOnly(directives, directives.Cleaned, params.IsGroup, nil) &&
		params.InlineStatusRequested
	if handleInlineStatus {
		if params.BuildStatusReplyFn != nil {
			statusReply := params.BuildStatusReplyFn()
			sendInlineReply(statusReply)
		}
		directives.HasStatusDirective = false
	}

	// 4. 内联命令处理
	if inlineCommand != nil && params.HandleCommandsFn != nil {
		inlineCtx := params.Command
		inlineCtx.RawBodyNormalized = inlineCommand.Command
		inlineCtx.CommandBodyNormalized = inlineCommand.Command
		reply, _ := params.HandleCommandsFn(inlineCtx)
		if reply != nil {
			if inlineCommand.Cleaned == "" {
				if params.Typing != nil {
					params.Typing.Cleanup()
				}
				return &InlineActionResult{
					Kind:  "reply",
					Reply: []autoreply.ReplyPayload{*reply},
				}, nil
			}
			sendInlineReply(reply)
		}
	}

	// 5. directive ack 发送
	if params.DirectiveAck != nil {
		sendInlineReply(params.DirectiveAck)
	}

	// 6. 空配置检测
	// skipWhenConfigEmpty 需要 channel dock 查询（延迟到集成阶段）。

	// 7. 通用命令处理
	if params.HandleCommandsFn != nil {
		reply, shouldContinue := params.HandleCommandsFn(params.Command)
		if !shouldContinue {
			if params.Typing != nil {
				params.Typing.Cleanup()
			}
			var replies []autoreply.ReplyPayload
			if reply != nil {
				replies = []autoreply.ReplyPayload{*reply}
			}
			return &InlineActionResult{
				Kind:  "reply",
				Reply: replies,
			}, nil
		}
	}

	return &InlineActionResult{
		Kind:           "continue",
		Directives:     directives,
		AbortedLastRun: abortedLastRun,
	}, nil
}
