package signal

// Signal 事件处理类型定义 — 继承自 src/signal/monitor/event-handler.types.ts (118L)

// SignalEnvelope SSE receive 事件的信封结构
type SignalEnvelope struct {
	SourceNumber *string            `json:"sourceNumber,omitempty"`
	SourceUuid   *string            `json:"sourceUuid,omitempty"`
	SourceName   *string            `json:"sourceName,omitempty"`
	Timestamp    *int64             `json:"timestamp,omitempty"`
	DataMessage  *SignalDataMessage `json:"dataMessage,omitempty"`
	EditMessage  *struct {
		DataMessage *SignalDataMessage `json:"dataMessage,omitempty"`
	} `json:"editMessage,omitempty"`
	SyncMessage     interface{}        `json:"syncMessage,omitempty"`
	ReactionMessage *SignalReactionMsg `json:"reactionMessage,omitempty"`
}

// SignalDataMessage 数据消息
type SignalDataMessage struct {
	Timestamp   *int64             `json:"timestamp,omitempty"`
	Message     *string            `json:"message,omitempty"`
	Attachments []SignalAttachment `json:"attachments,omitempty"`
	GroupInfo   *struct {
		GroupID   *string `json:"groupId,omitempty"`
		GroupName *string `json:"groupName,omitempty"`
	} `json:"groupInfo,omitempty"`
	Quote *struct {
		Text *string `json:"text,omitempty"`
	} `json:"quote,omitempty"`
	Reaction *SignalReactionMsg `json:"reaction,omitempty"`
}

// SignalReactionMsg 反应消息
type SignalReactionMsg struct {
	Emoji               *string `json:"emoji,omitempty"`
	TargetAuthor        *string `json:"targetAuthor,omitempty"`
	TargetAuthorUuid    *string `json:"targetAuthorUuid,omitempty"`
	TargetSentTimestamp *int64  `json:"targetSentTimestamp,omitempty"`
	IsRemove            *bool   `json:"isRemove,omitempty"`
	GroupInfo           *struct {
		GroupID   *string `json:"groupId,omitempty"`
		GroupName *string `json:"groupName,omitempty"`
	} `json:"groupInfo,omitempty"`
}

// SignalAttachment 附件
type SignalAttachment struct {
	ID          *string `json:"id,omitempty"`
	ContentType *string `json:"contentType,omitempty"`
	Filename    *string `json:"filename,omitempty"`
	Size        *int64  `json:"size,omitempty"`
}

// SignalReceivePayload SSE receive 事件 payload
type SignalReceivePayload struct {
	Envelope  *SignalEnvelope `json:"envelope,omitempty"`
	Exception *struct {
		Message string `json:"message,omitempty"`
	} `json:"exception,omitempty"`
}

// ptrStr 辅助：安全解引用 *string
func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ptrInt64 辅助：安全解引用 *int64
func ptrInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// ptrBool 辅助：安全解引用 *bool
func ptrBool(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}
