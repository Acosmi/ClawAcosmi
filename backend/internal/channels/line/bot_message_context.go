package line

// TS 对照: line/bot-message-context.ts (461L)
// LINE 消息上下文构建 — 骨架实现。
// Go 端暂无 LINE SDK 集成，此文件仅定义接口和类型，
// 以便在未来 LINE 集成阶段逐步实现。

// LineEventType LINE 事件类型。
type LineEventType string

const (
	LineEventMessage  LineEventType = "message"
	LineEventPostback LineEventType = "postback"
	LineEventFollow   LineEventType = "follow"
	LineEventUnfollow LineEventType = "unfollow"
	LineEventJoin     LineEventType = "join"
	LineEventLeave    LineEventType = "leave"
)

// LineSourceType LINE 消息来源类型。
type LineSourceType string

const (
	LineSourceUser  LineSourceType = "user"
	LineSourceGroup LineSourceType = "group"
	LineSourceRoom  LineSourceType = "room"
)

// LineInboundEvent LINE 入站事件（简化）。
// TS 对照: line/bot-message-context.ts LineWebhookEvent
type LineInboundEvent struct {
	Type       LineEventType  `json:"type"`
	SourceType LineSourceType `json:"sourceType"`
	SourceID   string         `json:"sourceId"`   // userId / groupId / roomId
	UserID     string         `json:"userId"`     // 发送者 userId
	ReplyToken string         `json:"replyToken"` // 回复 token
	Timestamp  int64          `json:"timestamp"`
	// 消息字段
	MessageType string `json:"messageType,omitempty"` // text / image / audio / video
	MessageText string `json:"messageText,omitempty"`
	// Postback 字段
	PostbackData string `json:"postbackData,omitempty"`
}

// LineBotMessageContext LINE 消息上下文。
// TS 对照: line/bot-message-context.ts buildLineMessageContext
type LineBotMessageContext struct {
	Body       string `json:"body"`
	From       string `json:"from"`     // userId
	To         string `json:"to"`       // LINE bot channel ID
	ChatType   string `json:"chatType"` // "direct" | "group"
	IsGroup    bool   `json:"isGroup"`
	GroupID    string `json:"groupId,omitempty"`
	SenderID   string `json:"senderId"`
	SenderName string `json:"senderName,omitempty"`
	Channel    string `json:"channel"` // "line"
	ReplyToken string `json:"replyToken"`
	// 线程回复（LINE 不原生支持线程，此字段为扩展预留）
	ThreadID string `json:"threadId,omitempty"`
}

// BuildBotMessageContext 构建 LINE 消息上下文。
// TODO(LINE): 完整实现需要 LINE SDK（line-bot-sdk-go）集成。
// 当前为骨架实现，仅从 LineInboundEvent 提取基本字段。
func BuildBotMessageContext(event *LineInboundEvent, botChannelID string) *LineBotMessageContext {
	if event == nil {
		return nil
	}

	chatType := "direct"
	isGroup := false
	groupID := ""

	if event.SourceType == LineSourceGroup || event.SourceType == LineSourceRoom {
		chatType = "group"
		isGroup = true
		groupID = event.SourceID
	}

	return &LineBotMessageContext{
		Body:       event.MessageText,
		From:       event.UserID,
		To:         botChannelID,
		ChatType:   chatType,
		IsGroup:    isGroup,
		GroupID:    groupID,
		SenderID:   event.UserID,
		Channel:    "line",
		ReplyToken: event.ReplyToken,
	}
}
