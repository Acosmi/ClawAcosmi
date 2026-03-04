package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/channels/ratelimit"
)

// Telegram 媒体组发送 — 对应 TS src/telegram/send-media-group.ts
// 使用 sendMediaGroup API 批量发送多个照片/视频/文档/音频

// InputMediaType 输入媒体类型
type InputMediaType string

const (
	InputMediaPhoto    InputMediaType = "photo"
	InputMediaVideo    InputMediaType = "video"
	InputMediaDocument InputMediaType = "document"
	InputMediaAudio    InputMediaType = "audio"
)

// InputMedia 媒体组中的单个媒体项
type InputMedia struct {
	Type      InputMediaType `json:"type"`
	Media     string         `json:"media"`                // file_id 或 URL
	Caption   string         `json:"caption,omitempty"`    // 仅第一项的 caption 显示
	ParseMode string         `json:"parse_mode,omitempty"` // "HTML" | "MarkdownV2"
}

// SendMediaGroupOpts 媒体组发送选项
type SendMediaGroupOpts struct {
	Token            string
	AccountID        string
	MessageThreadID  *int
	Silent           bool
	ReplyToMessageID *int
}

// SendMediaGroupResult 媒体组发送结果
type SendMediaGroupResult struct {
	MessageIDs []string
	ChatID     string
}

// sendMediaGroupResponse 解析 sendMediaGroup API 响应
type sendMediaGroupResponse struct {
	OK     bool          `json:"ok"`
	Result []interface{} `json:"result,omitempty"`
	Desc   string        `json:"description,omitempty"`
}

// SendMediaGroup 批量发送媒体组（照片/视频/文档/音频）。
// 对应 TS sendMediaGroup()。最多 10 个媒体项。
func SendMediaGroup(ctx context.Context, client *http.Client, chatID string, media []InputMedia, opts SendMediaGroupOpts) (*SendMediaGroupResult, error) {
	if len(media) == 0 {
		return nil, fmt.Errorf("media group must contain at least one item")
	}
	if len(media) > 10 {
		return nil, fmt.Errorf("media group cannot exceed 10 items (got %d)", len(media))
	}

	// 速率限制
	if err := ratelimit.GlobalTelegramLimiter().Wait(ctx); err != nil {
		return nil, fmt.Errorf("telegram rate limit wait: %w", err)
	}

	mediaJSON, err := json.Marshal(media)
	if err != nil {
		return nil, fmt.Errorf("marshal media: %w", err)
	}

	params := map[string]interface{}{
		"chat_id": chatID,
		"media":   json.RawMessage(mediaJSON),
	}
	if opts.MessageThreadID != nil {
		params["message_thread_id"] = *opts.MessageThreadID
	}
	if opts.Silent {
		params["disable_notification"] = true
	}
	if opts.ReplyToMessageID != nil {
		params["reply_to_message_id"] = *opts.ReplyToMessageID
	}

	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMediaGroup", TelegramAPIBaseURL, opts.Token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)

	var result sendMediaGroupResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("telegram sendMediaGroup decode: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("%d: %s", resp.StatusCode, result.Desc)
	}

	// 提取返回的消息 ID 列表
	var msgIDs []string
	resolvedChat := chatID
	for _, raw := range result.Result {
		if m, ok := raw.(map[string]interface{}); ok {
			if mid, exists := m["message_id"]; exists {
				switch v := mid.(type) {
				case float64:
					msgIDs = append(msgIDs, strconv.Itoa(int(v)))
				case json.Number:
					msgIDs = append(msgIDs, v.String())
				}
			}
			if chat, exists := m["chat"]; exists {
				if chatMap, ok := chat.(map[string]interface{}); ok {
					if cid, exists := chatMap["id"]; exists {
						if v, ok := cid.(float64); ok {
							resolvedChat = strconv.FormatInt(int64(v), 10)
						}
					}
				}
			}
		}
	}

	slog.Debug("telegram sendMediaGroup success",
		"chatId", chatID,
		"count", len(media),
		"messageIds", strings.Join(msgIDs, ","),
	)

	return &SendMediaGroupResult{
		MessageIDs: msgIDs,
		ChatID:     resolvedChat,
	}, nil
}
