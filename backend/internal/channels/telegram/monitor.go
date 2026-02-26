package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// Telegram Monitor — 继承自 src/telegram/monitor.ts (216L)
// 替代 grammy runner 的长轮询实现

// MonitorConfig 长轮询配置
type MonitorConfig struct {
	Token     string
	AccountID string
	Config    *types.OpenAcosmiConfig
	Deps      *TelegramMonitorDeps
	Handlers  *TelegramHandlerContext

	// DY-014: Webhook 模式支持
	UseWebhook    bool
	WebhookPath   string
	WebhookPort   int
	WebhookSecret string
	WebhookURL    string
}

type getUpdatesResponse struct {
	OK     bool              `json:"ok"`
	Result []json.RawMessage `json:"result,omitempty"`
	Desc   string            `json:"description,omitempty"`
}

// 轮询重启策略 — 对齐 TS TELEGRAM_POLL_RESTART_POLICY
var pollRestartPolicy = struct {
	initialMs float64
	maxMs     float64
	factor    float64
	jitter    float64
}{2000, 30000, 1.8, 0.25}

// DY-014: maxRetryTime — 对齐 TS runner.maxRetryTime: 5 * 60 * 1000
// 如果在此窗口内持续失败，则放弃重试并返回最后的错误。
const maxRetryTime = 5 * time.Minute

// MonitorTelegramProvider 启动 Telegram 长轮询或 Webhook 模式。
// DY-014: 增加 webhook 模式分支和 maxRetryTime 超时机制。
// 完整管线: getUpdates → parse → dedup → dispatch → bot handlers
func MonitorTelegramProvider(ctx context.Context, cfg MonitorConfig) error {
	account := ResolveTelegramAccount(cfg.Config, cfg.AccountID)
	token := cfg.Token
	if token == "" {
		token = account.Token
	}
	if token == "" {
		return fmt.Errorf("telegram bot token missing for account %q", account.AccountID)
	}

	client, err := NewDefaultTelegramHTTPClient(account)
	if err != nil {
		return fmt.Errorf("create HTTP client: %w", err)
	}

	// DY-014: Webhook 模式 — 对齐 TS monitor.ts L153-167
	if cfg.UseWebhook {
		return startWebhookMode(ctx, client, token, cfg)
	}

	// 长轮询模式
	return startPollingMode(ctx, client, token, account, cfg)
}

// startWebhookMode 启动 Webhook HTTP 服务器模式。
// DY-014: 对齐 TS monitor.ts L153-167 startTelegramWebhook 调用。
func startWebhookMode(ctx context.Context, client *http.Client, token string, cfg MonitorConfig) error {
	slog.Info("telegram: starting webhook mode", "account", cfg.AccountID)

	server, err := StartTelegramWebhookServer(ctx, client, WebhookServerConfig{
		Token:     token,
		AccountID: cfg.AccountID,
		Path:      cfg.WebhookPath,
		Port:      cfg.WebhookPort,
		Secret:    cfg.WebhookSecret,
		PublicURL: cfg.WebhookURL,
		Handlers:  cfg.Handlers,
	})
	if err != nil {
		return fmt.Errorf("start webhook server: %w", err)
	}

	// 等待 context 取消
	<-ctx.Done()

	// 优雅关闭
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Warn("telegram: webhook server shutdown error", "err", err)
	}

	return ctx.Err()
}

// startPollingMode 启动长轮询模式。
// DY-014: 增加 maxRetryTime 超时机制和异常恢复。
func startPollingMode(ctx context.Context, client *http.Client, token string, account ResolvedTelegramAccount, cfg MonitorConfig) error {
	lastUpdateID, _ := ReadTelegramUpdateOffset(account.AccountID)
	allowedUpdates := AllowedUpdates()
	restartAttempts := 0

	// DY-014: maxRetryTime — 记录首次连续失败的时间
	var firstFailureTime *time.Time

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := pollOnce(ctx, client, token, &lastUpdateID, allowedUpdates, account.AccountID, cfg.Handlers)
		if err == nil {
			restartAttempts = 0
			firstFailureTime = nil // 成功时重置失败计时器
			continue
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		isConflict := isGetUpdatesConflict(err)
		isRecoverable := IsRecoverableTelegramNetworkError(err, NetworkCtxPolling)

		if !isRecoverable && !isConflict {
			return err
		}

		// DY-014: maxRetryTime 检查 — 对齐 TS runner.maxRetryTime: 5min
		now := time.Now()
		if firstFailureTime == nil {
			firstFailureTime = &now
		}
		if now.Sub(*firstFailureTime) > maxRetryTime {
			slog.Error("telegram poll: max retry time exceeded, giving up",
				"maxRetryTime", maxRetryTime,
				"totalAttempts", restartAttempts,
				"lastError", err,
			)
			return fmt.Errorf("telegram poll: max retry time (%v) exceeded: %w", maxRetryTime, err)
		}

		restartAttempts++
		delayMs := computeBackoff(pollRestartPolicy.initialMs, pollRestartPolicy.maxMs,
			pollRestartPolicy.factor, pollRestartPolicy.jitter, restartAttempts)

		reason := "network error"
		if isConflict {
			reason = "getUpdates conflict"
		}

		slog.Warn("telegram poll error, retrying",
			"reason", reason,
			"err", err,
			"retryIn", fmt.Sprintf("%dms", delayMs),
			"attempt", restartAttempts,
			"timeSinceFirstFailure", time.Since(*firstFailureTime).Round(time.Second),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(delayMs) * time.Millisecond):
		}
	}
}

