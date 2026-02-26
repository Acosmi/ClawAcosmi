package dingtalk

// handler.go — 钉钉消息处理
// 基于 dingtalk-stream-sdk-go 回调数据

import (
	"log/slog"

	dingtalkstream "github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
)

// ExtractTextFromCallback 从 SDK 回调数据中提取纯文本
func ExtractTextFromCallback(data *dingtalkstream.BotCallbackDataModel) string {
	if data == nil {
		return ""
	}
	return data.Text.Content
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
