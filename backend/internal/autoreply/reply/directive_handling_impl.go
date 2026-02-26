package reply

import (
	"fmt"
	"strings"

	"github.com/anthropic/open-acosmi/internal/autoreply"
)

// TS 对照: auto-reply/reply/directive-handling.impl.ts (504L)

// ExecDefaults exec 指令的默认值。
type ExecDefaults struct {
	Host     ExecHost
	Security ExecSecurity
	Ask      ExecAsk
	Node     string
}

// ResolveExecDefaults 解析 exec 指令的默认值（session > agent > global）。
// TS 对照: directive-handling.impl.ts resolveExecDefaults (L32-59)
func ResolveExecDefaults(sessionEntry *SessionEntry, agentExecHost, agentExecSecurity, agentExecAsk, agentExecNode string, globalExecHost, globalExecSecurity, globalExecAsk, globalExecNode string) ExecDefaults {
	host := ExecHost(firstNonEmpty(
		sessionOrEmpty(sessionEntry, func(e *SessionEntry) string { return e.ExecHost }),
		agentExecHost,
		globalExecHost,
		"sandbox",
	))
	security := ExecSecurity(firstNonEmpty(
		sessionOrEmpty(sessionEntry, func(e *SessionEntry) string { return e.ExecSecurity }),
		agentExecSecurity,
		globalExecSecurity,
		"deny",
	))
	ask := ExecAsk(firstNonEmpty(
		sessionOrEmpty(sessionEntry, func(e *SessionEntry) string { return e.ExecAsk }),
		agentExecAsk,
		globalExecAsk,
		"on-miss",
	))
	node := firstNonEmpty(
		sessionOrEmpty(sessionEntry, func(e *SessionEntry) string { return e.ExecNode }),
		agentExecNode,
		globalExecNode,
	)
	return ExecDefaults{Host: host, Security: security, Ask: ask, Node: node}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func sessionOrEmpty(entry *SessionEntry, fn func(*SessionEntry) string) string {
	if entry == nil {
		return ""
	}
	return fn(entry)
}

// HandleDirectiveOnlyParams handleDirectiveOnly 参数。
// TS 对照: directive-handling.impl.ts handleDirectiveOnly params (L61-89)
type HandleDirectiveOnlyParams struct {
	Directives            InlineDirectives
	SessionEntry          *SessionEntry
	SessionKey            string
	ElevatedEnabled       bool
	ElevatedAllowed       bool
	ElevatedFailures      []ElevatedGateFailure
	Provider              string
	Model                 string
	DefaultProvider       string
	DefaultModel          string
	CurrentThinkLevel     autoreply.ThinkLevel
	CurrentVerboseLevel   autoreply.VerboseLevel
	CurrentReasoningLevel autoreply.ReasoningLevel
	CurrentElevatedLevel  autoreply.ElevatedLevel
	ResolvedProvider      string // 模型指令解析后有效 provider
	ResolvedModel         string // 模型指令解析后有效 model

	// exec 默认值参数（来自 config/agent/session）
	ExecDefaults ExecDefaults

	// 模型相关
	ModelErrorText    string
	ModelSwitchLabel  string
	ModelSwitchAlias  string
	ModelIsDefault    bool
	ModelHasSelection bool
	ProfileOverride   string

	// 队列相关
	QueueAckText string

	// xhigh 降级
	ShouldDowngradeXHigh bool

	// runtime 沙箱状态
	RuntimeIsSandboxed bool
}

// HandleDirectiveOnly 处理纯指令消息，返回确认文本。
// TS 对照: directive-handling.impl.ts handleDirectiveOnly (L61-504)
func HandleDirectiveOnly(params HandleDirectiveOnlyParams) *autoreply.ReplyPayload {
	directives := params.Directives
	runtimeIsSandboxed := params.RuntimeIsSandboxed
	shouldHintDirectRuntime := directives.HasElevatedDirective && !runtimeIsSandboxed
	resolvedProvider := params.ResolvedProvider
	if resolvedProvider == "" {
		resolvedProvider = params.Provider
	}
	resolvedModel := params.ResolvedModel
	if resolvedModel == "" {
		resolvedModel = params.Model
	}

	// 模型指令错误直接返回
	if params.ModelErrorText != "" {
		return &autoreply.ReplyPayload{Text: params.ModelErrorText}
	}

	// /think 无有效级别
	if directives.HasThinkDirective && directives.ThinkLevel == "" {
		if directives.RawThinkLevel == "" {
			level := params.CurrentThinkLevel
			if level == "" {
				level = "off"
			}
			return &autoreply.ReplyPayload{
				Text: WithOptions(
					fmt.Sprintf("Current thinking level: %s.", level),
					FormatThinkingLevels(resolvedProvider, resolvedModel),
				),
			}
		}
		return &autoreply.ReplyPayload{
			Text: fmt.Sprintf("Unrecognized thinking level %q. Valid levels: %s.",
				directives.RawThinkLevel,
				FormatThinkingLevels(resolvedProvider, resolvedModel),
			),
		}
	}

	// /verbose 无有效级别
	if directives.HasVerboseDirective && directives.VerboseLevel == "" {
		if directives.RawVerboseLevel == "" {
			level := params.CurrentVerboseLevel
			if level == "" {
				level = "off"
			}
			return &autoreply.ReplyPayload{
				Text: WithOptions(fmt.Sprintf("Current verbose level: %s.", level), "on, full, off"),
			}
		}
		return &autoreply.ReplyPayload{
			Text: fmt.Sprintf("Unrecognized verbose level %q. Valid levels: off, on, full.", directives.RawVerboseLevel),
		}
	}

	// /reasoning 无有效级别
	if directives.HasReasoningDirective && directives.ReasoningLevel == "" {
		if directives.RawReasoningLevel == "" {
			level := params.CurrentReasoningLevel
			if level == "" {
				level = "off"
			}
			return &autoreply.ReplyPayload{
				Text: WithOptions(fmt.Sprintf("Current reasoning level: %s.", level), "on, off, stream"),
			}
		}
		return &autoreply.ReplyPayload{
			Text: fmt.Sprintf("Unrecognized reasoning level %q. Valid levels: on, off, stream.", directives.RawReasoningLevel),
		}
	}

	// /elevated 无有效级别
	if directives.HasElevatedDirective && directives.ElevatedLevel == "" {
		if directives.RawElevatedLevel == "" {
			if !params.ElevatedEnabled || !params.ElevatedAllowed {
				return &autoreply.ReplyPayload{
					Text: FormatElevatedUnavailableText(runtimeIsSandboxed, params.ElevatedFailures, params.SessionKey),
				}
			}
			level := params.CurrentElevatedLevel
			if level == "" {
				level = "off"
			}
			lines := []string{
				WithOptions(fmt.Sprintf("Current elevated level: %s.", level), "on, off, ask, full"),
			}
			if shouldHintDirectRuntime {
				lines = append(lines, FormatElevatedRuntimeHint())
			}
			return &autoreply.ReplyPayload{Text: strings.Join(lines, "\n")}
		}
		return &autoreply.ReplyPayload{
			Text: fmt.Sprintf("Unrecognized elevated level %q. Valid levels: off, on, ask, full.", directives.RawElevatedLevel),
		}
	}

	// /elevated 但不可用
	if directives.HasElevatedDirective && (!params.ElevatedEnabled || !params.ElevatedAllowed) {
		return &autoreply.ReplyPayload{
			Text: FormatElevatedUnavailableText(runtimeIsSandboxed, params.ElevatedFailures, params.SessionKey),
		}
	}

	// /exec 验证
	if directives.HasExecDirective {
		if directives.InvalidExecHost {
			return &autoreply.ReplyPayload{
				Text: fmt.Sprintf("Unrecognized exec host %q. Valid hosts: sandbox, gateway, node.", directives.RawExecHost),
			}
		}
		if directives.InvalidExecSecurity {
			return &autoreply.ReplyPayload{
				Text: fmt.Sprintf("Unrecognized exec security %q. Valid: deny, allowlist, full.", directives.RawExecSecurity),
			}
		}
		if directives.InvalidExecAsk {
			return &autoreply.ReplyPayload{
				Text: fmt.Sprintf("Unrecognized exec ask %q. Valid: off, on-miss, always.", directives.RawExecAsk),
			}
		}
		if directives.InvalidExecNode {
			return &autoreply.ReplyPayload{Text: "Exec node requires a value."}
		}
		if !directives.HasExecOptions {
			// 显示当前 exec 默认值
			execDefaults := params.ExecDefaults
			nodeLabel := "node=(unset)"
			if execDefaults.Node != "" {
				nodeLabel = fmt.Sprintf("node=%s", execDefaults.Node)
			}
			return &autoreply.ReplyPayload{
				Text: WithOptions(
					fmt.Sprintf("Current exec defaults: host=%s, security=%s, ask=%s, %s.",
						execDefaults.Host, execDefaults.Security, execDefaults.Ask, nodeLabel),
					"host=sandbox|gateway|node, security=deny|allowlist|full, ask=off|on-miss|always, node=<id>",
				),
			}
		}
	}

	// 队列指令 ack
	if params.QueueAckText != "" {
		return &autoreply.ReplyPayload{Text: params.QueueAckText}
	}

	// xhigh 不支持
	if directives.HasThinkDirective &&
		directives.ThinkLevel == autoreply.ThinkXHigh &&
		!params.ShouldDowngradeXHigh {
		// 若 provider 不支持 xhigh 则报错
		hint := FormatXHighModelHint()
		return &autoreply.ReplyPayload{
			Text: fmt.Sprintf("Thinking level \"xhigh\" is only supported for %s.", hint),
		}
	}

	// 组装确认消息
	var parts []string

	if directives.HasThinkDirective && directives.ThinkLevel != "" {
		if directives.ThinkLevel == autoreply.ThinkOff {
			parts = append(parts, "Thinking disabled.")
		} else {
			parts = append(parts, fmt.Sprintf("Thinking level set to %s.", directives.ThinkLevel))
		}
	}

	if directives.HasVerboseDirective && directives.VerboseLevel != "" {
		switch directives.VerboseLevel {
		case autoreply.VerboseOff:
			parts = append(parts, FormatDirectiveAck("Verbose logging disabled."))
		case autoreply.VerboseFull:
			parts = append(parts, FormatDirectiveAck("Verbose logging set to full."))
		default:
			parts = append(parts, FormatDirectiveAck("Verbose logging enabled."))
		}
	}

	if directives.HasReasoningDirective && directives.ReasoningLevel != "" {
		switch directives.ReasoningLevel {
		case autoreply.ReasoningOff:
			parts = append(parts, FormatDirectiveAck("Reasoning visibility disabled."))
		case autoreply.ReasoningStream:
			parts = append(parts, FormatDirectiveAck("Reasoning stream enabled (Telegram only)."))
		default:
			parts = append(parts, FormatDirectiveAck("Reasoning visibility enabled."))
		}
	}

	if directives.HasElevatedDirective && directives.ElevatedLevel != "" {
		switch directives.ElevatedLevel {
		case autoreply.ElevatedOff:
			parts = append(parts, FormatDirectiveAck("Elevated mode disabled."))
		case autoreply.ElevatedFull:
			parts = append(parts, FormatDirectiveAck("Elevated mode set to full (auto-approve)."))
		default:
			parts = append(parts, FormatDirectiveAck("Elevated mode set to ask (approvals may still apply)."))
		}
		if shouldHintDirectRuntime {
			parts = append(parts, FormatElevatedRuntimeHint())
		}
	}

	if directives.HasExecDirective && directives.HasExecOptions {
		var execParts []string
		if directives.ExecHost != "" {
			execParts = append(execParts, fmt.Sprintf("host=%s", directives.ExecHost))
		}
		if directives.ExecSecurity != "" {
			execParts = append(execParts, fmt.Sprintf("security=%s", directives.ExecSecurity))
		}
		if directives.ExecAsk != "" {
			execParts = append(execParts, fmt.Sprintf("ask=%s", directives.ExecAsk))
		}
		if directives.ExecNode != "" {
			execParts = append(execParts, fmt.Sprintf("node=%s", directives.ExecNode))
		}
		if len(execParts) > 0 {
			parts = append(parts, FormatDirectiveAck(fmt.Sprintf("Exec defaults set (%s).", strings.Join(execParts, ", "))))
		}
	}

	if params.ShouldDowngradeXHigh {
		parts = append(parts, fmt.Sprintf("Thinking level set to high (xhigh not supported for %s/%s).", resolvedProvider, resolvedModel))
	}

	if params.ModelHasSelection {
		label := fmt.Sprintf("%s/%s", resolvedProvider, resolvedModel)
		labelWithAlias := label
		if params.ModelSwitchAlias != "" {
			labelWithAlias = fmt.Sprintf("%s (%s)", params.ModelSwitchAlias, label)
		}
		if params.ModelIsDefault {
			parts = append(parts, fmt.Sprintf("Model reset to default (%s).", labelWithAlias))
		} else {
			parts = append(parts, fmt.Sprintf("Model set to %s.", labelWithAlias))
		}
		if params.ProfileOverride != "" {
			parts = append(parts, fmt.Sprintf("Auth profile set to %s.", params.ProfileOverride))
		}
	}

	if directives.HasQueueDirective {
		if directives.QueueMode != "" {
			parts = append(parts, FormatDirectiveAck(fmt.Sprintf("Queue mode set to %s.", directives.QueueMode)))
		} else if directives.QueueReset {
			parts = append(parts, FormatDirectiveAck("Queue mode reset to default."))
		}
		if directives.DebounceMs != nil {
			parts = append(parts, FormatDirectiveAck(fmt.Sprintf("Queue debounce set to %dms.", *directives.DebounceMs)))
		}
		if directives.Cap != nil {
			parts = append(parts, FormatDirectiveAck(fmt.Sprintf("Queue cap set to %d.", *directives.Cap)))
		}
		if directives.DropPolicy != "" {
			parts = append(parts, FormatDirectiveAck(fmt.Sprintf("Queue drop set to %s.", directives.DropPolicy)))
		}
	}

	ack := strings.TrimSpace(strings.Join(parts, " "))
	if ack == "" && directives.HasStatusDirective {
		return nil
	}
	if ack == "" {
		ack = "OK."
	}
	return &autoreply.ReplyPayload{Text: ack}
}

// FormatThinkingLevels 格式化有效的 thinking 级别列表（占位，调用方根据实际 provider 填充）。
// TS 对照: thinking.ts formatThinkingLevels
func FormatThinkingLevels(provider, model string) string {
	// 默认级别列表；完整逻辑依赖 provider/model 能力检测
	_ = provider
	_ = model
	return "off, low, medium, high, xhigh"
}

// FormatXHighModelHint 格式化 xhigh 支持的模型提示。
// TS 对照: thinking.ts formatXHighModelHint
func FormatXHighModelHint() string {
	return "claude-3-7-sonnet (and later Sonnet/Opus variants)"
}
