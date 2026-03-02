package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/openacosmi/claw-acismi/pkg/retry"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// Discord 发送共享工具 — 继承自 src/discord/send.shared.ts (424L)
// 替换 @buape/carbon 的 RequestClient 为原生 net/http

const (
	discordTextLimit         = 2000
	discordMaxStickers       = 3
	discordMissingPermission = 50013
	discordCannotDM          = 50007
)

// DiscordRecipient 接收方
type DiscordRecipient struct {
	Kind DiscordTargetKind
	ID   string
}

// DiscordClientOpts Discord 客户端选项
type DiscordClientOpts struct {
	Token     string
	AccountID string
	Retry     *retry.Config
	Verbose   bool
}

// DiscordClient 解析后的 Discord 客户端
type DiscordClient struct {
	Token     string
	AccountID string
	RetryCfg  retry.Config
}

// NewDiscordClient 创建 Discord 客户端
func NewDiscordClient(cfg *types.OpenAcosmiConfig, opts DiscordClientOpts) (DiscordClient, error) {
	account := ResolveDiscordAccount(cfg, opts.AccountID)
	token, err := resolveClientToken(opts.Token, account.AccountID, account.Token)
	if err != nil {
		return DiscordClient{}, err
	}
	retryCfg := mergeRetryConfig(discordAPIRetryDefaults, opts.Retry)
	return DiscordClient{
		Token:     token,
		AccountID: account.AccountID,
		RetryCfg:  retryCfg,
	}, nil
}

func resolveClientToken(explicit, accountID, fallbackToken string) (string, error) {
	t := NormalizeDiscordToken(explicit)
	if t != "" {
		return t, nil
	}
	fb := NormalizeDiscordToken(fallbackToken)
	if fb == "" {
		return "", fmt.Errorf(
			"discord bot token missing for account %q (set discord.accounts.%s.token or DISCORD_BOT_TOKEN for default)",
			accountID, accountID)
	}
	return fb, nil
}

// discordREST 执行 Discord REST API 请求（非 GET）
func discordREST(ctx context.Context, method, path, token string, body interface{}) (json.RawMessage, error) {
	return discordRESTWithHeaders(ctx, method, path, token, body, nil)
}

// discordRESTWithHeaders 执行 Discord REST API 请求，支持额外 header。
func discordRESTWithHeaders(ctx context.Context, method, path, token string, body interface{}, extraHeaders map[string]string) (json.RawMessage, error) {
	apiURL := discordAPIBase + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("discord rest: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, apiURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("discord rest: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discord rest: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("discord rest: read body: %w", err)
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
			Message:    fmt.Sprintf("Discord API %s %s failed (%d)%s", method, path, resp.StatusCode, suffix),
		}
	}

	return json.RawMessage(respBody), nil
}

// discordPOST 执行 POST 请求
func discordPOST(ctx context.Context, path, token string, body interface{}) (json.RawMessage, error) {
	return discordREST(ctx, http.MethodPost, path, token, body)
}

// discordPATCH 执行 PATCH 请求
func discordPATCH(ctx context.Context, path, token string, body interface{}) (json.RawMessage, error) {
	return discordREST(ctx, http.MethodPatch, path, token, body)
}

// discordPUT 执行 PUT 请求
func discordPUT(ctx context.Context, path, token string, body interface{}) (json.RawMessage, error) {
	return discordREST(ctx, http.MethodPut, path, token, body)
}

// discordDELETE 执行 DELETE 请求
func discordDELETE(ctx context.Context, path, token string) error {
	_, err := discordREST(ctx, http.MethodDelete, path, token, nil)
	return err
}

// discordGET 执行 GET 请求（非泛型版本）
func discordGET(ctx context.Context, path, token string) (json.RawMessage, error) {
	return discordREST(ctx, http.MethodGet, path, token, nil)
}

// ── DY-005: retry wrapper — 对齐 TS createDiscordRetryRunner ──────────

