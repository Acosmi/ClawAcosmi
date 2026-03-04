package dingtalk

// handler.go — 钉钉消息处理
// 基于 dingtalk-stream-sdk-go 回调数据

import (
	"log/slog"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/channels"
	dingtalkstream "github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
)

// ExtractTextFromCallback 从 SDK 回调数据中提取纯文本
func ExtractTextFromCallback(data *dingtalkstream.BotCallbackDataModel) string {
	if data == nil {
		return ""
	}
	return data.Text.Content
}

// ExtractMultimodalMessageFromCallback 提取钉钉回调中的统一多模态消息结构。
func ExtractMultimodalMessageFromCallback(data *dingtalkstream.BotCallbackDataModel) *channels.ChannelMessage {
	if data == nil {
		return &channels.ChannelMessage{}
	}

	cm := &channels.ChannelMessage{
		Text:        strings.TrimSpace(ExtractTextFromCallback(data)),
		MessageID:   strings.TrimSpace(data.MsgId),
		MessageType: strings.ToLower(strings.TrimSpace(data.Msgtype)),
	}
	if cm.MessageType == "" {
		cm.MessageType = "text"
	}

	if att := extractAttachmentFromContent(cm.MessageType, data.Content); att != nil {
		cm.Attachments = append(cm.Attachments, *att)
	}

	if cm.Text == "" && len(cm.Attachments) > 0 {
		switch cm.Attachments[0].Category {
		case "image":
			cm.Text = "[钉钉图片消息]"
		case "audio":
			cm.Text = "[钉钉语音消息]"
		case "document":
			cm.Text = "[钉钉文件消息]"
		case "video":
			cm.Text = "[钉钉视频消息]"
		}
	}

	return cm
}

// IsSingleChat 判断是否为单聊
func IsSingleChat(data *dingtalkstream.BotCallbackDataModel) bool {
	return data != nil && data.ConversationType == "1"
}

// IsGroupChat 判断是否为群聊
func IsGroupChat(data *dingtalkstream.BotCallbackDataModel) bool {
	return data != nil && data.ConversationType == "2"
}

// LogCallbackInfo 记录回调信息
func LogCallbackInfo(data *dingtalkstream.BotCallbackDataModel) {
	if data == nil {
		return
	}
	slog.Info("dingtalk message received",
		"msg_id", data.MsgId,
		"sender_nick", data.SenderNick,
		"sender_id", data.SenderId,
		"conversation_type", data.ConversationType,
		"conversation_id", data.ConversationId,
	)
}

func extractAttachmentFromContent(msgType string, content interface{}) *channels.ChannelAttachment {
	m, ok := content.(map[string]interface{})
	if !ok {
		return nil
	}

	category := ""
	switch strings.ToLower(strings.TrimSpace(msgType)) {
	case "image", "picture":
		category = "image"
	case "audio", "voice":
		category = "audio"
	case "file":
		category = "document"
	case "video":
		category = "video"
	default:
		return nil
	}

	fileKey := pickFirstString(m, "downloadCode", "download_code", "mediaId", "media_id", "fileKey", "file_key", "url", "photoURL")
	fileName := pickFirstString(m, "fileName", "filename", "name")
	mimeType := pickFirstString(m, "mimeType", "contentType")

	return &channels.ChannelAttachment{
		Category: category,
		FileKey:  fileKey,
		FileName: fileName,
		MimeType: mimeType,
	}
}

func pickFirstString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if s, ok := v.(string); ok {
				trimmed := strings.TrimSpace(s)
				if trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}
