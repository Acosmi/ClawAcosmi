package autoreply

import "context"

// TS 对照: auto-reply/types.ts

// BlockReplyContext 阻塞回复上下文。
// TS 对照: types.ts L4-7
type BlockReplyContext struct {
	Ctx       context.Context
	TimeoutMs int
}

// ModelSelectedContext 模型选定上下文。
// TS 对照: types.ts L10-14
type ModelSelectedContext struct {
	Provider   string
	Model      string
	ThinkLevel string
}

// ReplyPayload 回复载荷。
// TS 对照: types.ts L46-59
type ReplyPayload struct {
	Text           string         `json:"text,omitempty"`
	MediaURL       string         `json:"mediaUrl,omitempty"`
	MediaURLs      []string       `json:"mediaUrls,omitempty"`
	ReplyToID      string         `json:"replyToId,omitempty"`
	ReplyToTag     bool           `json:"replyToTag,omitempty"`
	ReplyToCurrent bool           `json:"replyToCurrent,omitempty"`
	AudioAsVoice   bool           `json:"audioAsVoice,omitempty"`
	IsError        bool           `json:"isError,omitempty"`
	ChannelData    map[string]any `json:"channelData,omitempty"`
}

// GetReplyOptions 获取回复选项。
// TS 对照: types.ts L16-44
type GetReplyOptions struct {
	RunID                 string
	Ctx                   context.Context
	OnAgentRunStart       func(runID string)
	OnReplyStart          func()
	OnTypingCleanup       func()
	IsHeartbeat           bool
	OnPartialReply        func(payload ReplyPayload)
	OnReasoningStream     func(payload ReplyPayload)
	OnBlockReply          func(payload ReplyPayload, ctx *BlockReplyContext)
	OnToolResult          func(payload ReplyPayload)
	OnModelSelected       func(ctx ModelSelectedContext)
	DisableBlockStreaming bool
	BlockReplyTimeoutMs   int
	SkillFilter           []string
	HasRepliedRef         *BoolRef
}

// BoolRef 可变布尔引用（用于跨回调跟踪状态）。
type BoolRef struct {
	Value bool
}
