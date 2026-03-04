package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

// Telegram 回复投递 — 继承自 src/telegram/bot/delivery.ts (563L)
// 核心功能：将 AI 生成的回复投递到 Telegram 聊天

// ReplyPayload 回复载荷
type ReplyPayload struct {
	Text     string
	TextMode string // "markdown" or "html"
	Media    []ReplyMediaItem
	Sticker  string
	Voice    *VoicePayload
}

// ReplyMediaItem 回复媒体项
type ReplyMediaItem struct {
	URL         string
	ContentType string
	Caption     string
}

// VoicePayload 语音回复载荷
type VoicePayload struct {
	URL         string
	ContentType string
	Duration    int
}

// DeliverRepliesParams 投递回复参数
type DeliverRepliesParams struct {
	Client             *http.Client
	Token              string
	ChatID             string
	Replies            []ReplyPayload
	ReplyToMode        string // "off", "first", "all"
	ReplyToMessageID   int
	TextLimit          int
	Thread             *TelegramThreadSpec
	DisableLinkPreview bool // 对齐 TS linkPreview: TS 默认 true(启用)，Go 使用反转语义 DisableLinkPreview 默认 false(启用)
	ReplyQuoteText     string
	ReplyMarkup        [][]InlineButton
	ChunkMode          autoreply.ChunkMode // "length"(默认) 或 "newline"（段落预分割）
	TableMode          MarkdownTableMode   // 表格渲染模式，穿透至所有文本渲染调用
}

// shouldReplyTo 判断是否应附加 reply-to 参数。
// 对齐 TS: replyToMode==="off" → never; "all" → always; "first"(默认) → only first.
func shouldReplyTo(mode string, replyToID int, hasReplied bool) bool {
	if replyToID <= 0 {
		return false
	}
	switch mode {
	case "off":
		return false
	case "all":
		return true
	default: // "first" 或空
		return !hasReplied
	}
}

// applyReplyTo 向 sendParams 添加 reply-to 参数。
func applyReplyTo(sendParams map[string]interface{}, replyToID int, quoteText string) {
	rp := map[string]interface{}{"message_id": replyToID}
	if quoteText != "" {
		rp["quote"] = quoteText
	}
	sendParams["reply_parameters"] = rp
}

// isVoiceMessagesForbidden 检测 VOICE_MESSAGES_FORBIDDEN 错误。
// 对齐 TS delivery.ts isVoiceMessagesForbidden。
func isVoiceMessagesForbidden(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "VOICE_MESSAGES_FORBIDDEN")
}

// deliverTextChunks 投递文本分块消息（共享逻辑，供文本-only 和 voice fallback 使用）。
// 对齐 TS delivery.ts sendTelegramText: 非解析错误时 throw（快速失败）。
// 返回 (delivered, hasReplied, error)。
func deliverTextChunks(
	ctx context.Context, client *http.Client, params DeliverRepliesParams,
	chunks []string, plainText string,
	hasReplied bool, attachButtons bool,
) (bool, bool, error) {
	anyDelivered := false
	for i, chunk := range chunks {
		sendParams := map[string]interface{}{
			"chat_id":    params.ChatID,
			"text":       chunk,
			"parse_mode": "HTML",
		}
		applyThreadParams(sendParams, params.Thread)

		if shouldReplyTo(params.ReplyToMode, params.ReplyToMessageID, hasReplied) {
			applyReplyTo(sendParams, params.ReplyToMessageID, params.ReplyQuoteText)
		}

		if params.DisableLinkPreview {
			sendParams["link_preview_options"] = map[string]interface{}{"is_disabled": true}
		}

		// 按钮附加在首个 chunk（对齐 TS L116: i === 0）
		if attachButtons && i == 0 && len(params.ReplyMarkup) > 0 {
			if kb := buildInlineKeyboard(params.ReplyMarkup); kb != nil {
				sendParams["reply_markup"] = kb
			}
		}

		_, apiErr := callTelegramAPI(ctx, client, params.Token, "sendMessage", sendParams)
		if apiErr != nil {
			if isParseError(apiErr) {
				fallback := plainText
				if fallback == "" {
					fallback = chunk
				}
				sendParams["text"] = fallback
				delete(sendParams, "parse_mode")
				_, apiErr = callTelegramAPI(ctx, client, params.Token, "sendMessage", sendParams)
			}
			if apiErr != nil {
				// 对齐 TS: 非解析错误 throw（快速失败）
				return anyDelivered, hasReplied, fmt.Errorf("telegram delivery: sendMessage: %w", apiErr)
			}
		}
		anyDelivered = true
		if !hasReplied {
			hasReplied = true
		}
	}
	return anyDelivered, hasReplied, nil
}

