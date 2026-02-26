package wecom

// sender.go — 企业微信消息发送
// 直接 HTTP API: POST /cgi-bin/message/send

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// WeComSender 企业微信消息发送器
type WeComSender struct {
	client  *WeComClient
	agentID int
}

// NewWeComSender 创建消息发送器
func NewWeComSender(client *WeComClient, agentID int) *WeComSender {
	return &WeComSender{
		client:  client,
		agentID: agentID,
	}
}

// SendText 发送文本消息
func (s *WeComSender) SendText(ctx context.Context, toUser, text string) error {
	msg := map[string]interface{}{
		"touser":  toUser,
		"msgtype": "text",
		"agentid": s.agentID,
		"text": map[string]string{
			"content": text,
		},
	}
	return s.sendMessage(ctx, msg)
}

// SendMarkdown 发送 Markdown 消息
func (s *WeComSender) SendMarkdown(ctx context.Context, toUser, content string) error {
	msg := map[string]interface{}{
		"touser":  toUser,
		"msgtype": "markdown",
		"agentid": s.agentID,
		"markdown": map[string]string{
			"content": content,
		},
	}
	return s.sendMessage(ctx, msg)
}

// SendCard 发送卡片消息
func (s *WeComSender) SendCard(ctx context.Context, toUser, title, description, url string) error {
	msg := map[string]interface{}{
		"touser":  toUser,
		"msgtype": "textcard",
		"agentid": s.agentID,
		"textcard": map[string]string{
			"title":       title,
			"description": description,
			"url":         url,
		},
	}
	return s.sendMessage(ctx, msg)
}

// sendMessage 底层消息发送
func (s *WeComSender) sendMessage(ctx context.Context, msg map[string]interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal wecom message: %w", err)
	}

	respBody, err := s.client.DoAPIRequest(ctx, "POST", "/cgi-bin/message/send", body)
	if err != nil {
		return fmt.Errorf("send wecom message: %w", err)
	}

	var resp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		MsgID   string `json:"msgid"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("decode send response: %w", err)
	}
	if resp.ErrCode != 0 {
		return fmt.Errorf("wecom send error: code=%d, msg=%s", resp.ErrCode, resp.ErrMsg)
	}

	slog.Debug("wecom message sent", "msg_id", resp.MsgID)
	return nil
}
