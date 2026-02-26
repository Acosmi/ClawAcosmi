package autoreply

import (
	"fmt"
	"strings"
)

// TS 对照: auto-reply/templating.ts (193L)

// MsgContext 入站消息上下文。
// TS 对照: templating.ts MsgContext 类型
type MsgContext struct {
	Body              string
	RawBody           string
	CommandBody       string
	Transcript        string
	ThreadStarterBody string
	UntrustedContext  []string
	ChatType          string
	ConversationLabel string
	BodyForAgent      string
	BodyForCommands   string
	CommandAuthorized bool

	// 频道元信息
	ChannelType       string
	ChannelID         string
	Provider          string
	Surface           string
	From              string
	To                string
	SenderID          string
	SenderName        string
	SenderDisplayName string
	SenderUsername    string
	SenderE164        string
	IsGroup           bool
	IsColdStart       bool

	// 会话/路由元信息
	SessionKey              string
	AccountID               string
	OriginatingChannel      string
	OriginatingTo           string
	MessageThreadID         string
	MessageSid              string
	MessageSidFirst         string
	MessageSidLast          string
	CommandSource           string
	CommandTargetSessionKey string
	Timestamp               int64

	// 群组/会话元信息
	ThreadLabel  string
	GroupChannel string
	GroupSubject string

	// 媒体信息
	MediaType             string
	MediaURL              string
	MediaCount            int
	HasAttachments        bool
	SuppressedAttachments int

	// 扩展字段（Window 2 新增）
	BodyStripped      string // 剥离指令后的消息体
	WasMentioned      string // 是否被 @提及 ("true" | "")
	GroupSystemPrompt string // 群组系统提示

	// D-W1b 新增: group / agent 扩展字段
	GroupID            string                 // TS: groupId — 群组标识
	GroupSpace         string                 // TS: groupSpace — 群组空间
	SpawnedBy          string                 // TS: spawnedBy — 发起者标识
	ClientCapabilities map[string]interface{} // TS: clientCapabilities — 客户端能力
	Images             []MessageImage         // TS: parsed attachment images
}

// MessageImage 消息图片附件。
// 对应 TS: chat-attachments.ts → ParsedImage
type MessageImage struct {
	Type     string `json:"type"` // "image" | "url"
	Data     string `json:"data"` // base64 或 URL
	MimeType string `json:"mimeType,omitempty"`
}

// FinalizedMsgContext 最终化后的消息上下文（所有必要字段已填充）。
// TS 对照: templating.ts FinalizedMsgContext
type FinalizedMsgContext = MsgContext

// TemplateContext 模板变量上下文。
type TemplateContext struct {
	Model    string
	Provider string
	Date     string
	Time     string
	Weekday  string
	Timezone string
}

// FormatTemplateValue 格式化模板变量值。
// TS 对照: templating.ts L70-85
func FormatTemplateValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case int, int64, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ApplyTemplate 对字符串应用模板变量替换。
// 支持 {{var}} 语法。
// TS 对照: templating.ts L87-120
func ApplyTemplate(str string, ctx *TemplateContext) string {
	if str == "" || ctx == nil {
		return str
	}
	if !strings.Contains(str, "{{") {
		return str
	}

	replacements := map[string]string{
		"{{model}}":    ctx.Model,
		"{{provider}}": ctx.Provider,
		"{{date}}":     ctx.Date,
		"{{time}}":     ctx.Time,
		"{{weekday}}":  ctx.Weekday,
		"{{timezone}}": ctx.Timezone,
	}

	result := str
	for k, v := range replacements {
		if strings.Contains(result, k) {
			result = strings.ReplaceAll(result, k, v)
		}
	}
	return result
}
