package gateway

// server_channel_webhooks.go — 频道 webhook HTTP 路由
// Phase 5: 飞书 SDK EventDispatcher / 企业微信 XML 回调
// 钉钉使用 Stream 长连接，无需 HTTP webhook

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/anthropic/open-acosmi/internal/channels"
	"github.com/anthropic/open-acosmi/internal/channels/feishu"
	"github.com/anthropic/open-acosmi/internal/channels/wecom"

	"github.com/larksuite/oapi-sdk-go/v3/core/httpserverext"
)

// ChannelWebhookFeishu 飞书 webhook HTTP 处理器。
// 代理到飞书 SDK EventDispatcher（自动验签 + 解密 + URL challenge + 事件去重）。
func ChannelWebhookFeishu(mgr *channels.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		plugin := mgr.GetPlugin(channels.ChannelFeishu)
		if plugin == nil {
			http.Error(w, "feishu channel not configured", http.StatusNotFound)
			return
		}

		feishuPlugin, ok := plugin.(*feishu.FeishuPlugin)
		if !ok {
			http.Error(w, "feishu plugin type mismatch", http.StatusInternalServerError)
			return
		}

		dispatcher := feishuPlugin.GetDispatcher(channels.DefaultAccountID)
		if dispatcher == nil {
			http.Error(w, "feishu dispatcher not initialized", http.StatusServiceUnavailable)
			return
		}

		// 使用 SDK httpserverext 处理 HTTP 请求
		// SDK 自动处理：验签、解密、URL 验证 challenge、事件去重
		handler := httpserverext.NewEventHandlerFunc(dispatcher)
		handler(w, r)
	}
}

// ChannelWebhookWeCom 企业微信 webhook HTTP 处理器。
// 处理 GET（URL 验证）和 POST（消息回调）。
func ChannelWebhookWeCom(mgr *channels.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		plugin := mgr.GetPlugin(channels.ChannelWeCom)
		if plugin == nil {
			http.Error(w, "wecom channel not configured", http.StatusNotFound)
			return
		}

		wecomPlugin, ok := plugin.(*wecom.WeComPlugin)
		if !ok {
			http.Error(w, "wecom plugin type mismatch", http.StatusInternalServerError)
			return
		}

		handler := wecomPlugin.GetCallbackHandler(channels.DefaultAccountID)
		if handler == nil {
			http.Error(w, "wecom callback handler not initialized", http.StatusServiceUnavailable)
			return
		}

		switch r.Method {
		case http.MethodGet:
			// URL 验证（企业微信首次配置回调 URL 时发送 GET 验证请求）
			query := r.URL.Query()
			msgSignature := query.Get("msg_signature")
			timestamp := query.Get("timestamp")
			nonce := query.Get("nonce")
			echostr := query.Get("echostr")

			plainEchoStr, err := handler.VerifyURL(msgSignature, timestamp, nonce, echostr)
			if err != nil {
				slog.Warn("wecom: URL verification failed", "error", err)
				http.Error(w, "verification failed", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(plainEchoStr))

		case http.MethodPost:
			// 消息回调
			query := r.URL.Query()
			msgSignature := query.Get("msg_signature")
			timestamp := query.Get("timestamp")
			nonce := query.Get("nonce")

			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
			defer r.Body.Close()
			if err != nil {
				http.Error(w, "read body failed", http.StatusBadRequest)
				return
			}

			if err := handler.HandleCallback(msgSignature, timestamp, nonce, string(body)); err != nil {
				slog.Warn("wecom: callback handling failed", "error", err)
				http.Error(w, "callback failed", http.StatusInternalServerError)
				return
			}

			// 企业微信要求回调返回 "success"
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("success"))

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
