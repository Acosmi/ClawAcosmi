package discord

// Discord Interactions Endpoint Ed25519 签名验证
//
// Discord 对 HTTP Interactions Endpoint 使用 Ed25519 签名验证。
// 参考: https://discord.com/developers/docs/interactions/receiving-and-responding#security-and-authorization
//
// 请求头:
//   - X-Signature-Ed25519: hex 编码的 Ed25519 签名
//   - X-Signature-Timestamp: Unix 时间戳字符串
//
// 验证逻辑: ed25519.Verify(publicKey, timestamp+body, signature)

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
)

// ── 签名验证 ──

// VerifyDiscordInteractionSignature 验证 Discord Interaction 的 Ed25519 签名。
// publicKeyHex: hex 编码的 Ed25519 公钥（从 Discord Developer Portal 获取）
// signatureHex: hex 编码的 Ed25519 签名（来自 X-Signature-Ed25519 header）
// timestamp: 时间戳字符串（来自 X-Signature-Timestamp header）
// body: 请求体原始字节
func VerifyDiscordInteractionSignature(publicKeyHex, signatureHex, timestamp string, body []byte) bool {
	pubKey, err := hex.DecodeString(publicKeyHex)
	if err != nil || len(pubKey) != ed25519.PublicKeySize {
		return false
	}

	sig, err := hex.DecodeString(signatureHex)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return false
	}

	// Discord 签名验证: message = timestamp + body
	message := make([]byte, 0, len(timestamp)+len(body))
	message = append(message, []byte(timestamp)...)
	message = append(message, body...)

	return ed25519.Verify(pubKey, message, sig)
}

// ── HTTP Handler ──

// discordInteractionType Discord Interaction 类型
const (
	interactionTypePing               = 1
	interactionTypeApplicationCommand = 2
	interactionTypeMessageComponent   = 3
	interactionTypeAutocomplete       = 4
	interactionTypeModalSubmit        = 5
)

// discordInteractionResponseType 响应类型
const (
	interactionResponsePong = 1
)

// interactionPayload 交互请求体（仅解析 type 字段用于 PING 处理）
type interactionPayload struct {
	Type int `json:"type"`
}

// interactionResponse PING 响应
type interactionResponse struct {
	Type int `json:"type"`
}

// DiscordInteractionHandler Discord HTTP Interactions Endpoint handler。
// 处理 Discord 通过 HTTP POST 发送的 Interaction 请求。
type DiscordInteractionHandler struct {
	// PublicKey hex 编码的 Ed25519 应用公钥。
	PublicKey string

	// OnInteraction 处理非 PING 的 Interaction 回调。
	// body 为原始请求体（已验证签名），interactionType 为解析后的类型。
	OnInteraction func(w http.ResponseWriter, body []byte, interactionType int)
}

// ServeHTTP 实现 http.Handler 接口。
func (h *DiscordInteractionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 读取请求体（限制 4MB）
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// 获取签名头
	signature := r.Header.Get("X-Signature-Ed25519")
	timestamp := r.Header.Get("X-Signature-Timestamp")

	if signature == "" || timestamp == "" {
		http.Error(w, "Missing signature headers", http.StatusUnauthorized)
		return
	}

	// Ed25519 签名验证
	if !VerifyDiscordInteractionSignature(h.PublicKey, signature, timestamp, body) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// 解析 interaction type
	var payload interactionPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// PING → 自动 PONG
	if payload.Type == interactionTypePing {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(interactionResponse{Type: interactionResponsePong})
		return
	}

	// 其他 interaction 分发到回调
	if h.OnInteraction != nil {
		h.OnInteraction(w, body, payload.Type)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}
