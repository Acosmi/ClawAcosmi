package reply

import (
	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
	"github.com/Acosmi/ClawAcosmi/internal/infra"
)

// TS 对照: auto-reply/reply/directive-handling.parse.ts (216L)

// ExecHost exec 主机模式（别名 → infra.ExecHost）。
type ExecHost = infra.ExecHost

const (
	ExecHostSandbox = infra.ExecHostSandbox
	ExecHostGateway = infra.ExecHostGateway
	ExecHostNode    = infra.ExecHostNode
)

// ExecSecurity exec 安全模式（别名 → infra.ExecSecurity）。
type ExecSecurity = infra.ExecSecurity

const (
	ExecSecurityDeny      = infra.ExecSecurityDeny
	ExecSecurityAllowlist = infra.ExecSecurityAllowlist
	ExecSecurityFull      = infra.ExecSecurityFull
)

// ExecAsk exec 确认模式（别名 → infra.ExecAsk）。
type ExecAsk = infra.ExecAsk

const (
	ExecAskOff    = infra.ExecAskOff
	ExecAskOnMiss = infra.ExecAskOnMiss
	ExecAskAlways = infra.ExecAskAlways
)

// QueueMode 队列模式。
type QueueMode string

const (
	QueueModeSteer        QueueMode = "steer"
	QueueModeFollowup     QueueMode = "followup"
	QueueModeCollect      QueueMode = "collect"
	QueueModeSteerBacklog QueueMode = "steer+backlog"
	QueueModeInterrupt    QueueMode = "interrupt"
)

// QueueDropPolicy 队列丢弃策略。
type QueueDropPolicy string

const (
	QueueDropOld       QueueDropPolicy = "old"
	QueueDropNew       QueueDropPolicy = "new"
	QueueDropSummarize QueueDropPolicy = "summarize"
)

// InlineDirectives 内联指令解析结果。
// TS 对照: directive-handling.parse.ts L18-61
type InlineDirectives struct {
	Cleaned string

	HasThinkDirective bool
	ThinkLevel        autoreply.ThinkLevel
	RawThinkLevel     string
	ThinkLevelSet     bool

	HasVerboseDirective bool
	VerboseLevel        autoreply.VerboseLevel
	RawVerboseLevel     string
	VerboseLevelSet     bool

	HasReasoningDirective bool
	ReasoningLevel        autoreply.ReasoningLevel
	RawReasoningLevel     string
	ReasoningLevelSet     bool

	HasElevatedDirective bool
	ElevatedLevel        autoreply.ElevatedLevel
	RawElevatedLevel     string
	ElevatedLevelSet     bool

	HasExecDirective    bool
	ExecHost            ExecHost
	ExecSecurity        ExecSecurity
	ExecAsk             ExecAsk
	ExecNode            string
	RawExecHost         string
	RawExecSecurity     string
	RawExecAsk          string
	RawExecNode         string
	HasExecOptions      bool
	InvalidExecHost     bool
	InvalidExecSecurity bool
	InvalidExecAsk      bool
	InvalidExecNode     bool

	HasStatusDirective bool

	HasModelDirective bool
	RawModelDirective string
	RawModelProfile   string

	HasQueueDirective bool
	QueueMode         QueueMode
	QueueReset        bool
	RawQueueMode      string
	DebounceMs        *int
	Cap               *int
	DropPolicy        QueueDropPolicy
	RawDebounce       string
	RawCap            string
	RawDrop           string
	HasQueueOptions   bool
}

// ParseInlineDirectivesOptions 解析选项。
type ParseInlineDirectivesOptions struct {
	ModelAliases         []string
	DisableElevated      bool
	AllowStatusDirective *bool // nil 表示 true（默认允许）
}