// discordRESTWithRetry 执行带重试的 Discord REST API 请求。
// 对齐 TS: createDiscordRetryRunner (infra/retry-policy.ts)
//   - shouldRetry: 仅在 429 (RateLimitError) 或 5xx 时重试
//   - retryAfterMs: 使用 DiscordAPIError.RetryAfter
func discordRESTWithRetry(ctx context.Context, method, path, token string, body interface{}, label string) (json.RawMessage, error) {
	cfg := discordAPIRetryDefaults
	cfg.Label = label
	cfg.ShouldRetry = func(err error, _ int) bool {
		if apiErr, ok := err.(*DiscordAPIError); ok {
			return apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= 500
		}
		return false
	}
	cfg.RetryAfterHint = func(err error) time.Duration {
		if apiErr, ok := err.(*DiscordAPIError); ok && apiErr.RetryAfter != nil {
			return time.Duration(*apiErr.RetryAfter * float64(time.Second))
		}
		return 0
	}

	return retry.DoWithResult(ctx, cfg, func(_ int) (json.RawMessage, error) {
		return discordREST(ctx, method, path, token, body)
	})
}

// discordPOSTWithRetry 执行带重试的 POST 请求。
func discordPOSTWithRetry(ctx context.Context, path, token string, body interface{}, label string) (json.RawMessage, error) {
	return discordRESTWithRetry(ctx, http.MethodPost, path, token, body, label)
}

// discordMultipartPOSTWithRetry 执行带重试的 multipart POST 请求。
func discordMultipartPOSTWithRetry(ctx context.Context, path, token string, payload interface{}, media *discordMedia, label string) (json.RawMessage, error) {
	cfg := discordAPIRetryDefaults
	cfg.Label = label
	cfg.ShouldRetry = func(err error, _ int) bool {
		if apiErr, ok := err.(*DiscordAPIError); ok {
			return apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= 500
		}
		return false
	}
	cfg.RetryAfterHint = func(err error) time.Duration {
		if apiErr, ok := err.(*DiscordAPIError); ok && apiErr.RetryAfter != nil {
			return time.Duration(*apiErr.RetryAfter * float64(time.Second))
		}
		return 0
	}

	return retry.DoWithResult(ctx, cfg, func(_ int) (json.RawMessage, error) {
		return discordMultipartPOST(ctx, path, token, payload, media)
	})
}

var customEmojiRe = regexp.MustCompile(`^<a?:([^:>]+):(\d+)>$`)
var variationSelectorRe = regexp.MustCompile(`[\x{FE0E}\x{FE0F}]`)

// NormalizeReactionEmoji 规范化反应 emoji
func NormalizeReactionEmoji(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("emoji required")
	}
	if m := customEmojiRe.FindStringSubmatch(trimmed); m != nil {
		return url.PathEscape(m[1] + ":" + m[2]), nil
	}
	return url.PathEscape(variationSelectorRe.ReplaceAllString(trimmed, "")), nil
}

// ParseRecipient 解析接收方
func ParseRecipient(raw string) (DiscordRecipient, error) {
	target, err := ParseDiscordTarget(raw, DiscordTargetParseOptions{
		AmbiguousMessage: fmt.Sprintf(
			`Ambiguous Discord recipient "%s". Use "user:%s" for DMs or "channel:%s" for channel messages.`,
			strings.TrimSpace(raw), strings.TrimSpace(raw), strings.TrimSpace(raw)),
	})
	if err != nil {
		return DiscordRecipient{}, err
	}
	if target == nil {
		return DiscordRecipient{}, fmt.Errorf("recipient is required for Discord sends")
	}
	return DiscordRecipient{Kind: target.Kind, ID: target.ID}, nil
}

// NormalizeStickerIds 规范化 sticker ID 列表
func NormalizeStickerIds(raw []string) ([]string, error) {
	var ids []string
	for _, entry := range raw {
		id := strings.TrimSpace(entry)
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("at least one sticker id is required")
	}
	if len(ids) > discordMaxStickers {
		return nil, fmt.Errorf("Discord supports up to 3 stickers per message")
	}
	return ids, nil
}

// NormalizeEmojiName 规范化 emoji 名称
func NormalizeEmojiName(raw, label string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return name, nil
}

// BuildReactionIdentifier 构建反应标识符
func BuildReactionIdentifier(emojiID, emojiName string) string {
	if emojiID != "" && emojiName != "" {
		return emojiName + ":" + emojiID
	}
	return emojiName
}

