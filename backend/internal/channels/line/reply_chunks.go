package line

// TS 对照: src/line/reply-chunks.ts + src/line/rich-menu.ts + src/line/auto-reply-delivery.ts
// 审计补全: 分块回复、Rich Menu、自动回复投递

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ---------- Reply Chunks (src/line/reply-chunks.ts) ----------

// SplitReplyChunks 将长文本分割为 LINE 可发送的消息块。
// LINE 限制: 单条文本 5000 字, 单次回复 5 条消息。
func SplitReplyChunks(text string, maxChunkSize int) []string {
	if maxChunkSize <= 0 {
		maxChunkSize = 4900
	}
	if len(text) <= maxChunkSize {
		return []string{text}
	}

	chunks := make([]string, 0)
	remaining := text
	for len(remaining) > 0 {
		if len(remaining) <= maxChunkSize {
			chunks = append(chunks, remaining)
			break
		}
		// 在 maxChunkSize 附近找换行符切割
		cutAt := maxChunkSize
		lastNL := strings.LastIndex(remaining[:cutAt], "\n")
		if lastNL > maxChunkSize/2 {
			cutAt = lastNL + 1
		}
		chunks = append(chunks, remaining[:cutAt])
		remaining = remaining[cutAt:]
	}
	return chunks
}

// SendChunkedReply 分块回复长文本。
func (c *Client) SendChunkedReply(ctx context.Context, replyToken, to, text string) error {
	chunks := SplitReplyChunks(text, 4900)

	// 第一条用 reply，剩余用 push
	if len(chunks) == 0 {
		return nil
	}

	// Reply 最多 5 条
	replyMsgs := make([]interface{}, 0, 5)
	for i, chunk := range chunks {
		if i >= 5 {
			break
		}
		replyMsgs = append(replyMsgs, NewTextMessage(chunk))
	}

	if err := c.ReplyMessage(ctx, replyToken, replyMsgs); err != nil {
		return err
	}

	// 超过 5 条 → push 剩余
	if len(chunks) > 5 && to != "" {
		for i := 5; i < len(chunks); i++ {
			if err := c.PushMessage(ctx, to, []interface{}{NewTextMessage(chunks[i])}); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---------- Rich Menu (src/line/rich-menu.ts) ----------

// RichMenu LINE Rich Menu 定义。
type RichMenu struct {
	Size        RichMenuSize   `json:"size"`
	Selected    bool           `json:"selected"`
	Name        string         `json:"name"`
	ChatBarText string         `json:"chatBarText"`
	Areas       []RichMenuArea `json:"areas"`
}

// RichMenuSize 菜单尺寸。
type RichMenuSize struct {
	Width  int `json:"width"`  // 2500
	Height int `json:"height"` // 843 or 1686
}

// RichMenuArea 菜单区域。
type RichMenuArea struct {
	Bounds RichMenuBounds `json:"bounds"`
	Action FlexAction     `json:"action"`
}

// RichMenuBounds 区域边界。
type RichMenuBounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// CreateRichMenu 创建 Rich Menu。
func (c *Client) CreateRichMenu(ctx context.Context, menu RichMenu) (string, error) {
	data, err := json.Marshal(menu)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/richmenu", strings.NewReader(string(data)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.ChannelToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create rich menu failed %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		RichMenuID string `json:"richMenuId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.RichMenuID, nil
}

// SetDefaultRichMenu 设置默认 Rich Menu。
func (c *Client) SetDefaultRichMenu(ctx context.Context, richMenuID string) error {
	return c.post(ctx, "/user/all/richmenu/"+richMenuID, nil)
}

// LinkRichMenuToUser 绑定 Rich Menu 到用户。
func (c *Client) LinkRichMenuToUser(ctx context.Context, userID, richMenuID string) error {
	return c.post(ctx, "/user/"+userID+"/richmenu/"+richMenuID, nil)
}

// ---------- Auto-Reply Delivery (src/line/auto-reply-delivery.ts) ----------

// DeliverAutoReply 投递自动回复,支持 markdown 转 Flex + 分块。
func (c *Client) DeliverAutoReply(ctx context.Context, replyToken, to, text string) error {
	// 1. 检测是否含 markdown → 转换
	if HasMarkdownToConvert(text) {
		return c.ReplyProcessed(ctx, replyToken, text)
	}

	// 2. 长文本 → 分块
	if len(text) > 4900 {
		return c.SendChunkedReply(ctx, replyToken, to, text)
	}

	// 3. 短文本 → 直接回复
	return c.ReplyText(ctx, replyToken, text)
}