// ParseInlineDirectives 从消息体解析所有内联指令。
// 按顺序链式提取 think → verbose → reasoning → elevated → exec → status → model → queue。
// TS 对照: directive-handling.parse.ts L63-189
func ParseInlineDirectives(body string, opts *ParseInlineDirectivesOptions) InlineDirectives {
	if opts == nil {
		opts = &ParseInlineDirectivesOptions{}
	}

	thinkResult := ExtractThinkDirective(body)
	verboseResult := ExtractVerboseDirective(thinkResult.Cleaned)
	reasoningResult := ExtractReasoningDirective(verboseResult.Cleaned)

	var elevatedResult ElevatedDirectiveResult
	if opts.DisableElevated {
		elevatedResult = ElevatedDirectiveResult{
			Cleaned:      reasoningResult.Cleaned,
			HasDirective: false,
		}
	} else {
		elevatedResult = ExtractElevatedDirective(reasoningResult.Cleaned)
	}

	execResult := ExtractExecDirective(elevatedResult.Cleaned)

	allowStatus := opts.AllowStatusDirective == nil || *opts.AllowStatusDirective
	var statusResult StatusDirectiveResult
	if allowStatus {
		statusResult = ExtractStatusDirective(execResult.Cleaned)
	} else {
		statusResult = StatusDirectiveResult{Cleaned: execResult.Cleaned, HasDirective: false}
	}

	modelResult := autoreply.ExtractModelDirective(statusResult.Cleaned, opts.ModelAliases)
	queueResult := ExtractQueueDirective(modelResult.Cleaned)

	return InlineDirectives{
		Cleaned: queueResult.Cleaned,

		HasThinkDirective: thinkResult.HasDirective,
		ThinkLevel:        thinkResult.ThinkLevel,
		RawThinkLevel:     thinkResult.RawLevel,
		ThinkLevelSet:     thinkResult.LevelSet,

		HasVerboseDirective: verboseResult.HasDirective,
		VerboseLevel:        verboseResult.VerboseLevel,
		RawVerboseLevel:     verboseResult.RawLevel,
		VerboseLevelSet:     verboseResult.LevelSet,

		HasReasoningDirective: reasoningResult.HasDirective,
		ReasoningLevel:        reasoningResult.ReasoningLevel,
		RawReasoningLevel:     reasoningResult.RawLevel,
		ReasoningLevelSet:     reasoningResult.LevelSet,

		HasElevatedDirective: elevatedResult.HasDirective,
		ElevatedLevel:        elevatedResult.ElevatedLevel,
		RawElevatedLevel:     elevatedResult.RawLevel,
		ElevatedLevelSet:     elevatedResult.LevelSet,

		HasExecDirective:    execResult.HasDirective,
		ExecHost:            execResult.ExecHost,
		ExecSecurity:        execResult.ExecSecurity,
		ExecAsk:             execResult.ExecAsk,
		ExecNode:            execResult.ExecNode,
		RawExecHost:         execResult.RawExecHost,
		RawExecSecurity:     execResult.RawExecSecurity,
		RawExecAsk:          execResult.RawExecAsk,
		RawExecNode:         execResult.RawExecNode,
		HasExecOptions:      execResult.HasExecOptions,
		InvalidExecHost:     execResult.InvalidExecHost,
		InvalidExecSecurity: execResult.InvalidExecSecurity,
		InvalidExecAsk:      execResult.InvalidExecAsk,
		InvalidExecNode:     execResult.InvalidExecNode,

		HasStatusDirective: statusResult.HasDirective,

		HasModelDirective: modelResult.HasDirective,
		RawModelDirective: modelResult.RawModel,
		RawModelProfile:   modelResult.RawProfile,

		HasQueueDirective: queueResult.HasDirective,
		QueueMode:         queueResult.QueueMode,
		QueueReset:        queueResult.QueueReset,
		RawQueueMode:      queueResult.RawQueueMode,
		DebounceMs:        queueResult.DebounceMs,
		Cap:               queueResult.Cap,
		DropPolicy:        queueResult.DropPolicy,
		RawDebounce:       queueResult.RawDebounce,
		RawCap:            queueResult.RawCap,
		RawDrop:           queueResult.RawDrop,
		HasQueueOptions:   queueResult.HasQueueOptions,
	}
}

// IsDirectiveOnly 判断消息是否仅包含指令（无实际内容）。
// TS 对照: directive-handling.parse.ts L192-215
func IsDirectiveOnly(directives InlineDirectives, cleanedBody string, isGroup bool, mentionStripper func(string) string) bool {
	if !directives.HasThinkDirective &&
		!directives.HasVerboseDirective &&
		!directives.HasReasoningDirective &&
		!directives.HasElevatedDirective &&
		!directives.HasExecDirective &&
		!directives.HasModelDirective &&
		!directives.HasQueueDirective {
		return false
	}
	stripped := StripStructuralPrefixes(cleanedBody)
	if isGroup && mentionStripper != nil {
		stripped = mentionStripper(stripped)
	}
	return len(stripped) == 0
}
