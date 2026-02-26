package line

// TS 对照: @line/bot-sdk types + src/line/flex-templates.ts
// LINE Messaging API SDK 类型定义

// ---------- Flex Message 类型 ----------

// FlexMessage LINE Flex Message。
type FlexMessage struct {
	Type     string        `json:"type"` // "flex"
	AltText  string        `json:"altText"`
	Contents FlexContainer `json:"contents"`
}

// FlexContainer 容器接口。
type FlexContainer struct {
	Type     string       `json:"type"` // "bubble" | "carousel"
	Header   *FlexBox     `json:"header,omitempty"`
	Body     *FlexBox     `json:"body,omitempty"`
	Footer   *FlexBox     `json:"footer,omitempty"`
	Contents []FlexBubble `json:"contents,omitempty"` // carousel only
}

// FlexBubble Flex bubble。
type FlexBubble struct {
	Type   string   `json:"type"` // "bubble"
	Header *FlexBox `json:"header,omitempty"`
	Body   *FlexBox `json:"body,omitempty"`
	Footer *FlexBox `json:"footer,omitempty"`
	Size   string   `json:"size,omitempty"` // "nano"|"micro"|"kilo"|"mega"|"giga"
}

// FlexBox 布局容器。
type FlexBox struct {
	Type     string          `json:"type"`   // "box"
	Layout   string          `json:"layout"` // "horizontal"|"vertical"|"baseline"
	Contents []FlexComponent `json:"contents"`
	Margin   string          `json:"margin,omitempty"`
	Spacing  string          `json:"spacing,omitempty"`
	Padding  string          `json:"paddingAll,omitempty"`
}

// FlexComponent 通用组件。
// type 可为 "text"|"button"|"image"|"separator"|"box"（嵌套布局）。
type FlexComponent struct {
	Type string `json:"type"`
	// text/button/image 字段
	Text   string      `json:"text,omitempty"`
	Size   string      `json:"size,omitempty"`
	Weight string      `json:"weight,omitempty"`
	Color  string      `json:"color,omitempty"`
	Wrap   bool        `json:"wrap,omitempty"`
	Flex   int         `json:"flex,omitempty"`
	Action *FlexAction `json:"action,omitempty"`
	Margin string      `json:"margin,omitempty"`
	// box 嵌套布局字段（type="box" 时有效）
	Layout   string          `json:"layout,omitempty"`
	Contents []FlexComponent `json:"contents,omitempty"`
	Spacing  string          `json:"spacing,omitempty"`
}

// FlexAction 动作。
type FlexAction struct {
	Type  string `json:"type"` // "uri"|"message"|"postback"
	Label string `json:"label,omitempty"`
	URI   string `json:"uri,omitempty"`
	Text  string `json:"text,omitempty"`
	Data  string `json:"data,omitempty"`
}

// ---------- Webhook 类型 ----------

// WebhookEvent LINE webhook 事件。
type WebhookEvent struct {
	Type       string         `json:"type"` // "message"|"follow"|"unfollow"|"postback"
	ReplyToken string         `json:"replyToken"`
	Source     EventSource    `json:"source"`
	Message    *EventMessage  `json:"message,omitempty"`
	Postback   *EventPostback `json:"postback,omitempty"`
	Timestamp  int64          `json:"timestamp"`
}

// EventSource 消息来源。
type EventSource struct {
	Type    string `json:"type"` // "user"|"group"|"room"
	UserID  string `json:"userId"`
	GroupID string `json:"groupId,omitempty"`
	RoomID  string `json:"roomId,omitempty"`
}

// EventMessage 消息内容。
type EventMessage struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "text"|"image"|"video"|"audio"|"sticker"|"location"
	Text     string `json:"text,omitempty"`
	FileName string `json:"fileName,omitempty"`
	FileSize int    `json:"fileSize,omitempty"`
}

// EventPostback postback 数据。
type EventPostback struct {
	Data string `json:"data"`
}

// WebhookBody webhook 请求体。
type WebhookBody struct {
	Destination string         `json:"destination"`
	Events      []WebhookEvent `json:"events"`
}

// ---------- Reply 类型 ----------

// TextMessage 文本回复。
type TextMessage struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// ReplyMessageRequest 回复请求。
type ReplyMessageRequest struct {
	ReplyToken string        `json:"replyToken"`
	Messages   []interface{} `json:"messages"`
}

// PushMessageRequest 推送请求。
type PushMessageRequest struct {
	To       string        `json:"to"`
	Messages []interface{} `json:"messages"`
}

// ---------- 构建辅助 ----------

// NewTextMessage 创建文本消息。
func NewTextMessage(text string) TextMessage {
	return TextMessage{Type: "text", Text: text}
}

// NewFlexBubble 创建 Flex bubble。
func NewFlexBubble() FlexBubble {
	return FlexBubble{Type: "bubble"}
}

// NewFlexBox 创建布局容器。
func NewFlexBox(layout string, contents ...FlexComponent) FlexBox {
	return FlexBox{
		Type:     "box",
		Layout:   layout,
		Contents: contents,
	}
}

// NewFlexText 创建文本组件。
func NewFlexText(text, size, weight, color string) FlexComponent {
	return FlexComponent{
		Type:   "text",
		Text:   text,
		Size:   size,
		Weight: weight,
		Color:  color,
		Wrap:   true,
	}
}

// NewFlexButton 创建按钮组件。
func NewFlexButton(label, uri string) FlexComponent {
	return FlexComponent{
		Type: "button",
		Action: &FlexAction{
			Type:  "uri",
			Label: label,
			URI:   uri,
		},
	}
}

// NewFlexSeparator 创建分隔线。
func NewFlexSeparator() FlexComponent {
	return FlexComponent{Type: "separator", Margin: "md"}
}

// ToFlexMessage 将 bubble 包装为 FlexMessage。
func ToFlexMessage(altText string, bubble FlexBubble) FlexMessage {
	return FlexMessage{
		Type:    "flex",
		AltText: altText,
		Contents: FlexContainer{
			Type:   "bubble",
			Header: bubble.Header,
			Body:   bubble.Body,
			Footer: bubble.Footer,
		},
	}
}
