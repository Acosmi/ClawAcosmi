package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	pkgmedia "github.com/openacosmi/claw-acismi/pkg/media"
)

// Discord 媒体发送工具 — 继承自 src/discord/send.shared.ts sendDiscordMedia (L339-396)
// 以及 src/discord/send.emojis-stickers.ts (58L)
// 替换 @buape/carbon rest.post({ files }) 为原生 multipart/form-data

// discordMedia 下载后的媒体数据（Discord multipart 所需字段名）
type discordMedia struct {
	Data        []byte
	FileName    string
	ContentType string
}

// loadDiscordMedia 从 HTTP URL 下载媒体文件。
// 委托给共享 pkg/media.LoadWebMedia。
func loadDiscordMedia(mediaURL string) (*discordMedia, error) {
	m, err := pkgmedia.LoadWebMedia(mediaURL, 0) // 使用默认大小限制
	if err != nil {
		return nil, fmt.Errorf("load discord media: %w", err)
	}
	return &discordMedia{
		Data:        m.Buffer,
		FileName:    m.Filename,
		ContentType: m.ContentType,
	}, nil
}

// loadDiscordMediaWithLimit 带大小限制的媒体加载（用于 emoji/sticker 上传）
func loadDiscordMediaWithLimit(mediaURL string, maxBytes int) (*discordMedia, error) {
	m, err := pkgmedia.LoadWebMedia(mediaURL, int64(maxBytes))
	if err != nil {
		return nil, fmt.Errorf("load discord media: %w", err)
	}
	return &discordMedia{
		Data:        m.Buffer,
		FileName:    m.Filename,
		ContentType: m.ContentType,
	}, nil
}

// discordMultipartPOST 使用 multipart/form-data POST 到 Discord API
// 用于带文件附件的消息发送（继承 @buape/carbon rest.post({ files }) 行为）
//
// Discord 的 multipart 格式约定：
// - payload_json: JSON 字符串，包含消息正文
// - files[0]: 二进制文件数据
func discordMultipartPOST(ctx context.Context, apiPath, token string, payload interface{}, media *discordMedia) (json.RawMessage, error) {
	apiURL := discordAPIBase + apiPath

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// 写入 payload_json 部分
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("discord multipart: marshal payload: %w", err)
	}
	jsonPartHeader := textproto.MIMEHeader{}
	jsonPartHeader.Set("Content-Disposition", `form-data; name="payload_json"`)
	jsonPartHeader.Set("Content-Type", "application/json")
	jsonPart, err := w.CreatePart(jsonPartHeader)
	if err != nil {
		return nil, fmt.Errorf("discord multipart: create json part: %w", err)
	}
	if _, err := jsonPart.Write(jsonData); err != nil {
		return nil, fmt.Errorf("discord multipart: write json: %w", err)
	}

	// 写入文件部分
	filePartHeader := textproto.MIMEHeader{}
	filePartHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="files[0]"; filename="%s"`, media.FileName))
	if media.ContentType != "" {
		filePartHeader.Set("Content-Type", media.ContentType)
	}
	filePart, err := w.CreatePart(filePartHeader)
	if err != nil {
		return nil, fmt.Errorf("discord multipart: create file part: %w", err)
	}
	if _, err := filePart.Write(media.Data); err != nil {
		return nil, fmt.Errorf("discord multipart: write file: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("discord multipart: close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("discord multipart: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discord multipart: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("discord multipart: read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		text := string(respBody)
		detail := formatDiscordAPIErrorText(text)
		suffix := ""
		if detail != "" {
			suffix = ": " + detail
		}
		var retryAfter *float64
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter = parseRetryAfterSeconds(text, resp)
		}
		return nil, &DiscordAPIError{
			StatusCode: resp.StatusCode,
			RetryAfter: retryAfter,
			Message:    fmt.Sprintf("Discord API POST %s failed (%d)%s", apiPath, resp.StatusCode, suffix),
		}
	}

	return json.RawMessage(respBody), nil
}

// SendDiscordMediaChunked 发送带媒体的 Discord 消息，支持文本分段。
// 对齐 TS sendDiscordMedia (send.shared.ts L339-396):
// 1. 将 text 通过 ChunkDiscordTextWithMode 分块
// 2. 第一个 chunk 与媒体一起发送（multipart）
// 3. 剩余 chunk 作为纯文本消息逐个发送
func SendDiscordMediaChunked(ctx context.Context, token string, channelID string, text string, mediaURL string, replyTo string, maxLines int, embeds []interface{}, chunkMode ChunkMode) (*DiscordSendResult, error) {
	// 1. 加载媒体
	media, err := loadDiscordMedia(mediaURL)
	if err != nil {
		return nil, err
	}

	// 2. 分块文本
	chunks := ChunkDiscordTextWithMode(text, ChunkDiscordTextOpts{
		MaxChars: discordTextLimit,
		MaxLines: maxLines,
	}, chunkMode)
	if len(chunks) == 0 {
		chunks = []string{""}
	}

	// 3. 第一个 chunk + 媒体一起发送
	firstChunk := chunks[0]
	payload := map[string]interface{}{}
	if firstChunk != "" {
		payload["content"] = firstChunk
	}
	if replyTo != "" {
		payload["message_reference"] = map[string]interface{}{
			"message_id":         replyTo,
			"fail_if_not_exists": false,
		}
	}
	if len(embeds) > 0 {
		payload["embeds"] = embeds
	}

	resp, err := discordMultipartPOSTWithRetry(ctx, fmt.Sprintf("/channels/%s/messages", channelID), token, payload, media, "media-chunked")
	if err != nil {
		return nil, err
	}

	// 解析首条消息的 ID
	var firstMsg struct {
		ID        string `json:"id"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal(resp, &firstMsg); err != nil {
		return nil, fmt.Errorf("discord send media chunked: decode response: %w", err)
	}

	// 4. 剩余 chunk 作为纯文本发送
	for _, chunk := range chunks[1:] {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		textPayload := map[string]interface{}{
			"content": chunk,
		}
		_, err := discordPOSTWithRetry(ctx, fmt.Sprintf("/channels/%s/messages", channelID), token, textPayload, "media-chunked-text")
		if err != nil {
			// 后续 chunk 发送失败不中断，记录但继续
			continue
		}
	}

	return &DiscordSendResult{
		MessageID: firstMsg.ID,
		ChannelID: firstMsg.ChannelID,
	}, nil
}
