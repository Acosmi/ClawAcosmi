package discord

// Discord 回复投递 — 继承自 src/discord/monitor/reply-delivery.ts (81L)
// Phase 9 实现：分块发送 + 表格转换 + 反应状态 (⏳→✅/❌)。
// W-024 fix: 补全 media/attachment 发送逻辑，对齐 TS 的 mediaUrl/mediaUrls 处理。
// W-025 fix: 添加 sendDiscordMessageWithRetry 抽象层（retry/rate limit），对齐 TS sendMessageDiscord。

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/anthropic/open-acosmi/internal/autoreply"
	"github.com/anthropic/open-acosmi/pkg/markdown"
	"github.com/anthropic/open-acosmi/pkg/retry"
)

// DiscordReplyOpts 回复选项
type DiscordReplyOpts struct {
	ThreadID      string
	ReplyToID     string
	TextLimit     int // 可配置的文本上限（来自 resolveTextChunkLimit），默认 2000
	MaxLines      int
	TableMode     string // "off"|"bullets"|"code"
	ChunkMode     ChunkMode
	ShowReactions bool // W-027 note: [Go 扩展] 是否显示 ⏳→✅ 反应，TS 中无此功能
}

// DeliverDiscordReplies 投递多条回复到 Discord 频道（含 media/attachment 支持）。
// 对齐 TS: src/discord/monitor/reply-delivery.ts deliverDiscordReply({ replies: ReplyPayload[] })
// 每条 ReplyPayload 可包含 text + mediaUrl/mediaUrls；
// 无 media 时走文本分块投递，有 media 时首条附带文本+文件、后续仅发文件。
func DeliverDiscordReplies(ctx context.Context, monCtx *DiscordMonitorContext, channelID string, replies []autoreply.ReplyPayload, opts DiscordReplyOpts) error {
	logger := monCtx.Logger.With("action", "reply", "channel", channelID)

	targetChannel := channelID
	if opts.ThreadID != "" {
		targetChannel = opts.ThreadID
	}

	// 收集所有发送的消息 ID（用于反应状态标记）
	var sentIDs []string

	for _, payload := range replies {
		// 构建 mediaList — 对齐 TS: mediaUrls ?? (mediaUrl ? [mediaUrl] : [])
		mediaList := resolveMediaList(payload)

		// 表格转换
		rawText := payload.Text
		tableMode := opts.TableMode
		if tableMode == "" {
			tableMode = "code"
		}
		text := rawText
		if tableMode != "off" {
			text = markdown.ConvertMarkdownTables(rawText, markdown.TableMode(tableMode))
		}

		// 跳过空回复 — 对齐 TS: if (!text && mediaList.length === 0) continue
		if strings.TrimSpace(text) == "" && len(mediaList) == 0 {
			continue
		}

		replyTo := strings.TrimSpace(opts.ReplyToID)

		if len(mediaList) == 0 {
			// ── 纯文本分块投递 ──────────────────────────────
			ids, err := deliverTextChunks(ctx, monCtx, targetChannel, channelID, text, replyTo, opts)
			if err != nil {
				markDiscordReplyReaction(monCtx, channelID, sentIDs, "❌")
				return err
			}
			sentIDs = append(sentIDs, ids...)
			continue
		}

		// ── 带 media 的投递 ──────────────────────────────
		// 对齐 TS: 第一条 media 附带文本发送，后续 media 只发文件
		ids, err := deliverMediaMessages(ctx, monCtx, targetChannel, channelID, text, replyTo, mediaList, logger)
		if err != nil {
			markDiscordReplyReaction(monCtx, channelID, sentIDs, "❌")
			return err
		}
		sentIDs = append(sentIDs, ids...)
	}

	// 成功：标记 ✅
	if opts.ShowReactions && len(sentIDs) > 0 {
		markDiscordReplyReaction(monCtx, channelID, sentIDs[:1], "✅")
	}

	logger.Debug("reply delivered",
		"payloads", len(replies),
		"sent", len(sentIDs),
		"channel", targetChannel,
	)
	return nil
}

// DeliverDiscordReply 投递单条纯文本回复到 Discord 频道（向后兼容）。
// 内部委托给 DeliverDiscordReplies。
func DeliverDiscordReply(ctx context.Context, monCtx *DiscordMonitorContext, channelID, text string, opts DiscordReplyOpts) error {
	return DeliverDiscordReplies(ctx, monCtx, channelID, []autoreply.ReplyPayload{
		{Text: text},
	}, opts)
}

// SendDiscordTypingAndReply 发送 typing 后投递纯文本回复（向后兼容）。
func SendDiscordTypingAndReply(ctx context.Context, monCtx *DiscordMonitorContext, channelID, text string, opts DiscordReplyOpts) error {
	_ = SendTypingIndicator(ctx, monCtx.Session, channelID)
	return DeliverDiscordReply(ctx, monCtx, channelID, text, opts)
}