// chunkText 对齐 TS delivery.ts chunkText 辅助函数。
// 根据 chunkMode 和 tableMode 将 markdown 分块为 HTML 片段。
func chunkText(md string, textLimit int, chunkMode autoreply.ChunkMode, tableMode MarkdownTableMode) []string {
	if md == "" {
		return nil
	}
	// 对齐 TS: chunkMode === "newline" → 先段落预分割再 markdown 分块
	var markdownChunks []string
	if chunkMode == autoreply.ChunkModeNewline {
		markdownChunks = autoreply.ChunkMarkdownTextWithMode(md, textLimit, autoreply.ChunkModeNewline)
	} else {
		markdownChunks = []string{md}
	}
	var result []string
	for _, chunk := range markdownChunks {
		nested := MarkdownToTelegramChunks(chunk, textLimit, tableMode)
		if len(nested) == 0 && chunk != "" {
			result = append(result, MarkdownToTelegramHTML(chunk, tableMode))
		} else {
			for _, c := range nested {
				result = append(result, c.HTML)
			}
		}
	}
	return result
}

// DeliverReplies 投递回复到 Telegram 聊天。
// 对齐 TS delivery.ts deliverReplies (L34-292)。
func DeliverReplies(ctx context.Context, params DeliverRepliesParams) (delivered bool, err error) {
	if len(params.Replies) == 0 {
		return false, nil
	}

	client := params.Client
	if client == nil {
		client = http.DefaultClient
	}

	textLimit := params.TextLimit
	if textLimit <= 0 {
		textLimit = 4096
	}

	chunkMode := params.ChunkMode
	if chunkMode == "" {
		chunkMode = autoreply.ChunkModeLength
	}
	tableMode := params.TableMode

	hasReplied := false

	for _, reply := range params.Replies {
		// 贴纸回复
		if reply.Sticker != "" {
			sendParams := map[string]interface{}{"chat_id": params.ChatID, "sticker": reply.Sticker}
			applyThreadParams(sendParams, params.Thread)
			if _, sErr := callTelegramAPI(ctx, client, params.Token, "sendSticker", sendParams); sErr != nil {
				slog.Warn("telegram delivery: sendSticker failed", "err", sErr)
			} else {
				delivered = true
			}
			continue
		}

		hasMedia := len(reply.Media) > 0 || (reply.Voice != nil && reply.Voice.URL != "")

		// 无媒体 — 文本-only 投递路径 (TS L108-132)
		if !hasMedia {
			if reply.Text == "" {
				continue
			}
			chunks := chunkText(reply.Text, textLimit, chunkMode, tableMode)
			if len(chunks) == 0 {
				continue
			}
			chunkDelivered, newHasReplied, chunkErr := deliverTextChunks(
				ctx, client, params, chunks, reply.Text, hasReplied, true,
			)
			if chunkDelivered {
				delivered = true
			}
			hasReplied = newHasReplied
			if chunkErr != nil {
				return delivered, chunkErr
			}
			continue
		}

		// Voice 消息路径 (TS L194-253)
		if reply.Voice != nil && reply.Voice.URL != "" {
			voiceData, voiceCT, voiceFN, dlErr := downloadMediaURL(ctx, client, reply.Voice.URL)
			if dlErr != nil {
				slog.Warn("telegram delivery: voice download failed", "url", reply.Voice.URL, "err", dlErr)
			} else {
				// 尝试 sendVoice
				mpParams := map[string]string{"chat_id": params.ChatID}
				if tp := BuildTelegramThreadParams(params.Thread); len(tp) > 0 {
					for k, v := range tp {
						mpParams[k] = fmt.Sprintf("%d", v)
					}
				}
				if shouldReplyTo(params.ReplyToMode, params.ReplyToMessageID, hasReplied) {
					mpParams["reply_to_message_id"] = fmt.Sprintf("%d", params.ReplyToMessageID)
				}
				_, voiceErr := callTelegramAPIMultipart(ctx, client, params.Token, "sendVoice", mpParams, "voice", voiceData, voiceFN)
				if voiceErr != nil {
					if isVoiceMessagesForbidden(voiceErr) {
						// 回退到文本 (TS L217-241)
						slog.Info("telegram: sendVoice forbidden, falling back to text")
						if reply.Text != "" {
							chunks := chunkText(reply.Text, textLimit, chunkMode, tableMode)
							fbDelivered, fbReplied, fbErr := deliverTextChunks(
								ctx, client, params, chunks, reply.Text, hasReplied, true,
							)
							if fbDelivered {
								delivered = true
							}
							hasReplied = fbReplied
							if fbErr != nil {
								return delivered, fbErr
							}
						}
					} else {
						slog.Warn("telegram delivery: sendVoice failed", "err", voiceErr)
					}
				} else {
					delivered = true
					if !hasReplied {
						hasReplied = true
					}
				}
				_ = voiceCT
			}
			continue
		}

		// 媒体投递路径 (TS L134-288)
		isFirstMedia := true
		var pendingFollowUpText string

		for _, media := range reply.Media {
			if media.URL == "" {
				continue
			}

			mediaData, contentType, fileName, dlErr := downloadMediaURL(ctx, client, media.URL)
			if dlErr != nil {
				slog.Warn("telegram delivery: media download failed", "url", media.URL, "err", dlErr)
				continue
			}

			apiMethod, fileField := resolveMediaAPIMethodWithFileName(contentType, fileName)

			// Caption splitting (TS L150-158)
			var htmlCaption string
			if isFirstMedia && reply.Text != "" {
				split := SplitTelegramCaption(reply.Text)
				if split.HasCaption {
					htmlCaption = RenderTelegramHTMLText(split.Caption, reply.TextMode, tableMode)
				}
				if split.HasFollowUp {
					pendingFollowUpText = split.FollowUp
				}
			}

			mpParams := map[string]string{"chat_id": params.ChatID}
			if htmlCaption != "" {
				mpParams["caption"] = htmlCaption
				mpParams["parse_mode"] = "HTML"
			}
			if tp := BuildTelegramThreadParams(params.Thread); len(tp) > 0 {
				for k, v := range tp {
					mpParams[k] = fmt.Sprintf("%d", v)
				}
			}
			if shouldReplyTo(params.ReplyToMode, params.ReplyToMessageID, hasReplied) {
				mpParams["reply_to_message_id"] = fmt.Sprintf("%d", params.ReplyToMessageID)
			}

			// 按钮附加到媒体（对齐 TS L162: shouldAttachButtonsToMedia = isFirstMedia && replyMarkup && !followUpText）
			if isFirstMedia && len(params.ReplyMarkup) > 0 && pendingFollowUpText == "" {
				if kb := buildInlineKeyboard(params.ReplyMarkup); kb != nil {
					if kbJSON, err := json.Marshal(kb); err == nil {
						mpParams["reply_markup"] = string(kbJSON)
					}
				}
			}

			_, uploadErr := callTelegramAPIMultipart(ctx, client, params.Token, apiMethod, mpParams, fileField, mediaData, fileName)
			if uploadErr != nil {
				slog.Warn("telegram delivery: media upload failed", "method", apiMethod, "err", uploadErr)
				continue
			}
			delivered = true
			if !hasReplied {
				hasReplied = true
			}

			// 后续文本消息 (TS L267-287: caption 拆分后的跟随文本)
			if pendingFollowUpText != "" && isFirstMedia {
				chunks := chunkText(pendingFollowUpText, textLimit, chunkMode, tableMode)
				// 按钮附加在首个 follow-up chunk (TS L279)
				fbDelivered, fbReplied, fbErr := deliverTextChunks(
					ctx, client, params, chunks, pendingFollowUpText, hasReplied, true,
				)
				if fbDelivered {
					delivered = true
				}
				hasReplied = fbReplied
				if fbErr != nil {
					return delivered, fbErr
				}
				pendingFollowUpText = ""
			}

			isFirstMedia = false
		}
	}

	return delivered, nil
}

