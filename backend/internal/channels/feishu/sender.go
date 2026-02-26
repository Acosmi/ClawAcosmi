package feishu

// sender.go — 飞书消息发送便捷封装
// 基于 client.go 的 SDK 客户端

import (
	"context"
	"log/slog"
)

// ReceiveID 类型常量（对应飞书 API receive_id_type）
const (
	ReceiveIDTypeOpenID  = "open_id"
	ReceiveIDTypeChatID  = "chat_id"
	ReceiveIDTypeUserID  = "user_id"
	ReceiveIDTypeUnionID = "union_id"
)

// FeishuSender 飞书消息发送器
type FeishuSender struct {
	client *FeishuClient
}

// NewFeishuSender 创建消息发送器
func NewFeishuSender(client *FeishuClient) *FeishuSender {
	return &FeishuSender{client: client}
}

// SendText 发送文本消息（便捷方法）
func (s *FeishuSender) SendText(ctx context.Context, receiveID, idType, text string) error {
	msgID, err := s.client.SendTextMessage(ctx, idType, receiveID, text)
	if err != nil {
		return err
	}
	slog.Debug("feishu text sent", "receive_id", receiveID, "message_id", msgID)
	return nil
}

// SendRichText 发送富文本消息
func (s *FeishuSender) SendRichText(ctx context.Context, receiveID, idType, content string) error {
	msgID, err := s.client.SendRichTextMessage(ctx, idType, receiveID, content)
	if err != nil {
		return err
	}
	slog.Debug("feishu rich text sent", "receive_id", receiveID, "message_id", msgID)
	return nil
}

// SendCard 发送互动卡片消息
func (s *FeishuSender) SendCard(ctx context.Context, receiveID, idType, cardJSON string) error {
	msgID, err := s.client.SendCardMessage(ctx, idType, receiveID, cardJSON)
	if err != nil {
		return err
	}
	slog.Debug("feishu card sent", "receive_id", receiveID, "message_id", msgID)
	return nil
}

// SendImage 发送图片消息
func (s *FeishuSender) SendImage(ctx context.Context, receiveID, idType, imageKey string) error {
	msgID, err := s.client.SendImageMessage(ctx, idType, receiveID, imageKey)
	if err != nil {
		return err
	}
	slog.Debug("feishu image sent", "receive_id", receiveID, "message_id", msgID, "image_key", imageKey)
	return nil
}

// SendAudio 发送语音消息（内联播放）
func (s *FeishuSender) SendAudio(ctx context.Context, receiveID, idType, fileKey string) error {
	msgID, err := s.client.SendAudioMessage(ctx, idType, receiveID, fileKey)
	if err != nil {
		return err
	}
	slog.Debug("feishu audio sent", "receive_id", receiveID, "message_id", msgID, "file_key", fileKey)
	return nil
}

// SendFile 发送文件消息
func (s *FeishuSender) SendFile(ctx context.Context, receiveID, idType, fileKey string) error {
	msgID, err := s.client.SendFileMessage(ctx, idType, receiveID, fileKey)
	if err != nil {
		return err
	}
	slog.Debug("feishu file sent", "receive_id", receiveID, "message_id", msgID, "file_key", fileKey)
	return nil
}