// FormatReactionEmoji 格式化 emoji 为显示字符串（BuildReactionIdentifier 别名）
// 继承自 send.shared.ts formatReactionEmoji (L405-407)
func FormatReactionEmoji(emojiID, emojiName string) string {
	return BuildReactionIdentifier(emojiID, emojiName)
}

// ResolveChannelIDFromRecipient 从接收方解析频道 ID
func ResolveChannelIDFromRecipient(ctx context.Context, token string, recipient DiscordRecipient) (channelID string, isDM bool, err error) {
	if recipient.Kind == DiscordTargetKindChannel {
		return recipient.ID, false, nil
	}
	// DY-005: 为用户创建 DM 频道（带重试）
	body := map[string]string{"recipient_id": recipient.ID}
	resp, err := discordPOSTWithRetry(ctx, "/users/@me/channels", token, body, "dm-channel")
	if err != nil {
		return "", false, fmt.Errorf("failed to create Discord DM channel: %w", err)
	}
	var dmChannel struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &dmChannel); err != nil {
		return "", false, fmt.Errorf("failed to parse DM channel: %w", err)
	}
	if dmChannel.ID == "" {
		return "", false, fmt.Errorf("failed to create Discord DM channel")
	}
	return dmChannel.ID, true, nil
}

// GetDiscordErrorCode 从错误中提取 Discord 错误码
func GetDiscordErrorCode(err error) int {
	if apiErr, ok := err.(*DiscordAPIError); ok {
		return apiErr.StatusCode
	}
	return 0
}

// BuildDiscordSendErrorFromErr 从错误构建 DiscordSendError。
// 当遇到 403 (Missing Permissions) 错误时，会异步探测频道权限以提供更详细的错误信息。
// ctxAndToken 可变参数顺序: [0]=context.Context, [1]=token string, [2]=hasMedia bool
// DY-004: 权限探测列表对齐 TS — 动态添加 SendMessagesInThreads 和 AttachFiles。
func BuildDiscordSendErrorFromErr(err error, channelID string, ctxAndToken ...interface{}) error {
	if _, ok := err.(*DiscordSendError); ok {
		return err
	}
	code := GetDiscordErrorCode(err)
	if code == discordCannotDM {
		return &DiscordSendError{
			Kind:    DiscordSendErrorKindDMBlocked,
			Message: "discord dm failed: user blocks dms or privacy settings disallow it",
		}
	}
	if code == discordMissingPermission {
		// Probe channel permissions to provide a more detailed error message
		var missing []string

		// 解析可变参数中的 hasMedia 标志
		hasMedia := false
		if len(ctxAndToken) >= 3 {
			if hm, ok := ctxAndToken[2].(bool); ok {
				hasMedia = hm
			}
		}

		if len(ctxAndToken) >= 2 {
			if ctx, ok := ctxAndToken[0].(context.Context); ok {
				if token, ok := ctxAndToken[1].(string); ok && token != "" {
					if summary, probeErr := FetchChannelPermissionsDiscord(ctx, channelID, token); probeErr == nil && summary != nil {
						// DY-004: 对齐 TS 权限探测列表 (send.shared.ts L235-242)
						requiredPerms := []string{"ViewChannel", "SendMessages"}
						if IsThreadChannelType(summary.ChannelType) {
							requiredPerms = append(requiredPerms, "SendMessagesInThreads")
						}
						if hasMedia {
							requiredPerms = append(requiredPerms, "AttachFiles")
						}
						for _, perm := range requiredPerms {
							if !sendPermissionsContains(summary.Permissions, perm) {
								missing = append(missing, perm)
							}
						}
					}
				}
			}
		}
		msg := fmt.Sprintf("missing permissions in channel %s. bot might be muted or blocked by role/channel overrides", channelID)
		if len(missing) > 0 {
			msg = fmt.Sprintf("missing permissions in channel %s: %s", channelID, strings.Join(missing, ", "))
		}
		return &DiscordSendError{
			Kind:               DiscordSendErrorKindMissingPerms,
			ChannelID:          channelID,
			MissingPermissions: missing,
			Message:            msg,
		}
	}
	return err
}

// SendDiscordTextOpts 发送文本消息选项
type SendDiscordTextOpts struct {
	Embeds    []interface{}
	ChunkMode ChunkMode
}

