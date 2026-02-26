package line

// TS 对照: src/line/send.ts (638L) — 审计补全: 15+ send 函数 + profile cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ---------- Profile Cache (TS: userProfileCache) ----------

// UserProfile 用户资料。
type UserProfile struct {
	DisplayName string `json:"displayName"`
	PictureURL  string `json:"pictureUrl,omitempty"`
	FetchedAt   time.Time
}

var (
	profileCache    = make(map[string]*UserProfile)
	profileCacheMu  sync.RWMutex
	profileCacheTTL = 5 * time.Minute
)

// GetUserProfile 获取用户资料(带缓存)。
func (c *Client) GetUserProfile(ctx context.Context, userID string) (*UserProfile, error) {
	profileCacheMu.RLock()
	cached, ok := profileCache[userID]
	profileCacheMu.RUnlock()
	if ok && time.Since(cached.FetchedAt) < profileCacheTTL {
		return cached, nil
	}

	url := c.BaseURL + "/profile/" + userID
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.ChannelToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("LINE profile API error %d: %s", resp.StatusCode, string(body))
	}

	var profile UserProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	profile.FetchedAt = time.Now()

	profileCacheMu.Lock()
	profileCache[userID] = &profile
	profileCacheMu.Unlock()

	return &profile, nil
}

// ---------- 扩展发送函数 ----------

// PushText 推送文本(便捷)。
func (c *Client) PushText(ctx context.Context, to, text string) (*LineSendResult, error) {
	if len(text) > 5000 {
		text = text[:4997] + "…"
	}
	err := c.PushMessage(ctx, to, []interface{}{NewTextMessage(text)})
	if err != nil {
		return nil, err
	}
	return &LineSendResult{ChatID: to}, nil
}

// PushImage 推送图片消息。
func (c *Client) PushImage(ctx context.Context, to, originalURL, previewURL string) (*LineSendResult, error) {
	if previewURL == "" {
		previewURL = originalURL
	}
	msg := map[string]string{
		"type":               "image",
		"originalContentUrl": originalURL,
		"previewImageUrl":    previewURL,
	}
	err := c.PushMessage(ctx, to, []interface{}{msg})
	if err != nil {
		return nil, err
	}
	return &LineSendResult{ChatID: to}, nil
}

// PushLocation 推送位置消息。
func (c *Client) PushLocation(ctx context.Context, to string, loc LocationData) (*LineSendResult, error) {
	msg := map[string]interface{}{
		"type":      "location",
		"title":     loc.Title,
		"address":   loc.Address,
		"latitude":  loc.Latitude,
		"longitude": loc.Longitude,
	}
	err := c.PushMessage(ctx, to, []interface{}{msg})
	if err != nil {
		return nil, err
	}
	return &LineSendResult{ChatID: to}, nil
}

// PushFlex 推送 Flex 消息。
func (c *Client) PushFlex(ctx context.Context, to string, fm FlexMessage) (*LineSendResult, error) {
	err := c.PushMessage(ctx, to, []interface{}{fm})
	if err != nil {
		return nil, err
	}
	return &LineSendResult{ChatID: to}, nil
}

// PushTemplate 推送模板消息。
func (c *Client) PushTemplate(ctx context.Context, to string, tmpl LineTemplateMessagePayload) (*LineSendResult, error) {
	altText := tmpl.AltText
	if altText == "" {
		altText = tmpl.Text
	}
	msg := map[string]interface{}{
		"type":     "template",
		"altText":  altText,
		"template": tmpl,
	}
	err := c.PushMessage(ctx, to, []interface{}{msg})
	if err != nil {
		return nil, err
	}
	return &LineSendResult{ChatID: to}, nil
}

// PushTextWithQuickReplies 推送带快速回复按钮的文本。
func (c *Client) PushTextWithQuickReplies(ctx context.Context, to, text string, labels []string) (*LineSendResult, error) {
	items := make([]map[string]interface{}, len(labels))
	for i, label := range labels {
		items[i] = map[string]interface{}{
			"type": "action",
			"action": map[string]string{
				"type":  "message",
				"label": label,
				"text":  label,
			},
		}
	}
	msg := map[string]interface{}{
		"type": "text",
		"text": text,
		"quickReply": map[string]interface{}{
			"items": items,
		},
	}
	err := c.PushMessage(ctx, to, []interface{}{msg})
	if err != nil {
		return nil, err
	}
	return &LineSendResult{ChatID: to}, nil
}

// ShowLoadingAnimation 显示加载动画(最长 20 秒)。
func (c *Client) ShowLoadingAnimation(ctx context.Context, chatID string, seconds int) error {
	if seconds <= 0 {
		seconds = 20
	}
	body := map[string]interface{}{
		"chatId":         chatID,
		"loadingSeconds": seconds,
	}
	return c.post(ctx, "/chat/loading/start", body)
}

// ReplyMessages 用消息数组回复。
func (c *Client) ReplyMessages(ctx context.Context, replyToken string, messages []interface{}) error {
	// LINE 限制每次最多 5 条消息
	if len(messages) > 5 {
		messages = messages[:5]
	}
	return c.ReplyMessage(ctx, replyToken, messages)
}
