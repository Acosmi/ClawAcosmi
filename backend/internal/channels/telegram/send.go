package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anthropic/open-acosmi/internal/channels/ratelimit"
	"github.com/anthropic/open-acosmi/pkg/retry"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// Telegram 消息发送 — 继承自 src/telegram/send.ts (918L)
// 使用直接 HTTP 调用替代 grammy Bot SDK。

// TelegramSendOpts 发送选项
type TelegramSendOpts struct {
	Token            string
	AccountID        string
	Verbose          bool
	MediaURL         string
	MaxBytes         int64
	TextMode         string // "markdown" | "html"
	PlainText        string
	AsVoice          bool
	AsVideoNote      bool
	Silent           bool
	ReplyToMessageID *int
	QuoteText        string
	MessageThreadID  *int
	Buttons          [][]InlineButton
}

// InlineButton 内联键盘按钮
type InlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

// TelegramSendResult 发送结果
type TelegramSendResult struct {
	MessageID string
	ChatID    string
}

const telegramMaxCaptionLength = 1024

var (
	parseErrRe       = regexp.MustCompile(`(?i)can't parse entities|parse entities|find end of the entity`)
	threadNotFoundRe = regexp.MustCompile(`(?i)400:\s*Bad Request:\s*message thread not found`)
	tMeLinkRe        = regexp.MustCompile(`(?i)^https?://t\.me/([A-Za-z0-9_]+)$`)
	tMeShortRe       = regexp.MustCompile(`(?i)^t\.me/([A-Za-z0-9_]+)$`)
	numericIDRe      = regexp.MustCompile(`^-?\d+$`)
	usernameRe       = regexp.MustCompile(`(?i)^[A-Za-z0-9_]{5,}$`)
)

// splitTelegramCaption 分割 caption 文本。
// Telegram caption 限制 1024 字符，超过时作为独立后续文本消息发送。
// 对齐 TS caption.ts splitTelegramCaption。
func splitTelegramCaption(text string) (caption, followUpText string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", ""
	}
	if len([]rune(trimmed)) > telegramMaxCaptionLength {
		return "", trimmed
	}
	return trimmed, ""
}

// normalizeChatID 规范化聊天 ID
func normalizeChatID(to string) (string, error) {
	trimmed := strings.TrimSpace(to)
	if trimmed == "" {
		return "", fmt.Errorf("recipient is required for Telegram sends")
	}
	normalized := StripTelegramInternalPrefixes(trimmed)

	// 处理 t.me 链接
	if m := tMeLinkRe.FindStringSubmatch(normalized); m != nil {
		normalized = "@" + m[1]
	} else if m := tMeShortRe.FindStringSubmatch(normalized); m != nil {
		normalized = "@" + m[1]
	}

	if normalized == "" {
		return "", fmt.Errorf("recipient is required for Telegram sends")
	}
	if strings.HasPrefix(normalized, "@") {
		return normalized, nil
	}
	if numericIDRe.MatchString(normalized) {
		return normalized, nil
	}
	if usernameRe.MatchString(normalized) {
		return "@" + normalized, nil
	}
	return normalized, nil
}

// normalizeMessageID 规范化消息 ID
func normalizeMessageID(raw string) (int, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return 0, fmt.Errorf("message id is required")
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid message id %q: %w", v, err)
	}
	return n, nil
}

// apiMessage Telegram API sendMessage 返回结构
type apiMessage struct {
	MessageID int `json:"message_id"`
	Chat      struct {
		ID int64 `json:"id"`
	} `json:"chat"`
}
type apiResponse struct {
	OK     bool        `json:"ok"`
	Result *apiMessage `json:"result,omitempty"`
	Desc   string      `json:"description,omitempty"`
}

// callTelegramAPI 调用 Telegram Bot API（JSON body）
// HD-4: 令牌桶限速（等价 grammy apiThrottler）
func callTelegramAPI(ctx context.Context, client *http.Client, token, method string, params map[string]interface{}) (*apiMessage, error) {
	// W3-D1: 速率限制 — 等价 @grammyjs/transformer-throttler
	if err := ratelimit.GlobalTelegramLimiter().Wait(ctx); err != nil {
		return nil, fmt.Errorf("telegram rate limit wait: %w", err)
	}

	body, _ := json.Marshal(params)
	url := fmt.Sprintf("%s/bot%s/%s", TelegramAPIBaseURL, token, method)

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
	var result apiResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("telegram API %s: decode failed: %w", method, err)
	}
	if !result.OK {
		return nil, fmt.Errorf("%d: %s", resp.StatusCode, result.Desc)
	}
	return result.Result, nil
}