// SendDiscordText 发送纯文本消息（分块）
// 继承自 send.shared.ts sendDiscordText (L281-337)
func SendDiscordText(ctx context.Context, token, channelID, text string, replyTo string, maxLinesPerMessage int, opts ...SendDiscordTextOpts) (*DiscordSendResult, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("message must be non-empty for Discord sends")
	}

	var embeds []interface{}
	var chunkMode ChunkMode
	if len(opts) > 0 {
		embeds = opts[0].Embeds
		chunkMode = opts[0].ChunkMode
	}

	chunks := ChunkDiscordTextWithMode(text, ChunkDiscordTextOpts{
		MaxChars: discordTextLimit,
		MaxLines: maxLinesPerMessage,
	}, chunkMode)
	if len(chunks) == 0 && text != "" {
		chunks = []string{text}
	}

	var last *DiscordSendResult
	isFirst := true
	for _, chunk := range chunks {
		body := map[string]interface{}{
			"content": chunk,
		}
		if isFirst && replyTo != "" {
			body["message_reference"] = map[string]interface{}{
				"message_id":         replyTo,
				"fail_if_not_exists": false,
			}
		}
		if isFirst && len(embeds) > 0 {
			body["embeds"] = embeds
		}
		// DY-005: 带重试的消息发送
		resp, err := discordPOSTWithRetry(ctx, fmt.Sprintf("/channels/%s/messages", channelID), token, body, "text")
		if err != nil {
			return nil, err
		}
		var msg struct {
			ID        string `json:"id"`
			ChannelID string `json:"channel_id"`
		}
		if err := json.Unmarshal(resp, &msg); err != nil {
			return nil, fmt.Errorf("discord send: decode response: %w", err)
		}
		last = &DiscordSendResult{
			MessageID: msg.ID,
			ChannelID: msg.ChannelID,
		}
		isFirst = false
	}

	if last == nil {
		return nil, fmt.Errorf("discord send failed (empty chunk result)")
	}
	return last, nil
}

// SendDiscordMedia 发送带文件附件的消息
// 继承自 send.shared.ts sendDiscordMedia (L339-396)
// 第一条消息使用 multipart/form-data 附带文件，后续分块用 SendDiscordText
func SendDiscordMedia(ctx context.Context, token, channelID, text, mediaURL string, replyTo string, maxLinesPerMessage int, opts ...SendDiscordTextOpts) (*DiscordSendResult, error) {
	media, err := loadDiscordMedia(mediaURL)
	if err != nil {
		return nil, fmt.Errorf("discord send media: %w", err)
	}

	var embeds []interface{}
	var chunkMode ChunkMode
	if len(opts) > 0 {
		embeds = opts[0].Embeds
		chunkMode = opts[0].ChunkMode
	}

	var chunks []string
	if text != "" {
		chunks = ChunkDiscordTextWithMode(text, ChunkDiscordTextOpts{
			MaxChars: discordTextLimit,
			MaxLines: maxLinesPerMessage,
		}, chunkMode)
	}
	if len(chunks) == 0 && text != "" {
		chunks = []string{text}
	}

	// 第一条消息带文件附件
	caption := ""
	if len(chunks) > 0 {
		caption = chunks[0]
	}

	payload := map[string]interface{}{}
	if caption != "" {
		payload["content"] = caption
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

	// DY-005: 带重试的媒体消息发送
	resp, err := discordMultipartPOSTWithRetry(ctx, fmt.Sprintf("/channels/%s/messages", channelID), token, payload, media, "media")
	if err != nil {
		return nil, err
	}
	var msg struct {
		ID        string `json:"id"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal(resp, &msg); err != nil {
		return nil, fmt.Errorf("discord send media: decode response: %w", err)
	}
	result := &DiscordSendResult{
		MessageID: msg.ID,
		ChannelID: msg.ChannelID,
	}

	// 后续分块用纯文本发送
	for _, chunk := range chunks[1:] {
		if strings.TrimSpace(chunk) == "" {
			continue
		}
		_, err := SendDiscordText(ctx, token, channelID, chunk, "", maxLinesPerMessage)
		if err != nil {
			return result, fmt.Errorf("discord send media follow-up chunk: %w", err)
		}
	}

	return result, nil
}
