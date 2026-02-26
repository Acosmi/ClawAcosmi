package whatsapp

// WhatsApp Cloud API Webhook 签名验证
//
// Meta/WhatsApp Cloud API 对 Webhook 使用 HMAC-SHA256 签名验证。
// 参考: https://developers.facebook.com/docs/graph-api/webhooks/getting-started
//
// 请求头:
//   - X-Hub-Signature-256: sha256=<hex_digest> (HMAC-SHA256)
//
// 验证逻辑: HMAC-SHA256(appSecret, body) == signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ── 签名验证 ──

// VerifyWhatsAppWebhookSignature 验证 WhatsApp webhook 的 HMAC-SHA256 签名。
// appSecret: Meta 应用密钥
// signatureHeader: X-Hub-Signature-256 header 原始值（格式: sha256=<hex>）
// body: 请求体原始字节
func VerifyWhatsAppWebhookSignature(appSecret, signatureHeader string, body []byte) bool {
	if appSecret == "" || signatureHeader == "" {
		return false
	}

	// 解析 "sha256=<hex>" 格式
	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}
	gotHex := signatureHeader[len(prefix):]

	gotSig, err := hex.DecodeString(gotHex)
	if err != nil {
		return false
	}

	// 计算期望的 HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(body)
	expectedSig := mac.Sum(nil)

	return hmac.Equal(gotSig, expectedSig)
}

// ── HTTP Handler ──

// WhatsAppWebhookHandler WhatsApp Cloud API Webhook HTTP handler。
// 处理两种请求:
//   - GET: 验证 webhook URL（hub.verify_token + hub.challenge）
//   - POST: 接收 webhook 事件（签名验证 + 事件分发）
type WhatsAppWebhookHandler struct {
	// AppSecret Meta 应用密钥（用于 HMAC-SHA256 签名验证）。
	AppSecret string

	// VerifyToken Webhook 验证 token（与 Meta Developer Portal 配置一致）。
	VerifyToken string

	// OnEvent 处理验证通过的 webhook 事件回调。
	// body 为原始请求体。
	OnEvent func(body []byte)
}

// ServeHTTP 实现 http.Handler 接口。
func (h *WhatsAppWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleVerification(w, r)
	case http.MethodPost:
		h.handleWebhookEvent(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleVerification 处理 webhook URL 验证请求。
// Meta 在配置 webhook URL 时发送 GET 请求验证:
//   - hub.mode = "subscribe"
//   - hub.verify_token = 配置的 token
//   - hub.challenge = 需要原样返回的 challenge 值
func (h *WhatsAppWebhookHandler) handleVerification(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	mode := q.Get("hub.mode")
	token := q.Get("hub.verify_token")
	challenge := q.Get("hub.challenge")

	if mode != "subscribe" || token != h.VerifyToken || challenge == "" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, challenge)
}

// handleWebhookEvent 处理 webhook 事件请求。
func (h *WhatsAppWebhookHandler) handleWebhookEvent(w http.ResponseWriter, r *http.Request) {
	// 读取请求体（限制 4MB）
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// HMAC-SHA256 签名验证
	if h.AppSecret != "" {
		signatureHeader := r.Header.Get("X-Hub-Signature-256")
		if signatureHeader == "" {
			http.Error(w, "Missing signature", http.StatusUnauthorized)
			return
		}
		if !VerifyWhatsAppWebhookSignature(h.AppSecret, signatureHeader, body) {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// 基础 JSON 有效性验证
	if !json.Valid(body) {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 分发到回调
	if h.OnEvent != nil {
		h.OnEvent(body)
	}

	// Meta 要求始终返回 200
	w.WriteHeader(http.StatusOK)
}
