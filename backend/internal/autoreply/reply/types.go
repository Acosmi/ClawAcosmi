// Package reply 回复核心子包。
// TS 对照: auto-reply/reply/ 目录
package reply

// ReplyDispatchKind 回复分发类型。
// TS 对照: reply-dispatcher.ts L8
type ReplyDispatchKind string

const (
	DispatchTool  ReplyDispatchKind = "tool"
	DispatchBlock ReplyDispatchKind = "block"
	DispatchFinal ReplyDispatchKind = "final"
)

// NormalizeReplySkipReason 回复跳过原因。
type NormalizeReplySkipReason string

const (
	SkipReasonEmpty     NormalizeReplySkipReason = "empty"
	SkipReasonHeartbeat NormalizeReplySkipReason = "heartbeat"
	SkipReasonDuplicate NormalizeReplySkipReason = "duplicate"
)

// FinalizeInboundContextOptions 入站上下文最终化选项。
// TS 对照: reply/inbound-context.ts L7-12
type FinalizeInboundContextOptions struct {
	ForceBodyForAgent      bool
	ForceBodyForCommands   bool
	ForceChatType          bool
	ForceConversationLabel bool
}

// ResponsePrefixContext 响应前缀上下文。
// TS 对照: reply/response-prefix-template.ts
type ResponsePrefixContext struct {
	Model         string // 短模型名（如 "gpt-5.2"）
	ModelFull     string // 完整模型 ID（如 "openai-codex/gpt-5.2"）
	Provider      string // Provider 名（如 "openai-codex"）
	ThinkingLevel string // 思考级别（如 "high", "low", "off"）
	IdentityName  string // Agent 身份名
	Date          string
	Time          string
	Weekday       string
	Timezone      string
}