// SendDiscordTypingAndReplies 发送 typing 后投递多条回复（含 media 支持）。
func SendDiscordTypingAndReplies(ctx context.Context, monCtx *DiscordMonitorContext, channelID string, replies []autoreply.ReplyPayload, opts DiscordReplyOpts) error {
	_ = SendTypingIndicator(ctx, monCtx.Session, channelID)
	return DeliverDiscordReplies(ctx, monCtx, channelID, replies, opts)
}

// ── 内部辅助函数 ────────────────────────────────────────

// resolveMediaList 从 ReplyPayload 中解析 media URL 列表。
// 对齐 TS: const mediaList = payload.mediaUrls ?? (payload.mediaUrl ? [payload.mediaUrl] : [])
func resolveMediaList(payload autoreply.ReplyPayload) []string {
	if len(payload.MediaURLs) > 0 {
		// 过滤空字符串
		var urls []string
		for _, u := range payload.MediaURLs {
			if strings.TrimSpace(u) != "" {
				urls = append(urls, u)
			}
		}
		return urls
	}
	if strings.TrimSpace(payload.MediaURL) != "" {
		return []string{payload.MediaURL}
	}
	return nil
}

// resolveChunkLimit 计算实际分块字符上限。
// 对齐 TS reply-delivery.ts L23: const chunkLimit = Math.min(params.textLimit, 2000)
// textLimit 来自配置（resolveTextChunkLimit），可能 > 2000，但 Discord 消息上限为 2000 字符，
// 因此取 min(textLimit, 2000)。
func resolveChunkLimit(textLimit int) int {
	limit := textLimit
	if limit <= 0 {
		limit = defaultMaxChars // 2000
	}
	if limit > defaultMaxChars {
		limit = defaultMaxChars
	}
	return limit
}

// deliverTextChunks 分块投递纯文本消息。
func deliverTextChunks(ctx context.Context, monCtx *DiscordMonitorContext, targetChannel, channelID, text, replyTo string, opts DiscordReplyOpts) ([]string, error) {
	logger := monCtx.Logger.With("action", "reply-text", "channel", targetChannel)

	// W-026 fix: 对齐 TS chunkLimit = Math.min(params.textLimit, 2000)
	chunkLimit := resolveChunkLimit(opts.TextLimit)
	chunkOpts := ChunkDiscordTextOpts{MaxChars: chunkLimit, MaxLines: opts.MaxLines}
	chunks := ChunkDiscordTextWithMode(text, chunkOpts, opts.ChunkMode)
	if len(chunks) == 0 && text != "" {
		chunks = []string{text}
	}

	var sentIDs []string
	isFirstChunk := true
	for i, chunk := range chunks {
		trimmed := strings.TrimSpace(chunk)
		if trimmed == "" {
			continue
		}
		ref := (*discordgo.MessageReference)(nil)
		if isFirstChunk && replyTo != "" {
			ref = &discordgo.MessageReference{MessageID: replyTo, ChannelID: channelID}
		}
		msg, err := sendDiscordMessageWithRetry(ctx, monCtx.Session, targetChannel, &discordgo.MessageSend{
			Content:   chunk,
			Reference: ref,
		})
		if err != nil {
			logger.Error("send reply chunk failed",
				"chunk", fmt.Sprintf("%d/%d", i+1, len(chunks)),
				"error", err,
			)
			return sentIDs, fmt.Errorf("send reply chunk %d/%d: %w", i+1, len(chunks), err)
		}
		if msg != nil {
			sentIDs = append(sentIDs, msg.ID)
		}
		isFirstChunk = false
	}
	return sentIDs, nil
}

// deliverMediaMessages 投递带 media 的消息。
// 对齐 TS reply-delivery.ts L61-79:
//   - 第一条 media 附带 text 发送
//   - 后续 media 以空 content 发送
func deliverMediaMessages(ctx context.Context, monCtx *DiscordMonitorContext, targetChannel, channelID, text, replyTo string, mediaList []string, logger interface {
	Error(msg string, keysAndValues ...interface{})
}) ([]string, error) {
	var sentIDs []string

	if len(mediaList) == 0 {
		return sentIDs, nil
	}

	// 第一条 media 附带文本
	firstMedia := mediaList[0]
	ids, err := sendMediaMessage(ctx, monCtx, targetChannel, channelID, text, replyTo, firstMedia)
	if err != nil {
		logger.Error("send media message failed",
			"mediaUrl", firstMedia,
			"error", err,
		)
		return sentIDs, fmt.Errorf("send media message: %w", err)
	}
	sentIDs = append(sentIDs, ids...)

	// 后续 media 无文本
	for _, extra := range mediaList[1:] {
		ids, err := sendMediaMessage(ctx, monCtx, targetChannel, channelID, "", "", extra)
		if err != nil {
			logger.Error("send extra media failed",
				"mediaUrl", extra,
				"error", err,
			)
			return sentIDs, fmt.Errorf("send extra media: %w", err)
		}
		sentIDs = append(sentIDs, ids...)
	}

	return sentIDs, nil
}

