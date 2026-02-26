package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Discord Bot Probe — 继承自 src/discord/probe.ts (194L)

// DiscordPrivilegedIntentStatus 特权意图状态
type DiscordPrivilegedIntentStatus string

const (
	IntentEnabled  DiscordPrivilegedIntentStatus = "enabled"
	IntentLimited  DiscordPrivilegedIntentStatus = "limited"
	IntentDisabled DiscordPrivilegedIntentStatus = "disabled"
)

// DiscordPrivilegedIntentsSummary 特权意图摘要
type DiscordPrivilegedIntentsSummary struct {
	MessageContent DiscordPrivilegedIntentStatus `json:"messageContent"`
	GuildMembers   DiscordPrivilegedIntentStatus `json:"guildMembers"`
	Presence       DiscordPrivilegedIntentStatus `json:"presence"`
}

// DiscordApplicationSummary 应用摘要
type DiscordApplicationSummary struct {
	ID      string                           `json:"id,omitempty"`
	Flags   *int                             `json:"flags,omitempty"`
	Intents *DiscordPrivilegedIntentsSummary `json:"intents,omitempty"`
}

// DiscordProbe Bot 探测结果
type DiscordProbe struct {
	OK          bool                       `json:"ok"`
	Status      *int                       `json:"status,omitempty"`
	Error       string                     `json:"error,omitempty"`
	ElapsedMs   int64                      `json:"elapsedMs"`
	Bot         *DiscordBotInfo            `json:"bot,omitempty"`
	Application *DiscordApplicationSummary `json:"application,omitempty"`
}

// DiscordBotInfo Bot 信息
type DiscordBotInfo struct {
	ID       string `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
}

// ProbeOptions 探测选项，支持 HTTP Client 注入。
type ProbeOptions struct {
	HTTPClient         *http.Client
	IncludeApplication bool
}

// Discord Application Flag 位定义
const (
	discordAppFlagGatewayPresence              = 1 << 12
	discordAppFlagGatewayPresenceLimited       = 1 << 13
	discordAppFlagGatewayGuildMembers          = 1 << 14
	discordAppFlagGatewayGuildMembersLimited   = 1 << 15
	discordAppFlagGatewayMessageContent        = 1 << 18
	discordAppFlagGatewayMessageContentLimited = 1 << 19
)

// ResolveDiscordPrivilegedIntentsFromFlags 从应用标志解析特权意图状态
func ResolveDiscordPrivilegedIntentsFromFlags(flags int) DiscordPrivilegedIntentsSummary {
	resolve := func(enabledBit, limitedBit int) DiscordPrivilegedIntentStatus {
		if flags&enabledBit != 0 {
			return IntentEnabled
		}
		if flags&limitedBit != 0 {
			return IntentLimited
		}
		return IntentDisabled
	}
	return DiscordPrivilegedIntentsSummary{
		Presence:       resolve(discordAppFlagGatewayPresence, discordAppFlagGatewayPresenceLimited),
		GuildMembers:   resolve(discordAppFlagGatewayGuildMembers, discordAppFlagGatewayGuildMembersLimited),
		MessageContent: resolve(discordAppFlagGatewayMessageContent, discordAppFlagGatewayMessageContentLimited),
	}
}

// fetchWithTimeout 执行带超时的 HTTP GET 请求。client 为 nil 时使用 http.DefaultClient。
func fetchWithTimeout(ctx context.Context, url string, timeoutMs int, headers map[string]string, client *http.Client) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	c := client
	if c == nil {
		c = http.DefaultClient
	}
	return c.Do(req)
}

// FetchDiscordApplicationSummary 获取 Discord 应用摘要。client 为 nil 时使用 http.DefaultClient。
func FetchDiscordApplicationSummary(ctx context.Context, token string, timeoutMs int, client *http.Client) *DiscordApplicationSummary {
	normalized := NormalizeDiscordToken(token)
	if normalized == "" {
		return nil
	}

	resp, err := fetchWithTimeout(ctx, discordAPIBase+"/oauth2/applications/@me", timeoutMs, map[string]string{
		"Authorization": "Bot " + normalized,
	}, client)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}

	var data struct {
		ID    string `json:"id"`
		Flags *int   `json:"flags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil
	}

	summary := &DiscordApplicationSummary{
		ID:    data.ID,
		Flags: data.Flags,
	}
	if data.Flags != nil {
		intents := ResolveDiscordPrivilegedIntentsFromFlags(*data.Flags)
		summary.Intents = &intents
	}
	return summary
}

// ProbeDiscord 探测 Discord Bot Token 有效性。
// opts 为 nil 时使用默认选项（http.DefaultClient，不包含 Application）。
func ProbeDiscord(ctx context.Context, token string, timeoutMs int, opts *ProbeOptions) DiscordProbe {
	started := time.Now()
	normalized := NormalizeDiscordToken(token)

	var client *http.Client
	var includeApplication bool
	if opts != nil {
		client = opts.HTTPClient
		includeApplication = opts.IncludeApplication
	}

	if normalized == "" {
		return DiscordProbe{
			OK:        false,
			Error:     "missing token",
			ElapsedMs: time.Since(started).Milliseconds(),
		}
	}

	resp, err := fetchWithTimeout(ctx, discordAPIBase+"/users/@me", timeoutMs, map[string]string{
		"Authorization": "Bot " + normalized,
	}, client)
	if err != nil {
		return DiscordProbe{
			OK:        false,
			Error:     err.Error(),
			ElapsedMs: time.Since(started).Milliseconds(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status := resp.StatusCode
		return DiscordProbe{
			OK:        false,
			Status:    &status,
			Error:     fmt.Sprintf("getMe failed (%d)", resp.StatusCode),
			ElapsedMs: time.Since(started).Milliseconds(),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return DiscordProbe{
			OK:        false,
			Error:     err.Error(),
			ElapsedMs: time.Since(started).Milliseconds(),
		}
	}

	var botData struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(body, &botData); err != nil {
		return DiscordProbe{
			OK:        false,
			Error:     err.Error(),
			ElapsedMs: time.Since(started).Milliseconds(),
		}
	}

	result := DiscordProbe{
		OK:        true,
		Bot:       &DiscordBotInfo{ID: botData.ID, Username: botData.Username},
		ElapsedMs: time.Since(started).Milliseconds(),
	}

	if includeApplication {
		result.Application = FetchDiscordApplicationSummary(ctx, normalized, timeoutMs, client)
	}

	return result
}

// FetchDiscordApplicationId 获取 Discord 应用 ID。client 为 nil 时使用 http.DefaultClient。
func FetchDiscordApplicationId(ctx context.Context, token string, timeoutMs int, client *http.Client) string {
	summary := FetchDiscordApplicationSummary(ctx, token, timeoutMs, client)
	if summary == nil {
		return ""
	}
	return summary.ID
}
