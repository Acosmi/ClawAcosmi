package telegram

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Telegram Webhook 管理 — 继承自 src/telegram/webhook-set.ts (42L) + webhook.ts (128L)

// SetTelegramWebhook 设置 Telegram Bot webhook URL
func SetTelegramWebhook(ctx context.Context, client *http.Client, token, webhookURL, secret string, dropPending bool) error {
	if client == nil {
		client = http.DefaultClient
	}
	params := map[string]interface{}{
		"url":                  webhookURL,
		"drop_pending_updates": dropPending,
		"allowed_updates":      AllowedUpdates(),
	}
	if secret != "" {
		params["secret_token"] = secret
	}
	_, err := WithTelegramAPIErrorLogging("setWebhook", func() (*apiMessage, error) {
		return callTelegramAPI(ctx, client, token, "setWebhook", params)
	}, nil)
	return err
}

// DeleteTelegramWebhook 删除 Telegram Bot webhook
func DeleteTelegramWebhook(ctx context.Context, client *http.Client, token string) error {
	_, err := WithTelegramAPIErrorLogging("deleteWebhook", func() (*apiMessage, error) {
		return callTelegramAPI(ctx, client, token, "deleteWebhook", nil)
	}, nil)
	return err
}

// WebhookServerConfig Webhook HTTP 服务器配置
type WebhookServerConfig struct {
	Token      string
	AccountID  string
	Path       string
	HealthPath string
	Port       int
	Host       string
	Secret     string
	PublicURL  string
	Handlers   *TelegramHandlerContext
}

// StartTelegramWebhookServer 启动 Telegram Webhook HTTP 服务器。
// 完整管线: HTTP POST → secret 验证 → parse update → dispatch → bot handlers
func StartTelegramWebhookServer(ctx context.Context, client *http.Client, cfg WebhookServerConfig) (*http.Server, error) {
	path := cfg.Path
	if path == "" {
		path = "/telegram-webhook"
	}
	healthPath := cfg.HealthPath
	if healthPath == "" {
		healthPath = "/healthz"
	}
	port := cfg.Port
	if port == 0 {
		port = 8787
	}
	host := cfg.Host
	if host == "" {
		host = "0.0.0.0"
	}

	mux := http.NewServeMux()
	mux.HandleFunc(healthPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Secret token 验证（使用等时比较防止时序攻击）
		if cfg.Secret != "" {
			headerSecret := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
			if !hmac.Equal([]byte(headerSecret), []byte(cfg.Secret)) {
				slog.Warn("telegram webhook: invalid secret token")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		// 读取并解析 update
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB 限制
		if err != nil {
			slog.Warn("telegram webhook: body read error", "err", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var update TelegramUpdate
		if err := json.Unmarshal(body, &update); err != nil {
			slog.Warn("telegram webhook: JSON parse error", "err", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// 分发到 bot handler（带 panic recovery，对齐 TS .catch → 500 + monitor.safeHandleUpdate）
		handlerOK := true
		if cfg.Handlers != nil {
			func() {
				defer func() {
					if rv := recover(); rv != nil {
						slog.Error("telegram webhook: handler panic", "updateId", update.UpdateID, "err", fmt.Sprint(rv))
						handlerOK = false
					}
				}()
				cfg.Handlers.HandleUpdate(r.Context(), &update)
			}()
		}

		if handlerOK {
			w.WriteHeader(http.StatusOK)
		} else {
			// 对齐 TS: handler 出错时返回 500，Telegram 会重试
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	publicURL := cfg.PublicURL
	if publicURL == "" {
		h := host
		if h == "0.0.0.0" {
			h = "localhost"
		}
		publicURL = fmt.Sprintf("http://%s:%d%s", h, port, path)
	}

	if err := SetTelegramWebhook(ctx, client, cfg.Token, publicURL, cfg.Secret, false); err != nil {
		return nil, fmt.Errorf("set webhook: %w", err)
	}

	// 对齐 TS: await server.listen(port, host) 确保端口绑定完成后再返回
	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}

	server := &http.Server{Handler: mux}

	go func() {
		slog.Info("telegram webhook listening", "url", publicURL)
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("telegram webhook server error", "err", err)
		}
	}()

	// 对齐 TS: abortSignal → shutdown 优雅关闭
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("telegram webhook shutdown error", "err", err)
		}
	}()

	return server, nil
}