func isParseError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// DY-022: 对齐 TS delivery.ts 三种模式: "parse entities" | "can't parse" | "find end of the entity"
	return strings.Contains(msg, "parse entities") ||
		strings.Contains(msg, "can't parse") ||
		strings.Contains(msg, "find end of the entity")
}

func chunkMarkdownForTelegram(text string, limit int) []string {
	if len([]rune(text)) <= limit {
		return []string{text}
	}
	var chunks []string
	runes := []rune(text)
	for len(runes) > 0 {
		end := limit
		if end > len(runes) {
			end = len(runes)
		}
		// 尝试在段落边界断开
		if end < len(runes) {
			for i := end; i > end/2; i-- {
				if runes[i] == '\n' && i > 0 && runes[i-1] == '\n' {
					end = i + 1
					break
				}
			}
		}
		chunks = append(chunks, string(runes[:end]))
		runes = runes[end:]
	}
	return chunks
}

func applyThreadParams(params map[string]interface{}, thread *TelegramThreadSpec) {
	if thread == nil {
		return
	}
	tp := BuildTelegramThreadParams(thread)
	for k, v := range tp {
		params[k] = v
	}
}

// mapStr 从 interface{} map 中安全取字符串字段。
func mapStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ResolveMedia 提取消息中的媒体引用（轻量级，不下载数据）。
func ResolveMedia(msg *TelegramMessage, maxBytes int64, token string) *TelegramMediaRef {
	if msg == nil {
		return nil
	}

	// 贴纸
	// DY-023: 对齐 TS delivery.ts — 过滤 is_animated/is_video 贴纸（仅处理静态 webp 贴纸）
	if msg.Sticker != nil && !msg.Sticker.IsAnimated && !msg.Sticker.IsVideo {
		return &TelegramMediaRef{
			Path:        fmt.Sprintf("sticker:%s", msg.Sticker.FileID),
			ContentType: "image/webp",
			StickerMetadata: &StickerMetadata{
				FileID:       msg.Sticker.FileID,
				FileUniqueID: msg.Sticker.FileUniqueID,
				Emoji:        msg.Sticker.Emoji,
				SetName:      msg.Sticker.SetName,
			},
		}
	}

	// 图片（选最大尺寸的 PhotoSize）
	if len(msg.Photo) > 0 {
		best := msg.Photo[len(msg.Photo)-1]
		if m, ok := best.(map[string]interface{}); ok {
			return &TelegramMediaRef{
				Path:        mapStr(m, "file_id"),
				ContentType: "image/jpeg",
				FileName:    "photo.jpg",
			}
		}
	}

	// 文档
	if msg.Document != nil {
		if m, ok := msg.Document.(map[string]interface{}); ok {
			ct := "application/octet-stream"
			if mime := mapStr(m, "mime_type"); mime != "" {
				ct = mime
			}
			return &TelegramMediaRef{
				Path:        mapStr(m, "file_id"),
				ContentType: ct,
				FileName:    mapStr(m, "file_name"),
			}
		}
	}

	// 语音
	if msg.Voice != nil {
		if m, ok := msg.Voice.(map[string]interface{}); ok {
			return &TelegramMediaRef{
				Path:        mapStr(m, "file_id"),
				ContentType: "audio/ogg",
				FileName:    "voice.ogg",
			}
		}
	}

	// 音频
	if msg.Audio != nil {
		if m, ok := msg.Audio.(map[string]interface{}); ok {
			ct := "audio/mpeg"
			if mime := mapStr(m, "mime_type"); mime != "" {
				ct = mime
			}
			fn := "audio.mp3"
			if name := mapStr(m, "file_name"); name != "" {
				fn = name
			}
			return &TelegramMediaRef{
				Path:        mapStr(m, "file_id"),
				ContentType: ct,
				FileName:    fn,
			}
		}
	}

	// DY-023: 视频笔记 (TS delivery.ts L396-399)
	if msg.VideoNote != nil {
		if m, ok := msg.VideoNote.(map[string]interface{}); ok {
			return &TelegramMediaRef{
				Path:        mapStr(m, "file_id"),
				ContentType: "video/mp4",
				FileName:    "video_note.mp4",
			}
		}
	}

	return nil
}