// callTelegramAPIMultipart 调用 Telegram Bot API（multipart 文件上传）
// HD-4: 令牌桶限速（等价 grammy apiThrottler）
func callTelegramAPIMultipart(ctx context.Context, client *http.Client, token, method string, params map[string]string, fileField string, fileData []byte, fileName string) (*apiMessage, error) {
	// W3-D1: 速率限制 — 等价 @grammyjs/transformer-throttler
	if err := ratelimit.GlobalTelegramLimiter().Wait(ctx); err != nil {
		return nil, fmt.Errorf("telegram rate limit wait: %w", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for k, v := range params {
		_ = writer.WriteField(k, v)
	}
	if fileField != "" && fileData != nil {
		part, err := writer.CreateFormFile(fileField, fileName)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(fileData); err != nil {
			return nil, err
		}
	}
	writer.Close()

	url := fmt.Sprintf("%s/bot%s/%s", TelegramAPIBaseURL, token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result apiResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("telegram API %s: decode failed: %w", method, err)
	}
	if !result.OK {
		return nil, fmt.Errorf("%d: %s", resp.StatusCode, result.Desc)
	}
	return result.Result, nil
}

// buildInlineKeyboard 构建内联键盘
func buildInlineKeyboard(buttons [][]InlineButton) map[string]interface{} {
	if len(buttons) == 0 {
		return nil
	}
	var rows [][]map[string]string
	for _, row := range buttons {
		var r []map[string]string
		for _, btn := range row {
			if btn.Text != "" && btn.CallbackData != "" {
				r = append(r, map[string]string{
					"text":          btn.Text,
					"callback_data": btn.CallbackData,
				})
			}
		}
		if len(r) > 0 {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return map[string]interface{}{"inline_keyboard": rows}
}

// resolveToken 解析 token
func resolveToken(explicit string, account ResolvedTelegramAccount) (string, error) {
	if t := strings.TrimSpace(explicit); t != "" {
		return t, nil
	}
	if account.Token == "" {
		return "", fmt.Errorf(
			"telegram bot token missing for account %q (set botToken/tokenFile or TELEGRAM_BOT_TOKEN)",
			account.AccountID)
	}
	return strings.TrimSpace(account.Token), nil
}

// resolveTableMode 从 config 动态解析 Markdown 表格渲染模式。
// 对应 TS resolveMarkdownTableMode({cfg, channel, accountId})。
// 优先级：账户级 → 全局级 → 默认值。
func resolveTableMode(cfg *types.OpenAcosmiConfig, account *ResolvedTelegramAccount) MarkdownTableMode {
	// 1. 账户级覆盖
	if account != nil && account.Config.Markdown != nil && account.Config.Markdown.Tables != "" {
		return mapGlobalTableMode(account.Config.Markdown.Tables)
	}
	// 2. 全局配置
	if cfg != nil && cfg.Markdown != nil && cfg.Markdown.Tables != "" {
		return mapGlobalTableMode(cfg.Markdown.Tables)
	}
	return TableModeDefault
}

// mapGlobalTableMode 将全局 MarkdownTableMode 映射到 Telegram 本地模式。
func mapGlobalTableMode(mode types.MarkdownTableMode) MarkdownTableMode {
	switch mode {
	case types.MarkdownTableOff:
		return TableModeOff
	case types.MarkdownTableBullets:
		return TableModeBullets
	case types.MarkdownTableCode:
		return TableModeCode
	default:
		return TableModeDefault
	}
}

// wrapChatNotFound 将 "chat not found" 错误包装为用户友好消息。
// HD-3: 对应 TS send.ts wrapChatNotFound。
func wrapChatNotFound(err error, chatID, rawTo string) error {
	if err == nil {
		return nil
	}
	errStr := err.Error()
	if !strings.Contains(strings.ToLower(errStr), "chat not found") {
		return err
	}
	return fmt.Errorf(
		"telegram send failed: chat not found (chat_id=%s). "+
			"Likely: bot not started in DM, bot removed from group/channel, "+
			"group migrated (new -100… id), or wrong bot token. Input was: %q",
		chatID, rawTo)
}

// buildTelegramRetryConfig 构建 Telegram 发送重试配置。
// HD-2: 对应 TS createTelegramRetryRunner。
func buildTelegramRetryConfig(accountRetry *types.OutboundRetryConfig, verbose bool) retry.Config {
	cfg := retry.Config{
		MaxAttempts:  3,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     15 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0.1,
		Label:        "telegram-send",
		ShouldRetry: func(err error, _ int) bool {
			return IsRecoverableTelegramNetworkError(err, NetworkCtxSend)
		},
	}
	// 合入账户级自定义重试配置
	if accountRetry != nil {
		if accountRetry.Attempts > 0 {
			cfg.MaxAttempts = accountRetry.Attempts
		}
		if accountRetry.MinDelayMs > 0 {
			cfg.InitialDelay = time.Duration(accountRetry.MinDelayMs) * time.Millisecond
		}
		if accountRetry.MaxDelayMs > 0 {
			cfg.MaxDelay = time.Duration(accountRetry.MaxDelayMs) * time.Millisecond
		}
	}
	if verbose {
		cfg.OnRetry = func(info retry.RetryInfo) {
			slog.Warn("telegram send retry",
				"attempt", info.Attempt, "delay", info.Delay, "err", info.Err)
		}
	}
	return cfg
}

// hasMessageThreadIDParam 检查 params 中是否包含 message_thread_id。
func hasMessageThreadIDParam(params map[string]interface{}) bool {
	v, ok := params["message_thread_id"]
	if !ok {
		return false
	}
	switch val := v.(type) {
	case int:
		return true
	case float64:
		return true
	case string:
		return strings.TrimSpace(val) != ""
	}
	return false
}

// removeMessageThreadIDParam 移除 params 中的 message_thread_id。
func removeMessageThreadIDParam(params map[string]interface{}) {
	delete(params, "message_thread_id")
}

// SendMessageTelegram 发送 Telegram 消息（文本或媒体）。
func SendMessageTelegram(ctx context.Context, cfg *types.OpenAcosmiConfig, to, text string, opts TelegramSendOpts) (*TelegramSendResult, error) {
	account := ResolveTelegramAccount(cfg, opts.AccountID)
	token, err := resolveToken(opts.Token, account)
	if err != nil {
		return nil, err
	}

	target := ParseTelegramTarget(to)
	chatID, err := normalizeChatID(target.ChatID)
	if err != nil {
		return nil, err
	}

	client, err := NewDefaultTelegramHTTPClient(account)
	if err != nil {
		return nil, fmt.Errorf("create HTTP client: %w", err)
	}

	// 确定 thread ID
	threadID := opts.MessageThreadID
	if threadID == nil {
		threadID = target.MessageThreadID
	}

	// HD-5: 动态解析 tableMode
	textMode := opts.TextMode
	if textMode == "" {
		textMode = "markdown"
	}
	tableMode := resolveTableMode(cfg, &account)

	renderText := func(raw string) string {
		return RenderTelegramHTMLText(raw, textMode, tableMode)
	}

	if strings.TrimSpace(text) == "" && opts.MediaURL == "" {
		return nil, fmt.Errorf("message must be non-empty for Telegram sends")
	}

	replyMarkup := buildInlineKeyboard(opts.Buttons)
	linkPreview := true
	if account.Config.LinkPreview != nil {
		linkPreview = *account.Config.LinkPreview
	}

	// HD-2: 构建重试配置
	retryCfg := buildTelegramRetryConfig(account.Config.Retry, opts.Verbose)

	// 发送文本消息（带 HD-1 thread fallback + HD-2 retry）
	sendText := func(rawText string, fallbackText string) (*apiMessage, error) {
		params := map[string]interface{}{
			"chat_id":    chatID,
			"text":       renderText(rawText),
			"parse_mode": "HTML",
		}
		if threadID != nil {
			params["message_thread_id"] = *threadID
		}
		if !linkPreview {
			params["link_preview_options"] = map[string]bool{"is_disabled": true}
		}
		if opts.Silent {
			params["disable_notification"] = true
		}
		if opts.ReplyToMessageID != nil {
			if q := strings.TrimSpace(opts.QuoteText); q != "" {
				params["reply_parameters"] = map[string]interface{}{
					"message_id": int(math.Trunc(float64(*opts.ReplyToMessageID))),
					"quote":      q,
				}
			} else {
				params["reply_to_message_id"] = *opts.ReplyToMessageID
			}
		}
		if replyMarkup != nil {
			params["reply_markup"] = replyMarkup
		}

		// HD-2: 带重试的 API 调用
		doSend := func(p map[string]interface{}) (*apiMessage, error) {
			return retry.DoWithResult(ctx, retryCfg, func(_ int) (*apiMessage, error) {
				return callTelegramAPI(ctx, client, token, "sendMessage", p)
			})
		}

		// HD-1: sendWithThreadFallback — thread_not_found 时去掉 thread_id 重试
		result, err := doSend(params)
		if err != nil {
			errStr := err.Error()

			// thread_not_found 回退
			if hasMessageThreadIDParam(params) && threadNotFoundRe.MatchString(errStr) {
				if opts.Verbose {
					slog.Warn("telegram thread not found, retrying without thread", "err", errStr)
				}
				removeMessageThreadIDParam(params)
				result, err = doSend(params)
				if err == nil {
					return result, nil
				}
				errStr = err.Error()
			}

			// HTML 解析错误回退到纯文本
			if parseErrRe.MatchString(errStr) {
				if opts.Verbose {
					slog.Warn("telegram HTML parse failed, retrying plain text", "err", errStr)
				}
				fb := fallbackText
				if fb == "" {
					fb = rawText
				}
				params["text"] = fb
				delete(params, "parse_mode")
				// HD-3: 包装 chat not found
				result, err = doSend(params)
				if err != nil {
					return nil, wrapChatNotFound(err, chatID, to)
				}
				return result, nil
			}
			// HD-3: 包装 chat not found
			return nil, wrapChatNotFound(err, chatID, to)
		}
		return result, nil
	}

	// 无媒体 — 发送纯文本
	if opts.MediaURL == "" {
		fb := opts.PlainText
		res, err := sendText(text, fb)
		if err != nil {
			return nil, err
		}
		msgID := "unknown"
		resolvedChat := chatID
		if res != nil {
			msgID = strconv.Itoa(res.MessageID)
			resolvedChat = strconv.FormatInt(res.Chat.ID, 10)
		}
		return &TelegramSendResult{MessageID: msgID, ChatID: resolvedChat}, nil
	}

	// 媒体发送 — 下载 → MIME 检测 → 路由到对应 API
	mediaData, mediaContentType, mediaFileName, dlErr := downloadMediaURL(ctx, client, opts.MediaURL)
	if dlErr != nil {
		slog.Warn("telegram media download failed, sending text only", "err", dlErr, "url", opts.MediaURL)
		// 降级为纯文本发送
		fb := opts.PlainText
		res, err := sendText(text, fb)
		if err != nil {
			return nil, err
		}
		msgID := "unknown"
		resolvedChat := chatID
		if res != nil {
			msgID = strconv.Itoa(res.MessageID)
			resolvedChat = strconv.FormatInt(res.Chat.ID, 10)
		}
		return &TelegramSendResult{MessageID: msgID, ChatID: resolvedChat}, nil
	}

	// 根据 MIME 类型选择 API 方法和字段名
	apiMethod, fileField := resolveMediaAPIMethodWithFileName(mediaContentType, mediaFileName)

	// 判断是否 videoNote (TS L392, L456-472)
	isVideoNote := strings.HasPrefix(strings.ToLower(mediaContentType), "video/") && opts.AsVideoNote
	if isVideoNote {
		apiMethod = "sendVideoNote"
		fileField = "video_note"
	}

	// 判断是否 voice (TS L488-520: resolveTelegramVoiceSend)
	if strings.HasPrefix(strings.ToLower(mediaContentType), "audio/") && opts.AsVoice {
		decision := ResolveTelegramVoiceSend(true, mediaContentType, mediaFileName, func(msg string) {
			slog.Info(msg)
		})
		if decision.UseVoice {
			apiMethod = "sendVoice"
			fileField = "voice"
		}
	}

	// Caption splitting (TS L398-405)
	var caption, followUpText string
	if isVideoNote {
		// videoNote 不支持 caption，全部作为后续文本
		caption = ""
		if strings.TrimSpace(text) != "" {
			followUpText = text
		}
	} else {
		caption, followUpText = splitTelegramCaption(text)
	}

	// 构建 multipart 参数
	mpParams := map[string]string{
		"chat_id": chatID,
	}
	if caption != "" {
		mpParams["caption"] = renderText(caption)
		mpParams["parse_mode"] = "HTML"
	}
	if threadID != nil {
		mpParams["message_thread_id"] = strconv.Itoa(*threadID)
	}
	if opts.ReplyToMessageID != nil {
		mpParams["reply_to_message_id"] = strconv.Itoa(*opts.ReplyToMessageID)
	}
	if opts.Silent {
		mpParams["disable_notification"] = "true"
	}

	mediaResult, err := retry.DoWithResult(ctx, retryCfg, func(_ int) (*apiMessage, error) {
		return callTelegramAPIMultipart(ctx, client, token, apiMethod, mpParams, fileField, mediaData, mediaFileName)
	})
	if err != nil {
		return nil, wrapChatNotFound(err, chatID, to)
	}

	mediaMsgID := "unknown"
	resolvedChat := chatID
	if mediaResult != nil {
		mediaMsgID = strconv.Itoa(mediaResult.MessageID)
		resolvedChat = strconv.FormatInt(mediaResult.Chat.ID, 10)
	}

	// 后续文本消息 (TS L552-566: caption 拆分后的跟随文本)
	if followUpText != "" {
		followRes, followErr := sendText(followUpText, "")
		if followErr != nil {
			slog.Warn("telegram follow-up text failed", "err", followErr)
		} else if followRes != nil {
			// 返回文本消息 ID 作为主消息
			return &TelegramSendResult{
				MessageID: strconv.Itoa(followRes.MessageID),
				ChatID:    resolvedChat,
			}, nil
		}
	}

	return &TelegramSendResult{MessageID: mediaMsgID, ChatID: resolvedChat}, nil
}

// ReactMessageTelegram 发送/移除反应 emoji。
func ReactMessageTelegram(ctx context.Context, cfg *types.OpenAcosmiConfig, chatIDInput, messageIDInput, emoji string, remove bool, accountID string) error {
	account := ResolveTelegramAccount(cfg, accountID)
	token, err := resolveToken("", account)
	if err != nil {
		return err
	}
	chatID, err := normalizeChatID(chatIDInput)
	if err != nil {
		return err
	}
	msgID, err := normalizeMessageID(messageIDInput)
	if err != nil {
		return err
	}

	client, err := NewDefaultTelegramHTTPClient(account)
	if err != nil {
		return err
	}

	var reactions []map[string]string
	trimmed := strings.TrimSpace(emoji)
	if !remove && trimmed != "" {
		reactions = []map[string]string{{"type": "emoji", "emoji": trimmed}}
	}

	params := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": msgID,
		"reaction":   reactions,
	}
	_, err = callTelegramAPI(ctx, client, token, "setMessageReaction", params)
	return err
}

// DeleteMessageTelegram 删除消息。
func DeleteMessageTelegram(ctx context.Context, cfg *types.OpenAcosmiConfig, chatIDInput, messageIDInput, accountID string) error {
	account := ResolveTelegramAccount(cfg, accountID)
	token, err := resolveToken("", account)
	if err != nil {
		return err
	}
	chatID, err := normalizeChatID(chatIDInput)
	if err != nil {
		return err
	}
	msgID, err := normalizeMessageID(messageIDInput)
	if err != nil {
		return err
	}

	client, err := NewDefaultTelegramHTTPClient(account)
	if err != nil {
		return err
	}

	params := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": msgID,
	}
	_, err = callTelegramAPI(ctx, client, token, "deleteMessage", params)
	return err
}

// EditMessageTelegram 编辑消息文本。
func EditMessageTelegram(ctx context.Context, cfg *types.OpenAcosmiConfig, chatIDInput, messageIDInput, text, accountID string, opts TelegramSendOpts) error {
	account := ResolveTelegramAccount(cfg, accountID)
	token, err := resolveToken(opts.Token, account)
	if err != nil {
		return err
	}
	chatID, err := normalizeChatID(chatIDInput)
	if err != nil {
		return err
	}
	msgID, err := normalizeMessageID(messageIDInput)
	if err != nil {
		return err
	}

	client, err := NewDefaultTelegramHTTPClient(account)
	if err != nil {
		return err
	}

	textMode := opts.TextMode
	if textMode == "" {
		textMode = "markdown"
	}
	htmlText := RenderTelegramHTMLText(text, textMode, TableModeDefault)

	params := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": msgID,
		"text":       htmlText,
		"parse_mode": "HTML",
	}
	if kb := buildInlineKeyboard(opts.Buttons); kb != nil {
		params["reply_markup"] = kb
	}

	_, err = callTelegramAPI(ctx, client, token, "editMessageText", params)
	if err != nil && parseErrRe.MatchString(err.Error()) {
		// 回退纯文本
		params["text"] = text
		delete(params, "parse_mode")
		_, err = callTelegramAPI(ctx, client, token, "editMessageText", params)
	}
	return err
}

// SendStickerTelegram 发送贴纸。
func SendStickerTelegram(ctx context.Context, cfg *types.OpenAcosmiConfig, to, fileID, accountID string) (*TelegramSendResult, error) {
	if strings.TrimSpace(fileID) == "" {
		return nil, fmt.Errorf("telegram sticker file_id is required")
	}

	account := ResolveTelegramAccount(cfg, accountID)
	token, err := resolveToken("", account)
	if err != nil {
		return nil, err
	}
	target := ParseTelegramTarget(to)
	chatID, err := normalizeChatID(target.ChatID)
	if err != nil {
		return nil, err
	}

	client, err := NewDefaultTelegramHTTPClient(account)
	if err != nil {
		return nil, err
	}

	params := map[string]interface{}{
		"chat_id": chatID,
		"sticker": strings.TrimSpace(fileID),
	}
	if target.MessageThreadID != nil {
		params["message_thread_id"] = *target.MessageThreadID
	}

	res, err := callTelegramAPI(ctx, client, token, "sendSticker", params)
	if err != nil {
		return nil, err
	}

	msgID := "unknown"
	resolvedChat := chatID
	if res != nil {
		msgID = strconv.Itoa(res.MessageID)
		resolvedChat = strconv.FormatInt(res.Chat.ID, 10)
	}
	return &TelegramSendResult{MessageID: msgID, ChatID: resolvedChat}, nil
}

// downloadMediaURL 下载媒体 URL 并返回字节数据、MIME 类型和文件名
func downloadMediaURL(ctx context.Context, client *http.Client, mediaURL string) ([]byte, string, string, error) {
	dlCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("download request build: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20)) // 50MB 限制
	if err != nil {
		return nil, "", "", fmt.Errorf("download read: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	// 从 URL 提取文件名
	fileName := "media"
	if parts := strings.Split(mediaURL, "/"); len(parts) > 0 {
		lastPart := parts[len(parts)-1]
		if qIdx := strings.Index(lastPart, "?"); qIdx > 0 {
			lastPart = lastPart[:qIdx]
		}
		if lastPart != "" && strings.Contains(lastPart, ".") {
			fileName = lastPart
		}
	}

	return data, contentType, fileName, nil
}

// resolveMediaAPIMethod 根据 MIME 类型选择 Telegram API 方法和字段名
func resolveMediaAPIMethod(contentType string) (method string, field string) {
	ct := strings.ToLower(contentType)
	switch {
	case strings.HasPrefix(ct, "image/gif"):
		return "sendAnimation", "animation"
	case strings.HasPrefix(ct, "image/"):
		return "sendPhoto", "photo"
	case strings.HasPrefix(ct, "video/"):
		return "sendVideo", "video"
	case strings.HasPrefix(ct, "audio/ogg"):
		return "sendVoice", "voice"
	case strings.HasPrefix(ct, "audio/"):
		return "sendAudio", "audio"
	default:
		return "sendDocument", "document"
	}
}

// resolveMediaAPIMethodWithFileName 根据 MIME + 文件扩展名选择 API 方法。
// DY-024: 对齐 TS delivery.ts — 同时检查 contentType 和文件扩展名 .gif。
func resolveMediaAPIMethodWithFileName(contentType, fileName string) (method string, field string) {
	// 先按 MIME 判断
	method, field = resolveMediaAPIMethod(contentType)
	// DY-024: 文件扩展名回退 — .gif 文件即使 MIME 不是 image/gif 也作为动画发送
	if method != "sendAnimation" && strings.HasSuffix(strings.ToLower(fileName), ".gif") {
		return "sendAnimation", "animation"
	}
	return method, field
}
