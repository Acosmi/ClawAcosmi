package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/pkg/retry"
)

// Discord REST API 客户端 — 继承自 src/discord/api.ts (137L)

const discordAPIBase = "https://discord.com/api/v10"

// discordAPIRetryDefaults Discord API 重试默认值
var discordAPIRetryDefaults = retry.Config{
	MaxAttempts:  3,
	InitialDelay: 500 * time.Millisecond,
	MaxDelay:     30 * time.Second,
	JitterFactor: 0.1,
}

// DiscordAPIError Discord API 错误
type DiscordAPIError struct {
	StatusCode int
	RetryAfter *float64 // 秒
	Message    string
}

func (e *DiscordAPIError) Error() string {
	return e.Message
}

// discordAPIErrorPayload Discord API 错误响应体
type discordAPIErrorPayload struct {
	Message    string   `json:"message,omitempty"`
	RetryAfter *float64 `json:"retry_after,omitempty"`
	Code       *int     `json:"code,omitempty"`
	Global     *bool    `json:"global,omitempty"`
}

// parseDiscordAPIErrorPayload 解析 Discord API 错误响应体
func parseDiscordAPIErrorPayload(text string) *discordAPIErrorPayload {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		return nil
	}
	var payload discordAPIErrorPayload
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil
	}
	return &payload
}

// parseRetryAfterSeconds 从响应体和 header 中提取 Retry-After 秒数
func parseRetryAfterSeconds(text string, resp *http.Response) *float64 {
	payload := parseDiscordAPIErrorPayload(text)
	if payload != nil && payload.RetryAfter != nil && math.IsInf(*payload.RetryAfter, 0) == false && !math.IsNaN(*payload.RetryAfter) {
		return payload.RetryAfter
	}
	header := resp.Header.Get("Retry-After")
	if header == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(header, 64)
	if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
		return nil
	}
	return &parsed
}

// formatRetryAfterSeconds 格式化重试等待时间
func formatRetryAfterSeconds(value *float64) string {
	if value == nil || math.IsInf(*value, 0) || math.IsNaN(*value) || *value < 0 {
		return ""
	}
	if *value < 10 {
		return fmt.Sprintf("%.1fs", *value)
	}
	return fmt.Sprintf("%ds", int(math.Round(*value)))
}

// formatDiscordAPIErrorText 格式化 Discord API 错误文本
func formatDiscordAPIErrorText(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	payload := parseDiscordAPIErrorPayload(trimmed)
	if payload == nil {
		looksJSON := strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")
		if looksJSON {
			return "unknown error"
		}
		return trimmed
	}
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = "unknown error"
	}
	retryAfter := formatRetryAfterSeconds(payload.RetryAfter)
	if retryAfter != "" {
		return fmt.Sprintf("%s (retry after %s)", message, retryAfter)
	}
	return message
}

// DiscordFetchOptions Discord API 请求选项
// DY-011: 添加 Client 字段，对齐 TS `fetchDiscord(path, token, fetcher = fetch)` 的可注入 fetch。
type DiscordFetchOptions struct {
	Retry  *retry.Config
	Label  string
	Client *http.Client // 自定义 HTTP Client，nil 时使用 http.DefaultClient
}

// mergeRetryConfig 逐字段合并重试配置：override 的非零值覆盖 base。
// 对齐 TS resolveRetryConfig(defaults, override) 的逐字段深度合并策略。
// DY-010: 修复原来 `retryCfg = *opts.Retry` 整体替换丢失 base 默认值的问题。
func mergeRetryConfig(base retry.Config, override *retry.Config) retry.Config {
	if override == nil {
		return base
	}
	merged := base
	if override.MaxAttempts > 0 {
		merged.MaxAttempts = override.MaxAttempts
	}
	if override.InitialDelay > 0 {
		merged.InitialDelay = override.InitialDelay
	}
	if override.MaxDelay > 0 {
		merged.MaxDelay = override.MaxDelay
	}
	if override.Multiplier > 0 {
		merged.Multiplier = override.Multiplier
	}
	if override.JitterFactor > 0 {
		merged.JitterFactor = override.JitterFactor
	}
	if override.ShouldRetry != nil {
		merged.ShouldRetry = override.ShouldRetry
	}
	if override.OnRetry != nil {
		merged.OnRetry = override.OnRetry
	}
	if override.RetryAfterHint != nil {
		merged.RetryAfterHint = override.RetryAfterHint
	}
	if override.Label != "" {
		merged.Label = override.Label
	}
	return merged
}

// FetchDiscord 执行 Discord API GET 请求并反序列化响应。
// 自动处理 429 限速重试。path 应以 "/" 开头，例如 "/users/@me"。
func FetchDiscord[T any](ctx context.Context, path string, token string, opts *DiscordFetchOptions) (T, error) {
	retryCfg := discordAPIRetryDefaults
	label := path
	if opts != nil {
		retryCfg = mergeRetryConfig(retryCfg, opts.Retry)
		if opts.Label != "" {
			label = opts.Label
		}
	}

	retryCfg.Label = label
	retryCfg.ShouldRetry = func(err error, _ int) bool {
		if apiErr, ok := err.(*DiscordAPIError); ok {
			return apiErr.StatusCode == http.StatusTooManyRequests
		}
		return false
	}
	retryCfg.RetryAfterHint = func(err error) time.Duration {
		if apiErr, ok := err.(*DiscordAPIError); ok && apiErr.RetryAfter != nil {
			return time.Duration(*apiErr.RetryAfter * float64(time.Second))
		}
		return 0
	}

	// DY-011: 使用注入的 HTTP Client（若有），否则回退到 http.DefaultClient
	httpClient := http.DefaultClient
	if opts != nil && opts.Client != nil {
		httpClient = opts.Client
	}

	return retry.DoWithResult(ctx, retryCfg, func(_ int) (T, error) {
		var zero T
		url := discordAPIBase + path

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return zero, fmt.Errorf("discord api: create request: %w", err)
		}
		req.Header.Set("Authorization", "Bot "+token)

		resp, err := httpClient.Do(req)
		if err != nil {
			return zero, fmt.Errorf("discord api: request failed: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			text := string(body) // 即使 body 读取部分失败也使用已读内容（对齐 TS res.text().catch(() => "")）
			detail := formatDiscordAPIErrorText(text)
			suffix := ""
			if detail != "" {
				suffix = ": " + detail
			}
			var retryAfter *float64
			if resp.StatusCode == http.StatusTooManyRequests {
				retryAfter = parseRetryAfterSeconds(text, resp)
			}
			return zero, &DiscordAPIError{
				StatusCode: resp.StatusCode,
				RetryAfter: retryAfter,
				Message:    fmt.Sprintf("Discord API %s failed (%d)%s", path, resp.StatusCode, suffix),
			}
		}

		if len(body) == 0 {
			return zero, fmt.Errorf("discord api: empty response body")
		}
		var result T
		if err := json.Unmarshal(body, &result); err != nil {
			return zero, fmt.Errorf("discord api: decode response: %w", err)
		}
		return result, nil
	})
}