// ResolveMediaFull 提取消息中的媒体并下载完整数据。
// 使用 GetTelegramFile + DownloadTelegramFile 获取实际文件内容。
func ResolveMediaFull(ctx context.Context, client *http.Client, msg *TelegramMessage, maxBytes int64, token string) *TelegramMediaRef {
	ref := ResolveMedia(msg, maxBytes, token)
	if ref == nil || ref.StickerMetadata != nil {
		return ref // 贴纸不需要下载数据
	}

	if client == nil {
		client = http.DefaultClient
	}

	fileID := ref.Path
	if fileID == "" {
		return ref
	}

	// 获取文件信息
	fileInfo, err := GetTelegramFile(ctx, client, token, fileID)
	if err != nil {
		slog.Warn("telegram: getFile failed", "fileID", fileID, "err", err)
		return ref
	}

	// 下载文件内容
	downloaded, err := DownloadTelegramFile(ctx, client, token, fileInfo, maxBytes)
	if err != nil {
		slog.Warn("telegram: download failed", "fileID", fileID, "err", err)
		return ref
	}

	ref.Data = downloaded.Data
	if downloaded.ContentType != "" {
		ref.ContentType = downloaded.ContentType
	}
	if downloaded.FileName != "" {
		ref.FileName = downloaded.FileName
	}

	return ref
}
