package line

// TS 对照: src/line/auto-reply-delivery.ts (182L)
// LINE 自动回复投递 — Flex Message、分块投递、快速回复按钮

import (
	"context"
	"fmt"
	"strings"
)

// ---------- ReplyPayload ----------

// ReplyPayload 回复载荷（对应 TS 端 ReplyPayload）。
type ReplyPayload struct {
	Text      string
	MediaURL  string
	MediaURLs []string
	// ChannelData LINE 特有扩展数据。
	ChannelData *LineChannelData
}

// ---------- AutoReplyDeps 依赖注入 ----------

// AutoReplyDeps deliverLineAutoReply 依赖接口。
// TS: LineAutoReplyDeps
type AutoReplyDeps struct {
	// ProcessLineMessage 将 markdown 转换为 LINE 消息结构。
	ProcessLineMessage func(text string) ProcessedLineMessage
	// ChunkMarkdownText 分割长文本为消息块。
	ChunkMarkdownText func(text string, limit int) []string
	// OnReplyError 可选: reply token 失败回调。
	OnReplyError func(err error)
}

// DefaultAutoReplyDeps 使用包内默认实现构建 deps。
func DefaultAutoReplyDeps() AutoReplyDeps {
	return AutoReplyDeps{
		ProcessLineMessage: ProcessLineMessage,
		ChunkMarkdownText:  defaultChunkMarkdownText,
	}
}

// defaultChunkMarkdownText 使用 SplitReplyChunks 实现。
func defaultChunkMarkdownText(text string, limit int) []string {
	return SplitReplyChunks(text, limit)
}

// ---------- QuickReply helpers ----------

// BuildQuickReplyMessage 给消息附加快速回复按钮。
func BuildQuickReplyMessage(msg map[string]interface{}, labels []string) map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(labels))
	for _, label := range labels {
		l := label
		if len(l) > 20 {
			l = l[:20]
		}
		items = append(items, map[string]interface{}{
			"type": "action",
			"action": map[string]string{
				"type":  "message",
				"label": l,
				"text":  label,
			},
		})
	}
	copy := make(map[string]interface{}, len(msg)+1)
	for k, v := range msg {
		copy[k] = v
	}
	copy["quickReply"] = map[string]interface{}{"items": items}
	return copy
}

// ---------- DeliverLineAutoReply ----------

// DeliverAutoReplyParams deliverLineAutoReply 参数。
type DeliverAutoReplyParams struct {
	Ctx            context.Context
	Client         *Client
	Payload        ReplyPayload
	To             string
	ReplyToken     string
	ReplyTokenUsed bool
	AccountID      string
	TextLimit      int
	Deps           AutoReplyDeps
}

