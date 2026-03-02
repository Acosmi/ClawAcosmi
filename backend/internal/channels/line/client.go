package line

// LINE Messaging API 客户端 + Webhook 处理器

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/openacosmi/claw-acismi/internal/channels/ratelimit"
)

// ---------- 客户端 ----------

// Client LINE API 客户端。
type Client struct {
	ChannelToken  string
	ChannelSecret string
	HTTPClient    *http.Client
	BaseURL       string
	Limiter       *ratelimit.ChannelLimiter // W3-D1: 令牌桶限速
}

const defaultBaseURL = "https://api.line.me/v2/bot"

// NewClient 创建客户端。
func NewClient(token, secret string) *Client {
	return &Client{
		ChannelToken:  token,
		ChannelSecret: secret,
		HTTPClient:    &http.Client{Timeout: 30 * time.Second},
		BaseURL:       defaultBaseURL,
		Limiter:       ratelimit.DefaultLineLimiter(), // W3-D1: LINE 默认限速
	}
}

// ReplyMessage 回复消息。
func (c *Client) ReplyMessage(ctx context.Context, replyToken string, messages []interface{}) error {
	body := ReplyMessageRequest{
		ReplyToken: replyToken,
		Messages:   messages,
	}
	return c.post(ctx, "/message/reply", body)
}

// PushMessage 推送消息。
func (c *Client) PushMessage(ctx context.Context, to string, messages []interface{}) error {
	body := PushMessageRequest{
		To:       to,
		Messages: messages,
	}
	return c.post(ctx, "/message/push", body)
}

// ReplyText 回复文本(便捷)。
func (c *Client) ReplyText(ctx context.Context, replyToken, text string) error {
	// LINE 限制单条消息 5000 字
	if len(text) > 5000 {
		text = text[:4997] + "…"
	}
	return c.ReplyMessage(ctx, replyToken, []interface{}{
		NewTextMessage(text),
	})
}

// ReplyProcessed 回复处理后的消息(markdown → Flex)。
func (c *Client) ReplyProcessed(ctx context.Context, replyToken, markdownText string) error {
	processed := ProcessLineMessage(markdownText)
	messages := make([]interface{}, 0, 1+len(processed.FlexMessages))
	if processed.Text != "" {
		text := processed.Text
		if len(text) > 5000 {
			text = text[:4997] + "…"
		}
		messages = append(messages, NewTextMessage(text))
	}
	for _, fm := range processed.FlexMessages {
		if len(messages) >= 5 { // LINE 限制每次最多 5 条
			break
		}
		messages = append(messages, fm)
	}
	if len(messages) == 0 {
		return nil
	}
	return c.ReplyMessage(ctx, replyToken, messages)
}

func (c *Client) post(ctx context.Context, path string, body interface{}) error {
	// W3-D1: 速率限制
	if c.Limiter != nil {
		if err := c.Limiter.Wait(ctx); err != nil {
			return fmt.Errorf("line rate limit wait: %w", err)
		}
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.ChannelToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("LINE API error %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// ---------- Webhook 验证 ----------

// ValidateSignature 验证 webhook 签名。
func (c *Client) ValidateSignature(body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(c.ChannelSecret))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ParseWebhookBody 解析 webhook 请求体。
func ParseWebhookBody(body []byte) (*WebhookBody, error) {
	var wb WebhookBody
	if err := json.Unmarshal(body, &wb); err != nil {
		return nil, fmt.Errorf("invalid webhook body: %w", err)
	}
	return &wb, nil
}

// ---------- Webhook Handler ----------

// WebhookHandler 处理 LINE webhook 请求。
type WebhookHandler struct {
	Client     *Client
	OnMessage  func(ctx context.Context, event WebhookEvent) error
	OnFollow   func(ctx context.Context, event WebhookEvent) error
	OnUnfollow func(ctx context.Context, event WebhookEvent) error
	OnPostback func(ctx context.Context, event WebhookEvent) error
}

// NewWebhookHandler 创建 webhook 处理器。
func NewWebhookHandler(client *Client) *WebhookHandler {
	return &WebhookHandler{Client: client}
}

// ServeHTTP 处理 HTTP 请求。
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 验证签名
	signature := r.Header.Get("X-Line-Signature")
	if signature == "" || !h.Client.ValidateSignature(body, signature) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	wb, err := ParseWebhookBody(body)
	if err != nil {
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	for _, event := range wb.Events {
		switch event.Type {
		case "message":
			if h.OnMessage != nil {
				_ = h.OnMessage(ctx, event)
			}
		case "follow":
			if h.OnFollow != nil {
				_ = h.OnFollow(ctx, event)
			}
		case "unfollow":
			if h.OnUnfollow != nil {
				_ = h.OnUnfollow(ctx, event)
			}
		case "postback":
			if h.OnPostback != nil {
				_ = h.OnPostback(ctx, event)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// ---------- 辅助 ----------

// ResolveSourceID 解析消息来源 ID（用于回复/推送）。
func ResolveSourceID(source EventSource) string {
	if source.GroupID != "" {
		return source.GroupID
	}
	if source.RoomID != "" {
		return source.RoomID
	}
	return source.UserID
}

// IsGroupChat 检测是否为群聊。
func IsGroupChat(source EventSource) bool {
	return source.Type == "group" || source.Type == "room"
}

// ExtractTextCommand 提取文本命令（去除 @mention 前缀）。
func ExtractTextCommand(text, botName string) string {
	text = strings.TrimSpace(text)
	if botName != "" {
		prefix := "@" + botName + " "
		if strings.HasPrefix(text, prefix) {
			text = strings.TrimPrefix(text, prefix)
		}
	}
	return text
}
