package feishu

// webhook.go — 飞书 HTTP 回调入口（SDK 模式）
// 使用 oapi-sdk-go/v3 SDK 的事件分发和验签能力

import (
	"context"
	"encoding/json"
	"log/slog"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// MessageHandler 消息回调函数
type MessageHandler func(*FeishuMessageEvent)

// CardActionHandler 卡片回传交互回调函数
// 返回 toast 或 nil；error 非 nil 时 SDK 返回错误响应。
type CardActionHandler func(ctx context.Context, action *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error)

// NewEventDispatcher 创建飞书事件分发器（SDK 模式）
// SDK 自动处理：验签、解密、URL 验证 challenge、事件去重
func NewEventDispatcher(verifyToken, encryptKey string, onMessage MessageHandler) *dispatcher.EventDispatcher {
	handler := dispatcher.NewEventDispatcher(verifyToken, encryptKey)

	// 注册 im.message.receive_v1 处理器
	handler.OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		if event == nil || event.Event == nil || event.Event.Message == nil {
			slog.Warn("feishu: received empty message event")
			return nil
		}

		msg := convertSDKMessage(event)
		slog.Info("feishu message received",
			"message_id", safeString(event.Event.Message.MessageId),
			"chat_id", safeString(event.Event.Message.ChatId),
			"chat_type", safeString(event.Event.Message.ChatType),
			"msg_type", safeString(event.Event.Message.MessageType),
		)

		if onMessage != nil {
			onMessage(msg)
		}
		return nil
	})

	// 注册 im.message.message_read_v1 空处理器 — 抑制 SDK 未注册事件的日志噪声
	handler.OnP2MessageReadV1(func(ctx context.Context, event *larkim.P2MessageReadV1) error {
		return nil
	})

	return handler
}

// RegisterCardActionHandler 在事件分发器上注册卡片回传交互（card.action.trigger）处理器。
// 通过 WebSocket 长连接接收回调，无需公网地址。
func RegisterCardActionHandler(d *dispatcher.EventDispatcher, handler CardActionHandler) {
	if handler == nil {
		return
	}
	d.OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
		return handler(ctx, event)
	})
}

// convertSDKMessage 将 SDK 事件转换为内部消息结构
func convertSDKMessage(event *larkim.P2MessageReceiveV1) *FeishuMessageEvent {
	msg := &FeishuMessageEvent{
		Message: &FeishuMessageInfo{},
	}

	e := event.Event
	if e.Message != nil {
		msg.Message.MessageID = safeString(e.Message.MessageId)
		msg.Message.RootID = safeString(e.Message.RootId)
		msg.Message.ParentID = safeString(e.Message.ParentId)
		msg.Message.CreateTime = safeString(e.Message.CreateTime)
		msg.Message.ChatID = safeString(e.Message.ChatId)
		msg.Message.ChatType = safeString(e.Message.ChatType)
		msg.Message.MessageType = safeString(e.Message.MessageType)

		// Content 是 JSON 字符串
		if e.Message.Content != nil {
			msg.Message.Content = *e.Message.Content
		}
	}

	if e.Sender != nil {
		msg.Sender = &FeishuSenderInfo{
			SenderType: safeString(e.Sender.SenderType),
			TenantKey:  safeString(e.Sender.TenantKey),
		}
		if e.Sender.SenderId != nil {
			msg.Sender.SenderID = &FeishuSenderID{
				UnionID: safeString(e.Sender.SenderId.UnionId),
				UserID:  safeString(e.Sender.SenderId.UserId),
				OpenID:  safeString(e.Sender.SenderId.OpenId),
			}
		}
	}

	return msg
}

// SDKLogLevel 获取 SDK 日志级别
func SDKLogLevel() larkcore.LogLevel {
	return larkcore.LogLevelInfo
}

// safeString 安全解引用字符串指针
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// EventToJSON 序列化事件为 JSON（调试用）
func EventToJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