// DeliverLineAutoReply 投递 LINE 自动回复。
// TS: deliverLineAutoReply()
// 返回 replyTokenUsed（更新后的值）。
func DeliverLineAutoReply(params DeliverAutoReplyParams) (bool, error) {
	ctx := params.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	client := params.Client
	payload := params.Payload
	to := params.To
	replyToken := params.ReplyToken
	replyTokenUsed := params.ReplyTokenUsed
	textLimit := params.TextLimit
	if textLimit <= 0 {
		textLimit = 5000
	}
	deps := params.Deps
	if deps.ProcessLineMessage == nil {
		deps.ProcessLineMessage = ProcessLineMessage
	}
	if deps.ChunkMarkdownText == nil {
		deps.ChunkMarkdownText = defaultChunkMarkdownText
	}

	lineData := payload.ChannelData
	if lineData == nil {
		lineData = &LineChannelData{}
	}

	// pushLineMessages — 批量推送（每次最多 5 条）。
	pushLineMessages := func(messages []interface{}) error {
		for i := 0; i < len(messages); i += 5 {
			end := i + 5
			if end > len(messages) {
				end = len(messages)
			}
			if err := client.PushMessage(ctx, to, messages[i:end]); err != nil {
				return err
			}
		}
		return nil
	}

	// sendLineMessages — 优先使用 reply token，超出部分 push。
	sendLineMessages := func(messages []interface{}, allowReplyToken bool) error {
		if len(messages) == 0 {
			return nil
		}

		remaining := messages
		if allowReplyToken && replyToken != "" && !replyTokenUsed {
			batch := remaining
			if len(batch) > 5 {
				batch = batch[:5]
			}
			if err := client.ReplyMessage(ctx, replyToken, batch); err != nil {
				if deps.OnReplyError != nil {
					deps.OnReplyError(err)
				}
				if pushErr := pushLineMessages(batch); pushErr != nil {
					return fmt.Errorf("push fallback failed: %w", pushErr)
				}
			}
			replyTokenUsed = true
			remaining = remaining[len(batch):]
		}

		if len(remaining) > 0 {
			return pushLineMessages(remaining)
		}
		return nil
	}

	// 构建富消息列表
	richMessages := make([]interface{}, 0)
	hasQuickReplies := len(lineData.QuickReplies) > 0

	// Flex message
	if lineData.FlexMessage != nil {
		altText := lineData.FlexMessage.AltText
		if len(altText) > 400 {
			altText = altText[:400]
		}
		richMessages = append(richMessages, map[string]interface{}{
			"type":     "flex",
			"altText":  altText,
			"contents": lineData.FlexMessage.Contents,
		})
	}

	// Template message
	if lineData.TemplateMessage != nil {
		if tmpl := buildTemplateMessage(lineData.TemplateMessage); tmpl != nil {
			richMessages = append(richMessages, tmpl)
		}
	}

	// Location
	if lineData.Location != nil {
		richMessages = append(richMessages, map[string]interface{}{
			"type":      "location",
			"title":     lineData.Location.Title,
			"address":   lineData.Location.Address,
			"latitude":  lineData.Location.Latitude,
			"longitude": lineData.Location.Longitude,
		})
	}

	// Process text → Flex + plain
	var processed ProcessedLineMessage
	if payload.Text != "" {
		processed = deps.ProcessLineMessage(payload.Text)
	}

	for _, flexMsg := range processed.FlexMessages {
		altText := flexMsg.AltText
		if len(altText) > 400 {
			altText = altText[:400]
		}
		richMessages = append(richMessages, FlexMessage{
			Type:     "flex",
			AltText:  altText,
			Contents: flexMsg.Contents,
		})
	}

	// 文本分块
	var chunks []string
	if processed.Text != "" {
		chunks = deps.ChunkMarkdownText(processed.Text, textLimit)
	}

	// 媒体图片
	mediaURLs := payload.MediaURLs
	if len(mediaURLs) == 0 && payload.MediaURL != "" {
		mediaURLs = []string{payload.MediaURL}
	}
	mediaMessages := make([]interface{}, 0, len(mediaURLs))
	for _, url := range mediaURLs {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		mediaMessages = append(mediaMessages, map[string]interface{}{
			"type":               "image",
			"originalContentUrl": url,
			"previewImageUrl":    url,
		})
	}

	if len(chunks) > 0 {
		hasRichOrMedia := len(richMessages) > 0 || len(mediaMessages) > 0
		if hasQuickReplies && hasRichOrMedia {
			combined := make([]interface{}, 0, len(richMessages)+len(mediaMessages))
			combined = append(combined, richMessages...)
			combined = append(combined, mediaMessages...)
			if err := sendLineMessages(combined, false); err != nil {
				if deps.OnReplyError != nil {
					deps.OnReplyError(err)
				}
			}
		}

		// 发送文本块（带快速回复附加到最后一块）
		for i, chunk := range chunks {
			isLast := i == len(chunks)-1
			var msg interface{}
			if isLast && hasQuickReplies {
				msg = buildTextWithQuickReplies(chunk, lineData.QuickReplies)
			} else {
				msg = map[string]string{"type": "text", "text": chunk}
			}

			if err := sendLineMessages([]interface{}{msg}, i == 0); err != nil {
				return replyTokenUsed, err
			}
		}

		if !hasQuickReplies || !hasRichOrMedia {
			if len(richMessages) > 0 {
				if err := sendLineMessages(richMessages, false); err != nil {
					return replyTokenUsed, err
				}
			}
			if len(mediaMessages) > 0 {
				if err := sendLineMessages(mediaMessages, false); err != nil {
					return replyTokenUsed, err
				}
			}
		}
	} else {
		combined := make([]interface{}, 0, len(richMessages)+len(mediaMessages))
		combined = append(combined, richMessages...)
		combined = append(combined, mediaMessages...)
		if hasQuickReplies && len(combined) > 0 {
			targetIdx := len(combined) - 1
			if replyToken != "" && !replyTokenUsed && targetIdx > 4 {
				targetIdx = 4
			}
			// 附加 quickReply 到目标消息
			if msgMap, ok := combined[targetIdx].(map[string]interface{}); ok {
				combined[targetIdx] = BuildQuickReplyMessage(msgMap, lineData.QuickReplies)
			}
		}
		if err := sendLineMessages(combined, true); err != nil {
			return replyTokenUsed, err
		}
	}

	return replyTokenUsed, nil
}

