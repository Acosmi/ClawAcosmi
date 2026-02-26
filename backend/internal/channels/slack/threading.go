package slack

// Slack 线程上下文 — 继承自 src/slack/threading.ts (45L)

// ReplyToMode 回复线程模式（与 types.ReplyToMode 对应）
type ReplyToMode string

const (
	ReplyToModeOff   ReplyToMode = "off"
	ReplyToModeFirst ReplyToMode = "first"
	ReplyToModeAll   ReplyToMode = "all"
)

// SlackThreadContext 解析后的 Slack 线程上下文
type SlackThreadContext struct {
	IncomingThreadTs string
	MessageTs        string
	IsThreadReply    bool
	ReplyToID        string
	MessageThreadID  string // 空字符串表示无 thread
}

// ResolveSlackThreadContext 解析消息的线程上下文。
// 决定消息是否为线程回复，以及回复应发往哪个 thread。
func ResolveSlackThreadContext(msg SlackMessageEventLike, replyToMode ReplyToMode) SlackThreadContext {
	incomingThreadTs := msg.GetThreadTs()
	eventTs := msg.GetEventTs()
	messageTs := msg.GetTs()
	if messageTs == "" {
		messageTs = eventTs
	}

	hasThreadTs := incomingThreadTs != ""
	isThreadReply := hasThreadTs &&
		(incomingThreadTs != messageTs || msg.GetParentUserID() != "")

	replyToID := incomingThreadTs
	if replyToID == "" {
		replyToID = messageTs
	}

	var messageThreadID string
	if isThreadReply {
		messageThreadID = incomingThreadTs
	} else if replyToMode == ReplyToModeAll {
		messageThreadID = messageTs
	}

	return SlackThreadContext{
		IncomingThreadTs: incomingThreadTs,
		MessageTs:        messageTs,
		IsThreadReply:    isThreadReply,
		ReplyToID:        replyToID,
		MessageThreadID:  messageThreadID,
	}
}

// ResolveSlackThreadTargets 解析线程投递目标的 thread_ts。
func ResolveSlackThreadTargets(msg SlackMessageEventLike, replyToMode ReplyToMode) (replyThreadTs, statusThreadTs string) {
	ctx := ResolveSlackThreadContext(msg, replyToMode)

	replyThreadTs = ctx.IncomingThreadTs
	if replyThreadTs == "" && replyToMode == ReplyToModeAll {
		replyThreadTs = ctx.MessageTs
	}

	statusThreadTs = replyThreadTs
	if statusThreadTs == "" {
		statusThreadTs = ctx.MessageTs
	}
	return
}

// SlackMessageEventLike 抽象接口，统一 SlackMessageEvent 和 SlackAppMentionEvent 的字段访问。
type SlackMessageEventLike interface {
	GetTs() string
	GetThreadTs() string
	GetEventTs() string
	GetParentUserID() string
}

// -- SlackMessageEvent 实现 SlackMessageEventLike --

// GetTs 返回消息时间戳。
func (m *SlackMessageEvent) GetTs() string { return m.Ts }

// GetThreadTs 返回线程时间戳。
func (m *SlackMessageEvent) GetThreadTs() string { return m.ThreadTs }

// GetEventTs 返回事件时间戳。
func (m *SlackMessageEvent) GetEventTs() string { return m.EventTs }

// GetParentUserID 返回父消息用户 ID。
func (m *SlackMessageEvent) GetParentUserID() string { return m.ParentUserID }

// -- SlackAppMentionEvent 实现 SlackMessageEventLike --

// GetTs 返回消息时间戳。
func (m *SlackAppMentionEvent) GetTs() string { return m.Ts }

// GetThreadTs 返回线程时间戳。
func (m *SlackAppMentionEvent) GetThreadTs() string { return m.ThreadTs }

// GetEventTs 返回事件时间戳。
func (m *SlackAppMentionEvent) GetEventTs() string { return m.EventTs }

// GetParentUserID 返回父消息用户 ID。
func (m *SlackAppMentionEvent) GetParentUserID() string { return m.ParentUserID }