// pollOnce 执行一次长轮询请求。
// DY-014: 增加 panic recovery 保护。
func pollOnce(ctx context.Context, client *http.Client, token string, lastUpdateID **int, allowedUpdates []string, accountID string, handlers *TelegramHandlerContext) (retErr error) {
	// DY-014: panic recovery — 对齐 TS registerUnhandledRejectionHandler
	defer func() {
		if r := recover(); r != nil {
			slog.Error("telegram pollOnce panic recovered", "panic", fmt.Sprintf("%v", r))
			retErr = fmt.Errorf("telegram pollOnce panic: %v", r)
		}
	}()

	params := url.Values{}
	params.Set("timeout", "30")
	if *lastUpdateID != nil {
		params.Set("offset", fmt.Sprintf("%d", **lastUpdateID+1))
	}
	for _, u := range allowedUpdates {
		params.Add("allowed_updates", u)
	}

	apiURL := fmt.Sprintf("%s/bot%s/getUpdates?%s", TelegramAPIBaseURL, token, params.Encode())

	pollCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(pollCtx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result getUpdatesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("getUpdates decode: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("%d: %s", resp.StatusCode, result.Desc)
	}

	for _, raw := range result.Result {
		var update TelegramUpdate
		if err := json.Unmarshal(raw, &update); err != nil {
			slog.Warn("telegram: failed to parse update", "err", err)
			continue
		}
		if update.UpdateID > 0 {
			id := update.UpdateID
			*lastUpdateID = &id
		}
		// 分发到 bot handler（带 panic recovery）
		if handlers != nil {
			safeHandleUpdate(ctx, handlers, &update)
		}
	}

	// 持久化 offset
	if *lastUpdateID != nil {
		if err := WriteTelegramUpdateOffset(accountID, **lastUpdateID); err != nil {
			slog.Warn("telegram: failed to persist update offset", "err", err)
		}
	}

	return nil
}

// safeHandleUpdate 以 panic-safe 方式处理单个更新。
// DY-014: 对齐 TS registerUnhandledRejectionHandler 的异常隔离机制。
func safeHandleUpdate(ctx context.Context, handlers *TelegramHandlerContext, update *TelegramUpdate) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("telegram: update handler panic recovered",
				"updateId", update.UpdateID,
				"panic", fmt.Sprintf("%v", r),
			)
		}
	}()
	handlers.HandleUpdate(ctx, update)
}

// isGetUpdatesConflict 检查是否为 getUpdates 409 冲突错误。
// DY-014: 增强检查逻辑，对齐 TS monitor.ts 中检查 error_code + description + method。
func isGetUpdatesConflict(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	// 检查 409 状态码 + conflict/terminated 关键词
	return strings.HasPrefix(s, "409") ||
		(strings.Contains(s, "conflict") && strings.Contains(s, "terminated")) ||
		(strings.Contains(s, "409") && strings.Contains(s, "getupdates"))
}

// computeBackoff 计算指数退避延迟（带单极性抖动）。
// DY-014: 对齐 TS retryInterval 单极性抖动: delay * (1 + jitter * rand)
// TS 原版 jitter 范围 [0, jitter*delay]，Go 之前是双极性 [-jitter*delay, +jitter*delay]。
func computeBackoff(initialMs, maxMs, factor, jitter float64, attempt int) int {
	delay := initialMs * math.Pow(factor, float64(attempt-1))
	if delay > maxMs {
		delay = maxMs
	}
	// 单极性抖动: [0, jitter*delay]
	j := delay * jitter * rand.Float64()
	result := delay + j
	if result > maxMs {
		result = maxMs
	}
	return int(result)
}