// ---------- template message builder ----------

func buildTemplateMessage(tmpl *LineTemplateMessagePayload) interface{} {
	if tmpl == nil {
		return nil
	}
	altText := tmpl.AltText
	if altText == "" {
		altText = tmpl.Text
	}
	if altText == "" {
		altText = "Template message"
	}

	var template map[string]interface{}

	switch tmpl.Type {
	case "confirm":
		template = map[string]interface{}{
			"type": "confirm",
			"text": tmpl.Text,
			"actions": []map[string]interface{}{
				{"type": "postback", "label": tmpl.ConfirmLabel, "data": tmpl.ConfirmData},
				{"type": "postback", "label": tmpl.CancelLabel, "data": tmpl.CancelData},
			},
		}
	case "buttons":
		actions := make([]map[string]interface{}, 0, len(tmpl.Actions))
		for _, a := range tmpl.Actions {
			actions = append(actions, templateActionToMap(a))
		}
		m := map[string]interface{}{
			"type":    "buttons",
			"text":    tmpl.Text,
			"actions": actions,
		}
		if tmpl.Title != "" {
			m["title"] = tmpl.Title
		}
		if tmpl.ThumbnailURL != "" {
			m["thumbnailImageUrl"] = tmpl.ThumbnailURL
		}
		template = m
	case "carousel":
		cols := make([]map[string]interface{}, 0, len(tmpl.Columns))
		for _, col := range tmpl.Columns {
			actions := make([]map[string]interface{}, 0, len(col.Actions))
			for _, a := range col.Actions {
				actions = append(actions, templateActionToMap(a))
			}
			c := map[string]interface{}{
				"text":    col.Text,
				"actions": actions,
			}
			if col.Title != "" {
				c["title"] = col.Title
			}
			if col.ThumbnailURL != "" {
				c["thumbnailImageUrl"] = col.ThumbnailURL
			}
			cols = append(cols, c)
		}
		template = map[string]interface{}{
			"type":    "carousel",
			"columns": cols,
		}
	default:
		return nil
	}

	return map[string]interface{}{
		"type":     "template",
		"altText":  altText,
		"template": template,
	}
}

func templateActionToMap(a TemplateAction) map[string]interface{} {
	m := map[string]interface{}{
		"type":  a.Type,
		"label": a.Label,
	}
	if a.Data != "" {
		m["data"] = a.Data
	}
	if a.URI != "" {
		m["uri"] = a.URI
	}
	if a.Text != "" {
		m["text"] = a.Text
	}
	return m
}

// buildTextWithQuickReplies 构建带快速回复按钮的文本消息。
func buildTextWithQuickReplies(text string, labels []string) map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(labels))
	for _, label := range labels {
		l := label
		if len(l) > 20 {
			l = l[:20]
		}
		items = append(items, map[string]interface{}{
			"type": "action",
			"action": map[string]string{
				"type":  "message",
				"label": l,
				"text":  label,
			},
		})
	}
	return map[string]interface{}{
		"type": "text",
		"text": text,
		"quickReply": map[string]interface{}{
			"items": items,
		},
	}
}