// sendMediaMessage 发送单条带文件附件的消息。
// 使用 loadDiscordMedia 下载远程文件，然后通过 discordgo.ChannelMessageSendComplex 发送。
// 对齐 TS: sendDiscordMedia (send.shared.ts L339-396)
func sendMediaMessage(ctx context.Context, monCtx *DiscordMonitorContext, targetChannel, channelID, text, replyTo, mediaURL string) ([]string, error) {
	// 下载 media（复用 send_media.go 中的 loadDiscordMedia）
	media, err := loadDiscordMedia(mediaURL)
	if err != nil {
		return nil, fmt.Errorf("load media from %s: %w", mediaURL, err)
	}

	ref := (*discordgo.MessageReference)(nil)
	if replyTo != "" {
		ref = &discordgo.MessageReference{MessageID: replyTo, ChannelID: channelID}
	}

	fileName := media.FileName
	if fileName == "" {
		fileName = "upload"
	}

	msg, err := sendDiscordMessageWithRetry(ctx, monCtx.Session, targetChannel, &discordgo.MessageSend{
		Content:   text,
		Reference: ref,
		Files: []*discordgo.File{
			{
				Name:        fileName,
				ContentType: media.ContentType,
				Reader:      bytes.NewReader(media.Data),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	var ids []string
	if msg != nil {
		ids = append(ids, msg.ID)
	}
	return ids, nil
}

// ── sendDiscordMessageWithRetry — TS sendMessageDiscord 等价抽象层 ──────
// 对齐 TS: createDiscordRetryRunner (infra/retry-policy.ts L45-70)
//   - shouldRetry: 仅在 429 (RateLimitError) 时重试
//   - retryAfterMs: 使用 Discord 返回的 Retry-After 头
//   - 默认最多 3 次尝试, 初始延迟 500ms, 最大延迟 30s
//
// discordgo 内置了基本的 429 处理, 但当其内置 rate limiter 未能覆盖时
// (如 cloudflare 级别的限速), 此层提供额外保护。
// W-025 fix

// discordReplyRetryDefaults 回复投递重试默认值
var discordReplyRetryDefaults = retry.Config{
	MaxAttempts:  3,
	InitialDelay: 500 * time.Millisecond,
	MaxDelay:     30 * time.Second,
	Multiplier:   2.0,
	JitterFactor: 0.1,
}

// sendDiscordMessageWithRetry 带 retry/rate-limit 的消息发送包装。
// 对齐 TS: sendMessageDiscord → createDiscordRetryRunner wrapping REST calls。
// 仅在 HTTP 429 (Too Many Requests) 或 5xx 服务端错误时重试。
func sendDiscordMessageWithRetry(ctx context.Context, session *discordgo.Session, channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
	cfg := discordReplyRetryDefaults
	cfg.Label = "discord-reply-send"
	cfg.ShouldRetry = func(err error, _ int) bool {
		return isDiscordRetryableError(err)
	}
	cfg.RetryAfterHint = func(err error) time.Duration {
		if restErr, ok := err.(*discordgo.RESTError); ok && restErr.Response != nil {
			if restErr.Response.StatusCode == http.StatusTooManyRequests {
				retryAfter := parseRetryAfterSeconds(string(restErr.ResponseBody), restErr.Response)
				if retryAfter != nil {
					return time.Duration(*retryAfter * float64(time.Second))
				}
			}
		}
		return 0
	}

	return retry.DoWithResult(ctx, cfg, func(_ int) (*discordgo.Message, error) {
		return session.ChannelMessageSendComplex(channelID, data)
	})
}

// isDiscordRetryableError 判断 discordgo 错误是否可重试。
// 仅对 429 (rate limit) 和 5xx (服务端错误) 重试。
func isDiscordRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if restErr, ok := err.(*discordgo.RESTError); ok && restErr.Response != nil {
		code := restErr.Response.StatusCode
		return code == http.StatusTooManyRequests || code >= 500
	}
	return false
}

// markDiscordReplyReaction 为消息添加反应标记。
// W-027 note: [Go 扩展] reaction 状态标记（⏳→✅/❌）为 Go 版新增功能，
// TS (reply-delivery.ts) 中不存在此机制。属于 Go 扩展功能，非 TS 对齐项。
func markDiscordReplyReaction(monCtx *DiscordMonitorContext, channelID string, messageIDs []string, emoji string) {
	for _, mid := range messageIDs {
		_ = monCtx.Session.MessageReactionAdd(channelID, mid, emoji)
	}
}
