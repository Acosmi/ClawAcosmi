package feishu

// client.go — 飞书 SDK 客户端封装
// 使用官方 oapi-sdk-go/v3 SDK

import (
	"context"
	"encoding/json"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// FeishuClient 飞书 SDK 客户端封装
type FeishuClient struct {
	SDK       *lark.Client
	AppID     string
	AppSecret string
	Domain    string // "feishu" 或 "lark"
}

// NewFeishuClient 创建飞书 SDK 客户端
func NewFeishuClient(acct *ResolvedFeishuAccount) *FeishuClient {
	opts := []lark.ClientOptionFunc{}

	// 国际版 Lark 域名
	if IsLarkDomain(acct.Config.Domain) {
		opts = append(opts, lark.WithOpenBaseUrl(lark.LarkBaseUrl))
	}

	client := lark.NewClient(acct.Config.AppID, acct.Config.AppSecret, opts...)

	return &FeishuClient{
		SDK:       client,
		AppID:     acct.Config.AppID,
		AppSecret: acct.Config.AppSecret,
		Domain:    acct.Config.Domain,
	}
}

// SendTextMessage 发送文本消息
func (c *FeishuClient) SendTextMessage(ctx context.Context, receiveIDType, receiveID, text string) (string, error) {
	// 使用 json.Marshal 正确转义特殊字符（引号/换行符/反斜杠等）
	escapedText, err := json.Marshal(text)
	if err != nil {
		return "", fmt.Errorf("feishu: failed to marshal text: %w", err)
	}
	content := fmt.Sprintf(`{"text":%s}`, string(escapedText))
	return c.sendMessage(ctx, receiveIDType, receiveID, "text", content)
}

// SendRichTextMessage 发送富文本消息
func (c *FeishuClient) SendRichTextMessage(ctx context.Context, receiveIDType, receiveID, postContent string) (string, error) {
	return c.sendMessage(ctx, receiveIDType, receiveID, "post", postContent)
}

// SendCardMessage 发送互动卡片消息
func (c *FeishuClient) SendCardMessage(ctx context.Context, receiveIDType, receiveID, cardJSON string) (string, error) {
	return c.sendMessage(ctx, receiveIDType, receiveID, "interactive", cardJSON)
}

// SendImageMessage 发送图片消息 (msg_type: "image")
func (c *FeishuClient) SendImageMessage(ctx context.Context, receiveIDType, receiveID, imageKey string) (string, error) {
	contentBytes, _ := json.Marshal(map[string]string{"image_key": imageKey})
	return c.sendMessage(ctx, receiveIDType, receiveID, "image", string(contentBytes))
}

// SendAudioMessage 发送语音消息 (msg_type: "audio", 飞书内联播放)
func (c *FeishuClient) SendAudioMessage(ctx context.Context, receiveIDType, receiveID, fileKey string) (string, error) {
	contentBytes, _ := json.Marshal(map[string]string{"file_key": fileKey})
	return c.sendMessage(ctx, receiveIDType, receiveID, "audio", string(contentBytes))
}

// SendFileMessage 发送文件消息 (msg_type: "file")
func (c *FeishuClient) SendFileMessage(ctx context.Context, receiveIDType, receiveID, fileKey string) (string, error) {
	contentBytes, _ := json.Marshal(map[string]string{"file_key": fileKey})
	return c.sendMessage(ctx, receiveIDType, receiveID, "file", string(contentBytes))
}

// sendMessage 底层消息发送
func (c *FeishuClient) sendMessage(ctx context.Context, receiveIDType, receiveID, msgType, content string) (string, error) {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveID).
			MsgType(msgType).
			Content(content).
			Build()).
		Build()

	resp, err := c.SDK.Im.Message.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("feishu send message: %w", err)
	}

	if !resp.Success() {
		return "", fmt.Errorf("feishu send message failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data != nil && resp.Data.MessageId != nil {
		return *resp.Data.MessageId, nil
	}
	return "", nil
}
